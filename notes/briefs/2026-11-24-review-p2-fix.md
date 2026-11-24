# Brief: review phase 2, fix wave

Plan `notes/review/2026-11-23/README.md` ("Method per phase": red test
first, one standing guard per defect class); decisions 35–46 in
`decisions.md`; findings in `phase-2-X1.md`, `phase-2-X2.md`,
`phase-2-X3.md`; **the verifier's verdicts, severities and corrections in
`phase-2-verify.md` win** (e.g. conc-60's corrections on `ConnectPeer`
and the expected cost). Base: `review/p2` at the commit of this brief.
Pinned sources: the table in `notes/briefs/2026-11-24-review-p2.md`.

The reviewers' and the verifier's probes are red-test material (copy what
helps, never the files as they are): each report's appendix, and
`/tmp/claude-1000/-home-markus-Dokumente-GitHub/8fe7f689-a76f-424d-88a1-c6d98442f53c/scratchpad/{X2,X3,V}/src/**/*probe*_test.go`.

Rules for every implementer: no sub-agents; no push, pull request, issue
or tag; never `git stash`; `PATH=$HOME/go/bin:$PATH`; commits small, one
per finding or class, message `fix(<area>): … (HR-P2-…)`; each defect gets
its regression test **red before the fix** (say in the report that you saw
it red, or why not); `make ci` green at the end; the guards of `CLAUDE.md`
you touch (`make race`, `make cover-check`, `make fuzz-smoke`) green; do
not edit the other implementer's paths. A note: fix it when small and
testable, otherwise one line why to accept (for the register). Wording:
never write about **HR-P1-auth-01** beyond its ID (decision 37) — not in
code, comments, docs, changelog or commit messages; `held/` is not yours
to read; describe the admin check as it is (decision 21). A defect inside
go-fabric, go-fabric-ui or go-hamapping: report it, do not work around it.
Report in the fixed shape: commits, guards with HEAD, per finding (fixed /
not fixed + reason, test name, red seen), changelog and docs lines,
differences from this brief, questions. Stop when the report is written;
do not wait for or poll background work.

## F1 — sync, bridge, fleet (Opus, review worktree)

Worktree `/home/markus/Dokumente/GitHub/go-ha2fabric-review`, branch
`review/p2`. Paths: `internal/bridge/**`, `internal/plan/**`,
`internal/exposure/**`, `internal/ha/snapshot.go`, `internal/ha/fanout.go`,
`internal/ha/subscription.go` (+ their tests, `hatest` as a test needs),
`cmd/ha2fabric/nodes.go`, `cmd/ha2fabric/warnings.go` (+ tests),
`docs/operations.md`, `changelog.md` (`[Unreleased]`, 0.7.0 — F1 owns both
and adds F2's lines from F2's report when the coordinator asks).
Report: `notes/review/2026-11-23/phase-2-fix-F1.md`.

| Row | What |
| --- | --- |
| data-30, err-61, err-65, bound-37 (the page half), bound-62 | decision 38, one design: the debounce gets a latest time (a stream of events cannot put the sync off without end); a failed sync while connected is retried with back-off; `Resync` says whether a sync was planned; a web UI warning (`sync_failed`, beside `label_unresolved`) for a sync that keeps failing, whatever the cause; one shared snapshot reader per connection (single-flight; the plan stays per node) |
| conc-60 (low) | decision 39, the bridge and fleet half: `stopRun` bounded by the stop's context (`cmd/ha2fabric/nodes.go`); the camera closes moved off the sync. The parallel, deadline-bound `camera.Host.Close` is F2's; agree its signature through the existing `Close(ctx)` (the ctx is honoured by F2) |
| bound-38 | decision 42, the bridge half: the states held once, not in two places (F2 caps each state at decode) |
| err-63 | after a dropped (queue full) or refused (`adapt.Service`) command, the device's attribute is set back to Home Assistant's state |
| bound-64 | `bridge.queue_full` / `bridge.command_dropped`: a rate-limited summary (count per interval), not one Error line per item |
| test-42 | a fuzz target `Map` → `Build` → `Apply` over hostile entity data in `make fuzz-smoke` (`Makefile` FUZZ_TARGETS); the two known go-hamapping refusals (HR-P2-adpt-33, -34) skipped by name until the pin moves (round 2 drops the skip) |
| doc-66 | decision 45: one docs sentence (`docs/operations.md:484-489`); the follow-up issue is the coordinator's |
| notes | err-67 (a cancelled sync at Error), conc-68 (exposure apply racing Delete), conc-69 (doc comment only): fix if small, else one line why to accept |
| not now | data-35 and the go-hamapping pin lift (round 2, after go-hamapping's release, decision 35) |

## F2 — camera, adapt, client, daemon edges (Opus, own worktree)

Worktree `/home/markus/Dokumente/GitHub/go-ha2fabric-review-F2`, branch
`review/p2-F2` from `review/p2` at this brief's commit (the coordinator
creates it). Paths: `internal/camera/**`, `internal/adapt/**`,
`internal/ha/client.go`, `message.go`, `camera.go` (+ tests),
`internal/web/**`, `internal/selection/**`, `internal/nodes/**`,
`internal/config/**`, `cmd/ha2fabric/main.go`,
`cmd/ha2fabric/supervisor.go` (+ tests). No `changelog.md`, no
`docs/operations.md`: put their lines in your report.
Report: `notes/review/2026-11-23/phase-2-fix-F2.md`.

| Row | What |
| --- | --- |
| conc-60 (low) | decision 39, the camera half: `camera.Host.Close(ctx)` ends the sessions in parallel and returns at the ctx's end |
| bound-40 | the Home Assistant side of the camera relay capped like the controller side (candidates per call and per session); ignored events no longer keep a session from its idle end |
| data-31 (closes HR-P1-data-36) | decision 41: control and bidirectional control characters dropped in `internal/adapt` before they reach go-hamapping's plan (the joiners U+200C/U+200D kept); refused in web UI names and node labels (`internal/nodes`, `internal/selection` where a name is written); `--dump-plan` quotes names (`main.go`) |
| data-32 | a select's option is not trimmed (`internal/adapt/command.go`) |
| bound-38 | decision 42, the decode half: a state over 256 KiB keeps its state and drops its attributes, with a log line; the verdict reason reaches the bridge (agree the field with the existing verdict types; if it needs a bridge change, report it, F1 or round 2 adds it) |
| bound-37 (the client half) | a frame over the read limit is named as such in the error the sync reports, not "connection dropped" |
| bound-39 | Home Assistant's error text cut (a few KiB) before it reaches the operation ring and the status |
| cfg-03 | decision 44: the Supervisor gate keeps re-reading after it opened; a downgrade closes it |
| net-01 | none in code (F1's docs) |
| notes | bound-41 (connection cap): one line why to accept or a small fix; data-70 none |

## Round 2 — after go-hamapping's release (coordinator-briefed)

The pin lift to go-hamapping's release with HR-P2-adpt-33/-34 (decision
35), a fixture cover with tilt in the end-to-end test, the fuzz skip
dropped, and **data-35** (decision 40: on a build failure the group is
mapped and built again without the refused entity, using go-hamapping's
typed build error if its release carries one).

## go-hamapping (its own pull request, decision 35)

In `/home/markus/Dokumente/GitHub/go-hamapping`, by its `CLAUDE.md`: a
Fable fact note first (the cover row's Type and EndProductType by
features against go-fabric v0.25.0's WindowCovering conformance), then an
Opus implementer: the cover row, `maxModes = 255`, a check that what `Map`
calls mappable builds (row tests over every device class × feature set),
a typed build error carrying the entity key (for data-35), the repeated
mode label (HR-P2-adpt-36: numbered, or unmappable — the fact note
recommends); `changelog.md`; release by the coordinator.

## Round 2 — the brief (after go-hamapping #42 / v0.7.0)

One Opus implementer, review worktree, branch `review/p2` (F1 and F2 are
integrated; the F2 worktree is gone or idle). Common rules as above.
Report `notes/review/2026-11-23/phase-2-fix-round2.md`.

| Row | What |
| --- | --- |
| pin lift | go-hamapping v0.7.0 (`go get`, `go mod tidy`); `changelog.md` `### Changed` line; no deprecated go-fabric identifier ([no-deprecated rule]) |
| adpt-33 / decision 47 | `internal/adapt/command.go` `services`: `cover`: `hamapping.MethodOpenTilt` → `open_cover_tilt`, `hamapping.MethodCloseTilt` → `close_cover_tilt` (no data keys; cite `core:homeassistant/components/cover/services.py:72-82` @2026.10.3); a fixture cover with tilt (features 255, `device_class: blind`) and a tilt-only one (240) in `testdata/ha/` and the end-to-end test; the X2 probes `TestX2ProbeCoverBuild`, `TestX2Probe256Options` green as tests |
| data-35 | decision 40: on `errors.As(err, &*hamapping.BuildError)` the group is mapped and built again without `be.Key` (its `Riders` lose the endpoint too), bounded by the group's entity count; the refused entity's verdict "unmappable: go-fabric refused …"; the probe `TestX2ProbeSiblingKnockedOut` as the red test |
| test-42 | `FuzzPlanBuild`: drop the skips for adpt-33/-34/-43; keep only the shapes of go-hamapping #45, by name |
| FuzzClean | `./internal/text/:FuzzClean` into `FUZZ_TARGETS` |
| bound-38 bridge | an `Oversized` state gets a verdict reason in `plan.Assess`; the `state_changed` path logs `ha.state_oversized` once per entity |
| conc-68 | an exposure apply racing a node's Delete does not write the deleted node's selection file again (`internal/selection` / its caller) |
| conc-69 | the doc comment in `internal/camera` (the session cap holds under go-fabric's per-camera command lock) |
| changelog / docs | F2's lines from `phase-2-fix-F2.md` ("Changelog lines", "Docs lines") into `changelog.md` and `docs/operations.md`; decision 48–51 where a page states them |

Then the whole guard set of `CLAUDE.md` (the coordinator runs it).
