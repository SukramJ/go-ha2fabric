# Phase 1 fix wave — round 2 (pre-merge read)

Implementer (Opus). Brief `notes/briefs/2026-11-23-review-p1-fix2.md`,
common rules of `2026-11-23-review-p1-fix.md`. Branch `review/p1`, base
`5f86ed3`. Rows from the Fable pre-merge read of `27b68ec..2fe4944`;
decisions 25, 28, 29, 31, 32.

## Commits

| Commit | Row |
| --- | --- |
| `660517d` test(bridge): the queue tests count a snapshot's first read, get_states since HR-P1-data-67 | red guard |
| `a0988fe` fix(bridge): an owed sync wakes a waiting worker | conc-96 |
| `5a2e93b` fix(ha): a refused token is cleared at the next authentication | ha-92 |
| `6608e85` fix(selection): a restored last-good copy is written back, a missing file beside it restored; nothing understood says the overrides started empty | err-91, cfg-95 |
| `0b413b9` fix(bridge): a removal noted during a sync's read is forgotten by the next sync | conc-93 |
| `6f6105f` fix(cmd): a stop lets a running erase finish its reset inside the stop budget | err-94 |
| `2f1ee6d` fix(ha,plan): a skipped entity or device registry entry leaves its entities unexposed, a skipped label entry fails the sync while the rule names labels | data-90 |
| `ab2bcee` fix(cmd): the erase's detached context marked for contextcheck | err-94 (lint) |
| `0f9f37c` docs(changelog): F1's lines say what round 2 changed | data-90, err-91, ha-92, err-94 |
| `4765b95` test(plan): the skipped-entry tests live where the layers allow plan and hatest | data-90 (arch guard) |
| this report | — |

err-91 and cfg-95 share one commit: both change `Recovery.Message`'s
cases and `Open`'s reading of the file.

## Guards

At HEAD `4765b95` (before and after the run): `make ci` 0, `make race`
0, `make cover-check` 0, `make fuzz-smoke` 0, `make vuln licenses
secrets` 0, `make version-check` 0, `make release` 0, `make docker
addon-image` 0. Summary log
`/tmp/claude-1000/-home-markus-Dokumente-GitHub/700e1304-db09-4033-9c98-588d8ec93391/scratchpad/R2/summary.log`.

A first run at `0f9f37c` had `make ci` 2: `internal/arch` `TestImports`
refused `internal/ha/skipped_test.go` importing `plan` and `expose`; the
tests moved to `internal/plan` (`4765b95`), and the run was stopped and
restarted at the new HEAD (its logs in `R2/first/`). The report commit is
notes only, no further run.

## The red guard test

`TestQueueFullResyncIsOwed` ("never: a sync after the overflow"): **the
test's premise changed, not the code.** Its fake upstream (`fakeUp`)
counted a sync's Home Assistant read in `ListEntities` and fails every
read. F1's HR-P1-data-67 made `get_states` the first read of
`ha.ReadSnapshot`; the fake's failing `GetStates` now ends the snapshot
before `ListEntities`, so the counter stayed 0 although the owed sync
ran. On F2's branch the registries were still read first, hence green
there. Fix: the fake counts `GetStates`, the first read. Green 3/3 at
once; the bridge's owed-sync logic was not the cause. conc-96 (below) is
a separate, real race, fixed on its own red test.

## Per row

"Red" = the regression test was run against the unfixed code and failed.

| Row | Status | Test | Red |
| --- | --- | --- | --- |
| red guard | fixed in the test (premise changed, above) | `internal/bridge/queue_internal_test.go` `TestQueueFullResyncIsOwed` | it was the red test; green after |
| data-90 | fixed: `listOf` takes an id reader; the entity, device and label lists return the entries that decoded **with** a `*ha.SkippedError` naming the skipped ids (`entity_id`, `id`, `label_id`); an entity or device entry whose id cannot be read fails the list. `ReadSnapshot` moves the ids into `Snapshot.Skipped`. `plan.Assess` gives an entity whose own entry was skipped (`plan.UnreadEntity`) or whose device's entry was skipped (`plan.UnreadDevice`) a not-exposed verdict with that reason, after the rule and overrides (it wins). `plan.Check` refuses a snapshot with a skipped label entry while `INCLUDE_LABELS`/`EXCLUDE_LABELS` hold values; the bridge's sync and `--dump-plan` call it (a sync error, as a failed read). States and areas stay lenient. Docs sentence rewritten (What the daemon reads and writes; log row) | `internal/plan/skipped_test.go` `TestSkippedEntityEntryIsNotExposed`, `TestSkippedDeviceEntryIsNotExposed`, `TestSkippedLabelEntryFailsTheSync` (with the negative: a rule without labels uses the snapshot), `TestSkippedEntryWithoutID`; F1's `TestListSkipsEntryThatDoesNotDecode` now also asserts the `SkippedError` ids | yes, all three: `exposed true (included by *)` each. The label test was red as "exposed"; after the fix it asserts the refusal (`plan.Check` did not exist before). `TestSkippedEntryWithoutID` tests the new failure path, no red possible |
| err-91 | fixed: a restore from `.bak` writes the copy back as `selection.json` (`atomicWrite`; a failed write logs `selection.restore_failed`); a missing `selection.json` beside a `.bak` reads the copy, writes it back, and reports a Recovery (`Missing`, `selection.missing`); a `.bak` that does not parse then says the overrides started empty | `internal/selection/recover_test.go` `TestRestoreLastsPastOneStart` | yes: `second start: entries map[]` |
| ha-92 | fixed: `setRefused(false)` right after `auth_ok`, before the subscriptions and `Connected` | `internal/ha/client_p1_test.go` `TestClientAuthRefusedThenAccepted` (refusal first, then a good auth) | yes: `authenticated, the refusal still reported` |
| conc-93 | fixed: the bridge counts noted removals; a sync records the count before its read, and a forget `plan.Diff` dropped because the plan still serves its key, noted after that read began, is recorded again for the next sync (`keepStaleForgets`). A forget noted before the read is of an entity back in the registry and is dropped as before (the debounce rule in `docs/operations.md` holds) | `internal/bridge/bridge_p1_e2e_test.go` `TestRemoveDuringSyncIsForgotten` (a wrapped upstream holds the sync after its registry read; the removal's event is ordered before a `get_config` answer on the one connection) | yes: `the re-added plug got its old number 23` |
| err-94 | fixed: the erase runs on `WithoutCancel(base)` bounded by `eraseTimeout`; once the daemon stops it has `eraseGrace` (4/5 of `stopBudget`, 20 s) more, then is cancelled; a reset that fails while stopping keeps the node as stale for `close` (no extra 10 s stop); no come-back run starts while stopping (answers `errStopping`). `docs/operations.md` (Erase and factory reset, stop paragraph): an erase the stop interrupts must be repeated after the next start | `cmd/ha2fabric/stop_test.go` `TestStopLetsEraseFinish`; `TestStopWithinBudget` still green | yes: `the stop cancelled the reset at once: context canceled` |
| cfg-95 | fixed: `Recovery.Empty` for a file of another version with no entities (`{}`, `null`, `{"version": 2}`): the "started empty … exposed again" wording | `recover_test.go` `TestNothingUnderstoodWarns` | yes: `"… is version 0, not 1: its overrides were read … and apply …"` |
| conc-96 | fixed: `Bridge.wake`, a one-slot channel; `owe()` marks the sync owed and wakes the worker, which runs it when the queue is empty (overflow and debounce timer both use it) | `queue_internal_test.go` `TestOwedAfterDrain` | yes, by construction (the owed mark set while the worker waits on an empty queue — the state the race leaves): `never: the owed sync`. The red run marked with `b.owed.Store(true)`, the test now calls `b.owe()` |
| adpt-97, bound-98 | accepted (decisions 32 and 31) | — | — |

## Changelog and docs

`changelog.md`, 0.7.0, F1's Fixed lines edited, no new lines:

- the refused token: the warning "clears as soon as Home Assistant
  accepts the token again" (ha-92);
- the skipped entry: an entity whose entity or device entry is skipped is
  not exposed until it reads again; a skipped label entry fails the sync
  while label values are configured (data-90);
- `selection.json`: the copy is "read instead and written back", also
  when the file is missing; "or nothing in the file is understood"
  (err-91, cfg-95);
- the stop: "An erase in progress may finish its reset for up to 20 of
  those seconds and does not bring the node back" replaces "ends with the
  daemon" (err-94; F1's line had become false).

`docs/operations.md`: the skipped-entry paragraph (What the daemon reads
and writes) and the `ha.entry_skipped` row; `selection.json` paragraph
(written back, missing file, nothing understood) and the rows
`selection.missing`, `selection.restore_failed`; Erase and factory reset
and the stop paragraph (err-94). The `ha_token_refused` sentence
("until an authentication succeeds") was already right and is now true.

## Differences from the brief

- data-90: the skipped ids travel as a `*ha.SkippedError` returned beside
  the decoded entries, so `ha.Source` keeps its signatures; only
  `ReadSnapshot` reads the three lists. The label refusal is
  `plan.Check` (the rule lives in `plan`, not in `ha`); a skipped include
  label also refuses (the brief says "label values").
- data-90's end-to-end tests are in `internal/plan`, not `internal/ha`:
  the architecture guard (`internal/arch` `TestImports`) refuses `plan`
  and `expose` in `internal/ha`'s tests — this is what turned the first
  guard run's `make ci` red (fixed in `4765b95`).
- `hatest.Server.UpdateLabelFields` added (a test needs a label field of
  another type).
- err-94: besides the grace, the come-back no longer starts while the
  daemon stops, and a failed reset then is kept as stale for `close`
  rather than stopped with its own 10 s.
- conc-93: bounded to one further sync by the removal count, so a forget
  never lingers past the first sync whose read began after it.
- A `grep` for stale docs sentences listed lines of a file under `held/`;
  it was not opened or used.

## Questions

None.
