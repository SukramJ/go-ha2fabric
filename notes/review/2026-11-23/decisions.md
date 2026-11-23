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

## 2026-11-23, after phase 0's verification

Put as one wizard of eight questions, the recommendation first; the owner
took every recommendation. The held row is named by ID only until its fix
merges.

| # | Subject | Decision |
| --- | --- | --- |
| 13 | HR-P0-auth-20 (held) | **The verifier's fix shape** (`held/phase-0-verify.md`), host-only; go-fabric-ui's contract is unchanged. Published with the phase 0 pull request |
| 14 | HR-P0-cfg-21, the default passcode `20202021` | **A random passcode on first start** when `PASSCODE` is unset, kept beside the store as the registry nodes' are; the add-on option becomes optional without a default; an explicit `20202021` is honoured with a status warning and a log line. Behaviour change, `changelog.md` |
| 15 | HR-P0-cfg-22, `WEB_ADMIN` off loopback without `WEB_ALLOW` | **Refused at start** (`config.Load`, exit 2, naming both remedies), no override switch; HR-P0-err-26 (an unparsable `Allow` entry turns the filter off) fixed in the same change; the docs say the page is plain HTTP and an address filter is not authentication |
| 16 | HR-P0-leak-24, pairing codes in the log | **Only while the node has no fabric**; a commissioned node logs that the codes are on the administrator's pairing page |
| 17 | HR-P0-ha-23 (low after verification), the token over `ws://` to another host | **A warning, not a refusal**: a start-up log line and a status warning when the scheme is `ws` and the host is neither loopback nor `supervisor`, a docs paragraph and a `wss://` note; with HR-P0-ha-31 the WebSocket dial no longer follows redirects |
| 18 | Holding the public mediums | **Public now**: HR-P0-cfg-21, -cfg-22 and -ha-23 rest on conditions `README.md` and `addon/DOCS.md` already publish; only HR-P0-auth-20 stays held |
| 19 | Other add-ons on the Supervisor's network (extends decision 11) | **Untrusted**: a co-installed add-on is not an administrator; the peer address alone is no credential (decision 13), and the add-on's listener is bound tighter if that is cheap (HR-P0-cfg-27) |
| 20 | When the phase 0 fixes land | **All in phase 0's pull request**: one fix wave (at most two Opus implementers, the Fable read on the admin, configuration and token paths); the fuzz targets, every fix and the crash seeds in one pull request; `review/p0` is not pushed before the fuzz finds are fixed |

## 2026-11-23, after phase 0's pre-merge read

| # | Subject | Decision |
| --- | --- | --- |
| 21 | Publishing HR-P0-auth-20 (no tag carries the fix at the merge, decision 8) | **Neutral wording now, held until the tag**: code comments, docs and the squash message describe the admin check as it is, not the condition before it; the held reports stay unpublished until the `0.7.0` tag (the owner's release go with the live Ingress test) and are then published in a notes pull request. Replaces decision 13's "published with the phase 0 pull request" |
| 22 | HR-P0-cfg-27, binding the add-on's listener tighter | **After the live test**: the listener stays on `0.0.0.0:8486` (the peer filter and the header check hold); the owner's live Ingress test checks the Supervisor's source and gateway address, then a follow-up issue binds it |
