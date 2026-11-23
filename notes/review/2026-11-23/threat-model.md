# Threat model, phase 0

Code at baseline `10ee7b4`. Pins: go-fabric `v0.25.0`, go-fabric-ui
`v0.13.0`, coder/websocket `v1.8.15` (module cache; `go.mod:6-11`), Home
Assistant core `2026.10.3` (`git show`), Supervisor `fe91b8db` and its tags
(`gh api`, raw files). Paths without a prefix are this repository; `ui:` is
go-fabric-ui, `gf:` go-fabric, `ws:` coder/websocket, `core:`
home-assistant/core, `sup:` home-assistant/supervisor.

**M** = measured (the line was read; the statement is what the line does).
**I** = inferred (follows from measured lines plus kernel, browser, Docker,
standard-library or Home Assistant behaviour that was not read at a pin; to
be shown by the unit named). Nothing was executed; no exploit was run.

Trust is decision 11: Core, the Supervisor and the token are trusted; entity
data is not; a commissioned controller is a fabric administrator limited to
the exposed entities' services; the LAN is untrusted.

**Held back: 1 row**, `HR-P0-auth-20` (medium), in `held/threat-model.md`
(git-excluded). Sections 3 and 5 mark where it sits (role R6, threat T4)
without its detail.

## 1. Assets

| # | Asset | Where it lives | Who is meant to reach it |
| --- | --- | --- | --- |
| A1 | The Home Assistant token: `HA_TOKEN`, else `SUPERVISOR_TOKEN`. An administrator's: every service, every registry, the camera proxy ("acts as an administrator", `addon/config.yaml:28-31`, `docs/operations.md:127-136`) | environment only (`internal/config/config.go:219-224`; no flag, `:188-210`); held in `Config.HAToken` and `ha.Options.Token` (`cmd/ha2fabric/main.go:74`); sent at `internal/ha/client.go:269-273` (WebSocket `auth`) and `internal/ha/camera.go:151-153` (Bearer). No other use (grep `Token` over non-test Go: the six lines above) (M) | Home Assistant at `HA_URL` only |
| A2 | The configured node's passcode and the codes that encode it (manual code, QR payload) | `PASSCODE` or `--passcode`, default `20202021` (`config.go:43`, `:191`; `addon/config.yaml:50`), `script/run.sh:17`; `node.Config` (`internal/config/node.go:47-52`); the log at every node start (`cmd/ha2fabric/nodes.go:107`, `:306-315`); the pairing page (`ui:fabricui/handler.go:365`, admin) | The operator; a controller during pairing |
| A3 | Registry nodes' passcodes | `nodes.json` beside the store, a temp file from `os.CreateTemp` renamed over it (`internal/nodes/nodes.go:459-487`; mode 0600 is `CreateTemp`'s, I); random and validated (`nodes.go:381-391`); the same log line per node (`nodes.go` in `cmd`, `:107`, `:403`) | as A2 |
| A4 | The nodes' stores: fabrics, operational keys, ACLs, endpoint numbers | `STORE_PATH` (`config.go:232-238`; add-on `/data/ha2fabric.db`, `run.sh:24`), `ha2fabric-<key>.db` per registry node (`nodes.go:80`); go-fabric's SQLite store (`node.go:65`). File mode and what is at rest in clear: not read here | The daemon; the add-on's backup (`addon/config.yaml:39-41`) |
| A5 | The selection (`selection.json`, `selection-<key>.json`) and the registry (`nodes.json`): which entities a controller reaches, which nodes exist | `internal/selection/selection.go:181-194`, `internal/nodes/nodes.go:459-487`; keys validated on load (`nodes.go:149`, `:267-272`) | The web UI's administrator |
| A6 | The log ring: the daemon's whole logger, served by the Operation page | `internal/web/operation.go:19-22` (ring in front of the stderr text handler), `main.go:72`, `:114`; routes `ui:handler.go:560-561` (admin since v0.10) | Administrator; stderr goes to the add-on log or the service manager |
| A7 | Camera stills and WebRTC signalling (SDP, ICE candidates). Media does not pass the daemon: only signalling is relayed (`internal/camera/camera.go:306-349`, `:454-517`) | stills `internal/ha/camera.go:126-180` → `camera.go:653-673`; opt-in `CAMERA` (`config.go:207`, `:299-303`) | A commissioned controller of an exposed camera |
| A8 | The power to make Home Assistant act: `call_service` on exposed entities | `internal/adapt/command.go:88-226`, `internal/bridge/command.go:56-89` | Commissioned controllers |
| A9 | The admin capability of the web UI: selection, windows, fabrics, erase, node registry, log level | `ui:handler.go:361-401`, `:553-561`; gate `main.go:194-203` | Section 2.1 |

## 2. Trust boundaries and entry points

### 2.1 The web listener, per mode

| Mode | Listens | Client filter | `Admin` | Host check |
| --- | --- | --- | --- | --- |
| Add-on (`SUPERVISOR_TOKEN` set, `config.go:219-220`) | `0.0.0.0:8486` on the host's network (`run.sh:70`, `addon/config.yaml:37`, `:43-44`) | `WEB_ALLOW=172.30.32.2` (`run.sh:71`); every other peer 403 before go-fabric-ui (`internal/web/web.go:86-99`) | peer address equals `172.30.32.2` (`main.go:196-197`, `web.go:136-143`, `config.go:63`) and `X-Fabric-UI: 1` (`ui:handler.go:777-780`) | every request of that peer (`main.go:212-217`), admitted `byPredicate` (`ui:host.go:72-73`) |
| Container image | `0.0.0.0:8099` (`Dockerfile:31`), run with `--network host` (`Dockerfile:6`, `README.md:119`) | none unless `WEB_ALLOW` (`web.go:87-89`) | `ReadOnly` (`main.go:200-201`, `web.go:149`) or token mode with `WEB_ADMIN` (`main.go:198-199`) | name rules (`ui:host.go:49-60`), `WEB_ALLOWED_HOSTS` (`main.go:116`) |
| Stand-alone binary | `127.0.0.1:8099` (`config.go:48`, `:203`) | as the image | as the image | as the image |

All modes (M): plain HTTP, `ReadHeaderTimeout` 10 s, `ReadTimeout` 30 s,
`IdleTimeout` 120 s, no `WriteTimeout`, no connection cap
(`web.go:106-118`). The filter and `From` read `r.RemoteAddr` only; no
forwarded header is read anywhere (grep `X-Forwarded|X-Remote-User|
X-Ingress|X-Hass` over non-test Go: none).

What go-fabric-ui `v0.13.0` leaves open without admin rights (M,
`ui:handler.go:361-386`, `:552-561`): `GET /node/status`, `GET /node/stream`,
`GET /nodes`, `GET /operation/info`, `GET /operation/log/level`. The status
of a non-admin carries the label, listen address, discriminator, vendor and
product (not the store path, `ui:handlers_node.go:156-165`), warning codes
without text (`:176-183`), the counters, and in token mode the token itself
(`:151`, `CSRFToken: h.token`). Everything else is an admin route.

Token mode (`WEB_ADMIN=true`, stand-alone): the token is 16 random bytes
(`ui:handler.go:340-344`), compared in constant time (`:781-782`), and
served by the open status route — whoever reaches the listener is the
administrator. `docs/operations.md:316-319` says so.

### 2.2 The Ingress path (add-on)

Browser → Core `/api/hassio_ingress/<token>/…` (no Core auth on the view,
`core:homeassistant/components/hassio/ingress.py:62-67`) → Supervisor
`/ingress/<token>/…`, which requires its session cookie
(`sup:supervisor/api/ingress.py:180-184`) → the add-on at the gateway
address, from `172.30.32.2`
(`notes/reads/read-2026-11-23-ingress-host-facts.md`, hop 2). `Host` and
`Origin` arrive as the browser sent them (same note). The daemon's whole
authentication of the user is "the peer is the Supervisor".

### 2.3 The Matter node

UDP `LISTEN` (default `:5540`), TCP too with `CAMERA` (`node.go:42-46`),
mDNS on every or the named interfaces. go-fabric's surface, not reviewed
here (it is the gold standard, `CLAUDE.md`, Rules); what this host decides:

- The node's own window opens at start while it has no fabric, and again
  after its last fabric is removed: `WindowOnDemand` is not set
  (`node.go:40-68`; `gf:node/config.go:523-535`) (M). `cmd` logs it
  (`nodes.go:279-281`, `:312-314`).
- `BasicCommissioning` is not set (`node.go:47-52`; default off,
  `gf:node/config.go:503-512`) (M): a commissioned node offers no window on
  the fixed passcode to its administrators.
- PBKDF: 1000 iterations and a constant salt (`node.go:22-28`, `:50-51`);
  issue #64 / PR #68 is the comment about it.

### 2.4 Home Assistant

One WebSocket to `HA_URL` (`client.go:197`), the token in-band after
`auth_required` (`client.go:262-273`); REST for stills on the URL derived
from it (`ha/camera.go:95-117`). In the add-on `ws://supervisor/core/
websocket` (`config.go:33`, `:226-228`). The endpoint is trusted; what it
carries about entities is not (2.5).

### 2.5 Entity data

Names, attributes, registry fields, label names, camera answers and
candidates, stills: produced by integrations and devices, carried by the
trusted Core. Sinks: `internal/adapt` and go-hamapping (the plan), go-fabric
attributes a controller reads, the log (text handler, `operation.go:20`),
the ring and the pages (go-fabric-ui renders; its review's T11), the
`--dump-plan` table (`main.go:270-283`).

## 3. Attacker roles

| # | Role | Reaches |
| --- | --- | --- |
| R1 | LAN client | the web listener (any mode not bound to loopback), the Matter UDP/TCP port, mDNS, and the network path between the daemon and a remote Home Assistant |
| R2 | Hostile page or origin in an operator's browser | the listener through the browser: DNS rebinding, cross-origin requests, framing |
| R3 | A peer that claims `172.30.32.2` | from the LAN by spoofing; from the Supervisor's Docker network by address takeover |
| R4 | Commissioned Matter controller (a fabric administrator) | every cluster of every exposed endpoint; with it `call_service` (A8), camera sessions and stills (A7), further commissioning windows |
| R5 | Hostile integration or device behind Home Assistant | entity data (2.5) |
| R6 | held back with `HR-P0-auth-20` | — |
| R7 | Reader of the daemon's log outside the UI (add-on log, journal, a pasted log) | A2, A3 as far as the log holds them |

## 4. Re-check of go-fabric-ui's host observations (its threat model, section 4)

| # | Then (`ec7deea`, go-fabric-ui `v0.9.2`) | At `10ee7b4` | State |
| --- | --- | --- | --- |
| HR-1 | held back there; the Host check | **Closed by #66** (`cb51196`). go-fabric-ui `v0.13.0` (`go.mod:6`) checks every request's `Host` before any route, bundle included (`ui:fabricui/options.go:403-407`, `ui:host.go:82-99`): loopback names, IP addresses without a zone, `AllowedHosts` (`ui:host.go:49-60`), else `AllowHost` (`:72-73`). This host passes `WEB_ALLOWED_HOSTS` and, in the add-on only, the Ingress source (`main.go:116`, `:212-217`); the value is validated at start (`config.go:322-326`, `:359-373`). An admin route's `Origin` must pass the name rules or, for a predicate admission, equal `Host` (`ui:handler.go:759-773`) | M. The owed live Ingress test stays the release gate (decision 8) |
| HR-2 | The image listens everywhere without a filter | **Holds, narrowed.** `Dockerfile:31` unchanged; no `WEB_ALLOW` default (`web.go:87-89`). Without `WEB_ADMIN` the network now gets the open routes of 2.1 only (`ReadOnly`, `web.go:145-149`), no longer the log or the read classes. With `WEB_ADMIN` the whole network is the administrator, over plain HTTP; nothing at start refuses or warns (`config.go:294-298`, `:312-321`) | M → `HR-P0-cfg-22` |
| HR-4 | The log ring is the daemon's whole logger | **Holds; the routes are admin-only now** (`ui:handler.go:560-561`). The ring is still every record of every package (`operation.go:19-22`, `main.go:72`), and this host does log pairing codes into it (`nodes.go:312-314`), against the port's statement that nothing it returns carries a passcode (`ui:fabricui/operations.go:25-26`, `ui:docs/embedding.md:320-321`) | M → `HR-P0-leak-24` |
| HR-5 | Plain HTTP stand-alone | **Holds.** `http.Server` on a plain TCP listener, no TLS option (`web.go:106-118`). In token mode the token (status answer) and the pairing codes cross the network in clear; in the add-on the hop is Supervisor → host on the Docker bridge | M → folded into `HR-P0-cfg-22` |
| HR-7 | An unparsable `Allow` entry is dropped; none left means no filter | **Holds.** `web.go:66-70` drops, `:87-89` turns the filter off; `web.New` returns no error for it. The daemon's configuration still refuses such a value first (`config.go:317-321`), so it bites a second caller of `web.New` only (today: tests) | M → `HR-P0-err-26` |

## 5. Per threat

| # | Threat | Entry | Existing control | Gap | M/I | Read further in |
| --- | --- | --- | --- | --- | --- | --- |
| T1 | LAN client reads or writes through the add-on's listener | `0.0.0.0:8486` on the host | 403 for every peer but `172.30.32.2`, before go-fabric-ui (`web.go:90-96`; `run.sh:71`) | the listener is on every interface although the Supervisor dials the bridge gateway only (`HR-P0-cfg-27`); the filter is set by `run.sh`, the admin gate by `cfg.Addon` — two places (`HR-P0-cfg-30`) | M | 1 S; X1 |
| T2 | LAN client spoofs `172.30.32.2` | TCP to the host's LAN address with a forged source | TCP needs the handshake's answer, which is routed to the real Supervisor on the bridge; reverse-path filtering may drop the SYN first | none seen for an off-bridge peer; HAOS's `rp_filter` setting not read | I | X1 (state the argument in `docs/operations.md`) |
| T3 | A container on the Supervisor's network takes over `172.30.32.2` (ARP) | another add-on with raw-socket rights | none in the daemon: the peer address is the only credential | by decision 11 the Supervisor is trusted; co-installed add-ons are not named there — owner question 1 | I | X1 |
| T4 | held back until its fix | — | — | **1 row held back** (`HR-P0-auth-20`) | — | 1 S; X1 |
| T5 | LAN client becomes administrator stand-alone | the image's `0.0.0.0:8099`, or any `WEB_LISTEN` off loopback, with `WEB_ADMIN` | `WEB_ALLOW` if set; documented (`docs/operations.md:316-319`) | no refusal or warning at start; plain HTTP (`HR-P0-cfg-22`) | M | 1 S |
| T6 | LAN client reads without rights | open routes of 2.1 on the image | status reduced for a non-admin (`ui:handlers_node.go:156-183`) | label, listen address, discriminator, counters, node list and the node stream are public on the image's default; discriminator and label are in mDNS anyway | M | 1 S (accept or bind the image to loopback, with T5) |
| T7 | DNS rebinding against a stand-alone daemon | browser → `127.0.0.1:8099` under an attacker's name | Host check (section 4, HR-1) | none seen; `*.localhost` rests on the resolver (`ui:host.go:43-48`, go-fabric-ui's decision 37) | M | X1 |
| T8 | Cross-origin write or read from a hostile page | admin routes; status (token) | `Origin` check and the custom header on every admin route (`ui:handler.go:738-748`, `:777-783`); under Ingress `Origin` must equal `Host` (`:771-772`) | none seen here; go-fabric-ui's review owns the browser side (CORS absence at `v0.13.0` not re-read) | M / I | X1 |
| T9 | Framing | `/` | go-fabric-ui sets `frame-ancestors 'self'` (`ui:fabricui/spa.go:105`, `:148`); this host passes no `FrameAncestors` (`main.go:112-120`) | under Ingress the page is framed by Home Assistant's frontend, same origin by path; to confirm in the live test | M / I | X1; owner's live test |
| T10 | Exhaustion of the web server | any peer the filter admits; in the add-on only the proxy | header, read and idle timeouts (`web.go:106-110`); 403 before the handler | no connection cap; a filtered peer still costs an accept and a 403; open streams are go-fabric-ui's (its finding 29) | M | X2 |
| T11 | LAN device commissions the bridge | PASE on the node's own window | a valid passcode is required and trivial ones are refused (`config.go:260-266`); registry nodes get random ones (`nodes.go:381-391`) | the configured node's default is the public test value and nothing refuses or warns (`HR-P0-cfg-21`) | M | 1 S |
| T12 | A commissioned controller makes Home Assistant do more than the exposed entities' services | Invoke on a bridged endpoint | closed table: the entity id is the plan's, never the controller's (`command.go:92-96`); services are constants per domain (`:37-82`); numbers are parsed and clamped (`:116`, `:121`, `:152`, `:217-220`, `:381-392`); anything else is `ErrCommand` (`:225`); queue bounded (`bridge/command.go:30`) | four text values go to Home Assistant as received from the plan: `hvac_mode` (`:127`), `preset_mode` (`:158`), `fan_mode` (`:176`), `option` (`:181`) — whether a controller's bytes can reach them is go-hamapping's row (boundary, decision 9) | M; origin of the text I | X2 |
| T13 | A controller's reach is wider than the operator expects | the exposure rule | disabled and hidden entities never (`internal/expose/expose.go:137-140`); exclude before include before Assist (`:133-166`); Assist exposure is Home Assistant's list | `EXPOSE_ASSIST` defaults to true (`config.go:201`, `addon/config.yaml:55`), and Home Assistant exposes new `cover`, `switch`, `scene`, `climate`, `media_player`, `vacuum`, `water_heater` entities to Assist by default (`core:homeassistant/components/homeassistant/exposed_entities.py:32-44`, `:67-69`, `:315-316`): a garage-door cover or any scene is a controller's without an operator's act. Where exposed: `lock.unlock` without a code (`command.go:46`), `script.turn_on` (`:75`), `automation.trigger` with Home Assistant's default `skip_condition` true (`:79-81`), `button.press` (`:55`), `valve` (`:65`) | M | X2 (the lock row's PIN and remote-operation rules at go-hamapping); phase 3 docs (does `docs/operations.md` say what a paired controller can do) |
| T14 | A controller exhausts the camera relay or Home Assistant through it | `ProvideOffer`, `ProvideIceCandidates`, `CaptureSnapshot` | one Home Assistant subscription per session, the old one of the same id closed first (`camera.go:310-321`); signals and commands bounded by `SignalTimeout` (`:208-209`, `:486`, `:548`); stills ≤ 64 000 bytes (`ha/camera.go:87`, `:168-174`) | `session.pending` grows without a bound until Home Assistant names its session (`camera.go:379-382`); no cap of sessions per camera in this package (go-fabric's kit may have one) (`HR-P0-bound-28`) | M; kit cap I | 1 S; X2 |
| T15 | Hostile entity text reaches a log, a page or a controller as markup or control bytes | names, attributes, error texts | slog's text handler quotes (`operation.go:20`); ring attributes are plain strings (`operation.go:81-87`); the pages interpolate text (go-fabric-ui's T11) | lengths and encodings into go-fabric attributes (NodeLabel 32 bytes and others) are `internal/adapt` and go-hamapping; not read here | I | 1 (adapt reader); X3; phase 0 fuzz |
| T16 | Hostile or oversized Home Assistant frames | the WebSocket | 64 MiB per frame (`client.go:50-52`, `:202`); a refused frame or event is logged with its size, not its content (`client.go:313`, `:335`); `Decode` has a fuzz target (`internal/ha/message_test.go:68`) | one entity with very large attributes can push `get_states` over the limit: the connection drops and reconnects without ever syncing | I | X2; phase 0 unit E |
| T17 | Hostile still | camera proxy answer | size bound; content type must be `image/jpeg`; only the JPEG header is decoded, dimensions 1..65535 (`camera.go:659-668`) | none seen; the bytes go to the controller as they came | M | 1 S; phase 0 fuzz |
| T18 | Hostile WebRTC answer or candidate from an integration | `camera/webrtc/offer` events | JSON decoded per event, unknown types ignored (`camera.go:458-515`); `sdpMLineIndex` range-checked (`:427-433`) | SDP and candidate text are relayed unparsed to the controller; size is the kit's bound | M | 1 S |
| T19 | The token is read on the network | `HA_URL` with `ws://` to another host | in the add-on the hop is the Supervisor's internal name (`config.go:33`); `wss://` verifies certificates (no `InsecureSkipVerify` in the tree, grep) | `ws://` to any host is accepted without a word (`config.go:251-253`); the usage text, the image's comment and the README show `ws://homeassistant.local:8123` (`config.go:188`, `Dockerfile:7`, `README.md:120`) — an mDNS name on the untrusted LAN (`HR-P0-ha-23`) | M | 1 S |
| T20 | The token is sent to something that is not Home Assistant | redirect; a wrong `HA_URL` | none: the dial follows redirects (`ws:dial.go:76`, `:89-98`) and the token goes to whichever endpoint says `auth_required` (`client.go:266-273`); the REST client is `net/http`'s default redirect policy (`client.go:139`), which drops `Authorization` across hosts (I) | by decision 11 the endpoint is trusted; a note (`HR-P0-ha-31`) | M / I | 1 S |
| T21 | The token in a log, a file or an error | — | env only; never formatted: the six uses of A1 are all there is; auth failure logs Home Assistant's message, not the token (`client.go:283-284`) | none seen for the token. `HA_URL` itself is logged verbatim (`main.go:86`, `client.go:225`) and echoed on a scheme error (`config.go:252`) — userinfo or a query would land in the ring (`HR-P0-leak-25`) | M | 1 S; `make secrets` (unit E) |
| T22 | The passcode in a log | start of every node | — | manual code and QR payload at info, every start, commissioned or not (`HR-P0-leak-24`); `-h` prints an environment `PASSCODE` as the flag's default (`HR-P0-leak-29`) | M; `-h` I | 1 S |
| T23 | Files: a secret readable, a corrupt file fails open, a key walks out of the directory | `nodes.json`, `selection*.json`, the stores | temp file and rename (`nodes.go:467-480`, `selection.go:181-194`); corrupt moved aside, registry starts empty and keeps the stores (`nodes.go:126-137`); keys validated (`nodes.go:149`, `:267-272`) | the stores' modes and the add-on's `/data` are not read here; a corrupt `selection.json` moved aside — what the node then exposes (Assist rule only?) is phase 1's | M / I | 1 S |
| T24 | Erase, factory reset, node creation by a non-admin | registry and maintenance routes | admin routes (`ui:handler.go:382-394`); `NODE_LIMIT` ≤ 32 (`config.go:304-308`, `nodes.go` in `cmd`, `:463-464`) | follows T4 and T5 | M | 1 S |
| T25 | The add-on's container | root in the Home Assistant base image on the host's network (`addon/Dockerfile:34-44`, no `USER`; `addon/config.yaml:37`) | the Supervisor's default profile; no extra privilege key in the manifest (grep `apparmor|privileged|full_access|hassio_api|auth_api`: none) | no AppArmor profile of its own (`ls addon/`: none); the plain image is distroless non-root (`Dockerfile:27`, `:33`) | M | phase 3 (images) |

## 6. Findings

"Contract" says whether the fix changes what an operator or client can
observe. Three medium rows (21, 22, 23) are not held: each restates what
`README.md`, `addon/DOCS.md` or `docs/operations.md` already publish; the
finding is the missing guard or warning. Owner question 2.

| ID | Sev. | Held | Where | Claim | State | Proposed fix | Contract |
| --- | --- | --- | --- | --- | --- | --- | --- |
| HR-P0-auth-20 | medium | **yes** | — | held back until its fix (`held/threat-model.md`) | — | — | — |
| HR-P0-cfg-21 | medium | no | `config.go:43`, `:191`, `:260-266`; `addon/config.yaml:50`; `node.go:47-52`; `gf:node/config.go:523-535` | The configured node's default passcode is the public test value `20202021`. It passes `ValidateSetupPIN`, and the node opens its own window at start while it has no fabric and again after its last fabric goes. Until the first pairing, any LAN device that knows the public value can commission the bridge and holds A8 and A7. The docs say "Change it" (`README.md:148`, `addon/DOCS.md:22`, `:62`, `docs/operations.md:394-395`); the daemon neither refuses nor warns, and no status-page warning exists (warning codes: `internal/exposure/exposure.go:342-347`, `nodes.go` in `cmd`, `:203`) | M | Wizard: (a) refuse the default unless an explicit opt-in, (b) a random passcode on first start kept beside the store, as the registry nodes get, (c) a `Warning` on the status page and a log line. (b) is the best design: registry nodes already work that way | yes for (a) and (b) |
| HR-P0-cfg-22 | medium | no | `Dockerfile:31`; `main.go:194-203`; `web.go:87-89`, `:106-118`; `config.go:294-298`, `:312-321`; `ui:handlers_node.go:151` | `WEB_ADMIN=true` with a listener off loopback and no `WEB_ALLOW` makes every LAN client the administrator, over plain HTTP: the pairing codes, the selection, erase. The image's default listener is `0.0.0.0:8099`, so on the image `WEB_ADMIN=true` alone does it. Documented (`docs/operations.md:316-319`, "keep it on a trusted network"), which decision 11 says the LAN is not; nothing at start refuses or warns. Carries HR-2 and HR-5 | M | Refuse `WEB_ADMIN` on a non-loopback `WEB_LISTEN` without `WEB_ALLOW` (or demand an explicit override), and move the image's default to loopback or document a required `WEB_ALLOW`; say in the docs that the page is plain HTTP | yes |
| HR-P0-ha-23 | medium | no | `config.go:188`, `:251-253`; `client.go:269-273`; `ha/camera.go:100-103`, `:151-153`; `Dockerfile:7`; `README.md:120` | `HA_URL` with `ws://` to another host sends the administrator token in clear, in the WebSocket `auth` and as a Bearer header for stills. The check is the scheme prefix only; the examples in the usage text, the image and the README are `ws://homeassistant.local:8123`, a name any LAN peer can answer for | M (the active attack is I) | A start-up warning when the scheme is `ws` and the host is neither loopback nor `supervisor`; a paragraph in `docs/operations.md` ("The token") and `wss://` or same-host in the examples | a new log line |
| HR-P0-leak-24 | low | no | `nodes.go` in `cmd`, `:107`, `:306-315`; `operation.go:19-22`; `ui:fabricui/operations.go:25-26`; `ui:docs/embedding.md:320-321` | Every node start logs the manual code and the QR payload — the passcode — at info, to stderr and the ring, also when the node is commissioned and the codes open nothing. go-fabric-ui's `Operations` port states that nothing it returns carries a passcode; this host's ring does. Documented as a feature (`docs/operations.md:176`, `:383`, `README.md:133`). The codes then live in the add-on log, the journal and any pasted log (R7); they matter again whenever the last fabric is removed | M | Wizard: log the codes only while the node has no fabric, or never and point to the pairing page; either way bring go-fabric-ui's sentence and this host into agreement | yes |
| HR-P0-leak-25 | note | no | `main.go:86`; `client.go:225`; `config.go:252` | `HA_URL` is logged and echoed verbatim; userinfo or a query string (a proxy's credential) is neither refused nor redacted | M | Refuse a URL with userinfo, or log `url.Redacted()` | no |
| HR-P0-err-26 | low | no | `web.go:66-70`, `:87-89` | HR-7 at this baseline: `web.New` drops an `Allow` entry that does not parse, and with none left serves everyone. Fail open inside the package; safe only through `config.go:317-321` | M | `web.New` returns an error for an entry that is not an IP address | no (the daemon's own path already refuses) |
| HR-P0-cfg-27 | low | no | `run.sh:70-71`; `addon/config.yaml:37`, `:43-44`; `notes/reads/read-2026-11-23-ingress-host-facts.md` (hop 2) | The add-on's web listener binds every interface of the host although its only client dials the Docker bridge's gateway address. The LAN gets a 403, but also the listener itself (T10) and the spoofing question (T2) | M (bind); the dial address is the note's reading of `sup:docker/app.py:188-191` | Bind the gateway address when it exists, or keep `0.0.0.0` and record why in `docs/operations.md`; for the live test either way | no |
| HR-P0-bound-28 | low | no | `camera.go:170-178`, `:367-383`, `:310-337` | A controller's ICE candidates queue without a bound until Home Assistant names its session; nothing in this package caps the sessions of one camera | M (absence); a cap in go-fabric's kit is not read | Cap `pending` (drop or refuse beyond a small number); read the kit's session bound in phase 1 | no |
| HR-P0-leak-29 | note | no | `config.go:181-187`, `:191` | The environment's value becomes each flag's default, so `ha2fabric -h` prints an environment `PASSCODE`; `--passcode` puts it on the command line. The token has no flag | M (the default); the printing is the `flag` package's (I) | Do not show the passcode's default in the usage text; note in the docs that the environment is the place for it | usage text |
| HR-P0-cfg-30 | note | no | `config.go:219-220`; `main.go:196-197`, `:212-217`; `run.sh:70-71` | Add-on mode is inferred from `SUPERVISOR_TOKEN` alone and switches the admin gate and the Host predicate in the daemon, while the listener and the client filter that the gate relies on are set by `run.sh`. The daemon does not check that, in add-on mode, `WEB_ALLOW` is exactly the Ingress proxy | M | In add-on mode default `WEB_ALLOW` to `IngressProxy` in `config` (or refuse another value); `run.sh` then only sets the listener | no |
| HR-P0-ha-31 | note | no | `client.go:197`; `ws:dial.go:76`, `:89-98`; `client.go:262-273` | The WebSocket dial follows redirects with the default HTTP client, and the token is sent in-band to the endpoint the chain ends at | M | Dial with a client that refuses redirects, or record the acceptance (decision 11) | no |
| HR-P0-cite-32 | note | no | `README.md` of this directory, "Facts the cut rests on" | Line ranges drifted: the client filter is `web.go:86-99` (plan: `83-98`), the empty-`Allow` branch `:87-89` (`84-86`), the server's timeouts `:106-110` (`103-109`), `admin` `main.go:194-203` and `allowHost` `:212-217` (plan cites the comment blocks with them), `WEB_ALLOW` validation `config.go:317-321`. No wrong fact | M | Correct the plan's ranges in the phase 0 commit | no |

Counts, public: 3 medium, 4 low, 5 note. Held: 1 row (medium).

## 7. For the owner

1. **Co-installed add-ons** (T3): decision 11 trusts the Supervisor and
   Core. Is another add-on on the Supervisor's network trusted too? If
   not, the peer address as the only credential needs a second factor
   that only the Supervisor can supply.
2. **Holding**: rows 21, 22 and 23 are published here because the docs
   already state the condition each rests on. If they are to be held
   until their fix, they move to `held/` before this file is committed.
3. `HR-P0-auth-20` needs a decision of its own; the options are in the
   held file.
