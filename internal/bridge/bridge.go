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
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"
	"github.com/SukramJ/go-fabric/device"
	"github.com/SukramJ/go-fabric/eligibility"
	"github.com/SukramJ/go-fabric/node"
	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/selection"
)

// Upstream is the Home Assistant side the bridge reads and calls;
// *ha.Client is the daemon's.
type Upstream interface {
	Connected() bool
	ListEntities(ctx context.Context) ([]ha.EntityEntry, error)
	ListDevices(ctx context.Context) ([]ha.DeviceEntry, error)
	ListAreas(ctx context.Context) ([]ha.Area, error)
	ListLabels(ctx context.Context) ([]ha.Label, error)
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
	// Selection is the web UI's overrides; nil is none. The rule reads
	// them at every sync: disabled/hidden, override, exclude, include,
	// Assist.
	Selection *selection.Store
	// OnChange, when set, is called on the worker after a sync that
	// changed the exposure (the entity ids whose decision moved) or the
	// endpoints (added, replaced or removed devices).
	OnChange func(exposure []string, endpoints bool)
	// Camera maps camera entities (config.Config.Camera). Off, an
	// exposed camera is skipped before go-hamapping sees it, with
	// cameraOff as its verdict.
	Camera bool
}

// Verdict is one candidate entity's line of the plan an operator reads:
// whether the exposure rule takes it and why, and go-hamapping's verdict.
type Verdict struct {
	EntityID     string `json:"entity_id"`
	Name         string `json:"name,omitempty"`
	Device       string `json:"device,omitempty"`
	Exposed      bool   `json:"exposed"`
	ExposeReason string `json:"expose_reason"`
	// Source is the exposure rule's layer that decided (expose.Source…).
	Source string `json:"source"`
	// Labels are the names of the entity's and its device's labels.
	Labels     []string `json:"labels,omitempty"`
	Row        string   `json:"row,omitempty"`
	State      string   `json:"state,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	DeviceType uint16   `json:"device_type,omitempty"`
	Endpoint   uint16   `json:"endpoint,omitempty"`

	// verdict is go-hamapping's verdict in full (with the clusters).
	verdict eligibility.Verdict
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
	// Warnings are the configuration's problems that do not stop the
	// bridge: a label value that names no Home Assistant label.
	Warnings []string `json:"warnings,omitempty"`
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
	names    map[string]string
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

	// worker-owned
	groups map[string]*group
	owner  map[string]*group
	known  map[string]bool
	unit   string
	warned string
	since  map[string]time.Time
	// forget is the stable key of every entity removed from Home
	// Assistant's registry since the last sync, by entity id: the next
	// sync forgets it instead of removing it ([Bridge.noteRemoved]).
	forget map[string]device.SourceKey
	// cameraLogged is set once the skipped cameras were logged.
	cameraLogged bool

	mu       sync.Mutex
	states   map[string]*ha.State
	verdicts []Verdict
	last     *Snapshot
	all      map[string]*Verdict
	status   Status
	warnings []fabricui.Warning
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
		forget: map[string]device.SourceKey{},
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
	case ha.EventEntityRegistryUpdated:
		// A remove takes the entity out of the registry for good
		// (homeassistant/helpers/entity_registry.py:171-175, :1728-1775 at
		// 2026.10.3): its endpoint number is forgotten, not kept. An
		// update (disabled, hidden, renamed, unexposed) keeps it.
		var u struct {
			Action   string `json:"action"`
			EntityID string `json:"entity_id"`
		}
		if err := json.Unmarshal(ev.Data, &u); err == nil && u.Action == "remove" && u.EntityID != "" {
			b.enqueue(func(context.Context) { b.noteRemoved(u.EntityID) })
		}
		b.resyncSoon()
	case ha.EventDeviceRegistryUpdated, ha.EventLabelRegistryUpdated:
		// A label rename is an update with the same label_id
		// (helpers/label_registry.py:183-224): the re-sync resolves the
		// names again.
		b.resyncSoon()
	}
}

// noteRemoved records the stable key of an entity removed from the
// registry, read from the last snapshot (the new one no longer has its
// entry), for the next sync to forget. It runs on the worker.
//
// Home Assistant keeps a removed entry's id in deleted_entities and gives
// it back when the entity is created again
// (homeassistant/helpers/entity_registry.py:1728-1775, :1624 at
// 2026.10.3), so without Forget a re-added entity would get its old
// number from go-fabric's store; with it, it gets a fresh one, as a new
// device would (node.Aggregator.Forget).
func (b *Bridge) noteRemoved(entityID string) {
	b.mu.Lock()
	snap := b.last
	b.mu.Unlock()
	if k := removedKey(snap, entityID); k != nil {
		b.forget[entityID] = k
	}
}

// removedKey is the stable key go-hamapping gives entityID's endpoint
// (its device's UID, "/", the registry entry id; see mapOptions and
// hamapping's Plan.options), or nil when s has no registry entry for it.
func removedKey(s *Snapshot, entityID string) device.SourceKey {
	if s == nil {
		return nil
	}
	for i := range s.Entities {
		r := &s.Entities[i]
		if r.EntityID != entityID || r.ID == "" {
			continue
		}
		md := adapt.OwnDevice(entityID, "")
		for j := range s.Devices {
			if r.DeviceID != "" && s.Devices[j].ID == r.DeviceID {
				areas := map[string]string{}
				for _, a := range s.Areas {
					areas[a.AreaID] = a.Name
				}
				md = adapt.Device(&s.Devices[j], areas)
			}
		}
		return device.StringKey(md.UID() + "/" + r.ID)
	}
	return nil
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
	Labels   []ha.Label
	States   []ha.State
	Config   ha.Config
	Exposed  ha.Exposed
}

// LabelRegistry is the snapshot's labels as the exposure rule resolves
// them.
func (s *Snapshot) LabelRegistry() expose.Labels {
	out := make(expose.Labels, len(s.Labels))
	for _, l := range s.Labels {
		out[l.LabelID] = l.Name
	}
	return out
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
	if s.Labels, err = up.ListLabels(ctx); err != nil {
		return nil, fmt.Errorf("config/label_registry/list: %w", err)
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
	names    map[string]string
	sig      string
}

// cameraOff is the verdict of an exposed camera while the camera option
// is off.
const cameraOff = "the camera option is off (CAMERA, add-on option camera)"

// isCamera reports whether an entity id is of the camera domain.
func isCamera(id string) bool { return strings.HasPrefix(id, "camera.") }

// assess applies the exposure rule and groups the exposed entities by
// device: the candidates' verdicts (go-hamapping's verdict left to the
// caller) and the devices to plan. A candidate is every entity with a
// state or a registry entry. names are the web UI's name overrides.
// Without camera, an exposed camera is not planned; its verdict is
// unmappable with cameraOff.
func assess(s *Snapshot, rule expose.Rule, names map[string]string, camera bool) ([]Verdict, []*planned) {
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

	labels := s.LabelRegistry()
	unit := s.Config.TemperatureUnit()
	groups := map[string]*planned{}
	var keys []string
	verdicts := make([]Verdict, 0, len(order))
	for _, id := range order {
		r, st := reg[id], states[id]
		e := expose.Entity{EntityID: id, Registered: r != nil, ExposedToAssist: s.Exposed.Entities[id][ha.AssistantConversation]}
		if r != nil {
			e.DisabledBy, e.HiddenBy = r.DisabledBy, r.HiddenBy
			e.Labels = slices.Clone(r.Labels)
			if dev := devs[r.DeviceID]; dev != nil {
				for _, l := range dev.Labels {
					if !slices.Contains(e.Labels, l) {
						e.Labels = append(e.Labels, l)
					}
				}
			}
		}
		d := rule.Decide(e, labels)
		v := Verdict{EntityID: id, Exposed: d.Exposed, ExposeReason: d.Reason, Source: d.Source}
		for _, l := range e.Labels {
			if n := labels[l]; n != "" {
				v.Labels = append(v.Labels, n)
			} else {
				v.Labels = append(v.Labels, l)
			}
		}
		if r != nil && r.DeviceID != "" && devs[r.DeviceID] != nil {
			v.Device = deviceName(devs[r.DeviceID])
		}
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
		if !camera && isCamera(id) {
			v.State, v.Reason = eligibility.StateUnmappable.String(), cameraOff
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
			g = &planned{key: key, device: md, descs: map[string]string{}, regs: map[string]*ha.EntityEntry{}, names: map[string]string{}}
			groups[key] = g
			keys = append(keys, key)
		}
		ent := adapt.Entity(adapt.Input{EntityID: id, Registry: r, State: st, Device: md, TemperatureUnit: unit, Name: names[id]})
		g.entities = append(g.entities, ent)
		g.regs[id] = r
		if names[id] != "" {
			g.names[id] = names[id]
		}
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

// logCameraOff logs, once per start, the exposed cameras the camera
// option left out.
func (b *Bridge) logCameraOff(verdicts []Verdict) {
	if b.cameraLogged {
		return
	}
	n := 0
	for i := range verdicts {
		if verdicts[i].Reason == cameraOff {
			n++
		}
	}
	if n > 0 {
		b.cameraLogged = true
		b.log.Info("bridge.camera_disabled", slog.Int("skipped", n), slog.String("enable", "CAMERA=true or the add-on option camera"))
	}
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
		verdicts[i].verdict = m.Verdict
	}
}

// deviceName is a registry device's name: the user's over the
// integration's (adapt.Device).
func deviceName(d *ha.DeviceEntry) string {
	if d.NameByUser != "" {
		return d.NameByUser
	}
	return d.Name
}

// mapOptions are go-hamapping's options for one device: the stable key
// of an entity is its registry entry id (helpers/entity_registry.py:
// 261-263), which a rename of the entity id keeps (:2051-2061, :2104),
// so a renamed entity keeps its endpoint number. An entity without a
// registry entry keeps go-hamapping's default, its entity id.
//
// names are the web UI's name overrides: an overridden entity's node label
// is the name as typed, without the device name (hamapping.Options.Name,
// verbatim; the owner's decision on H3 Q3, a name typed in a UI is final).
func mapOptions(regs map[string]*ha.EntityEntry, names map[string]string) hamapping.Options {
	return hamapping.Options{
		StableKey: func(e model.Entity) string {
			if r := regs[e.Key()]; r != nil {
				return r.ID
			}
			return ""
		},
		Name: func(e model.Entity) (string, bool) {
			n := names[e.Key()]
			return n, n != ""
		},
	}
}

// Dry is the plan without a node: what --dump-plan prints. names are the
// web UI's name overrides (nil: none).
func Dry(s *Snapshot, rule expose.Rule, names map[string]string, camera bool) []Verdict {
	verdicts, groups := assess(s, rule, names, camera)
	for _, g := range groups {
		_, mv := hamapping.Map(g.device, g.entities, mapOptions(g.regs, g.names))
		mapVerdicts(verdicts, mv)
	}
	return verdicts
}

// candidates is every entity's verdict as if the lists exposed it: the
// verdict the web UI shows for an entity the rule leaves out. Disabled
// and hidden entities stay out.
func candidates(s *Snapshot, names map[string]string, camera bool) map[string]*Verdict {
	out := map[string]*Verdict{}
	vs := Dry(s, expose.Rule{Include: []string{"*"}}, names, camera)
	for i := range vs {
		out[vs[i].EntityID] = &vs[i]
	}
	return out
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
	rule, names := b.rule(), b.names()
	verdicts, plannedGroups := assess(snap, rule, names, b.opts.Camera)
	all := candidates(snap, names, b.opts.Camera)
	b.logCameraOff(verdicts)
	warnings, uiWarnings := b.warn(rule, snap.LabelRegistry())

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
	b.mu.Lock()
	before := map[string]bool{}
	for i := range b.verdicts {
		before[b.verdicts[i].EntityID] = b.verdicts[i].Exposed
	}
	b.mu.Unlock()

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
	// An entity removed from the registry is forgotten instead of
	// removed; one that is back in the plan already (re-added within the
	// debounce) keeps its endpoint.
	live := map[string]bool{}
	for _, g := range next {
		for _, k := range g.keys {
			live[k.String()] = true
		}
	}
	forgetting := map[string]string{}
	for id, k := range b.forget {
		if !live[k.String()] {
			forgetting[k.String()] = id
		}
	}
	b.forget = map[string]device.SourceKey{}
	forgotten := map[string]bool{}
	drop := func(bt *node.Batch, k device.SourceKey) error {
		if _, ok := forgetting[k.String()]; ok {
			forgotten[k.String()] = true
			return bt.Forget(k)
		}
		return bt.Remove(k)
	}
	err = b.n.Aggregator().Batch(ctx, func(bt *node.Batch) error {
		var errs []error
		for _, g := range removed {
			for _, k := range g.keys {
				errs = append(errs, drop(bt, k))
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
					errs = append(errs, drop(bt, k))
				}
			}
		}
		// A removed entity the bridge did not serve (disabled, hidden,
		// unexposed): its stored number is freed all the same.
		for k := range forgetting {
			if !forgotten[k] {
				errs = append(errs, bt.Forget(device.StringKey(k)))
			}
		}
		for _, g := range added {
			for _, d := range g.devices {
				errs = append(errs, bt.Add(d))
			}
		}
		return errors.Join(errs...)
	})
	if err != nil {
		b.log.Error("bridge.batch", slog.String("error", err.Error()))
	}
	for _, k := range slices.Sorted(maps.Keys(forgetting)) {
		b.log.Info("bridge.forget", slog.String("entity", forgetting[k]), slog.String("key", k))
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
	var moved []string
	for i := range verdicts {
		v := &verdicts[i]
		if was, ok := before[v.EntityID]; ok && was != v.Exposed || !ok && v.Exposed {
			moved = append(moved, v.EntityID)
		}
		delete(before, v.EntityID)
	}
	for id, was := range before {
		if was {
			moved = append(moved, id)
		}
	}
	slices.Sort(moved)
	b.mu.Lock()
	b.verdicts = verdicts
	b.last = snap
	b.all = all
	b.status.Warnings = warnings
	b.warnings = uiWarnings
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
	changed := len(added)+len(replaced)+len(removed) > 0
	if b.opts.OnChange != nil && (len(moved) > 0 || changed) {
		b.opts.OnChange(moved, changed)
	}
}

// rule is the configured rule with the web UI's overrides.
func (b *Bridge) rule() expose.Rule {
	r := b.opts.Rule
	if b.opts.Selection != nil {
		r.Overrides = b.opts.Selection.Enabled()
	}
	return r
}

// names are the web UI's name overrides.
func (b *Bridge) names() map[string]string {
	if b.opts.Selection == nil {
		return nil
	}
	out := map[string]string{}
	for id, e := range b.opts.Selection.Entries() {
		if e.Name != "" {
			out[id] = e.Name
		}
	}
	return out
}

// WarningLabelUnresolved is the web UI's code of a label value that
// names no Home Assistant label (the log's bridge.label_unresolved).
const WarningLabelUnresolved = "label_unresolved"

// WarningSelectionCorrupt is the web UI's code of an overrides file that
// did not parse and was moved aside at start (the log's
// selection.corrupt).
const WarningSelectionCorrupt = "selection_corrupt"

// warn is the configuration's warnings against the label registry,
// logged when they change: a label value that names no label is used as
// a label_id and matches once such a label exists. The second result is
// the same warnings for the web UI, each with the time it was first seen
// in an unbroken run of syncs.
func (b *Bridge) warn(rule expose.Rule, labels expose.Labels) ([]string, []fabricui.Warning) {
	unresolved := rule.Unresolved(labels)
	out := make([]string, 0, len(unresolved))
	ui := make([]fabricui.Warning, 0, len(unresolved))
	since := map[string]time.Time{}
	now := time.Now()
	for _, v := range unresolved {
		msg := fmt.Sprintf("label %q names no Home Assistant label; it matches a label with that id once one exists", v)
		out = append(out, msg)
		at, ok := b.since[msg]
		if !ok {
			at = now
		}
		since[msg] = at
		ui = append(ui, fabricui.Warning{Code: WarningLabelUnresolved, Message: msg, Since: at})
	}
	b.since = since
	if len(out) == 0 {
		out, ui = nil, nil
	}
	if key := fmt.Sprint(out); key != b.warned {
		b.warned = key
		for _, w := range out {
			b.log.Warn("bridge.label_unresolved", slog.String("warning", w))
		}
	}
	return out, ui
}

// Warnings are the configuration's half-valid parts for the web UI's
// status (fabricui.Options.Warnings): the label values of the last sync
// that name no Home Assistant label, and an overrides file that was moved
// aside at start. The log carries the same findings.
func (b *Bridge) Warnings(context.Context) []fabricui.Warning {
	b.mu.Lock()
	out := slices.Clone(b.warnings)
	b.mu.Unlock()
	if b.opts.Selection != nil {
		if r, ok := b.opts.Selection.Recovered(); ok {
			out = append(out, fabricui.Warning{
				Code:    WarningSelectionCorrupt,
				Message: fmt.Sprintf("the overrides file did not parse (%s) and was moved to %s; the web UI's overrides started empty", r.Err, r.MovedTo),
				Since:   r.At,
			})
		}
	}
	return out
}

// Resync reads Home Assistant again and brings the endpoints to it, and
// returns when that sync has run: the web UI's reassemble.
func (b *Bridge) Resync(ctx context.Context) error {
	if !b.up.Connected() {
		return errors.New("bridge: Home Assistant is not connected")
	}
	done := make(chan struct{})
	select {
	case b.work <- func(ctx context.Context) {
		defer close(done)
		b.sync(ctx)
	}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if e := b.Status().SyncError; e != "" {
		return errors.New("bridge: " + e)
	}
	return nil
}

// build maps and builds one device's plan; a build failure leaves the
// device without endpoints and says why in its verdicts.
func (b *Bridge) build(ctx context.Context, p *planned) (*group, []hamapping.Verdict) {
	plan, mv := hamapping.Map(p.device, p.entities, mapOptions(p.regs, p.names))
	g := &group{key: p.key, sig: p.sig, device: p.device, regs: p.regs, descs: p.descs, names: p.names, plan: plan}
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
	return describe(adapt.Entity(adapt.Input{EntityID: id, Registry: g.regs[id], State: st, Device: g.device, TemperatureUnit: b.unit, Name: g.names[id]}))
}
