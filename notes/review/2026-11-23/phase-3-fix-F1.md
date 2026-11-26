# Phase 3 fix wave — F1 (code, tests, guards)

Brief `notes/briefs/2026-11-26-review-p3-fix.md`, section F1. Branch
`review/p3`, base `88358bf`; commits `3192856..HEAD` (see Guards for the
HEAD). Opus, no sub-agents. `PATH=$HOME/go/bin:$PATH`, gofumpt v0.12.0
(the Makefile's pin). Scratch: the session's scratchpad `p3F1/`.

## Per row

### Decision 54, HR-P0-cfg-27 — the add-on's web listener (`3192856`)

- **Shape chosen:** in the add-on (`SUPERVISOR_TOKEN` set), an
  *unspecified* `WEB_LISTEN` host (`0.0.0.0`, `::`, empty) becomes the
  address the Supervisor gives the add-on: `GET
  http://supervisor/addons/self/info` → `data.ip_address`, read in
  `cmd/ha2fabric` (`addonAddress`, `addonWebListen` in `supervisor.go`,
  called in `main.go` before `web.New`); the port stays. `script/run.sh`
  keeps `WEB_LISTEN="0.0.0.0:8486"`, which is the fallback. An explicit
  host stays as given.
- **Why the Supervisor's API and not the interface:** the Ingress proxy
  connects to exactly that value — `sup:supervisor/api/ingress.py:125-127`
  (`_create_url`: `http://{app.ip_address}:{app.ingress_port}`),
  `api/apps.py:308` (`ATTR_IP_ADDRESS: str(app.ip_address)`), for a
  host-network add-on `docker/app.py:188-191`
  (`self.sys_docker.network.gateway`), all at `fe91b8db` (read with
  `gh api`). The interface name `hassio` would be a guess at the
  Supervisor's convention; the API value is the proxy's target by
  construction. The add-on's token may read it without `hassio_api`
  (`api/middleware/security.py:107-116`, `/addons/self/[^/]+` in the
  bypass). Not `internal/config`: the configuration stays pure (no network
  read), and `cmd` already holds the Supervisor client (no redirect, no
  proxy from the environment); `supervisorGet` is now shared by the gate
  and this read.
- **Not found:** three tries, 1 s apart; an error, a result other than
  `ok`, or an unspecified / unparsable `ip_address` (the Supervisor's
  "no address" is `0.0.0.0`) → the bind as given, one `WARN
  ha2fabric.web_listen` with the error and a hint. Found → `INFO
  ha2fabric.web_listen listen=172.30.32.1:8486`. The client filter
  (`WEB_ALLOW=172.30.32.2`) is unchanged.
- **Tests:** `TestAddonAddress` (found; `0.0.0.0`; not an address;
  missing; error result; undecodable; 403), `TestAddonWebListen` (found
  for `0.0.0.0:`, `:`, `[::]:`; not found → kept with one warning after
  `addressTries` reads; explicit hosts kept).
- **Difference:** no unit test drives `run` itself into the bind (it would
  need a fake Supervisor at `http://supervisor`); the two functions carry
  it. Live check on a HAOS box is the coordinator's/owner's.
- Same commit: HR-P3-cfg-68, `label: str(1,32)` in `addon/config.yaml`
  with a comment (characters per the Supervisor, the daemon's 32 bytes and
  blank refusal stay).

### Decision 60 — HR-P3-test-01 (`39222c7`, `218658d`)

- `script/coverfloor`: `check` fails a package below its floor, a floor
  more than `staleFloorSlack` = 3.0 points under the measurement (a zero
  floor without slack) and a package with statements and no floor;
  `main_test.go` (red cases: 94 % vs floor 82 → "raise it to 94"; no
  floor; zero floor that measures). The profile parser is kept (verify).
- **Difference from A's fix:** no exemption list — every package has a
  floor, `internal/ha/hatest` and `script/*` too (brief: "every package
  needs a floor"); no per-floor spread column (the 3-point slack covers
  the measured 0.3–0.6 spread).
- Floors = whole percent below the lower of two full runs (run 1 / run 2):
  cmd 88 (88.2/88.2), adapt 96, bridge 95 (95.3/95.4), camera 95, config
  98, expose 97, exposure 96, ha 94 (96.5/95.9, then 94.7 in the guard
  run: a 1.8-point spread, `f2f9aa2`), hatest 95, nodes 91, plan
  99, selection 93, text 100, version 90, web 96, script/cite 79,
  script/coverfloor 81. Both runs exit 0 against them.
- Bite: stale check disabled → `TestCheck` red ("stale floor", "zero
  floor measures"); missing-floor check disabled → red ("no floor").

### Decision 60 — AST guards in `internal/arch` (`6f68e2b`)

Measured first: `internal/plan` imports `encoding/json fmt maps slices
sort strings`, `internal/expose` `fmt maps path slices strings`, no `go`
statement in either; `internal/adapt`'s go-fabric imports are
`cluster/spec/keypadinput`, `cluster/spec/levelcontrol`; the only
environment read in non-test code is `internal/config/config.go:199`
(`os.LookupEnv`), none in `cmd`; 15 `Fuzz*` = 15 `FUZZ_TARGETS`.

- api-04 `TestPure`: standard-library allowlist per pure package (plan,
  expose) and no `*ast.GoStmt`.
- api-05: `adaptGoFabric` freezes adapt's non-test go-fabric imports.
- api-06: every walked package directory needs a `layers` entry, `.`,
  `script/coverfloor`, `script/cite` included (no exemption list).
- api-07 `TestEnvironment`: `os.Getenv/LookupEnv/Environ/ExpandEnv`,
  `syscall.Getenv/Environ` (import alias resolved) only in
  `internal/config`.
- test-13 `TestFuzzTargets`: `FUZZ_TARGETS` (continuations joined) equals
  the module's `func Fuzz*` both ways.
- `TestASTGuardsRefuse` holds each on synthetic sources.
- Bites (mutate, red, revert; all exit 1): plan imports `sync`, `os`,
  `time`, `net`; `func init() { go func() {}() }` in plan; adapt imports
  `cluster/spec/onoff`, `node`; a new `internal/foo`; `doc.go` imports
  `internal/bridge`; `os.Getenv("SUPERVISOR_TOKEN")` in `cmd/ha2fabric`;
  `FuzzX` in `internal/text` not listed.

### Decision 61 — the citation check (`1320514`, `872e737`)

- `script/cite` (Go, no dependency), `make cite` (in `make ci`,
  `CITE_PATHS ?= cmd internal script testdata`). Module versions and
  directories from `go mod download -json` of the three modules. Blocks:
  Go comment groups (go/parser); text paragraphs, list items, table rows,
  headings. A path belongs to a module by a `gf:`/`ui:`/`hm:` prefix, or
  the module named last before it in the block when the path resolves
  there (exact or unique suffix) and is not this repo's own without a
  version, or — no module named — when exactly one module has the path
  exactly. Version = first `vX.Y.Z` within 300 chars after it; the marker
  "at go.mod's version" is allowed only without lines (a file the code
  reads at the pinned version: `admin_test.go`'s `contract/openapi.yaml`).
  Continuations `, :N-M` cite the same file up to the next path. History
  is skipped: `notes/`, `held/`, `changelog.md`, `addon/CHANGELOG.md`,
  `docs/reports/`. `-list` prints every recognised citation.
- Re-pin, every Go and testdata site read at its module's version (the
  ranges of C's tables checked line by line; differences: `node/config.go`
  `:956-965` (C: holds `:956-965`), `basic_information.go:649-651`,
  `fabricui/nodeswitch.go:69-73` and `fabricui/handler.go:211-213`,
  `fabricui/options.go:94-95` added to file-only citations,
  `node/mdns.go` → `node/config.go:1086-1092` (where `Validate` refuses
  an empty or repeated interface), `rows_gen.go:2365` for both the
  reciprocal and the 1..65279 bounds, file-only ones got their version).
  cite-65 in code/testdata: `message.go` `core.py:130-143`,
  `commands.py:266-275`, `get_states.json` `:561-567` added (read at core
  2026.10.3), `supervisor.go` header constant in `supervisor/const.py`
  (C's fact). cite-67: `config.go:70` `presentation.md "Ingress" at
  50b5188`.
- Unit tests (`TestCites` and others) plus `TestRepository` (the code
  paths hold). Bite: `node/config.go:956-965 at v0.25.0` in
  `internal/config/node.go` → `make cite` exit 2 naming it. At this HEAD
  `make cite` over code is green (60 citations); over the whole tree 56
  docs citations fail (README 1, by_design 4, operations 3, roadmap 48).

### Item 4

- **test-02** (`4ea0d99`): A's E.1 test with verify's corrected comment
  ("near MaxBackoff (320 ms)"); green ×3; with `if false && up >
  c.opts.MaxBackoff` red (322.9 ms > 150 ms).
- **test-10** (`f9f028c`): the fake refuses as core 2026.10.3 — unknown
  camera / camera off → `home_assistant_error` (`camera/helper.py:14-26`,
  `websocket_api/connection.py:303-305`), `SetWebRTC(id, false)` →
  `webrtc_offer_failed` / `webrtc_candidate_failed`
  (`camera/webrtc.py:213-248`), a candidate without its `candidate`
  string → `invalid_format` (`:325-339`, `connection.py:296-298`);
  `RemoveDevice` fires `device_registry_updated` remove
  (`helpers/device_registry.py:177-182`). `TestFakeRefusesAsCore` and
  `TestCameraOfferRefusedEndToEnd` (ha.Client → camera.HA → camera.Host →
  the controller gets a status error, 0 sessions, no offer taken).
  **Difference:** candidate validation checks the required `candidate`
  key only (`RTCIceCandidateInit.from_dict` lives in webrtc-models, not
  read).
- **test-11** (`4ea0d99`): `TestStatusDuringErase`, `TestStopWithinBudget`
  wait for the seam's entry (channel) instead of 500 ms.
  **Left out:** `TestCallContextKeepsConnection`'s precondition as a
  failure — made fatal, it failed under `make race` (the call returned at
  its 300 ms deadline after 0.63 s): elapsed time does not say whether
  the write was in progress, so the log stays (`f2f9aa2`). A real signal
  needs the server to report unread bytes; not done.
- **test-12** (`a836fb1`): `TestSupervisorGateBadAnswers` (undecodable 200,
  `result` ≠ ok, no `data.supervisor`) → closed, `supervisor_unknown`.
- **test-14** (`a836fb1`): `instructions` joins continuations;
  `imageListenProblems` reads both ENV forms and refuses `--web-listen` /
  `-web-listen` in ENTRYPOINT/CMD; `TestImageListenProblems`.
- **test-15**: **accepted (note), owner decision 67.** `baf60ee`'s test
  needed a seam (`buildPlan`), which decision 67 removes; the follow-up
  commit drops the seam and the test. No real input reaches the path:
  go-hamapping v0.8.0's `Plan.Build` (`plan.go:420-451`) refuses only with
  a `*BuildError` keyed by the endpoint's entity (`:431`, `:472` via
  `parts`; `Endpoint.Key` "is the entity's key", `:66-68`), which
  `bridge.build` always answers by dropping that entity and rebuilding;
  its one plain error, "plan built twice" (`:424`), the bridge never
  causes. So `bridge.go`'s whole-device branch is defensive code that
  v0.8.0 cannot reach; it stays uncovered. Guards after the removal at
  `2a7adae` (HEAD moved to F2's `c6a1f22` meanwhile): `ci` 0, `race` 0,
  `cover-check` 2 (`internal/bridge` 93.7 < 95, as verify's baseline
  measured); its floor is now 93, which the same profile passes.
- **ha-09** (`3369cba`): core sends every reply as `type: result` with
  `success` (`messages.py:63-65`, `:81-104`), a ping's as `pong`
  (`commands.py:730-736`); `callWith` now fails anything else with
  `ErrFrame`. Red first: `TestClientReplyShape` (result without success,
  a pong to call_service) was nil before. One existing test script
  answered a ping with a result; it now answers pong, as core.
- **api-08** (`3369cba`, `6d63acb`): `ha.TypeResult` is **kept** — ha-09
  uses it now (brief said drop; dropping it would leave the literal).
  Flag defaults derived from `DefaultVendorID`/`DefaultProductID`. The
  other unexports: accepted (decision 63), not done.
- **cfg-68**: in `3192856`.
- **test-03** (`966922b`): accepted (decision 63); the test comment says
  it holds the below-the-bound half only.

## Guards

Full run at HEAD `91ddf3d` (before = after): `ci` 0, `race` 2, `cover-check`
2, `fuzz-smoke` 0, `vuln` 0, `licenses` 0, `secrets` 0, `version-check` 0.
`race`: `TestCallContextKeepsConnection`'s new fatal precondition (test-11);
`cover-check`: `internal/ha` 94.7 < 95. Fixed in `f2f9aa2`; rerun at
`f2f9aa2`: `race` 0, `cover-check` 0 (ha 95.2). `ci`, `fuzz-smoke`, `vuln`,
`licenses`, `secrets`, `version-check` not rerun: `f2f9aa2` changes one
test's log and one floor (`make ci`'s lint/vet/test of that test ran green
in `race`). Lint alone: exit 0 at `91ddf3d`. Skipped: `make
release`, `docker`, `addon-image` (F2's; `run.sh` and `config.yaml`
changed in comments/schema only — the add-on image build is worth one
run by F2/the coordinator since `run.sh` is in it).

## For F2

- **Citation tool:** `go run ./script/cite docs README.md CONCEPT.md
  CLAUDE.md addon/DOCS.md` (problems only; exit 1 while any), `go run
  ./script/cite -list <paths>` (every recognised citation with its
  version and "ok"). After the docs are re-pinned the coordinator sets
  `CITE_PATHS ?= .` (or adds the doc paths) in the Makefile.
- **Changelog `[Unreleased]`:**
  - Fixed: "In the add-on the web UI listens only on the address the
    Supervisor gives the add-on (the Supervisor network's gateway, where
    the Ingress proxy connects), read at start, instead of every
    interface; when it cannot be read it keeps `0.0.0.0:8486` with a
    warning (decision 54, HR-P0-cfg-27)."
  - Fixed: "A Home Assistant reply that is not a result with `success`
    fails its command instead of passing as a success (HR-P3-ha-09)."
  - Changed: "The add-on's `label` option is `str(1,32)` in the schema."
  - Internal (if the file carries such lines): "`make cite` in `make
    ci`: every go-fabric, go-fabric-ui and go-hamapping citation names
    go.mod's version; `make cover-check` fails a floor more than 3 points
    under the measurement and a package without a floor; `internal/arch`
    holds the pure layers, adapt's go-fabric imports, layer entries,
    environment reads and the fuzz list."
- **Docs lines:**
  - `docs/operations.md:117` table: add-on `WEB_LISTEN` default →
    "add-on `0.0.0.0:8486`, bound at start to the add-on's Supervisor
    address (`GET /addons/self/info` `ip_address`, e.g. `172.30.32.1`)".
  - `docs/operations.md:121-137` "Host networking": "its web UI listens
    on the host too" → on the Supervisor network's address the
    Supervisor gives the add-on (where the proxy connects), so a LAN peer
    cannot reach the port; when the address cannot be read the daemon
    binds `0.0.0.0:8486` and logs `WARN ha2fabric.web_listen`; the filter
    to `172.30.32.2` stays either way. Decision 53's paragraph: this bind
    is the mitigation against LAN peers, not against a co-installed
    add-on on the `hassio` network.
  - `addon/DOCS.md`: one line of the same.
  - `CLAUDE.md:48-49` More guards: `make cite`; cover-check's floors are
    held both ways (stale > 3 points, a floor per package).
  - operations log list (if it has one): `ha2fabric.web_listen` (INFO
    bound / WARN not read).

## Questions

1. `docs/roadmap.md`'s finished survey items pinned at go-hamapping
   v0.2.1 (cite-66) cannot pass `make cite`, which requires go.mod's
   version. Either the roadmap stays out of `CITE_PATHS`, or the
   survey's finished items lose their path citations (the "Sources" pin
   says where they were read), or `script/cite` gets a history marker.
   Recommendation: keep `docs/roadmap.md` out of `CITE_PATHS` only if F2
   keeps the v0.2.1 pins; otherwise all docs in.
2. (Answered by decision 67: the seam is gone, test-15 accepted.)
3. The add-on bind is untested on a live HAOS box: the coordinator's
   live check should see `INFO ha2fabric.web_listen
   listen=172.30.32.1:8486` and Ingress working.
