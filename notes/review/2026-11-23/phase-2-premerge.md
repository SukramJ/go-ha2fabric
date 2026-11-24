# Phase 2 fix wave — pre-merge read (Opus)

**Verdict: not ready — blocking HR-P2-bound-90** (an owner's call on
decision 42: fix it, or accept it as decision 42's intent with one docs
sentence; either way small). Everything else is low or note and does not
block.

Read: the brief `notes/briefs/2026-11-24-review-p2-fix.md`, decisions
35–51, `phase-2-fix-F1.md`, `phase-2-fix-F2.md`, `phase-2-fix-round2.md`,
and the diff `d6003b4..a4a15bb` without `notes/`, on baseline `2324fbf`.
Lines are at `a4a15bb`. Probes in a `git archive a4a15bb` copy
(`…/scratchpad/PM/src`, files `internal/plan/pm_probe_test.go`,
`internal/bridge/pm_probe_test.go`, `internal/bridge/pm_export_test.go`).
I ran nothing in the worktree and changed nothing there except this file.
M = read or probe ran, I = inferred.

## Rows

| ID | Severity | What |
| --- | --- | --- |
| HR-P2-bound-90 | medium | An oversized state is mapped from an empty description: the device type and the capabilities change, and crossing the cap changes the endpoint again |
| HR-P2-data-91 | low | data-35: a refused entity still counts as served, so a command queued before the rebuild passes the HR-P1-conc-33 guard |
| HR-P2-doc-92 | low | `bridge.sync_failed` and the `sync_failed` warning claim more than the code does when the batch fails |
| HR-P2-doc-93 | note | The reads table's "once for every node on the connection" reads as one read per node |
| HR-P2-data-94 | note | data-31: a registry name made only of unsafe characters names the entity after its device; labels in `nodes.json` are not checked at load |
| HR-P2-test-95 | note | err-63's test checks that an apply is queued, not that the attribute comes back |
| HR-P2-data-96 | note | data-35's riders are dropped with the refused entity: accepted (the answer to round 2's question 1) |

### HR-P2-bound-90 — medium — an oversized state builds a different device

- **Where.** `internal/ha/message.go:171-191` (attributes dropped),
  `internal/plan/plan.go:189-193` (only a reason is set; the entity is
  still planned), `internal/bridge/bridge.go:1064-1067` (a `state_changed`
  whose description moved re-syncs and applies the state that has no
  attributes).
- **Claim.** Decision 42 drops the attributes and adds a reason, but the
  entity is still mapped, and go-hamapping reads its whole description
  (`device_class`, `supported_features`, modes, bounds) from those
  attributes. The fixtures with every state stripped give these results
  (probe `TestPMProbeOversized`, `plan.Dry`):
  - `climate.bedroom_ac` changes from air_conditioner `0x72` to thermostat `0x301`.
  - `light.kitchen` changes from extended_color `0x10d` to onoff `0x100`.
  - Both media players become unmappable (they lose their endpoint).
  - The covers stay window_covering, with `supported_features` 0.

  Exposure does not widen: `Exposed` is the same in both runs, because the
  rule reads the registry and the Assist list, not attributes. The
  commands a controller then sends are ones Home Assistant refuses (a
  service without its required feature, a setpoint outside the real
  bounds), and err-63 sets the state back. So no wrong physical action was
  found (I). What does change is the device type, its clusters and the
  controller's view of them. A state that crosses the cap and back changes
  the description twice. Each change re-syncs and **Replaces** the endpoint
  under a different device type, so a controller may re-create the
  accessory each time (I).
- **M/I.** M for the mapping (probe), I for the Replace churn (read).
- **Fix.** Pick one (the owner decides):
  - (a) Fail closed. In `plan.Assess`, an exposed entity whose state is
    `Oversized` is unmappable with `OversizedReason`, and it is not
    grouped. The endpoint then goes away while the state is over the cap
    and comes back with its stored number, never as another device type.
  - (b) Keep the last whole description of a served entity and apply only
    `state` and availability from an oversized one.

  (a) is about five lines. (b) is the better result for the operator but
  needs state kept per entity. Or accept the current behaviour as decision
  42's intent and add one sentence after the `ha.state_oversized` row of
  `docs/operations.md`: "the entity is mapped without them, as a simpler
  device."
- **Red test.** `plan.Assess` on the fixture snapshot with
  `climate.bedroom_ac` oversized. For (a), its verdict is unmappable and no
  group carries it. For (b), a bridge e2e test in which an oversized
  `state_changed` for a served entity causes no Replace.

### HR-P2-data-91 — low — a refused entity counts as served

- **Where.** `internal/bridge/bridge.go:740-747` puts every
  `g.Entities` key into `serving` once `len(g.devices) > 0`. The
  `plan.Group` keeps the refused entity on purpose (round 2 report).
  `internal/bridge/command.go:89` is the guard of HR-P1-conc-33, and
  `bridge.go:216-218` documents `serving` as "served on an endpoint".
- **Claim.** After data-35, `serving["climate.hall"]` is true while
  `climate.hall` has no endpoint (probe `TestPMRefusedIsServing`:
  `serving[climate.hall]=true`). Suppose a controller's command for an
  entity was queued while the old device served it, and the sync that
  runs before it rebuilds the device with that entity refused. The command
  passes the guard and becomes a `call_service` for an entity the bridge
  no longer serves. Before data-35, the whole device had no devices and
  the guard dropped the command. The window is narrow, and the command
  was a controller's real one, so there is no widening beyond that.
- **M/I.** M (probe), I for the race.
- **Fix.** Build `serving` from what the plan built:
  `g.plan.DeviceFor(e.Key()) != nil`, or skip the keys whose merged
  verdict is unmappable.
- **Red test.** The probe as a test: after `TestRefusedEntityKeepsItsSiblings`'
  setup, `serving` holds `switch.coffee` and not `climate.hall`. Also
  `command()` for `climate.hall` gives `bridge.command_unserved` and
  makes no call.

### HR-P2-doc-92 — low — what `sync_failed` says when the batch failed

- **Where.** `docs/operations.md:274` (`bridge.sync_failed`: "could not
  read Home Assistant or apply its plan … the endpoints stay as the last
  sync that succeeded left them"). `internal/exposure/exposure.go:341`,
  `:363` (the warning's text). `internal/bridge/bridge.go:683` and `:777`.
- **Claim.** A failed batch counts as a failed sync for the retry and for
  the warning, which F1 lists as a difference from the brief. But it is
  logged as `bridge.batch`, never as `bridge.sync_failed`, and the
  operations of the batch that succeeded are applied (a batch is not a
  transaction, `bridge.go:660-662`). So "the endpoints stay as the last
  sync that succeeded left them" is wrong for that path, and the log row
  names a line that is not written.
- **M/I.** M (read).
- **Fix.** In the docs row: "could not read Home Assistant
  (`bridge.sync_failed`) or go-fabric refused part of its batch
  (`bridge.batch`)". In the warning: "the endpoints stay as the last sync
  left them; what failed is tried again".
- **Red test.** None (docs). Optionally, `TestProviderSyncFailed` with a
  batch error asserts the message names no "last sync that succeeded".

### HR-P2-doc-93 — note — "once for every node on the connection"

`docs/operations.md:212`: the reads table says the lists are read "each
connection and each registry change, once for every node on the
connection". The natural reading is one read per node, which is what
bound-62 removed. Suggested wording: "…, one read shared by every node on
the connection". M (read).

### HR-P2-data-94 — note — data-31 edges

- `internal/adapt/adapt.go:165-166`: when a registry `name` holds only
  unsafe characters, it cleans to "", and the entity is then named after
  its device. It does not fall through to `original_name` or
  `friendly_name`. Harmless; a one-line fix is to clean first and fall
  through on "".
- `cmd/ha2fabric/nodes.go:552` `nodeLabel` only cuts: a label already in
  `nodes.json` from an older version is not checked or cleaned at load,
  and it becomes the NodeLabel (F2 saw this too). Clean it with
  `text.Clean` at load, or accept it, since a file this version writes
  cannot hold one.

Nothing collides: names are not keys. Every refusal path wraps the right
error: `nodes` wraps `fabricui.ErrInvalidArgument`, and `selection.Set`
refuses before it locks. M (read).

### HR-P2-test-95 — note — err-63's test checks the queue only

`internal/bridge/sync_internal_test.go:205-223` asserts that work was
queued or a sync owed. It does not assert that the attribute is back. The
fix is still correct: go-hamapping v0.7.0's generated `apply` calls
`Set(p)` unconditionally (`rows_gen.go:159-167`), so the queued apply does
overwrite the controller's value (M, read). A stronger test would check
the attribute after the apply runs: an e2e test with a full command queue
and the endpoint's attribute read back. Accept as is, or strengthen.

### HR-P2-data-96 — note — the riders of a refused entity

**Answer to round 2's question 1: dropping the riders with the refused
entity is right. Do not retry them.** In go-hamapping v0.7.0 every rider
rides on `Endpoints[0]` (`plan.go:232-245`), and `BuildError` does not
say whose declaration go-fabric refused. Suppose a rider's own
declaration is the cause and the rider is kept. It moves to the next
primary, which then fails with that primary's key. The loop drops a sound
primary in each round until no primary is left, so one faulty rider
could take every sound endpoint of the device. Dropping the riders loses
at most the refused endpoint and what rides on it, which is typically a
battery or measurement of the same physical thing. The loop is bounded
either way (`bridge.go:972-990`: each round removes at least one entity,
and a round that removes none falls back to the old whole-device
failure). The better cut belongs to go-hamapping: a `BuildError` that
names the binding whose declaration was refused. That is a go-hamapping
issue candidate, not work for this pull request.

## The focus items, checked

1. **Fail-open or exposure widening.** data-35's loop is bounded by the
   group's entity count. A refused entity or rider has no binding in the
   plan that is kept: `Apply` returns nil, `DeviceFor` is nil, no
   endpoint, no command source. Its verdict is unmappable with
   go-fabric's reason, and a group with an unchanged signature keeps it
   through `old.verdicts`. One gap: `serving` (HR-P2-data-91). bound-38
   does not widen exposure, but it does change the device type
   (HR-P2-bound-90). data-31 is in HR-P2-data-94. M.
2. **Concurrency.** Each of these was checked by reading; the race run is
   noted on the first item.
   - **Shared snapshot.** I found no write into a snapshot. `Assess`
     clones the label slices, `adapt.State` clones the attributes,
     `sentRangeLocked` clones, `b.states` only points into the snapshot,
     and data-31 builds new strings. `go test -race ./internal/ha -run
     'Snapshots|Fanout'` is green (M).
   - **Single-flight.** The read is detached, the wait is not, and a
     failed flight is shared only with callers whose trigger came before
     it began. Correct.
   - **Debounce and retry.** `armed` makes a replaced timer do nothing.
     `retryLater` yields to a pending timer. `retry` lives on the worker
     only and `failures` under `mu`. No retry runs while disconnected.
   - **`stopRun` and the camera close.** After the deadline, `Close`'s
     `ectx` is cancelled, so the parallel `End`/`Unsubscribe` calls and
     the waiter goroutine end with it. `closeAll` runs on the cancelled
     worker ctx, so its bound is `SignalTimeout` (10 s), as the docs say.
     A worker that overran ends on its own within that.
   - **`Store.Close` against `Set`.** `Close` takes `mu`, which `Set`
     holds while it writes. `Delete` closes the store before
     `RemoveFiles`.
   - **The Supervisor gate.** State is atomic, and version and since are
     under `mu`. A read that fails while the gate is open leaves it open,
     as documented.
3. **err-63 and bound-64.** The restore runs on all three paths. A full
   work queue owes a sync instead, which applies every state. `dropLog`
   writes one line at the first drop, then a count every interval, and
   goes quiet after an empty interval. At most one timer outlives the
   bridge. Correct (M, read). The test gap is HR-P2-test-95.
4. **Tilt.** `MethodOpenTilt` → `open_cover_tilt` and `MethodCloseTilt` →
   `close_cover_tilt`, without data, match go-hamapping v0.7.0's "What a
   host must do". `MethodTilt` → `set_cover_tilt_position` was already
   there. The citation `cover/services.py:72-82` at 2026.10.3 spans
   `SERVICE_OPEN_COVER_TILT` to `[CoverEntityFeature.CLOSE_TILT]` (read
   from the tag; it is Fable's to confirm as a reason citation). The
   fixtures `cover.tilt_blind` (255, blind) and `cover.tilt_slats` (240,
   tilt only) are the brief's. M.
5. **Docs and changelog against the code.** Every new claim in
   `docs/operations.md` and `changelog.md` was checked against the code:
   - retry 5 s doubling to 5 min, warning from the 2nd failure: correct;
   - debounce latest time = 5 × 1 s: correct;
   - 256 KiB per state, 64 MiB read limit, 512-byte cut: correct;
   - 256 candidates per session, 1 KiB per candidate: correct;
   - 10 s per camera session: correct;
   - Supervisor re-check every 10 minutes: correct;
   - `bridge.build_refused` and `bridge.build_failed` narrowed: correct;
   - tilt at 0 % (decision 48): correct.

   Two exceptions: HR-P2-doc-92 and HR-P2-doc-93. The `ha.state_oversized`
   row is true as far as it goes; HR-P2-bound-90 is what it leaves out.
6. **Riders.** See HR-P2-data-96.

### Held-hint scan (decisions 21, 24, 37)

The diff adds no sentence about HR-P1-auth-01's or HR-P0-auth-20's
condition, in code, docs, changelog, notes or commit messages (`git log
d6003b4..a4a15bb` searched). Tracked lines that touch the subject, all
clean:
- `docs/operations.md:128-134` (net-01, new) argues against a LAN peer
  that forges the proxy's address. It says nothing about who else is on
  the Supervisor's network. It is adjacent, as decision 43 ordered, and
  is not the condition.
- `docs/operations.md:449-460` and `changelog.md:41-50` describe the
  check as it is (decision 21).
- `cmd/ha2fabric/admin_test.go:123` predates this wave and is a check
  row.
- The ID-only mentions in `notes/` (`decisions.md:38,43,51,64,96`,
  `phase-1-S.md:14,71,298,869`, `phase-2-X1.md:18,153-158`, the verify
  and register files) are unchanged since phase 2's verifier read them.

Nothing held-worthy came up, so there is no `held/phase-2-premerge.md`.

## Re-read of round 3

**Verdict: ready to merge.** One new note (HR-P2-bound-97), accept;
nothing blocks.

Read: `phase-2-fix-round3.md`, decision 52, and the diff `9b274b3..f851339`
without `notes/`. Lines are at `f851339`. Probes in a `git archive f851339`
copy (`…/scratchpad/PM2/src`): `TestOversizedStateTakesTheEndpoint`,
`TestRefusedEntityIsNotServed`, `TestDroppedCommandAttributeBack`,
`TestLabelCheckedAtLoad`, `TestUnsafeOnlyRegistryName` and
`TestOversizedUnmappable` green (M). Nothing run in the worktree, nothing
changed there except this section.

| ID | Severity | What |
| --- | --- | --- |
| HR-P2-bound-97 | note | A state that flaps across the 256 KiB cap removes and re-adds its endpoint once per crossing, at most about once per debounce; bounded, accept |

### HR-P2-bound-97 — note — a state flapping around the cap

- **Where.** `internal/bridge/bridge.go:1080-1090` (`hollow` re-syncs under
  the cap, a served entity re-syncs over it), `:451-468` (`resyncSoon`,
  `maxWait`), `:625-635` (`hollow` built from the snapshot's `Oversized`
  and the verdicts' `Exposed`).
- **Claim.** Every crossing in either direction costs one full sync and
  one endpoint removal or addition that a controller sees. It is not a
  storm (M, read):
  - an entity that **stays** over the cap triggers nothing after the
    first sync: it is no longer owned (`g == nil`), and `hollow` re-syncs
    only on `Oversized == 0`. Many oversized events of one entity cost a
    Debug line each (`logOversized` is Warn once per entity, and the
    client's `ha.state_oversized` is too, `internal/ha/client.go:699-702`)
    and no sync;
  - a fast flap (crossings closer than the debounce) coalesces: the timer
    is re-armed but capped at `first + 5 × Debounce` (data-30), so one sync
    per 5 s at most;
  - a slow flap (crossings more than one debounce apart) is one sync per
    crossing, so at worst about one per debounce (1 s). That is the same
    bound as the paths that already re-sync per event (a moved
    description, an unknown entity, a removed state); the syncs run on the
    one worker in turn, the read is the shared single-flight reader
    (bound-62), and a full work queue owes the sync rather than queueing
    another (`fire`). Other work is delayed by the syncs, not starved: the
    worker takes the queue in order.
- **Before round 3** the same flap re-typed the endpoint on each crossing
  (bound-90's last clause), so the churn is not new; only its form is.
- **Fix, if wanted.** A hysteresis for `hollow` (for example stay
  unmappable until a state under the cap has held for some debounces).
  Not for this pull request; a follow-up candidate if a real entity shows
  it.

### The focus items, checked

1. **The endpoint number is kept** (M). `TestOversizedStateTakesTheEndpoint`
   takes the endpoint, crosses back under the cap and asserts the same
   number; the recovery is the `hollow` re-sync, as the report says (it
   saw the test red with that re-sync off). The number comes from the
   node's stored numbering by stable key, which the removal does not
   forget (only a registry removal does, `forget`).
2. **The served entity's oversized `state_changed`** (M, read). It is
   written into `b.states` before the `Oversized` check
   (`bridge.go:1066-1072`), so until the sync a `restore` or a command
   reads the stripped state. Harmless: go-hamapping v0.7.0's generated
   `apply` sets only what the state carries (a missing attribute leaves
   `n == 0` or skips its case, e.g. `rows_gen.go:2905-2919`), so the state
   string alone is applied, which is current; `adapt.Service` refuses a
   range command without the other bound (`ErrCommand`, restore). Not a
   finding.
3. **`hollow` against a failed sync** (I). It is rebuilt at each read
   before the batch; if the batch fails, `owner` still holds the old
   group, so the entity takes the `g != nil` path and the failed sync's
   retry (err-61) brings it to the plan. No lost recovery.
4. **data-91's served set** (M). `serving` has one reader,
   `command.go:89` (the HR-P1-conc-33 guard), and one writer, the sync
   (`bridge.go:752-766`), now from `g.plan.DeviceFor(key) != nil` and
   skipping a group without a plan. `restore` and `apply` go by `owner`,
   which still holds a refused entity, but `apply` on it reaches
   `plan.Apply` with no binding (nil, nothing set). Right at every use.
5. **data-94 at load** (M). `nodes.Open` cleans a refused label and logs
   `nodes.label_refused` at Warn; it returns no error and does not rewrite
   the file. A label that cleans to "" stays "" (no code path treats an
   empty label as an error; `nodeLabel` only cuts). Not fatal.
6. **doc-92, doc-93, test-95** read against the code: correct.

Held-hint scan of `9b274b3..f851339` (code, docs, changelog, commit
messages): nothing about HR-P1-auth-01's or HR-P0-auth-20's condition.
