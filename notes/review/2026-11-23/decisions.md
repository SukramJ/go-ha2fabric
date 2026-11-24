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

## 2026-11-23, after phase 1's verification

Put as two wizards of four questions, the recommendation first; the owner
took every recommendation and, in row 30, also the third issue. Sources:
`phase-1-S.md`, `phase-1-GA.md`, `phase-1-GB.md`, `phase-1-verify.md`
(section 7). The held row is named by ID only.

| # | Subject | Decision |
| --- | --- | --- |
| 23 | S's four held camera lows (HR-P1-bound-02, -conc-03, -ha-04, -bound-17) | **Published now**, as HR-P0-bound-28 was: only a commissioned controller reaches them (decision 11). HR-P1-ha-04 is merged into HR-P1-ha-62 (one defect); fixed in phase 1's pull request |
| 24 | (held) HR-P1-auth-01 | **Held until it is measured and tagged**: X1 (phase 2) or the owner's live Ingress test measures it; published with the `0.7.0` tag or later, together with the tighter bind of decision 22. Until then every tracked file describes the admin check as it is (decision 21) |
| 25 | HR-P1-err-06 (with -cfg-35), a corrupt or other-version `selection.json` | **A last-good copy**: `selection.json.bak` written before each rename and read when the file does not parse; a lenient read of a file of another version; a warning that says entities turned off are exposed again when neither helps |
| 26 | HR-P1-adpt-30, a climate range setpoint from stale state | **Both**: a local stop-gap (the bounds last sent per entity, closes the race and the refused raise) and a go-hamapping issue for one range command carrying both bounds |
| 27 | HR-P1-cfg-12, the container image's listener (T6) | **Bound to loopback**: the image drops `WEB_LISTEN=0.0.0.0:8099` and takes the binary's default; breaking for image users (`changelog.md`, docs: `-e WEB_LISTEN=…` with `WEB_ALLOW`) |
| 28 | HR-P1-ha-07, -ha-08, a refused token and the back-off | **Slow down and warn**: the back-off no longer resets on an authentication that is followed by a drop; after `auth_invalid` minutes, then hourly; a status warning; no exit, in the add-on and stand-alone alike |
| 29 | HR-P1-err-64, the stop budget | **`timeout: 30` and one deadline**: `addon/config.yaml` `timeout: 30`; one deadline for the whole stop; the configured node stopped in parallel with the registry nodes; the web server's shutdown error logged |
| 30 | go-hamapping issues (decision 9) | **Three issues**: the range command (HR-P1-adpt-30), an exported stable-key helper (HR-P1-adpt-69), and the primary endpoint's independence of input order as a question (HR-P1-adpt-68) |

## 2026-11-23, after phase 1's fix wave

Put as one wizard of four questions, the recommendation first; the owner
took every recommendation and both further issues. Sources:
`phase-1-fix-F1.md`, `phase-1-fix-F2.md`.

| # | Subject | Decision |
| --- | --- | --- |
| 31 | HR-P1-conc-03, the idle end of a camera session | **1 h**, beside the cap of 8 sessions per camera; replaced by go-fabric's signal that a session's controller is gone once it exists |
| 32 | HR-P1-adpt-30, how long the range last sent is held | **30 s** (or until a state shows it); the stop-gap goes with go-hamapping #39 |
| 33 | HR-P1-cfg-34, the label ids a renamed label is followed by | **In memory only**; the warning says the exclusion lapsed, the docs recommend the label id |
| 34 | Follow-up issues after phase 1's pull request | **Three**: go-fabric's camera kit (a session cap option, a signal that a session's controller is gone, whether CameraAvStreamManagement serialises its commands; a Fable-checked hand-over); here a `--health` flag for an image health check (HR-P1-sup-19); here the add-on as an unprivileged user with a profile and a watchdog after the live Ingress test (HR-P1-sup-18) |

## 2026-11-24, after phase 2's verification

Put as three wizards of four questions, the recommendation first; the owner
took every recommendation. Sources: `phase-2-X1.md`, `phase-2-X2.md`,
`phase-2-X3.md`, `phase-2-verify.md` (questions section). The held row is
named by ID only.

| # | Subject | Decision |
| --- | --- | --- |
| 35 | HR-P2-adpt-33 (tilt covers mappable in go-hamapping v0.6.0, refused by go-fabric v0.25.0) with HR-P2-adpt-34 (256 vs 255 select options) | **Fixed in go-hamapping now**: its own pull request and release; the pin is lifted in phase 2's fix wave; if the release slips, a known-issue line in `changelog.md` for `0.7.0` |
| 36 | HR-P2-data-30, first held | **Published now** and fixed in phase 2's pull request (as decision 23); it gives an untrusted role no new reach |
| 37 | (held) HR-P1-auth-01 | **Published with the `0.7.0` tag**, with the documentation sentence and, the same day, a proposal upstream; labelled by the verifier's reading of the measurement (decision 24 continued) |
| 38 | Sync scheduling (HR-P2-err-61, -err-65, -bound-37, -bound-62, -data-30) | **One design in phase 2**: a failed sync retried with back-off; `Resync` says whether a sync was planned; a web UI warning for a sync that keeps failing; one shared snapshot reader per connection (the plan stays per node); the debounce gets a latest time |
| 39 | HR-P2-conc-60 (verifier: medium → low, a gap in HR-P1-err-64's fix) | **Full fix, low accepted**: the camera sessions end in parallel within the caller's deadline; `stopRun` bounded by the stop's context; the camera closes moved off the sync |
| 40 | HR-P2-data-35, one refused entity drops its whole device | **A partial device**: the sound entities stay; the refused one is listed unmappable with its reason |
| 41 | HR-P2-data-31, closes HR-P1-data-36 | **Dropped**: control and bidirectional control characters removed in `internal/adapt` (the joiners U+200C/U+200D kept), refused in web UI names and node labels, `--dump-plan` quotes; no go-fabric issue; the option labels a go-hamapping candidate |
| 42 | HR-P2-bound-38, heap of a large `get_states` | **256 KiB per state**: an oversized entity's attributes dropped with a log line and a verdict reason; the states held once |
| 43 | The owner's live Ingress test, phase 2's items | **The full list**: the framing origin (HR-P2-frame-02), the source address the daemon sees, HAOS's `rp_filter` (HR-P2-net-01), the held files' items; the T2 argument in `docs/operations.md` regardless |
| 44 | HR-P2-cfg-03, the Supervisor gate after it opened | **Keeps re-reading**: a downgrade closes it again; in phase 2's pull request |
| 45 | HR-P2-doc-66, an entity removed while disconnected keeps its endpoint number | **A docs sentence now and a follow-up issue** for a persisted set of served keys (a design change for the owner later) |
| 46 | The remaining lows (HR-P2-data-32, -bound-39, -bound-40, -bound-64, -err-63, -test-42) | **All in phase 2's pull request**, each with a red regression test: the option not trimmed; Home Assistant's error text cut; the Home Assistant side of the camera relay capped like the controller side; a rate-limited summary for dropped items; the device state restored after a dropped or refused command; a fuzz target `Map` → `Build` → `Apply` in `fuzz-smoke` |

## 2026-11-24, during phase 2's fix wave

From 2026-11-24 the owner set Opus for every role this plan gives Fable,
until further notice; the go-hamapping fact note below and every later
read of phase 2 are by Opus.

| # | Subject | Decision |
| --- | --- | --- |
| 47 | Decision 35's scope, after the fact note (`go-hamapping/notes/reads/fact-2026-11-24-cover-tilt.md`): covers with tilt build but two command gaps remain | **Build and both gaps**: a tilt-only cover's open, close and lift go to the tilt axis; a tilt without `SET_TILT_POSITION` uses new methods `open_tilt` / `close_tilt`; go-hamapping's pull request C8, this host's `internal/adapt` in round 2. HR-P2-adpt-43 (a thermostat whose `min_temp` is above its `max_temp`, mappable and refused; found by F1's fuzz target) joins C8 |
| 48 | A tilt without `SET_TILT_POSITION`: where `GoToTiltPercentage` opens | **At 0 % only**, as the lift without `SET_POSITION` (matter.js); home-assistant-matter-hub opens from 50 % — a recorded divergence |
| 49 | Cover rows outside C8: `stop_tilt`, window covers as Rollershade, `PositionAwareLift` always | **go-hamapping issues** #43, #44, decided after the owner's live tests (with go-hamapping #15); HR-P2-adpt-43's remaining shapes → #45 |
| 50 | When the `sync_failed` warning shows | **From the second failure in a row**; the first is retried silently after 5 s |
| 51 | A `READ_LIMIT` option | **No**: the 256 KiB cap per state (decision 42) keeps `get_states` far below 64 MiB, and the sync error now names the limit |
| 52 | HR-P2-bound-90 (pre-merge read): an entity whose state exceeds the 256 KiB cap is mapped from its stripped state as another device type | **Unmappable with a reason** in `plan.Assess` ("state over 256 KiB"); no change of device type, the operator sees the reason; extends decision 42 |
