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
	"sync"
	"sync/atomic"
	"time"

	"github.com/SukramJ/go-fabric/device"
	"github.com/SukramJ/go-fabric/eligibility"
	"github.com/SukramJ/go-fabric/node"
	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// Upstream is the Home Assistant side the bridge reads and calls: the
// snapshot's reads, the connection's state and the service call;
// *ha.Client is the daemon's.
type Upstream interface {
	ha.Source
	Connected() bool
	CallService(ctx context.Context, sc ha.ServiceCall) error
}

// Options configure a [Bridge].
type Options struct {
	// Input is the plan's input at every sync: the effective exposure
	// rule (the configured one with the web UI's overrides, read at every
	// sync), the names, the camera option and the dry camera ports
	// (plan.Effective). Required.
	Input func() plan.Input
	// Debounce coalesces registry events into one re-sync; 0 is one
	// second.
	Debounce time.Duration
	// Logger is the bridge's logger; nil is slog.Default().
	Logger *slog.Logger
	// OnChange, when set, is called on the worker after a sync that
	// changed the exposure (the entity ids whose decision moved) or the
	// endpoints (added, replaced or removed devices).
	OnChange func(exposure []string, endpoints bool)
	// CameraUpstream is Home Assistant's side of the camera relay
	// (camera.HA of the client). Without it a camera has no host ports
	// and go-hamapping skips its row.
	CameraUpstream camera.Upstream
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
	*plan.Group
	plan     *hamapping.Plan
	verdicts []hamapping.Verdict
	devices  []*device.Device
	keys     []device.SourceKey
	// cameras are the entities whose camera host the build attached.
	cameras map[string]bool
}

// built is g as plan.Diff reads it: its signature and its endpoints with
// their stable keys.
func (g *group) built() *plan.Built[*device.Device] {
	return &plan.Built[*device.Device]{Key: g.Key, Sig: g.Sig, Keys: g.keys, Endpoints: g.devices}
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
	// hosts are the camera hosts by entity id: made when go-hamapping
	// asks for an entity's ports, closed when no built device of the
	// entity carries them any more.
	hosts map[string]*camera.Host

	mu       sync.Mutex
	states   map[string]*ha.State
	verdicts []plan.Verdict
	last     *ha.Snapshot
	all      map[string]*plan.Verdict
	status   Status
	warnings []Warning
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
			for id, h := range b.hosts {
				h.Close(ctx)
				delete(b.hosts, id)
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
	if k := plan.RemovedKey(snap, entityID); k != nil {
		b.forget[entityID] = k
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
func (b *Bridge) Plan() []plan.Verdict {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.verdicts)
}

// logCameraOff logs, once per start, the exposed cameras the camera
// option left out.
func (b *Bridge) logCameraOff(verdicts []plan.Verdict) {
	if b.cameraLogged {
		return
	}
	n := 0
	for i := range verdicts {
		if verdicts[i].Reason == plan.CameraOff {
			n++
		}
	}
	if n > 0 {
		b.cameraLogged = true
		b.log.Info("bridge.camera_disabled", slog.Int("skipped", n), slog.String("enable", "CAMERA=true or the add-on option camera"))
	}
}

// sync reads everything and brings the Aggregator's set to it in one
// batch: new devices added, changed ones replaced, vanished ones removed.
func (b *Bridge) sync(ctx context.Context) {
	if !b.up.Connected() {
		return
	}
	in := b.opts.Input()
	snap, err := ha.ReadSnapshot(ctx, b.up, in.Rule.Assist)
	if err != nil {
		b.log.Error("bridge.sync_failed", slog.String("error", err.Error()))
		b.setStatus(func(s *Status) { s.SyncError = err.Error() })
		return
	}
	b.unit = snap.Config.TemperatureUnit()
	verdicts, plannedGroups := plan.Assess(snap, in)
	// The candidates map with the input's dry ports: the live hosts are
	// made for the devices the bridge builds, not for every camera.
	all := plan.Candidates(snap, in)
	b.logCameraOff(verdicts)
	warnings, uiWarnings := b.warn(plan.Warnings(in, plan.Labels(snap)))

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

	// Keep or build, by signature; plan.Diff makes the batch of the
	// result: an entity removed from the registry is forgotten instead
	// of removed, one back in the plan already (re-added within the
	// debounce) keeps its endpoint.
	served := make(map[string]*plan.Built[*device.Device], len(b.groups))
	for k, g := range b.groups {
		served[k] = g.built()
	}
	next := map[string]*group{}
	planned := make([]*plan.Built[*device.Device], 0, len(plannedGroups))
	for _, p := range plannedGroups {
		if old := b.groups[p.Key]; old != nil && old.Sig == p.Sig {
			next[p.Key] = old
			planned = append(planned, served[p.Key])
			plan.MergeVerdicts(verdicts, old.verdicts)
			continue
		}
		g, mv := b.build(ctx, p)
		plan.MergeVerdicts(verdicts, mv)
		next[p.Key] = g
		planned = append(planned, g.built())
	}
	ch := plan.Diff(served, planned, b.forget)
	b.forget = map[string]device.SourceKey{}
	err = b.n.Aggregator().Batch(ctx, func(bt *node.Batch) error {
		errs := make([]error, 0, len(ch.Ops))
		for _, op := range ch.Ops {
			switch op.Kind {
			case plan.Remove:
				errs = append(errs, bt.Remove(op.Key))
			case plan.Forget:
				errs = append(errs, bt.Forget(op.Key))
			case plan.Replace:
				errs = append(errs, bt.Replace(op.Key, op.Endpoint))
			case plan.Add:
				errs = append(errs, bt.Add(op.Endpoint))
			}
		}
		return errors.Join(errs...)
	})
	if err != nil {
		b.log.Error("bridge.batch", slog.String("error", err.Error()))
	}
	for _, k := range slices.Sorted(maps.Keys(ch.Forgotten)) {
		b.log.Info("bridge.forget", slog.String("entity", ch.Forgotten[k]), slog.String("key", k))
	}
	for _, k := range slices.Concat(ch.Replaced, ch.Removed) {
		b.groups[k].plan.Close()
	}
	b.groups = next
	b.closeCameras(ctx)
	b.owner = map[string]*group{}
	for _, g := range next {
		for _, e := range g.Entities {
			b.owner[e.Key()] = g
		}
	}
	// Every state onto its device: values and reachability.
	endpoints := 0
	for _, g := range next {
		endpoints += len(g.devices)
		for _, e := range g.Entities {
			b.apply(ctx, g, e.Key())
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
		slog.Int("added", len(ch.Added)), slog.Int("replaced", len(ch.Replaced)), slog.Int("removed", len(ch.Removed)))
	changed := len(ch.Added)+len(ch.Replaced)+len(ch.Removed) > 0
	if b.opts.OnChange != nil && (len(moved) > 0 || changed) {
		b.opts.OnChange(moved, changed)
	}
}

// Warning is a configuration finding of the last sync and when an
// unbroken run of syncs first saw it.
type Warning struct {
	Message string
	Since   time.Time
}

// View is what the last sync left for readers: the web UI's exposure
// provider (internal/exposure) reads it.
type View struct {
	// Snapshot is what the sync read; nil before the first sync.
	Snapshot *ha.Snapshot
	// Verdicts are the candidates' verdicts, as [Bridge.Plan] returns them.
	Verdicts []plan.Verdict
	// All are the sync's plan.Candidates: each entity's verdict as if the
	// rule exposed it.
	All map[string]*plan.Verdict
	// Warnings are the sync's findings (plan.Warnings) with their age.
	Warnings []Warning
}

// View returns the last sync's read model.
func (b *Bridge) View() View {
	b.mu.Lock()
	defer b.mu.Unlock()
	return View{Snapshot: b.last, Verdicts: slices.Clone(b.verdicts), All: b.all, Warnings: slices.Clone(b.warnings)}
}

// warn logs the configuration's warnings (plan.Warnings) when they
// change. The second result is the same warnings, each with the time it
// was first seen in an unbroken run of syncs.
func (b *Bridge) warn(found []string) ([]string, []Warning) {
	out := slices.Clone(found)
	ui := make([]Warning, 0, len(out))
	since := map[string]time.Time{}
	now := time.Now()
	for _, msg := range out {
		at, ok := b.since[msg]
		if !ok {
			at = now
		}
		since[msg] = at
		ui = append(ui, Warning{Message: msg, Since: at})
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
func (b *Bridge) build(ctx context.Context, p *plan.Group) (*group, []hamapping.Verdict) {
	hp, mv := hamapping.Map(p.Device, p.Entities, plan.MapOptions(p.Regs, p.Names, b.ports))
	g := &group{Group: p, plan: hp}
	devs, err := hp.Build(ctx)
	if err != nil {
		b.log.Warn("bridge.build_failed", slog.String("device", p.Device.Name.Default), slog.String("error", err.Error()))
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
	for i := range hp.Endpoints {
		g.keys = append(g.keys, hp.Endpoints[i].Options.StableKey)
	}
	b.attachCameras(g, hp, devs)
	hp.OnChange(b.onCommand)
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
	if g.Descs[sc.EntityID] != g.Describe(sc.EntityID, sc.NewState, b.unit) {
		b.resyncSoon()
	}
	b.apply(ctx, g, sc.EntityID)
}
