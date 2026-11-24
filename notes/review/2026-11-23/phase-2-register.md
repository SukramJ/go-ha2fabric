# Phase 2 — the dispositions

Branch `review/p2`; the pull request is squashed, its single commits stay
retrievable through the pull request's reference. Sources: `phase-2-X1.md`
(IDs 01–04), `phase-2-X2.md` (30–43), `phase-2-X3.md` (60–70),
`phase-2-verify.md`, the pre-merge read with its re-read of round 3
(`phase-2-premerge.md`, IDs 90–97), the fix reports `phase-2-fix-F1.md`,
`phase-2-fix-F2.md`, `phase-2-fix-round2.md`, `phase-2-fix-round3.md`,
`decisions.md` rows 35–52. The commits are `review/p2`'s
(`git log 2324fbf..f851339`); F2 worked on a branch of its own, so its
report's SHAs differ from the ones cited here. Severity is the verifier's
for every medium, every merge and the sampled lows (0 struck, 1 medium →
low, 0 merged), the pre-merge reader's for 90–97, and the reviewer's for
the rest. HR-P2-adpt-43 was found by F1's fuzz target and named by
decision 47; the coordinator graded it low, as HR-P2-adpt-34. From 2026-11-24 the Fable roles
(verification reads, the reference checks, the pre-merge read) were run by
Opus (owner rule, `a814251`).

**1 row held back** (decision 37): phase 1's, published with the `0.7.0`
tag. HR-P2-data-30 was held at first and published by decision 36.

Totals after the merges: 0 high · 4 medium (one held; one found by the
pre-merge read in the fix wave and fixed before the merge) · 17 low ·
17 note.

| ID | sev | disposition | where |
| --- | --- | --- | --- |
| (held) HR-P1-auth-01 | medium | held | decision 37 |
| HR-P2-data-30 | medium | fixed `b3d8de2` | decisions 36, 38: the debounce fires at the latest 5 debounces after the first request |
| HR-P2-adpt-33 | medium | fixed upstream go-hamapping #42 (v0.7.0), pin `eae6cf6`; `dcb5268`, `edf7f43` | decisions 35, 47, 48: covers with tilt built; `open_tilt` / `close_tilt` → `open_cover_tilt` / `close_cover_tilt` |
| HR-P2-bound-90 | medium | fixed `07bd1bb` | decision 52, pre-merge read: an oversized state's entity is unmappable with a reason, not re-typed; its endpoint comes back on its number under the cap |
| HR-P2-adpt-34 | low | fixed upstream go-hamapping #42, pin `eae6cf6`; test `edf7f43` | 256 select options unmappable before the build |
| HR-P2-bound-37 | low | fixed `ac1464b`, `3aafa56` | decision 51: the read limit named in the sync's error, the `sync_failed` warning; no `READ_LIMIT` option |
| HR-P2-bound-38 | low | fixed `64830c9`, `9eb4169`, `b68d3d2`; completed by bound-90 | decision 42: 256 KiB per state, attributes dropped and logged; only the served states held |
| HR-P2-bound-39 | low | fixed `794f093` | Home Assistant's error text cut to 512 bytes |
| HR-P2-bound-40 | low | fixed `09b404a` | Home Assistant's candidates: 256 a session, 1 KiB each; ignored events are no activity |
| HR-P2-data-31 | low | fixed `1f14268`, `7da8a1b`; completed by data-94 | decision 41: control and bidi control characters dropped, refused in names and labels, `--dump-plan` quotes; closes HR-P1-data-36 |
| HR-P2-data-32 | low | fixed `f372730` | a select's option, a fan's preset, a climate's fan mode sent untrimmed |
| HR-P2-data-35 | low | fixed `ebe6c1a`; completed by data-91 | decision 40: a partial device, the refused entity unmappable with go-fabric's reason |
| HR-P2-test-42 | low | fixed `a00b8ac`, `b72ab7b`, `f07d05a` | decision 46: `FuzzPlanBuild` in `fuzz-smoke`; skips only go-hamapping #45's shapes |
| HR-P2-conc-60 | medium → low | fixed `66bf80c`, `d05c385`, `1a7dbff` | decision 39: the bridge's join bounded by the stop, camera closes off the sync, sessions end in parallel |
| HR-P2-err-61 | low | fixed `4f07038`, `3aafa56` | decisions 38, 50: retry 5 s doubling to 5 min; `sync_failed` from the second failure |
| HR-P2-bound-62 | low | fixed `5fd40b1`, `0681b86` | decision 38: one shared single-flight snapshot reader per connection |
| HR-P2-err-63 | low | fixed `39c726d`; test `d9622dc` (test-95) | decision 46: a dropped or refused command puts Home Assistant's state back |
| HR-P2-bound-64 | low | fixed `0c9b38b`, `b72ab7b` | decision 46: dropped work and commands as a rate-limited summary |
| HR-P2-data-91 | low | fixed `cce7717` | pre-merge read: an entity go-fabric refused is not served |
| HR-P2-doc-92 | low | fixed `7cbdb99` (docs, warning text) | pre-merge read: `sync_failed` says what a failed batch leaves |
| HR-P2-net-01 | note | documented `340f0ae`; measured in the owner's live Ingress test | decision 43: the T2 argument in `docs/operations.md` |
| HR-P2-frame-02 | note | open → the owner's live Ingress test | decision 43 |
| HR-P2-cfg-03 | note | fixed `5c32d54` | decision 44: the Supervisor gate keeps re-reading; a downgrade closes it |
| HR-P2-net-04 | note | no change | confirms T11: no LAN peer reaches `call_service` before commissioning |
| HR-P2-adpt-36 | note | fixed upstream go-hamapping #42 (v0.7.0), pin `eae6cf6` | ModeSelect labels equal after the cut are numbered |
| HR-P2-bound-41 | note | accepted | a cap per peer address later; a listener cap would let a refused peer shut the Ingress proxy out |
| HR-P2-doc-66 | note | documented `340f0ae`; follow-up go-ha2fabric #73 | decision 45: persist the served keys |
| HR-P2-err-65 | note | fixed `4f07038` | `Resync` says whether it planned |
| HR-P2-err-67 | note | fixed `2eae39e` | a cancelled sync or `call_service` is no failure |
| HR-P2-conc-68 | note | fixed `2eb7324` | a deleted node's store writes no file after its delete |
| HR-P2-conc-69 | note | fixed `59e0cb0` (comment) | the session cap holds under go-fabric's per-camera command lock |
| HR-P2-data-70 | note | no change | confirms T15's life-cycle half: no unquoted sink |
| HR-P2-doc-93 | note | fixed `7cbdb99` (docs) | one read shared by every node on the connection |
| HR-P2-data-94 | note | fixed `4a21330`, `df008ac` | an unsafe-only registry name falls through; labels checked at load (`nodes.label_refused`, not fatal) |
| HR-P2-test-95 | note | fixed `d9622dc` | the dropped command's attribute read back end to end |
| HR-P2-data-96 | note | accepted | the riders are dropped with the refused entity; the better cut is go-hamapping #46 |
| HR-P2-bound-97 | note | accepted (re-read of round 3) | a state flapping across the cap churns its endpoint at most once per debounce; a hysteresis is a follow-up candidate |
| HR-P2-adpt-43 | low (coordinator, as HR-P2-adpt-34) | fixed upstream go-hamapping #42, pin `eae6cf6`; the remaining shapes go-hamapping #45 | decisions 47, 49: inverted thermostat bounds unmappable |

Upstream: go-hamapping #42 (v0.7.0: adpt-33, -34, -36, -43 and
`BuildError`, merged), #43 (`stop_tilt`), #44 (window covers as
Rollershade, `PositionAwareLift` always; both decision 49, after the
owner's live tests), #45 (thermostat bounds beyond go-fabric's default
and ranges inside the deadband), #46 (`BuildError` names the refused
entity, data-96). Follow-up here: go-ha2fabric #73 (doc-66, the served
keys persisted). go-fabric: none new.

Standing guards added: `FuzzPlanBuild` (`internal/bridge`, Map → Build →
Apply over hostile entity data) and `FuzzClean` (`internal/text`), both
in `fuzz-smoke`; `FuzzEntityState` and `selection`'s `FuzzOpen` now
assert `text.Safe` names; `TestCoverMappableBuilds` in go-hamapping
(what Map calls mappable builds); the tilt fixtures `cover.tilt_blind`
and `cover.tilt_slats` in `testdata/ha/` with `TestCoverTilt`,
`TestCoverShapesBuild` and `TestSelect256Options`;
`TestDebounceHasALatestTime`, `TestOneEventOneSnapshot`,
`TestOversizedStateTakesTheEndpoint`, `TestRefusedEntityIsNotServed`;
and every fix's regression test (red seen, or stated otherwise in the
fix reports).

Guards at the pull request's head: see the pull request body.
