# Brief: review phase 3, fix wave

Plan `notes/review/2026-11-23/README.md` ("Method per phase"); decisions
`decisions.md` 53–63; reports `phase-3-A.md`, `phase-3-B.md`,
`phase-3-C.md`, verification `phase-3-verify.md` (its corrections win over
the reports). Branch `review/p3` (base `0a47095` + notes). Two
implementers, **disjoint paths**, Opus, **no sub-agents**, no push, no PR,
no issue, no settings change. Each commits on its own branch, small
commits, red test first where the row names one, runs the guards of its
paths, and writes its report `notes/review/2026-11-23/phase-3-fix-<F>.md`
(fixed shape: per row the commits, guards with HEAD, differences from the
row, left out, questions, changelog lines and docs lines for the other
implementer). `PATH=$HOME/go/bin:$PATH`. Pinned sources as in
`notes/briefs/2026-11-26-review-p3.md`.

HR-P3-doc-30 is done in PR #81 (decision 56); do not touch those lines.

## F1 — code, tests, guards (worktree `../go-ha2fabric-review`, branch `review/p3`)

Owns: `cmd/`, `internal/`, `testdata/`, `script/`, `Makefile`,
`addon/config.yaml`, `script/run.sh`, `.golangci*`, `go.mod`/`go.sum`
(only if a guard needs nothing new: no new dependency).

1. **Decision 54, HR-P0-cfg-27**: in the add-on the web listener binds to
   the Supervisor network's gateway address (the address the Ingress
   proxy connects to; measured 2026-11-25: the add-on's `ip_address`
   172.30.32.1 on interface `hassio`, the proxy 172.30.32.2), found at
   start — not a constant. Read how `script/run.sh:77-78` and
   `internal/config` set `WEB_LISTEN`/`WEB_ALLOW` today and choose the
   cleanest shape (e.g. a `WEB_LISTEN` host value meaning "the Supervisor
   network's address", resolved in `internal/config` or `cmd` from the
   interface or the Supervisor's `/network/info`; say which and why in the
   report). Not found: today's bind plus one warning line. The client
   filter stays. Tests for found / not found; docs lines for F2.
2. **Decision 60**: HR-P3-test-01 (port go-fabric's stale-floor check —
   read it at `/home/markus/go/pkg/mod/thingfabric.dev/go-fabric@v0.26.0`,
   its `script/` or Makefile; a floor > 3.0 points under the measurement
   fails; every package needs a floor; floors just under the lower of two
   runs, verify's note: runs differ 0.3–0.5); one AST guard in
   `internal/arch` for HR-P3-api-04 (`internal/plan`: no `sync`, `os`,
   `time`, `net`, no `go` statement — confirm against what plan uses
   today), api-05 (`internal/adapt`'s go-fabric imports frozen to today's
   set), api-06 (every package has a layer entry, the root package too),
   api-07 (`os.Getenv`/`LookupEnv`/`Environ` only in `internal/config`
   and `cmd`'s documented places — measure first), test-13 (fuzz-smoke
   list equals the `Fuzz*` functions). Each guard: show it bites (mutate,
   red, revert) in the report.
3. **Decision 61**: the citation check — unit C's script
   (`/tmp/claude-1000/-home-markus-Dokumente-GitHub/887352f1-63e8-4aab-8e2b-bd2771c4c6a3/scratchpad/p3/C/cites.py`,
   appendix A of `phase-3-C.md`) as a tool in the repo (Go under
   `script/` preferred, no new dependency; Python acceptable only if CI
   already has it) and a `make` target in `make ci`: every go-fabric,
   go-fabric-ui, go-hamapping citation names `go.mod`'s version and its
   file/lines exist there (module cache). Then re-pin every such citation
   **in Go files and `testdata/`** (C's tables; cite-60..62, -65, -67's
   `config.go:70`). Docs citations are F2's — give F2 the tool's command.
4. HR-P3-test-02 (verify's corrected comment: 320 ms), test-10 (the fake
   refuses what core 2026.10.3 refuses, with the citation), test-11,
   test-12, test-14, test-15, ha-09 (a reply without `type: result` or
   without `success` is an error, not a success — check core's shape
   first), api-08 (drop unused `ha.TypeResult`; the rest accepted unless
   trivial), cfg-68 (`label: str(1,32)` in `addon/config.yaml`'s schema).
5. Guards before the report: `make ci race cover-check fuzz-smoke`,
   `make vuln licenses secrets`, `make version-check`.

## F2 — supply chain and docs (own worktree `../go-ha2fabric-review-F2`, branch `review/p3-F2` from `review/p3`)

Owns: `.github/`, `Dockerfile`, `addon/Dockerfile`, `.dockerignore`,
`licenses/` (new), new files under `script/licenses/` only, `THIRD-PARTY-NOTICES.md`,
`README.md`, `CONCEPT.md`, `CLAUDE.md`, `docs/`, `addon/DOCS.md`,
`addon/CHANGELOG.md` (not the doc-30 lines), `changelog.md`,
`repository.yaml`, `SECURITY.md` (new). Makefile lines you need: write
them into your report; the coordinator adds them.

1. **Decision 57** HR-P3-sup-31: Dependabot auto-merge for patch updates
   only, never the five family modules (go-fabric, go-fabric-ui,
   go-hamapping, go-hamqtt, go-ha-catalog); sup-47 (gate on the PR
   author, not `github.actor`). The required-check change is a setting:
   list it for the coordinator.
2. **Decision 58** HR-P3-sup-32: the release and both image workflows
   publish only for a tag whose commit is on `main` (check in the
   workflow), `workflow_dispatch` only from `main`; sup-46 (input via env,
   not expanded in `run:`). Settings (tag ruleset, `ghcr` environment,
   checks bound to the Actions app, secret scanning, push protection):
   write the exact `gh api` calls into your report, do not run them;
   go-fabric-ui's are in its repo's `notes/review/2026-11-17/decisions.md`
   decision 56 and the session note `d56-calls.sh` may be gone — derive
   from the API docs. A workflow environment reference (`environment:
   ghcr`) only together with the call that creates it.
3. **Decision 62** HR-P3-sup-34/35/36: base images by digest (both
   Dockerfiles; Dependabot `docker` ecosystem for both), every action by
   full SHA with the version comment, provenance and an SBOM attested for
   the release archive and the images (`actions/attest-build-provenance`,
   `actions/attest-sbom` or the buildx `provenance`/`sbom` options —
   choose and say why; mind the permissions each needs).
4. **Decision 59** HR-P3-sup-37 with verify's extension: a generated
   `licenses/` tree (linked Go modules via `go-licenses save` or
   equivalent in the pinned tools, Go's own licence, go-fabric's NOTICE
   with matter.js's NOTICE it points to, go-fabric-ui's npm licences as
   shipped in its embedded `dist`), copied into the release archive and
   both images; a check that it is current (generator under
   `script/licenses/`, Makefile line for the coordinator, CI step).
   doc-38: `THIRD-PARTY-NOTICES.md` against `go.mod` (verify struck the
   humanize/bigfft part).
5. Docs: doc-39, doc-40, doc-41, doc-42, doc-43, sup-49 (a line in the
   docs on the old `ghcr.io/sukramj` packages staying at 0.6.3), sup-45
   (CodeQL: add `actions`; blocking is a setting → list it), doc-50
   (`repository.yaml` maintainer: `SukramJ <me@sukramj.dev>`), sup-44
   (`SECURITY.md`: report privately via GitHub's private vulnerability
   reporting), sup-48 (record what is still undecided in issue #72's
   terms in `docs/operations.md` or leave a line for the coordinator to
   comment on #72 — say which). Citations in docs (C's tables cite-60..67,
   cite-63 by_design matter-hub composition per C's and verify's lines,
   cite-64 CONCEPT ranges per verify `:18-19, :236-237, :304-312` — check
   them against A's note `:234-237, :293-309` and take what the code
   shows), CLAUDE.md state line pins (go-hamapping v0.8.0), roadmap
   survey pins (cite-66: finished items at the survey's v0.2.1, open
   items at go.mod's version). Use F1's citation tool when it lands
   (ask the coordinator); until then C's script from the path above.
6. **Decision 53**: one paragraph in `docs/operations.md` (and a line in
   `addon/DOCS.md`) on co-installed add-ons: what another add-on on the
   same Home Assistant can do against the add-on's web listener and what
   the operator can do about it. Read `held/phase-1-S.md`,
   `held/phase-1-verify.md`, `held/phase-2-X1.md`, `held/phase-2-verify.md`
   (they become public in this pull request) and the coordinator's
   measured facts: on a real HAOS other add-ons run with `NET_RAW` on the
   `hassio` bridge sharing L2 with the Supervisor; an interception of the
   Supervisor's address was observed, the full forged exchange was not
   completed on a live box; the Supervisor forwards no secret the add-on
   could verify; decision 37 amended: docs only, no Supervisor issue.
   Mention decision 54's bind as the mitigation it is (LAN peers) and is
   not (a co-installed add-on on the `hassio` network).
7. `changelog.md` `[Unreleased]` lines for both implementers (F1 gives
   you its lines; the coordinator merges them if F1 finishes later).
8. Guards for your paths: `make ci` (docs tests), `make version-check`,
   `make release`, `actionlint` if available, and `make docker
   addon-image` once at the end (slow machine: once).
