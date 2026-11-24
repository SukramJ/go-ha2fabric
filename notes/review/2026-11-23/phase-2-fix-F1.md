# Phase 2 fix wave — F1 (sync, bridge, fleet)

Implementer F1 (Opus). Brief `notes/briefs/2026-11-24-review-p2-fix.md`,
section F1. Worktree `go-ha2fabric-review`, branch `review/p2`, base
`d6003b4`. Findings: `phase-2-X2.md`, `phase-2-X3.md`, the verifier's
`phase-2-verify.md` (wins), decisions 35–46.

## Commits

| Commit | Rows |
| --- | --- |
| `b3d8de2` fix(bridge): the re-sync debounce gets a latest time | data-30 |
| `2eae39e` fix(bridge): a cancelled sync or call_service is no failure | err-67 |
| `4f07038` fix(bridge): a failed sync is retried with a back-off; Resync says whether it planned | err-61, err-65 |
| `3aafa56` fix(exposure): a sync that keeps failing is a sync_failed warning | err-61, bound-37 (page half) |
| `5fd40b1` fix(ha): one shared single-flight snapshot reader per connection | bound-62 |
| `39c726d` fix(bridge): a dropped or refused command puts Home Assistant's state back on the device | err-63 |
| `0681b86` test(ha): gofumpt-stable fake of the shared reader's test | bound-62 |
| `0c9b38b` fix(bridge): dropped work and commands logged as a rate-limited summary | bound-64 |
| `9eb4169` fix(bridge): only the served entities' states are held beside the snapshot | bound-38 (bridge half) |
| `66bf80c` fix(fleet): the bridge's join bounded by the stop's context, an erase waits outside the node's lock | conc-60 |
| `d05c385` fix(bridge): camera hosts closed beside the worker, not in the sync | conc-60 |
| `a00b8ac` test(bridge): fuzz target Map, Build and Apply over hostile entity data in fuzz-smoke | test-42 |
| `340f0ae` docs: the sync's scheduling, the drop summaries, the stop's bridge join, a deletion while disconnected, the forged Ingress source | doc-66, net-01, the docs of every row |
| `b72ab7b` fix(bridge): lint of the phase 2 fixes | bound-64, test-42 |
| this report | — |

## Guards

At `b72ab7b` (HEAD before and after the run): `make ci` 0, `make race` 0,
`make cover-check` 0, `make fuzz-smoke` 0 (with `FuzzPlanBuild`). An
earlier run at `340f0ae` failed `make ci` on lint only (contextcheck in
`drops.go`, gocritic rangeValCopy in a test, ST1018 for a literal U+202E
in a fuzz seed); `b72ab7b` fixes them. This report's commit is notes only:
no second run. Not run (not in the brief's set for F1): `make vuln
licenses secrets`, `make release`, `make docker addon-image`.

## Per row

Red seen at base `d6003b4` with the reviewers' probes, unchanged, copied
into the tree for one run and then removed:
`TestP2X3FailedSyncIsRetried` (1 read after 3 s),
`TestP2X3OneEventOneSnapshot` (9 reads),
`TestP2X3DroppedCommandRestoresTheState` (both subtests),
`TestX2ProbeResyncStarved` (5 s, last sync 5.6 s before the flood ended).

| Row | Status | Test | Red seen |
| --- | --- | --- | --- |
| data-30 | fixed: `resyncSoon` keeps the first pending request; the timer fires at `min(last + Debounce, first + 5 × Debounce)`; a timer replaced after it fired does nothing (`armed` count) | `TestDebounceHasALatestTime` | yes, at the previous commit, and the X2 probe |
| err-61 | fixed: a sync that fails (read or batch) while connected and not cancelled re-arms the one timer after `Options.Retry` (5 s), doubling to 5 min, reset by a sync that succeeded; a pending sync is the retry; none while disconnected | `TestFailedSyncIsRetried` | by the X3 probe (the test needs the new `Retry` option) |
| err-65 | fixed: `sync` returns its error and `Resync` returns it (`errNotConnected` when Home Assistant went away before the sync ran); `served.Resync` without a run returns `errNotRunning` instead of nil | `TestResyncSaysWhetherItPlanned`, `TestResyncWithoutRun` | yes, both at the previous commit |
| bound-37 (page half) | fixed: `View.SyncFailure` from the second failure in a row, whatever the cause (a cancel and the not-connected path not counted); the provider's warning `sync_failed` (`exposure.WarningSyncFailed`) with the error and since when | `TestSyncFailureInView`, `TestProviderSyncFailed` | build failure only (new API) |
| bound-62 | fixed: `ha.Snapshots`, single-flight: a caller joins the read in flight or the last finished one when it began at or after the caller's trigger, else starts one; the read is detached from the caller's ctx, the wait is not. One per connection through `ha.Fanout.Snapshots(src)`, wired in `cmd/ha2fabric/nodes.go`. The trigger time goes through `sync(ctx, since)`: the latest event's for the debounce, the connection's, the `Resync` call's, the retry's firing, now for an owed sync | `TestSnapshotsShared`, `TestSnapshotsWaitEndsWithCaller`, `TestFanoutSnapshots`, `TestOneEventOneSnapshot` | by the X3 probe (9 reads) |
| err-63 | fixed: `restore(id)` enqueues the apply of the current state from all three paths (queue full, `adapt.Service` refused, `call_service` failed); a full work queue owes a sync instead | `TestDroppedCommandRestoresTheState` (from the X3 probe) | yes, both subtests |
| bound-64 | fixed: `dropLog`, one Error line at a run's first dropped item (`dropped=1`, the command's `entity`), then one per 10 s with the count while items keep dropping, quiet after an empty interval | `TestQueueFullLogsASummary`, `TestCommandDroppedLogsASummary` | yes, in a copy at `0681b86`: 76 `queue_full` lines for 76 drops |
| bound-38 (bridge half) | fixed: `b.states` holds only the served entities' states (pointers into the snapshot, then each `state_changed`'s); an unserved entity's state is not kept. With bound-62 the nodes of a connection share one snapshot instead of one each. Unchanged: a sync holds the last snapshot (the read model) while it reads the next | `TestStatesHeldOnce` (e2e; `export_test.go` `HeldStates`) | yes: 33 states held for one served entity |
| conc-60 (bridge, fleet) | fixed: `stopRun(ctx)` = `leave` + `join(ctx)`, an overrun logged `ha2fabric.bridge_stop_overrun`; an erase takes the run out under `s.mu` and waits outside it, bounded by its `ectx`; `closeCameras` hands each close to a goroutine (`b.closing`), `Run`'s end closes the rest in parallel and waits for all. The camera half is F2's: the signature stays `Close(ctx)`; until F2's `Close` returns at ctx's end, the worker's end still waits for it, but `stopRun` and the sync no longer do | `TestCloseBoundedByBridge`, `TestCameraCloseOffTheSync` (seam `closeHost`) | yes, both (the second after the seam alone) |
| test-42 | fixed: `FuzzPlanBuild` in `internal/bridge` (from X2's probe; the property strengthened: every verdict not unmappable must build, partial included), in `FUZZ_TARGETS`; known refusals skipped by name (`knownRefusal`) | the target; `make fuzz-smoke` | yes: without the skip the seeds fail on HR-P2-adpt-33 and on a new refusal (below) |
| doc-66 | fixed: one sentence in the deleted-entity paragraph of `docs/operations.md` (deleted while stopped or not connected: re-created, it gets its old number back) | — (docs) | — |
| err-67 (note) | fixed: a sync or `call_service` whose worker ctx ended logs at Debug (`bridge.sync_cancelled`, `bridge.call_service_cancelled`), sets no `SyncError` or command error and schedules no retry | `TestCancelledSyncIsNoFailure`, `TestCancelledCommandIsNoFailure` | yes, both |
| conc-68 (note) | not fixed, accept: the fix (a `Close` on the selection store that makes `Set` fail, called by `served.close`) lies in `internal/selection`, F2's path; the window is narrow, nothing is lost, the orphan file only gives a later node of the same label a suffix | — | — |
| conc-69 (note) | not fixed: the doc comment belongs in `internal/camera/camera.go` (F2's path); for F2 or round 2 | — | — |
| net-01 (F2's table: F1's docs) | docs: the forged-source argument (T2) after the Ingress paragraph of `docs/operations.md`, marked as not yet measured on HAOS | — | — |

## New finding (upstream, go-hamapping v0.6.0)

`FuzzPlanBuild`'s own seeds found a third refusal of what `Map` calls
mappable: a thermostat row with `min_temp` above `max_temp`. Seeds
`water_heater.a` (`min_temp` 1e9, `max_temp` -1e9: partially mappable)
and `climate.b` (`min_temp` 30, `max_temp` 7: mappable) both fail `Build`
with `thermo: setpoint configuration cannot be fixed:
absMinHeatSetpointLimit … is above absMaxHeatSetpointLimit …`. X2's probe
did not see it: it checked `mappable` only, and its climate seed has
`min_temp` below `max_temp`. Skipped by name beside adpt-33/-34
(`knownRefusal`: climate and water_heater with that error) until the pin
moves. It belongs in decision 35's go-hamapping pull request (the check
that what Map calls mappable builds): swap or refuse inverted limits.

## changelog.md and docs

- `changelog.md`, `[0.7.0]` → `### Fixed`, beside phase 1's lines: five
  bullets (data-30/err-61/bound-37/err-65/bound-62; bound-38; conc-60;
  err-63; bound-64/err-67). F2's lines not added yet (the coordinator
  asks).
- `docs/operations.md`: the reads table (one read for every node, the
  retry); log rows `bridge.sync_failed`, `bridge.queue_full`,
  `bridge.command_dropped`, `ha2fabric.bridge_stop_overrun`;
  `bridge.command_unmapped` (state back); the label debounce's latest
  time (five seconds); `sync_failed` among the status page's warnings;
  the doc-66 sentence; the stop budget (a bridge not waited for beyond
  it; "the camera sessions … end in parallel" rests on F2's `Close`); the
  net-01 argument.

## Differences from the brief

- Three known refusals skipped in the fuzz target, not two (above).
- err-65 also changes `served.Resync` without a run (nil →
  `errNotRunning`), X3's second half of the row: go-fabric-ui then answers
  the reassemble as failed, and the stored selection waits for the next
  run.
- A failed batch counts as a failed sync for the retry and the warning
  (`settle` already expects the next sync to apply it).
- The warning starts at the second failure in a row ("keeps failing"),
  not the first.
- `cmd/ha2fabric/warnings.go` needed no change: `sync_failed` reaches every
  node's status through the provider's `Warnings`.
- One formatting commit (`0681b86`): gofumpt did not settle on an aligned
  block of one-line methods.

## For F2 and the coordinator

- Every node on the connection now shares the snapshot, so nothing may
  change it in place. Nothing does today (`internal/adapt`, `plan`,
  `exposure` and `bridge` only read it; `sentRangeLocked` clones). F2's
  data-31 (control characters dropped in `internal/adapt`) must build new
  values, not edit `State.Attributes` maps in place.
- bound-38's verdict reason (F2's decode half): no bridge field added; if
  F2 reports one, round 2 adds it.
- `Makefile` `FUZZ_TARGETS` gained `./internal/bridge/:FuzzPlanBuild`; a
  target F2 adds touches the same lines.

## Questions

1. An ID for the thermostat refusal, and does it join decision 35's
   go-hamapping pull request?
2. `sync_failed` from the second failure in a row: the right threshold,
   or from the first?
