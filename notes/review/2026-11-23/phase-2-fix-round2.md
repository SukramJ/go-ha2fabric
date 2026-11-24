# Phase 2 fix wave — round 2 (after go-hamapping v0.7.0)

Implementer round 2 (Opus). Brief `notes/briefs/2026-11-24-review-p2-fix.md`,
section "Round 2 — the brief". Worktree `go-ha2fabric-review`, branch
`review/p2`, base `1e62d2f`. Context: `phase-2-fix-F1.md`,
`phase-2-fix-F2.md`, `phase-2-X2.md`, decisions 35–51; go-hamapping
v0.7.0's changelog ("What a host must do") and its C8 report ("Host
to-do").

## Commits

| Commit | Rows |
| --- | --- |
| `eae6cf6` chore(deps): go-hamapping v0.7.0 | pin lift |
| `dcb5268` fix(adapt): a cover's open_tilt and close_tilt call open_cover_tilt and close_cover_tilt | adpt-33 |
| `edf7f43` test(bridge): fixture covers with tilt end to end, every cover shape and 256 options build or are unmappable | adpt-33, adpt-34 |
| `ebe6c1a` fix(bridge): an entity go-fabric refuses is dropped and its device built again without it | data-35 |
| `f07d05a` test(bridge): FuzzPlanBuild skips only the thermostat shapes of go-hamapping #45 | test-42 |
| `f2abf51` build: FuzzClean in fuzz-smoke | FuzzClean |
| `b68d3d2` fix(plan,bridge): an oversized state's verdict says so, its state_changed is logged once per entity | bound-38 (bridge) |
| `2eb7324` fix(selection): a deleted node's store writes no file after its delete | conc-68 |
| `59e0cb0` docs(camera): the session cap holds under go-fabric's per-camera command lock | conc-69 |
| `a9c6477` docs: phase 2's camera, client, Supervisor and selection fixes in the changelog and the operator's page | changelog / docs |
| `f0fb638` chore(deps): THIRD-PARTY-NOTICES names go-hamapping v0.7.0 | pin lift |
| this report | — |

## Guards

At `f0fb638` (HEAD before and after the run): `make ci` 0 (lint with
staticcheck, so SA1019: no deprecated go-fabric identifier), `make race`
0, `make cover-check` 0, `make fuzz-smoke` 0 (with `FuzzPlanBuild` and
`FuzzClean`). Logs in the session's scratchpad `R2/`. This report's
commit is notes only: no second run. Not run (the coordinator's, by the
brief): `make vuln licenses secrets`, `make release`, `make docker
addon-image`.

## Per row

| Row | Status | Test | Red seen |
| --- | --- | --- | --- |
| pin lift | done: go-hamapping v0.6.0 → v0.7.0 (`go get`, `go mod tidy`); go-fabric stays v0.25.0; builds and tests green without a code change; `changelog.md` `[0.7.0]` `### Changed` bump line extended (and the release intro said `v0.5.0`, now `v0.7.0`); `THIRD-PARTY-NOTICES.md` pin | the suite | — |
| adpt-33 / decision 47 | done: `services["cover"]` `MethodOpenTilt` → `open_cover_tilt`, `MethodCloseTilt` → `close_cover_tilt`, no data (`cover/services.py:72-82`, as the brief gives it). Fixtures `cover.tilt_blind` (255, blind) and `cover.tilt_slats` (240) in `testdata/ha/` (states, entity registry, exposure) | `TestService` (two rows), `TestCoverTilt` (e2e: FeatureMap Lift+Tilt, Type TiltBlindLift, EndProductType InteriorBlind, GoToTiltPercentage → `set_cover_tilt_position`; the tilt-only cover's UpOrOpen/DownOrClose → tilt 100/0; with features 112 → `open_cover_tilt` / `close_cover_tilt`), `TestCoverShapesBuild` (X2's probe, 11 classes × 10 feature sets) | yes: `TestService` without the entry ("command has no service"); `TestCoverTilt` with the entry commented out ("never: a call on cover.tilt_slats"); at v0.6.0 the tree does not build (the methods are new) |
| adpt-34 | done by the pin | `TestSelect256Options` (X2's probe: dry and live both unmappable, same reason, no "go-fabric refused") | not separately (v0.6.0 does not build with the adapt change) |
| data-35 / decision 40 | done: `bridge.build` on `errors.As(err, &*hamapping.BuildError)` drops `be.Key` and `be.Riders` from the group's entities, marks their verdicts unmappable "go-fabric refused the device: …" and maps and builds again; bounded (each round removes at least one entity, a round that removes none falls back to the old whole-device failure); a failure that is no `BuildError` keeps the old path. The `plan.Group` stays whole (its descriptions and the owner map still see the refused entity, so a change of it re-plans). Log `bridge.build_refused` (`entity`, `riders`) | `TestRefusedEntityKeepsItsSiblings` (X2's `TestX2ProbeSiblingKnockedOut` with a refusal v0.7.0 still has: a thermostat with only `min_temp` 50 joining the plug's device) | yes: "switch.coffee lost its endpoint" |
| test-42 | done: `knownRefusal` keeps only go-hamapping #45's shapes, by attributes: (a) climate / water_heater with one of `min_temp`/`max_temp` and "setpoint configuration cannot be fixed", (b) climate with `heat_cool`/`auto` and "setpoint limits"; two seeds for them. The adpt-33/-34/-43 skips are gone | `FuzzPlanBuild` | yes: without the skip both new seeds fail; the old cover/select/inverted-bounds seeds pass without it |
| FuzzClean | done: `./internal/text/:FuzzClean` in `FUZZ_TARGETS` | `make fuzz-smoke` | — |
| bound-38 (bridge) | done: `plan.Verdict.Oversized` set by `Assess`, reason `OversizedReason(n)` ("Home Assistant's state is N bytes, over the cap of 262144: its attributes were dropped"), kept by `MergeVerdicts` after go-hamapping's reason; `Bridge.stateChanged` logs `ha.state_oversized` at Warn the first time per entity, Debug after (worker-owned set) | `TestOversizedReason` (Assess and Dry), `TestStateChangedOversizedLogged` | yes, both |
| conc-68 | done: `selection.Store.Close` (under `s.mu`: a `Set` in flight finishes, later ones return `ErrClosed` and write nothing); `fleet.Delete` closes the served node's store after `close` and before `RemoveFiles` | `TestClosedStoreWritesNothing` | yes (build failure: new API) |
| conc-69 | done: doc comment on `camera.Host.ProvideOffer` (the cap holds because go-fabric serialises a provider's commands, `cluster/camera/provider.go:125-131`, `state.go:15-26` at v0.25.0, both read; a slow Home Assistant holds the camera's other commands meanwhile) | — (comment) | — |
| changelog / docs | done (below) | — | — |

## changelog.md and docs

- `changelog.md` `[0.7.0]`: intro and `### Changed` bump line name
  go-hamapping `v0.7.0`, with covers with tilt served (adpt-33), a
  tilt-only cover's commands on its tilt, decision 48 (a tilt without
  `SET_TILT_POSITION` opens at 0 % only), 256 options and inverted bounds
  unmappable (adpt-34, -43), numbered labels. `### Fixed`: data-35; F2's
  eight lines as given (bound-38's extended by the `state_changed` log and
  the plan line); conc-68.
- `docs/operations.md`: F2's docs lines (log rows
  `camera.candidates_exceeded`, `camera.candidate_refused`,
  `camera.close_overrun`, `ha.state_oversized`; `ha2fabric.supervisor` and
  `_unreadable`; the administrator paragraph; the cameras list; the stop
  paragraph; the read-limit symptom and the 512-byte cut in "What the daemon
  reads and writes"; names and labels with control characters), plus
  `bridge.build_refused` and `bridge.build_failed` narrowed. Decision 50
  was already on the page (F1); decision 51 is stated by the read-limit
  sentence; decisions 48 and 49 are no page's subject (no page describes
  cover rows), 48 is in the changelog.

## Differences from the brief

- data-35's red test uses a thermostat (go-hamapping #45 shape (a)), not
  the probe's cover: v0.7.0 builds covers with tilt.
- data-35 drops the riders together with the key (the brief's "its Riders
  lose the endpoint too"); a rider whose own declarations were sound loses
  its endpoint as well, since the error does not say which declaration
  go-fabric refused.
- `TestCoverTilt` also drives the open_tilt/close_tilt path end to end by
  turning the 240 fixture into 112 in the test (no third fixture).
- `THIRD-PARTY-NOTICES.md` and the changelog's release intro (`v0.5.0`)
  were stale and are corrected.
- `ha.state_oversized` from a `state_changed` is logged once per entity
  per node's bridge (each bridge decodes its own events).

## Not done

- `CLAUDE.md`'s state line still says go-hamapping v0.6.0: a guard file,
  left to the coordinator.

## Questions

1. data-35: drop the riders with the key (as done), or keep them and let
   the next round find out (they move to a new primary, possibly to be
   refused again)?
2. `CLAUDE.md` state line: update to go-hamapping v0.7.0 in this pull
   request?
