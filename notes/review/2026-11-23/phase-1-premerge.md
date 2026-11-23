# Phase 1 fix wave — pre-merge read (Fable), rounds 1 and 2

Two bounded reads of the fix wave on `review/p1` before the merge. One
row of phase 1 is held (decision 24) and is not part of either read.
M = read or probe ran, I = inferred.

## Verdict

**Ready to merge.** Round 2 closes every row of the first read; no fix
fails open and none regresses. Four new rows, one low (a docs sentence)
and three notes; none blocks the merge.

## Round 1 (`27b68ec..2fe4944`): rows 90–98

The first read found two fixes that failed open and one warning that did
not clear; verdict then: not ready.

| ID | Severity | What |
| --- | --- | --- |
| HR-P1-data-90 | medium | A registry entry that did not decode was dropped from its list; its entity, still having a state, was judged as unregistered — without `hidden_by`, its labels and its device's labels |
| HR-P1-err-91 | medium | A restore from `selection.json.bak` was not written back; the second start read "missing", without overrides and without a warning |
| HR-P1-ha-92 | low | `ha_token_refused` stayed up for the whole of a good connection |
| HR-P1-conc-93 | low | A removal noted while a sync was between its read and its batch was consumed by that sync and dropped |
| HR-P1-err-94 | low | A stop cancelled a running erase at once, possibly between the fabrics' removal and the store's reset |
| HR-P1-cfg-95 | note | `{}` or `null` read as "version 0" with a message that its overrides "apply" |
| HR-P1-conc-96 | note | The failed send and the owed mark were two steps; a drained queue left the owed sync waiting |
| HR-P1-adpt-97 | note | The held thermostat range, 30 s (decision 32) |
| HR-P1-bound-98 | note | The camera cap's "idle longest" (decision 31) |

## Round 2 (`5f86ed3..4765b95`, notes excluded)

Read: the diff, `phase-1-fix-round2.md`, the brief, the commit messages
`5f86ed3..HEAD`. Lines at `4765b95`. Probes in a `git archive 4765b95`
copy (`…/scratchpad/PM2/src`, files `internal/plan/probe_pm2_test.go`,
`internal/selection/probe_pm2_test.go`); nothing in the worktree was run
or changed except this file.

### Per row

| Row | Closed | M/I | How it was checked |
| --- | --- | --- | --- |
| data-90 | yes | M | `internal/plan/plan.go:139-146`, `:191-196`: the not-exposed verdict is set after the rule, the overrides and "no state yet", so nothing later turns it back. Probe: an override "on" with the entity's entry unread, and with its device's entry unread, is not exposed (reasons `UnreadEntity`, `UnreadDevice`); `plan.Candidates` (the web UI's "as if exposed" view) gives the same verdict, and `internal/exposure/exposure.go:166-177` shows that reason. A label entry without a readable `label_id` still counts (`internal/ha/client.go:755-761` appends the empty id), so `plan.Check` refuses. `plan.Assess` has two callers of a fresh snapshot, the bridge's sync and `--dump-plan`; both call `plan.Check` first (`internal/bridge/bridge.go:461-464`, `cmd/ha2fabric/main.go:384-387`); the web UI only re-reads the bridge's last snapshot, which passed it. `ha.Client` is the only `ha.Source` outside tests, so no wrapper drops the entries returned beside a `*ha.SkippedError`. Areas and states stay lenient: the rule reads no area field, a missing state is "no state yet" |
| err-91 | yes | M | `internal/selection/selection.go:152-157`, `:166-193`. Probe: after a restore the second start reads the file, same entries, no Recovery; a write-back that fails (read-only directory) keeps the restored entries and the Recovery. A selection cleared in the web UI is a file with no entities — it parses, the copy is not read: `restart after clear: entries=map[] recovered=false`. See HR-P1-err-99 for the file deleted by hand |
| ha-92 | yes | M | `internal/ha/client.go:294-296`; the back-off's reset stays in `Run`. `TestClientAuthRefusedThenAccepted` green 3× under `-race` |
| conc-93 | yes | I + M | `internal/bridge/bridge.go:379-390`, `:458-460`, `:520-525`. A forget is kept only when Diff dropped it (its key still planned) and it was noted after the read began; it is then older than the next sync's read, so it is kept at most once. The remove event itself schedules that next sync. A removal noted before the read keeps the old rule (the entity is back). Test green 3× under `-race` |
| err-94 | yes | I + M | `cmd/ha2fabric/nodes.go:277-289`, `:311-314`, `:333-335`. The web server's shutdown is bounded at 5 s (`internal/web/web.go:134`), so `close` takes `op` by `lockBy` at the latest when the grace (20 s) ends, and has 5 s for a node the reset left running; a reset that succeeded leaves nothing to stop and starts no run. The deferred `AfterFunc` stop and `cancel` leave no goroutine. Stop and erase tests green 2× under `-race`. Go-fabric's reaction to the cancelled context was not re-read (I) |
| cfg-95 | yes | M (read) | `Recovery.Empty` only for a file of another version without entities; a version 1 file with no entities is no Recovery |
| conc-96 | yes | I + M | `internal/bridge/bridge.go:244-247`, `:267-273`: the mark is stored before the token is sent; a worker that takes the token with work queued finds the mark after its last function, one that waits takes the token; a token left over finds no mark and does nothing. Queue tests green 3× under `-race` |
| red guard | yes | M (read) | The fake counts the first read; the bridge's code is as before |

### Denial of service by entry data (data-90)

Two things now fail the whole sync: an entity or device entry whose id
cannot be read, and any label entry that does not decode while label
values are configured. A failed sync is fail-static, as a failed read
always was: the endpoints served stay as they were, nothing new is
exposed, `bridge.sync_failed` and the status' `sync_error` say why, and
the registry event of the repair starts the next sync. Before F1 any
entry that did not decode had this effect, so round 2 narrows it. The
ids are Home Assistant's own, and field types are set by code running
inside Home Assistant, not by a device's payload (I, core not re-read
for this); who can do that needs no bridge to do harm. **Acceptable.**
The cost that remains: while such a sync fails, a change that should
take an entity away (hidden, an exclude label) waits too.

### New rows

| ID | Severity | Where | M/I | What | Fix |
| --- | --- | --- | --- | --- | --- |
| HR-P1-err-99 | low | `internal/selection/selection.go:166-183`; `docs/operations.md:61-63` | M | Deleting `selection.json` by hand no longer resets the overrides: the last-good copy is restored, with its "turned on" entries, so those entities are exposed again. Loud at that start (`selection.missing`, status page), silent from the next one, when the restored file is the file. Probe: `restart after rm selection.json: entries=map[light.a:{true } light.b:{false }] recovered=true`. The docs say the restore happens, not how to start without overrides | One sentence in `docs/operations.md` (Files): to start without overrides clear them in the web UI, or delete `selection.json` and `selection.json.bak` together |
| HR-P1-data-100 | note | `internal/plan/plan.go:338-345`, `internal/bridge/bridge.go:367` | M | `RemovedKey` reads only `Snapshot.Entities`; an entity removed from the registry while its entry was unread in the last snapshot gets no forget, so created again it has its old endpoint number. Probe: `RemovedKey(light.a)=<nil>`. No exposure | None now, or keep a skipped entry's registry id beside its entity id |
| HR-P1-err-101 | note | `cmd/ha2fabric/nodes.go:272-289` | I | `erase` refuses only when `s.closed`; one that takes `op` after the daemon's base ended and before `close` does still resets, with a grace counted from its own start, which can end after the stop budget. Needs a request already inside the handler at the signal | `if s.closed \|\| s.base.Err() != nil { return errStopping }` |
| HR-P1-err-102 | note | `cmd/ha2fabric/nodes.go:713-728`, `:786-790` | I | A registry node's erase holds the fleet's `op` for its length, so a reset that hangs at a stop keeps every registry node's stop waiting up to 20 s and leaves them 5 s together. Only with a reset that takes longer than about 15 s | None; within decision 29 |

`plan.Check` also refuses for a blank label value (`INCLUDE_LABELS=" "`)
and for a skipped label the rule does not name: stricter than needed,
closed, no row.

### Changelog, docs, commit messages

- `changelog.md` (0.7.0, F1's lines): refused token "clears as soon as
  Home Assistant accepts the token again"; the skipped entry sentence;
  `selection.json` "read instead and written back … also when
  `selection.json` is missing … or nothing in the file is understood";
  the stop's "up to 20 of those seconds and does not bring the node
  back" — each matches the code (M, read).
- `docs/operations.md`: the skipped-entry paragraph (including "the web
  UI shows why" and "so is an area"), the `selection.json` paragraph, the
  rows `ha.entry_skipped`, `selection.missing`,
  `selection.restore_failed`, and the erase and stop paragraphs match the
  code (M, read). `ha2fabric.node_erased` is at warn when the reset
  fails, as written.
- The eleven commit messages `5f86ed3..HEAD` are one subject line each;
  none speaks of the held row, and no added tracked line does. No attribution lines.

### Not done here

The full guard set was not run again (the fix report has it green at
`4765b95`); the probes and the tests named above ran in the copy only.

## Before the push

1. Nothing must change.
2. Suggested with the merge: the docs sentence of HR-P1-err-99; the one
   line of HR-P1-err-101.
3. Register rows for HR-P1-data-100 and HR-P1-err-102 (accepted).
