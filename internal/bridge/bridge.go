// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package bridge is the daemon's composition: Home Assistant's registry
// and states, read through internal/ha, become go-hamqtt entities
// (internal/adapt), go-hamapping maps each Home Assistant device onto
// go-fabric's device kit, and the node's Aggregator serves the result.
// A controller's command comes back through go-hamapping's OnChange and
// leaves as a call_service.
//
// It follows go-fabric's cookbook "Embed in a daemon"
// (docs/guides/embed-in-a-daemon.md at v0.11.0): node.Config from the
// daemon's config (§2), one aggregator batch for the whole exposed set at
// start and on every registry change (§4), states to attributes through
// the plan's typed Sets (§5), reachability from each entity's
// availability and from the Home Assistant connection (§6), the pairing
// code to the log and the web UI (§7).
//
// The bridge holds no mapping: go-hamapping's Map decides every device
// type and cluster, and the bridge imports go-fabric's facade only
// (internal/arch).
package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SukramJ/go-fabric/device"
	"github.com/SukramJ/go-fabric/eligibility"
	"github.com/SukramJ/go-fabric/node"
	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// Upstream is the Home Assistant side the bridge reads and calls;
// *ha.Client is the daemon's.
type Upstream interface {
	Connected() bool
	ListEntities(ctx context.Context) ([]ha.EntityEntry, error)
	ListDevices(ctx context.Context) ([]ha.DeviceEntry, error)
	ListAreas(ctx context.Context) ([]ha.Area, error)
	GetStates(ctx context.Context) ([]ha.State, error)
	GetConfig(ctx context.Context) (ha.Config, error)
	ListExposed(ctx context.Context) (ha.Exposed, error)
	CallService(ctx context.Context, sc ha.ServiceCall) error
}

// Options configure a [Bridge].
type Options struct {
	// Rule is the exposure rule.
	Rule expose.Rule
	// Debounce coalesces registry events into one re-sync; 0 is one
	// second.
	Debounce time.Duration
	// Logger is the bridge's logger; nil is slog.Default().
	Logger *slog.Logger
}

// Verdict is one candidate entity's line of the plan an operator reads:
// whether the exposure rule takes it and why, and go-hamapping's verdict.
type Verdict struct {
	EntityID     string `json:"entity_id"`
	Name         string `json:"name,omitempty"`
	Device       string `json:"device,omitempty"`
	Exposed      bool   `json:"exposed"`
	ExposeReason string `json:"expose_reason"`
	Row          string `json:"row,omitempty"`
	State        string `json:"state,omitempty"`
	Reason       string `json:"reason,omitempty"`
	DeviceType   uint16 `json:"device_type,omitempty"`
	Endpoint     uint16 `json:"endpoint,omitempty"`
}

// Status is the bridge's state for the web UI and the health endpoint.
type Status struct {
	HAConnected bool      `json:"ha_connected"`
	LastSync    time.Time `json:"last_sync"`
	SyncError   string    `json:"sync_error,omitempty"`
	Endpoints   int       `json:"endpoints"`
	Candidates  int       `json:"candidates"`
	Commands    uint64    `json:"commands"`
	CommandErrs uint64    `json:"command_errors"`
	LastCommand string    `json:"last_command_error,omitempty"`
}

// group is one Home Assistant device and the plan serving its exposed
// entities.
type group struct {
	key      string
	sig      string
	device   *model.Device
	entities []string
	regs     map[string]*ha.EntityEntry
	descs    map[string]string
	plan     *hamapping.Plan
	verdicts []hamapping.Verdict
	devices  []*device.Device
	keys     []device.SourceKey
}

// Bridge is the composition. Its work — syncs, states, reachability —
// runs on one goroutine ([Bridge.Run]); controller commands run on
// another, so a slow service call never holds an attribute report.
type Bridge struct {
	n    *node.Node
	up   Upstream
	opts Options
	log  *slog.Logger

	work chan func(context.Context)
	cmds chan model.Command

	connected atomic.Bool
	complete  bool

	// worker-owned
	groups map[string]*group
	owner  map[string]*group
	known  map[string]bool
	unit   string

	mu       sync.Mutex
	states   map[string]*ha.State
	verdicts []Verdict
	status   Status
	timer    *time.Timer
	synced   chan struct{}
}

// New returns the bridge of node n and upstream up. The node may be
// started or not; [Bridge.Run] serves it.
func New(n *node.Node, up Upstream, opts Options) *Bridge {
	if opts.Debounce <= 0 {
		opts.Debounce = time.Second
	}
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Bridge{
		n: n, up: up, opts: opts, log: log,
		work:   make(chan func(context.Context), 1024),
		cmds:   make(chan model.Command, 256),
		groups: map[string]*group{}, owner: map[string]*group{}, known: map[string]bool{},
		states: map[string]*ha.State{},
		synced: make(chan struct{}),
	}
}

// Run serves the bridge until ctx ends: the worker and the command
// queue. Home Assistant's connection reaches it through the
// [ha.Handler] methods the bridge implements.
func (b *Bridge) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	wg.Go(func() { b.commandLoop(ctx) })
	defer wg.Wait()
	for {
		select {
		case <-ctx.Done():
			b.mu.Lock()
			if b.timer != nil {
				b.timer.Stop()
			}
			b.mu.Unlock()
			for _, g := range b.groups {
				g.plan.Close()
			}
			return nil
		case fn := <-b.work:
			fn(ctx)
		}
	}
}

// enqueue hands work to the worker; it never blocks the caller (an
// ha.Handler method on the client's goroutine).
func (b *Bridge) enqueue(fn func(context.Context)) {
	select {
	case b.work <- fn:
	default:
		b.log.Error("bridge.queue_full", slog.String("hint", "Home Assistant sends faster than the bridge applies; a re-sync follows"))
		b.resyncSoon()
	}
}

// Synced is closed after the first sync finished.
func (b *Bridge) Synced() <-chan struct{} { return b.synced }

// Connected implements ha.Handler: a (re)connection reads everything
// again.
func (b *Bridge) Connected(string) {
	b.connected.Store(true)
	b.setStatus(func(s *Status) { s.HAConnected = true })
	b.enqueue(func(ctx context.Context) { b.sync(ctx) })
}

// Disconnected implements ha.Handler: while Home Assistant is away every
// endpoint is unreachable (cookbook §6, "Upstream down").
func (b *Bridge) Disconnected(error) {
	if !b.connected.Swap(false) {
		return
	}
	b.setStatus(func(s *Status) { s.HAConnected = false })
	b.enqueue(func(ctx context.Context) {
		for _, g := range b.groups {
			for _, d := range g.devices {
				d.SetReachable(ctx, false)
			}
		}
		b.log.Warn("bridge.upstream_unusable", slog.String("hint", "every endpoint is unreachable until Home Assistant is back"))
	})
}

// Event implements ha.Handler.
func (b *Bridge) Event(ev ha.Event) {
	switch ev.EventType {
	case ha.EventStateChanged:
		var sc ha.StateChanged
		if err := json.Unmarshal(ev.Data, &sc); err != nil {
			b.log.Warn("bridge.event_refused", slog.String("event", ev.EventType), slog.String("error", err.Error()))
			return
		}
		b.enqueue(func(ctx context.Context) { b.stateChanged(ctx, sc) })
	case ha.EventEntityRegistryUpdated, ha.EventDeviceRegistryUpdated:
		b.resyncSoon()
	}
}

// resyncSoon schedules one sync after the debounce; events within it
// coalesce.
func (b *Bridge) resyncSoon() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(b.opts.Debounce, func() {
		select {
		case b.work <- func(ctx context.Context) { b.sync(ctx) }:
		default:
		}
	})
}

func (b *Bridge) setStatus(fn func(*Status)) {
	b.mu.Lock()
	fn(&b.status)
	b.mu.Unlock()
}

// Status returns the bridge's state.
func (b *Bridge) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.status
}

// Plan returns every candidate entity's verdict, in entity id order.
func (b *Bridge) Plan() []Verdict {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.verdicts)
}

// Snapshot is what one sync read from Home Assistant.
type Snapshot struct {
	Entities []ha.EntityEntry
	Devices  []ha.DeviceEntry
	Areas    []ha.Area
	States   []ha.State
	Config   ha.Config
	Exposed  ha.Exposed
}

// Read reads the registries, the states, the configuration and — when
// the rule follows Assist — the exposure list.
func Read(ctx context.Context, up Upstream, rule expose.Rule) (*Snapshot, error) {
	var s Snapshot
	var err error
	if s.Entities, err = up.ListEntities(ctx); err != nil {
		return nil, fmt.Errorf("config/entity_registry/list: %w", err)
	}
	if s.Devices, err = up.ListDevices(ctx); err != nil {
		return nil, fmt.Errorf("config/device_registry/list: %w", err)
	}
	if s.Areas, err = up.ListAreas(ctx); err != nil {
		return nil, fmt.Errorf("config/area_registry/list: %w", err)
	}
	if s.States, err = up.GetStates(ctx); err != nil {
		return nil, fmt.Errorf("get_states: %w", err)
	}
	if s.Config, err = up.GetConfig(ctx); err != nil {
		return nil, fmt.Errorf("get_config: %w", err)
	}
	if rule.Assist {
		if s.Exposed, err = up.ListExposed(ctx); err != nil {
			return nil, fmt.Errorf("homeassistant/expose_entity/list: %w", err)
		}
	}
	return &s, nil
}

// planned is one device's exposed entities, ready for go-hamapping.
type planned struct {
	key      string
	device   *model.Device
	entities []model.Entity
	regs     map[string]*ha.EntityEntry
	descs    map[string]string
	sig      string
}

// assess applies the exposure rule and groups the exposed entities by
// device: the candidates' verdicts (go-hamapping's verdict left to the
// caller) and the devices to plan. A candidate is every entity with a
// state or a registry entry.
func assess(s *Snapshot, rule expose.Rule) ([]Verdict, []*planned) {
	reg := map[string]*ha.EntityEntry{}
	for i := range s.Entities {
		reg[s.Entities[i].EntityID] = &s.Entities[i]
	}
	devs := map[string]*ha.DeviceEntry{}
	for i := range s.Devices {
		devs[s.Devices[i].ID] = &s.Devices[i]
	}
	areas := map[string]string{}
	for _, a := range s.Areas {
		areas[a.AreaID] = a.Name
	}
	states := map[string]*ha.State{}
	ids := map[string]bool{}
	for i := range s.States {
		states[s.States[i].EntityID] = &s.States[i]
		ids[s.States[i].EntityID] = true
	}
	for id := range reg {
		ids[id] = true
	}
	order := make([]string, 0, len(ids))
	for id := range ids {
		order = append(order, id)
	}
	sort.Strings(order)

	unit := s.Config.TemperatureUnit()
	groups := map[string]*planned{}
	var keys []string
	verdicts := make([]Verdict, 0, len(order))
	for _, id := range order {
		r, st := reg[id], states[id]
		e := expose.Entity{EntityID: id, Registered: r != nil, ExposedToAssist: s.Exposed.Entities[id][ha.AssistantConversation]}
		if r != nil {
			e.DisabledBy, e.HiddenBy = r.DisabledBy, r.HiddenBy
		}
		d := rule.Decide(e)
		v := Verdict{EntityID: id, Exposed: d.Exposed, ExposeReason: d.Reason}
		if st != nil {
			v.Name, _ = st.Attributes["friendly_name"].(string)
		}
		if d.Exposed && st == nil {
			v.Exposed, v.ExposeReason = false, d.Reason+"; no state yet"
		}
		if !v.Exposed {
			verdicts = append(verdicts, v)
			continue
		}
		var md *model.Device
		var key string
		if r != nil && r.DeviceID != "" && devs[r.DeviceID] != nil {
			md = adapt.Device(devs[r.DeviceID], areas)
		} else {
			name, _ := st.Attributes["friendly_name"].(string)
			if name == "" {
				name = id
			}
			md = adapt.OwnDevice(id, name)
		}
		key = md.UID()
		v.Device = md.Name.Default
		g := groups[key]
		if g == nil {
			g = &planned{key: key, device: md, descs: map[string]string{}, regs: map[string]*ha.EntityEntry{}}
			groups[key] = g
			keys = append(keys, key)
		}
		ent := adapt.Entity(adapt.Input{EntityID: id, Registry: r, State: st, Device: md, TemperatureUnit: unit})
		g.entities = append(g.entities, ent)
		g.regs[id] = r
		g.descs[id] = describe(ent)
		verdicts = append(verdicts, v)
	}
	out := make([]*planned, 0, len(keys))
	for _, k := range keys {
		g := groups[k]
		dev, _ := json.Marshal(g.device)
		sig := string(dev)
		for _, e := range g.entities {
			sig += "|" + e.Key() + "=" + g.descs[e.Key()]
		}
		g.sig = sig
		out = append(out, g)
	}
	return verdicts, out
}

// describe is an entity's description as a comparable string: a change
// means a new plan for its device.
func describe(e model.Entity) string {
	b, _ := json.Marshal(struct {
		P string
		D *model.Description
	}{string(e.Platform()), e.Desc()})
	return string(b)
}

// mapVerdicts merges go-hamapping's verdicts into the candidates'.
func mapVerdicts(verdicts []Verdict, mv []hamapping.Verdict) {
	idx := map[string]int{}
	for i := range verdicts {
		idx[verdicts[i].EntityID] = i
	}
	for _, m := range mv {
		i, ok := idx[m.Key]
		if !ok {
			continue
		}
		verdicts[i].Row, verdicts[i].State, verdicts[i].Reason, verdicts[i].DeviceType = m.Row, m.State.String(), m.Reason, m.DeviceType
	}
}

// Dry is the plan without a node: what --dump-plan prints.
func Dry(s *Snapshot, rule expose.Rule) []Verdict {
	verdicts, groups := assess(s, rule)
	for _, g := range groups {
		_, mv := hamapping.Map(g.device, g.entities)
		mapVerdicts(verdicts, mv)
	}
	return verdicts
}

// sync reads everything and brings the Aggregator's set to it in one
// batch: new devices added, changed ones replaced, vanished ones removed.
func (b *Bridge) sync(ctx context.Context) {
	if !b.up.Connected() {
		return
	}
	snap, err := Read(ctx, b.up, b.opts.Rule)
	if err != nil {
		b.log.Error("bridge.sync_failed", slog.String("error", err.Error()))
		b.setStatus(func(s *Status) { s.SyncError = err.Error() })
		return
	}
	b.unit = snap.Config.TemperatureUnit()
	verdicts, plannedGroups := assess(snap, b.opts.Rule)

	b.mu.Lock()
	b.states = map[string]*ha.State{}
	for i := range snap.States {
		b.states[snap.States[i].EntityID] = &snap.States[i]
	}
	b.mu.Unlock()
	b.known = map[string]bool{}
	for i := range verdicts {
		b.known[verdicts[i].EntityID] = true
	}

	next := map[string]*group{}
	var added, replaced, removed []*group
	var closing []*group
	for _, p := range plannedGroups {
		if old := b.groups[p.key]; old != nil && old.sig == p.sig {
			next[p.key] = old
			mapVerdicts(verdicts, old.verdicts)
			continue
		}
		g, mv := b.build(ctx, p)
		mapVerdicts(verdicts, mv)
		next[p.key] = g
		if old := b.groups[p.key]; old != nil {
			replaced = append(replaced, g)
			closing = append(closing, old)
		} else {
			added = append(added, g)
		}
	}
	for k, old := range b.groups {
		if next[k] == nil {
			removed = append(removed, old)
			closing = append(closing, old)
		}
	}
	err = b.n.Aggregator().Batch(ctx, func(bt *node.Batch) error {
		var errs []error
		for _, g := range removed {
			for _, k := range g.keys {
				errs = append(errs, bt.Remove(k))
			}
		}
		for _, g := range replaced {
			old := b.groups[g.key]
			oldKeys := map[device.SourceKey]bool{}
			for _, k := range old.keys {
				oldKeys[k] = true
			}
			for i, k := range g.keys {
				if oldKeys[k] {
					errs = append(errs, bt.Replace(k, g.devices[i]))
					delete(oldKeys, k)
				} else {
					errs = append(errs, bt.Add(g.devices[i]))
				}
			}
			for _, k := range old.keys {
				if oldKeys[k] {
					errs = append(errs, bt.Remove(k))
				}
			}
		}
		for _, g := range added {
			for _, d := range g.devices {
				errs = append(errs, bt.Add(d))
			}
		}
		if !b.complete {
			bt.MarkComplete()
		}
		return errors.Join(errs...)
	})
	b.complete = true
	if err != nil {
		b.log.Error("bridge.batch", slog.String("error", err.Error()))
	}
	for _, g := range closing {
		g.plan.Close()
	}
	b.groups = next
	b.owner = map[string]*group{}
	for _, g := range next {
		for _, id := range g.entities {
			b.owner[id] = g
		}
	}
	// Every state onto its device: values and reachability.
	endpoints := 0
	for _, g := range next {
		endpoints += len(g.devices)
		for _, id := range g.entities {
			b.apply(ctx, g, id)
		}
	}
	for i := range verdicts {
		if g := b.owner[verdicts[i].EntityID]; g != nil && g.plan != nil {
			if d := g.plan.DeviceFor(verdicts[i].EntityID); d != nil {
				verdicts[i].Endpoint, _ = d.EndpointNumber()
			}
		}
	}
	b.mu.Lock()
	b.verdicts = verdicts
	b.status.LastSync = time.Now()
	b.status.SyncError = ""
	b.status.Endpoints = endpoints
	b.status.Candidates = len(verdicts)
	select {
	case <-b.synced:
	default:
		close(b.synced)
	}
	b.mu.Unlock()
	b.log.Info("bridge.synced", slog.Int("candidates", len(verdicts)), slog.Int("devices", len(next)), slog.Int("endpoints", endpoints),
		slog.Int("added", len(added)), slog.Int("replaced", len(replaced)), slog.Int("removed", len(removed)))
}

// build maps and builds one device's plan; a build failure leaves the
// device without endpoints and says why in its verdicts.
func (b *Bridge) build(ctx context.Context, p *planned) (*group, []hamapping.Verdict) {
	plan, mv := hamapping.Map(p.device, p.entities)
	g := &group{key: p.key, sig: p.sig, device: p.device, regs: p.regs, descs: p.descs, plan: plan}
	for _, e := range p.entities {
		g.entities = append(g.entities, e.Key())
	}
	devs, err := plan.Build(ctx)
	if err != nil {
		b.log.Warn("bridge.build_failed", slog.String("device", p.device.Name.Default), slog.String("error", err.Error()))
		for i := range mv {
			if mv[i].State != eligibility.StateUnmappable {
				mv[i].State = eligibility.StateUnmappable
				mv[i].Reason = "go-fabric refused the device: " + err.Error()
			}
		}
		g.verdicts = mv
		return g, mv
	}
	g.verdicts = mv
	g.devices = devs
	for i := range plan.Endpoints {
		g.keys = append(g.keys, plan.Endpoints[i].Options.StableKey)
	}
	plan.OnChange(b.onCommand)
	return g, mv
}

// apply carries one entity's current state onto its device.
func (b *Bridge) apply(ctx context.Context, g *group, id string) {
	if len(g.devices) == 0 {
		return
	}
	b.mu.Lock()
	st := b.states[id]
	b.mu.Unlock()
	if st == nil {
		return
	}
	for _, p := range g.plan.Apply(ctx, id, adapt.State(st)) {
		if p.Err != nil {
			b.log.Debug("bridge.apply", slog.String("entity", id), slog.String("cluster", p.Cluster), slog.String("error", p.Err.Error()))
		}
	}
}

// stateChanged applies one state_changed: the value onto the device, or a
// re-sync when the entity's description moved (a capability attribute
// appeared) or the entity is new.
func (b *Bridge) stateChanged(ctx context.Context, sc ha.StateChanged) {
	b.mu.Lock()
	if sc.NewState == nil {
		delete(b.states, sc.EntityID)
	} else {
		b.states[sc.EntityID] = sc.NewState
	}
	b.mu.Unlock()
	if sc.NewState == nil || !b.known[sc.EntityID] {
		b.resyncSoon()
		return
	}
	g := b.owner[sc.EntityID]
	if g == nil {
		return
	}
	if g.descs[sc.EntityID] != b.descOf(g, sc.EntityID, sc.NewState) {
		b.resyncSoon()
	}
	b.apply(ctx, g, sc.EntityID)
}

// descOf is the description an entity would have with st.
func (b *Bridge) descOf(g *group, id string, st *ha.State) string {
	return describe(adapt.Entity(adapt.Input{EntityID: id, Registry: g.regs[id], State: st, Device: g.device, TemperatureUnit: b.unit}))
}
