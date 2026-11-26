# Brief: review phase 4 run 1, fix wave

Plan `notes/review/2026-11-23/README.md` ("Method per phase"); decisions
`decisions.md` 68–75; reports `phase-4-S-web.md`, `phase-4-S-ha.md`,
`phase-4-P.md`, `phase-4-D.md`; verification `phase-4-verify.md` (its
corrections win over the reports, e.g. S-web's line numbers). Base:
branch `review/p4` at `da10e2a` (code `c3a9770` + notes). Two
implementers, **disjoint paths**, Opus, **no sub-agents**, no push, no PR,
no issue, no settings change. Small commits, red test first where the row
names one (the reports' probe appendices are the red tests), the guards
of your paths, and a report `notes/review/2026-11-23/phase-4-fix-<F>.md`
(fixed shape: per row the commits, guards with HEAD, differences from the
row, left out, questions; F1 adds its `changelog.md` lines and docs lines
for F2). `PATH=$HOME/go/bin:$PATH`. Pinned sources as in
`notes/briefs/2026-11-26-review-p4.md`; go-fabric v0.27.0 is in the module
cache. The machine is slow: package tests while working, the full guards
of your paths once at the end.

Rules from `CLAUDE.md` hold: no deprecated go-fabric identifier (SA1019);
a user-visible change gets a `changelog.md` line; docs follow behaviour.

## F1 — Go code, tests, the lift (worktree `../go-ha2fabric-review`, branch `review/p4`)

Paths: `go.mod`, `go.sum`, `licenses/` (regenerate by
`script/licenses/generate.sh`), `internal/**`, `cmd/**`, `script/cite/**`,
`script/coverfloor/**`, `.golangci.yaml`; citations inside these files.

1. **Decision 69, the lift**: go-fabric v0.27.0 (go-fabric-ui v0.14.1 and
   go-hamapping v0.8.0 stay unless `go mod tidy` demands otherwise — say
   so); `licenses/` regenerated; every citation in your paths re-pinned to
   v0.27.0 **and re-read** — the reports' "v0.27.0" sections list moved
   ranges (S-ha: about eight in `internal/camera/camera.go`; S-web:
   `config/node.go:28`, `config.go:143`, `:493`, `node.go:44`; D:
   `arch_test.go` camera.md `133-140` → `137-144`); `make cite` checks
   only existence, so read each. P's note on a failed-reservation Forget
   answering nil (go-fabric #616's scope): a comment at the call.
2. **Decision 72, HR-P4-bound-20**: eviction only within the offering
   fabric, a per-fabric share of half the cap, RESOURCE_EXHAUSTED
   otherwise (as the kit's existing refusal); S-ha's probe red first;
   tests for: other fabric's live view untouched, own oldest idle evicted,
   share reached → refused. HR-P4-cite-23 (the comment) with it.
3. **Decision 73, HR-P4-err-40**: the refused keys keep their old plan
   live (states, reachability, commands to `call_service`, the read
   model's endpoint) until a later batch applies them; P's probe red
   first.
4. **Decision 74, HR-P4-err-64**: Delete whose stop fails → 500, record,
   files and fleet entry kept, a second Delete retries. With it
   HR-P4-conc-65 (rollback stop with a deadline, outside the lock or
   bounded; the `fleet.Close` text) and HR-P4-err-66.
5. **HR-P4-test-60** (decision 70), D's shape: `nodeStart = 90 s` shared,
   `do()` returning the error with method, path, elapsed; `want` /
   `eventually` name the error; a client per daemon without keep-alive;
   `endpointIDs` with error; every helper D lists.
6. **HR-P4-test-61** (hatest copies under its lock), **HR-P4-test-68**
   (fake as core where cheap; else a comment naming the difference),
   **HR-P4-conc-69** (`After`).
7. **HR-P4-test-62**: `internal/ha` floor 94, the floors header's spread
   corrected. **HR-P4-test-63**: `script/cite` fails on a cited file
   missing at `go.mod`'s version (red by D's mutation).
   **HR-P4-test-67**: a check that the version banner finds its modules
   (a built-binary test or `debug.ReadBuildInfo` path test — say which).
8. **HR-P4-err-41** (the full verdict with its reason in both branches),
   **HR-P4-bound-43** (`strings.Builder` or a hash — keep the signature's
   meaning; P's timing probe as a benchmark or bound test), P's
   `ErrNoStableKey` probe as a standing test in `internal/plan`.
9. **HR-P4-bound-21** (`sdpMid` bounded with the candidate),
   **HR-P4-data-22** (U+2028/U+2029 unsafe), **HR-P4-cfg-05**
   (`WEB_ALLOWED_HOSTS` refuses what go-fabric-ui refuses, at
   `config.Load`).
10. **HR-P4-sup-70**: `gosec` on `cmd/` with one reasoned `nolint`.
11. Guards at the end: `make ci`, `make race`, `make cover-check`,
    `make fuzz-smoke`, `make vuln licenses secrets`, `make cite`,
    `GOARCH=386 go test ./...`. Report the changelog lines and any docs
    lines (operations.md, by_design.md for decision 72) for F2.

## F2 — images, workflows, docs (own worktree `../go-ha2fabric-review-F2`, branch `review/p4-F2` from `da10e2a`)

Paths: `addon/**`, `Dockerfile`, `script/run.sh`, `.github/**`,
`Makefile` (lint lines only), `docs/**`, `README.md`, `CONCEPT.md`,
`SECURITY.md`, `THIRD-PARTY-NOTICES.md`, `changelog.md`; citations in
these files.

1. **HR-P4-cfg-01** (decision 75): `ENV S6_CMD_RECEIVE_SIGNALS=1` in
   `addon/Dockerfile`; a guard test that the add-on image sets it (where
   the image tests live — `cmd/ha2fabric/image_test.go` is F1's path:
   write the test text into your report for the coordinator, or put the
   check in the lint job — say which); `script/run.sh:7` comment,
   `addon/config.yaml:44-47` comment, `docs/operations.md` stop sentence.
   **HR-P4-cfg-02**: the docs line (`--stop-timeout 30`, Compose
   `stop_grace_period`).
2. **HR-P4-sup-03**: drop `# syntax=` from `Dockerfile`; the lint grep
   covers `# syntax=` lines. **HR-P4-sup-04**: a lint-job check that the
   Go version agrees in `go.mod`, both Dockerfiles and every workflow's
   `GO_VERSION`.
3. **HR-P4-doc-24**: one sentence on `SSL_CERT_FILE` / `SSL_CERT_DIR` for
   `wss://` with a private CA (operations.md, add-on docs only if they
   offer stand-alone).
4. Docs for decision 72 (camera sessions per fabric; `docs/by_design.md`
   if matter-hub differs — read F1's line when it lands, ask the
   coordinator), decision 74 (Delete refused), and go-fabric v0.27.0
   re-pins in your files (`make cite` after F1's lift; until then
   re-read the ranges against the v0.27.0 module cache).
5. `changelog.md` `[Unreleased]`: your lines and F1's (the coordinator
   merges F1's if it finishes later); a `### Changed` line for the
   go-fabric v0.27.0 bump.
6. Guards for your paths: `make ci`, `make version-check`, `make release`,
   `actionlint` if available, and `make docker addon-image` once at the
   end, including a stop-time check of the add-on image (`docker stop -t
   30` on a TERM handler, as S-web's appendix) if the machine allows.

## After (coordinator)

Cherry-pick F2 onto `review/p4`, full guard set, pre-merge read (Opus,
IDs 90–99), go-hamapping issue (decision 72) and go-fabric issue
(adpt-42), register `phase-4-register.md`, CLAUDE.md state line, push,
PR without tag (decision 71).
