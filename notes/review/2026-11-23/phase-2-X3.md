# Phase 2, unit X3 — life cycle and flows

Reviewer X3 (Opus). Baseline `2324fbf`; every `file:line` below is at that
commit (read in an archive of it), references at their pins (`gf:` go-fabric
v0.25.0, `hm:` go-hamapping v0.6.0, `ui:` go-fabric-ui v0.13.0). M = read
or measured by a probe; I = inferred. Probes ran in a private archive copy
(`go test -count=1`, and `-race` with `CGO_ENABLED=1`); their texts are in
the appendix. No item is held back: none is an open weakness an untrusted
party exploits (decisions 11 and 19).

## Summary

| ID | Severity | One line |
| --- | --- | --- |
| HR-P2-conc-60 | medium | A camera host's `Close` ends its sessions one after another, each up to two 10 s timeouts, and ignores its ctx; the bridge worker, an erase and the stop wait for it without a bound — decision 29's one deadline does not hold |
| HR-P2-err-61 | low | A sync that fails while Home Assistant stays connected is never tried again; a failed first sync leaves the node without devices until some later event |
| HR-P2-bound-62 | low | Every registry event and every reconnect reads the whole snapshot once per node (up to 33 concurrent `get_states` on one connection) |
| HR-P2-err-63 | low | A controller command dropped at the full queue or refused by `adapt.Service` leaves the device at the controller's value; only a failed `call_service` restores Home Assistant's state |
| HR-P2-bound-64 | low | `bridge.queue_full` (and `bridge.command_dropped`) log one Error line per dropped item: a burst floods stderr and evicts the 1000-record operation ring |
| HR-P2-err-65 | note | `Resync` (the web UI's reassemble) reports success when its sync took the "not connected" path, and when the node is between erase and restart |
| HR-P2-doc-66 | note | An entity removed while the connection is down (or the daemon stopped) is removed, not forgotten: re-created, it gets its old endpoint number; `docs/operations.md` says only "within the debounce" |
| HR-P2-err-67 | note | A stop, erase or delete during a sync logs `bridge.sync_failed` at Error with `context canceled` and sets `SyncError` |
| HR-P2-conc-68 | note | An exposure apply racing a node's Delete can write the deleted node's `selection-<key>.json` again (an orphan file that blocks the key) |
| HR-P2-conc-69 | note | `ProvideOffer` runs Home Assistant's offer and up to two unsubscribes inside go-fabric's per-provider command lock; the host's `MaxSessions` check holds only because of that lock |
| HR-P2-data-70 | note | T15's life-cycle half: hostile text in errors crossing goroutines reaches only slog (quoted) and unread status fields; no unquoted sink |

Counts: **0 high · 1 medium · 4 low · 6 note**.

## Findings

### HR-P2-conc-60 — medium — camera `Close` is serial and unbounded; nothing above it bounds the wait

Flow (shutdown):
1. `cmd/ha2fabric/main.go:195-197` makes the 25 s `sctx` and cancels `rctx`.
2. Each bridge's worker sees `ctx.Done` and closes every camera host in
   turn: `internal/bridge/bridge.go:234-236` (`h.Close(ctx)` with the ended ctx).
3. `internal/camera/camera.go:358-385` `Close`: for each session in turn,
   `h.unsubscribe(ctx, ss)` (`:374`, `:627-634`: `WithoutCancel` + `SignalTimeout`)
   and `sig.End(ectx, …)` (`:376-380`: `WithoutCancel` + `SignalTimeout`);
   default `SignalTimeout` 10 s (`:292-294`). Then `h.wg.Wait()` (`:384`).
4. `sig.End` is go-fabric's `Provider.End` → `signal` → `peers.ConnectPeer`
   and `Invoke` to the controller (gf:`cluster/camera/provider_peer.go:137-176`):
   to a controller that is gone it waits until its ctx ends.
5. `served.close` → `stopRun` waits `<-s.done` without a ctx
   (`cmd/ha2fabric/nodes.go:181-185`, called at `:442`); `fleet.Close` waits
   for every member (`:796-807`), `main` waits for both (`main.go:209-216`).

So one camera holding *k* sessions of controllers that went away costs up
to *k* × 20 s (8 × 20 s = 160 s at `MaxSessions`), and the cameras of a
bridge add up. Such sessions are the normal case decision 31 names ("a
controller that went away never ends its session"; the idle end is 1 h).
The documented "one budget of 25 seconds" (`docs/operations.md:635-645`,
decision 29) then does not hold; the Supervisor kills the add-on at 30 s
(`addon/config.yaml` `timeout: 30`) before the nodes' `Stop`.

The same `Close` runs on two live paths:
- the **bridge worker** during a sync, when a camera leaves the bridge or its
  row changes: `bridge.go:560` → `internal/bridge/camera.go:59-69`. The worker
  applies no state and no command re-apply for the whole time; the queue
  fills and the owed sync follows (I);
- an **erase** holds `s.mu` (write) across `stopRun` (`nodes.go:289-299`),
  so `View`, `Resync`, `Discards`, `Warnings` of that node block, and with
  them `fleet.Warnings`/`fleet.Discards` under `f.mu.RLock`
  (`nodes.go:735-763`): the status and counter endpoints of every node hang
  for the duration (I).

M: the probe `TestP2X3CloseHonoursItsContext` (8 sessions, `SignalTimeout`
200 ms, a signaller that waits for its ctx, `Close` with a 100 ms ctx):
`Close of 8 sessions took 1.604512821s` — exactly 8 × 200 ms. I: the
controller-gone wait of `ConnectPeer` (gf line read, not run).

This is phase 1's HR-P1-err-64 (decision 29) incomplete: the budget bounds
`lockBy` and `stopNode`, not the bridge join.

Fix: (a) `Host.Close` ends its sessions in parallel and bounds each signal
by `min(SignalTimeout, ctx's deadline)` (keeping `WithoutCancel` so an
ended ctx still sends, but with the caller's deadline); (b) `stopRun` takes
the ctx and stops waiting at its end (`select { <-s.done; <-ctx.Done() }`),
logging the overrun; (c) the worker's `closeCameras` hands the closes to a
goroutine joined at `Run`'s end, so a sync never waits on a controller.
Red test: the probe (and a `served.close` variant with a hanging host).

### HR-P2-err-61 — low — a failed sync while connected is not retried

Flow: `Connected` → `bridge.go:283` enqueue sync → `:461` `ReadSnapshot`
fails (any call: `CallTimeout` 30 s on a large `get_states`, an error
answer, a strict `SkippedError` of an entry without an id,
`internal/ha/client.go:714-726`, `:736`) → `:465-468` logs `bridge.sync_failed`,
sets `SyncError`, returns. Nothing schedules another sync: the next one
comes only from a registry event, a state of an unknown entity
(`:841-843`), a reconnect, or the web UI's reassemble.

Consequence: after a failed **first** sync the node serves no device at all
in a quiet installation until one of those happens; after a later failure
a registry change the failed sync was for (a new or excluded entity) stays
unapplied. Compounds HR-P2-bound-62: with many nodes the concurrent
`get_states` make the timeout likelier.

M: probe `TestP2X3FailedSyncIsRetried`: `1 read(s)` after 3 s,
`status "get_states: fake: no snapshot"`.

Fix: on a failed sync while `up.Connected()`, re-arm a retry with its own
back-off (e.g. 5 s doubling to 5 min, reset by a successful sync), through
`resyncSoon`'s timer so it coalesces with events; the status keeps
`SyncError` meanwhile. Red test: the probe.

### HR-P2-bound-62 — low — one snapshot read per node per event

Flow: `client.go:437` `h.Event(ev)` → `internal/ha/fanout.go:69-75` hands each
event to every bridge → `bridge.go:338-349` each bridge `resyncSoon` →
after its own 1 s debounce each runs `sync` → `:461` its own
`ReadSnapshot` (six to seven commands, `internal/ha/snapshot.go:49-85`). The same on
every reconnect (`fanout.go:59-66` → `bridge.go:280-284`). With
`NODE_LIMIT` up to 32 (`internal/config/config.go:56-57`) that is up to 33
concurrent `get_states` and registry lists on the one WebSocket; each
result is decoded into its own copy (`DefaultReadLimit` 64 MiB per frame,
`client.go:58`).

Who triggers it: any entity, device or label registry update and any state
of an entity not in the last snapshot — data of integrations, untrusted by
decision 11 (an integration that churns its registry entries makes every
node re-read everything about once a second).

M: probe `TestP2X3OneEventOneSnapshot`: `one entity_registry_updated read
the snapshot 9 times (once per node)`. I: memory and Home Assistant load
scale with the node count.

Fix: one snapshot reader per connection shared by the bridges — a
single-flight read keyed by a generation (each event or `Connected` bumps
it; a bridge whose debounce fires takes the newest snapshot read after its
trigger). The plan stays per node (`plan.Assess` is pure). Red test: the
probe (at most one read per event and generation).

### HR-P2-err-63 — low — a command that never reaches `call_service` leaves the controller's value

Flow: a controller's command or write changes the cluster's attribute and
reaches the daemon through the plan's `OnChange`
(hm:`plan.go:532-539`: "the change a command made") → `internal/bridge/command.go:39-46`
`onCommand`: when `b.cmds` (256) is full the command is dropped with an
Error line and a counter; → `command.go:86-91`: when `adapt.Service`
refuses the command (`bridge.command_unmapped`) it returns. Neither path
restores Home Assistant's state; only a failed `call_service` does
(`command.go:102-110`, "The controller already saw its change applied").
The device keeps the controller's value until that entity's next
`state_changed` — for a static entity, indefinitely.

M: probe `TestP2X3DroppedCommandRestoresTheState`, both subtests fail
(`no re-apply and no sync owed`; `adapt: command has no service: …`). I:
that the attribute shows the commanded value (hm's contract, not run).

Fix: one helper `restore(id)` that enqueues the `apply` of the current
state, called from all three failure paths (the drop path calls it through
`enqueue`, which already falls back to an owed sync). Red test: the probe.

### HR-P2-bound-64 — low — one Error line per dropped item

Flow: `bridge.go:256-263` `enqueue` logs `bridge.queue_full` at Error for
**every** item that does not fit the 1024 queue; `command.go:43` the same
per dropped command. A burst of state events (Home Assistant's start, an
integration flapping its states — untrusted data) beyond the queue writes
one line per dropped event to stderr and to go-fabric's log ring, which
keeps 1000 records (gf:`logring/logring.go:54-55`); the operation page then
shows only the flood and loses the lines before it (the token, start-up
and pairing lines). I (read; not measured at scale).

Fix: log the overflow once per episode (when `owed` goes from false to
true) with a count of what was dropped when the owed sync runs; the same
for dropped commands (a counter and one line per burst). Red test: flood
the queue (`flood` in `queue_internal_test.go`) and count `queue_full`
records in a capture handler: exactly one.

### HR-P2-err-65 — note — `Resync` reports success without a sync

`bridge.go:759-781`: `Resync` checks `up.Connected()` once, then waits for
its closure; the closure's `sync` re-checks (`:452-455`) and, when the
connection dropped in between, only marks endpoints unreachable without
touching `SyncError`, so `Resync` returns nil and go-fabric-ui answers
`reassembled: true` (ui:`fabricui/handlers_exposure.go:217-229`).
`served.Resync` returns nil while the node is between erase and restart
(`nodes.go:200-207`). The stored selection is applied by the next sync in
both cases, so nothing is lost. I. Fix: the closure reports whether it
planned (`sync` returns a bool or an error), and `Resync` returns
`bridge: Home Assistant is not connected` when it did not.

### HR-P2-doc-66 — note — a removal missed while disconnected is not forgotten

`bridge.go:338-341`, `:364-373`: the forget is recorded only from the
`entity_registry_updated` event with `action: remove`. An entity removed
while the connection is down (or the daemon is stopped) is absent from the
next snapshot and is **removed** (`plan.Diff`, `internal/plan/diff.go:101`), not forgotten; re-created, it gets its old endpoint number from
go-fabric's store. `docs/operations.md:484-489` states that a deleted entity
loses its number, with only the debounce as exception. Either a doc
sentence ("removed while the bridge was not connected: it keeps its
number") or, better, a persisted set of the stable keys served at the last
sync so a key absent from the registry (not merely unexposed) is forgotten.
I.

### HR-P2-err-67 — note — stop-time `bridge.sync_failed` at Error

A stop, an erase (`stopRun`, `nodes.go:292`) or a delete cancels the
worker's ctx; an in-flight sync fails in `ReadSnapshot` with
`context canceled` and logs `bridge.sync_failed` at Error and sets
`SyncError` (`bridge.go:465-468`); an in-flight command logs
`bridge.call_service_failed` (`command.go:102-103`). Noise that reads as
a fault in the operation ring. I. Fix: skip the log and status when
`ctx.Err() != nil`.

### HR-P2-conc-68 — note — apply racing Delete re-creates the selection file

`fleet.Delete` (`nodes.go:676-708`) holds `f.op`, closes the node and
removes its files; `exposure.Provider.Apply` (`internal/exposure/exposure.go:290-328`)
runs under neither `f.op` nor the served node's locks. A request that took
the node's entry before the delete writes `selection-<key>.json` (and
`.bak`) after `RemoveFiles`; the orphan file makes `onDisk`
(`internal/nodes/nodes.go:333-341`) refuse the key for a later node of the same
label (it gets a suffix). Narrow window, no data loss. I. Fix: the
selection store gets a `Close` that makes `Set` fail, called by
`served.close`.

### HR-P2-conc-69 — note — the offer runs inside go-fabric's command lock

go-fabric serialises every WebRtcTransportProvider command of one provider
(gf:`cluster/camera/provider.go:125-131`, `state.go:20-26`). Inside it,
`ProvideOffer` (`internal/camera/camera.go:404-475`) unsubscribes an evicted and a
replaced session (`:428`, `:438`, each up to `SignalTimeout`) and waits for
Home Assistant's offer (`:440`, up to the request's ctx or `CallTimeout`
30 s). A slow Home Assistant therefore holds every other command of that
camera (`EndSession`, `ProvideIceCandidates`) for up to about 50 s (I).
The host's own cap check (`:412-424`) and its insert (`:451-460`) are not
atomic across the unlocked `Subscribe`; that is safe only because go-fabric
serialises — worth a comment, or holding a reservation under `h.mu`. The
eviction's unsubscribe could move to the goroutine that already tells the
victim (`:431-434`). I.

### HR-P2-data-70 — note — T15, life-cycle half

Errors carrying Home Assistant or integration text that cross goroutines:
`call_service` errors (`ha.Error`, `internal/ha/message.go:66`, the integration's
exception text) → `command.go:95-103` → the `bridge.call_service_failed`
line and `Status.LastCommand`; sync errors → `bridge.go:466-467` →
`Status.SyncError` → `Resync`'s error → go-fabric-ui's log line
(ui:`handlers_exposure.go:224-226`); the camera provider's error event →
`camera.go:705-707` `camera.ha_error`; dial and close errors →
`client.go:242` `ha.disconnected`. Every sink is slog's text handler
(`internal/web/operation.go:20`, quoting) or a status field. `Status`
(`bridge.go:84-100`) says it is "for the web UI and the health endpoint",
but only `Discards` and, through `Resync`, `SyncError` are read
(`nodes.go:227`, `:293`); `LastCommand`, `CommandErrs` and `Warnings` have no
reader. No unquoted sink in this unit's scope; the ring's rendering on the
pages is X2's. M (read).

## Phase 1 fixes read adversarially

| Row | Verdict |
| --- | --- |
| HR-P1-conc-32 (+conc-60, conc-96) | Holds: `owe` sets the mark before the wake (`bridge.go:267-274`), the worker runs the owed sync on an empty queue on both branches (`:238-248`); no lost wake found. Residual: the owed sync waits for an empty queue, so under load above the worker's rate it is deferred while drops continue (bounded by the burst); and the per-drop logging is HR-P2-bound-64. `-race` clean. |
| HR-P1-conc-93 | Holds: `readFrom` is taken before the read (`bridge.go:458-460`), the forgets are taken after it (`:518-521`), `keepStaleForgets` re-records only removals newer than the read (`:379-392`), and the event that noted the removal also arms `resyncSoon`, so the next sync exists. Gap outside the event: HR-P2-doc-66. |
| HR-P1-err-94 | Holds for the erase itself (grace `stopBudget*4/5`, `nodes.go:92`, `:277-288`); incomplete for the stop as a whole: HR-P2-conc-60 (the bridge join is unbounded, and the erase holds `s.mu` across it). |
| HR-P1-err-101 | Holds: `s.base.Err()` checked under `s.op` (`nodes.go:270-274`) and again before the relaunch (`:333-335`). |
| HR-P1-err-91 | Holds: `restore` writes the last-good copy back (`internal/selection/selection.go:188-196`), `openMissing` too. |
| HR-P1-ha-07/-08 (decision 28) | Holds: the back-off resets only after `up > MaxBackoff` (`client.go:231-233`), `up` is set by a deferred assignment after `began` (`:297-298`), an `auth_invalid` waits `authBackoff` doubling to an hour (`:235-240`) and is reset only by an authentication (`:228-230`); the refusal is cleared at `auth_ok` (`:296`). |
| HR-P1-ha-62 (+ha-04) | Holds: a timed-out offer leaves `pending[id]` for `unsubscribeLate` (`client.go:550-555`, `:568-576`), which ends at the result or at the connection's `fail` (closes `ch`, `:452-473`). Probe `TestP2X3ReconnectLeavesNoGoroutine` (5 drops, a live and a given-up offer each time) passes: no goroutine left. |
| HR-P1-err-64 (decision 29) | Incomplete: HR-P2-conc-60. `lockBy` and `stopNode` take the budget, `stopRun`'s `<-s.done` and the camera closes do not; `docs/operations.md:635-645` promises the 25 s. |

## Other flows read, no finding

- **Reconnect mid-command**: an in-flight `call_service` fails with
  `ErrDisconnected` (`client.go:452-473` closes `pending`), the bridge
  restores Home Assistant's last state (`command.go:102-110`) and the
  reconnect's sync reads the truth. Mid-sync: the sync fails, the
  Disconnected closure marks endpoints unreachable, the reconnect syncs. A
  snapshot cannot mix two connections: the reconnect waits at least
  `MinBackoff`, a call in between gets `ErrNotConnected`. Mid-camera: the
  stream closes with `ErrDisconnected` (`client.go:466-469`), the relay ends
  the session towards the controller (`camera.go:644-664`).
- **Events before `Connected`**: the reader runs before the subscriptions
  finish (`client.go:300-305` vs `:314-321`), so a `state_changed` can reach
  a bridge before its `Connected`; harmless, the `Connected` sync follows.
- **Fanout under a stuck consumer**: every `Handler` method of the bridge
  only enqueues (`bridge.go:256-263`, `:280-349`); lock order `Fanout.mu` →
  `Bridge.mu`, never reversed; `served.install`/`stopRun` call `Add`/`Remove`
  under `s.mu`, and no Fanout path takes `s.mu`. `Remove` precedes `cancel`
  (`nodes.go:181-185`), so no event reaches a stopped bridge.
- **Registry fleet**: `Create`, `Rename`, `Delete`, `Erase`, `Close` are
  serialised by `f.op`; `NODE_LIMIT` is checked under it (`nodes.go:597`);
  `served.close`'s `lockBy` cannot contend with a fleet erase (both under
  `f.op`). Delete with a cancelled request ctx still stops the node
  (gf:`node/node.go:1495-1513`: a free lifecycle slot is taken whatever the
  ctx).
- **Goroutines of the inventory**: each `go` statement has its end —
  `main.go:50` (signal ctx), `:199` (joined or abandoned at exit by design),
  `:363` (joined), `nodes.go:156` (joined by `stopRun`, unbounded:
  HR-P2-conc-60), `camera.go:431`, `:461`, `:918` (host `wg`, ended by
  `h.cancel` and the stream closes), `client.go:302` (joined),
  `:552` (ends with the result or the connection), `web.go:131` (`Shutdown`).
  The debounce timer may fire after `Run` and pushes into a dead queue; no
  goroutine waits on it.
- **Web write → node**: `exposure.Apply` stores the selection, go-fabric-ui
  runs `Reassemble` on a detached ctx (ui:`handlers_exposure.go:217-222`),
  the bridge's `Resync` waits for the sync; a failure is reported as
  `reassembled: false` and the next sync applies the stored file (but see
  HR-P2-err-61 and -err-65).

## Not read, and why

- go-hamapping's command path inside `Plan` (boundary only, decision 9);
  the attribute's value after a controller write is taken from its
  `OnChange` contract.
- `internal/adapt`, `internal/plan` beyond `Assess`'s candidate set and
  `Diff`, `internal/expose`: X2's.
- `internal/web` and the supervisor gate beyond their goroutine ends: X1's.
- go-fabric's `Batch` behaviour under a cancelled ctx (the batch at an
  erase or delete is thrown away with the store).
- No full-daemon shutdown probe (the composition needs a real node with a
  camera row and a gone controller); HR-P2-conc-60's chain above the camera
  host is read, not measured.

## Probes run

| Probe | Package | Result at `2324fbf` |
| --- | --- | --- |
| `TestP2X3CloseHonoursItsContext` | `internal/camera` | **FAIL** (1.60 s for 8 sessions, ctx 100 ms) → HR-P2-conc-60 |
| `TestP2X3FailedSyncIsRetried` | `internal/bridge` | **FAIL** (1 read in 3 s) → HR-P2-err-61 |
| `TestP2X3OneEventOneSnapshot` | `internal/bridge` | **FAIL** (9 reads for 9 nodes) → HR-P2-bound-62 |
| `TestP2X3DroppedCommandRestoresTheState` | `internal/bridge` | **FAIL** (both subtests) → HR-P2-err-63 |
| `TestP2X3ReconnectLeavesNoGoroutine` | `internal/ha` | pass (HR-P1-ha-62 holds) |
| `go test -race` of `internal/ha`, `internal/camera`, `internal/bridge`, `internal/nodes`, `internal/selection`, `cmd/ha2fabric` | | no `DATA RACE`; only the probes above fail |

## Questions for the owner

1. **HR-P2-conc-60, the camera close.** Recommendation: **parallel ends
   bounded by the caller's deadline, and `stopRun` bounded by the stop's
   ctx** (fix (a)+(b)), plus the worker's camera closes moved off the sync
   (fix (c)). Best architecture: the stop budget becomes a property of every
   join rather than of the outer wait, and a controller that went away can
   no longer stall a node's state updates. Alternative: only (a)+(b),
   leaving the sync-time stall for go-fabric's "controller gone" signal
   (decision 34) — smaller, but the stall stays until that signal exists.
2. **HR-P2-bound-62, one snapshot per event per node.** Recommendation:
   **one shared single-flight snapshot reader per connection**, the plan
   staying per node. It removes the N-fold read and makes HR-P2-err-61's
   retry cheap. Alternative: keep per-node reads and stagger the debounce —
   simpler, but the load stays N-fold.
3. **HR-P2-doc-66, a removal missed while disconnected.** Recommendation:
   **a doc sentence now**, and a later issue for a persisted served-key set
   if the owner wants the forget to be exact; the forget-on-event design is
   the owner's F1 decision, so changing it is a decision, not a fix.

## Appendix: probe texts

`internal/camera/p2x3_probe_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package camera_test

import (
	"context"
	"testing"
	"time"

	gfcamera "thingfabric.dev/go-fabric/cluster/camera"

	"github.com/SukramJ/go-ha2fabric/internal/camera"
)

// goneController is a signaller whose controller went away: every
// signal waits until its context ends, as go-fabric's Provider.End does
// on ConnectPeer to a peer that no longer answers.
type goneController struct{}

func (goneController) Answer(ctx context.Context, _ uint16, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (goneController) IceCandidates(ctx context.Context, _ uint16, _ []gfcamera.ProviderICECandidate) error {
	<-ctx.Done()
	return ctx.Err()
}

func (goneController) End(ctx context.Context, _ uint16, _ uint8) error {
	<-ctx.Done()
	return ctx.Err()
}

// TestP2X3CloseHonoursItsContext (HR-P2-conc-60): Close of a host whose
// sessions' controller is gone returns by its ctx's deadline, not after
// one SignalTimeout per session in turn.
func TestP2X3CloseHonoursItsContext(t *testing.T) {
	up := &upstream{}
	const timeout = 200 * time.Millisecond
	h := camera.New(entity, up, camera.Options{SignalTimeout: timeout, RetryInterval: time.Millisecond, EvictIdle: time.Hour})
	h.Attach(goneController{})
	for i := range camera.MaxSessions {
		if _, err := h.ProvideOffer(context.Background(), offer(uint16(i+1))); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	began := time.Now()
	h.Close(ctx)
	took := time.Since(began)
	t.Logf("Close of %d sessions took %v (ctx deadline 100ms, SignalTimeout %v)", camera.MaxSessions, took, timeout)
	if took > 3*timeout {
		t.Fatalf("Close took %v with a ctx of 100ms: one SignalTimeout per session in sequence", took)
	}
}
```

`internal/bridge/p2x3_probe_internal_test.go` (uses `fakeUp`,
`rangeBridge`, `rangeCmd` of `queue_internal_test.go`):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package bridge

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// TestP2X3FailedSyncIsRetried (HR-P2-err-61): a sync that fails while
// Home Assistant stays connected is tried again; without a later event
// or a reconnect the bridge otherwise keeps its last plan (none at all
// after a failed first sync) for good.
func TestP2X3FailedSyncIsRetried(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	b := New(nil, up, Options{Debounce: 5 * time.Millisecond, Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = b.Run(ctx) }()
	defer func() { cancel(); <-done }()
	b.Connected("2026.10.3")
	deadline := time.Now().Add(5 * time.Second)
	for up.reads.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if up.reads.Load() < 1 {
		t.Fatal("no first sync")
	}
	time.Sleep(3 * time.Second)
	if n := up.reads.Load(); n < 2 {
		t.Fatalf("a failed sync while connected was not tried again within 3 s: %d read(s), status %q", n, b.Status().SyncError)
	}
}

// TestP2X3OneEventOneSnapshot (HR-P2-bound-62): one registry event on the
// daemon's one connection reads Home Assistant once, not once per node.
func TestP2X3OneEventOneSnapshot(t *testing.T) {
	up := &fakeUp{}
	up.connected.Store(true)
	fan := &ha.Fanout{}
	const nodes = 9 // the configured node and NODE_LIMIT's default of 8
	ctx, cancel := context.WithCancel(context.Background())
	var dones []chan struct{}
	for range nodes {
		b := New(nil, up, Options{Debounce: 5 * time.Millisecond, Logger: slog.New(slog.DiscardHandler), Input: func() plan.Input { return plan.Input{} }})
		d := make(chan struct{})
		dones = append(dones, d)
		go func() { defer close(d); _ = b.Run(ctx) }()
		b.connected.Store(true)
		fan.Add(b)
	}
	defer func() {
		cancel()
		for _, d := range dones {
			<-d
		}
	}()
	fan.Event(ha.Event{EventType: ha.EventEntityRegistryUpdated, Data: json.RawMessage(`{"action":"update","entity_id":"light.a"}`)})
	time.Sleep(500 * time.Millisecond)
	if n := up.reads.Load(); n > 1 {
		t.Fatalf("one entity_registry_updated read the snapshot %d times (once per node)", n)
	}
}

// TestP2X3DroppedCommandRestoresTheState (HR-P2-err-63): a controller's
// command that never reaches call_service (the queue full, or the
// command not mappable) carries Home Assistant's state onto the device
// again, as a failed call_service does (command.go:104-110): the
// controller already saw its value applied.
func TestP2X3DroppedCommandRestoresTheState(t *testing.T) {
	t.Run("queue full", func(t *testing.T) {
		b, _ := rangeBridge(t)
		for range cap(b.cmds) {
			b.cmds <- rangeCmd("temperature_low", "19")
		}
		b.onCommand(rangeCmd("temperature_low", "20"))
		if len(b.work) == 0 && !b.owed.Load() {
			t.Fatal("a dropped command leaves the device at the controller's value: no re-apply and no sync owed")
		}
	})
	t.Run("unmapped", func(t *testing.T) {
		b, _ := rangeBridge(t)
		b.command(context.Background(), rangeCmd("no_such_method", "1"))
		if len(b.work) == 0 && !b.owed.Load() {
			t.Fatalf("an unmapped command leaves the device at the controller's value: no re-apply (status %q)", b.Status().LastCommand)
		}
	})
}
```

`internal/ha/p2x3_probe_test.go` (uses `newRecorder` of `client_test.go`;
passes):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.

package ha_test

import (
	"context"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/ha/hatest"
)

// TestP2X3ReconnectLeavesNoGoroutine (X3 task 5, passes at 2324fbf):
// five drops with a WebRTC subscription open each time, then the end of
// Run: the client's goroutines (reader, late unsubscribes) all end.
func TestP2X3ReconnectLeavesNoGoroutine(t *testing.T) {
	srv := hatest.New(t)
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	c := ha.New(ha.Options{URL: srv.URL(), Token: hatest.Token, MinBackoff: 10 * time.Millisecond, MaxBackoff: 20 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	r := newRecorder()
	done := make(chan struct{})
	go func() { defer close(done); _ = c.Run(ctx, r) }()
	for range 5 {
		r.wait(t, "connected")
		sub, err := c.WebRTCOffer(context.Background(), "camera.front_door", "v=0")
		if err != nil {
			t.Fatal(err)
		}
		// A subscription whose caller gives up before the result.
		sctx, scancel := context.WithCancel(context.Background())
		scancel()
		_, _ = c.WebRTCOffer(sctx, "camera.front_door", "v=0")
		srv.Down()
		r.wait(t, "disconnected")
		for range sub.Events() {
		}
		srv.Up()
	}
	r.wait(t, "connected")
	cancel()
	<-done
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		buf := make([]byte, 1<<16)
		t.Fatalf("goroutines %d before, %d after:\n%s", before, after, buf[:runtime.Stack(buf, true)])
	}
}
```
