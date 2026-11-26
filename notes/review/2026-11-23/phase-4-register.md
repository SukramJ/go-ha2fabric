# Phase 4, run 1 — the dispositions

Branch `review/p4` on `main` at `c3a9770` (#82); register written at
`cfec2a2`. The pull request is squashed; its single commits stay
retrievable through the pull request's reference. Sources:
`phase-4-S-web.md` (IDs 01–05), `phase-4-S-ha.md` (20–24), `phase-4-P.md`
(40–43), `phase-4-D.md` (60–70), `phase-4-verify.md` (its grades win), the
pre-merge read (`phase-4-premerge.md`, IDs 90–99), the fix reports
`phase-4-fix-F1.md` (with its "Fix round" for 90–99) and
`phase-4-fix-F2.md`, `decisions.md` rows 68–78. The commits are
`review/p4`'s (`git log c3a9770..HEAD`); F2 worked on `review/p4-F2` and
its commits were cherry-picked, so F2's report's SHAs are replaced here by
the ones on `review/p4` (mapped by subject: `f53b4c0` → `82f3fe7`,
`712cdf1` → `95596da`, `8f3355c` → `d02ba21`, `1dc3f9a` → `db37549`,
`26d019a` → `8600017`). Every SHA below was checked with
`git show --stat` and is an ancestor of `cfec2a2`. Severity is the
verifier's for run 1 (the four challenged lows, a sample of seven lows and
a skim of every note: 0 re-graded, 0 struck, no new row 80–89) and the
pre-merge reader's for 90–99. The Fable roles (verification, the
pre-merge read) were run by Opus (owner rule 2026-11-24). Five agents for
the run, the security unit split upfront (decision 68).

**No row held back.** S-web, S-ha, P and D each state no held file (no
new exploitable security item). HR-P1-auth-01 stays open, published and
documented (decision 37 as amended on 2026-11-25, decision 53).

**go-fabric v0.27.0** lifted in this pull request (decision 69):
`fe9cc38` (go.mod, `licenses/`, every code citation re-pinned and
re-read), `d02ba21` (the docs' citations, `THIRD-PARTY-NOTICES.md`,
changelog); no deprecated identifier (SA1019, 0 lint issues);
go-fabric-ui v0.14.1 and go-hamapping v0.8.0 unchanged. The pre-merge
read re-read 24 ranges at v0.27.0: all hold.

Totals, run 1 after verification: 0 high · 0 medium · 11 low · 14 note =
25. Pre-merge read: 0 high · 0 medium · 6 low · 3 note = 9 (no ID 92).
Phase 4: 0 high · 0 medium · 17 low · 17 note = 34; closed as 32 fixed,
1 accepted (bound-98, its comment fixed), 1 upstream (adpt-42); none
open. The carried rows below are not counted again.

**Verdict: run 1 has no confirmed high or medium**, neither after
verification (`phase-4-verify.md`, "Run 1: any confirmed high or
medium?") nor in the pre-merge read of its fix wave (`phase-4-premerge.md`,
Counts). By README "Done" it is the review's convergence run: no run 2.
No tag with this pull request (decision 71); the owner decides 0.7.1 /
0.8.0 afterwards.

| ID | sev | disposition | where |
| --- | --- | --- | --- |
| HR-P4-cfg-01 | low | fixed `82f3fe7`; guard `4424a66` | decision 75: `ENV S6_CMD_RECEIVE_SIGNALS=1` in `addon/Dockerfile`'s final stage, so the stop budget is the daemon's 25 s, not s6-overlay's 3 s; `script/run.sh:7`, `addon/config.yaml`'s timeout comment, `docs/operations.md` corrected; `TestAddonStopSignal` (red with the `ENV` line removed; also holds `run.sh`'s `exec`); measured 23 s `clean exit` on the built add-on image; the owner's live check owed (below) |
| HR-P4-cfg-02 | note | fixed `82f3fe7` | `--stop-timeout 30` / Compose `stop_grace_period: 30s` in `Dockerfile`'s usage, `README.md`, `docs/operations.md` |
| HR-P4-sup-03 | low | fixed `95596da` | decision 75: `# syntax=` dropped from `Dockerfile`; the lint step refuses a syntax directive not pinned by `@sha256:` |
| HR-P4-sup-04 | note | fixed `95596da` | decision 75: lint step "one Go version in go.mod, both Dockerfiles and every workflow"; the `golang` patch auto-merge kept (its pull request is red until completed by hand) |
| HR-P4-cfg-05 | note | fixed `507d628` | `WEB_ALLOWED_HOSTS` refuses white space as go-fabric-ui does; `FuzzLoad` checks with `fabricui.NewHandler` |
| HR-P4-bound-20 | low (fix adjusted by the verifier) | fixed `ac4e41b` (host share); docs `db37549`, `by_design` `2d53731`; upstream SukramJ/go-hamapping#50, follow-up thingfabric/go-ha2fabric#83 | decisions 72, 76: eviction only within the offering fabric, a per-fabric share of 4 of 8, otherwise RESOURCE_EXHAUSTED (supersedes decision 23's eviction rule); the third-fabric lock-out carried into go-hamapping#50 (a total that follows the share, or a fair-share eviction); #83 passes the kit's `MaxSessions`/`MaxSessionsPerFabric` through the camera row after #50, and the host's share goes |
| HR-P4-bound-21 | note | fixed `825a55a`; docs and changelog `5a33929` | `MaxCandidateBytes` counts the line plus `sdpMid`, both directions; a controller's longer one CONSTRAINT_ERROR (see bound-98) |
| HR-P4-data-22 | note | fixed `6c64417` | U+2028 and U+2029 unsafe, cleaned to a space |
| HR-P4-cite-23 | note | fixed `ac4e41b` | `MaxSessions`' comment names the kit's `MaxSessions`/`MaxSessionsPerFabric` at v0.27.0 and go-hamapping's camera row that does not set them |
| HR-P4-doc-24 | note | fixed `82f3fe7` | `docs/operations.md`, The token: `wss://` with a private CA through `SSL_CERT_FILE` / `SSL_CERT_DIR` |
| HR-P4-err-40 | low | fixed `55859bd`, `d7e083f`; test `6743f75` | decision 73: a stale plan for exactly the refused keys, live (states, reachability, the read model's endpoint, the command gate) until a batch applies them; completes HR-P1-err-31; the plan kept open is held by test-91's seam |
| HR-P4-err-41 | low | fixed `4ec34a3`; changelog `5a33929` | the pre-Map verdicts carry their reason in `Full`; `TestItemKeepsPlanReason` |
| HR-P4-adpt-42 | low (I) | upstream thingfabric/go-fabric#633 | decision 75: an empty retry batch does not reassemble, a failed reassembly is never redone; a go-fabric issue beside #616, outside the fix wave |
| HR-P4-bound-43 | note | fixed `35102ff`; changelog `5a33929` | the group signature in one grown builder (same string), `BenchmarkAssessManyEntities` (3.46 s → 0.09 s per op) |
| HR-P4-test-60 | low | fixed `ec824ac`, lint `f9b484b` | decision 70, the carried test-helper row (below): `do` returns the client's error, `nodeStart` 90 s bounds requests and waits, a client per daemon without keep-alive; `TestDaemonDoKeepsTheError` |
| HR-P4-test-61 | low | fixed `86f0ae6` | `hatest` answers copies made under its lock; `TestAnswersAreCopies` (DATA RACE before) |
| HR-P4-test-62 | low | fixed `2b72e1a` | decision 75: `internal/ha` floor 94, the floors header's spread named; `TestHAFloorHoldsItsSpread`; deterministic tests of the timing branches left as the lasting fix |
| HR-P4-test-63 | low | fixed `4756103`, `78c4393` | `script/cite` reports a pinned path the named module does not have ("not in go-fabric at v0.27.0"); the verifier's mutation exits 1 |
| HR-P4-err-64 | low | fixed `79268ce`, `d7e083f`; docs `db37549`, `5a33929` | decision 74: a Delete whose stop fails is refused (500), record, files and member kept, a second Delete retries; the node has left its fabrics already (doc-94) |
| HR-P4-conc-65 | note | fixed `79268ce`, `d7e083f`; completed `375c4c4` | Create's rollback stop bounded by `stopBudget`; its failure handled by err-93 |
| HR-P4-err-66 | note | fixed `79268ce` | an erase stops a node an earlier erase kept stale, or refuses before touching the run; `TestEraseStopsAnEarlierStaleNode` |
| HR-P4-test-67 | note | fixed `cb7b540` | `TestVersionBannerFindsItsModules` in a binary that links go-fabric and go-hamapping |
| HR-P4-test-68 | note | fixed `86f0ae6` | MATCH_ALL, a lossless `WaitCall`, the still's own type; the one difference left (core's 401 before the entity check) named in a comment |
| HR-P4-conc-69 | note | fixed `86f0ae6` | a read that began in the trigger's tick is not reused; `TestSnapshotsSameTickIsNotReused` |
| HR-P4-sup-70 | note | fixed `923785a` | decision 75: `gosec` on `cmd/`, one reasoned `nolint` (G101 on a warning code) |
| HR-P4-test-90 | low | fixed `730821f` | pre-merge read: `TestEvictionTakesTheFabricsOwnIdlest` on a plain upstream, deterministic (`-race -count=50` ok) |
| HR-P4-test-91 | low | fixed `6743f75` | pre-merge read: the `closePlan` seam; the stale plan not closed while refused, the refused build's plan closed once, the stale plan once when the Replace applies; the reader's mutation fails it |
| HR-P4-err-93 | low | fixed `375c4c4` | decision 77: a rollback whose stop fails hands the node to `fleet.orphans`, keeps its files, warns; `fleet.Close` stops it; `TestCreateRollbackStopFails` at the call site |
| HR-P4-doc-94 | low | fixed `5a33929` | pre-merge read: `docs/operations.md`, `addon/DOCS.md`: a refused Delete has left the fabrics already |
| HR-P4-doc-95 | low | fixed `5a33929` | pre-merge read: the candidate bullet both ways with `sdpMid`; F1's missing changelog lines (err-41, bound-43, conc-65/err-66/err-93) |
| HR-P4-doc-96 | note | fixed `5a33929` | pre-merge read: `camera.session_evicted` logs `fabric` |
| HR-P4-doc-97 | low | fixed `2d53731`, `c018dc6` | pre-merge read: `docs/by_design.md`, "Cameras behind an option": matter-hub (`664575a`) bounds no WebRTC session; here 8 per camera, 4 per fabric, eviction within the fabric (decisions 23, 72) |
| HR-P4-bound-98 | note | accepted (decision 78); comment `18bfe8b`, docs `5a33929` | CONSTRAINT_ERROR kept for a controller's candidate over 1024 bytes; the comment and docs say the bound is the host's, not the spec's |
| HR-P4-test-99 | note | fixed `bd88458` | pre-merge read: `eventually` clears the last client error first |

Carried rows:

| ID | sev | disposition | where |
| --- | --- | --- | --- |
| test helper of phase 3 (`daemon.call` drops the client's error, status 0; phase 3's Windows flake of `TestNodeRegistry`) | low | fixed as HR-P4-test-60 (`ec824ac`, `f9b484b`) | decision 70: in phase 4's fix wave |
| HR-P1-auth-01 | medium | open, published `da9d940`, documented `bcf1ad8`, `2b9da12` | decisions 37 (amended 2026-11-25: docs only, no Supervisor issue), 53; unchanged in phase 4; the fix needs a verifiable secret from the Supervisor |
| HR-P1-sup-18 / T25 | low | open → issue #72 | decisions 34, 70: #72 stays outside this wave; unchanged |

Upstream: thingfabric/go-fabric#633 (adpt-42: a failed reassembly after
Batch is never redone); SukramJ/go-hamapping#50 (bound-20: pass the kit's
`MaxSessions` and `MaxSessionsPerFabric` from the host through the camera
row; decision 76's lock-out). Follow-up here: thingfabric/go-ha2fabric#83
(the session bounds through the camera row, after go-hamapping#50); #71,
#72, #73 unchanged, each its own wave (decision 70).

Standing guards added: `TestAddonStopSignal` (the add-on's
`S6_CMD_RECEIVE_SIGNALS=1` and `run.sh`'s `exec`); the lint job's
syntax-directive pin and its one-Go-version check; `gosec` on `cmd/`;
`script/cite`'s missing-file check (`TestCites` rows); the `internal/ha`
floor with its spread (`TestHAFloorHoldsItsSpread`);
`TestVersionBannerFindsItsModules`; `TestNoEmptyKey` (P's
`ErrNoStableKey` probe as a standing test, on go-fabric's facade);
`BenchmarkAssessManyEntities`; `TestEvictionStaysInTheFabric`,
`TestEvictionTakesTheFabricsOwnIdlest`, `TestFabricShareRefused`,
`TestCandidateMidIsBounded`; `TestRefusedReplaceKeepsServedEndpointLive`
with the `closePlan` seam; `TestDeleteWhoseStopFailsIsRefused`,
`TestCloseRetriesAFailedStop`, `TestEraseStopsAnEarlierStaleNode`,
`TestCreateRollbackStopFails`; `TestDaemonDoKeepsTheError`;
`TestAnswersAreCopies`; `TestSnapshotsSameTickIsNotReused`;
`TestItemKeepsPlanReason`; `FuzzLoad` against go-fabric-ui; and every
fix's regression test (red seen, or stated otherwise in the fix reports).

## Done check

README "Done" (`README.md:235-241`), its four conditions:

1. **Every finding down to note has a closing status.** Phase 4: the 34
   rows above (fixed, accepted or upstream; none open). Phases 0–3: their
   registers; the rows still open are open by decision with an issue or a
   documented status (HR-P1-auth-01, decisions 37, 53; HR-P1-sup-18 → #72).
2. **Every defect class found has a standing guard** in `make ci` or a
   workflow: phase 4's under "Standing guards added" above; phases 0–3's
   in their registers.
3. **A convergence run over the then-current HEAD yields no new high or
   medium:** run 1 over `c3a9770` (verification: 0 high, 0 medium) and the
   pre-merge read of its fix wave (0 high, 0 medium).
4. **The held-back rows are published with the pull request that carries
   their fix:** every held report of phases 0–2 published in phase 3
   (decision 53, `da9d940`); phase 4 holds none back.

## Owed outside this pull request

- **The owner's live check of the bind** (decision 54, HR-P0-cfg-27) on
  the next release: the log shows `ha2fabric.web_listen` with the
  Supervisor's address of the add-on (e.g. `listen=172.30.32.1:8486`), and
  Ingress works.
- **The owner's live check of the add-on's stop** (HR-P4-cfg-01,
  `S6_CMD_RECEIVE_SIGNALS`) on the next release: stopping the add-on, the
  log shows the node's stop lines after `ha2fabric.stopping`, not a cut.
- **The first real run of attest and SBOM** on the next tag (HR-P3-sup-36;
  decision 71: no tag with this pull request); `gh attestation verify`
  afterwards.
- **The owner's in-place production update 0.6.3 → 0.7.0** under the old
  repository address (decision 56, HR-P3-doc-30), the result recorded — if
  not yet recorded.

Guards at the pull request's head: see the pull request body.
