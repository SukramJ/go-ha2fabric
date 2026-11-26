// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

// Package bridge is the reconciler: it brings one go-fabric node to what
// internal/plan decides from Home Assistant. Its worker reads a snapshot
// (internal/ha) at every connection and after registry changes, assesses
// it with the plan's input, builds each changed device group through
// go-hamapping, applies plan.Diff in one aggregator batch, and carries
// every state onto its device; a controller's command comes back through
// go-hamapping's OnChange and leaves as a call_service. What the last sync
// left (View) is the read model the web UI's provider shows
// (internal/exposure).
//
// It follows go-fabric's cookbook "Embed in a daemon"
// (docs/guides/embed-in-a-daemon.md at v0.27.0): one aggregator batch for
// the whole exposed set at start and on every registry change (§4),
// states to attributes through the plan's typed Sets (§5), reachability
// from each entity's availability and from the Home Assistant connection
// (§6). The node's configuration (§2) is internal/config's, the pairing
// codes in the log (§7) are the composition root's.
//
// The bridge holds no mapping and no exposure rule: go-hamapping's Map
// decides every device type and cluster, internal/plan which entities it
// gets; the bridge imports go-fabric's facade only, and neither the web
// UI nor the configuration (internal/arch).
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

	hamapping "sukramj.dev/go-hamapping"
	"sukramj.dev/go-hamqtt/model"
	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/eligibility"
	"thingfabric.dev/go-fabric/node"

	"thingfabric.dev/go-ha2fabric/internal/adapt"
	"thingfabric.dev/go-ha2fabric/internal/camera"
	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/plan"
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
	// second. A stream of events closer than it puts the sync off by at
	// most maxWait debounces from the first ([Bridge.resyncSoon]).
	Debounce time.Duration
	// Logger is the bridge's logger; nil is slog.Default().
	Logger *slog.Logger
	// OnChange, when set, is called on the worker after a sync that
	// changed the exposure (the entity ids whose decision moved) or the
	// endpoints (added, replaced or removed devices).
	OnChange func(exposure []string, endpoints bool)
	// Retry is the first wait before a sync that failed while Home
	// Assistant stays connected is tried again; it doubles with each
	// failure up to maxRetry and starts again after a sync that succeeded
	// (HR-P2-err-61). 0 is five seconds.
	Retry time.Duration
	// Snapshots is the connection's shared snapshot reader
	// (ha.Fanout.Snapshots), so the nodes on one connection read Home
	// Assistant once per event, not once each (HR-P2-bound-62); nil reads
	// through the upstream alone.
	Snapshots *ha.Snapshots
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
	// Discards counts the controller commands a row answered with success
	// and the entity had no action for (bridge.command_discarded).
	Discards uint64 `json:"discards"`
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
	// stale are the entities whose endpoint's Replace go-fabric refused
	// (decision 73, HR-P4-err-40): the node still serves the device of an
	// earlier group under the key, so that group's plan stays live for
	// them — states, reachability, commands and the read model's endpoint
	// — until a later batch applies the Replace. Each maps to that group.
	stale map[string]*group
	// served are the devices the node serves under the refused keys, for
	// unreachable.
	served []*device.Device
}

// live is the group whose plan serves entity id: g, or the earlier group
// whose device the node still serves after a refused Replace.
func (g *group) live(id string) *group {
	if s := g.stale[id]; s != nil {
		return s
	}
	return g
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
	n     *node.Node
	up    Upstream
	snaps *ha.Snapshots
	opts  Options
	log   *slog.Logger

	work chan func(context.Context)
	cmds chan model.Command

	connected atomic.Bool
	// owed is a sync the worker owes: work that did not fit the queue
	// (an event, a Connected, a Disconnected, the debounce's sync). The
	// worker runs it once the queue is drained, so the snapshot it reads
	// is newer than every closure applied before it; a sync reads
	// everything and marks every endpoint unreachable while Home
	// Assistant is away, so nothing a dropped closure carried is lost.
	owed atomic.Bool
	// drops are the logs of the work and the command queue's dropped
	// items ([dropLog]).
	drops struct{ work, cmds *dropLog }
	// wake tells a waiting worker that a sync is owed: the failed send and
	// the mark are two steps, so the worker can drain the queue between
	// them and would otherwise wait with the sync owed (HR-P1-conc-96).
	wake chan struct{}

	// worker-owned
	groups map[string]*group
	owner  map[string]*group
	known  map[string]bool
	// oversized holds the entities whose state_changed over
	// ha.MaxStateBytes was logged at Warn (HR-P2-bound-38).
	oversized map[string]bool
	// hollow holds the exposed entities the last sync left unmappable
	// for a state over ha.MaxStateBytes: a state under the cap re-syncs
	// (HR-P2-bound-90, decision 52).
	hollow map[string]bool
	unit   string
	warned string
	since  map[string]time.Time
	// cameraLogged is set once the skipped cameras were logged.
	cameraLogged bool
	// hosts are the camera hosts by entity id: made when go-hamapping
	// asks for an entity's ports, closed when no built device of the
	// entity carries them any more.
	hosts map[string]*camera.Host
	// closing are the camera hosts' closes in flight ([Bridge.closeCameras]).
	closing sync.WaitGroup
	// labelIDs are the label ids the rule's label values resolved to at
	// the last sync (expose.Rule.LabelIDs): a renamed label keeps its
	// inclusion or exclusion.
	labelIDs map[string]string
	// down is set once the endpoints were marked unreachable for an
	// outage of Home Assistant, so the outage is logged once.
	down bool
	// retry is the wait before the next retry of a failed sync, 0 after a
	// sync that succeeded.
	retry time.Duration

	mu sync.Mutex
	// states are the served entities' current states: the snapshot's,
	// then each state_changed's.
	states   map[string]*ha.State
	verdicts []plan.Verdict
	last     *ha.Snapshot
	all      map[string]*plan.Verdict
	status   Status
	warnings []Warning
	timer    *time.Timer
	// first is the first request the pending timer serves, zero while
	// none is pending, and latest its latest: the sync's read begins after
	// it. armed counts the timers armed, so a timer that fired while it
	// was replaced does nothing ([Bridge.fire]).
	first  time.Time
	latest time.Time
	armed  uint64
	synced chan struct{}
	// forget is the stable key of every entity removed from Home
	// Assistant's registry since the last sync, by entity id: the next
	// sync forgets it instead of removing it ([Bridge.noteRemoved]).
	// Recorded under mu at the event, never through the droppable queue.
	forget map[string]device.SourceKey
	// removals counts the removals noted; removedAt is each forget's
	// count. A sync whose read began before a removal may still plan the
	// entity, so that forget is kept for the next sync (HR-P1-conc-93).
	removals  uint64
	removedAt map[string]uint64
	// serving are the entity ids the last sync serves on an endpoint: a
	// queued command runs only for one of them ([Bridge.command]).
	serving map[string]bool
	// viewLabelIDs is labelIDs for readers ([View]).
	viewLabelIDs map[string]string
	// failures counts the syncs that failed in a row while Home Assistant
	// was connected, failure is the last one's error and failedSince the
	// first one's time: from the second, a warning ([View.SyncFailure]).
	failures    int
	failure     string
	failedSince time.Time
	// ranges are the heat/cool ranges the command loop last sent, by
	// entity id, until a state shows them ([Bridge.sentRangeLocked]).
	ranges map[string]sentRange
}

// New returns the bridge of node n and upstream up. The node may be
// started or not; [Bridge.Run] serves it.
func New(n *node.Node, up Upstream, opts Options) *Bridge {
	if opts.Debounce <= 0 {
		opts.Debounce = time.Second
	}
	if opts.Retry <= 0 {
		opts.Retry = 5 * time.Second
	}
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	snaps := opts.Snapshots
	if snaps == nil {
		snaps = ha.NewSnapshots(up)
	}
	b := &Bridge{
		n: n, up: up, snaps: snaps, opts: opts, log: log,
		work:   make(chan func(context.Context), 1024),
		wake:   make(chan struct{}, 1),
		cmds:   make(chan model.Command, 256),
		groups: map[string]*group{}, owner: map[string]*group{}, known: map[string]bool{},
		forget: map[string]device.SourceKey{}, removedAt: map[string]uint64{}, ranges: map[string]sentRange{},
		states: map[string]*ha.State{},
		synced: make(chan struct{}),
	}
	b.drops.work = &dropLog{log: log, msg: "bridge.queue_full", every: dropEvery, hint: "Home Assistant sends faster than the bridge applies; a re-sync follows once the queue is drained"}
	b.drops.cmds = &dropLog{log: log, msg: "bridge.command_dropped", every: dropEvery, hint: "the command queue is full; the device shows Home Assistant's state again"}
	return b
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
			b.closePlans(nil)
			b.closeAll(ctx)
			return nil
		case fn := <-b.work:
			fn(ctx)
			if len(b.work) == 0 && b.owed.Swap(false) {
				_ = b.sync(ctx, time.Now())
			}
		case <-b.wake:
			if len(b.work) == 0 && b.owed.Swap(false) {
				_ = b.sync(ctx, time.Now())
			}
		}
	}
}

// enqueue hands work to the worker; it never blocks the caller (an
// ha.Handler method on the client's goroutine). Work that does not fit
// is dropped and a sync is owed instead ([Bridge.owed]): only work a sync
// covers goes through the queue.
func (b *Bridge) enqueue(fn func(context.Context)) {
	select {
	case b.work <- fn:
	default:
		b.owe()
		b.drops.work.drop()
	}
}

// owe marks a sync owed and wakes the worker if it waits; a worker busy
// with the queue finds the mark after its next function.
func (b *Bridge) owe() {
	b.owed.Store(true)
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Synced is closed after the first sync finished.
func (b *Bridge) Synced() <-chan struct{} { return b.synced }

// Connected implements ha.Handler: a (re)connection reads everything
// again.
func (b *Bridge) Connected(string) {
	b.connected.Store(true)
	b.setStatus(func(s *Status) { s.HAConnected = true })
	since := time.Now()
	b.enqueue(func(ctx context.Context) { _ = b.sync(ctx, since) })
}

// Disconnected implements ha.Handler: while Home Assistant is away every
// endpoint is unreachable (cookbook §6, "Upstream down"). The marking is
// the sync's while Home Assistant is not connected, so a dropped closure
// leaves it owed, not lost.
func (b *Bridge) Disconnected(error) {
	if !b.connected.Swap(false) {
		return
	}
	b.setStatus(func(s *Status) { s.HAConnected = false })
	b.enqueue(func(ctx context.Context) { _ = b.sync(ctx, time.Now()) })
}

// unreachable marks every served endpoint unreachable, and logs the
// outage once.
func (b *Bridge) unreachable(ctx context.Context) {
	for _, g := range b.groups {
		for i, d := range g.devices {
			// A group's devices are top-level, never parts, so go-fabric's
			// ErrNotBridged cannot come; any error stays a Debug line, as a
			// reachability patch's does (apply), with the endpoint's
			// stable key (g.keys runs beside g.devices).
			if err := d.SetReachable(ctx, false); err != nil {
				key := ""
				if i < len(g.keys) {
					key = g.keys[i].String()
				}
				b.log.Debug("bridge.unreachable", slog.String("key", key), slog.String("error", err.Error()))
			}
		}
		for _, d := range g.served {
			if err := d.SetReachable(ctx, false); err != nil {
				b.log.Debug("bridge.unreachable", slog.String("error", err.Error()))
			}
		}
	}
	if !b.down {
		b.down = true
		b.log.Warn("bridge.upstream_unusable", slog.String("hint", "every endpoint is unreachable until Home Assistant is back"))
	}
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
			b.noteRemoved(u.EntityID)
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
// entry), for the next sync to forget. It runs on the client's goroutine,
// under mu, so a full queue cannot drop it.
//
// Home Assistant keeps a removed entry's id in deleted_entities and gives
// it back when the entity is created again
// (homeassistant/helpers/entity_registry.py:1728-1775, :1624 at
// 2026.10.3), so without Forget a re-added entity would get its old
// number from go-fabric's store; with it, it gets a fresh one, as a new
// device would (node.Aggregator.Forget).
func (b *Bridge) noteRemoved(entityID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if k := plan.RemovedKey(b.last, entityID); k != nil {
		b.forget[entityID] = k
		b.removals++
		b.removedAt[entityID] = b.removals
	}
}

// keepStaleForgets records again, for the next sync, each forget that
// Diff dropped because the plan still serves its key while the removal
// was noted after this sync's read began: the snapshot may predate the
// removal, so the entity is not known to be back. A forget noted before
// the read is of an entity back in the registry, which keeps its endpoint.
func (b *Bridge) keepStaleForgets(forget map[string]device.SourceKey, removedAt map[string]uint64, readFrom uint64, forgotten map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, k := range forget {
		if _, done := forgotten[k.String()]; done || removedAt[id] <= readFrom {
			continue
		}
		if _, newer := b.forget[id]; !newer {
			b.forget[id], b.removedAt[id] = k, removedAt[id]
		}
	}
}

// maxRetry bounds the back-off of a failed sync's retry.
const maxRetry = 5 * time.Minute

// errNotConnected is a sync's answer while Home Assistant is away.
var errNotConnected = errors.New("bridge: Home Assistant is not connected")

// maxWait is how many debounces a stream of re-sync requests can put a
// sync off, counted from the first request (HR-P2-data-30).
const maxWait = 5

// resyncSoon schedules one sync after the debounce; requests within it
// coalesce, but a stream of them cannot put the sync off without end: it
// runs at the latest maxWait debounces after the first request the timer
// serves (HR-P2-data-30). A full queue when it fires leaves the sync owed.
func (b *Bridge) resyncSoon() {
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.first.IsZero() {
		b.first = now
	}
	b.latest = now
	at := now.Add(b.opts.Debounce)
	if latest := b.first.Add(maxWait * b.opts.Debounce); at.After(latest) {
		at = latest
	}
	b.armLocked(at)
}

// retryLater schedules the retry of a failed sync after its back-off,
// unless a sync is pending already: that one is the retry.
func (b *Bridge) retryLater() {
	if b.retry == 0 {
		b.retry = b.opts.Retry
	}
	wait := b.retry
	b.retry = min(2*b.retry, max(maxRetry, b.opts.Retry))
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.first.IsZero() {
		return
	}
	b.first, b.latest = now, now.Add(wait)
	b.armLocked(b.latest)
}

// armLocked arms the one sync timer to fire at at, in place of a pending
// one. Called with mu held.
func (b *Bridge) armLocked(at time.Time) {
	if b.timer != nil {
		b.timer.Stop()
	}
	b.armed++
	armed := b.armed
	b.timer = time.AfterFunc(time.Until(at), func() { b.fire(armed) })
}

// fire is the sync timer's end: the sync it served goes to the worker.
// A timer replaced after it fired does nothing; the one that replaced it
// serves its requests.
func (b *Bridge) fire(armed uint64) {
	b.mu.Lock()
	if armed != b.armed {
		b.mu.Unlock()
		return
	}
	b.first = time.Time{}
	since := b.latest
	b.mu.Unlock()
	select {
	case b.work <- func(ctx context.Context) { _ = b.sync(ctx, since) }:
	default:
		b.owe()
	}
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
// While Home Assistant is not connected it marks every endpoint
// unreachable instead and returns errNotConnected. A read or a batch that
// fails while Home Assistant stays connected is tried again after a
// back-off (HR-P2-err-61); the error is the sync's. Its snapshot is a
// read that began at or after since, the time of what the sync is for
// ([ha.Snapshots]).
func (b *Bridge) sync(ctx context.Context, since time.Time) error {
	if !b.up.Connected() {
		b.unreachable(ctx)
		return errNotConnected
	}
	err := b.syncConnected(ctx, since)
	switch {
	case err == nil:
		b.retry = 0
		b.mu.Lock()
		b.failures, b.failure, b.failedSince = 0, "", time.Time{}
		b.mu.Unlock()
	case ctx.Err() == nil:
		b.mu.Lock()
		if b.failures == 0 {
			b.failedSince = time.Now()
		}
		b.failures++
		b.failure = err.Error()
		b.mu.Unlock()
		if b.up.Connected() {
			b.retryLater()
		}
	}
	return err
}

// syncConnected is the sync while Home Assistant is connected.
func (b *Bridge) syncConnected(ctx context.Context, since time.Time) error {
	in := b.opts.Input()
	in.Rule.LabelIDs = b.labelIDs
	b.mu.Lock()
	readFrom := b.removals
	b.mu.Unlock()
	snap, err := b.snaps.Read(ctx, since, in.Rule.Assist)
	if err == nil {
		err = plan.Check(snap, in)
	}
	if err != nil {
		if ctx.Err() != nil {
			// A stop, an erase or a delete ended the worker: no failure
			// (HR-P2-err-67).
			b.log.Debug("bridge.sync_cancelled", slog.String("error", err.Error()))
			return err
		}
		b.log.Error("bridge.sync_failed", slog.String("error", err.Error()))
		b.setStatus(func(s *Status) { s.SyncError = err.Error() })
		return err
	}
	b.unit = snap.Config.TemperatureUnit()
	verdicts, plannedGroups := plan.Assess(snap, in)
	// The candidates map with the input's dry ports: the live hosts are
	// made for the devices the bridge builds, not for every camera.
	all := plan.Candidates(snap, in)
	b.logCameraOff(verdicts)
	labels := plan.Labels(snap)
	warnings, uiWarnings := b.warn(plan.Warnings(in, labels))
	b.labelIDs = in.Rule.Resolutions(labels)

	b.known = map[string]bool{}
	exposed := map[string]bool{}
	for i := range verdicts {
		b.known[verdicts[i].EntityID] = true
		exposed[verdicts[i].EntityID] = verdicts[i].Exposed
	}
	b.hollow = map[string]bool{}
	for i := range snap.States {
		if st := &snap.States[i]; st.Oversized > 0 && exposed[st.EntityID] {
			b.hollow[st.EntityID] = true
		}
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
	b.mu.Lock()
	forget, removedAt := b.forget, b.removedAt
	b.forget, b.removedAt = map[string]device.SourceKey{}, map[string]uint64{}
	b.mu.Unlock()
	ch := plan.Diff(served, planned, forget)
	b.keepStaleForgets(forget, removedAt, readFrom, ch.Forgotten)
	// A batch is not a transaction (node.Partition.Batch): each operation
	// applies or fails on its own. Since go-fabric v0.26.0 a call the
	// batch holds (a key another endpoint serves at the call) answers nil
	// and is judged at the commit, its refusal joined into Batch's error
	// without saying which call it was; so what applied is read from the
	// node's set after the batch (settle), and the calls' errors, kept by
	// index, are for the log and a Forget's own failure.
	opErrs := make([]error, len(ch.Ops))
	ran := false
	err = b.n.Aggregator().Batch(ctx, func(bt *node.Batch) error {
		ran = true
		for i, op := range ch.Ops {
			var e error
			switch op.Kind {
			case plan.Remove:
				e = bt.Remove(op.Key)
			case plan.Forget:
				// A Forget whose reservation go-fabric cannot read answers
				// nil here and erases nothing for the key; the commit joins
				// "the erasure of … is skipped" into Batch's error
				// (go-fabric node/reserve.go:615-631 at v0.27.0). settle
				// then takes it as applied, so the entity, created again,
				// gets its old number. Typing that error so it can be
				// recorded again is go-fabric #616's scope.
				e = bt.Forget(op.Key)
			case plan.Replace:
				e = bt.Replace(op.Key, op.Endpoint)
			case plan.Add:
				e = bt.Add(op.Endpoint)
			}
			if e != nil {
				opErrs[i] = fmt.Errorf("%s %s: %w", opName(op.Kind), op.Key, e)
			}
		}
		return errors.Join(opErrs...)
	})
	if err != nil {
		b.log.Error("bridge.batch", slog.String("error", err.Error()))
	}
	if !ran {
		// Refused before fn ran (a stopped node, node.ErrStopped): no
		// call was made, so every operation failed with the batch.
		for i := range opErrs {
			opErrs[i] = err
		}
	}
	b.settle(ch.Ops, opErrs, next)
	for _, k := range slices.Sorted(maps.Keys(ch.Forgotten)) {
		b.log.Info("bridge.forget", slog.String("entity", ch.Forgotten[k]), slog.String("key", k))
	}
	b.closePlans(next)
	b.groups = next
	b.closeCameras(ctx)
	b.owner = map[string]*group{}
	for _, g := range next {
		for _, e := range g.Entities {
			b.owner[e.Key()] = g
		}
	}
	// The states of the entities the bridge serves, pointing into the
	// snapshot: every other state is the snapshot's alone (HR-P2-bound-38).
	b.mu.Lock()
	b.states = map[string]*ha.State{}
	for i := range snap.States {
		if id := snap.States[i].EntityID; b.owner[id] != nil {
			b.states[id] = &snap.States[i]
			b.settleRangeLocked(id, &snap.States[i])
		}
	}
	b.mu.Unlock()
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
			if d := g.live(verdicts[i].EntityID).plan.DeviceFor(verdicts[i].EntityID); d != nil {
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
	// Served is what the plan built: an entity go-fabric refused stays in
	// its group without an endpoint (HR-P2-data-35, HR-P2-data-91).
	serving := map[string]bool{}
	for _, g := range next {
		if len(g.devices) == 0 || g.plan == nil {
			continue
		}
		for _, e := range g.Entities {
			if g.live(e.Key()).plan.DeviceFor(e.Key()) != nil {
				serving[e.Key()] = true
			}
		}
	}
	b.mu.Lock()
	b.serving = serving
	b.viewLabelIDs = b.labelIDs
	b.verdicts = verdicts
	b.last = snap
	b.all = all
	b.status.Warnings = warnings
	b.warnings = uiWarnings
	b.down = false
	b.status.LastSync = time.Now()
	b.status.SyncError = ""
	if err != nil {
		b.status.SyncError = err.Error()
	}
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
	return err
}

// settle takes back what the batch did not apply (HR-P1-err-31), so the
// next sync tries it again. What applied is the node's set after the
// batch, not the calls' answers: go-fabric (since v0.26.0) holds a call whose key
// another endpoint serves, answers it nil and may refuse it at the
// commit (node.Partition.Batch). An Add or a Replace applied when the set
// serves its very endpoint under its key; a refused Add's key is not
// served, so it leaves its group's keys and endpoints; a group with a
// refused Add or Replace keeps no signature, so the next sync rebuilds it
// and plan.Diff adds or replaces its keys again; until then a refused
// Replace's entities stay on the plan whose device the node still serves
// (keepStale, decision 73). A Forget applied unless
// its call failed or an endpoint still serves its key, itself or as a
// part (the node's refusal); a Forget not applied is recorded again for
// the next sync. A failed Remove needs nothing: the node refuses it for a
// key it does not serve.
//
// The set is the default partition's: go-fabric judges a key against
// every partition, but this daemon creates no other (each node of the
// registry is a node of its own, cmd/ha2fabric/nodes.go). go-fabric lets
// a Forget pass whose key a device the same batch added serves; that
// cannot come from this bridge, since plan.Diff drops a forget whose key
// a planned group serves and go-hamapping's part keys (<key>#n) are no
// entity's key, so every endpoint that serves the key is the refusal.
func (b *Bridge) settle(ops []plan.Op[*device.Device], errs []error, next map[string]*group) {
	set := b.n.Aggregator().Endpoints()
	byKey := make(map[string]node.Endpoint, len(set))
	for _, ep := range set {
		if k := ep.Spec().StableKey; k != nil {
			byKey[k.String()] = ep
		}
	}
	// By the very device: an endpoint of a type that is not comparable
	// (another host's) is never compared with ==.
	applied := func(op plan.Op[*device.Device]) bool {
		d, ok := byKey[op.Key.String()].(*device.Device)
		return ok && d != nil && d == op.Endpoint
	}
	// The keys the set serves: a Forget of one of them is the node's
	// refusal.
	served := map[string]bool{}
	for _, ep := range set {
		servedKeys(ep, served)
	}
	refused := map[string]plan.OpKind{}
	for i, op := range ops {
		switch op.Kind {
		case plan.Add, plan.Replace:
			if !applied(op) {
				refused[op.Key.String()] = op.Kind
			}
		case plan.Forget:
			if errs[i] == nil && !served[op.Key.String()] {
				continue
			}
			b.mu.Lock()
			if _, newer := b.forget[op.Entity]; !newer {
				b.forget[op.Entity] = op.Key
			}
			b.mu.Unlock()
		case plan.Remove:
		}
	}
	if len(refused) == 0 {
		return
	}
	for _, g := range next {
		failed := false
		keys, devs := g.keys[:0:0], g.devices[:0:0]
		for i, k := range g.keys {
			kind, ok := refused[k.String()]
			failed = failed || ok
			if kind == plan.Add {
				continue
			}
			keys, devs = append(keys, k), append(devs, g.devices[i])
		}
		if !failed {
			continue
		}
		cp := *g.Group
		cp.Sig = ""
		g.Group, g.keys, g.devices = &cp, keys, devs
		for _, k := range keys {
			if refused[k.String()] == plan.Replace {
				b.keepStale(g, k.String(), byKey[k.String()])
			}
		}
	}
}

// keepStale keeps the plan live that serves the endpoint the node still
// serves under key, whose Replace by g's device it refused (decision
// 73): the earlier group of g's device, or the group it kept stale
// after an earlier refusal. Its entities on that endpoint — the
// endpoint's own and its riders — go to that plan, and a camera's host
// back to the served device's provider.
func (b *Bridge) keepStale(g *group, key string, ep node.Endpoint) {
	d, ok := ep.(*device.Device)
	old := b.groups[g.Key]
	if !ok || d == nil || old == nil {
		return
	}
	for _, c := range append([]*group{old}, slices.Collect(maps.Values(old.stale))...) {
		if !slices.Contains(c.devices, d) {
			continue
		}
		for i := range c.plan.Endpoints {
			ep := &c.plan.Endpoints[i]
			if ep.Options.StableKey == nil || ep.Options.StableKey.String() != key {
				continue
			}
			if g.stale == nil {
				g.stale = map[string]*group{}
			}
			for _, id := range append([]string{ep.Key}, ep.Riders...) {
				g.stale[id] = c
				if h := b.hosts[id]; h != nil && c.cameras[id] {
					if err := h.AttachDevice(d); err != nil {
						b.log.Error("bridge.camera_unattached", slog.String("entity", id), slog.String("error", err.Error()))
					}
				}
			}
			g.served = append(g.served, d)
			b.log.Warn("bridge.replace_refused", slog.String("key", key), slog.String("entity", ep.Key),
				slog.String("hint", "the endpoint served stays live on its earlier plan until a sync replaces it"))
			return
		}
	}
}

// closePlans closes the plans of the groups in b.groups, and of the
// groups they keep stale, that no group of next uses any more, as its
// own or kept stale.
func (b *Bridge) closePlans(next map[string]*group) {
	used := map[*hamapping.Plan]bool{}
	for _, g := range next {
		used[g.plan] = true
		for _, s := range g.stale {
			used[s.plan] = true
		}
	}
	for _, g := range b.groups {
		for _, c := range append([]*group{g}, slices.Collect(maps.Values(g.stale))...) {
			if c.plan != nil && !used[c.plan] {
				used[c.plan] = true
				c.plan.Close()
			}
		}
	}
}

// servedKeys adds the key ep serves and each of its parts' keys, at any
// depth, to keys.
func servedKeys(ep node.Endpoint, keys map[string]bool) {
	spec := ep.Spec()
	stack := spec.Parts[:0:0]
	stack = append(stack, spec)
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if s.StableKey != nil {
			keys[s.StableKey.String()] = true
		}
		stack = append(stack, s.Parts...)
	}
}

// opName is a batch operation's name in an error.
func opName(k plan.OpKind) string {
	switch k {
	case plan.Remove:
		return "remove"
	case plan.Forget:
		return "forget"
	case plan.Replace:
		return "replace"
	default:
		return "add"
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
	// LabelIDs are the label ids the rule's label values resolved to
	// (expose.Rule.LabelIDs), for a reader's own assessment.
	LabelIDs map[string]string
	// SyncFailure, when set, is a sync that keeps failing while Home
	// Assistant is connected, whatever the cause: the last error, since
	// the first failure of the run. Nil after a sync that succeeded and
	// while only one sync failed (its retry may still succeed).
	SyncFailure *Warning
}

// View returns the last sync's read model.
func (b *Bridge) View() View {
	b.mu.Lock()
	defer b.mu.Unlock()
	v := View{Snapshot: b.last, Verdicts: slices.Clone(b.verdicts), All: b.all, Warnings: slices.Clone(b.warnings), LabelIDs: maps.Clone(b.viewLabelIDs)}
	if b.failures >= 2 {
		v.SyncFailure = &Warning{Message: b.failure, Since: b.failedSince}
	}
	return v
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
// returns when that sync has run: the web UI's reassemble. Its error is
// the sync's, errNotConnected when Home Assistant went away before the
// sync ran (HR-P2-err-65).
func (b *Bridge) Resync(ctx context.Context) error {
	if !b.up.Connected() {
		return errNotConnected
	}
	done := make(chan struct{})
	var err error
	since := time.Now()
	select {
	case b.work <- func(ctx context.Context) {
		defer close(done)
		err = b.sync(ctx, since)
	}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if errors.Is(err, errNotConnected) {
		return err
	}
	if err != nil {
		return fmt.Errorf("bridge: %w", err)
	}
	return nil
}

// build maps and builds one device's plan. An endpoint go-fabric refuses
// (go-hamapping's *BuildError) costs only its entity and the entities
// riding on it: they are unmappable with the refusal as their reason, and
// the device is mapped and built again without them, at most once per
// entity (HR-P2-data-35, decision 40). Any other build failure leaves the
// device without endpoints and says why in its verdicts.
func (b *Bridge) build(ctx context.Context, p *plan.Group) (*group, []hamapping.Verdict) {
	entities := p.Entities
	var refused []hamapping.Verdict
	for {
		hp, mv := hamapping.Map(p.Device, entities, plan.MapOptions(p.Regs, p.Names, b.ports))
		g := &group{Group: p, plan: hp}
		devs, err := hp.Build(ctx)
		if err == nil {
			mv = append(mv, refused...)
			g.verdicts = mv
			g.devices = devs
			for i := range hp.Endpoints {
				g.keys = append(g.keys, hp.Endpoints[i].Options.StableKey)
			}
			b.attachCameras(g, hp, devs)
			hp.OnChange(b.onCommand)
			hp.OnDiscard(b.onDiscard)
			return g, mv
		}
		reason := "go-fabric refused the device: " + err.Error()
		var be *hamapping.BuildError
		if errors.As(err, &be) {
			drop := map[string]bool{be.Key: true}
			for _, k := range be.Riders {
				drop[k] = true
			}
			rest := slices.DeleteFunc(slices.Clone(entities), func(e model.Entity) bool { return drop[e.Key()] })
			if len(rest) < len(entities) {
				b.log.Warn("bridge.build_refused", slog.String("device", p.Device.Name.Default), slog.String("entity", be.Key),
					slog.Any("riders", be.Riders), slog.String("error", err.Error()))
				for i := range mv {
					if drop[mv[i].Key] {
						mv[i].State = eligibility.StateUnmappable
						mv[i].Reason = reason
						refused = append(refused, mv[i])
					}
				}
				hp.Close()
				entities = rest
				continue
			}
		}
		b.log.Warn("bridge.build_failed", slog.String("device", p.Device.Name.Default), slog.String("error", err.Error()))
		for i := range mv {
			if mv[i].State != eligibility.StateUnmappable {
				mv[i].State = eligibility.StateUnmappable
				mv[i].Reason = reason
			}
		}
		mv = append(mv, refused...)
		g.verdicts = mv
		return g, mv
	}
}

// apply carries one entity's current state onto its device.
func (b *Bridge) apply(ctx context.Context, g *group, id string) {
	g = g.live(id)
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

// logOversized logs a state_changed's state that came over
// ha.MaxStateBytes without its attributes, as the client logs
// get_states': at Warn the first time for the entity, at Debug after.
func (b *Bridge) logOversized(ctx context.Context, st *ha.State) {
	level := slog.LevelDebug
	if !b.oversized[st.EntityID] {
		if b.oversized == nil {
			b.oversized = map[string]bool{}
		}
		b.oversized[st.EntityID] = true
		level = slog.LevelWarn
	}
	b.log.Log(ctx, level, "ha.state_oversized", slog.String("entity_id", st.EntityID), slog.Int("bytes", st.Oversized),
		slog.Int("max", ha.MaxStateBytes), slog.String("hint", "the entity's attributes were dropped"))
}

// stateChanged applies one state_changed: the value onto the device, or a
// re-sync when the entity's description moved (a capability attribute
// appeared), the entity is new, or its state crossed ha.MaxStateBytes
// (an oversized entity is unmappable, HR-P2-bound-90). Only a served
// entity's state is kept (HR-P2-bound-38): the next sync reads every
// other one again.
func (b *Bridge) stateChanged(ctx context.Context, sc ha.StateChanged) {
	if sc.NewState != nil && sc.NewState.Oversized > 0 {
		b.logOversized(ctx, sc.NewState)
	}
	g := b.owner[sc.EntityID]
	b.mu.Lock()
	switch {
	case sc.NewState == nil:
		delete(b.states, sc.EntityID)
	case g != nil:
		b.states[sc.EntityID] = sc.NewState
		b.settleRangeLocked(sc.EntityID, sc.NewState)
	}
	b.mu.Unlock()
	if sc.NewState == nil || !b.known[sc.EntityID] {
		b.resyncSoon()
		return
	}
	if g == nil {
		if b.hollow[sc.EntityID] && sc.NewState.Oversized == 0 {
			b.resyncSoon()
		}
		return
	}
	if sc.NewState.Oversized > 0 {
		// Not applied: its attributes were dropped; the sync takes the
		// endpoint away.
		b.resyncSoon()
		return
	}
	if g.Descs[sc.EntityID] != g.Describe(sc.EntityID, sc.NewState, b.unit) {
		b.resyncSoon()
	}
	b.apply(ctx, g, sc.EntityID)
}
