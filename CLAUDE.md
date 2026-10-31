# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Home Assistant entities as a bridge for Matter controllers, in pure Go on go-fabric. Twin of `go-hamapping`; both are built on
[go-fabric](https://github.com/SukramJ/go-fabric) and share its project
setup. **State: 0.4.0** (brief H3, two follow-ups, the family bump F1 and its follow-ups F2, brief R1–R5): the daemon `cmd/ha2fabric` and its
add-on. `CONCEPT.md` is the design, `docs/operations.md` the operator's
page, `docs/by_design.md` the differences from matter-hub,
`docs/reports/H2.md`, `H3.md`, `F1.md` and `F2.md` the waves' reports. Layout: `internal/ha`
(the WebSocket client, `hatest` its fake on the fixtures in `testdata/ha/`),
`internal/expose` (the exposure rule with labels and overrides),
`internal/selection` (the web UI's overrides file), `internal/adapt` (HA
onto go-hamqtt's model, a command back to `call_service`),
`internal/bridge` (the composition on go-fabric's node and go-hamapping's
plans, and the web UI's exposure provider), `internal/config`,
`internal/web` (go-fabric-ui's handler behind the client filter),
`internal/arch` (the import guard).

## Commands

```sh
make ci        # build vet test lint — what the pipeline runs
make race      # go test -race (CGO_ENABLED=1 for this run only)
make fmt       # gofumpt -w .
```

Go 1.27.3, `CGO_ENABLED=0`. Dependencies: go-fabric, go-fabric-ui,
go-hamapping, go-hamqtt (`model`), go-ha-catalog, coder/websocket — `THIRD-PARTY-NOTICES.md`.
Every further dependency is a decision: Apache-2.0 / MIT / BSD / ISC only,
and it updates the notices.

More guards: `make cover-check` (floors in `script/coverfloor/floors.txt`,
a ratchet), `make fuzz-smoke`, `make secrets`, `make docker addon-image`,
`make release` (the dry run; `make version-check` holds
`internal/version`, `addon/config.yaml` and `changelog.md` together).

Home Assistant facts come from `home-assistant/core` at a pinned tag
(2026.10.3), read from its raw files and cited `file:line`; never run.
How a real Home Assistant bridge behaves: the helper reference is
[home-assistant-matter-hub](https://github.com/RiDDiX/home-assistant-matter-hub)
at a pinned commit (`664575a`), cited `file:line` in `docs/by_design.md` —
never a gold standard for Matter behaviour, which stays go-fabric's.

## Rules

- **Naming.** "Matter" is a trademark of the Connectivity Standards
  Alliance and is not part of this project's name, its packages, its
  binaries or its images; it appears only descriptively. See the README.
- **No mapping of our own.** Which entity becomes which device type and
  cluster is go-hamapping's `Map`; the daemon translates Home Assistant's
  registry and states into go-hamqtt's model (`internal/adapt`) and
  applies the plan. A gap there is a question to go-hamapping, not a
  workaround here.
- **Credentials** (the long-lived token, `SUPERVISOR_TOKEN`) come from the
  environment only, and are never logged or persisted.
- **go-fabric is the gold standard** for everything on the Matter side:
  device types, clusters, the facade (`node`, `device`, `contract`, the
  `<cluster>def` packages). This repository never reimplements a
  cluster or reaches below go-fabric's facade.
- **Official documents in English**: issues, pull requests, docs, notes.
- **One pull request per wave**, squash-merged; CI is the gate.
- **No attribution lines** in commits or pull request bodies.
- A user-visible change gets a `changelog.md` entry under `[Unreleased]`.

## Which model does which work

As go-fabric's CLAUDE.md, adapted: a statement about Home Assistant or
matter-hub that lands in the repo is cited later as fact, so the work that
produces such statements gets the strongest model, and the work a guard
checks may use a cheaper one.

| Work | Model |
| --- | --- |
| Analysis, planning, briefs, reviews, the wave report; anything written into `docs/by_design.md`, `CONCEPT.md` or a comment that cites `home-assistant/core` or matter-hub `file:line`; the translation in `internal/adapt` and the exposure rule in `internal/expose` (both are facts about Home Assistant); credential handling (`internal/config`, the token's path to the client); rebases and merge conflicts where upstream may have overtaken the change; a question to go-hamapping or go-fabric | **Fable** (the default for this repo) |
| Implementation of a finished brief with guards behind it: a bridge or web change whose tests and fixtures already hold the behaviour, a dependency bump with its notices, a `changelog.md` entry for a finished diff | **Opus** (also what fast mode runs on) |
| Work a test or formatter checks immediately: `gofumpt`, regenerating the coverage floors, adding a test case to an existing pattern; search and first reading of a source tree (`home-assistant/core`, matter-hub, go-hamapping, this repo) whose result a stronger model or the owner reads again | **Sonnet**, including as the model of a delegated search subagent |

Sonnet never writes a fact about Home Assistant or matter-hub into the
repo, and never touches the credential paths named in the first row. When
in doubt about which row a task is in, it is in the first.

## Agents and token use

The owner's rule (go-fabric, 2026-10-14): efficient use of tokens. A wave
costs on the order of half a million to a million tokens per implementing
agent before the owner's review; the rules below are what keeps that from
doubling.

- **Read slices, not files.** `sed -n a,bp`, `grep -n` with a tight
  pattern and `head`; a whole file only when the task is the file. Never
  re-read what is already in context; cite the earlier read.
- **Delegate first reads.** A source tree's inventory (`home-assistant/core`
  at the pinned tag, matter-hub at the pinned commit, go-hamapping, this
  repo) is a Sonnet read written to `notes/reads/read-*.md` with
  `file:line` for every fact and no judgement; the owner spot-checks the
  load-bearing lines and verifies every Home Assistant or matter-hub fact
  before it enters a brief, `docs/by_design.md` or a report — Sonnet's
  result is a map, never a citation.
- **Logs by their summary.** Guards (`make ci`, `make race`,
  `make cover-check`, `make fuzz-smoke`, `make secrets`,
  `make docker addon-image`, `make release`) run in the background with
  a summary log (HEAD before and after, one `exit` line per guard); read
  the summary, and open a guard's log only on failure, with `grep` for
  `FAIL`, `DATA RACE`, `_test.go:` and the error strings — never `cat`.
  The same for CI: the job log is fetched once to a file and grepped.
- **One turn, every independent call.** Reads, checks and background
  starts that do not depend on each other go out together; a wait is a
  `Monitor` or an `until` loop in the background, not polling by hand.
- **Short artefacts.** A brief (`notes/briefs/<date>-<wave>-<topic>.md`)
  names decisions and sections, not prose; a report
  (`docs/reports/<wave>.md`) follows the fixed shape — per section with
  the commits, guards with HEAD, differences from the brief or issue,
  decisions, left out, questions, `by_design` candidates; a `by_design`
  entry is one paragraph with the citations and one sentence of
  reasoning; status messages to the owner are short, outcome first, in
  German.
- **Skip what cannot have changed.** No image build for a change that
  does not touch the binary or the add-on; no release dry run when
  `internal/version`, `addon/config.yaml` and `changelog.md` did not
  move; no second guard run for a notes-only commit — say what was
  skipped and why.
- **Edit by script.** Several replacements in several files are one
  scripted call with asserted anchors, not a chain of interactive
  edits; a generated file is regenerated, never patched.
- **Agents read what the brief names.** An implementing agent gets the
  file list and the pinned sources in the brief, reads those, and reports
  in the fixed shape; it does not survey the tree. The brief is copied to
  `notes/briefs/` in the wave's first commit. One agent per brief; two
  agents only on disjoint paths.
