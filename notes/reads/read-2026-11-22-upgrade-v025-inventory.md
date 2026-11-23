# Inventory: go-ha2fabric → go-fabric v0.25.0 (thingfabric.dev) + go-fabric-ui v0.13.0

Host: `/home/markus/Dokumente/GitHub/go-ha2fabric` @ ad41fb8 (go-fabric v0.22.2, go-fabric-ui v0.9.2, go-hamapping v0.4.12).
Read-only; all builds in scratch copies under `…/scratchpad/w/` (`host` = v0.24.1 + ui v0.12.0; `host25a` = v0.25.0 + ui v0.13.0;
`hm`/`hm25` = go-hamapping main lifted). Logs: `w/host-test-0241.log`, `w/host-test-025.log`, `w/hm-test-025.log`.
Confidence tags: **[build]** verified by build/vet/lint/test in scratch, **[code]** read in code, **[inferred]**.
`gf:N` = line N of `scratchpad/gf-CHANGELOG.md`; `ui:N` = line N of `scratchpad/ui-changelog.md`.

Side effect to know: I ran `git fetch` in `/home/markus/Dokumente/GitHub/go-hamapping` (remote-tracking refs only; main == origin/main == e02b1c4).

---

## 1. Blockers

### 1.1 go-hamapping — HARD BLOCKER for v0.25.0 (verdict: wave is blocked until go-hamapping releases on thingfabric.dev)

- go-hamapping v0.4.12 and main (e02b1c4, pseudo v0.4.13-0.20261121124602-e02b1c4e1839) require `github.com/SukramJ/go-fabric v0.22.2`
  (go.mod:6). No version on the new path exists. **[build: go list]**
- Its public API leaks go-fabric types in both directions **[code: `go doc -all`]**:
  `Plan.Build(ctx) ([]*device.Device, error)`, `Plan.DeviceFor(key) *device.Device`, `Endpoint.Type *device.Type`,
  `Endpoint.Options device.Options`, `Endpoint.Declarations []device.Declaration`, `Row.Type *device.Type`,
  `Row.State eligibility.State`, `Verdict` embeds `eligibility.Verdict`, `Measure.Class contract.MeasurementClass`,
  `Patch.Value` = typed `<cluster>def.Patch`. It imports 206 of the 239 go-fabric packages the daemon links.
- The host passes these straight into go-fabric. With the host on `thingfabric.dev/go-fabric v0.25.0` and go-hamapping v0.4.12 the
  build fails **[build, `host25a` before the replace]**:
  - `internal/plan/plan.go:236` — `m.Verdict` (old-path `eligibility.Verdict`) assigned to new-path type
  - `internal/bridge/bridge.go:595-596` — `mv[i].State` vs `eligibility.StateUnmappable`
  - `internal/bridge/bridge.go:604`, `:608` — `[]*device.Device` from `hp.Build` into `g.devices` / `attachCameras`
  - `internal/adapt/adapt_test.go:110` — `v.State != c.state`
  (first errors only; dependents are not type-checked past them).
- **Can the host carry both module paths in one build? No.** It does not compile, and no adapter can fix it: `*device.Device` is an
  opaque kit object (servers, ports, compose state) that only the same copy's `node` can attach; there is nothing to convert.
  Even hypothetically, a second copy would cost and risk:
  - size: go-fabric is ~9.9 MB of symbols in a 41.3 MB stripped binary (`go tool nm -size`, `host25a`); go-hamapping pulls
    206/239 of its packages, so ~+10 MB (~+25 %) **[build]**;
  - duplicate global state, each copy its own **[code, v0.25.0]**: the device-type registry (`device/type.go:207 typesByID`),
    `device/hostserver_gen.go:147 hostServers`, `device/lifecycle.go:73 composeMu` (the lock that makes attach/AddPart atomic —
    two copies would not serialise each other), the internal seams wired in `init()` (`bridge/seam.go:28`, `bridge/case_seam.go:13`,
    `bridge/facade_seam.go:8`, `node/seam.go:12`, `store/seal.go:724`, `transport/udp/listener.go:178` — type-assert on their own
    copy's types and answer false for the other's), `node/config.go:720 basicInformationChecks`; sentinel errors
    (`errors.Is(err, device.ErrNotBridged)` etc.) are distinct values per copy; `version.GoFabric()` reports one of two.
    OS singletons (mDNS :5353, UDP listen) would only collide if both copies ran a node — moot, since it cannot compile.
  Verdict: not an option; do not attempt a bridge layer.
- **go-hamapping is already broken on go-fabric ≥ v0.23.0**, on the old path too **[build]**: main builds, vets and is SA1019-clean
  at v0.23.0/v0.24.0/v0.24.1, but `go test .` fails (passes at v0.22.2):
  - `TestRowsNameGoFabricTypes` (`table_test.go:33`, 58 rows) and `TestCamera` (`rows_camera_test.go:96,102`): compare
    `*device.Type` by pointer; since v0.23 `<Name>Type()`, `device.Lookup`, `device.Types`, `Device.Type` return copies —
    "compare types by `ID`" (gf:682). Test-only as far as seen; non-test pointer compares in go-hamapping not audited.
  - `TestFan` (`rows_test.go:531`): after a controller write of FanControl `FanMode`=Off no command arrives (10 s timeout).
    Candidate cause gf:625 ("a host hears of a controller's write only when the write changes the value") or gf:624 — **not
    diagnosed [inferred]**. This is a product path (fan off from a controller), not just a test: go-hamapping v0.4.12 must not
    be run against go-fabric ≥ v0.23 until this is understood.
- What a go-hamapping release on `thingfabric.dev/go-fabric v0.25.0` needs, roughly **[build, `hm25`]**:
  1. the go-fabric changelog one-liner (gf:27-32) over `*.go` + `go.mod`, `go get thingfabric.dev/go-fabric@v0.25.0`, tidy, gofumpt
     — builds and vets clean with no source change;
  2. `make generate`: after the bare rewrite `TestGeneratedRowsAreCurrent` (`rows_gen_test.go:33`) and `internal/gen/cmd TestRun`
     (`main_test.go:42`) fail — `rows_gen.go` is stale against the generator;
  3. type identity by `ID` in tests (and any non-test compare);
  4. diagnose and fix `TestFan`; review its rows against gf:567 (BooleanState ChangeEvent default), gf:568 (WindowCovering unknown
     initial position), gf:669 (`device.New` refuses bare Fan/ModeSelect/RoboticVacuumCleaner/Camera/… without required Initial),
     gf:727 (bridged SerialNumber/NodeLabel only when stated), gf:110 (string bounds in UTF-8 bytes; `options.go:63 cut`);
  5. its `internal/arch`, THIRD-PARTY-NOTICES, README/CONCEPT/CLAUDE links, changelog; a tag (API types change path → a minor).
  go-hamapping changes are made directly, in its own PR (host CLAUDE.md, "Work that belongs in go-fabric").

### 1.2 go-hamqtt, go-ha-catalog — no blocker **[build]**
go-hamqtt v0.40.0 requires go-ha-catalog + go-mqtt only; go-ha-catalog has no SukramJ requirement. Neither imports go-fabric.

### 1.3 go-fabric-ui — no blocker, but it forces the path
v0.10.0/v0.11.0 → go-fabric v0.23.0; v0.12.0 → v0.24.1 (old path); v0.12.1/v0.13.0 → `thingfabric.dev/go-fabric v0.25.0`
(go.mod at each tag). go-fabric-ui's own module path stays `github.com/SukramJ/go-fabric-ui`. ui ≥ v0.12.1 hands
`node.Node` types of the new path to `fabricui.NewNodeSwitch` etc. → ui v0.13.0 is only possible together with v0.25.0 (ui:62-68).

### 1.4 Interim stop (v0.24.1 + ui v0.12.0, old path) — compiles, not recommended
Host builds/vets clean there with go-hamapping v0.4.12 lifted by MVS **[build]**, but that runs go-hamapping on a go-fabric it
fails its own tests on (1.1), and v0.24.1 is the deprecated end of the old path. Use only as a scratch method, not a release.

---

## 2. Compile breaks

With go-hamapping moved (scratch `replace` to `hm25`): **none**. `go build ./...` and `go vet ./...` are clean at
v0.24.1 + ui v0.12.0 and at v0.25.0 + ui v0.13.0 **[build]**. Without it: the four sites in 1.1.

Mechanical items of the move (all covered by the one-liner gf:27-32, but check each) **[build/code]**:
| What | Where | Note |
| --- | --- | --- |
| imports `github.com/SukramJ/go-fabric/...` | 22 packages' imports across `internal/**`, `cmd/**` (node ×8 files, device ×7, fabrictest ×9, controller/client ×6, cluster/spec/* , eligibility ×4, clock ×3, contract ×2, cluster/camera ×2, logring ×1) | rewrite |
| runtime string | `internal/version/version.go:29` `dep("github.com/SukramJ/go-fabric")` | not an import: left unrewritten it silently yields `unknown` in `--version`/Operation info; `version_test.go:23` only checks the substring "go-fabric" and would not catch it |
| import guard | `internal/arch/arch_test.go:33` `goFabric = "github.com/SukramJ/go-fabric/"` | rewritten by the one-liner; arch test passes after **[build]** |
| go.mod | `go.mod:6` | the one-liner turns it into `thingfabric.dev/go-fabric v0.22.2`, which does not resolve — delete the line, then `go get thingfabric.dev/go-fabric@v0.25.0 github.com/SukramJ/go-fabric-ui@v0.13.0`, `go mod tidy` (old path leaves the graph entirely once go-hamapping moved) |
| formatting | `internal/adapt/adapt_test.go`, `internal/adapt/command.go`, `internal/bridge/bridge.go`, `internal/bridge/camera.go`, `internal/camera/camera.go`, `internal/camera/camera_test.go`, `internal/plan/plan.go` | gofumpt/goimports: import blocks re-sort (`thingfabric.dev` after `github.com`); `make fmt` |
| lint (errcheck) | `internal/bridge/bridge.go:244` `d.SetReachable(ctx, false)` | `Device.SetReachable` returns `error` since v0.23 (gf:789); compiles as a statement, fails `make lint`. Handle/log; `device.ErrNotBridged` on a part |

---

## 3. Deprecated go-fabric uses (SA1019)

**None** — in code and tests **[build]**: `staticcheck -checks SA1019 ./...` (2026.2.2) and golangci-lint 2.14 (`staticcheck`,
`tests: true`) report 0 at v0.24.1 + ui v0.12.0; full golangci-lint at v0.25.0 reports only the items of §2.
Cross-check by grep of every `Deprecated:` identifier in the packages the host imports (node: `EndpointPersistence`,
`SupportedFabrics`, `MaxPathsPerInvoke`, `Network.BLE`, `Node.Persistence`, `ErrCommissioningWindowAlreadyOpen/NotConfigured`,
`OTAApplyPolicy`, `OTAProviderConfig.ApplyPolicy/RebootQuery/TransferBlockSize`; device: `NewRootNode`; contract: `BLEPeripheral`;
cluster/camera + cluster/spec: `DataVersion` trackers, `spec.Hooks`, `Server.Notify/NotifyFrom/ChangedFrom/ReportFrom/Emit`,
`EmitOn`, `ServerConfig.BeforeWrite`, `RequestCheck.Unchecked`): no hit in the host **[code]**. go-fabric's own v0.24.0
Deprecated section states "No use in … go-ha2fabric (ec7deea)" for its entries (gf:188-190). No `//nolint` for staticcheck exists
in the host. Migration guide sections to cite in the report if anything turns up later: "Changed in v0.23.0: the node, the device
and the controller" (guide :544), "Moved in v0.25.0: the module path" (guide :787).
go-hamapping main is SA1019-clean at v0.24.1 as well **[build]**.

---

## 4. Behaviour changes that touch this host

| Change | Host path | What to do | Conf. |
| --- | --- | --- | --- |
| Store schema v7 → v12 (gf:146, :517, :639, :651, :697). Host uses `node.Store{Path}` (`internal/config/node.go:60`), no own backend, no `Migrations()` pin | every node's store (configured + registry) | Nothing in code; `Upgrade` runs at start. Operator note: one-way — an add-on downgrade/restore onto a v12 store is not expected to open (behaviour of v0.22.2 on a v12 file not tested). Say so in changelog / addon CHANGELOG / operations.md | code + inferred |
| String bounds count UTF-8 bytes (gf:110); stored over-long labels are cut, not refused (gf:111) | `internal/config/config.go:267-270` (LABEL ≤ 32 bytes, `len`), `cmd/ha2fabric/nodes.go:385-390` (registry label cut to 32 bytes on a rune boundary) | Already bytes — nothing. Bridged names are go-hamapping's (`options.go:63 cut`) → its wave | code |
| `Config.Validate` refusals (gf:798, :674, :670, :365, :364) | `internal/config/node.go:40-63`: VendorName "go-ha2fabric" (12), ProductName "Home Assistant bridge" (21), NodeLabel ≤ 32 checked, SoftwareVersionString = version, no SerialNumber, no HostName, `Health.Interval = node.DefaultProbeInterval`, Hints unset | Nothing refuses; all bridge/cmd e2e node starts pass at v0.25.0 | build |
| Empty NodeLabel = ProductName, no longer refused (gf:575) | `config.go` defaults LABEL to "Matter Bridge" | none | code |
| Root SerialNumber no longer derived from UniqueID (gf:727) | `node.go:53-59` states none → root BasicInformation serves no SerialNumber | Visible to controllers. Owner: state one (what?) or accept | code |
| mDNS `DT`/`_T<id>` inferred from the first bridged device's type instead of RootNode 22 when `ProductDescription.DeviceType` is zero (gf:566) | `node.go:52` leaves it zero | Commissioner UIs pick the icon from DT; it now depends on which entity is first. Owner: set Aggregator explicitly or accept matter.js's inference | code |
| `Commissioning.LocationCapability` zero = IndoorOutdoor (was Indoor) (gf:563) | `node.go:46-51` sets none | none unless the owner wants `IndoorOnly` | code |
| `Device.SetReachable/Rename/Update` return error (gf:789) | `bridge.go:244` | §2 | build |
| Controller write that changes nothing is not delivered/reported (gf:624, :625) | commands reach the host through go-hamapping's `OnChange` (`bridge.go:609`, `internal/bridge/command.go`) | go-hamapping's wave (TestFan). Host: re-run the C1/C2/C3 e2e after; they pass today at v0.25.0 with unmigrated rows, which proves little for fan | build + inferred |
| BooleanState ChangeEvent default (gf:567), WindowCovering null initial position (gf:568), `device.New` required Initial (gf:669) | via go-hamapping rows; host shows a refusal as verdict "go-fabric refused the device" (`bridge.go:592-600`) | go-hamapping's wave; host e2e (cover C1, vacuum, select, camera R6) pass at v0.25.0 | build |
| `spec` Set/Initial held to struct conformance; a WebRTC session returned without streams → FAILURE (gf:735); SetStreamPriorities judging (gf:135) | `internal/camera/camera.go:300` ff. | `internal/camera` and `bridge_r6_e2e` pass at v0.25.0; no change seen. Still off by default (`CAMERA`) | build |
| WebRTC **requestor** ACL grant (gf:571) | host serves the provider, not a requestor | not used | code |
| Reassemble that removes an endpoint ends no subscription; log `matter.bridge.reassemble.endpoint_removed` gone (gf:570) | bridge batches (`node.Batch`); docs | none: no hit for the key in `docs/`, `README.md`, `addon/` | code |
| Subscription MaxInterval ≥ 2 s (gf:569); fabrictest `Commission` waits for the CASE session (gf:158) | tests | pass | build |
| OTA provider / softwareupdate / `OTAProviderConfig.GrantFabric` (gf:562, :192-195) | none (`grep OTA` empty) | not used | code |
| `schema.SchemaSnapshotSHA256` pin (gf:141, :564) | not pinned | not used | code |
| Store port growth, `StoreFacade`, `cluster/core` wiring, `bridge` single-run/HandshakeKey, sqlstore, BLE/GATT, JointFabric consent, cluster/lock alias, levelcontrol scenes | host stays on the facade (`internal/arch`) | not used | code |
| Log line `case.session_established` now at completion, `case.session_establishing` at Debug (gf:117) | host docs | none: not quoted in `docs/`, `README.md`, `addon/` | code |

Not read line by line: the ~1000 non-breaking bullets of v0.23.0 Added/Changed/Fixed (gf:304-1141). Covered instead by: every
"Breaking" bullet (read), keyword scan (Validate, UTF-8, NodeLabel, node/device/fabrictest/camera/logring prefixes) and the test run.

---

## 5. go-fabric-ui wiring (v0.9.2 → v0.13.0)

Wiring today: `cmd/ha2fabric/main.go:112-116` → `fabricui.Options{Node, Exposure, Admin, Reassemble, Events, Warnings, Counters,
Erase, Logger, Operations, Registry, NodeConfig}`; handler built by `fabricui.Handler` in `internal/web/web.go:69` behind the
source-address filter (`web.go:76-88`). **No controller is wired** (`Options.Controller` unset; no `fabricui/gofabric` import,
no own `ControllerSource`); no `SoftwareUpdateSource`. Admin: add-on `web.From("172.30.32.2")`, stand-alone `web.ReadOnly`,
`WEB_ADMIN` → nil = token mode (`main.go:185-194`).

Tests failing at the target, all UI-caused, 7 **[build, `host-test-025.log`]**: `cmd/ha2fabric TestRun`,
`internal/bridge TestLabelWithAComma / TestWebUI / TestUIWarnings`, `internal/web TestOperationPage / TestHandler / TestAllow`.
Everything else passes (adapt, arch, camera, config, expose, exposure, ha, nodes, plan, selection, version, the non-UI bridge e2e).

| Hosts item | Used? | What the host must do | Conf. |
| --- | --- | --- | --- |
| **Host check on every route** (ui:338-347), proxy semantics (ui:230-242, ui:99-104) | **Yes — breaks the add-on.** HA core's ingress view copies the browser's headers, `Host` included (`core/homeassistant/components/hassio/ingress.py:28-37, 266-272` in the local checkout 2026.11.0b6 — not the pinned 2026.10.3, and the Supervisor hop is not read; a Fable fact note is needed before this is cited), so the add-on sees e.g. `homeassistant.local:8123` → 403 `host_not_allowed` on every request, bundle included | Add-on: `AllowHost: web.From(config.IngressProxy)` (admit by the proxy's source address; embedding.md "Behind an Ingress", second kind). Stand-alone: a daemon reached by IP or `localhost` passes; reached by a DNS name it needs `AllowedHosts` → new option (e.g. `WEB_ALLOWED_HOSTS`) or documented limitation — owner decision. Tests: `httptest.NewRequest` defaults `Host: example.com` → `web_test.go:39-52`, `operation_test.go:60` need a loopback/IP Host | build (tests) + code (core) |
| **Origin on admin routes** (ui:348-351) | Yes | With `AllowHost`, an Origin must equal the request's own host:port — holds when the proxy forwards the browser's Host; if the Supervisor rewrote Host to the add-on's IP the Origin would be refused on every admin route. Decides whether AllowHost alone suffices → live test under Ingress is mandatory before release | inferred |
| **`X-Fabric-UI: 1` in host mode** (ui:331-337) | Yes (add-on and ReadOnly are host mode) | The shipped page sends it. Host tests acting as admin must: `bridge_h3_e2e_test.go:144-157` (`ui.do`), `:217` (stream), `operation_test.go` helper. `nodes_test.go:84-87` (token mode) already passes | build |
| **Admin-only reading routes** (ui:322-330) | **Yes — changes the product.** Stand-alone default (`web.ReadOnly`) loses every page but status: `/node/exposure`, fabrics, endpoints, diagnostics, pairing, operation log → 403. `main_test.go:97` polls `/api/v1/node/exposure` without rights → TestRun never healthy; `bridge_h3_e2e_test.go:399` sees warnings reduced to codes | Owner decision: keep (stand-alone without `WEB_ADMIN` = status only), or change the default. Then fix `README.md:160`, `docs/operations.md:303-311` ("read-only" no longer means "reads everything"), TestRun's probe (`/api/v1/node/status` or token mode) | build |
| **Strict JSON bodies** (ui:352-357) | Tests only | Host tests already send `Content-Type: application/json` (`bridge_h3_e2e_test.go:155`, `nodes_test.go:84`, `operation_test.go:25`); bodies with unknown members → 400, re-check after the header fixes | code |
| **`Handler` takes the default prefix only** (ui:358-361) | No `Prefix` set | none | code |
| **`Options.Events` is not the handler's hub** (ui:362-366) | Host only publishes (`cmd/ha2fabric/nodes.go:115,118`), never reads it | none. Note: host events reach admin streams only | code |
| Event type must match `[a-z_]+` (ui:557-560) | publishes `fabricui.EventExposureChanged/EndpointsChanged` constants | none | code |
| Bundle serves index for `/` only (ui:522-528) | `addon/config.yaml` ingress entry / docs links | check no link to a deep path (none found in a quick grep) | inferred |
| `NewHandler` returning an error (ui:375-376), `MaxStreams`, `Done` (ui:377-381) | `web.go:69` uses `Handler` (panics on bad options) | Recommended: `NewHandler` and return the error from `web.New`/`Run` once `AllowedHosts` comes from user config; pass the daemon's ctx `Done()` so streams end at shutdown (`web.go:117-121` shuts down with 5 s) | code |
| `FrameAncestors`, CSP (ui:403-414, :545-555) | Ingress parent is same-origin | none | code |
| **`ConfiguredWindow` / `WindowOnDemand`** (ui:243-250) | Not set in production (only `bridge_h3_e2e_test.go:137`) | A node without a fabric reopens its own window on the configured passcode after a page-opened window closes. Owner: accept (document in operations.md) or set go-fabric `Commissioning.WindowOnDemand` in `internal/config/node.go` | code |
| `window_kind` `administrator_basic` / `other` (ui:271-274, :122-124) | page only | none | code |
| **`ControllerSource` growth / `OnboardingCodes`** (ui:18-34), `CommissionTimeout` (ui:95-100), `ErrICDMultiAdmin` (ui:111-114), `ErrNotImplemented/ErrNotRead` (ui:106-110), commissioning 600 s bound (ui:258-260) | **No controller** | not used. Onboarding inventory on/off is no question here unless a controller is planned | code |
| **`UpdateCheckFailures`** (ui:251-257) | no `SoftwareUpdateSource` | not used | code |
| **32-bit `int64`** (ui:261-267) | host has no generated client of the contract | none; relevant only if the add-on ships armv7/i386 images (the handler is fixed there) | code |
| go-fabric path (ui:62-68) | yes | §1.3 | build |
| Problem `type` URLs still `github.com/SukramJ/go-fabric-ui/...` | tests | no string to change | build |

---

## 6. Docs / metadata to update

- `go.mod`, `go.sum` (§2). `.github/dependabot.yml:4-22`: no path patterns — nothing to change; consider whether go-fabric bumps
  should stay in the `go-minor-patch` group (go-fabric-ui took them out, ui:543-544).
- `THIRD-PARTY-NOTICES.md:20` (go-fabric source → `https://github.com/thingfabric/go-fabric`, `v0.25.0`), `:42` (go-fabric-ui
  `v0.13.0`), go-hamapping row to its new tag; `make licenses` (go-licenses on the vanity path) must be re-run.
- `CLAUDE.md:8` (link), `:9` state line (append the wave; version), and the dependency paragraph if wording names the path.
- `README.md:10, :19, :36, :187` (repo links; GitHub redirects but the owner moved), `:73` and `docs/operations.md:10`
  (`sukramj.github.io/go-fabric/reference/glossary/` → `https://docs.thingfabric.dev/…`), `docs/operations.md:7` (operations guide
  link pinned at `v0.11.0`), `CONCEPT.md:151-155`.
- `docs/operations.md:303-311` (web UI section: still says go-fabric-ui `v0.8.0`; admin/read-only semantics, Host names, the
  reopened window, store v12 note), `README.md:160` (`WEB_ADMIN` row), `addon/DOCS.md` (same statements), `docs/by_design.md` if the
  DT / SerialNumber decisions land.
- `changelog.md` `[Unreleased]`: `### Changed` lines for go-fabric (path + v0.22.2 → v0.25.0), go-fabric-ui v0.9.2 → v0.13.0,
  go-hamapping; the user-visible ones (stand-alone read rights, Host check, store upgrade one-way, mDNS DT, SerialNumber).
  `addon/CHANGELOG.md` + `addon/config.yaml` version + `internal/version/version.go:20` together (`make version-check`).
- `internal/arch/arch_test.go:33` (§2); comments citing `go-fabric … at v0.x` (`internal/config/node.go:31-39`, `web.go:5-14`,
  `main.go:139-142,153-157`) stay historically correct — update only the ones whose behaviour changed (web.go's package comment).
- `Makefile:4`, `Dockerfile:21-23`, `addon/Dockerfile:27-29`, `script/*.sh`, workflows: reference only the host's own module
  path / owner — **no change** **[code]**. `licenses/`: only coder/websocket — no change.
- `notes/briefs/<date>-<wave>.md` (brief in the first commit), `docs/reports/<wave>.md`.

---

## 7. Open owner questions

1. Order: go-hamapping wave first (v0.23/v0.24 adaptation + path move, own PR + tag), then this host — confirm; who diagnoses
   `TestFan` (it may be a go-fabric question rather than a go-hamapping fix).
2. Stand-alone UI without `WEB_ADMIN`: status-only from now on (ui v0.10 admin gates), or another default?
3. Stand-alone under a DNS name: add `WEB_ALLOWED_HOSTS` (→ `Options.AllowedHosts`), or document IP/localhost only?
4. Add-on: `AllowHost` by the Ingress proxy's address — needs the Supervisor/core fact at the pin (Fable note) and a live Ingress
   test (Host + Origin behaviour) before release.
5. `ProductDescription.DeviceType`: set Aggregator explicitly, or accept the inferred first bridged type in mDNS `DT`?
6. Root `SerialNumber`: state one or serve none?
7. Unfabric'd node reopening its own window: accept, or `Commissioning.WindowOnDemand`?
8. `LocationCapability` IndoorOutdoor as the new default — fine for a bridge?
9. Store v12 is one-way: acceptable for add-on users restoring an older add-on version; wording for the changelog.
10. Host version for the wave (0.7.0? user-visible UI semantics change) and whether the interim v0.24.1 stop is skipped (recommended).

---

## 8. Suggested slicing

- **PR A — go-hamapping (its repo, prerequisite):** (1) go-fabric v0.24.1 on the old path: tests green (type identity by ID,
  TestFan, row review per §1.1 item 4); (2) path move to `thingfabric.dev/go-fabric v0.25.0` + `make generate` + notices/docs;
  tag. Two commits or two PRs, so the behaviour fix is reviewable without the rewrite noise.
- **PR B — go-ha2fabric, one PR for the wave (repo rule), commits:**
  1. `notes/briefs/…` the brief.
  2. `chore(deps)`: one-liner rewrite, go.mod/go.sum (go-fabric v0.25.0, go-fabric-ui v0.13.0, go-hamapping new tag), `make fmt`,
     `internal/arch/arch_test.go:33`, `internal/version/version.go:29`, THIRD-PARTY-NOTICES. Gate: build, vet, SA1019 = 0.
  3. `fix(bridge)`: `bridge.go:244` SetReachable error; node config decisions (DT, SerialNumber, WindowOnDemand, IndoorOnly) as
     decided.
  4. `feat(web)`: `AllowHost` in the add-on, optional `AllowedHosts` option, `NewHandler` + `Done`; test helpers send
     `X-Fabric-UI: 1` and a loopback Host; TestRun's probe; stand-alone rights as decided.
  5. `docs`: operations.md, README, addon/DOCS.md, CONCEPT/CLAUDE links and state line, changelog.md, addon/CHANGELOG.md,
     version bump, report.
  Guards before the PR: `make ci race cover-check fuzz-smoke vuln licenses secrets release docker addon-image`; a live Ingress
  check of the add-on image (Host/Origin) — the one thing no test here covers.
