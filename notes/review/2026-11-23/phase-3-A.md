# Phase 3 — unit A: architecture and tests

Reviewer: Opus (unit A of `notes/briefs/2026-11-26-review-p3.md`).
Baseline `0a47095`; every `file:line` is at the baseline unless a pin is
named (`gf:` go-fabric v0.26.0, `core:` Home Assistant core 2026.10.3).
Probes ran in scratch copies made with `git archive 0a47095`
(`…/scratchpad/p3/A` and `…/p3/A2`); the worktree was not touched except
for this file. M = measured (a probe or a run), I = inferred (read).

No new exploitable security item: no `held/phase-3-A.md`.

## 1. The package cut

### Import edges (M, `go list -f '{{.ImportPath}} {{.Imports}}' ./...`)

| Package | Module imports | go-fabric | Other modules |
| --- | --- | --- | --- |
| `cmd/ha2fabric` | bridge, camera, config, expose, exposure, ha, nodes, plan, selection, text, version, web | node | go-fabric-ui `fabricui` |
| `internal/adapt` | ha, text | `cluster/spec/keypadinput`, `cluster/spec/levelcontrol` | go-ha-catalog, go-hamapping, go-hamqtt `model` |
| `internal/bridge` | adapt, camera, ha, plan | device, eligibility, node | go-hamapping, go-hamqtt `model` |
| `internal/camera` | ha | `cluster/camera`, `cluster/spec/{cameraavstreammanagement,webrtctransportprovider}`, contract, device | go-hamqtt `model` |
| `internal/config` | expose, text, version | device, node | — |
| `internal/expose` | — | — | — |
| `internal/exposure` | bridge, expose, ha, plan, selection | eligibility | `fabricui` |
| `internal/ha` | — | — | coder/websocket |
| `internal/ha/hatest` | ha | — | coder/websocket |
| `internal/nodes` | text | node | `fabricui` |
| `internal/plan` | adapt, expose, ha | device, eligibility | go-hamapping, go-hamqtt `model` |
| `internal/selection`, `internal/text`, `internal/version` | text / — / — | — | — |
| `internal/web` | — | logring | `fabricui` |

Every edge is inside `internal/arch`'s tables (`arch_test.go:37-119`), and
every rule `CLAUDE.md` "Architecture" states holds at the baseline (I, by
the table). `internal/text` (phase 2) is in `layers` (`arch_test.go:71`) and
is a leaf; the Supervisor gate is `cmd/ha2fabric/supervisor.go` (no package
of its own) and the state cap sits inside `internal/ha`: neither adds an
import edge, so neither needs a layer row.

### Does the guard bite? (M, 27 mutations, appendix A)

A file with one forbidden import was added to a copy and
`go test -run TestImports ./internal/arch` run. Red (as it should be): bridge
→ config, bridge → `fabricui`, plan → `node`, plan → camera, bridge →
`cluster/camera` (code and test), bridge → `fabrictest`/`im` (code),
`cmd` → `controller`, ha → hatest (code), expose → ha, web → config,
camera → bridge, bridge → `cmd/ha2fabric`, hatest → `fabrictest`, a new
`internal/foo` or `internal/adapt/sub` importing a module package, bridge →
`golang.org/x/sync`. Green (allowed, as intended): plan's test → camera,
bridge's test → `fabrictest`. **Green where `CLAUDE.md`/`CONCEPT.md` say
otherwise or are silent**: plan → `sync`/`os`/`time` and a goroutine in
plan (api-04); adapt → `cluster/spec/onoff` and → `node` (api-05); text →
`node`; a new package `internal/foo` importing only go-fabric `node`, and
the root package `.` importing `internal/bridge` (api-06).

### Exported surface (M, AST script, appendix B)

Exported identifiers of `internal/*` that no other package's production
code uses — api-08.

## 2. Test quality (249 tests in 73 files)

- **Assert nothing / only "no error"** (M, AST scan, appendix C): no test
  without an assertion; five tests assert only through a polling helper
  (`waitFor`, `e.eventually`) — `queue_internal_test.go:112,127,139`,
  `sync_internal_test.go:289`, `cmd/ha2fabric/stop_test.go:228` — each a
  real condition. No test whose only assertions are `err != nil`.
- **Sleeps and races** (M): 22 timing-sensitive tests ran
  `-race -count=50` (camera 5, bridge 7, ha 10) and 6 of `cmd/ha2fabric`
  ran `-race -count=5`: **0 failures, 0 data races**. The R7 fix of
  `TestSupervisorGateRecloses` holds. Three tests take their precondition
  from a sleep without asserting it (test-11).
- **Tests that do not bite** (M): the back-off reset (test-02) and the R7
  camera change (test-03).
- **Negative cases on error paths** (M, the coverage profile's uncovered
  blocks, appendix D): 197 uncovered blocks of 15 packages; the ones that
  carry a decision are the Supervisor gate's read (test-12) and the bridge's
  whole-device build refusal (test-15). The rest are log-only branches,
  unreachable defaults or OS faults (a file that cannot be moved aside).
- **Tests that test the fake**: none found; the fake's shapes are compared
  with core below.

## 3. The fake against core 2026.10.3

Read: `internal/ha/hatest/hatest.go` and `camera.go` in full; core
`websocket_api/connection.py:199-320`, `commands.py` (call_service
`:282-350`, subscribe_events `:190-220`, unsubscribe `:246-263`, ping
`:731-735`), `const.py:35-46`, `camera/webrtc.py:205-350`,
`homeassistant/exposed_entities.py:414-442`, `helpers/area_registry.py:88-104`,
`helpers/label_registry.py:169-224`, `helpers/entity_registry.py:369-403`.

Holds (M by reading both): `unknown_command` with "Unknown command."
(`hatest.go:308-313` vs core `connection.py:236-240`); subscribe result
`null` then events; event envelope and `Event._as_dict` keys; call_service
result `{"context": …}` (core `commands.py:292-295`, `response` only with
`return_response`); `not_found` for an unknown unsubscribe
(`camera.go:204-209` vs core `commands.py:261-263`); the offer's order —
result, then the session event (`camera.go:173-176` vs core
`webrtc.py:276-292`); the camera proxy's 401/403/404/503
(`camera.go:218-243`); `homeassistant/expose_entity/list` omits entities
exposed to nobody (`hatest.go:274-275`, `SetExposed` deletes; core
`exposed_entities.py:433-441`); the label events' shapes; the entity
registry list's 22 keys equal core's `as_partial_dict`
(`entity_registry.py:378-401`).

Differs:

| What | Fake | Core | Matters |
| --- | --- | --- | --- |
| id not increasing | fails the test, closes (`hatest.go:218-223`) | `id_reuse` error, connection stays (`connection.py:226-231`) | no: stricter, the client never reuses |
| camera/webrtc/offer, /candidate for a camera without WebRTC or unknown | accepted (`camera.go:161-185`) | `webrtc_offer_failed` / `webrtc_candidate_failed` result error (`webrtc.py:211-245`), unknown camera a `home_assistant_error` | yes: test-10 |
| candidate payload | any map (`camera.go:181`) | `_parse_webrtc_candidate_init` → `invalid_format` (`webrtc.py:323-330, 335-341`) | low: test-10 |
| device_registry_updated | update only (`hatest.go:436-449`) | also create, remove | note: test-10 |
| subscribe result vs a concurrent `fire` | an event can precede the result (`hatest.go:276-280` then `:317`; `fire` from another goroutine) | never: one event loop, `connection.subscriptions[...]` then `send_result` (`commands.py:216-220`) | no: the client registers before it writes (`client.go:536-538`) |
| a result without `success`, a reply of another type | never sent | never sent | the client takes either as success (ha-09) |

## 4. Guards

**Floors** (M, `make cover` in the copy, 1 m 13 s, then `coverfloor`):

| Package | measured | floor | gap |
| --- | --- | --- | --- |
| cmd/ha2fabric | 88.0 | 75 | 13.0 |
| internal/adapt | 96.9 | 95 | 1.9 |
| internal/bridge | 94.0 | 82 | 12.0 |
| internal/camera | 95.2 | 90 | 5.2 |
| internal/config | 98.5 | 92 | 6.5 |
| internal/expose | 97.6 | 95 | 2.6 |
| internal/exposure | 96.8 | 96 | 0.8 |
| internal/ha | 95.2 | 80 | 15.2 |
| internal/nodes | 91.1 | 88 | 3.1 |
| internal/plan | 99.6 | 98 | 1.6 |
| internal/selection | 93.7 | 90 | 3.7 |
| internal/version | 90.9 | 75 | 15.9 |
| internal/web | 96.2 | 85 | 11.2 |
| internal/text | 100.0 | — | no floor |
| internal/ha/hatest | 95.4 | — | no floor (test support) |
| script/coverfloor | 0.0 | — | no floor, no test |

→ test-01.

**Fuzz** (M): `FUZZ_TARGETS` (`Makefile:80-85`) names 15 targets; the tree
has 15 `func Fuzz` (`internal/{ha,config,selection,nodes,camera,adapt,bridge,text}`);
the sets are equal. Nothing holds them equal (test-13).

**Defect classes of the registers 0–2 → standing guard**:

| Class (rows) | Guard | Bites |
| --- | --- | --- |
| import cut, facade (A1, CLAUDE.md) | `TestImports`, `TestGuardRefuses` | M: 22 of 27 mutations red; the 5 green are api-04, -05, -06 |
| hostile parse (P0 data-01/02, err-03; P2 test-42) | 15 fuzz targets in `fuzz-smoke` (CI) | I (red seen per the registers); list drift unguarded (test-13) |
| unsafe text (P2 data-31, data-94; P1 data-36) | `TestClean`, `TestSafe`, `TestUnsafeOnlyRegistryName`, `FuzzClean`, `FuzzEntityState` | M: dropping U+2067..2069 from `text.Unsafe` red in `text` and `adapt` |
| secrets in logs (P0 leak-24/25, ha-23) | "token not in log" asserts in `TestRun`, `TestDumpPlan`, the client and gate tests; `make secrets` | I |
| web gate (P0 auth-20/50/51, P1 test-16) | `admin_test.go` table, `TestRights`, `TestAtLeast` | I (red seen in the fix reports) |
| HA-side bounds (P2 bound-37/38/39/40/90) | `TestErrorTextIsCut`, `state_cap_test.go`, `TestHACandidatesAreBounded`, `TestOversizedStateTakesTheEndpoint` | M for bound-39 and bound-40 (mutations red); I for the rest |
| ignored events are no activity (P2 bound-40) | `TestIgnoredEventsDoNotKeepTheSession` | M: touching on every event red |
| controller-side camera bounds (P0 bound-28, P1 bound-02/17) | `bounds_test.go` | I |
| debounce has a latest time (P2 data-30) | `TestDebounceHasALatestTime` | M: `maxWait` 1000 red |
| reconnect back-off (P1 ha-07) | `TestClientBackoffGrowsAfterShortConnection`, `…ResetsAfterStableConnection` | M: **the reset half does not bite** (test-02) |
| auth back-off (P1 ha-08, ha-92) | `TestClientAuthRefused*` | I |
| batch settle (P1 err-31; R7) | `TestFailedBatchOperationIsRetried`, the three R7 tests | M: R7 revert red ×4 |
| camera session end (P1 conc-03 → decision 31, R7) | `TestQuietSessionStays` | M: **the revert stays green** (test-03) |
| one snapshot per event (P2 bound-62) | `TestOneEventOneSnapshot` | I |
| owed sync, queue (P1 conc-32/96) | `queue_internal_test.go` | I |
| stop budget (P1 err-64) | `TestAddonStopTimeout`, `TestStopWithinBudget` | I; the latter vacuous on a slow runner (test-11) |
| image defaults (P1 cfg-12, sup-19) | `TestImageDefaults` | I; partial (test-14) |
| plan determinism (P1 test-38) | `TestAssessIgnoresInputOrder` | I |
| go-hamapping shapes (P2 adpt-33/34/43) | `FuzzPlanBuild`, `TestCoverShapesBuild`, `TestSelect256Options` | I |
| Supervisor gate (P2 cfg-03; P0 auth-50/51) | `TestSupervisorGate*`, `TestAtLeast` | I; the read's negative cases missing (test-12) |
| coverage loss | `make cover-check` | M: **up to 15.9 points of slack** (test-01) |
| credentials from the environment only (CLAUDE.md Rules) | none | — (api-07) |

## 5. The R7 delta's tests

- **Settle from `Aggregator().Endpoints()`** (M): reverting `a2b00bb`'s
  hunks of `internal/bridge/bridge.go` (`patch -R`) turns
  `TestRefusedForgetIsRetried`, `TestRefusedReplaceIsRetried`,
  `TestForgetOfAStoppedBatchIsKept` (`bridge_r7_e2e_test.go:25,82,130`) and
  `TestFailedBatchOperationIsRetried` (`bridge_p1_e2e_test.go:67`) red.
  Holds.
- **Camera end without the idle timer** (M): `camera.go` of `a2b00bb^`
  (with the 1 h `IdleTimeout`, import paths moved) in place of the
  baseline's leaves `go test ./internal/camera/` green → test-03.

## Findings

### HR-P3-test-01 — medium — the coverage ratchet does not bite

`script/coverfloor/main.go:64-75` fails a package only below its floor;
nothing fails a floor that trails the measurement, and a package without
a floor passes silently (`:64` `floor, ok := floors[p]`, `:66` `if ok &&`).
Measured: five floors trail by 11.2–15.9 points (`floors.txt:3,5,12,14,15`:
cmd 75 vs 88.0, bridge 82 vs 94.0, ha 80 vs 95.2, version 75 vs 90.9, web
85 vs 96.2); `internal/text` (100.0, phase 2's layer) has no floor. The
header calls the floors a ratchet (`main.go:12-13`, `floors.txt:1-2`,
`CLAUDE.md:48-49`), but a regression of 15 points in `internal/ha` passes
`make cover-check` and CI. Reference: go-fabric fails a floor more than
`staleFloorSlack` = 3.0 points under the measurement
(gf:`script/coverfloor/main.go:73-81`, `:275-286`; its review W6-test-05
found the same 5–13 point trail). M.
**Fix**: port the stale check (3 points plus the run spread, a zero floor
without slack), fail a package of `./...` that has statements and no floor
(exempt `internal/ha/hatest`, `script/*` by an explicit list), raise the
floors to the whole percent below the measurement, add a floor for
`internal/text`; give `script/coverfloor` a `main_test.go` as go-fabric's.
**Red test**: `script/coverfloor/main_test.go` — a profile at 94 % with a
floor of 82 fails ("raise it to 93"); a profile with a package and no
floor fails.

### HR-P3-test-02 — low — the back-off reset of HR-P1-ha-07 has no biting test

`internal/ha/client.go:238-240` resets the back-off after a connection up
longer than `MaxBackoff`. `TestClientBackoffResetsAfterStableConnection`
(`client_p1_test.go:188-201`) runs with `MaxBackoff` 50 ms and accepts a
reconnect within 1 s, so without the reset (the delay capped at 50 ms) it
passes. Measured: `if false && up > c.opts.MaxBackoff` leaves
`./internal/ha` and `./cmd/ha2fabric` green. HR-P1-ha-07's fix is right; its
test is incomplete (register phase 1, row HR-P1-ha-07). M.
**Fix** (test only): a `hostile` server whose first connections drop at
once until the delay reaches `MaxBackoff` (400 ms), then one that stays
500 ms; the reconnect after it must come within 100 ms (`MinBackoff`
10 ms). **Red test**: appendix E.1, red with the mutation above.

### HR-P3-test-03 — note — the R7 camera change has no test that fails on its revert

`TestQuietSessionStays` (`internal/camera/bounds_test.go:117-134`) watches
a quiet session for 100 ms (`:129`); the removed timer was 1 h, so the
baseline's tests pass with the pre-R7 `camera.go` (M, §5). The behaviour
(no idle end; go-fabric ends a lost controller's sessions through
`EndSession`, decision 31) is held by the code alone. A test that sees a
1 h timer needs a clock seam the package does not have, and adding one for
a timer that no longer exists is a convenience seam. M.
**Proposed**: accept (decision 31); the test's comment should say it holds
the eviction-below-the-bound half only. Owner question 2.

### HR-P3-api-04 — low — `internal/plan`'s purity is held only against `node`

`CLAUDE.md:78-80` and `CONCEPT.md:99-100`: "`internal/plan` is pure, with
no node and no goroutines". `arch_test.go:63-65,127-129` forbids `node`
and nothing else: a plan file importing `sync`, `os` or `time`, or starting
a goroutine, passes `TestImports` (M, appendix A). Holds today: `plan`
imports `encoding/json fmt maps slices sort strings` and has no `go`
statement (M). **Fix**: in `internal/arch`, a standard-library allowlist
for `internal/plan` (and for `internal/expose`, the rule's other pure
package), and an AST walk refusing `*ast.GoStmt` in their non-test files.
**Red test**: the mutation files of appendix A (`plan` → `sync`; a
`go func(){}()` in plan) must fail `TestImports`.

### HR-P3-api-05 — low — `internal/adapt`'s go-fabric reach is not frozen

`CLAUDE.md:87-92` ("Not built yet (A2); new code does not deepen what it
removes") and `CONCEPT.md:104-107`: adapt imports nothing of go-fabric
once A2 lands. Today it imports `cluster/spec/keypadinput` and
`cluster/spec/levelcontrol` (`internal/adapt/command.go:18-19`, since
`0ee0fd2`); the guard lets any facade or def package in (adapt →
`cluster/spec/onoff` and adapt → `node` green, M). **Fix**:
`adaptGoFabric = {"cluster/spec/keypadinput", "cluster/spec/levelcontrol"}`
in `internal/arch`, refusing any other go-fabric import of adapt; emptied
with A2. **Red test**: the two mutations above. (For unit C: the line
ranges `CONCEPT.md:105` cites, `command.go:15-16, :208-209, :276-284`, are
now `:18-19`, `:234-237`, `:293-309`.)

### HR-P3-api-06 — note — a package can sit outside the layer table

`arch_test.go:171-176`: a package without a `layers` entry is reported only
when it imports a module package, and the root package `.` is exempt. M: a
new `internal/foo` importing only go-fabric `node` passes; `doc.go`'s
package importing `internal/bridge` passes. **Fix**: every walked package
directory has an entry (an explicit exemption list for `script/*`), `.`
gets `{}`. **Red test**: the two mutations.

### HR-P3-api-07 — low — "credentials from the environment only" has no standing guard

`CLAUDE.md:143-144`. Holds today (M): the only environment read in
production code is `internal/config/config.go:199` (`OSEnv`); the token
reaches the client and the gate through `config.Config`. Nothing stops a
package from `os.Getenv("SUPERVISOR_TOKEN")` beside the configuration
(and its logging rules). **Fix**: in `internal/arch`, an AST check that
`os.Getenv`, `os.LookupEnv`, `os.Environ` appear in no non-test file
outside `internal/config`. **Red test**: a file in `cmd/ha2fabric` calling
`os.Getenv` fails.

### HR-P3-api-08 — note — exported surface used inside its package only

M (appendix B): no other package's production code uses, among others,
`config.ErrHelp`, `SplitLabels`, `ValidationError`, `DefaultVendorID`,
`DefaultProductID` (`config.go:47-48`; the flag defaults repeat them as
the strings `"0xFFF1"`, `"0x8001"`, `:239-240`), `adapt.NamespaceDevice`,
`NamespaceEntity`, `plan.UnreadDevice`, `UnreadEntity`, `OversizedReason`,
`ha.Decode`, `ha.Type*`; `ha.TypeResult` (`message.go:26`) is used
nowhere. **Fix**: unexport what only the package and its in-package tests
use; derive the flag defaults from the constants. No test (a lint).

### HR-P3-ha-09 — note — any non-event reply to a pending id is taken as a result

`client.go:449-457` hands every message that is not an `event` to the
pending call; `:565-574` returns success when `success` is absent or the
type is neither `result` nor `pong` (`TypeResult` unused). Core always
sends `type: result` with `success` (core `websocket_api/messages.py`
`result_message`/`error_message`), so no live path differs; the fake does
the same. I. **Fix**: a reply of another type, or a result without
`success`, fails the call (`ErrFrame`). **Red test**: a `scripted` server
answering `{"id":N,"type":"result"}`; `Call` must error (today: nil;
`client.go:572` uncovered).

### HR-P3-test-10 — low — the fake answers camera commands core refuses

`hatest/camera.go:161-185` accepts `camera/webrtc/offer` and `/candidate`
for any entity and any candidate map. Core answers a camera without WebRTC
with `webrtc_offer_failed` / `webrtc_candidate_failed`
(core `camera/webrtc.py:211-245, 260, 342`), an unknown camera with an
error, and a candidate that is not an `RTCIceCandidateInit` with
`invalid_format` (`:323-330`). So a refused offer is tested only against
`internal/camera`'s own stub (`camera_test.go:568`), never through
`ha.Client` → `camera.HA` → `camera.Host`; and the fake has no
`device_registry_updated` create/remove (`hatest.go:436-449`). I.
**Fix**: `SetWebRTC(entityID, false)` in the fake answering core's error
result; candidate validation as core's; `RemoveDevice` firing
`device_registry_updated` remove. **Red test**: an end-to-end camera test
whose offer is refused reaches the controller as the relay's error, and
the session count stays 0.

### HR-P3-test-11 — note — preconditions taken from a sleep

`cmd/ha2fabric/stop_test.go:92` and `:125` sleep 500 ms and assume the
erase reached the slowed `newNode` / `factoryReset`;
`internal/ha/client_p1_test.go:342-346` logs (not fails) when the write
ended before the server read. On a slower runner each passes without the
state it tests. M: all pass (`-race -count=5`, `-count=5 -v` shows the
write scenario built). **Fix**: the seams signal a channel on entry and the
test waits for it; the client test fails (`t.Fatal`) when its precondition
did not hold.

### HR-P3-test-12 — note — the Supervisor gate's read has no negative cases

`cmd/ha2fabric/supervisor.go:198-206`: an undecodable 200 body,
`result` ≠ `"ok"` and a missing `data.supervisor` are uncovered (M, the
profile). Each leaves the gate where it was (fails closed from unknown,
`:150-166`; I). **Red test**: `fakeSupervisor` variants with those three
bodies; the gate stays closed with `warningSupervisorUnknown`.

### HR-P3-test-13 — note — the fuzz-smoke list is held equal by hand

15 targets in `Makefile:80-85`, 15 `func Fuzz` in the tree (M); a new
target left out of `FUZZ_TARGETS` never runs in CI. **Fix**: a test in
`internal/arch` that parses `FUZZ_TARGETS` and compares it with the
`func Fuzz*` it finds. **Red test**: a `FuzzX` added without the list
fails.

### HR-P3-test-14 — note — `TestImageDefaults` reads only single-line `ENV`

`cmd/ha2fabric/image_test.go:28-41` checks `WEB_LISTEN` only on a line
that starts with `ENV`; a continuation line (`ENV A=1 \` then
`WEB_LISTEN=…`) or `ENTRYPOINT [… "--web-listen", …]` escapes. Holds today
(`Dockerfile:37` is single-line). I. **Fix**: join continuation lines; also
refuse `--web-listen` in `ENTRYPOINT`/`CMD`.

### HR-P3-test-15 — note — the whole-device build refusal is untested

`internal/bridge/bridge.go:1072-1081` (an error go-hamapping does not
attribute to one entity: every entity of the device unmappable) is
uncovered (M). **Red test**: a `bridge` internal test with a build that
fails with a plain error; every verdict of the group is unmappable with
the reason, nothing is served.

## Totals

0 high · 1 medium · 5 low · 9 note (15).

## Not read

- **Read in full**: `internal/arch/arch_test.go`, `internal/ha/hatest/*`,
  `internal/bridge/{queue,sync}_internal_test.go`,
  `bridge_r7_e2e_test.go`, `bridge_e2e_test.go`,
  `internal/ha/client_test.go`, `client_p1_test.go`,
  `cmd/ha2fabric/{main,exits,web,image,supervisor}_test.go`, the R7 diff
  of `internal/camera/bounds_test.go`; slices of `stop_test.go`,
  `warnings_test.go`, `internal/ha/camera_test.go`.
- **Not read line by line** (machine and token budget): the other 55 test
  files — `internal/bridge` e2e files c1, c2, e1, f2, h3, p1, p2, p2r2,
  p2r3, r5, r6, u6 and `bridge_test.go`, `fuzz_test.go`; `internal/camera`
  `camera_test.go`, `close_test.go`, `ha_test.go`, `fuzz_test.go`, most of
  `bounds_test.go`; `internal/ha` fanout, snapshot, state_cap, message,
  excerpt, scrub, url, hostile, fuzz, the rest of camera; `internal/config`,
  `nodes`, `selection`, `web`, `expose`, `exposure`, `plan`, `adapt`,
  `version`, `text` tests; `cmd/ha2fabric/{admin,nodes,stop,warnings}_test.go`
  in the rest. Each of the 249 tests was covered by the AST scans
  (assertions, err-only asserts, sleeps; appendix C) and the coverage
  profile; the ones with sleeps were run under `-race`. A finding in their
  bodies (a wrong expectation) would be missed.
- **Fixtures**: key sets checked for the entity registry list only;
  `device_registry_list.json`, `get_states.json`, `get_config.json` and the
  rest rest on their `_mirrors` citations (unit C).
- `CONCEPT.md`: only "Decided" and the cut (`:98-145`).

## Probes run

| Probe | Result |
| --- | --- |
| 27 arch mutations (appendix A) | 22 red, 5 green (api-04, -05, -06) |
| `make cover` + `coverfloor` | pass; floors table above |
| R7 settle revert (`patch -R` of `a2b00bb` on `bridge.go`) | 4 tests red (guard bites) |
| R7 camera revert (`a2b00bb^:camera.go`) | camera package green (test-03) |
| back-off reset removed (`client.go:238`) | ha, cmd green (test-02) |
| touch on every camera event | `TestIgnoredEventsDoNotKeepTheSession` red |
| `maxWait` 5 → 1000 | `TestDebounceHasALatestTime` red |
| `Excerpt` never cuts | `TestErrorTextIsCut` red |
| U+2067..2069 dropped from `text.Unsafe` | `TestClean`, `TestSafe`, `TestUnsafeOnlyRegistryName` red |
| HA candidate bound off | `TestHACandidatesAreBounded` red |
| 22 timing tests `-race -count=50`; 6 cmd tests `-race -count=5` | all pass, no race |

## Questions for the owner

1. **Coverage ratchet** (test-01): port go-fabric's stale-floor check
   (3 points), require a floor for every package, raise the floors now?
   Recommendation: yes — it is the best design (the same guard as
   go-fabric, one shape across the family); today the guard does not
   detect a loss under 11–16 points in five packages.
2. **R7 camera change** (test-03): accept without a biting test (decision
   31), or add a clock seam to `internal/camera`? Recommendation: accept —
   a seam for a timer that no longer exists is a convenience seam the
   architecture standard rules out; the eviction below the bound stays
   tested.
3. **One AST guard in `internal/arch`** for api-04, -05, -06, -07 and
   test-13 (plan/expose stdlib allowlist and no goroutines; adapt's
   go-fabric allowlist; a layer entry for every package; environment reads
   in `internal/config` only; the fuzz list)? Recommendation: yes, one
   standing guard per rule `CLAUDE.md` states, in the wave that fixes this
   phase.

## Appendix — probes

### A. Arch mutations

`scripts/mut.sh <root> <pkg-dir> <suffix> <package> <import>` writes
`<pkg-dir>/zz_mut<suffix>.go` with `import _ "<import>"`, runs
`go test -count=1 -run TestImports ./internal/arch`, removes the file. The
green ones (the red regression tests for api-04/05/06):

```
internal/plan      plan   sync | os | time
internal/adapt     adapt  thingfabric.dev/go-fabric/cluster/spec/onoff
internal/adapt     adapt  thingfabric.dev/go-fabric/node
internal/text      text   thingfabric.dev/go-fabric/node
internal/foo       foo    thingfabric.dev/go-fabric/node
.                  fabric thingfabric.dev/go-ha2fabric/internal/bridge
```

and, for the goroutine rule, `internal/plan/zz_mut.go`:

```go
package plan

func init() { go func() {}() }
```

### B. Exported surface

An AST script listing each `internal/*` package's exported top-level
identifiers and the packages that select them (`alias.Ident`), split into
production and test users; 219 identifiers have at most one production
user outside their package, 124 none (68 of them no test user outside
either). The list in api-08 is the subset
with a decision behind it.

### C. Test scans

An AST script over every `_test.go`: per `Test*`/`Fuzz*` the number of
`t.Error*/t.Fatal*/t.Fail*` calls, of helpers taking `t`, of `time.Sleep`;
and the tests whose every asserting `if` is `err != nil`. Output: 26 tests
with a sleep in their body (all but `cmd/ha2fabric` `TestRun` and
`internal/web` `TestRun`, `TestRunLoopback` repeated under `-race`), 5
assert through helpers only, 0 err-only. `FuzzEventData` asserts through panics only (by design of a
decode fuzz target).

### D. Uncovered blocks

`awk` over `coverage.out` (`-coverpkg=./...`): blocks never covered by any
test binary, 197 (34 `cmd/ha2fabric/nodes.go`, 23 `main.go`, 22
`internal/bridge/bridge.go`, 18 `internal/nodes/nodes.go`, 15
`internal/camera/camera.go`, 14 `hatest.go`, 12 `internal/ha/client.go`,
the rest ≤ 8 per file).

### E. Red tests proposed

E.1 (test-02), in `internal/ha/client_p1_test.go`:

```go
// TestClientBackoffResetsAfterStableConnection, rewritten: the delay first
// grows to MaxBackoff, then a connection up longer than MaxBackoff resets
// it, so the next attempt comes after MinBackoff, not MaxBackoff.
func TestClientBackoffResetsAfterLongConnection(t *testing.T) {
	var n atomic.Int32
	var dropped atomic.Int64 // unix nanos of the long connection's end
	var next atomic.Int64
	url := hostile(t, func(ctx context.Context, ws *websocket.Conn) {
		i := n.Add(1)
		if i == 7 && dropped.Load() != 0 {
			next.CompareAndSwap(0, time.Now().UnixNano())
		}
		send(ctx, ws, `{"type":"auth_required"}`)
		if _, _, err := ws.Read(ctx); err != nil {
			return
		}
		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
		if i == 6 { // after five short ones the delay is at MaxBackoff
			time.Sleep(500 * time.Millisecond)
			dropped.Store(time.Now().UnixNano())
		}
		_ = ws.Close(websocket.StatusGoingAway, "bye")
	})
	start(t, ha.Options{URL: url, Token: "t", MinBackoff: 10 * time.Millisecond, MaxBackoff: 400 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	deadline := time.Now().Add(10 * time.Second)
	for next.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no attempt after the long connection")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if d := time.Duration(next.Load() - dropped.Load()); d > 150*time.Millisecond {
		t.Fatalf("reconnected %v after a connection up longer than MaxBackoff; want about MinBackoff", d)
	}
}
```

Red with `client.go:238` `if false && up > c.opts.MaxBackoff` (the delay
is then 400 ms).

E.2 (test-01), `script/coverfloor/main_test.go`: run the check on a
two-line profile (`pkg/a.go:1.1,2.2 94 1`, `pkg/a.go:3.1,4.2 6 0`) with a
floor of 82 → exit 1 naming 93; with no floor for `pkg` → exit 1.
