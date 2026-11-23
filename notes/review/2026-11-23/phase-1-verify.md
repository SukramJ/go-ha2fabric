# Phase 1 — verification

Verifier (Fable). Code at `5eefc0a`. Brief
`notes/briefs/2026-11-23-review-p1.md` ("Verifier"). Read: the three
reports (`phase-1-S.md`, `phase-1-GA.md`, `phase-1-GB.md`) and
`held/phase-1-S.md`; then the code and the references again, line by line
for everything a verdict below rests on. Prefixes as in the reports
(`gf:`, `hm:`, `core:`, `sup:`). **M** = the line was read at its pin by
the verifier, or the verifier's probe ran; **I** = inferred.

Held rows are named by ID and verdict only; their details are in
`held/phase-1-verify.md` (git-excluded). At the time of this read none of
the three reports is committed, so no wording has been published yet.

## 1. Verdicts

Every medium, the three duplicate pairs, and the sample of lows (S: 3 of
13, G-A: 2 of 6, G-B: 2 of 4). 0 struck; 1 medium → low; 2 low → note;
3 rows merged into their duplicates.

| ID | Unit | Reviewer | Verified | Verdict | Held |
| --- | --- | --- | --- | --- | --- |
| HR-P1-auth-01 | S | medium (I) | medium (I) | confirmed | yes |
| HR-P1-adpt-30 | G-A | medium | medium | confirmed | no |
| HR-P1-conc-60 | G-B | medium | low | merged into HR-P1-conc-32 | no |
| HR-P1-conc-32 | G-A | low | low | confirmed; survives, takes conc-60's probe and fix | no |
| HR-P1-err-06 | S | low | low | confirmed; survives | no |
| HR-P1-cfg-35 | G-A | low | low | merged into HR-P1-err-06 | no |
| HR-P1-ha-62 | G-B | low | low | confirmed; survives, HR-P1-ha-04 merged into it | no (decision 23) |
| HR-P1-ha-04 | S | low | low | merged into HR-P1-ha-62 (one defect) | no (decision 23) |
| HR-P1-bound-02 | S | low | low | confirmed | no (decision 23) |
| HR-P1-ha-08 | S | low | low | confirmed | no |
| HR-P1-err-31 | G-A | low | low | confirmed | no |
| HR-P1-conc-33 | G-A | low | note | re-graded | no |
| HR-P1-err-61 | G-B | low | note | re-graded | no |
| HR-P1-err-64 | G-B | low | low | confirmed | no |

Not sampled, severity the reviewer's: S `conc-03`, `bound-17` (both in
section 3 for the hold question only), `err-05`, `ha-07`, `data-09`,
`cfg-10`, `cfg-12`, `test-16`, `sup-18`; G-A `cfg-34`, `test-38`; G-B
`test-70`; every note.

Counts after verification, duplicates removed (with the verifier's
recommendation of question 1): **0 high, 2 medium** (auth-01, adpt-30),
**19 low** (S 12, G-A 4, G-B 3), **20 note** (S 8, G-A 4, G-B 8).

## 2. The rows

### HR-P1-auth-01 — medium (I) — confirmed — held

`held/phase-1-verify.md`: which part is measured, which inferred, and why
medium holds.

### HR-P1-adpt-30 — medium — confirmed

- M (read): in Auto, a write of OccupiedHeatingSetpoint is the command
  `temperature_low` and one of OccupiedCoolingSetpoint is
  `temperature_high`, for a write and for a change alike
  (`hm:rows/climate.yaml:93-96`, `:101-104`, `:113-120`;
  `hm:rows_gen.go:818`, `:820`, `:823-824`). `adapt.Service` takes the
  other bound from `cur` (`internal/adapt/command.go:134-149`), which is
  `b.states[id]` (`internal/bridge/command.go:58-61`), written only by
  the worker on a sync or a `state_changed`
  (`internal/bridge/bridge.go:387-391`, `:645-652`). The command loop
  runs the next command as soon as `CallService` returned
  (`command.go:45-54`, `:69`); nothing ties the two.
- M (probe V2, `internal/adapt/probe_v_test.go` `TestProbeVRange`):
  `PROBE V2a: first map[target_temp_high:24 target_temp_low:20] <nil>; second map[target_temp_high:26 target_temp_low:18] <nil>`;
  `PROBE V2b: first map[target_temp_high:24 target_temp_low:25] <nil>; second map[target_temp_high:28 target_temp_low:18] <nil>`.
- M (read): Home Assistant refuses a call whose low is above its high
  (`core:homeassistant/components/climate/services.py:121-131`), and the
  two keys are `Inclusive` (`:54-59`), so the other bound must be sent.
- Case (b) needs no race: a range moved up past the old high (V2b) has
  its first call refused, and the second call sends the old low. The new
  low is lost whenever the state is as it was before the write. Case (a)
  is a race against Home Assistant's `state_changed`. I: an integration
  that writes its state inside the service call has the event on the
  wire before the result, and the worker usually wins; a thermostat
  whose state follows a device report or a cloud poll loses the first
  bound each time.
- Why medium holds: a controller's two-setpoint change in Auto is the
  ordinary way to move a range; the thermostat ends on a setpoint the
  user did not choose, and the controller was told both writes
  succeeded. No security consequence.
- Fix: as G-A (the range command in go-hamapping as an `upstream`
  candidate; here the bounds last sent per entity, used as `other` until
  a state shows them). The red test must cover (b) without a delay in
  the fake, so it is red by construction and not by timing.

### HR-P1-conc-32 (with HR-P1-conc-60) — low — one row

- One defect: `enqueue` drops on a full queue and falls back to
  `resyncSoon`, whose timer drops too when the queue is still full
  (`internal/bridge/bridge.go:214-221`, `:313-325`); `Disconnected` has
  already cleared `connected` when its closure is dropped (`:236-259`),
  and a sync while Home Assistant is away returns at once (`:367-370`).
  G-A adds the dropped `noteRemoved` (`:280-283`). M (read).
- M (probe V1, `internal/bridge/probe_v_internal_test.go`
  `TestProbeVQueueFull`, no worker running, known entities without an
  endpoint so the drain itself arms nothing):
  `PROBE V1: queued after flood=1024 (cap 1024), after the timer=1024, closures run=1024, states applied=1023 of 1100, queued after drain and 200 ms=0, queue_full lines=78, upstream_unusable logged=false, connected flag=false`.
- Why low, not medium. The loss needs three things at once: a backlog of
  1024 closures (the worker held in one long sync while events arrive),
  then no further event for the whole debounce (every overflowing event
  re-arms the timer, so the timer fires only one debounce after the last
  dropped event), and the queue still full at that moment. For the
  `Disconnected` half the connection must drop while the queue is full;
  a worker waiting on a Home Assistant call is released by that same
  drop and frees a slot. What is lost is stale values or stale
  reachability until the next sync (a registry event, a reconnect, a
  capability change) or the entity's next change; no command path and no
  role's reach is affected. The probe shows the mechanism with the
  worker stopped; how often a running daemon meets it is I, and G-B
  says so. The log line's promise ("a re-sync follows") is wrong in that
  case, which is the defect.
- Survivor: **HR-P1-conc-32** (the file is G-A's and its claim is the
  wider one). It takes G-B's probe as its red test and one fix: a
  "re-sync owed" flag the worker honours after every function
  (G-B), the unreachable marking in `sync`'s not-connected path and the
  removed entity recorded outside the queue (G-A). The two proposals are
  one design: level-triggered state instead of droppable closures.

### HR-P1-err-06 (with HR-P1-cfg-35) — low — one row

- M (read): a file that does not parse or names another version is moved
  aside and the store starts empty
  (`internal/selection/selection.go:89-103`); the configured node's
  input is the configured rule with that store
  (`cmd/ha2fabric/main.go:76-83`); a registry node's rule is empty
  (`cmd/ha2fabric/nodes.go:418-423`), so it exposes nothing; the warning
  text says "the web UI's overrides started empty"
  (`internal/exposure/exposure.go:344-352`) and not what is exposed
  again. S's probe S5 was not re-run; the read is sufficient.
- Low holds: the direction is fail-open on the configured node, towards
  commissioned controllers only, after a fault outside the daemon (the
  write is atomic) or a file of another version. Only one version exists
  today, so the "certain case" (a downgrade) is a future one.
- Survivor: **HR-P1-err-06** (S owns the file and has the probe); it
  takes G-A's observation that a registry node fails closed. The two
  reviewers recommend different fixes: question 3.

### HR-P1-ha-62 — low — confirmed

M (read `internal/ha/subscription.go:104-127`,
`internal/ha/client.go:417-457`,
`core:homeassistant/components/camera/webrtc.py:275-280`,
`core:homeassistant/components/websocket_api/connection.py:258-268`) and
M (probe V4, `internal/ha/probe_v_test.go` `TestProbeVSubscribeTimeout`):
`PROBE V4: Subscribe err=ha: camera/webrtc/offer: context deadline exceeded; connected still=true; commands the server saw=[camera/webrtc/offer]`.
HR-P1-ha-04 (unit S) is the same defect: the same lines, the same core
reference, the same fix. HR-P1-ha-62 survives (decision 23).

### HR-P1-bound-02 — low — confirmed

M (read): the host's map has no cap and every offer is accepted
(`internal/camera/camera.go:183`, `:328-357`); the kit bounds only the
id space (`gf:cluster/camera/provider.go:210-224`) and its table is an
uncapped slice (`:97`, `:158-168`); the port's result is stored as it
comes (`:557-563`). M (probe V3, `internal/camera/probe_v_test.go`
`TestProbeVSessions`):
`PROBE V3: offers accepted=2000 of 2000, upstream subscriptions=2000, still open=2000, goroutines +2000`.
The cost on Home Assistant's side per session stays I. Low holds: one
session per request, a commissioned controller.

### HR-P1-ha-08 — low — confirmed

M (read): `auth_invalid` is an attempt that did not authenticate, so the
delay doubles to `MaxBackoff` and stays (`internal/ha/client.go:167-190`,
`:301-302`); core counts each as a wrong login with a log line and a
notification (`core:homeassistant/components/websocket_api/auth.py:104`,
`:128`; `core:homeassistant/components/http/ban.py:111-146`) and bans the
address only when a threshold is configured (`ban.py:148-164`; the
Supervisor's address is exempt, `:153-155`). The ban therefore needs the
operator's `login_attempts_threshold`; without it the cost is one
notification a minute. That the add-on's refusals reach core at all is I:
there the connection goes through the Supervisor's proxy
(`internal/config/config.go:34`), which was not read. S's probe S6 was not
re-run.

### HR-P1-err-31 — low — confirmed

M (read): a batch is not a transaction and "what applied stays applied
whatever fails" (`gf:node/aggregator.go:361-427`, doc `:362-374`); the
bridge logs the joined error, then closes the replaced and removed
plans, takes `next` as served, counts its endpoints and clears
`SyncError` (`internal/bridge/bridge.go:427-452`, `:489-497`); `Resync`
reads only `SyncError` (`:590-593`). `b.forget` is emptied before the
batch (`:426`), so a failed `Forget` is lost as well. Reachability is I,
as G-A says.

### HR-P1-conc-33 — low → note

The read is right (`internal/bridge/command.go:25-32`, `:56-69`: no
check that the entity is still served). The grade is not: the command
was sent by a commissioned controller while the entity was exposed, so
the controller gains nothing it did not hold when it sent it (decision
11); the call is late, not out of bounds. The window is the queue's
backlog. The fix stays worth its few lines.

### HR-P1-err-61 — low → note

The read is right (`cmd/ha2fabric/nodes.go:240-242` drops the node
whatever `FactoryReset` returned; `gf:node/factory_reset.go:51-55`,
`:83-91`; `gf:node/node.go:1497-1512`). No trigger is reachable in this
daemon today: `served.op` serialises erase and close, so nothing else
holds the node's lifecycle slot (a free slot is taken whatever the
context's state, `gf:node/node.go:1503-1506`), and no host code runs the
bridge's Start or Stop that could keep `stopLocked` from finishing. A
reset that fails after the stop (`eraseStores`) leaves a stopped node,
for which dropping the reference is correct. A gap against go-fabric's
documented contract without a path to it is a note; the fix (keep `s.n`
when `rerr != nil`, stop it before the come-back) stays.

### HR-P1-err-64 — low — confirmed

M (read): after `cancel()` the wait covers the web server's `Shutdown`
(5 s, `internal/web/web.go:125-127`) and the client; only then the 10 s
node budget starts, spent in sequence (`cmd/ha2fabric/main.go:173-181`),
and `fleet.Close` and `served.close` take `op` without a deadline
(`cmd/ha2fabric/nodes.go:335`, `:666`). The Supervisor's default stop
timeout is 10 s, range 10–300 (`sup:supervisor/apps/validate.py:558-560`;
`sup:supervisor/docker/app.py:204-206`); `addon/config.yaml` sets none
(`grep`). `webErr` is not read on the signal path (`main.go:165-172`).
The coder/websocket close timings were not re-read (I).

## 3. Held policy

| ID | Held by S | Verifier |
| --- | --- | --- |
| HR-P1-auth-01 | yes | **holding is justified** |
| HR-P1-bound-02 | yes | holding is not needed |
| HR-P1-conc-03 | yes | holding is not needed |
| HR-P1-ha-04 | yes | holding is not needed |
| HR-P1-bound-17 | yes | holding is not needed |

Reasons:

1. **The role.** Decision 11: a commissioned controller is a fabric
   administrator the operator paired; it is bounded to the exposed
   entities' services, and a camera's session, still and candidates are
   that camera's services. Neither the LAN nor a co-installed add-on
   (decision 19) reaches any of the four: the commands need a CASE
   session. "Exploitable by an untrusted role" is not met.
2. **The class.** Each is a missing bound whose cost is at most one
   Home Assistant request or session per controller request. That is
   HR-P0-bound-28's class and role exactly (the candidate queue of one
   session), and that row was published and fixed in the open
   (`phase-0-register.md`). bound-02 is literally its other half.
3. **Decision 21** governs the wording about the admin check; none of
   the four touches it.
4. **Cost of holding.** Four rows, two go-fabric facade questions and
   G-B's duplicate would have to stay out of every tracked file, issue
   and commit message until a fix merges, and the go-fabric issues
   cannot be written without saying what they are for.

The counter-argument, for the owner: a paired controller of a large
ecosystem is software the operator does not control, and bound-02's
number (65 535 sessions per camera, each a provider session in Home
Assistant) is a cheap way to exhaust Home Assistant from a fabric. It
remains a commissioned administrator doing so, and the same controller
can call the exposed services in a loop without any defect.

The owner published the four (decision 23).

## 4. A high the reviewers missed

None found in the code read for this verification
(`internal/bridge/bridge.go:160-660`, `command.go`,
`internal/adapt/command.go:84-150`, `internal/ha/client.go:160-190`,
`:270-460`, `subscription.go:95-150`, `internal/camera/camera.go:176-190`,
`:318-432`, `internal/selection/selection.go:80-108`,
`internal/web/web.go:86-190`, `internal/plan/plan.go:100-200`,
`cmd/ha2fabric/main.go:70-86`, `:126-184`, `:240-276`,
`cmd/ha2fabric/nodes.go:212-262`, `:328-350`, `:415-425`, `:655-680`).
One thing checked and cleared: a `state_changed` of an entity the last
sync did not know schedules a full re-sync (`bridge.go:653-656`); `known`
holds every entity with a state or a registry entry
(`internal/plan/plan.go:110-118`, `:130`; `bridge.go:392-395`), so
ordinary traffic does not re-sync in a loop.

## 5. Not read, and why

Everything outside the rows above: the verifier's task is the mediums,
the duplicates, the sample and the held policy. Not re-run: the
reviewers' probes (S1–S9, G-A's four, G-B's three); the verifier's own
four replace them where a verdict rests on a measurement. Not read at a
pin: Linux networking, coder/websocket's close, the
Supervisor's Core proxy, go-fabric's cancellation of an invoke's
context. Nothing ran against a live system.

## 6. Probes

In the verifier's `git archive 5eefc0a` copy (`…/scratchpad/V/src`),
`go test ./internal/<pkg>/ -run TestProbeV -v -count=1`; all four passed
(a probe logs, it does not assert).

| Probe | File | For |
| --- | --- | --- |
| V1 | `internal/bridge/probe_v_internal_test.go` `TestProbeVQueueFull` | conc-32 / conc-60 |
| V2 | `internal/adapt/probe_v_test.go` `TestProbeVRange` | adpt-30 |
| V3 | `internal/camera/probe_v_test.go` `TestProbeVSessions` | bound-02 (output in the held file) |
| V4 | `internal/ha/probe_v_test.go` `TestProbeVSubscribeTimeout` | ha-62 |

## 7. Questions for the owner (recommendation first)

1. **The four held lows of S** (`bound-02`, `conc-03`, `ha-04`,
   `bound-17`): **publish them now**, as HR-P0-bound-28 was, or hold them
   until their fix merges (S's recommendation). Publishing also settles
   the duplicate `ha-04` / `ha-62` without rewording G-B's report; the
   held file says what changes in either case.
2. **HR-P1-auth-01** stays held: **until what?** Decision 21 ties a held
   row to the tag that carries its fix; this row has no fix in this
   repository. Recommendation: held until the documentation sentence the
   held file proposes is agreed and merged with phase 1's pull request;
   the measurement is X1's (phase 2) or the live test's.
3. **HR-P1-err-06, the fix** — S: keep the exposed set while the
   selection is "recovered"; G-A: accept and document. Recommendation:
   **a last-good copy** (`selection.json.bak`, written before each
   rename and read when the file does not parse), **a lenient read of a
   file of another version**, and a warning text that says entities
   turned off are exposed again when neither helps. It is the design
   that can be stated as correct today: it loses at most the last
   change and needs nothing from the node's store. S's shape is the
   better one in principle, but rests on reading the previously exposed
   set from the store, which neither reviewer established; it can
   follow if G-A's fix wave finds the set at hand.
4. **HR-P1-adpt-30**: G-A's "both" (the go-hamapping issue and the
   local bounds-last-sent). **Recommendation: yes, both**; the local
   part alone already closes (a) and (b).
5. **HR-P1-conc-32**: one change for the bridge's queue (the flag, the
   marking in `sync`, the removed entity outside the queue).
   **Recommendation: yes**, in G-A's fix wave, with G-B's probe as the
   red test.

*Answered by the owner on 2026-11-23: decisions 23–30 (`decisions.md`).*
