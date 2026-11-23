# Phase 1 fix wave — F2, the plan side

Brief `notes/briefs/2026-11-23-review-p1-fix.md`, section F2; findings in
`phase-1-GA.md` and `phase-1-GB.md` (conc-60); the verdicts of
`phase-1-verify.md`; decisions 26 and 30. Branch `review/p1-F2` from
`27b68ec`. Paths: `internal/bridge`, `internal/adapt`, `internal/plan`,
`internal/expose`, `internal/exposure`.

## Commits

| Commit | Row |
| --- | --- |
| `d8cd067` fix(bridge): a full queue owes a sync instead of dropping work | HR-P1-conc-32 (+ conc-60) |
| `54f02bf` fix(bridge,adapt): a climate range completes from the bounds last sent | HR-P1-adpt-30 |
| `46a71c7` fix(bridge): a failed batch operation is not taken as served | HR-P1-err-31 |
| `09c41ba` fix(bridge): a queued command of an entity no longer served is dropped | HR-P1-conc-33 |
| `dfdcdff` fix(expose,bridge): a renamed label keeps its exclusion | HR-P1-cfg-34 |
| `7b465b6` test(plan,bridge): the plan side's missing negative cases | HR-P1-test-38 |
| `7974490` fix(bridge): the batch settle's switch is exhaustive; the range test indexes its commands | HR-P1-err-31 (lint) |
| this report | — |

## Guards

At `7974490`: `make ci` 0, `make race` 0, `make cover-check` 0,
`make fuzz-smoke` 0 (summary log, HEAD before and after `7974490`). The
first run at `7b465b6` had `make ci` 2 (golangci-lint: `exhaustive` in
`settle`, `rangeValCopy` in a test), fixed in `7974490`; race, cover-check
and fuzz-smoke were 0 then too. The report commit is notes only, no second
run.

Not run: `make vuln licenses secrets`, `make release`, `make docker
addon-image` — no dependency, version, add-on or image file moved.

## Per row

| Row | Status | Test | Red seen |
| --- | --- | --- | --- |
| adpt-30 | fixed (stop-gap of decision 26) | `internal/bridge/queue_internal_test.go` `TestRangeSetpointsInSequence` (b) "raise past the old high", (a) "state not yet reported", and "state shows them, then moves" (the hold ends); `internal/adapt/adapt_test.go` `TestServiceRangeNeverInverted` | yes: (b) `a call with low 25 above high 24`, `last call [18 28]`; (a) `last call [18 26]`; adapt `low 25 high 24, want 25 25`. The third case is a guard for the fix, green before it. All by construction, no delay in a fake |
| conc-32 (+ conc-60) | fixed, one change | `queue_internal_test.go` `TestQueueFullResyncIsOwed` (G-B's probe), `TestQueueFullDisconnected`, `TestQueueFullRemoveIsForgotten` | yes, all three: `never: a sync after the overflow`, `never: the endpoint unreachable`, `forget <nil>` |
| err-31 | fixed | `internal/bridge/bridge_p1_e2e_test.go` `TestFailedBatchOperationIsRetried` (a foreign endpoint holds the light's key) | yes: `Resync: <nil>` |
| conc-33 (note) | fixed | `bridge_p1_e2e_test.go` `TestQueuedCommandOfUnexposedEntity` | yes: `switch.coffee: unexpected [switch.turn_on switch.coffee]` |
| cfg-34 | fixed (G-A's "further" shape plus the warning) | `bridge_p1_e2e_test.go` `TestExcludeLabelRenamed`; `internal/expose/expose_test.go` `TestLabelIDsFollowARename`; `internal/plan/plan_test.go` `TestWarnings` (two cases added) | yes, end to end: after the rename `Exposed:true … Source:rule:include`. The unit tests are of the new API (no red possible before it existed) |
| test-38 | done | `internal/plan/diff_test.go` `TestDiff` "an entity moved to another device"; `bridge_p1_e2e_test.go` `TestEntityMovedToAnotherDevice`; `plan_test.go` `TestAssessIgnoresInputOrder` (standing guard: states, entities, devices, labels, areas shuffled 20 times, verdicts, group keys and signatures equal); the row's other cases are the tests above | gap, not defect: green on first run (the moved case once failed on a test race — `EndpointOf` read before the topology was published — fixed in the test) |
| data-37 (note) | accepted | — | — |
| bound-39 (note) | accepted; the moved case pinned in `TestDiff` | — | — |

### What each fix does

- **adpt-30.** `adapt.Service` moves the other bound with one that crosses
  it (low 25 against high 24 sends 25/25), so no call is one Home
  Assistant refuses (`climate/services.py:121-131` at 2026.10.3, a strict
  `low > high`). The bridge keeps per entity the range its last successful
  `set_temperature` carried (`Bridge.ranges`, under `mu`) and completes the
  next `temperature_low`/`temperature_high` from it until a state (a
  `state_changed` or a sync's snapshot) shows exactly that range, or
  `rangeHold` (30 s) passes. With both, (b) ends on 25/28 with no refused
  call and no command error, and (a) ends on 20/26.
- **conc-32.** `Bridge.owed` (`atomic.Bool`) is set by `enqueue`'s overflow
  and by the debounce timer when its send finds the queue full; the worker
  runs a sync after a function when the flag is set **and the queue is
  empty** (see Differences). `sync`'s not-connected path marks every
  endpoint unreachable (logged once per outage, `Bridge.down`);
  `Disconnected` now enqueues a sync. A registry `remove` is recorded under
  `mu` at the event (`noteRemoved`, `forget` moved under `mu`). The
  `bridge.queue_full` hint: "a re-sync follows once the queue is drained".
- **err-31.** The batch keeps each operation's error by index, wrapped with
  its kind and key. `settle`: a refused Add's key leaves its group's keys
  and endpoints; a group with a refused Add or Replace keeps no signature,
  so the next sync rebuilds it and `plan.Diff` adds or replaces again; a
  failed Forget is recorded again. The joined error is `SyncError`, so
  `Resync` returns it; a later clean sync clears it. Retry is on the next
  sync (any event, reconnect, the web UI's reassemble), no timer of its own:
  a persistent refusal would otherwise read Home Assistant in a loop.
- **conc-33.** The sync publishes the entity ids it serves on an endpoint
  (`Bridge.serving`); `command` drops a command whose entity is not among
  them, logs `bridge.command_unserved` and counts it as a command error.
- **cfg-34.** `expose.Rule.LabelIDs`: the id each label value resolved to
  before; a value that names no label now follows it while that label
  exists (Home Assistant keeps the `label_id` on a rename,
  `helpers/label_registry.py:183-224`, as G-A cites). New methods
  `Followed`, `Resolutions`, `Excludes`. The bridge keeps the memory from
  sync to sync and gives it to the web UI's provider through
  `View.LabelIDs`, so previews decide as the bridge. Warnings: an
  unresolved exclude value "excludes nothing now"; a followed value names
  the id and the new name. The memory is not persisted: after a restart a
  renamed label configured by its old name is unresolved again, and the
  warning says the exclusion has lapsed.

## Accepted (one line each, for the register)

- **data-37:** Home Assistant validates every unclamped value
  (`climate/services.py:117-160`, `light/const.py:174-195`) and refuses
  what is out of range; go-hamapping's rows send one colour group per
  change, so the asymmetry has no effect a controller can use.
- **bound-39:** a moved entity's old key keeps its stored number, bounded by
  Home Assistant's device churn; forgetting it needs `plan.Built` to carry
  each key's entity, or go-hamapping's stable-key helper (HR-P1-adpt-69) —
  better done with that issue than by parsing the key here.

## Changelog lines (for F1 / the coordinator, `[Unreleased]`, Fixed)

- A controller that moves a thermostat's heat/cool range writes two
  setpoints; the second call now carries the first one's new bound instead
  of the stale state's, and a bound that crosses the other moves it along,
  so Home Assistant no longer refuses the first call (HR-P1-adpt-30).
- A burst of Home Assistant events that overflows the bridge's queue no
  longer loses the promised re-sync, the unreachable marking of an outage or
  the forgetting of a removed entity (HR-P1-conc-32).
- An endpoint go-fabric refuses in a sync's batch is no longer counted as
  served: the status shows `sync_error`, the web UI's reassemble returns it,
  and the next sync adds it again (HR-P1-err-31).
- A controller command still queued when its entity leaves the bridge is
  dropped (`bridge.command_unserved`), not sent to Home Assistant
  (HR-P1-conc-33).
- An include or exclude label configured by its name keeps working after
  the label is renamed in Home Assistant, while the daemon runs; the warning
  of an exclude label that names no label says it excludes nothing now
  (HR-P1-cfg-34).

## Docs sentences (for F1, `docs/operations.md`)

- Log table, new row: "`bridge.command_unserved` | warn | A controller's
  command waited in the queue while a sync took its entity off the bridge;
  it was dropped and counted as a command error."
- Log table, new row: "`bridge.batch` | error | go-fabric refused part of a
  sync's batch (the operation and the key); what it refused is tried again
  at the next sync, and the status shows it as `sync_error` until a sync
  succeeds."
- Log table, `bridge.label_unresolved`: append "or follows a renamed label
  by its id; an exclude label that names no label excludes nothing."
- Labels section, after "until the label exists or the value is
  corrected.": "When Home Assistant renames a label, a value that named it
  follows its label id while the daemon runs, and the warning names the id
  and the new name; a restart resolves the names afresh, so update the
  value (or configure the label id) after a rename. An exclude label that
  names no label excludes nothing, and its warning says so."

## The go-hamapping issue (decision 30, HR-P1-adpt-30)

- **Title:** a heat/cool range as one command carrying both bounds.
- **Why:** in Auto, a write of OccupiedHeatingSetpoint is the command
  `temperature_low` and one of OccupiedCoolingSetpoint `temperature_high`
  (`rows/climate.yaml:93-96`, `:101-104`, `:113-120`; `rows_gen.go:818`,
  `:820`, `:823-824` at v0.6.0), each with one value. Home Assistant's
  `climate.set_temperature` takes both bounds or neither
  (`climate/services.py:54-59`, `Inclusive`) and refuses a low above the
  high (`:121-131`), so a host completes the other bound from a state that
  may not show the first write yet; a range moved past its old high is
  refused or ends on a bound the user did not choose. go-fabric's Thermostat
  server already holds both setpoints after the write, including its own
  deadband adjustment (`cluster/thermo/setpoints.go:500-520` at v0.25.0).
- **Shape:** one method (e.g. `MethodTemperatureRange`) whose payload
  carries both bounds read from the Thermostat server's two attributes after
  the write (one command per write request or per change of either), in
  place of the two single-bound methods in Auto; the single-bound methods
  stay for Heat and Cool.
- **Acceptance:** a write of both setpoints in one request, and a
  SetpointRaiseLower in mode Both, give one command with both new values;
  a write of one setpoint that the deadband adjusts gives one command with
  both values the server holds.
- **What stays here:** `adapt.Service` maps the range command to one
  `set_temperature` with both keys; the stop-gap (`Bridge.ranges`, the
  crossing guard in `adapt`) is removed once go-hamapping is bumped.

## Differences from the brief

- conc-32: the owed sync runs when the queue is drained, not after every
  function. A sync after each closure would read Home Assistant once per
  queued event in a flood, and the closures still queued are older than its
  snapshot, so applying them after it would roll values back. The overflow
  sets the flag directly; the debounce timer is no longer armed by it.
- adpt-30: besides the bounds last sent, `adapt.Service` moves the other
  bound with one that crosses it; without that, case (b)'s first call is
  still refused (the bounds last sent alone only fix the end state, with a
  command error and a moment where Home Assistant's state is put back on
  the device).
- adpt-30 is tested at the bridge's command path with a fake upstream, not
  end to end through a controller write: the fixtures have no `heat_cool`
  thermostat and the fake Home Assistant cannot hold a call (its reader is
  serial, `hatest` is F1's). The end-to-end tests of conc-33 wrap the
  bridge's upstream instead (`setup.wrap`, a test-only seam).
- err-31: no automatic retry timer; the next sync retries.
- cfg-34: the label memory lives in the process, not on disk.

## Questions

1. cfg-34: should the label memory be persisted (beside `selection.json`)
   so a rename survives a restart? Recommendation: no for now — the warning
   says what lapsed, and `docs/operations.md` recommends the label id for
   exclude labels; revisit if operators hit it.
2. adpt-30: `rangeHold` is 30 s. A thermostat that reports later than that
   loses the stop-gap; one whose report never matches exactly (rounding)
   is held for 30 s. Acceptable until go-hamapping's range command?
