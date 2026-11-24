# Phase 2 — verification

Verifier (Fable). Code at `2324fbf`. Brief
`notes/briefs/2026-11-24-review-p2.md` ("Verifier"). Read: the three
reports (`phase-2-X1.md`, `phase-2-X2.md`, `phase-2-X3.md`) and the two
held reports; then the code and the references again for everything a
verdict below rests on. Prefixes as in the reports (`gf:` go-fabric
v0.25.0, `hm:` go-hamapping v0.6.0, `ui:`, `sup:`). **M** = the line was
read at its pin by the verifier, or the verifier's run of the probe
printed the quoted line; **I** = inferred. Probes ran in the verifier's
own `git archive 2324fbf` copy (`…/scratchpad/V/src`), `go test -count=1
-run … -timeout …`, from the reports' appendix texts unchanged. No
container was run and nothing ran against a live system.

Held rows are named by ID, severity and verdict only; their details are
in `held/phase-2-verify.md` (git-excluded).

## 1. Verdicts

Every medium, the held measurement, and the sample of lows (X2: 3 of 9
read, 4 more by their probe; X3: 2 of 4 read, 1 more by its probe; X1 has
no low, one note checked). 0 struck; 1 medium → low; 0 merged (three
groups are cross-referenced for one fix, section 3).

| ID | Unit | Reviewer | Verified | Verdict | Held |
| --- | --- | --- | --- | --- | --- |
| HR-P1-auth-01 | X1 (measurement) | medium, "measured, confirmed" | medium | see the held file | yes |
| HR-P2-data-30 | X2 | medium | medium | confirmed | no (published, decision 36) |
| HR-P2-adpt-33 | X2 | medium | medium | confirmed at both pins; upstream; not a regression of 0.7.0 | no |
| HR-P2-conc-60 | X3 | medium | low | re-graded; a gap of HR-P1-err-64's fix, not a regression | no |
| HR-P2-data-35 | X2 | low | low | confirmed | no |
| HR-P2-bound-38 | X2 | low | low | confirmed | no |
| HR-P2-data-32 | X2 | low | low | confirmed (probe) | no |
| HR-P2-err-61 | X3 | low | low | confirmed | no |
| HR-P2-err-63 | X3 | low | low | confirmed | no |
| HR-P2-cfg-03 | X1 | note | note | confirmed | no |

Reproduced by the probe only (the lines not re-read, severity the
reviewer's): X2 `adpt-34`, `bound-37`, `bound-39`, `bound-40`; X3
`bound-62`. Not sampled, severity the reviewer's: X2 `data-31`,
`test-42`; X3 `bound-64`; every other note.

Counts after verification: **0 high, 2 medium** (data-30, published by decision 36; adpt-33),
**14 low** (X2 9, X3 5), **12 note** (X1 4, X2 2, X3 6). Beside them
phase 1's held HR-P1-auth-01, medium, unchanged.

## 2. The rows

### HR-P1-auth-01 — medium — held

`held/phase-2-verify.md`: what the measurement shows, what it does not,
and the publication question.

### HR-P2-data-30 — medium — confirmed (published by decision 36)

### The read and the probe

- M (read): the three registry events and two cases of a state event
  call `resyncSoon` (`internal/bridge/bridge.go:338-349`, `:841-843`,
  `:849-851`); `resyncSoon` stops the pending timer and arms a new one
  on every call (`:394-407`), 1 s by default (`:197-198`); no other
  caller of a sync exists beside the connection's handlers, the owed
  mark and `Resync` (`:242`, `:246`, `:283`, `:295`, `:767`); a command
  is checked against `b.serving`, which only a sync writes
  (`internal/bridge/command.go:72-79`, `bridge.go:605`).
- M (probe `TestX2ProbeResyncStarved`, verifier's run): `switch.coffee,
  disabled, kept its endpoint for 5 s (25 debounces) while sensor.x2_new
  reported every 50 ms; last sync 5.562s before the flood ended`.
- One narrowing of X2's "who can make the stream": every entity of the
  last snapshot is known, exposed or not — `Assess` writes a verdict for
  each id (`internal/plan/plan.go:158-198`) and `known` is the verdicts'
  ids (`bridge.go:487-490`). So an ordinary chatty sensor that existed
  at the last sync does not do it; only entities that appeared since
  (they stay unknown because the sync they ask for is the one they put
  off), or a served entity whose description keeps differing. The rates
  of real integrations stay I.

### Severity

Medium holds on the functional half: a newly added integration whose
entities together report without a one-second gap is an ordinary event,
and from then on nothing changes on the bridge — not its own entities,
not any other registry change — without a line in the log, until a
reconnect, a restart or the web UI's reassemble. The security half
alone would be low: nothing becomes exposed that the rule did not
expose; what is delayed is a withdrawal from controllers the operator
paired (HR-P1-conc-33's reasoning, a note there), and an entity
disabled in Home Assistant no longer acts on a call anyway (I).

### Overlap with X3

Not a duplicate of any X3 row, but the same timer:

- X3 `err-61` proposes its retry "through `resyncSoon`'s timer" — under
  this row's condition that retry would be put off with everything
  else. The two fixes are one design: a pending sync has a latest time.
- X3 `bound-62` names the same triggers as a cost (one read per node
  per event); with this row fixed by a latest time, a continuous stream
  becomes a full read per node every `maxWait`, which makes `bound-62`'s
  shared reader matter more. Decide them together (question 5 of the
  tracked file, to which this row belongs once it is public).
- X2 `bound-37` and X3 `err-61` state the same visible consequence (a
  registry change not applied) from other causes; that is why their
  tracked sentences are not leaks.

### Should it be held?

No. Recommendation: **publish it now, with the reports' commit**, and
fix it in the phase 2 pull request.

1. The hold rule is for "an exploitable weakness" open to an untrusted
   role. Here the untrusted input gains no reach: it freezes the
   bridge's set as it is. It is a robustness defect with a security
   edge, the class of X3's `err-61` and X2's own `bound-37`, both
   public in the same reports.
2. Decision 23 published phase 1's camera lows for a weaker reason
   (their role was a commissioned controller); decision 18 published
   mediums that rest on public conditions. X2 itself leans this way
   (its question 6).
3. The cost of holding is real here: the fix must be designed with
   `err-61` and `bound-62` (above), whose briefs, tests and commit
   messages would have to avoid naming why the timer gets a latest
   time; the red test is the probe, which cannot be committed while the
   row is held.
4. Counter-argument, for the owner: until the fix merges, the row tells
   a reader how to keep an operator's exposure change from applying,
   with nothing but state reports. It needs a foothold in Home
   Assistant's entity data, is undone by a reassemble or a restart, and
   the fix is days away on an untagged main.

Options (question 2 of the tracked file): (a) public now, fixed in the
phase 2 pull request — **recommended**; (b) held until its fix merges,
then published with that pull request (X2's recommendation); (c) held
to the tag (decision 21's rule, made for a different kind of row).

If (b) or (c): the tracked verify file already says nothing; question 5
there must not list this row, and `err-61`'s brief must not explain the
latest time by it.

### HR-P2-adpt-33 — medium — confirmed, upstream

- M (read, go-hamapping v0.6.0): the row declares Lift always and Tilt
  when the entity has a tilt feature (`hm:rows/cover.yaml:26-30`,
  `hm:rows_gen.go:1427-1436` in the `declare` function); Type and
  EndProductType come from `device_class` alone, Rollershade for blind,
  shade and none, Drapery, Awning, Shutter (`hm:rows/cover.yaml:32-37`,
  `hm:rows_gen.go:1437-1460`). `Build` returns at the first refused
  endpoint (`hm:plan.go:402-405`).
- M (read, go-fabric v0.25.0): the server refuses a Type or
  EndProductType its features disallow
  (`gf:cluster/cover/windowcovering_server.go:385-399`, `ErrEnumValue`);
  Rollershade, Drapery and Awning are `LF & !TL`, Shutter
  `LF & !TL | !LF & TL`, only TiltBlindLift is `LF & TL`, Unknown is
  optional (`gf:cluster/spec/windowcovering/definition_gen.go:273-283`).
- M (probe `TestX2ProbeCoverBuild`, verifier's run): 36 lines
  `device_class … features …: mappable, refused: … cover: Type or
  EndProductType not allowed under the FeatureMap` (none, blind, shutter,
  curtain, awning, shade × 143, 240, 255, 128, 139, 191), then
  `36 cover shapes are mappable and do not build`.
- **Not a regression of 0.7.0.** The pins of the last tag `v0.6.3`
  (go-fabric v0.22.2, go-hamapping v0.4.12) carry the same check
  (`cluster/cover/windowcovering_server.go:377` there) and the same row
  (Tilt at `rows_gen.go:1395`, Rollershade for blind at `:1402-1418`
  there). I: read at those pins, not run at `v0.6.3`.
- Medium holds: no hostile value is needed, an ordinary kind of entity
  (a venetian blind) has no endpoint, the preview calls it mappable, and
  with HR-P2-data-35 its device's other entities go too. No security
  consequence.
- Release blocker: by regression, no — `v0.6.3` behaves the same (I). By
  cost, fix it before the tag anyway: go-hamapping is changed directly
  (its own pull request), the change is one row plus a "what Map calls
  mappable builds" check, and the pin lift fits the phase 2 fix wave.
  If go-hamapping's release slips, tag with a known-issue line in
  `changelog.md`; do not hold the tag for it. Question 1.
- The fix must also set EndProductType to a member allowed with both
  features (the generated conformance of that enum was not read by the
  verifier; go-hamapping's fix reads it).

### HR-P2-conc-60 — medium → low

- M (read): the read is right. `Close` ends its sessions in turn, each
  with `unsubscribe` and `sig.End` under `WithoutCancel` +
  `SignalTimeout` (`internal/camera/camera.go:358-385`, `:627-634`;
  10 s default `:292-294`); the worker calls it with the ended ctx
  (`internal/bridge/bridge.go:234-237`) and during a sync
  (`:560` → `internal/bridge/camera.go:59-70`); `stopRun` waits
  `<-s.done` without a ctx (`cmd/ha2fabric/nodes.go:181-185`), under
  `s.mu` in `close` (`:437-443`) and in `erase` (`:289-296`);
  `stopping.Wait()` has no bound (`cmd/ha2fabric/main.go:209-216`).
- M (probe `TestP2X3CloseHonoursItsContext`, verifier's run):
  `Close of 8 sessions took 1.6050339s (ctx deadline 100ms, SignalTimeout
  200ms)`.
- Two corrections. (1) Hop 4: `ConnectPeer` returns at once
  (`gf:node/peer_path.go:59-61`); the wait for a controller that is gone
  is in `sess.Invoke` (`gf:cluster/camera/provider_peer.go:171-173`),
  still I (not run). (2) "k × 20 s" is the ceiling: the unsubscribe
  waits only when Home Assistant is connected and does not answer; at a
  stop the client ends with the same cancel. The expected cost is one
  `SignalTimeout` per orphaned session, 80 s for a full camera.
- **A gap, not a regression.** The loop in `Close` and `stopRun`'s
  unbounded wait are the same at `5eefc0a` (`camera.go:285-311`,
  `nodes.go:143-147` there). Phase 1's fixes made the case smaller, not
  larger: before them a camera's sessions had no cap and no idle end;
  now at most 8, each gone after an idle hour (`camera.go:131`, `:138`).
  What phase 1 added is the promise: `docs/operations.md` ("one budget
  of 25 seconds") and decision 29 state a bound the bridge join does not
  keep. HR-P1-err-64's fix is incomplete on this path.
- Why low. The consequence at a stop is the one HR-P1-err-64 had for
  every stop (the Supervisor kills the add-on before the nodes'
  `Stop`), and that row was low. It needs a camera on the bridge with a
  session whose controller went away without ending it, inside the last
  hour. The two live paths (the worker held during a sync that drops a
  camera; an erase holding `s.mu`, so the status reads of that node
  wait, against `docs/operations.md`'s "An erase runs while the status
  pages keep answering") are I, bounded by the same sum and end by
  themselves. No role gains anything.
- Fix as X3, in this wave: the docs promise the budget. Question 4.

### HR-P2-data-35 — low — confirmed

M (read): one device is one group and one plan
(`internal/plan/plan.go:206-229`); on a build error every verdict of the
group that is not already unmappable becomes "go-fabric refused the
device" and the group has no devices
(`internal/bridge/bridge.go:785-799`). M (probe
`TestX2ProbeSiblingKnockedOut`): `switch.coffee: endpoint 0, unmappable:
go-fabric refused the device: hamapping: entity cover.living_blind …`.
Low holds by itself (it needs an entity the row lets through and
go-fabric refuses); its weight today is HR-P2-adpt-33, which makes the
trigger ordinary. Fix both in one wave.

### HR-P2-bound-38 — low — confirmed

M (read): the frame limit is 64 MiB (`internal/ha/client.go:58`), the
result is split into raw entries and each decoded
(`client.go:736-748`), attributes are `map[string]any`
(`internal/ha/message.go:129`), the bridge keeps every state in
`b.states` and the snapshot in `b.last`
(`internal/bridge/bridge.go:481-485`, `:609`). M (probe
`TestX2ProbeDecodeAmplification`, 14 s): `frame 60 MiB; heap in use
after decode, before GC 1143 MiB (total allocated 2949 MiB); kept by the
1571 states alone 1044 MiB`. Low holds: it needs a result near the limit
made of small values; the out-of-memory end on the add-on's hardware is
I.

### HR-P2-data-32 — low — confirmed

M (probe `TestX2ProbeOptionTrimmed`): `the controller chose "Eco ",
select_option carries "Eco"`; `the controller chose " Boost",
select_option carries "Boost"`. M (read): the trim is
`internal/adapt/command.go:97`. The option list is the entity's own
(`hm:options.go:124-146`). Low holds.

### HR-P2-err-61 — low — confirmed

M (read): a failed read logs, sets `SyncError` and returns
(`internal/bridge/bridge.go:461-468`); the only callers of a sync are
the connection's two handlers, the timer, the owed mark and `Resync`
(`:242`, `:246`, `:283`, `:295`, `:402`, `:767`). M (probe
`TestP2X3FailedSyncIsRetried`): `a failed sync while connected was not
tried again within 3 s: 1 read(s), status "get_states: fake: no
snapshot"`. Low holds.

### HR-P2-err-63 — low — confirmed

M (read): a full command queue drops with a log line and a counter
(`internal/bridge/command.go:39-46`), a refused `adapt.Service` returns
(`:86-91`), only a failed `CallService` enqueues the re-apply
(`:102-110`); the plan's callback runs after the change
(`hm:plan.go:532-539`). M (probe
`TestP2X3DroppedCommandRestoresTheState`): both subtests fail with the
report's lines. I, as X3 says: that the attribute shows the commanded
value. Low holds, at its lower edge: 256 queued commands or a mapping
gap.

### HR-P2-cfg-03 — note — confirmed

M (read): `run` returns once the gate is open
(`cmd/ha2fabric/supervisor.go:118-127`); its doc comment says so
("until the gate opens or ctx ends", `:113-115`), so the behaviour is
stated, not assumed. Note holds (the Supervisor is trusted, decision
11). The recheck after opening is optional.

## 3. Duplicates and one-fix groups

No row is struck as a duplicate; three groups share a fix and are
planned together.

| Group | Rows | Verdict |
| --- | --- | --- |
| A failing sync is neither tried again nor shown | X2 `bound-37` (its "no page says so" half), X3 `err-61`, `err-65` | Not one defect: `bound-37` is the cause over the frame limit and its own error text, `err-61` the missing retry, `err-65` a wrong success. One fix: the retry with back-off, `Resync` reporting whether it planned, and one web UI warning for a sync that keeps failing (X2's question 5), which serves all three. `bound-62` (one read per node) belongs to the same design. Question 5. |
| T15 | X2 `bound-39` (length), X3 `data-70` (quoting, note) | Complementary. X3's "no unquoted sink" is right for its scope; X2's `data-31` adds the one unquoted sink outside it (`--dump-plan`). X3's remark that `LastCommand` has no reader means `bound-39`'s status half costs memory only; the ring and stderr halves stand. `bound-64` (one line per drop) also fills the ring: fix beside `bound-39`. |
| The plan that does not build | X2 `adpt-33`, `adpt-34`, `data-35`, `test-42` | Four rows, one wave: the two upstream fixes, the partial device, and the fuzz target that found them as the standing guard. |

X3 `conc-69` and X2 `bound-40` both concern the camera session but
different directions; no overlap.

## 4. Leak check of the tracked reports

Sentences that say more about a held row than its ID and severity.
What each gives away is in the held file; here the lines only.

| File | Lines | Verdict |
| --- | --- | --- |
| `phase-2-X1.md` | 64-65 | **Leak** — reword before the commit (drop the last sentence of the claim) |
| `phase-2-X1.md` | 40 (fourth clause of the last cell), 54 (title), 207 (question's title) | Minor — reword to neutral terms |
| `phase-2-X1.md` | 154-158 | Acceptable under decision 21 (the check as it is); the row ID beside it is the register's own pairing |
| `phase-2-X2.md` | 518-520 | Minor — the comparison names a class; shorten to "publish with the fix or at the tag?" |
| `phase-2-X2.md` | 21, 37-39, 444, 448, 493 | Clean (ID and severity only) |
| `phase-2-X3.md` | — | Clean; lines 93-96 and 119-122 are X3's own public rows |

Nothing in the three tracked reports copies text of `held/*.md` of
phases 0 and 1.

## 5. Probes run by the verifier

| Probe | Result at `2324fbf` |
| --- | --- |
| `TestP2X3CloseHonoursItsContext` | FAIL, 1.605 s (conc-60) |
| `TestP2X3FailedSyncIsRetried` | FAIL, 1 read (err-61) |
| `TestP2X3OneEventOneSnapshot` | FAIL, 9 reads (bound-62) |
| `TestP2X3DroppedCommandRestoresTheState` | FAIL, both subtests (err-63) |
| `TestP2X3ReconnectLeavesNoGoroutine` | pass |
| `TestX2ProbeCoverBuild` | FAIL, 36 shapes (adpt-33) |
| `TestX2ProbeSiblingKnockedOut` | FAIL (data-35) |
| `TestX2Probe256Options` | FAIL (adpt-34) |
| `TestX2ProbeOversizedStates` | FAIL, `authenticated 8 times, last sync moved false` (bound-37) |
| `TestX2ProbeErrorTextInRing` | FAIL on length, 1 048 703 bytes (bound-39) |
| `TestX2ProbeOptionTrimmed` | FAIL (data-32) |
| `TestX2ProbeDecodeAmplification` | pass, a measurement (bound-38) |
| `TestX2ProbeHACandidatesUnbounded` | FAIL, 5000 candidates, no idle end (bound-40) |
| the held row's probe | see the held file |

Not run: `TestX2ProbeHostileText`, `TestX2ProbeConnections`,
`FuzzX2PlanBuild` (not in the sample; the machine is slow), X1's
`TestProbeX1WriteRoute` (its text is not in a report's appendix — the
fix wave cannot lift it from the report as the brief asks; X1 should
paste it), every container measurement (by the brief).

## 6. Not verified

- The Home Assistant side of any claim (`core:`): none of the sampled
  rows rests on one.
- That `v0.6.3` shows HR-P2-adpt-33 when run (read at its pins only).
- `sess.Invoke` to a controller that is gone waiting until its ctx ends
  (conc-60, I as in X3).
- The unsampled lows and notes.

## 7. Questions for the owner

The three reports' questions and the verifier's, the most important
first; the recommendation is the first option.

1. **HR-P2-adpt-33 and the `0.7.0` tag.** (a) Fix in go-hamapping now
   (Type and EndProductType by the features, and `maxModes` 255 for
   adpt-34), lift the pin in the phase 2 fix wave; if its release
   slips, tag with a known-issue line. (b) Tag with the known issue and
   fix later. (c) Hold the tag until the fix. Recommendation (a): it is
   no regression (`v0.6.3` has it, I), so it does not gate the tag, but
   it is the cheapest medium of the phase.
2. **(held) HR-P2-data-30: held or public.** Options and the
   recommendation in the held file.
3. **(held) HR-P1-auth-01: when it is published.** Options and the
   recommendation in the held file.
4. **HR-P2-conc-60, the camera close.** (a) Parallel ends bounded by
   the caller's deadline, `stopRun` bounded by the stop's ctx, and the
   worker's closes off the sync (X3's (a)+(b)+(c)). (b) Only (a)+(b),
   the sync-time wait left to go-fabric's "controller gone" signal
   (decision 34). Recommendation (a): the budget becomes a property of
   every join, which is what `docs/operations.md` already says. Accept
   the re-grade to low?
5. **The sync's scheduling as one design** (err-61, err-65, bound-62,
   bound-37's question). (a) One change: a retry with back-off after a
   failed sync, `Resync` reporting whether it planned, a web UI warning
   `sync_failed` with the read-limit error named, and one shared
   single-flight snapshot read per connection. (b) The retry and the
   warning now, the shared reader as an issue. (c) Row by row.
   Recommendation (a) if the fix wave has room, else (b); not (c) — the
   rows touch the same timer and would be changed twice.
6. **HR-P2-data-35, a refused entity and its device.** (a) A partial
   device: the sound entities served, the refused one unmappable with
   its reason; a typed build error upstream. (b) All or nothing, as
   today. Recommendation (a).
7. **HR-P2-data-31 / HR-P1-data-36, control and bidi characters.**
   (a) Drop them in `internal/adapt` (keep the joiners), refuse them in
   a web UI name and a registry node's label, quote `--dump-plan`; the
   option label as a go-hamapping candidate. (b) Refuse such a name and
   fall back to the entity id. Recommendation (a), as X2.
8. **HR-P2-bound-38, a cap per state.** (a) 256 KiB of raw JSON; over
   it the state is kept without attributes, logged once with its entity
   id, with a verdict reason. (b) No cap, document the read limit.
   Recommendation (a).
9. **The live Ingress test's list** (X1's questions 1 and 2). Add: the
   frontend frames the panel from the panel's own origin; the source
   address the daemon sees under `host_network`; HAOS's `rp_filter`;
   and the held file's items. (a) Add all. (b) Framing and source
   address only. Recommendation (a); no code change expected.
10. **Two small ones.** HR-P2-cfg-03: recheck the Supervisor's version
    after the gate opened — (a) yes, low priority, (b) leave it, the
    comment states it; recommendation (a). HR-P2-doc-66: (a) a doc
    sentence now and an issue for a persisted served-key set, (b) the
    sentence only; recommendation (a), the forget-on-event design is
    the owner's F1 decision.
