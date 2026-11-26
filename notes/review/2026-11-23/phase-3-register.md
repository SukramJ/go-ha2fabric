# Phase 3 — the dispositions

Branch `review/p3`, rebased onto `main` at `4f5f6e3` (#81); the pull
request is squashed, its single commits stay retrievable through the pull
request's reference. Sources: `phase-3-A.md` (IDs 01–15), `phase-3-B.md`
(30–50), `phase-3-C.md` (60–68), `phase-3-verify.md` (its re-grades win),
the pre-merge read (`phase-3-premerge.md`, IDs 90–99), the fix reports
`phase-3-fix-F1.md`, `phase-3-fix-F2.md`, `decisions.md` rows 53–67,
`held-reports/README.md`. The commits are `review/p3`'s after the rebase
(`git log 4f5f6e3..HEAD`); the two fix reports' SHAs were replaced by
script with the rebased ones (mapped by commit subject). The reports'
baseline `0a47095` is on `main`; the pre-merge read's HEAD `9d301eb` is
`55c80b0` here. Severity is the verifier's for every medium and the sampled
lows (0 struck, 1 low → note: sup-33; one sub-claim of doc-38 struck), the
pre-merge reader's for 90–99, and the reviewer's for the rest. The Fable
roles (verification, the pre-merge read, units A and C) were run by Opus
(owner rule 2026-11-24).

**No row held back.** Phase 3 has no held file; decision 53 publishes every
held report of phases 0–2 (`held-reports/`, `da9d940`). HR-P1-auth-01 stays
open, published and documented (decision 37 as amended on 2026-11-25,
decision 53).

Totals after verification, phase 3's own rows: 0 high · 5 medium (doc-30
fixed by #81 ahead of this pull request) · 21 low (four from the pre-merge
read) · 29 note (six from the pre-merge read) = 55. The six carried rows
of phases 0–2 below are not counted again.

| ID | sev | disposition | where |
| --- | --- | --- | --- |
| HR-P3-test-01 | medium | fixed `bc0de68`, `8a34a00`; floors `4f73c04`, `55c80b0`, `2b9da12` | decision 60: a floor more than 3 points under the measurement and a package without a floor fail; floors at the whole percent below the lowest run |
| HR-P3-doc-30 | medium | fixed `4f5f6e3` (#81, ahead of this pull request) | decision 56: keep the old repository address and update in place; the owner's live update 0.6.3 → 0.7.0 is owed (below) |
| HR-P3-sup-31 | medium | fixed `8461530`; required checks owed (below) | decision 57: patch updates only, the family never; `coverage`, `fuzz-smoke`, `docker` required |
| HR-P3-sup-32 | medium | fixed `3640556`, `9d4513d`; ruleset and environment owed (below) | decisions 58, 65, 66: a tag must be on `main`; no manual rebuild; `ghcr` for `v*` tags |
| HR-P3-sup-37 | medium | fixed `913d8a2`, `03e8b1f`, `76d2b59`, `ecf166e` | decision 59: the generated `licenses/` tree (modules, Go, go-fabric's NOTICE, the UI's npm licences) in the archives and both images; `licenses-check` in CI |
| HR-P3-test-02 | low | fixed `8544480` | `TestClientBackoffResetsAfterLongConnection` bites (red with the reset removed) |
| HR-P3-api-04 | low | fixed `8ab9cb5` | decision 60: `TestPure` (standard-library allowlist, no `go` statement) |
| HR-P3-api-05 | low | fixed `8ab9cb5` | decision 60: `adaptGoFabric` freezes adapt's go-fabric imports |
| HR-P3-api-07 | low | fixed `8ab9cb5` | decision 60: `TestEnvironment`, environment reads only in `internal/config` |
| HR-P3-test-10 | low | fixed `b58361a` | the fake refuses as core 2026.10.3; `TestFakeRefusesAsCore`, `TestCameraOfferRefusedEndToEnd` |
| HR-P3-sup-34 | low | fixed `03e8b1f`, `76d2b59`, `9d4513d` | decisions 62, 64: base images by digest, Dependabot `docker`, the lint job's grep; the add-on for amd64 and aarch64 only; closes HR-P1-sup-19 (d) |
| HR-P3-sup-35 | low | fixed `76d2b59` | decision 62: every action by full SHA, held by the lint job |
| HR-P3-sup-36 | low | fixed `3640556`; docs `bcf1ad8` | decision 62: `actions/attest` provenance and SBOM for the archives and images; first real run on the next tag (below) |
| HR-P3-doc-38 | low (one sub-claim struck) | fixed `913d8a2` | the notices at `go.mod`'s paths and versions, held by `licenses-check`; "names neither go-humanize nor bigfft" struck by the verifier |
| HR-P3-doc-39 | low | fixed `b93fb2e` | "Mapped today" from go-hamapping v0.8.0's rows |
| HR-P3-doc-41 | low | fixed `b93fb2e` | the 0.7.0 line: no longer activity for the eviction |
| HR-P3-sup-44 | low | fixed `2e908b7`; secret scanning owed (below) | decision 58: `SECURITY.md`; scanning and push protection by setting |
| HR-P3-cite-60 | low | fixed `b221a5e`, `b2bcc99`, `1957bd3`; guard `e258c5b`, `2b9da12` | decision 61: go.mod's version on every go-fabric citation, `make cite` in `make ci` and the lint job |
| HR-P3-cite-61 | low | fixed `b221a5e` | decision 61: go-fabric-ui v0.14.1 ranges |
| HR-P3-cite-62 | low | fixed `b221a5e`, `1957bd3` | decision 61: go-hamapping v0.8.0 ranges |
| HR-P3-cite-63 | low | fixed `b2bcc99` | matter-hub's composition gates as read at `664575a` |
| HR-P3-cite-64 | low | fixed `b2bcc99` | the adapter's Matter knowledge by symbol |
| HR-P3-sup-90 | low | fixed `2b9da12` | pre-merge read: `make cite` in the lint job; `TestRepository` fails instead of skipping under `CI` |
| HR-P3-doc-91 | low | fixed `2b9da12` (docs) | pre-merge read: the Ingress session is the Supervisor's to check |
| HR-P3-doc-92 | low | fixed `2b9da12` (docs) | pre-merge read: `hassio_api` reaches Home Assistant only with a role above `default` |
| HR-P3-chg-93 | low | fixed `2b9da12` | pre-merge read: F1's changelog lines (ha-09, cfg-68) |
| HR-P3-test-03 | note | accepted; comment `5fe84c3` | decision 63: no clock seam for a timer that no longer exists (decision 31) |
| HR-P3-api-06 | note | fixed `8ab9cb5` | decision 60: every package directory needs a layer entry |
| HR-P3-api-08 | note | fixed in part `287fef0`, `db08b7d`; the rest accepted | decision 63: `TypeResult` kept (ha-09 uses it); flag defaults from the constants; no other unexport |
| HR-P3-ha-09 | note | fixed `287fef0` | a reply that is not a result with `success` (or a ping's `pong`) fails its call; `TestClientReplyShape` |
| HR-P3-test-11 | note | fixed `8544480`; one precondition accepted `4f73c04` | waits on the seam's entry; `TestCallContextKeepsConnection`'s precondition stays a log (elapsed time does not show a write in progress) |
| HR-P3-test-12 | note | fixed `68e5cb8` | `TestSupervisorGateBadAnswers` |
| HR-P3-test-13 | note | fixed `8ab9cb5` | decision 60: `TestFuzzTargets`, both ways |
| HR-P3-test-14 | note | fixed `68e5cb8` | `TestImageListenProblems`: both `ENV` forms, the flag in `ENTRYPOINT`/`CMD` |
| HR-P3-test-15 | note | accepted (seam and test dropped `e45d198`, floor `55c80b0`) | decision 67: no real input reaches the whole-device branch at go-hamapping v0.8.0; the branch stays as defensive code (pre-merge read) |
| HR-P3-sup-33 | low → note | settings: required checks bound to app 15368 owed (below) | decisions 58, 63; family consistency with go-fabric-ui's UR-P3-sup-64 |
| HR-P3-doc-40 | note | fixed `b93fb2e` | README status, `make build`, the import rule, a `NODE_LIMIT` row |
| HR-P3-doc-42 | note | fixed `06ebd36` | roadmap markers `[x]` with their release; "Left:" lines kept |
| HR-P3-doc-43 | note | fixed `06ebd36` | a web UI node's rule is its selection alone; `CONCEPT.md` status 0.7.0 |
| HR-P3-sup-45 | note | fixed `76d2b59`; the required check owed (below) | CodeQL for `actions` beside `go` |
| HR-P3-sup-46 | note | fixed `3640556`, then gone with the dispatch `9d4513d` | decision 65: no `workflow_dispatch` input left |
| HR-P3-sup-47 | note | fixed `8461530` | the gate reads the pull request's author and the actor; `reviewers:` replaced by `.github/CODEOWNERS` |
| HR-P3-sup-48 | note | fixed in part `03e8b1f`; the rest open → issue #72 (comment owed, below) | `BUILD_VERSION` from `VERSION`, OCI labels; the user drop, `chown`, AppArmor, watchdog and the bind are #72's |
| HR-P3-sup-49 | note | documented `bcf1ad8` | README and `SECURITY.md`: the `ghcr.io/sukramj/` images stay at 0.6.3; a package description is the owner's |
| HR-P3-doc-50 | note | fixed `2e908b7` | `repository.yaml`: `SukramJ <me@sukramj.dev>` |
| HR-P3-cite-65 | note | fixed `b221a5e`, `b2bcc99` | the narrow core ranges; `docs/reports/H2.md` left as history |
| HR-P3-cite-66 | note | fixed `b2bcc99`; roadmap out of `make cite` `ecf166e` | decision 67: finished survey items pinned at go-hamapping v0.2.1, open items at v0.8.0; the roadmap is history like `docs/reports/` |
| HR-P3-cite-67 | note | fixed `b2bcc99`, `913d8a2`, `b221a5e` | `CLAUDE.md`'s state, the notices' catalog entry, `config.go`'s Ingress pin |
| HR-P3-cfg-68 | note | fixed `4d2da0a` | decision 63: `label: str(1,32)` in the schema; the daemon's byte and blank checks stay; closes HR-P1-cfg-21 (b) |
| HR-P3-cfg-94 | note | accepted | pre-merge read: only an address not local to the host fails the bind (exit 1, fails closed); a host-network add-on gets the host's `hassio` bridge |
| HR-P3-test-95 | note | fixed `2b9da12` | pre-merge read: `internal/ha`'s floor 93 from the measured 1.8-point spread |
| HR-P3-sup-96 | note | accepted | pre-merge read: decision 57 says "patch updates only", which takes an action's SHA or a base digest too; the owner knows by this row |
| HR-P3-sup-97 | note | accepted | pre-merge read: only an admin creates a `v*` tag (decision 66), one at a time; `:latest` order needs no shared concurrency group |
| HR-P3-sup-98 | note | accepted | pre-merge read: the amd64 SBOM is right for every arch while the linked module set is identical (measured); per-arch SBOM if one becomes arch-specific |
| HR-P3-api-99 | note | accepted | pre-merge read: the environment guard is a tripwire; nothing dot-imports `os`/`syscall` or uses `golang.org/x/sys/unix` |

Carried rows of phases 0–2:

| ID | sev | disposition | where |
| --- | --- | --- | --- |
| HR-P0-cfg-27 | low | fixed `4d2da0a`; docs `bcf1ad8` | decision 54: the add-on binds the web UI to the Supervisor's `ip_address` of the add-on, today's bind with a warning when it cannot be read; the owner's live check of the bind is owed (below) |
| HR-P1-sup-19 (d) | note | fixed via HR-P3-sup-34 (`03e8b1f`, `76d2b59`) | base images by digest, Dependabot `docker` |
| HR-P1-cfg-21 (b) | note | fixed via HR-P3-cfg-68 (`4d2da0a`) | the Supervisor's schema read at `fe91b8db` |
| HR-P2-frame-02 | note | closed by the owner's live Ingress test of 2026-11-25 | decision 43: passed — an administrator has full access through Ingress, non-administrators do not see the panel, no framing errors |
| HR-P1-sup-18 / T25 | low | open → issue #72 | decision 34; the comment text is `phase-3-fix-F2.md`, "HR-P3-sup-48" |
| HR-P1-auth-01 | medium | published `da9d940`, open, documented `bcf1ad8`, `2b9da12` | decisions 37 (amended 2026-11-25: docs only, no Supervisor issue), 53; `docs/operations.md` "Co-installed add-ons"; the fix needs a verifiable secret from the Supervisor |

Upstream: none new (unit C: every go-fabric, go-fabric-ui and go-hamapping
fact holds at the new pins; only this repository's ranges drifted).
Follow-up here: go-ha2fabric #72 (sup-48, HR-P1-sup-18 / T25), #71
(`--health`, which #72's watchdog waits for). go-fabric: none.

Standing guards added: `script/coverfloor`'s stale-floor and missing-floor
checks (`TestCheck`) with a floor for every package; the AST guards of
`internal/arch` (`TestPure`, `adaptGoFabric`, the layer entry for every
package, `TestEnvironment`, `TestFuzzTargets`, `TestASTGuardsRefuse`);
`script/cite` as `make cite` in `make ci` and the lint job (`TestCites`,
`TestRepository`); `script/licenses/generate.sh --check` in the security
job, `licenses/MODULES.txt` checked in every archive and both images; the
lint job's grep for actions by full SHA and base images by digest; CodeQL
for `actions`; Dependabot for `docker`; `TestAddonAddress`,
`TestAddonWebListen`, `TestClientBackoffResetsAfterLongConnection`,
`TestClientReplyShape`, `TestFakeRefusesAsCore`,
`TestCameraOfferRefusedEndToEnd`, `TestSupervisorGateBadAnswers`,
`TestImageListenProblems`; and every fix's regression test (red seen, or
stated otherwise in the fix reports).

## Owed outside this pull request

- **Repository settings** (`phase-3-fix-F2.md`, "Repository settings for
  the coordinator"; decisions 57, 58, 66), run by `gh api` only after the
  owner confirms the exact calls: the `v*` tag rulesets (update, deletion,
  force-move; creation by admins), the `ghcr` environment for `v*` tags
  before the first tag after the merge, required checks bound to app 15368
  with `coverage`, `fuzz-smoke`, `docker` (sup-31, sup-33), secret scanning
  and push protection (sup-44), the two "consider" calls as the owner
  decides. The `codeql (actions)` required check only after it has
  reported once (a required context that never reports blocks every
  merge).
- **The #72 comment** (`phase-3-fix-F2.md`, "HR-P3-sup-48"): what is still
  undecided for the non-root add-on, including the bind of decision 54.
- **The owner's live check of the bind** (decision 54, HR-P0-cfg-27): the
  log shows `ha2fabric.web_listen` with the Supervisor's address of the
  add-on (e.g. `listen=172.30.32.1:8486`), and Ingress works.
- **The owner's in-place production update 0.6.3 → 0.7.0** under the old
  repository address (decision 56, doc-30), the result recorded.
- **The first real run of attest and SBOM** on the next tag (sup-36): the
  attest steps, the `ghcr` environment and the SBOM step run only on
  GitHub; `gh attestation verify` afterwards.

Guards at the pull request's head: see the pull request body.
