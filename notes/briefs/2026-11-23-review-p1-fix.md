# Brief: review phase 1, fix wave

Plan `notes/review/2026-11-23/README.md` ("Method per phase": red test
first, one standing guard per defect class); decisions 23–30 in
`decisions.md`; findings in `phase-1-S.md`, `phase-1-GA.md`,
`phase-1-GB.md`; **the verifier's verdicts, merges and severities in
`phase-1-verify.md` win**. Base: `review/p1` at the commit of this brief.
Pinned sources: the table in `notes/briefs/2026-11-23-review-p1.md`.

The reviewers' and the verifier's probes (red-test material, copy what
helps, never the files as they are) are in
`/tmp/claude-1000/-home-markus-Dokumente-GitHub/700e1304-db09-4033-9c98-588d8ec93391/scratchpad/{S,GB,V}/src/**/probe_*_test.go`.

Rules for both: no sub-agents; no push, pull request or tag; never
`git stash`; `PATH=$HOME/go/bin:$PATH`; commits small, one per finding or
class, message `fix(<area>): … (HR-P1-…)`; each defect gets its regression
test **red before the fix** (say in the report that you saw it red, or why
not: a gap, not a defect); `make ci` green at the end; the guards of
`CLAUDE.md` you touch (`make race`, `make cover-check`, `make fuzz-smoke`)
green; do not edit the other implementer's paths. A note: fix it when the
fix is small and testable, otherwise say in the report why it should be
accepted (one line, for the register). Wording (decision 24): never write
about **HR-P1-auth-01** beyond its ID — not in code, comments, docs, changelog or commit messages;
`held/` is not yours to read. A defect inside go-fabric, go-fabric-ui or
go-hamapping: report it, do not work around it (except decision 26's
stop-gap). Report in the fixed shape: commits, guards with HEAD, per
finding (fixed / not fixed + reason, test name, red seen), changelog lines,
differences from this brief, questions. Stop when the report is written.

## F1 — the security and daemon paths (Opus, review worktree)

Worktree `/home/markus/Dokumente/GitHub/go-ha2fabric-review`, branch
`review/p1`. Paths: `cmd/ha2fabric/**`, `internal/config/**`,
`internal/web/**`, `internal/ha/**` (not `hatest` beyond what a test
needs), `internal/camera/**`, `internal/selection/**`,
`internal/nodes/**`, `addon/**`, `script/run.sh`, `Dockerfile`,
`.dockerignore` (new), `docs/operations.md`, `README.md`, `CONCEPT.md` if
a statement there changes, `changelog.md` (`[Unreleased]`, 0.7.0; F1 owns
it and adds F2's lines from F2's report only if the coordinator asks).
Report: `notes/review/2026-11-23/phase-1-fix-F1.md`.

| Row | What |
| --- | --- |
| bound-02, conc-03, bound-17 | the camera bounds as S proposed in those rows (sessions per camera, the end of a session whose controller is gone, stills and candidates); the go-fabric facade questions inside them go into your report, not into code |
| ha-62 (+ ha-04) | `Subscribe` whose context ends before the result sends `unsubscribe_events` (or the equivalent close) |
| err-05 | a failed re-offer tells the controller / ends cleanly |
| err-06 (+ cfg-35) | decision 25: `selection.json.bak`, lenient read of another version, warning text; the registry-node behaviour stated |
| ha-07, ha-08 | decision 28 |
| data-09 | one wrong-typed field no longer fails the whole registry list (skip the entry, log once) |
| cfg-10 | an existing secret file with a wider mode: tighten or warn, as S proposed |
| cfg-12 | decision 27 (breaking, `changelog.md`, docs) |
| test-16 | a test that holds the admin table at the daemon's wiring |
| err-61 (note) | keep the node reference when `FactoryReset` fails |
| err-64 | decision 29 |
| test-70 | the negative cases of 61, 62 (60 is F2's) |
| notes | cfg-11, ha-13, ha-14, cfg-15, sup-19 (`.dockerignore`), data-20, cfg-21, conc-22, conc-63, cfg-66, data-67, conc-71, err-72: fix if small, else one line why to accept |
| not now | sup-18 (after the live Ingress test, its own change); HR-P0-cfg-27 (decision 22) |

## F2 — the plan side (Opus, own worktree)

Worktree `/home/markus/Dokumente/GitHub/go-ha2fabric-review-F2`, branch
`review/p1-F2` from `review/p1` at this brief (create it:
`git -C /home/markus/Dokumente/GitHub/go-ha2fabric-review worktree add ../go-ha2fabric-review-F2 -b review/p1-F2 review/p1`).
Paths: `internal/plan/**`, `internal/bridge/**`, `internal/adapt/**`,
`internal/expose/**`, `internal/exposure/**`. Changelog lines and any
docs sentence you need go into your report, not into files outside your
paths. Report: `notes/review/2026-11-23/phase-1-fix-F2.md` (committed on
your branch).

| Row | What |
| --- | --- |
| adpt-30 | decision 26, the stop-gap: the bounds last sent per entity; red test case (b) (raise past the old high, red by construction, not timing), then (a); describe the go-hamapping issue's shape in the report |
| conc-32 (+ conc-60) | one change: a worker-owned "re-sync owed" flag set by the overflow and the timer, the unreachable marking in `sync`'s not-connected path, the removed entity recorded outside the queue; G-B's probe `TestProbeGBResyncLostOnFullQueue` as the red test; fix the "a re-sync follows" log line |
| err-31 | a failed batch operation is not recorded as served and is retried; `SyncError` set |
| conc-33 (note) | re-check exposure when a queued command runs, if small |
| cfg-34 | keep the label id each configured name last resolved to; the warning says the exclusion has lapsed |
| test-38 | the negative cases listed in the row |
| data-37, bound-39 | notes: fix if small, else one line why to accept |
| not now | data-36 (go-fabric upstream note), adpt-68 and adpt-69 (go-hamapping issues, decision 30) |
