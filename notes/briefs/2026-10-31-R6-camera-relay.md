# Brief R6 (go-ha2fabric) — the camera relay (0.5.0)

Owner decisions (2026-10-31, evening): build R6 now; one pull request on
`roadmap/r6-camera`, squash-merged, auto-merge; release 0.5.0 at the end
(the `[Unreleased]` entries of the go-fabric bump and the side-panel title
go into it). The camera is **opt-in, default off**: go-fabric's ecosystem
gate (`docs/guides/camera.md:414-425` @v0.15.5: no host device carries a
camera cluster before a live test with Apple Home has passed) is the
owner's rule, and this daemon follows it by keeping the camera behind an
option until that test is recorded.

Sources (maps with `file:line`; verify a load-bearing line before it
enters code, a comment or a doc): `notes/reads/read-r6-gofabric-camera-kit.md`,
`read-r6-ha-webrtc.md`, `read-r6-camera.md` (§Q5 the WebSocket shapes),
`read-r6-ha2fabric-relay.md`, `../go-hamapping/notes/reads/read-r6-ports-model.md`.
go-fabric `v0.15.5` is checked out in `../go-fabric` (`cluster/camera/
provider_peer.go`, `provider.go`, `avstream.go`; `cluster/spec/
webrtctransportprovider/server_gen.go`, `cluster/spec/cameraavstreammanagement/
server_gen.go`, `definition_gen.go`; `node/webrtc_provider_peers.go` and its
test; `bridge/tcp_listen.go:25-30`; `docs/guides/camera.md`). Home Assistant
facts only via `git -C ../core show 2026.10.3:<path>`; never run it.
go-hamapping's side is the brief `../go-hamapping/notes/briefs/
2026-10-31-R6-camera-rows.md` (read §2 and §3: `Options.Ports func(model.Entity) any`,
the five port interfaces, the camera row's features VDO, ADO, SNP).

Facts verified by the owner:

- The camera's signals are `camera.Provider.Answer/IceCandidates/End`
  (`cluster/camera/provider_peer.go:78-120`); each is one Invoke on the
  session's peer over a CASE session the node initiates over UDP or reuses
  (`:124-150`). A payload over 1178 bytes on a UDP session is
  `bridge.ErrPayloadTooLarge` (`bridge/ota_provider_link.go:48-65, 155`); an
  SDP is usually larger; the controller's TCP session carries it when the
  node accepts TCP (`node.TCPConfig{Incoming: true}`, `bridge/tcp_listen.go:
  25-30`; go-fabric `docs/guides/operations.md:54`). The daemon builds its
  node config in `internal/bridge/node.go:36-37` with no TCP today.
- The node attaches the signalling route to every `*camera.Provider` the
  kit assembled, at Start and after each aggregator assembly
  (`node/webrtc_provider_peers.go:43-56`, `node/aggregator.go:592`). The
  kit stores the session after the port's `ProvideOffer` returned
  (`cluster/camera/provider.go:516-520` per the earlier map); `Answer`
  before that is `camera.ErrUnknownSession`.
- `Provider.End` removes the session whatever the peer answered; the kit's
  `EndSession` command removes it too and calls the port (`provider_peer.go:
  113-120`; `docs/guides/camera.md` transport model).
- Home Assistant: `camera/webrtc/offer {entity_id, offer}` answers a
  `result` first, then pushes `event` messages on the same id: `session`
  (`session_id`), `answer` (`answer`: the SDP), `candidate` (`candidate`:
  `{candidate, sdpMid, sdpMLineIndex}` — webrtc-models 0.3.0
  `webrtc_models/__init__.py:76-85`), `error` (`code`, `message`)
  (`camera/webrtc.py:40-94, 252-303`). `camera/webrtc/candidate {entity_id,
  session_id, candidate}` forwards a controller candidate (`:333-348`).
  `unsubscribe_events {subscription: <offer id>}` closes the session
  (`websocket_api/commands.py:259-263`, `camera/webrtc.py:276-278`,
  `camera/__init__.py:730-733`). Envelopes: `websocket_api/messages.py:65,
  100-104, 109`.
- A still: `GET /api/camera_proxy/<entity_id>?width=&height=` with
  `Authorization: Bearer <token>` (`http/auth.py:226-230, 250`;
  `camera/__init__.py:776, 789-792`): 401 with a wrong header, 403 without,
  503 when the camera is off (`:796-808`); the image is scaled down only
  (`util/img_util.py:23-24, 43-44`). The token is the daemon's long-lived
  token or `SUPERVISOR_TOKEN` (`internal/config/config.go:64-66`); in the
  add-on the HTTP base is the Supervisor proxy of the core API
  (`http://supervisor/core/api/...` when the WebSocket URL is
  `ws://supervisor/core/websocket`): derive the HTTP base from the WebSocket
  URL (`ws`→`http`, `wss`→`https`, `/websocket` → `/api/camera_proxy/...`),
  and name this derivation in `docs/operations.md` as the one thing the live
  test must confirm.
- The daemon's client has no per-id event channel and no HTTP side
  (`internal/ha/client.go:303-315, 51-58`); `hatest` answers unknown
  commands with `unknown_command` (`hatest.go:279`). The token is never
  logged (`client_test.go:189-190` asserts it); keep that assertion true for
  every new log line and error string.
- The import guard (`internal/arch/arch_test.go`) allows go-fabric `node`,
  `device`, `contract`, `eligibility` and one-level `cluster/spec/<x>`.

## §1 Config and node

- `internal/config`: `Camera bool`, env `CAMERA` (`true`/`false`, default
  false), add-on option `camera` (bool, default `false`) in
  `addon/config.yaml` options and schema, mapped like the other booleans in
  the add-on's run script. Doc comment: the ecosystem gate, one sentence.
- `internal/bridge/node.go`: when `Camera` is on, `Network.TCP =
  node.TCPConfig{Incoming: true}` (the SDP fact above). A unit test on the
  config builder for both values.
- When `Camera` is off, camera entities are not mapped: skip the `camera`
  domain before `hamapping.Map` (`bridge.go:940`) with one Info line per
  start (`bridge.camera_disabled`, count of skipped entities), not per sync.

## §2 `internal/ha`: a subscription and a still

- `Subscribe(ctx, msgType string, payload map[string]any) (*Subscription, error)`:
  sends the command with `subscribe` semantics (the pending map), returns
  after the `result`; `Subscription.Events() <-chan json.RawMessage` yields
  each `event` of that id (the inner `event` object); `Subscription.ID()`;
  `Subscription.Unsubscribe(ctx) error` sends `unsubscribe_events
  {subscription: id}` and closes the channel. A reconnect closes every open
  subscription (channel closed, `Err()` reports it); the camera relay ends
  its sessions on that. Keep `Options.Events` re-subscription as it is.
- `Image(ctx, entityID string, width, height int) (data []byte, contentType string, err error)`:
  the HTTP GET above with the Bearer header, `width`/`height` only when > 0;
  status 401/403/503 become typed errors (`ErrUnauthorized`,
  `ErrCameraOff`), the body limit is go-fabric's snapshot bound
  (`camera.DefaultMaxSnapshotSize`; take the number from
  `cluster/camera/avstream.go:~75` and cite it) plus a margin — read with
  `io.LimitReader`. The HTTP client is built once with the WebSocket
  options' TLS settings, if any. The token appears in the request header
  only; no log line and no error string carries it (extend the existing
  log-assertion test).
- Tests in `internal/ha` with the existing fake server pattern: subscribe
  → result → three events → unsubscribe; reconnect closes; `Image` happy
  path, 403, 503, over-size.

## §3 `internal/camera`: the host per entity

A new package; it alone may import `cluster/camera`, plus
`cluster/spec/webrtctransportprovider` and `cluster/spec/
cameraavstreammanagement` (extend `internal/arch/arch_test.go` with that
rule and the reason: the signals are methods of `*camera.Provider`,
`docs/guides/camera.md:93-96`). It implements, for one entity:

- `wpdef.ProviderPort`:
  - `ProvideOffer(ctx, args)`: `Subscribe("camera/webrtc/offer",
    {entity_id, offer: args.Sdp})`; store the session (Matter id
    `args.SessionID`, peer node, fabric, `PeerEndpointId =
    args.OriginatingEndpointID`, usage, the stream ids: the first of
    `args.VideoStreams`/`AudioStreams` when given, else the host's nominal
    stream ids); start the reader goroutine; return the
    `WebRTCSessionStruct`. The reader: `session` → remember HA's
    `session_id`; `answer` → `signaller.Answer(ctx, id, sdp)`; `candidate`
    → `signaller.IceCandidates(ctx, id, []{Candidate, SdpMid (null when
    empty), SdpmLineIndex})` one per event (no batching: go2rtc trickles
    them, `go2rtc/__init__.py:343-354`); `error` → log at Warn with code
    and message, `signaller.End(ctx, id, reason)` and unsubscribe. The
    first `Answer` may hit `camera.ErrUnknownSession` because the kit
    stores the session after the port returned: retry on that error only,
    up to ten times at 50 ms, then log and end the session. Every other
    signal error is logged once (`camera.signal_failed` with the command
    and the error; `bridge.ErrPayloadTooLarge` gets its own line telling
    the operator the controller came in over UDP) and ends the session.
  - `SolicitOffer`: not served — Home Assistant's API takes the client's
    offer only (`camera/webrtc.py:254-256`); return an error; the kit
    answers FAILURE (`ProviderPort` doc: "An error answers its status,
    FAILURE without one"). If a facade-level way to carry a status exists
    (grep `MatterStatusCode` in `contract`, `device`, `cluster/spec`), use
    INVALID_IN_STATE; otherwise FAILURE, and say which in the report.
  - `ProvideAnswer`: never expected (no solicited offer); error.
  - `ProvideIceCandidates(ctx, sessionID, candidates)`: one
    `camera/webrtc/candidate` per candidate with HA's `session_id`; a
    candidate before the `session` event arrived is queued until it does.
  - `EndSession(ctx, sessionID, reason)`: unsubscribe (HA closes the
    session), drop the local state.
- The signaller is a small interface (`Answer`, `IceCandidates`, `End`) that
  `*camera.Provider` satisfies; `Attach(p *camera.Provider)` sets it (the
  bridge calls it after `Build`, §4). Signals before `Attach` wait for it
  (a ready channel) up to the same bound.
- `csdef.VideoStreamPort`, `AudioStreamPort`, `SnapshotStreamPort`: one
  nominal stream each (ids 1), allocate returns that id (reuse), modify and
  deallocate are no-ops that validate the id; `VideoStreamAllocated`
  no-op. Document in the package comment that the host does not touch the
  media: go2rtc produces the stream, the daemon relays signalling only
  (`docs/roadmap.md` §R6 finding).
- `csdef.SnapshotCapturePort.CaptureSnapshot(ctx, streamID, resolution)`:
  `Image(entity, resolution.Width, resolution.Height)`; the real size from
  `image.DecodeConfig` (stdlib `image/jpeg`); `ImageCodec` JPEG (the
  constant from `definition_gen.go`); a non-JPEG content type is an error.
- `Close()` ends every session with `End(reason)` — the reason from
  `wpdef`/`wrdef`'s `WebRTCEndReasonEnum` that fits (read the enum in
  `definition_gen.go`, cite the value you pick for "the camera went away"
  and for "HA reported an error").
- Unit tests with a fake HA (interface `Upstream` in the package:
  `Subscribe`, `Candidate`, `Image`) and a fake signaller: offer → answer
  → two candidates; candidate from the controller before the `session`
  event; HA `error` event ends the session; `EndSession` unsubscribes;
  `Answer` retry on `ErrUnknownSession`; `ErrPayloadTooLarge` logged and
  ends; snapshot happy path and 503; `Close`. Floor for the package: 90.

## §4 `internal/bridge` wiring

- `hamapping.Options.Ports`: for a `camera` entity when `Camera` is on,
  return the entity's `*camera.Host` (one per entity id, kept in a map;
  created lazily, closed on `Remove`/`Forget` and on `Replace` when the
  entity's row changes). nil for everything else.
- After `plan.Build` (`bridge.go:945`): for each built device of a camera
  entity, `dev.Server(wpdef.ClusterID).(*camera.Provider)` (the existing
  caller: `node/webrtc_provider_peers.go:43-56` and
  `device/camera_transport_declaration_test.go:130-137`) → `host.Attach(p)`.
  A camera device without a `*camera.Provider` is a bug: log at Error with
  the entity and skip it.
- `internal/adapt`: `supported_features` already reaches
  `Description.Extra` (`adapt.go:176-179`); confirm a camera needs no
  domain case and say so in the report. If the row's `when` needs the
  feature under another key, align with go-hamapping's test fixture.

## §5 Tests end to end

- Fixture `testdata/ha/camera.front_door.json` (or the fixture format in
  use): state `idle`, `supported_features: 2`, the attributes a camera state
  carries at 2026.10.3 (`camera/__init__.py` `state_attributes` — cite the
  lines you copy: `access_token`, `entity_picture`, `frontend_stream_type`
  if present). A second camera `camera.doorbell_still` with
  `supported_features: 0` stays unmapped.
- `hatest`: `camera/webrtc/offer` → `result`, then scripted events
  `session`, `answer`, one `candidate` (the test sets the script);
  `camera/webrtc/candidate` → `result` and records the candidate;
  `unsubscribe_events` → `result` and records the id; an HTTP handler
  `/api/camera_proxy/{entity}` that requires `Authorization: Bearer <the
  fixture token>` and serves a small JPEG (generate it in the test with
  `image/jpeg`), 403 otherwise. The fake's HTTP base is what the client
  derives from its WebSocket URL.
- `internal/bridge/bridge_r6_e2e_test.go`, the harness of
  `bridge_r5_e2e_test.go` (`fabrictest.Invoke`, `client.CommandAt`,
  `fabrictest.ReadCluster`): with `Camera` on, the camera endpoint has
  device type 0x0142; `ProvideOffer` (a short SDP) answers a session id; the
  fake received the offer SDP; the Answer and the candidate reach the
  controller's requestor — model the requestor side on go-fabric's own test
  `node/webrtc_provider_peers_test.go:83-172` (read it fully; it is the
  specification for how the controller-side requestor receives the signals
  in `fabrictest`). If that cannot be set up in this harness, assert the
  HA-side effects and the session bookkeeping end to end, assert the
  signals at unit level (§3), and report the difference. Then
  `ProvideIceCandidates` → the fake records the candidate with HA's
  session id; `EndSession` → the fake records the unsubscribe;
  `CaptureSnapshot` → JPEG bytes with the decoded resolution. With `Camera`
  off (the default): no camera endpoint; the unmapped camera never appears.
- `internal/config` tests for `CAMERA`; the node-config test (§1).

## §6 Docs, version, release

- `docs/operations.md`: section "Cameras (opt-in)" — what the relay does
  and does not do (signalling only, go2rtc or a native WebRTC camera
  produces the stream; stills via the camera proxy), the option, why it is
  off by default (the ecosystem gate, cited), the TCP note (a controller
  over UDP cannot receive the answer: the log line to look for), the
  Supervisor proxy derivation as the live-test item, and the limits (no
  SolicitOffer, no ICE-server relay, no audio/video parameters from HA).
- `docs/by_design.md`: paragraphs for SolicitOffer not served, the nominal
  stream (points at go-hamapping's entry), ICE servers not relayed (the
  controller's `IceServers` in `OfferArgs` are the camera's to use; HA's
  provider uses its own, `camera/__init__.py:708-718`), the opt-in gate.
  Each with citations and one sentence of reasoning.
- `docs/roadmap.md` §R6: tick go-fabric (released in v0.15.5), go-hamapping
  (v0.4.0, `ports: host`), go-ha2fabric; mark go-hamqtt "not needed:
  `Description.Extra` carries `supported_features`"; list left open:
  SnapshotCamera for cameras without STREAM, SolicitOffer, ICE servers,
  the Apple Home live test as the condition to flip the default.
- `README.md` option table (`CAMERA`), `addon/DOCS.md` option table,
  `addon/CHANGELOG.md` (`## Unreleased` → `## 0.5.0` with the camera line),
  `changelog.md` `## [0.5.0] - 2026-10-31` (Added: the camera relay and the
  option; Changed: the two `[Unreleased]` items), `internal/version` 0.5.0,
  `addon/config.yaml` version 0.5.0, README and CLAUDE.md state lines
  ("0.5.0 … brief R6"), `THIRD-PARTY-NOTICES.md` go-hamapping pin → `v0.4.0`,
  `script/coverfloor/floors.txt` + `internal/camera 90`.
- Dependency: go-hamapping `v0.4.0`. Until the owner messages you that the
  tag exists, develop against `replace github.com/SukramJ/go-hamapping =>
  ../go-hamapping` (that checkout is on `roadmap/r6-camera`) and never
  commit the replace; the final commits pin `v0.4.0` after `go get`.
- Commit the brief first (`notes/briefs/`), then small commits per section
  on `roadmap/r6-camera`. Guards in the background with a summary log:
  `make ci`, `make race`, `make cover-check`, `make secrets`, `make release`
  (the dry run; the version files move), `make docker` only if it works on
  this Mac (`addon-image` does not; say so). Read a log only on failure, by
  grep. Push and open the pull request (`gh pr create --body-file`, no
  attribution lines), `gh pr merge --auto --squash`.

## §7 Report (hand back, fixed shape)

Per section the commits; guards with HEAD; differences from the brief;
decisions; left out; questions; `by_design` candidates; and the two facts
the owner must confirm in a live test (the Supervisor proxy path; the
controller's TCP session).
