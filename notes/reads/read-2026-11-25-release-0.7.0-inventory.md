# Inventory: the 0.7.0 release — what is new in the family and not yet used

Host: `/home/markus/Dokumente/GitHub/go-ha2fabric` @ f7885e6 (main; go-fabric v0.25.0, go-fabric-ui v0.14.0,
go-hamapping v0.7.0; unreleased 0.7.0, last release v0.6.3). Read-only. `held/` of the review worktree was not read.
Trial lift in a scratch copy (`git archive HEAD`, scratchpad `ha2f/`): `go get thingfabric.dev/go-fabric@v0.26.0
thingfabric.dev/go-fabric-ui@v0.14.1 github.com/SukramJ/go-hamapping@v0.7.1`, `go mod tidy`.
Confidence tags: **[build]** verified in the scratch copy, **[code]** read in code, **[inferred]**.

Sources, cited at their tags:
`gf:N` = go-fabric `CHANGELOG.md` at `v0.26.0`, line N; `ui:N` = go-fabric-ui `changelog.md` at `v0.14.1`;
`hm:N` = go-hamapping `changelog.md` at `v0.7.1`. Host lines are at f7885e6.

## 0. The trial lift, in one paragraph

The three lifts are one move: go-fabric-ui v0.14.1 and go-hamapping v0.7.1 both require go-fabric v0.26.0
(ui:15-18, hm:13-16), so neither can be taken alone. In the scratch copy: `go build ./...` and `go vet ./...`
clean; `golangci-lint run --enable-only staticcheck` **0 issues** (SA1019 included, the config disables no check):
the host uses **no deprecated identifier** of v0.26.0, so the owner rule needs no migration **[build]**.
`go test ./...`: every package passes except **one bridge test, `TestFailedBatchOperationIsRetried`
(`internal/bridge/bridge_p1_e2e_test.go:67`, "the light was not added again")** — the regression predicted
by gf:72 below **[build]**. The lift is therefore not a pin-only change: it needs the fix in 1.2.

## 1. go-fabric v0.25.0 → v0.26.0

### 1.1 Table

| # | Item | Source | Bridge use (f7885e6) | Class | Effort | Recommendation |
| --- | --- | --- | --- | --- | --- | --- |
| G1 | CASE session installed from the handshake step's own value; with `Network.TCP` on a second route existed (inferred upstream) | gf:21 | The node runs TCP incoming with `CAMERA` (`internal/config/node.go:50`): the daemon is in the reach the entry names | **mandatory** (security fix, reaches us with CAMERA) | S (the lift) | Lift in 0.7.0 |
| G2 | `node.Batch` judged at its end state: key refusals (`ErrDuplicateKey`, `ErrUnknownKey`, `ErrReservedKey`, `ErrPolicyMismatch`, Matter-time refusal, `*surface.WithheldError`, attach error) may arrive from `Batch` at the commit, joined, no longer from the call; an Add of a key the set holds is **held** and returns nil at the call | gf:72; doc `node/aggregator.go:520-548` (`Partition.Batch`), held Add `node/aggregator.go:622-650` | The one call site: `internal/bridge/bridge.go:675-692` (`Aggregator().Batch` with `bt.Remove` :680, `bt.Forget` :682, `bt.Replace` :684, `bt.Add` :686). Errors are kept by op index only from the call (`opErrs[i]`); `settle` (`bridge.go:803-841`) takes an op without a call error as applied. A commit-time refusal is in `err` (logged `bridge.batch`, :690) but no op is marked refused, so the group keeps its signature and the next sync never adds it again. **Proven by the failing test above** | **mandatory** | M | Fix in 0.7.0 (see 1.2) |
| G3 | `Batch` after its `fn` returned answers `ErrBatchDone` | gf:39, gf:72 (`node/aggregator.go:65`) | The bridge never keeps `bt` beyond `fn` (`bridge.go:675-692`) **[code]** | none | — | Nothing to do |
| G4 | `Aggregator.Forget` / `Batch.Forget` of a part key another endpoint serves is refused `ErrUnknownKey`; a Forget of a key a device of the same batch adds as a part no longer refuses | gf:71, gf:72 | `bt.Forget` `bridge.go:682`; a failed Forget is re-queued by `settle` (`bridge.go:812-817`) — but only for a call error; a **held** Forget refused at commit is lost the same way as G2 | mandatory (part of G2) | (in G2) | Same fix as G2 |
| G5 | `Device.AddPart` on a device a node holds refuses a duplicate part key (`node.ErrDuplicateKey`) | gf:75 (`device/device.go:795`) | Not called by the host (no hit in `internal/`, `cmd/`); go-hamapping calls it in `Plan.Build` before the node holds the device (go-hamapping `plan.go:469` at v0.7.1), its error arrives as `*hamapping.BuildError`, which `internal/bridge/bridge.go:986-1004` already handles | none | — | Nothing to do; covered by the build path |
| G6 | `Add` / `Replace` refuse a parts tree that holds one `StableKey` twice, and a device whose own or part key another endpoint serves, before a withheld error | gf:76, gf:152 | Reaches the host as a call error or a commit error of G2; go-hamapping's part keys are `<key>#n` (go-hamapping `plan.go:465`), unique per entity **[code]** | covered by G2 | — | Nothing beyond G2 |
| G7 | Atomic `Replace` by a declaration the specification level withholds: endpoint taken off the wire (keeps its number), recorded in `Node.Surface`, returns `*surface.WithheldError` (#585, closed) | gf:151 | The host sets no `surface.Policies` (no `Surface` in `internal/`, `cmd/`), so the level is the pin's (`surface/level.go:19-20`) and go-hamapping's rows are all known to it: nothing is withheld today. If it ever were, `settle` would treat the Replace as refused and rebuild every sync | optional (latent) | S | Later: when a level is configured, treat `*surface.WithheldError` as "served as withheld", not as a refusal; no 0.7.0 work |
| G8 | Same-key `Replace` (and Remove+Add in one batch) re-announces: new DataVersions, full report to subscriptions (#589); `endpoint.Spec.Instance` set by the Aggregator | gf:40, gf:170, gf:178 | Every signature change is a `bt.Replace` (`bridge.go:684`): controllers' `DataVersionFilter` no longer hides a rebuilt device's values. Automatic | recommended (comes with the lift) | — | Take with the lift; one changelog line |
| G9 | Camera: `Provider.PeerGone` ends every WebRTC session of a requestor the node proved lost (an exchange the node opened went unanswered, no session left in either stack) through the port's `EndSession`; the node wires it at `Start` (`node/webrtc_provider_peers.go:24-28`, `:77-79`, `:86-91`, call `:200`); #590 closed | gf:42-43, gf:179 (`cluster/camera/provider_gone.go:41`) | Automatic: the node calls `EndSession` on the host (`internal/camera/camera.go:633`), which unsubscribes Home Assistant's session. **Decision 31** said the 1 h idle end (`camera.go:143-146` `DefaultIdleTimeout`, `:307`, `:508`, `:552-558` `expire`) is replaced by this signal once it exists — it now exists. The 1 h end has a user-visible cost: "a view kept open longer than an hour must be opened again" (`docs/operations.md:429-433`). Limit (go-fabric's own doc, `provider_gone.go:36-40`): a controller alive on Matter that abandoned its media is not "gone"; that is the media plane's verdict, the host's. The cap's eviction of the idlest session (`camera.go:444-470`, `EvictIdle` 1 min) covers that case under pressure | **recommended** (decision 31) | S–M (drop the timer and its options, tests in `internal/camera/*_test.go`, docs, changelog; plus one e2e that a lost requestor's session ends) | Do in 0.7.0 as decision 31 says, **or** keep the idle end as a long backstop (question Q2) |
| G10 | Camera: `ProviderConfig.MaxSessions` / `MaxSessionsPerFabric`, refused by the kit before the port | gf:41 (`cluster/camera/provider.go:62-83`) | The host bounds 8 sessions per camera itself with eviction of the idlest (`camera.go:130-139`, `:444-470`); a kit bound would refuse before the host can evict. The camera device is declared by go-hamapping (`ports.go:59-66` at v0.7.1), which exposes no option for the bound | optional | M (go-hamapping option first) | Later; a per-fabric bound (one fabric cannot take all 8) is the interesting part — a go-hamapping issue when wanted |
| G11 | `cluster/core` `Messages.SetLogger` | gf:45 (`cluster/core/messages.go:820`) | The host mounts no Messages server (no `RootServers`; no hit) | none | — | Nothing |
| G12 | `AccessControl.OnAttributeChange`, GroupKeyManagement change notification on fabric removal | gf:46 (`cluster/core/access_control.go:482`) | Node-internal wiring; the host holds no AccessControl | none | — | Nothing (comes with the lift) |
| G13 | `diagevent` `Ring.ForgetFabric`; the bridge now removes a removed fabric's index from the ring itself | gf:36, gf:155 (`diagevent/ring.go:244`) | The host keeps no `diagevent.Ring` (its Operation page is `logring`, `internal/web/operation.go:12-20`) | none | — | Nothing (the fix in gf:155 comes with the lift) |
| G14 | `secure/sigma.Established`, `bridge.CaseAdapter.SetOnEstablished` | gf:47 (`secure/sigma/protocol.go:224`, `bridge/handlers.go:1034`) | Not used; the host stays above `node` | none | — | Nothing |
| G15 | Deprecations: 39 featureless `<cluster>.Feature`, `IsFailSafeArmed`, `ScenesConfig.Fabrics`, `controller/dcl` forwarders, `DefaultSustainRetries`, `CaseSessionEstablished` / `SetOnSessionEstablished` / `SnapshotResponder` | gf:85-90 | staticcheck (SA1019) after the lift: 0 issues **[build]**; no hit by name either | none (owner rule satisfied) | — | Nothing; keep SA1019 in CI |
| G16 | `node.NewAESGCMSealer`, `Reseal`, `ErrUnseal`, `ErrNoSealer` — a facade-only host can seal its store | gf:32 | The store is not sealed (no `Sealer` in `internal/config`) | optional | M | Later, as a feature (a key in the add-on's `/data` buys little; it is a design question, not a release item) |
| G17 | Several servers refuse input they accepted before; `endpoint.Assemble` refuses an endpoint that materialises no cluster | gf:54-76 | The whole test suite but G2's test passes on v0.26.0 **[build]** | — | — | Covered by the suite |

### 1.2 The G2 fix, shape

The batch's commit errors are joined and carry the key only in their text (`node/aggregator.go:1015-1034`,
`:1083` at v0.26.0: `fmt.Errorf("%w: %s", ErrDuplicateKey, key)`, sometimes a part key). Matching op keys
out of error text would be a convenience seam. The structural shape **[inferred]**: after `Batch` returns,
`settle` derives what was applied from the node, not from the call results — for each Add or Replace op,
whether `Aggregator().Endpoints()` holds `op.Endpoint` (the same instance under `op.Key`); for each Forget,
whether it still needs re-queueing (the key is still served or still reserved). The call errors stay for the
log. Test: `TestFailedBatchOperationIsRetried` turns green; add a red test for a held Forget refused at the
commit (G4). An upstream request for a typed commit error (`*node.OpError{Op, Key}` or a per-call result
from `Batch`) would make this direct; not required for 0.7.0.

## 2. go-fabric-ui v0.14.0 → v0.14.1, and v0.13.0 / v0.14.0

| # | Item | Source | Bridge use | Class | Effort | Recommendation |
| --- | --- | --- | --- | --- | --- | --- |
| U1 | v0.14.1: go-fabric v0.26.0 as the minimum; no route, contract or behaviour change (contract stays 0.14.0) | ui:8-18 | `go.mod:10` pins v0.14.0 | mandatory with G1 (MVS: comes with the go-fabric lift) | S | Lift with go-fabric |
| U2 | v0.13.0: the controller family's onboarding inventory (`ControllerSource` +3 methods, `gofabric.ControllerOptions.OnboardingCodes`, `capabilities.onboarding_codes`) | ui:80-97 (Hosts), ui:99-119 | The host serves no controller family (`Options.Controller` unset, `cmd/ha2fabric/main.go:169-176`) | none (not a bridge feature) | — | Nothing |
| U3 | v0.14.0: module move, problem `type` URIs | ui:27-70 | Done in brief M (f7885e6) | done | — | — |
| U4 | `Options` fields the host leaves at their default: `Nodes` (:190; the host uses `Registry`), `ExposureReason` (:207; the host has an `Exposure`), `Controller` (:219), `Prefix` (:265; `NewHandler` takes only `/api/v1`), `ConfiguredWindow` (:281), `AllowEmptyTopology` (:286), `FrameAncestors` (:300), `MaxStreams` (:333, default 64) | `fabricui/options.go` at v0.14.1; `docs/embedding.md` table, lines 36-58 | Unset on purpose **[code]**. `FrameAncestors` matters only if the live test's framing check (HR-P2-frame-02, decision 43) shows a parent other than the Ingress's own origin; the Ingress needs no entry (`docs/embedding.md` "Behind an Ingress", "Framing") | optional | S | Nothing for 0.7.0; revisit `FrameAncestors` only on the live test's result |
| U5 | `docs/provider.md` (ExposureProvider) | v0.14.1 | Unchanged since the host's provider was written; no new optional interface | none | — | Nothing |

Nothing in v0.13.0, v0.14.0 or v0.14.1 is a node-family feature the bridge leaves unwired.

## 3. go-hamapping v0.7.0 → v0.7.1 and open issues touching the host

| # | Item | Source | Bridge use | Class | Effort | Recommendation |
| --- | --- | --- | --- | --- | --- | --- |
| H1 | v0.7.1: go-fabric v0.26.0; events through `Record<Event>` (internal, same events, same endpoint); "no API change here" | hm:9-20 | Nothing for the host to change; tests pass **[build]** | mandatory with G1 (keeps go-hamapping off the deprecated `Emit<Event>`, owner rule) | S | Lift with go-fabric |
| H2 | v0.7.0 "What a host must do": `open_tilt` / `close_tilt` | hm:24-32 | Already served (review phase 2, round 2; changelog 0.7.0 cover lines) | done | — | — |
| H3 | #43 `stop_tilt` for a cover with `STOP_TILT` but no `STOP` | go-hamapping issue #43 | Would need a host method in `internal/adapt/command.go:42-44` (`cover.stop_cover_tilt`) once go-hamapping adds it; decided after the owner's live controller tests (decision 49) | later | S (host part) | After the live tests |
| H4 | #44 window covers as Rollershade, `PositionAwareLift` always | #44 | Mapping only; no host change | later | — | After the live tests (decision 49) |
| H5 | #45 climate shapes mappable but refused | #45 | Mapping only; the host already turns such a refusal into an unmappable entity via `BuildError` (`bridge.go:986-1004`) | later | — | go-hamapping's |
| H6 | #46 `BuildError` names the entity whose declaration was refused | #46 | Would let `bridge.go:987-991` drop only that entity instead of the primary and all riders | later (recommended once released) | S | Adopt when go-hamapping releases it |
| H7 | Older open issues with a host side: #39 range command (retires the 30 s stop-gap of decision 32), #40 stable-key helper, #41 primary endpoint and input order | #39-#41 | Host follow-ups when released | later | S each | Not for 0.7.0 |

## 4. Open go-ha2fabric issues

| # | Issue | Doable now? | Class | Effort | Recommendation |
| --- | --- | --- | --- | --- | --- |
| #71 | `--health` for `HEALTHCHECK` and the add-on watchdog | Technically yes, but the design has a catch **[code]**: in the add-on the web listener filters every client but `172.30.32.2` (`internal/web/web.go:95-107`, `script/run.sh:77-78`), so a loopback probe gets 403; stand-alone the same holds whenever `WEB_ALLOW` is set, and `WEB_LISTEN=""` has no listener at all. A clean design needs a loopback-only health endpoint (or a fixed answer the filter exempts), which is a security-surface decision. The add-on can use the Supervisor's `watchdog: tcp://[HOST]:[PORT:8486]` without `--health` | optional | M | Not in 0.7.0; a 0.7.x wave with its own design note |
| #72 | Unprivileged add-on, AppArmor profile, watchdog | No: decision 34 and the issue say after the live Ingress test, and the `/data` ownership change must be tested on a real Supervisor | later | M | After the live test, next release |
| #73 | Persisted served keys | No: a design change against "a sync reads everything" (`CLAUDE.md`, Architecture), the owner's decision (decision 45) | later | L | Not for 0.7.0 |

## 5. Release blockers already recorded

| Item | Recorded | Before the tag? |
| --- | --- | --- |
| The owner's live Ingress test | Decisions 8, 12, 22, 24, 43 | **Yes** — decision 8: "a tag only on the owner's release go, together with the owed live Ingress test". Its list (decision 43 and 22): the framing origin (HR-P2-frame-02), the source address the daemon sees and the Supervisor's gateway (for cfg-27), HAOS's `rp_filter` (HR-P2-net-01, `docs/operations.md:136-137`), the held files' items; also the camera proxy URL derivation in the add-on (`docs/operations.md:405-412`) and the Supervisor admin gate (`X-Remote-User-Groups`, Supervisor 2026.10.4, `cmd/ha2fabric/supervisor.go:34`) |
| HR-P0-cfg-27, binding the add-on's listener tighter | Decision 22 | Not by decision 22 (a follow-up issue after the live test); decisions 24 and 37 bear on its timing → **question Q4** |
| HR-P1-auth-01 | Decisions 21, 24, 37 | Decisions 21, 24, 37 |
| The go-fabric v0.26.0 lift with G2's fix | This note | **Yes** if 0.7.0 is to carry G1 (the CASE/TCP fix reaches a CAMERA daemon) |
| How the live test gets a build | **[inferred]** | The image workflows publish only on a tag or a dispatch; a dispatch with tag input `0.7.0` passes the version check and pushes `:0.7.0` **and `:latest`** (`.github/workflows/addon-image.yml:62-88`, tags step after `:104`), i.e. publishes before the tag; a dispatch without input pushes a `git describe` tag and `:latest`, which the manifest's `version: "0.7.0"` does not name. So the live test before the tag runs as a **local add-on** (the repository copied into the HAOS `/addons` share, built by the Supervisor from `addon/Dockerfile` without the `image:` key) → **question Q5** |

## 6. Release mechanics

- `make release` (`Makefile:119-134`) runs `version-check` (`Makefile:113-118`: `internal/version/version.go:20` `0.7.0`,
  `addon/config.yaml:7` `"0.7.0"`, a `changelog.md` section) and stages the archives and `RELEASE_NOTES.md`
  (`script/extract-release-notes.sh`). The three agree today.
- `release-on-tag.yml` (on `v*`, owner `thingfabric` only, `:11-31`) creates the GitHub release with the
  archives and the changelog section; `docker-build-push.yml` pushes `ghcr.io/thingfabric/go-ha2fabric:{0.7.0,latest}`
  (`:28`, `:83-85`); `addon-image.yml` pushes `ghcr.io/thingfabric/go-ha2fabric-addon-{amd64,aarch64,armv7}`
  (`:33`) after checking the tag against `addon/config.yaml` and `internal/version` (`:80-88`).
- **First images under `ghcr.io/thingfabric`.** The org has one container package today (`go-fabric-chip`, public;
  `gh api /orgs/thingfabric/packages`). `go-ha2fabric` and the three `go-ha2fabric-addon-*` packages do not
  exist yet: the first push creates them **private** by default, and the Supervisor cannot pull a private
  image anonymously. After the tag's workflows: set the four packages public and linked to the repository
  (or create them public beforehand), then check `docker pull` anonymously. The org's Actions package
  permission must allow `GITHUB_TOKEN` to create packages (the workflows ask `packages: write`).
- Before the tag: the changelog's `## [0.7.0] - 2026-11-23` date to the tag's date; the go-fabric v0.26.0 /
  go-fabric-ui v0.14.1 / go-hamapping v0.7.1 lines (and G2, G8, G9) in `changelog.md` and `addon/CHANGELOG.md`
  ("Updated libraries" line); `CLAUDE.md`'s state line; `docs/operations.md:429-433` if G9 drops the 1 h end.
- Old installations: the add-on repository URL changed (already in `addon/CHANGELOG.md` 0.7.0); no `0.7.0`
  under `ghcr.io/sukramj/` (changelog 0.7.0, Changed).
- After the tag: HR-P1-auth-01 per decisions 21 and 37; the cfg-27
  follow-up issue with the live test's measured addresses (decision 22); #72 opens for work.

## 7. Proposed release scope

**Must (before the tag)**
1. One wave: go-fabric v0.26.0 + go-fabric-ui v0.14.1 + go-hamapping v0.7.1 (G1, U1, H1), with the G2/G4 fix
   in `internal/bridge` (`settle` from the node's end state; red tests), changelog lines. M.
2. The owner's live Ingress test on that build (as a local add-on), with decision 43's list.
3. Release mechanics: changelog date, package visibility of the four new ghcr packages.

**Should (in the same wave, cheap)**
4. G9 per decision 31: drop the 1 h idle end now that the node ends a lost requestor's sessions; one e2e
   test that a lost requestor's session reaches `EndSession` and unsubscribes Home Assistant; docs. S–M.
5. G8: a changelog line (controllers see a rebuilt device's values); no code.

**Later (0.7.x / 0.8.0)**
- cfg-27 bind (after the live test, decision 22) — unless Q4 puts it before the tag.
- #72 unprivileged add-on with watchdog (after the live test); #71 `--health` with its own design (listener
  filter); #73 persisted served keys (design).
- go-hamapping #46 (`BuildError` owner), #43/#44 after live controller tests, #39-#41.
- G10 per-fabric camera bound (needs a go-hamapping option), G16 sealed store, G7 withheld handling once a
  specification level is configurable; an upstream issue for a typed batch commit error (G2's direct form).

## 8. Questions for the owner (recommendation first)

- **Q1 — Lift go-fabric v0.26.0 (with go-fabric-ui v0.14.1, go-hamapping v0.7.1) into 0.7.0?**
  Recommended: **yes**, in one wave before the tag, with the G2 fix. Reasons: the CASE fix (gf:21) reaches a
  daemon with `CAMERA` (TCP on); the camera end of decision 31 exists only from v0.26.0; and shipping 0.7.0 on
  v0.25.0 would make the next release's lift carry the batch change on its own. Alternative: tag 0.7.0 on
  v0.25.0 now and lift in 0.7.1 (no code risk before the live test, but 0.7.0 ships without the CASE fix).
- **Q2 — The camera's 1 h idle end once go-fabric ends a lost controller's sessions?**
  Recommended: **drop it, as decision 31 said** — it cuts live views longer than an hour
  (`docs/operations.md:432-433`), and the cap's eviction of the idlest session still covers a controller that
  stays on Matter but abandoned its view. Alternative: keep it as a backstop with a longer bound (e.g. 12 h).
- **Q3 — G2's fix shape: derive what the batch applied from the node's set after `Batch` (host-only), plus an
  upstream issue for a typed commit error?** Recommended: **yes, both**; never match keys in error text.
- **Q4 — cfg-27 (the tighter add-on bind) before the tag or after?** Decision 22 puts it after the live test
  as a follow-up issue; decisions 24 and 37 bear on it (HR-P1-auth-01). Recommended: **after the tag as
  decided in 22**, with decision 37 governing; confirm that 37 supersedes 24.
- **Q5 — How the live test gets its build?** Recommended: **a local add-on** built by the Supervisor from the
  release candidate commit (no image published before the tag); not a `workflow_dispatch`, which would push
  `:0.7.0` and `:latest` before the tag.
- **Q6 — #71 `--health` in 0.7.0?** Recommended: **no**; it needs a design for the client filter (a loopback
  probe is refused in the add-on), and the add-on's watchdog can be a plain `tcp://` check with #72 after the
  live test.
