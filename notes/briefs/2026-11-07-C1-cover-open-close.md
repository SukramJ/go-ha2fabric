# Brief C1 (go-ha2fabric) — covers with OPEN|CLOSE only: the go-hamapping v0.4.4 bump and its fixture

Owner decision (2026-11-07): start #42 right after go-hamapping v0.4.4 (PR #19,
brief C1 there: `notes/briefs/2026-11-07-C1-cover-command-gates.md`). One
pull request on `fix/cover-open-close`, squash-merged, auto-merge. Whether
the wave ends in release 0.5.4 is the owner's call at the pull request; the
bump line and the fixture are the same either way. Reported in #40
(comment 6711638340); the host follow-up is #42.

Sources, verified 2026-11-07 (cite, do not re-derive):
- go-hamapping v0.4.4 `rows/cover.yaml` (the `commands` of `window_covering`):
  `StopMotion` only with STOP; `GoToLiftPercentage` → `set_position` only
  with SET_POSITION; without SET_POSITION `LiftPercent100thsValue` 0 renders
  `payload_open`, any other value `payload_close` (matter.js
  `WindowCoveringServer.ts:576-590` @db021da0; go-fabric
  `cluster/cover/windowcovering_server.go:144-158, 624-634` @v0.18.0).
- Home Assistant 2026.10.3: `set_cover_position` requires SET_POSITION,
  `stop_cover` STOP (`homeassistant/components/cover/services.py:49-62`);
  flags OPEN 1, CLOSE 2, SET_POSITION 4, STOP 8 (`cover/const.py:44-54`).
  Read from raw files, never run.
- This repo (main after #39): `internal/adapt/command.go:44` renders the
  cover's command binding payloads `payload_open/close/stop` as
  `open_cover`/`close_cover`/`stop_cover`, `:104` the `set_position` method
  as `set_cover_position` — unchanged by this wave; the row decides.
  Fixtures: `testdata/ha/get_states.json` has one cover,
  `cover.living_blind` (`supported_features: 15`, `device_class: blind`,
  `current_position: 70`, device `dev-somfy-1`, platform `somfy`), exposed
  in `expose_entity_list.json`, registered in `entity_registry_list.json`.
  The e2e harness asserts a service call with `e.ha.WaitCall` and a
  predicate on `ha.ServiceCall` (`internal/bridge/bridge_e2e_test.go:240-252`);
  `hatest` records every `call_service` (`internal/ha/hatest/hatest.go:284-286`).

## §1 Bump

`go get github.com/SukramJ/go-hamapping@v0.4.4 && go mod tidy`; no other
module moves (report if one does). `THIRD-PARTY-NOTICES.md:32` pin →
`v0.4.4`. `changelog.md` `[Unreleased]` `### Fixed`: the symptom (a cover
with OPEN|CLOSE only got `set_cover_position`/`stop_cover`, which Home
Assistant refuses with `ServiceNotSupported`, #40) and the behaviour now
(0 opens, any other value closes; `StopMotion` without STOP answers success
and calls nothing), naming go-hamapping v0.4.4.

## §2 Fixtures

Two covers beside `cover.living_blind`, each with a registry entry on an
existing or new device, a state and an entry in `expose_entity_list.json`
(every fixture file cites the Home Assistant core lines it mirrors, as the
existing entries do — copy their form):
- `cover.scene_blind`: `supported_features: 3`, `device_class: blind`, no
  `current_position`, state `open` — a template cover with open/close
  scenes (the reporter's case).
- `cover.position_blind`: `supported_features: 7`, `current_position: 40`.
Check that no existing test counts candidates, endpoints or exposed
entities in a way the two new entities break (`grep -n 'candidates\|endpoints' internal/bridge/*_test.go cmd/ha2fabric/*_test.go`),
and adjust those counts with the reason in the commit.

## §3 Tests (e2e, the pattern of `bridge_e2e_test.go:240-252`)

In the bridge e2e file of this wave (`internal/bridge/bridge_c1_e2e_test.go`):
- `cover.scene_blind` has an endpoint; its WindowCovering FeatureMap has Lift
  and not PositionAwareLift (`fabrictest.ReadCluster`, the FeatureMap
  attribute); `GoToLiftPercentage{0}` → `WaitCall` `cover.open_cover` on
  `cover.scene_blind`; `GoToLiftPercentage{10000}` → `close_cover`;
  `StopMotion` → the invoke succeeds and **no** call arrives (a bounded
  wait that must time out; use the harness's existing negative pattern if
  there is one, else a short `WaitCall` with a context deadline and an
  expected `DeadlineExceeded`); `UpOrOpen` → `open_cover`.
- `cover.position_blind`: `GoToLiftPercentage{2500}` → `set_cover_position`
  with `position` 75; `StopMotion` → no call.
- `cover.living_blind` (15) unchanged: `StopMotion` → `stop_cover`.
Existing e2e assertions unchanged. `internal/adapt` tests: none needed
(nothing changes there); say so.

## §4 Guards, commits

Commits, each green under `make ci`: (1) this brief; (2) bump + notices;
(3) fixtures + tests; (4) changelog. Guards in the background with one
summary log (HEAD, exit per guard): `make ci`, `make race`,
`make cover-check`, `make secrets`. Skip `make fuzz-smoke` (the fuzz target
is `internal/ha`'s decoder; nothing there moved) and the image build (owner's
call at the pull request with the release) — say so. Local work only:
branch, commits, guards; the owner's session pushes and opens the pull
request.

## §5 Report (hand back, fixed shape)

Per section the commits (SHA, title); guards with HEAD and exit; differences
from this brief; decisions; left out; questions; `by_design` candidates.
Under 200 words.
