# Phase 1, unit S — held rows

Untracked (`held/` is in `.git/info/exclude`). Baseline `5eefc0a`. Marks and
prefixes as in `notes/review/2026-11-23/phase-1-S.md`: **M** measured (the
line was read, or the probe named ran), **I** inferred. `sup:` is Supervisor
at `fe91b8db` (`gh api`, raw), `gf:` go-fabric v0.25.0, `core:` Home
Assistant core 2026.10.3, `ws:` coder/websocket v1.8.15 (module cache).
Probes ran only in the reviewer's `git archive` copy
(`…/scratchpad/S/src`); nothing was run against a live system.

Five rows: one medium, four low. The tracked report names them by ID and
severity only.

| ID | Sev. | Subject |
| --- | --- | --- |
| HR-P1-auth-01 | medium (I) | a co-installed add-on that takes the Ingress proxy's address is the administrator; the groups header is no factor against it |
| HR-P1-bound-02 | low | no bound on the sessions of one camera |
| HR-P1-conc-03 | low | a session whose controller is gone never ends |
| HR-P1-ha-04 | low | an offer whose wait ends early leaves Home Assistant's session, counted nowhere |
| HR-P1-bound-17 | low | stills and candidates: one Home Assistant request per controller request, unbounded |

---

## HR-P1-auth-01 — medium (I) — the add-on's administrator against a co-installed add-on

Role R3 (a peer that claims `172.30.32.2`), threat T3, decision 19
("a co-installed add-on is not an administrator; the peer address alone is
no credential").

**Claim.** The add-on's admin check is three tests: the peer address is
`172.30.32.2`, the request carries `X-Remote-User-Groups` with
`system-admin`, and the Supervisor is 2026.10.4 or later
(`cmd/ha2fabric/main.go:247-260`, `internal/web/web.go:141-148`,
`:165-180`, `cmd/ha2fabric/supervisor.go:90`). The second and third are
facts about the Supervisor's forwarding; they hold only for a request the
Supervisor really sent. A peer that owns the address `172.30.32.2` on the
Supervisor's Docker network writes the header itself. So against the role
decision 19 names, the address is still the only credential.

**Lines.**

| Step | Line | M/I |
| --- | --- | --- |
| The check reads `r.RemoteAddr` and one request header, nothing else | `internal/web/web.go:141-148`, `:165-180` | M |
| The listener is on every interface of the host, the Supervisor's bridge included | `script/run.sh:77`, `addon/config.yaml:37` | M |
| A peer with another address of that network is refused before go-fabric-ui, with the header set or not | `internal/web/web.go:91-104`; probe S8: `add-on / co-installed add-on 172.30.33.5 with forged groups: 29 routes -> map[403:29]` | M |
| The Supervisor leaves `NET_RAW` in an add-on's capability set unless a feature flag is on, and says why | `sup:supervisor/docker/app.py:402-407`: "NET_RAW allows ARP and MAC spoofing on the shared internal network", dropped only with `FeatureFlag.APP_DROP_NET_RAW` (default `False`) | M |
| With `NET_RAW` a container on the bridge can answer ARP for `172.30.32.2` towards the host, complete a TCP handshake to `172.30.32.1:8486` from that address and send any header | Linux bridge and ARP behaviour, not read at a pin, not run | I |
| Nothing the Supervisor forwards is a secret the add-on could verify: the headers are plain values; the Ingress session cookie passes through (it is not in the filter, `sup:supervisor/api/ingress.py:47-64`), but the call that validates a session is Core's only (`:164-173`, `require_home_assistant`) | — | M |

**Consequence.** Every admin route: the pairing codes (a fabric of the
attacker's own, and with it `call_service` on the exposed entities), the
selection, erase, the node registry, the log. The attacker is an add-on
the operator installed, with default capabilities and no API role of its
own.

**Why medium and not high.** It needs a hostile add-on on the same host;
the path was not run; an add-on with `homeassistant_api` or `hassio_api`
needs none of this. **Why not low:** decision 19 names exactly this role,
and phase 0's fix (the groups header, the version gate) reads as a second
factor in the code comments and the docs, which it is against a Home
Assistant user and is not against this role.

**What the daemon can and cannot do.**

1. It cannot authenticate the Supervisor with what Ingress offers today
   (table, last row). This is a property of Ingress for every add-on on
   the host's network.
2. **Bind the listener to the bridge's gateway address** (HR-P0-cfg-27,
   decision 22: after the live test). It takes the LAN out of the
   picture; it does not stop a container on the bridge.
3. **Leave the host's network for the web listener.** Not available: the
   Matter node needs the host's network, and one container has one
   network mode.
4. **A Supervisor feature**: a per-add-on secret on forwarded requests
   (the add-on already holds `SUPERVISOR_TOKEN`; an HMAC over method,
   path and a timestamp with a key derived from it would do), or
   `APP_DROP_NET_RAW` on by default. An upstream proposal, outside this
   repository.
5. **Say it**: `docs/operations.md` ("The web UI") and `addon/DOCS.md`
   state that the add-on trusts the Supervisor's network, and that
   another add-on with raw-socket rights on it can act as the proxy.

**Recommendation.** 5 now (wording per decision 21: the check as it is),
2 with decision 22's follow-up, 4 as an issue at the Supervisor if the
owner's live test or X1 measures the path. The row stays held until the
docs sentence is agreed, because the sentence is the publication.

**Regression test.** None in this repository can be red: the daemon's
code is correct for the input it can see. X1 (phase 2) owes the
measurement: two containers on a user-defined bridge, one with `NET_RAW`,
the ARP answer, the request with the header, the daemon's answer.

---

## HR-P1-bound-02 — low — no bound on the sessions of one camera

The session half of HR-P0-bound-28, as its own row.

**Claim.** A camera's host keeps its sessions in a map keyed by the
16-bit session id and accepts every `ProvideOffer`
(`internal/camera/camera.go:183`, `:328-367`). go-fabric's kit bounds
nothing but the id space: `generateSessionID` answers
RESOURCE_EXHAUSTED only when all 65 535 ids are in use
(`gf:cluster/camera/provider.go:210-224`), and the session table is a
slice without a cap (`:97`, `:158-168`). Each session is one
`camera/webrtc/offer` subscription in Home Assistant — a session of the
WebRTC provider, go2rtc's WebSocket client or an integration's own
(`core:homeassistant/components/camera/webrtc.py:275-295`,
`core:homeassistant/components/go2rtc/__init__.py:320-356`) — one relay
goroutine and one buffered subscription here (`camera.go:355`,
`internal/ha/subscription.go:121`).

**Measured.** Probe `internal/camera/probe_s_test.go`
`TestProbeSSessionsPerCamera`:
`PROBE S1: offers accepted=5000, Home Assistant subscriptions opened=5000, still open=5000, goroutines +5000`.

**Who.** R4, a commissioned controller with Operate on the camera's
endpoint: 65 535 sessions per camera and node. The cost on Home
Assistant's side per session (a go2rtc consumer, a peer connection) is
the larger one and was not measured (I).

**Reference.** The kit mirrors chip's server, which leaves the number
of sessions to the delegate — here the host (`gf:provider.go:495-501`,
the port call at `:557`). What chip's camera app does at its limit was
not read (I). A real camera serves a handful of viewers.

**Fix.** `MaxSessions` per host (proposal: 8, a constant beside
`MaxPendingCandidates`, with the same kind of comment: a controller
ecosystem opens one live view per viewer; eight viewers of one camera is
beyond any household). A new offer beyond it is refused with
`contract.StatusError{Status: contract.StatusResourceExhausted}` before
the subscription is opened; a re-offer of a held session does not count.
A facade question for go-fabric goes with it (an `upstream` candidate,
theme `adpt`): the kit could take `ProviderConfig.MaxSessions` and refuse
before the port; until then the host's bound is the bound.

**Regression test (red today).** `TestSessionsPerCameraAreBounded`: open
`MaxSessions` offers, the next answers an error that carries
RESOURCE_EXHAUSTED and `upstream.streams` did not grow; after
`EndSession` of one, an offer is accepted again; a re-offer at the bound
is accepted.

---

## HR-P1-conc-03 — low — a session whose controller is gone never ends

**Claim.** A session leaves the host on exactly these paths
(`internal/camera/camera.go`): the controller's `EndSession` (`:420-430`),
Home Assistant's error event (`:534-537`), a signal that failed
(`:588-595`), the subscription's end with an error — the connection
dropped or the reader fell behind (`:495-498`) —, a re-offer (`:331-339`),
a rebuilt device (`:244-264`), `Close` (`:285-311`), and the fabric's
removal through the kit (`gf:cluster/camera/fabric_state.go:48-68`).
None of them is "the controller is no longer there". Home Assistant
sends nothing more once the answer and the candidates went out, so no
signal fails; the kit has no idle or liveness rule
(`gf:cluster/camera/provider.go`: the session table changes only in
`upsertSession`, `removeSession` and `MatterDropFabric`). A controller
that loses power or the network, or just does not send `EndSession`,
leaves the session in CurrentSessions, the relay goroutine parked on the
subscription's channel (`camera.go:483`) and Home Assistant's session
open until the Home Assistant connection drops or the daemon restarts
(M: the absence of another path; that Home Assistant itself keeps such a
session is I — go2rtc's own timeout was not read).

**Why its own row.** HR-P1-bound-02's cap bounds the number; without an
end, a cap of eight is used up by eight orphans and the camera then
refuses its owner. The two fixes belong together.

**Fix.** The best design ends a session on the evidence the node has:
(a) the peer's CASE session and subscription state — a facade question
to go-fabric (an `upstream` candidate, `adpt`): the kit or the node could
tell the host when no session with the peer node is live any more;
(b) here, meanwhile: an
idle bound — a session that has had no controller command and no Home
Assistant event for N minutes (proposal: 10, far above ICE's
keep-alive-free signalling phase) is ended with `ReasonUnknown` through
`end`, which tells the controller and closes Home Assistant's session;
and at the cap the oldest idle session is evicted rather than the new
offer refused. (b) is cheap and does not wait for (a).

**Regression test (red today).** With a fake clock: an offer, the answer,
no further traffic; after the idle bound `signaller.seen()` holds `End`
with reason 12 and the stream's `unsubscribed()` is 1.

---

## HR-P1-ha-04 — low — an offer whose wait ends early leaves Home Assistant's session

**Claim.** `Client.Subscribe` sends the command and waits for its result
under the caller's context (`internal/ha/subscription.go:104-127`). When
the context ends first, `callWith` returns the context's error
(`internal/ha/client.go:452-457`) and `Subscribe` drops the local stream
(`subscription.go:122-125`) — and sends nothing. Home Assistant registers
the subscription's teardown when it takes the command, then sends the
result (`core:homeassistant/components/camera/webrtc.py:275-280`), and
goes on to open the provider's session (`:295`). Its events then arrive
for an id the client no longer knows and are dropped
(`client.go:336-350`). Nobody unsubscribes: the session lives until the
connection drops.

**Measured.** Probe `internal/ha/probe_s_test.go`
`TestProbeSSubscribeTimeout`:
`PROBE S7: WebRTCOffer err=ha: camera/webrtc/offer: context deadline exceeded; connected still=true; commands Home Assistant saw=[camera/webrtc/offer]`
— no `unsubscribe_events` follows, the connection stays up.

**Who.** The context is the controller's invoke
(`internal/camera/camera.go:340`). Home Assistant sends the result at
once (`webrtc.py:280`), so the wait ends early only when the connection
to Home Assistant is stalled for `CallTimeout` (30 s, `client.go:49`)
or — I, go-fabric's cancellation of an invoke's context on a closed
exchange or session was not read — when the controller abandons the
request while the command is on its way. A narrow window; the leak in
it is whole. Each such offer is a Home Assistant
session that no table counts: not the host's map, not the kit's
CurrentSessions (the port returned an error, `gf:provider.go:557-560`),
so HR-P1-bound-02's cap does not see it.

**Fix.** In `Subscribe`: when `callWith` fails after the command was
written (any error but a write error), send `unsubscribe_events` for the
id on a context of its own (`context.WithoutCancel`, `CallTimeout`),
best effort, before returning. Home Assistant answers `not_found` when
the command had failed on its side; that is ignored. One place; every
future subscription command gets it.

**Regression test (red today).** `hatest`: a script that delays the
result of `camera/webrtc/offer`; `WebRTCOffer` with a 50 ms context fails;
`Server.Unsubscribed()` holds the command's id within a second.

---

## HR-P1-bound-17 — low — stills and candidates: one Home Assistant request per controller request

**Claim.** Two more places where a controller's request becomes Home
Assistant work without a bound of this host's:

- `CaptureSnapshot` is one `GET /api/camera_proxy/<entity>` per invoke,
  each up to `CallTimeout` (30 s), no coalescing and no limit on those in
  flight (`internal/camera/camera.go:678-698`,
  `internal/ha/camera.go:140-194`). For many integrations a still is a
  fresh frame grab from the device. The kit's `op` lock serialises the
  WebRTC provider's commands (`gf:cluster/camera/provider.go:86-90`);
  whether CameraAvStreamManagement's invokes are serialised alike was not
  read (I).
- After Home Assistant named its session, `ProvideIceCandidates` sends
  one `camera/webrtc/candidate` command per list entry, in sequence, under
  the controller's context (`camera.go:410-415`). The list is "min 1"
  only (`gf:cluster/spec/webrtctransportprovider/definition_gen.go:881`);
  `MaxPendingCandidates` (64) bounds only the queue before the session
  event (`camera.go:399-404`). A TCP message of 64 000 bytes holds
  thousands of short entries; the kit holds `op` for the whole run, so
  the camera's other commands wait.

**Who.** R4. No amplification beyond one-to-one, hence low; the bound is
missing all the same, and HR-P0-bound-28's reasoning ("an ICE agent
gathers a few dozen at most") applies to the whole session, not only to
the time before the session event.

**Fix.** (1) Stills: one request in flight per camera, later callers
share its result (a single-flight keyed by width and height) and a result
younger than about a second is reused. (2) Candidates: the 64 per call
and a per-session total (proposal: 256) refused with RESOURCE_EXHAUSTED.

**Regression test (red today).** (1) Twenty concurrent `CaptureSnapshot`
calls against an upstream that blocks: `upstream.Image` was entered once.
(2) A call with 65 candidates after the session event is refused and
`upstream.cands()` is empty.
