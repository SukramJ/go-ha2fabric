# Phase 3 fix wave — pre-merge read

Reader: the plan's Fable role, run on Opus (owner rule 2026-11-24). Branch
`review/p3`, HEAD `9d301eb`; diff `0a47095..9d301eb` without `notes/`, plus
`held-reports/`, `phase-3-fix-F1.md`, `phase-3-fix-F2.md`, decisions 53–67,
the brief `notes/briefs/2026-11-26-review-p3-fix.md`. Marks: **M** measured
(a line read at the pin, a probe run in the scratch copy `git archive
9d301eb` under the session's scratchpad `p3/P`), **I** inferred. Supervisor
read at `fe91b8db` with `gh api`. No `make` in the worktree.

## Verdict

**Ready to merge.** No high or medium finding. The lows (90–93) are
small text and workflow edits worth one commit before the merge; the notes
(94–99) need nothing for this pull request.

## Findings

| ID | Sev. | Where | Claim | Fix |
| --- | --- | --- | --- | --- |
| HR-P3-sup-90 | low (M) | `.github/workflows/ci.yml` (no step), `script/cite/main_test.go:170-177` | Decision 61 puts the citation check "in `make ci`", and it is there (`Makefile` `ci:`), but no CI job runs `make ci` or `make cite`: the docs' citations (`CITE_PATHS`' six doc files) are checked only locally. The code paths are covered by `TestRepository` in `go test ./...`, but it `t.Skip`s when `go mod download` fails, so a broken module read passes silently. | Add a `make cite` step to the `lint` job (after setup-go; the module cache is restored there); make `TestRepository` fail instead of skip when `CI` is set. |
| HR-P3-doc-91 | low (M) | `docs/operations.md` (Co-installed add-ons, "the Ingress session is Home Assistant's to validate") | The Supervisor validates the Ingress session itself: `sup:supervisor/api/ingress.py:181-183` (`request.cookies.get(COOKIE_INGRESS)` → `self.sys_ingress.validate_session`). The conclusion holds (the add-on has nothing it can verify), the attribution does not. The held report `phase-2-X1.md` has the same words ("Core's to validate"); it stays unchanged by decision 53. | "the Ingress session is the Supervisor's to check, and the add-on has no call that checks it". |
| HR-P3-doc-92 | low (M) | `docs/operations.md`, same paragraph, "(one with `homeassistant_api` or `hassio_api` reaches Home Assistant itself without this)" | `hassio_api` alone does not reach Home Assistant: with the default role an add-on may read only `/.+/info` (`sup:supervisor/api/middleware/security.py` `role_access[ROLE_DEFAULT]`); `/core/.+` and `/homeassistant/.+` are the `homeassistant`, `manager` and `admin` roles'. The parenthesis overstates what an untrusted add-on already has. | "(one with `homeassistant_api`, or `hassio_api` with a role above `default`, reaches Home Assistant itself …)". |
| HR-P3-chg-93 | low (M) | `changelog.md` `[Unreleased]` | F1's lines are missing: ha-09 (a reply without `type: result` and `success` now fails its command, `internal/ha/client.go:565-586`) and cfg-68 (`label: str(1,32)`: the Supervisor now refuses an empty or a longer-than-32-character label in the options form, a user-visible change). F2 left them to the coordinator. | Add F1's two lines (its report, "For F2", Changelog). |
| HR-P3-cfg-94 | note (I) | `cmd/ha2fabric/supervisor.go:353-384`, `main.go:170-189` | The fallback covers a failed *read* only. A Supervisor answer whose address is not local to the host makes the bind fail and the daemon exit 1 (web error → `code = 1`), so a restart loop instead of today's bind. Fails closed, and for a host-network add-on the value is `network.gateway`, the host's `hassio` bridge (`sup:supervisor/docker/app.py:188-191`), so not expected. | Accept; or on a bind error with the Supervisor's address retry once with the given `WEB_LISTEN` and the same warning. |
| HR-P3-test-95 | note (I) | `script/coverfloor/floors.txt` (`internal/ha 94`, header "spread 0.3-0.5") | F1 measured `internal/ha` at 96.5 / 95.9 / 94.7 (a 1.8-point spread); the floor is 0.7 under the lowest run, so a CI run can go red without a change. The header's spread is stale for that package. | Watch the first CI `coverage` runs; lower to 93 with the measured spread as the reason if it flakes; fix the header. |
| HR-P3-sup-96 | note (M) | `.github/workflows/dependabot-auto-merge.yml` | The auto-merge also takes patch updates of the `github-actions` and `docker` groups (an action's new SHA, a Home Assistant base digest), not only Go modules. That is decision 57's wording ("patch updates only"), so no defect; the owner should know a base image or an action can land without a review. The family exclusion holds: the `family` group is first and matches both domains; the step refuses any `dependency-names` under them. | None, or restrict the step to `package-ecosystem == gomod`. |
| HR-P3-sup-97 | note (I) | `docker-build-push.yml`, `addon-image.yml` (`:latest`) | Concurrency is per ref; two tags pushed in quick succession run in parallel, and an older tag's job that checked out before the newer tag existed can push `:latest` after it. | Accept (an admin pushes tags one at a time); or a `concurrency` group shared by all tags without cancel. |
| HR-P3-sup-98 | note (M) | `release-on-tag.yml`, `docker-build-push.yml` (SBOM step) | The SBOM is built for `GOARCH=amd64` and attested for every archive and for the multi-arch index. Measured: the linked module set is identical for amd64, arm64 and arm today (same `go list -deps` module list), so the SBOM is correct now; a future arch-specific dependency would make it wrong for the others. | None now; per-arch SBOM if a dependency becomes arch-specific. |
| HR-P3-api-99 | note (M) | `internal/arch/ast_test.go:116-140` | The environment guard matches `os.*`/`syscall.*` selectors by import name; a dot import of `os` or `golang.org/x/sys/unix.Getenv` passes. Nobody does either; the guard is a tripwire, not a proof. | None, or refuse dot imports of `os`/`syscall` and add `golang.org/x/sys/unix`. |

## What was checked and holds

**Decision 54's bind (priority 1).**
- M: `/addons/self/info` is reachable with the add-on's token without
  `hassio_api`: `security.py` v1 `api_bypass` has
  `/addons/self/(?!security|update)[^/]+`, checked before the
  `access_hassio_api` branch in `check_principal_access`. Route
  `/addons/{app}/info` → `info_data_v1` → `info_data`, which carries
  `ATTR_IP_ADDRESS: str(app.ip_address)` (`api/apps.py:308`, `:318-323`);
  for `host_network` that is `self.sys_docker.network.gateway`
  (`docker/app.py:188-191`); the proxy dials `http://{app.ip_address}:{ingress_port}`
  (`api/ingress.py:125-127`).
- M: every failure of the read (error, non-200, redirect, result ≠ ok,
  unparsable or `0.0.0.0` address, cancelled context) keeps `WEB_LISTEN` as
  given with one `WARN`: exactly the previous behaviour. The client filter
  (`WEB_ALLOW=172.30.32.2`) is untouched; the read uses the existing
  Supervisor client (no proxy from the environment, no redirect, 10 s
  timeout). Worst start delay of the web UI: 3 × 10 s + 2 s; the nodes
  start before it. No failure path is more open than before.
- M: `TestAddonAddress`, `TestAddonWebListen`, the gate tests pass in the
  scratch copy.

**Workflows (priority 2).**
- M: the tag-on-main check uses `git merge-base --is-ancestor` on the
  checked-out commit (`git rev-parse HEAD`, or `GITHUB_SHA` in the release
  job); `merge-base` peels an annotated tag either way, the checkout has
  `fetch-depth: 0` and `main` is fetched explicitly, so neither a
  lightweight/annotated difference nor a shallow history bypasses it.
  Version must also equal `addon/config.yaml` and `internal/version`.
- M: untrusted input reaches `run:` only through `env` (`GITHUB_REF_NAME`,
  `dependency-names`, the PR URL); the version is regex-checked in the
  image workflows and equality-checked in the release job before use.
- M: permissions: top level `contents: read`; `id-token`, `attestations`,
  `artifact-metadata` (and `packages`/`contents: write`) only on the
  publishing jobs, matching `actions/attest`'s README at the pinned SHA;
  inputs `subject-path`, `subject-name`, `subject-digest`, `sbom-path`,
  `push-to-registry` exist in its `action.yml`.
- M: SHA pins resolve to the commented tags (4 checked:
  checkout v7.0.1, attest v4.2.2, fetch-metadata v3.1.0, codeql-action
  v4.39.3). The lint job's pin grep refuses an unpinned `uses:`/`FROM`.
- M: the SBOM step F2 could not run works: `cyclonedx-gomod` v1.12.0
  `app -json -licenses -std -main cmd/ha2fabric` on the scratch copy (made
  a git repository with a tag, as the checkout is) → CycloneDX 1.6, 20
  components, main component version from the tag. Without a git
  repository it fails ("no git repository found"); both jobs check out with
  history, so this holds in CI.

**Dockerfiles (priority 3).** M: every digest is an OCI index (golang,
distroless, both Home Assistant bases), and the two base digests equal
today's `3.24-2026.11.1` tags. Per-arch stages `base-amd64`,
`base-aarch64`, `FROM base-${BUILD_ARCH}`; `LICENSE`,
`THIRD-PARTY-NOTICES.md` and `licenses/` copied into both; the daemon image
runs as `nonroot`; the add-on stays root (pre-existing, issue #72).

**Docs and held reports (priority 4).** M: the nine published held reports
are byte-identical to `held/*.md`; the README's status rows and the
"interception measured, full forged exchange not completed" label match
`phase-2-X1.md` and decision 53. The operations paragraph does not
overclaim the bind ("takes the listener off the LAN's addresses; does not
stop an add-on on the Supervisor network"), cites `app.py:402-407`
correctly (the `NET_RAW` comment quoted verbatim) and gives no advice that
weakens security. Two wording errors: 91, 92.

**Guards (priority 5).** M, in the scratch copy:
`script/licenses/generate.sh --check` exit 0; one changed byte in
`licenses/` → exit 1. `go run ./script/cite $(CITE_PATHS)` with
`GOPROXY=off` exit 0 (no network needed with a warm module cache, which
the build needs anyway); a line range past the file's end → exit 1; a
`v0.25.0` pin → exit 1 "names v0.25.0, go.mod has v0.26.0".
`go test ./internal/arch ./script/...` pass. The generator, the cite
check and the AST guard are deterministic (sorted module list, fixed walk
order, no clock). The CI gap is 90.

**F1's question, the whole-device build refusal.** M: go-hamapping
v0.8.0's `Plan.Build` (`plan.go:420-451`) ignores its context and returns
a plain error only for "plan built twice"; every endpoint refusal is a
`*BuildError`. So the branch `bridge.go:1072-1081` is not reachable from
today's inputs. **Keep it: defensive, not dead.** It is also the retry
loop's termination path when a `*BuildError` names no entity of the group
(`len(rest) == len(entities)`), and `Build`'s signature allows any error;
removing it would turn a future plain error into a device with neither
endpoints nor a verdict. A one-line comment that v0.8.0 cannot reach it
would help the next coverage reader.

**Rest of the diff (priority 6).** M: `ha.callWith` now demands `pong` for
a ping and `result` with `success` for everything else
(`client.go:565-586`); `TypeResult` kept because it is used. Config flag
defaults derived from the constants print the same `0xFFF1`/`0x8001`.
Coverage floors and the stale check read correctly (`check` skips
zero-statement packages, fails missing floors, stale floors, floors
without statements).
