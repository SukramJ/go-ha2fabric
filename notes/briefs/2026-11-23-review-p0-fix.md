# Brief: review phase 0, fix wave

Plan `notes/review/2026-11-23/README.md` ("Method per phase": red test
first, one standing guard per defect class); decisions 13–20 in
`decisions.md`; findings in `threat-model.md`, `phase-0-evidence.md`,
`phase-0-verify.md` (the verifier's verdicts and severities win), held
details in `held/threat-model.md` and `held/phase-0-verify.md` (untracked;
never copy their content into a tracked file before the coordinator says
so). Base: `review/p0` at `609b5eb`. Pinned sources: the table in
`notes/briefs/2026-11-23-review-p0.md`.

Rules for both: no sub-agents; no push, pull request or tag; never
`git stash`; commits small, one per finding or class, message
`fix(<area>): … (HR-P0-…)`; each defect gets its regression test red
before the fix (state in the report that you saw it red); `make ci` green
at the end; the guards of `CLAUDE.md` you touch (`make race`,
`make cover-check`, `make fuzz-smoke`) green; do not edit the other
implementer's paths. `changelog.md` under `[Unreleased]` (0.7.0) is F1's;
F2 puts its changelog lines in its report. Report in the fixed shape:
commits, guards with HEAD, per finding (fixed / not fixed, test name,
red seen), differences from this brief, questions.

## F1 — the admin, configuration and token paths (Opus, review worktree)

Worktree `/home/markus/Dokumente/GitHub/go-ha2fabric-review`, branch
`review/p0`. Paths: `cmd/ha2fabric/**`, `internal/config/**`,
`internal/web/**`, `internal/ha/client.go` (+ its tests),
`internal/nodes/**`, `addon/**`, `script/run.sh`, `Dockerfile`,
`docs/operations.md`, `README.md`, `changelog.md`, `CONCEPT.md` if a
statement there changes.

| Finding | Decision | Work |
| --- | --- | --- |
| HR-P0-auth-20 (held) | 13 | The verifier's fix shape in `held/phase-0-verify.md`. Commit message and code comments describe the behaviour neutrally (the admin check in the add-on); no exploit wording in tracked text until the coordinator publishes |
| HR-P0-cfg-21 | 14 | Random passcode on first start when `PASSCODE` is unset, kept beside the store as `internal/nodes` keeps the registry nodes' (`nodes.go:376-391`); add-on option optional without default (`addon/config.yaml`, `run.sh`); explicit `20202021` (and every value go-fabric's validator would accept but that is a spec test value, if go-fabric exposes such a list — check, do not invent) → status warning + log line |
| HR-P0-cfg-22 + HR-P0-err-26 | 15 | `config.Load` refuses `WEB_ADMIN` with non-loopback `WEB_LISTEN` and empty `WEB_ALLOW` (exit 2, both remedies named); `web.New` refuses an unparsable `Allow` entry instead of dropping it; docs: plain HTTP, an address filter is not authentication |
| HR-P0-leak-24 | 16 | `logPairing` (`cmd/ha2fabric/nodes.go:306-315`) only while the node has no fabric; otherwise one line pointing to the administrator's pairing page |
| HR-P0-ha-23 + HR-P0-ha-31 | 17 | Warning (log + status warning) for `ws` to a host that is neither loopback nor `supervisor`; the WebSocket dial follows no redirect; docs paragraph + `wss://` note. `HA_URL` validated by `url.Parse` with a host (covers the config half of HR-P0-err-03) |
| HR-P0-leak-25 (note) | — | `HA_URL` with userinfo refused at load; logged without query |
| HR-P0-cfg-27 (low) | 19 | Bind the add-on listener tighter only if the Supervisor's gateway address is known without a new dependency; otherwise report why not |
| HR-P0-leak-29 (note) | — | `-h` must not print an environment `PASSCODE` as the flag default |
| HR-P0-cfg-30 (note) | — | Your judgement after reading: fix if small, else report |
| HR-P0-data-02 (note) | — | `nodes.Slug`'s documented bound made true (fix the comment or the code, whichever is right); copy the seeds from `held/phase-0-fuzz-seeds/internal/nodes/testdata/fuzz/` under the package's `testdata/fuzz/` |
| HR-P0-test-06, F1 part | — | Tests for the error branches on hostile input in `internal/ha/client.go` (`:262-286`, `:313`, `:335`, `:477`) and the daemon's exits (`main.go:121-134`, `nodes.go:461-479`) |
| HR-P0-doc-05 (note), F1 part | — | `README.md:174`: fuzz-smoke runs every target of the Makefile's `FUZZ_TARGETS` |
| HR-P0-cite-32 (note) | — | Not yours (coordinator) |

Fable reads F1's diff before the pull request (CLAUDE.md model table).

## F2 — the data paths (Opus, own worktree)

Worktree `/home/markus/Dokumente/GitHub/go-ha2fabric-review-F2`, branch
`review/p0-F2` from `609b5eb` (the coordinator creates it). Paths:
`internal/adapt/**`, `internal/camera/**`, `internal/ha/camera.go` (+ its
tests), the fuzz seeds of `internal/ha`, `.github/workflows/ci.yml` header
comment, nothing else.

| Finding | Work |
| --- | --- |
| HR-P0-data-01 (low) | `clampInt` clamps in float before converting (`adapt/command.go:389-392`), `color_temp_kelvin` (`:330`) clamped to a sane range — read go-hamapping v0.6.0 / Home Assistant core 2026.10.3 for the range, cite it; a test per architecture-dependent case (386 via `GOARCH=386 go test ./internal/adapt`) |
| HR-P0-err-03 (low) | `ha.HTTPBase` (`ha/camera.go:95-117`) builds the URL with `net/url` (IPv6 literal bracketed) and returns only a URL that parses; seeds from `/home/markus/Dokumente/GitHub/go-ha2fabric-review/held/phase-0-fuzz-seeds/internal/ha/testdata/fuzz/` under `internal/ha/testdata/fuzz/` |
| HR-P0-bound-28 (low) | A cap on the queued ICE candidates per session in `internal/camera` (pick a bound, state why, a test at the bound); the session-count half is phase 1's |
| HR-P0-doc-05 (note) | `ci.yml:6` header: fuzz-smoke runs every target of `FUZZ_TARGETS`. (`README.md:174` is F1's path: F1 does that line) |
| HR-P0-test-06, F2 part | A test for `internal/camera/camera.go:461`'s error branch |
