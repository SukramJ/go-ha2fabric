# Owner decisions, whole-codebase review

Each put as a wizard step with the recommendation first; the owner's choice
is the row. IDs are `HR-…` (`README.md`, "Finding IDs"). Decisions of
go-fabric-ui's review are cited as "ui decision n"
(`go-fabric-ui/notes/review/2026-11-17/decisions.md`).

## 2026-11-17, for the three repositories (restated)

| # | Subject | Decision |
| --- | --- | --- |
| 1 | Scope | go-fabric-ui, go-fabric-server and go-ha2fabric, one after the other; the Loom repositories are excluded (ui decision 1). On 2026-11-23 the owner put go-ha2fabric before go-fabric-server |
| 2 | Cut | **Adapted, five phases**: 0 basis, 1 package pass, 2 flows and adversarial, 3 architecture, tests, docs and supply chain, 4 convergence; no matter.js parity phase (ui decision 2) |
| 3 | Baseline | **origin/main at the start**: `10ee7b4fb57ca6e9889ecf9e1189bbcd91212025` (2026-11-23); what lands later is delta (ui decision 3) |
| 4 | Models | **Opus by default**; **Fable** for the security paths, the verification of high and medium, every reference citation and planning; **Sonnet or a script** for inventories (ui decision 4) |
| 5 | Process | Workflow per phase, one bundled fix pull request per phase, squash, auto-merge on green CI; open security items held back until their fix merges (ui decision 5) |
| 6 | Agent caps and sessions | **At most five agents per phase for review and verification, at most two for implementation; one fresh session per phase**; the next repository's plan is written while this one is in phase 4 (ui decision 6) |

## 2026-11-23, before phase 0

| # | Subject | Decision |
| --- | --- | --- |
| 7 | Phase 1 cut (about 9 000 production lines) | **One phase**, not halves: Fable S-unit (web wiring, configuration and secrets, the HA client and token, the camera relay, the node and selection files, the add-on's `run.sh` and manifest), two Opus readers for the rest, one Fable verifier, inventory by script; one branch, one pull request |
| 8 | Releases during the review | **No tag in the review.** Each phase's pull request merges into main; the fixes land in the unreleased `0.7.0` with their `changelog.md` lines. A tag only on the owner's release go, together with the owed live Ingress test (replaces ui decision 15 here) |
| 9 | go-hamapping | **Boundary only**: the review reads how this repository calls go-hamapping (plans, rows, the go-fabric types its API carries); a defect inside go-hamapping becomes an issue there, not a fix here |
| 10 | Fuzz targets | **Standing**: every target written in phase 0 is committed and joins `make fuzz-smoke` (the CI job exists); a hit becomes a finding with its seed as the regression (replaces ui decision 9 here) |
| 11 | Trust in the threat model | **Home Assistant trusted, its data not**: Core, the Supervisor and the token are trusted; entity data (names, attributes, camera offers, stills from integrations) is untrusted input; a commissioned Matter controller is a fabric administrator but reaches only the exposed entities' services; the LAN is untrusted (the web listener on the host network, the Matter port) |
| 12 | Live Home Assistant | **Fixtures only**: Home Assistant facts from `home-assistant/core` at `2026.10.3`, cited, never run; tests on `hatest` and `testdata/ha/`. The live Ingress test stays the owner's release gate, outside the review |
