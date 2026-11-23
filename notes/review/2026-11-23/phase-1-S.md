# Phase 1, unit S — the security paths

Reviewer S (Fable). Code at `5eefc0a` (phase 1's baseline). Brief
`notes/briefs/2026-11-23-review-p1.md`, "Unit S". Paths without a prefix
are this repository; `gf:` go-fabric v0.25.0, `ui:` go-fabric-ui v0.13.0,
`ws:` coder/websocket v1.8.15 (module cache), `core:` Home Assistant core
2026.10.3 (`git show`), `sup:` Supervisor `fe91b8db` (`gh api`, raw).

**M** = measured: the line was read at its pin, or the probe named ran.
**I** = inferred. Probes ran only in the reviewer's `git archive` copy of
`5eefc0a`; nothing ran in the worktree or against a live system.

**Held back: 1 row** (medium), in `held/phase-1-S.md` (git-excluded):
`HR-P1-auth-01`, named here by ID and severity only. The four camera lows
S held (`HR-P1-bound-02`, `-conc-03`, `-ha-04`, `-bound-17`) are
published by the owner's decision 23 (`decisions.md`); their text below is
the held file's, unchanged.

Counts: **0 high, 1 medium (held), 13 low, 8 note.**

## 1. The eight owed items

### 1.1 The admin gate in each mode

One predicate, in one place: `admin` (`cmd/ha2fabric/main.go:247-260`),
handed to go-fabric-ui as `Options.Admin` (`main.go:150`).

| Mode | The check as it is | M/I |
| --- | --- | --- |
| Add-on | The client filter admits only `172.30.32.2` (`internal/web/web.go:91-104`; the value is `config.IngressProxy`, defaulted and enforced by `internal/config/config.go:296-300`, `:381-383`, and set again by `script/run.sh:78`). An administrator is a request from that address whose `X-Remote-User-Groups` lists `system-admin` (`web.go:165-180`), once the daemon has read a Supervisor of 2026.10.4 or later (`cmd/ha2fabric/supervisor.go:90`, `:95-146`; `main.go:253-254`). Until the version is read, and below it, no request is an administrator's. go-fabric-ui adds `X-Fabric-UI: 1` and the Origin rule (`ui:fabricui/handler.go:738-748`, `:777-783`) | M |
| Container image | No filter unless `WEB_ALLOW`; `web.ReadOnly` (`web.go:186`, `main.go:258`): no request is an administrator's. With `WEB_ADMIN` as the next row | M |
| Stand-alone | `127.0.0.1:8099` by default (`config.go:49`); `ReadOnly` | M |
| `WEB_ADMIN` | go-fabric-ui's token mode (`main.go:255-256`, nil; `ui:handler.go:340-344`, `:781-782`). Refused at start off loopback without `WEB_ALLOW` (`config.go:389-391`); the add-on ignores it (`main.go:249` is tested first) | M |

**Every admin path is behind the one predicate** (M). `internal/web` adds
no route (`web.go:76-80`). go-fabric-ui routes every `x-admin` route of
the node family, the registry routes and the three operation routes
through `adminOnly`/`adminOnlyBecause` (`ui:handler.go:361-401`,
`:535-561`, `:710-728`), and a registry node's handler shares the default's
`admin` and token (`ui:handler.go:601-624`). `allowHost` uses `web.From`
(`main.go:269-274`): that is the Host check, not an admin decision.

Probe S8 (`internal/web/probe_s_test.go`, `TestProbeSAdminTable`): 29
admin routes (node family, registry, operation, two per-node), each
client kind, in the add-on's wiring (filter, `IngressAdmin`, `From` as
`AllowHost`) and in stand-alone `ReadOnly`:

```
add-on / LAN peer with forged groups: 29 routes -> map[403:29]
add-on / co-installed add-on 172.30.33.5 with forged groups: 29 routes -> map[403:29]
add-on / proxy, no groups header: 29 routes -> map[403:29]
add-on / proxy, non-admin user: 29 routes -> map[403:29]
add-on / proxy, admin, no X-Fabric-UI: 29 routes -> map[403:29]
add-on / proxy, admin user: 29 routes -> map[404:2 503:27]
stand-alone ReadOnly, no filter / (each of the six): 29 routes -> map[403:29]
```

(503 and 404: the probe's handler has no node; the request passed the
gate.)

**Can a LAN peer reach admin or a write?** Add-on: no — the filter answers
403 before go-fabric-ui, whatever headers the peer sends (M, probe). That
a LAN peer cannot hold a TCP connection from `172.30.32.2` to the host is
T2's argument (I; X1). Image and stand-alone without `WEB_ADMIN`: no
request is an administrator's (M). With `WEB_ADMIN`: only a peer that
`WEB_ALLOW` names, or loopback (M, `config.go:389-391`); decision 15's
caveat stands — an address filter is not authentication, and the page is
plain HTTP.

**Can a co-installed add-on?** Every admin route refused it in the probe (M, probe row 2).
One residual row is held: **`HR-P1-auth-01`, medium (I)**.

Small things at the gate, none a defect: the gate, once open, does not
read the version again (`supervisor.go:100-106`; a Supervisor does not go
back). `web.From` on a `RemoteAddr` it cannot parse is false
(`web.go:143-147`; test `web_test.go:183-209`). An IPv6 peer with a zone
never matches a `WEB_ALLOW` entry (`net.ParseIP` refuses the zone,
`web.go:96-98`): fail closed. Findings: `HR-P1-test-16` (no test holds
the table above), `HR-P1-ha-13`, `HR-P1-cfg-15`.

### 1.2 The camera relay

- **Sessions per camera**: its own row, **`HR-P1-bound-02`, low**.
  Two rows go with it: **`HR-P1-conc-03`** (low) and
  **`HR-P1-bound-17`** (low).
- **The session's end and its Home Assistant subscription** (M,
  `internal/camera/camera.go`). Every path that takes a session out of
  the host's map closes Home Assistant's: `EndSession` (`:420-430`), Home
  Assistant's error event (`:534-537` → `end` → `drop` → `unsubscribe`,
  `:599-629`), a failed signal (`:588-595`), a re-offer (`:331-339`), a
  rebuilt device (`:244-264`), `Close` (`:285-311`), a removed fabric
  through the kit's `MatterDropFabric`
  (`gf:cluster/camera/fabric_state.go:48-68`). A dropped Home Assistant
  connection or a reader that fell 64 events behind ends the subscription
  with an error, and the relay then ends the session towards the
  controller (`:495-498`; `internal/ha/client.go:341-346`, `:385-388`).
  `unsubscribe` runs on a context of its own, so a cancelled request
  still closes the session (`:464-470`). The kit serialises a provider's
  commands (`gf:cluster/camera/provider.go:86-90`), so two offers of one
  id cannot interleave in `ProvideOffer`. Findings: `HR-P1-err-05`
  (a failed re-offer), and **`HR-P1-ha-04`** (low; merged into `HR-P1-ha-62`).
- **Hostile answers and candidates (T18)** (M). Each event is decoded on
  its own; one that does not decode is logged without its content and
  skipped (`:484-488`); unknown types are ignored (`:538-539`);
  `sdpMLineIndex` outside 0..65535 is refused (`:454-456`); an empty
  candidate is the end-of-candidates mark and is not sent (`:521-525`).
  Go's decoder turns invalid UTF-8 into U+FFFD, so the kit always gets
  valid UTF-8; control characters and NUL pass (probe S9:
  `answer="v=0\r\n\x00�"`). The SDP and the candidate text go to the
  controller unparsed; their size is bounded by the frame limit here
  (64 MiB, `client.go:54`) and by the kit's one-message bound on the way
  out (`camera.ErrPayloadTooLarge`, handled at `:588-590`, the session
  ends). The relay is signalling only, so a hostile integration can at
  most send the controller a session description of its choosing — which
  is what a camera integration does by design. An empty `answer` is
  relayed as an empty SDP (`:518-519`); a note inside `HR-P1-data-20`.
- **The still (T17)** (M). At most 64 000 bytes, read through a
  `LimitReader` of one byte more (`internal/ha/camera.go:87`, `:182-188`);
  the content type must be `image/jpeg` (`camera.go:684-686`); only the
  JPEG header is decoded and the size must be 1..65535 each way
  (`:687-693`; the only decoder registered is JPEG, `:33`); the entity id
  is path-escaped, the size parameters are integers (`ha/camera.go:145-155`);
  no redirect is followed (`client.go:141`, test
  `client_url_test.go:85`). The bytes go to the controller as they came;
  nothing else of the response does. No finding on the still's content.
  One row concerns the still's path: **`HR-P1-bound-17`** (low).

### 1.3 T6: the image's public read routes — recommendation: **bind**

`HR-P1-cfg-12` (low). On the image's default (`Dockerfile:31`,
`0.0.0.0:8099`, `--network host`) every LAN client reads, without rights
(M, `ui:handlers_node.go:150-183`, `ui:handler.go:552`, `:557-558`,
`ui:operations.go:180-190`; probe S8's last lines): the label, the
Matter listen address, the discriminator, vendor and product, the
warning **codes** in force, the counters, whether a window is open, the
node list with every registry node's key and label, the daemon's and
go-fabric's version and the uptime, the log level, and the reduced event
stream (up to go-fabric-ui's stream cap). Discriminator and label are in
mDNS anyway; the version, the node list and the warning codes are not.

Recommendation: take `WEB_LISTEN=0.0.0.0:8099` out of the image, so that
the image, like the binary, serves `127.0.0.1:8099` unless the operator
sets `WEB_LISTEN` (and then says who with `WEB_ALLOW`). It is the better
design: one default for both forms, T5 and T6 closed by the same line,
and the status page of a headless container is reached the way the docs
already describe for `WEB_ADMIN` (`docs/operations.md:367-371`).
Accepting instead means documenting the list above as public. Owner
question 1.

### 1.4 T23: files

| File | Written | Mode | M/I |
| --- | --- | --- | --- |
| `nodes.json`, `passcode.json` | temp file in the directory, `Sync`, rename (`internal/nodes/nodes.go:477-502`) | 0600 (`os.CreateTemp`) | M, probe S3a: `nodes.json -rw-------, passcode.json -rw-------, selection.json -rw-------` under umask 022 |
| `selection.json`, `selection-<key>.json` | the same (`internal/selection/selection.go:173-201`) | 0600 | M, probe S3a |
| the stores (`ha2fabric.db`, `ha2fabric-<key>.db`) | go-fabric's opener creates a new file 0600 before SQLite opens it (`gf:store/sqlite.go:42-47`); the `-wal`/`-shm` files take the database's mode (SQLite's rule, I) | 0600 | M / I |
| a file that is already there | read as it is; its mode is neither checked nor tightened, and a file moved aside as corrupt keeps its mode | as found | M, probe S3b, S3c → `HR-P1-cfg-10` |

In the add-on all of it is under `/data`, the container's own volume, and
in the add-on's backup (`addon/config.yaml:39-41`): a backup holds the
passcodes and the nodes' operational keys. No directory `fsync` follows a
rename; after a power loss the old file is what remains, which is
consistent (note, no row).

**After a corrupt `selection.json` was moved aside** the store starts
empty (`selection.go:89-103`), so the node exposes what the configured
rule says without any override: with the default `EXPOSE_ASSIST=true`,
Assist's list plus `INCLUDE`, minus `EXCLUDE`. Every override is gone,
those that **hid** an entity too: an entity the operator switched off in
the web UI is exposed again. The same for a file of another `version`
and for a truncated one (M, probe S5). The daemon logs it and shows the
status warning `selection_corrupt`
(`internal/exposure/exposure.go:344-352`). → `HR-P1-err-06`, owner
question 2.

**After a corrupt `nodes.json`**: the registry starts empty, the nodes'
stores stay, and no new node takes a key whose files exist
(`nodes.go:129-140`, `:309-317`): fail closed (M). **After a corrupt
`passcode.json`**: moved aside, a new passcode drawn; a paired node keeps
its fabrics (`passcode.go:55-69`) (M).

**Key validation** (M). A registry key matches
`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$` and is not reserved
(`nodes.go:277-288`); it is checked on load (`:152`) and every allocated
key is a slug that passes it (`:291-304`). No `/`, no leading `.`: a key
cannot leave the directory through `StoreFile` or `SelectionFile`
(`:83-86`), and holds no glob metacharacter for `RemoveFiles` (`:456`).
What `check` does not look at is `HR-P1-cfg-11` (note). Selection keys
are entity ids and never become a path; they are not validated on load
(`HR-P1-data-20`).

### 1.5 The Home Assistant client

- **Authentication** (M). The token goes in one `auth` message, only
  after the endpoint that was dialled said `auth_required`
  (`internal/ha/client.go:277-306`), and as the camera proxy's Bearer
  header (`ha/camera.go:165-167`). It is in no log line and no error:
  the auth failure carries Home Assistant's message (`:302`); a dial
  error has its URL rewritten (`:586-597`).
- **The token's reach** (M). Scheme `ws` or `wss` only, a host required,
  no userinfo (`config.go:407-428`); `wss` verifies certificates (no
  `InsecureSkipVerify` in the tree). Neither the WebSocket handshake nor
  the still follows a redirect (`client.go:141-158`, `:215`; tests
  `client_url_test.go:66-100`): HR-P0-ha-31 and -56 are fixed as stated.
  `ws://` to another host warns in the log and on the status page
  (`config.go:166-177`, `cmd/ha2fabric/warnings.go:42-54`): HR-P0-ha-23
  is fixed as decided (17). Residuals: the Supervisor gate's client is
  the one token-carrying client that follows redirects (`HR-P1-ha-13`,
  note); all three clients take a proxy from the environment
  (`HR-P1-ha-14`, note). In the add-on a hand-set `HA_URL` would get
  `SUPERVISOR_TOKEN` (`config.go:270-280`); `run.sh` sets none, and
  `TokenInClear` warns (no row).
- **Read limits** (M). 64 MiB per frame (`client.go:54`, `:220`); a frame
  that is not a message is logged with its size only (`:331`); a still
  64 000 bytes; the Supervisor's answer 1 MiB (`supervisor.go:169`).
  T16's consequence (a `get_states` over the limit) meets `HR-P1-ha-07`.
- **Reconnect and back-off**. Doubling from 1 s to 60 s, but reset by
  every authentication (`client.go:167-190`): `HR-P1-ha-07` (low). A
  refused token is retried for ever: `HR-P1-ha-08` (low). A write's
  context ends the whole connection: `HR-P1-conc-22` (note).
- **Decode of hostile results** (`message.go`) (M). `Decode` takes an
  object or an array, refuses an empty array, a message without a type
  and a negative id (`:76-108`; fuzz target `FuzzDecode`); an event for
  an id that is no subscription is dropped (`client.go:348-350`); a
  second result for one id finds no waiter (`:358-364`). `Tuple` decodes
  an identifier of any JSON type (`message.go:260-285`). The other
  string fields are strict, and one wrong type fails the whole list:
  `HR-P1-data-09` (low).

### 1.6 `config`

Every variable against its check (M, `internal/config/config.go`):

| Variable | Check | Manifest and `run.sh` |
| --- | --- | --- |
| `HA_URL` | `checkHAURL` `:407-428`; required outside the add-on `:306-310`; `-h` shows no default `:230` | not set by `run.sh`; the add-on's default `:278-280` |
| `HA_TOKEN`, `SUPERVISOR_TOKEN` | environment only, required `:267-273`, `:311-313`; never in an issue text (test `config_test.go:173`) | `with-contenv` (`run.sh:1`) |
| `LISTEN` | `host:port`, port 1..65535 `:314-316`, `:439-449` | `:$(port)` (`run.sh:16`), schema `port` |
| `PASSCODE` | unset → 0 → the random one; decimal, `ValidateSetupPIN` `:317-325`; `-h` shows none `:236`, `:262-264` | exported only when set (`run.sh:21-23`); schema `int(1,99999998)?`, no default (`addon/config.yaml:71`) |
| `DISCRIMINATOR` | 0..4095, base 0 `:326-330` | `int(0,4095)` |
| `VENDOR_ID`, `PRODUCT_ID` | 1..0xFFFF, base 0 `:430-437` | `int(1,65535)` |
| `LABEL` | not empty, at most 32 bytes `:333-339` | `str`, unbounded (`HR-P1-cfg-21`) |
| `STORE_PATH` | its directory exists `:340-344` | `/data/ha2fabric.db` (`run.sh:30`) |
| `INCLUDE`, `EXCLUDE` | `expose.Rule.Validate` `:345-347` | one per line; split on newline, comma, blank |
| `INCLUDE_LABELS`, `EXCLUDE_LABELS` | split on comma and newline `:512-520` | one per line (`HR-P1-cfg-21`: a comma in a name) |
| `EXPOSE_ASSIST`, `WEB_ADMIN`, `CAMERA` | `ParseBool` `:348-362` | `bool`; `WEB_ADMIN` not set in the add-on |
| `NODE_LIMIT` | 0..32 `:363-367` | `int(0,32)`, guarded (`run.sh:62-64`) |
| `LOG_LEVEL` | `slog.Level.UnmarshalText` `:368-370` (also takes `INFO+2`; harmless) | `list(debug\|info\|warn\|error)` |
| `MDNS_INTERFACES` | no empty or repeated name `:491-508` | guarded (`run.sh:67-71`) |
| `WEB_LISTEN` | `host:port` `:371-375`; empty disables | `0.0.0.0:8486` (`run.sh:77`) = `ingress_port` (`config.yaml:44`) |
| `WEB_ALLOW` | each an IP address `:376-380`; in the add-on exactly the proxy `:296-300`, `:381-383` | `172.30.32.2` (`run.sh:78`) |
| `WEB_ALLOWED_HOSTS` | go-fabric-ui's rules, at start `:392-396`, `:470-484` | — |

- **The passcode on first start (decision 14)** holds (M): `config.Load`
  leaves 0 (`:317-318`), `run` draws or reads it before any node is built
  (`main.go:101-126`), `--dump-plan` returns before (`main.go:84-86`), a
  file that cannot be kept is a warning, not an exit
  (`main.go:106-116`), `20202021` set explicitly warns in the log and on
  the status page (`warnings.go:33-41`). The draw is uniform over
  1..99 999 998 and re-drawn until valid (`nodes.go:389-400`).
- **`WEB_ADMIN` refusal (decision 15)** holds (M, `:389-391`; test
  `config_test.go:297`). It judges the listen address as text
  (`HR-P1-cfg-15`, note).
- `run.sh` and the manifest agree with the checks; two literals of the
  proxy's address remain (`run.sh:78`, `config.go:70`), and the daemon
  refuses a mismatch. `run.sh` logs no secret (`:80`).

### 1.7 Lengths and encodings of entity text in S's files (T15)

S's files handle entity text in four places (M): the log (slog's text
handler quotes, `internal/web/operation.go:19-22`); the ring's attributes
as plain strings (`:81-87`), rendered by go-fabric-ui; the selection's
`Name`, stored as given, of any length, under any key (`selection.go:104-106`,
`:156-169`; probe S5: `entries=2, name bytes=100000`) — the cut to
NodeLabel's 32 bytes is not in S's files (unit G-A, item 5); and the
camera's SDP and candidates (1.2). `LABEL` is checked for 32 bytes, not
for valid UTF-8 or control characters; it is the operator's. One note,
`HR-P1-data-20`.

### 1.8 The images

| | Plain image (`Dockerfile`) | Add-on image (`addon/Dockerfile`) |
| --- | --- | --- |
| User | `nonroot:nonroot` on distroless static (`:27`, `:33`); `/data` owned by it (`:29`) | root: no `USER` (`:34-44`), on the host's network (`addon/config.yaml:37`), the Supervisor's default profile (no `apparmor` key, no profile file) |
| Ports | `EXPOSE 5540/udp 8099/tcp` (`:32`); with `CAMERA` the node also listens on 5540/tcp, which is not named; the web default is every interface (`:31`, 1.3) | `ingress_port: 8486`; no `ports:` key, so nothing is published beyond the host network |
| Copied | the binary and an empty `/data` (`:28-29`); the build stage takes the whole context (`COPY . .`, `:14`) and there is no `.dockerignore` | the binary and `run.sh` (`:35-37`); the same `COPY . .` (`:20`) |
| Healthcheck | none; distroless has no shell, so one needs a mode of the binary | none; no `watchdog:` in the manifest |
| Base | by tag, not digest (`:10`, `:27`) | `BUILD_FROM`, by tag (`:8`, `:34`) |

`HR-P1-sup-18` (low: root, no profile, no watchdog) and `HR-P1-sup-19`
(note: `.dockerignore`, healthcheck, the TCP port, digests — the last is
phase 3's).

## 2. Findings

Held rows by ID and severity only.

### HR-P1-auth-01 — medium (I) — held

`held/phase-1-S.md`.

### HR-P1-bound-02 — low — no bound on the sessions of one camera

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

### HR-P1-conc-03 — low — a session whose controller is gone never ends

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

### HR-P1-ha-04 — low — merged into HR-P1-ha-62 (verifier; one defect with G-B's row) — an offer whose wait ends early leaves Home Assistant's session

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

### HR-P1-bound-17 — low — stills and candidates: one Home Assistant request per controller request

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

### HR-P1-err-05 — low — a failed re-offer drops the session and tells nobody

- **Where:** `internal/camera/camera.go:329-344`.
- **Claim:** `ProvideOffer` takes the held session of the id out of the
  map and closes its Home Assistant session before it opens the new one.
  When the new subscription fails, the host has no session of the id,
  the kit still lists it (the port's error leaves the table as it was,
  `gf:cluster/camera/provider.go:557-560`), and the controller is not
  sent `End`: its stream is dead, CurrentSessions says it is not, and
  its candidates answer "no session" until it ends the session itself.
- **Reference:** the kit's contract — the port's error is the command's,
  the session of a re-offer stays (`gf:provider.go:495-501`).
- **M:** probe `internal/camera/probe_s_test.go` `TestProbeSReofferFails`:
  `PROBE S2: re-offer err=context deadline exceeded; first stream unsubscribed=1; candidates afterwards err=camera: no WebRTC session of the id: 7; signals to the controller=[]`.
- **Fix:** on a failed re-offer end the session properly: `sig.End(id,
  ReasonICEFailed)`, which also takes it out of CurrentSessions. (Keeping
  the old Home Assistant session until the new one stands would be wrong:
  the controller already replaced its offer.)
- **Red test:** as the probe, asserting `signaller.seen()` holds the
  `End` of session 7.

### HR-P1-err-06 — low — a selection file that is moved aside takes the "hidden" overrides with it

- **Where:** `internal/selection/selection.go:89-103`; `cmd/ha2fabric/main.go:76-80`.
- **Claim:** see 1.4. A file that does not parse, or names another
  `version`, is moved aside and the node runs on the configured rule
  alone. Overrides with `"enabled": false` are how an operator keeps an
  Assist-exposed lock, cover or scene away from the controllers; they are
  lost with the rest, and every commissioned controller gets those
  entities at the next sync. The direction is fail open. The file is
  written atomically, so damage is rare; a file of a later version met
  by an older daemon (a downgrade, a restored backup) is the certain
  case.
- **Reference:** `nodes.json` beside it fails closed (`nodes.go:111-115`).
- **M:** probe `internal/selection/probe_s_test.go` `TestProbeSHidesLost`:
  `version 2: overrides after Open=0, recovered=true`, the same for
  `truncated` and `no version`.
- **Fix (owner question 2):** recommended — while a selection is
  "recovered", the node keeps the exposed set it had: entities that hold
  an endpoint number in the node's store stay, nothing is added, and the
  status warning says so until an administrator saves a selection (or
  dismisses the state). It neither exposes what was hidden nor removes
  what a controller has rooms and automations for. Where that set cannot
  be had cheaply (unit G-A's knowledge), the fallback is to read a file
  of another version leniently (`entities` in the known shape is
  honoured, with the warning) and to keep "moved aside, rule only" for a
  file that is not JSON.
- **Red test:** a version-2 file with `{"lock.front_door": {"enabled":
  false}}` over a snapshot where Assist exposes the lock: the plan after
  `Open` does not expose `lock.front_door`.

### HR-P1-ha-07 — low — the back-off starts again at every authentication

- **Where:** `internal/ha/client.go:170-188`.
- **Claim:** `authed` resets the delay to `MinBackoff` and stops the
  doubling. A connection that authenticates and then fails — a
  subscription Home Assistant refuses, a first frame over the read limit
  (T16), a proxy that drops after the handshake — is retried once a
  second for ever, each round an authentication, the subscriptions and
  possibly a `get_states` of megabytes, with a warn line each. No jitter.
- **Reference:** the doc comment's intent, "starts again at MinBackoff
  after a connection that authenticated" (`:163-166`) — meant for a
  connection that worked.
- **M:** probe `internal/ha/probe_s_test.go` `TestProbeSBackoff`:
  `auth_ok then drop: 128 authentications in 3 s (MinBackoff 20 ms, MaxBackoff 320 ms), last gap 20ms`.
- **Fix:** reset only after a connection that stayed up for a while
  (proposal: longer than `MaxBackoff`), otherwise keep doubling; add
  jitter of about a fifth.
- **Red test:** the probe's server; the gaps between authentications
  must grow to `MaxBackoff`.

### HR-P1-ha-08 — low — a refused token is retried for ever

- **Where:** `internal/ha/client.go:167-190`, `:301-302`.
- **Claim:** `auth_invalid` is a failed attempt like any other: retried
  every `MaxBackoff` (60 s) without end. Each one is a failed login in
  Home Assistant: a warning in its log, the "Login attempt failed"
  notification, and with `ip_ban_enabled` the daemon's address is banned
  at the threshold (`core:homeassistant/components/http/ban.py:111-170`;
  the Supervisor's address is exempt, `:153-155`, so the add-on is
  spared the ban, not the notifications). Stand-alone on Home Assistant's
  own host the banned address is the host's. The token is the
  environment's and cannot change while the process runs; only the
  add-on's can become valid again (a Supervisor restart).
- **M:** probe S6: `auth_invalid: 13 authentications in 3 s (MinBackoff
  20 ms, MaxBackoff 320 ms), last gap 320ms` — steady at `MaxBackoff`.
  The ban itself is core's code as read, not run (I).
- **Fix:** after `auth_invalid` a status warning (`ha_token_refused`) and
  a long back-off of its own (5 min doubling to 1 h); stand-alone, after
  three in a row, stop trying until restart and say so. No exit: the
  paired node goes on serving its last state.
- **Red test:** `hatest` with a wrong token: at most three
  authentications within the test's window at `MaxBackoff` 20 ms, and
  the warning is set.

### HR-P1-data-09 — low — one wrong-typed field fails a whole registry list

- **Where:** `internal/ha/message.go:189-209`, `:224-249`, `:289-292`;
  `internal/ha/client.go:494-496`.
- **Claim:** the registry lists decode into structs with strict string
  fields. One entry whose `name` (device or entity), `original_name` or
  area name is a number makes `json.Unmarshal` fail for the list, the
  snapshot read fails (`internal/ha/snapshot.go:41-52`), and no node
  syncs until the entry is gone. The code already knows the class:
  `Tuple` exists because the registry does not validate identifier types
  (`message.go:238-257`). Core converts six device fields to text
  (`core:homeassistant/helpers/device_registry.py:314-340`:
  `hw_version`, `manufacturer`, `model`, `model_id`, `serial_number`,
  `sw_version`), not `name` (`:414-415`, `attr.ib` without a validator).
  That an integration can get a non-string name into the registry is I.
- **M:** probe `internal/ha/probe_s2_test.go`
  `TestProbeSDecodeAllOrNothing`:
  `devices: err=json: cannot unmarshal number into Go struct field .1.name of type string`;
  the same for an entity's `original_name`.
- **Fix:** a lenient text type for the registry's free-text fields
  (string as itself, null as "", anything else as its JSON text), as
  `Tuple` does; or decode the list entry by entry and skip, with a
  warning, the one that does not decode. The second is the better
  design: one bad entry costs one entity, whatever the field.
- **Red test:** the probe's lists through `hatest`: the snapshot holds
  the two good devices.

### HR-P1-cfg-10 — low — the mode of a secret file that is already there is not checked

- **Where:** `internal/nodes/nodes.go:121-142`; `internal/nodes/passcode.go:50-69`.
- **Claim:** 0600 is the mode of what the daemon writes. A `nodes.json`
  or `passcode.json` that was put there otherwise — restored from a
  backup, copied between hosts — is read as it is and stays readable by
  whoever its mode says until the next write replaces it; `passcode.json`
  is never written again. A file moved aside as corrupt keeps its mode
  and still holds the passcodes. Stand-alone on a shared host a local
  user reads the passcodes of nodes whose window is open; in the add-on
  `/data` is the container's.
- **M:** probe `internal/nodes/probe_s_test.go` `TestProbeSFileModes`:
  `S3b: after Open of 0644 files: nodes.json -rw-r--r--, passcode.json -rw-r--r-- (passcode read 34567890 created=false err=<nil>, nodes=1)`;
  `S3c: aside 1 file(s), mode -rw-r--r--`.
- **Fix:** at `Open` and in `ConfiguredPasscode`, `os.Chmod(path, 0o600)`
  when the mode is wider (and a log line), before and after a move aside.
- **Red test:** the probe with assertions.

### HR-P1-cfg-11 — note — `nodes.json` entries the check lets through

- **Where:** `internal/nodes/nodes.go:147-164`.
- **Claim:** `check` looks at the key, a non-empty listen address, the
  discriminator's width and the passcode. It accepts a listen value that
  is no address, an empty or over-long label, two nodes on one port or
  discriminator, and more nodes than `NODE_LIMIT`. The file is the
  daemon's own and 0600; what a start makes of such an entry is unit
  G-B's.
- **M:** probe `TestProbeSRegistryCheck`: `S4: nodes accepted=3`.
- **Fix:** `net.SplitHostPort` and a port range on `Listen`, unique
  ports and discriminators, a label of 1..32 bytes after `nodeLabel`'s
  cut; the count against the limit where the fleet starts them.
- **Red test:** the probe's file is moved aside.

### HR-P1-cfg-12 — low — the image answers the open routes to the whole network

- **Where:** `Dockerfile:31`; `internal/web/web.go:91-94`.
- **Claim and recommendation:** 1.3. Among the public fields the warning
  codes say which of the daemon's own warnings are in force.
- **M** (the routes, `ui:` lines in 1.3).
- **Fix:** remove `WEB_LISTEN` from the image's `ENV`; `README.md:120-124`,
  `docs/operations.md:95` and `changelog.md` say that the image's web UI
  is on loopback unless `WEB_LISTEN` is set. A behaviour change for
  image users.
- **Red test:** a test that reads `Dockerfile` and fails on a `WEB_LISTEN`
  that is not loopback (the repository has such file guards in
  `make version-check`'s style), or none and the docs line.

### HR-P1-ha-13 — note — the Supervisor gate's client follows redirects

- **Where:** `cmd/ha2fabric/supervisor.go:84`, `:154-155`.
- **Claim:** the gate sends `SUPERVISOR_TOKEN` as a Bearer header with a
  default `http.Client`; the two Home Assistant clients refuse redirects
  for this reason (HR-P0-ha-31, -56). The Supervisor is trusted and Go
  drops the header across hosts (I, `net/http`'s rule), so a note.
- **Fix:** `CheckRedirect` that refuses, as `internal/ha/client.go:150`.
- **Red test:** a fake Supervisor that answers 302 to a second server:
  the second server sees no request.

### HR-P1-ha-14 — note — the three token-carrying clients take a proxy from the environment

- **Where:** `internal/ha/client.go:141`, `:158`; `cmd/ha2fabric/supervisor.go:84`
  (each on `http.DefaultTransport`).
- **Claim:** `HTTP_PROXY`/`HTTPS_PROXY` in the daemon's environment route
  the WebSocket handshake, the stills and the Supervisor call through
  that proxy (I: `http.ProxyFromEnvironment`, not run); for `ws://` and
  `http://supervisor` the token is then the proxy's to read. The
  operator's own setting; the docs do not mention it, and the gate's
  call to `supervisor` should never leave the host.
- **Fix:** the gate's transport with `Proxy: nil`; one sentence in
  `docs/operations.md` ("The token") for `HA_URL`.

### HR-P1-cfg-15 — note — "loopback" is judged on the text of `WEB_LISTEN`

- **Where:** `internal/config/config.go:453-463`, `:389-391`;
  `internal/web/web.go:116-121`.
- **Claim:** `localhost:8099` counts as loopback by name, and the
  listener then binds what the resolver answers for the name. The
  decision-15 refusal is about the address that is bound.
- **Fix:** accept only a literal loopback address as loopback in
  `config`, or check `ln.Addr()` in `web.Run` and refuse there.
- **Red test:** `WEB_ADMIN=true WEB_LISTEN=localhost:8099` without
  `WEB_ALLOW` is refused (or: accepted and documented as resolving
  through the hosts file).

### HR-P1-test-16 — low — no test holds the admin table at the daemon's wiring

- **Where:** `cmd/ha2fabric/web_test.go:20-52` (predicates only);
  `internal/web/web_test.go:93-132` (`TestHost` builds the add-on with
  `Admin: From(...)`, not `IngressAdmin`); `:183-209` (the predicate).
- **Claim:** the tests show the predicate's truth table and one admin
  write through the proxy. No test sends each kind of client against the
  admin routes through the handler as `run` builds it (filter,
  `admin(cfg, gate)`, `allowHost`), so a later wiring change — a route
  mounted beside go-fabric-ui's, a predicate swapped — has nothing red.
  Also missing in S's files: the negative cases of the camera rows, a
  file's mode when it pre-exists
  (`HR-P1-cfg-10`), a back-off that grows after an authenticated failure
  (`HR-P1-ha-07`; `client_test.go:194-226` cover reconnect and
  `auth_invalid` once), a re-offer that fails (`HR-P1-err-05`).
- **Fix:** probe S8 as a standing test in `cmd/ha2fabric`, the route list
  taken from go-fabric-ui's contract (`contract/openapi.yaml`, every
  `x-admin` operation) rather than typed, so a new admin route joins by
  itself.
- **Red test:** it is the test; it is green today (a guard, as
  HR-P0-test-06 was).

### HR-P1-sup-18 — low — the add-on runs as root, without a profile or a watchdog

- **Where:** `addon/Dockerfile:34-44`; `addon/config.yaml` (no
  `apparmor`, no `watchdog`); `script/run.sh:82`.
- **Claim:** T25, owed here by item 8. The daemon needs no privilege: its
  ports are above 1024, mDNS binds 5353 with `SO_REUSEPORT`, `/data` is
  its own. It runs as root on the host's network because nothing says
  otherwise. Without `watchdog` a daemon that hangs is not restarted;
  without a profile the container has Docker's default one.
- **M** (the files); that the node runs unprivileged in the add-on's base
  image is I (the plain image does, `Dockerfile:33`).
- **Fix:** in `run.sh`, `chown` `/data` once and `exec s6-setuidgid` to an
  unprivileged user; an `apparmor.txt` with the network, `/data` and the
  binary; `watchdog: tcp://[HOST]:[PORT:8486]`. For the owner's live
  test before it merges (Ingress, pairing, the store's ownership after an
  update from a root-owned `/data`).
- **Red test:** none in Go; `make addon-image` plus a `docker run … id -u`
  check in the image workflow.

### HR-P1-sup-19 — note — image details

- **Where:** `Dockerfile:14`, `:27-32`; `addon/Dockerfile:8`, `:20`.
- **Claim:** (a) no `.dockerignore`: `COPY . .` puts every untracked file
  of a local build context (notes, `held/`, an `.env`) into the build
  stage and its cache — not into the final image; (b) no `HEALTHCHECK`
  in the plain image; (c) `EXPOSE` omits 5540/tcp, which `CAMERA` opens
  (`internal/config/node.go:44`); (d) base images by tag (phase 3's
  supply unit).
- **Fix:** a `.dockerignore` (`.git`, `notes`, `held`, `dist`, `*.db`);
  `ha2fabric --health` (a GET of the local status route) as the
  healthcheck's command; `EXPOSE 5540/tcp`.

### HR-P1-data-20 — note — entity text in S's files

- **Where:** `internal/selection/selection.go:104-106`, `:156-169`;
  `internal/camera/camera.go:518-519`; `internal/config/config.go:333-339`.
- **Claim:** 1.7. The selection stores a name of any length under any
  key and reads back whatever the file holds; an empty Home Assistant
  `answer` goes to the controller as an empty SDP; `LABEL` is checked for
  bytes, not for UTF-8 or control characters.
- **Fix:** `selection.Set` and `Open` drop or refuse a key that is not an
  entity id (`domain.object_id`) and bound `Name` (to what the plan
  keeps); an empty answer ends the session as Home Assistant's error
  does; `LABEL` must be valid UTF-8 without control characters.
- **Red test:** per line, one table case each.

### HR-P1-cfg-21 — note — two edges between the manifest and the checks

- **Where:** `internal/config/config.go:512-520`, `:333-339`;
  `addon/config.yaml:75`.
- **Claim:** (a) a Home Assistant label whose name holds a comma cannot
  be named in `INCLUDE_LABELS`/`EXCLUDE_LABELS` — it splits in two, and
  each half is reported as unresolved; the label id works. (b) the
  manifest takes any `label`; the daemon refuses one over 32 bytes at
  start, so the add-on does not start and the reason is in its log only.
- **Fix:** (a) a sentence in `addon/DOCS.md` and `docs/operations.md`
  ("use the label id for a name with a comma"); (b)
  `label: "match(^.{1,32}$)"` in the schema catches most of it in the
  form (characters, not bytes; the daemon's check stays).

### HR-P1-conc-22 — note — a caller's context can end the Home Assistant connection

- **Where:** `internal/ha/client.go:426-430`.
- **Claim:** `callWith` writes with the caller's context, and
  coder/websocket closes the connection when a write's context ends
  while the write is in progress (`ws:conn.go:171-182`). One command
  whose context is cancelled at that moment — a controller's invoke, a
  timeout under back-pressure — drops the one connection every node
  shares; all of them resync after the reconnect. The window is a frame
  write, so rare (I: not reproduced).
- **Fix:** write on `context.WithoutCancel(ctx)` bounded by
  `CallTimeout`; the wait for the result keeps the caller's context.
- **Red test:** a server that stops reading; a `Call` with a 10 ms
  context returns its error and `Client.Connected()` stays true.

## 3. Phase 0 rows re-read

No phase 0 fix was found wrong. HR-P0-auth-20, -50, -51: the check is as
1.1 states it (M, the lines there; tests `web_test.go:183-209`,
`supervisor_test.go:18-127`, `web_test.go` of `cmd`). HR-P0-cfg-21, -22,
-30, -54, err-26, ha-23, -31, -56, leak-25, -29: as 1.5 and 1.6 (M).
HR-P0-bound-28: the 64 candidates before the session event hold
(`camera.go:399-404`, test `camera_test.go:315`); what it left to phase 1
is `HR-P1-bound-02` and its companions. HR-P0-cfg-27 stays open by decision 22.

## 4. Counts

| Severity | Count | IDs |
| --- | --- | --- |
| high | 0 | — |
| medium | 1 | HR-P1-auth-01 (held) |
| low | 13 | HR-P1-bound-02, -conc-03, -ha-04 (merged into -ha-62), -bound-17; HR-P1-err-05, -err-06, -ha-07, -ha-08, -data-09, -cfg-10, -cfg-12, -test-16, -sup-18 |
| note | 8 | HR-P1-cfg-11, -ha-13, -ha-14, -cfg-15, -sup-19, -data-20, -cfg-21, -conc-22 |

`upstream` candidates (theme `adpt`): two facade questions to go-fabric,
inside `HR-P1-bound-02` and `HR-P1-conc-03`.

## 5. Not read, and why

- `internal/ha/subscription.go`, `snapshot.go`, `fanout.go`: read only as
  far as the camera relay and the decode path need them (unit G-B's).
- `cmd/ha2fabric/nodes.go`: the pairing log line and the label cut only
  (G-B's); how a start treats the registry entries of `HR-P1-cfg-11` was
  not followed.
- `internal/exposure`, `internal/plan`: the selection's consumers only
  (G-A's); whether the "previously exposed set" of `HR-P1-err-06`'s fix is
  cheap to have was not established.
- go-fabric beyond `cluster/camera/provider.go`, `provider_peer.go`,
  `fabric_state.go`, `store/sqlite.go`, `node/facade.go:149-154`: how the
  node cancels an invoke's context, whether CameraAvStreamManagement's
  invokes are serialised, and the kit's outgoing size bound were not
  read; each is marked I where a row rests on it.
- go-fabric-ui beyond the route table, the admin and Origin tests, the
  status and operation handlers: its pages and its stream cap are its
  own review's.
- coder/websocket: `conn.go:171-196` only. Go's `net/http` redirect and
  proxy rules, SQLite's side-file modes, Linux networking:
  not read at a pin (I).
- Supervisor: `api/ingress.py:40-70`, `:164-200`,
  `:400-426` at `fe91b8db`; its history is in phase 0's held reports and
  was not re-read.
- Home Assistant core: `camera/webrtc.py:240-350`,
  `websocket_api/auth.py:80-130`, `http/ban.py:111-170`,
  `helpers/device_registry.py:300-340`, `:400-420`, `:513-541`, and the
  go2rtc provider by `grep` only. Nothing was run (decision 12).
- Docs (`docs/`, `addon/DOCS.md`, `README.md`) against the code: phase 3;
  read here only where a row cites them.
- No live run: the Ingress path, T2 and HR-P1-auth-01 are the owner's
  live test and X1.

## 6. Probes

All in `…/scratchpad/S/src` (a `git archive` of `5eefc0a`), run with
`go test ./internal/<pkg>/ -run TestProbeS -v -count=1`; all passed (a
probe logs, it does not assert).

| Probe | File | Output line |
| --- | --- | --- |
| S1 | `internal/camera/probe_s_test.go` `TestProbeSSessionsPerCamera` | under `HR-P1-bound-02` |
| S2 | the same file, `TestProbeSReofferFails` | `re-offer err=context deadline exceeded; first stream unsubscribed=1; candidates afterwards err=camera: no WebRTC session of the id: 7; signals to the controller=[]` |
| S3 | `internal/nodes/probe_s_test.go` `TestProbeSFileModes` | `S3a: nodes.json -rw-------, passcode.json -rw-------, selection.json -rw-------`; S3b and S3c under `HR-P1-cfg-10` |
| S4 | the same file, `TestProbeSRegistryCheck` | `nodes accepted=3 (a 200-byte label, a listen that is no address, an empty label, a repeated port and discriminator)` |
| S5 | `internal/selection/probe_s_test.go` `TestProbeSHidesLost` | `version 2: overrides after Open=0, recovered=true` (and `truncated`, `no version`); `keys: entries=2, name bytes=100000` |
| S6 | `internal/ha/probe_s_test.go` `TestProbeSBackoff` | `auth_ok then drop: 128 authentications in 3 s (MinBackoff 20 ms, MaxBackoff 320 ms), last gap 20ms`; `auth_invalid: 13 authentications in 3 s …, last gap 320ms` |
| S7 | the same file (second probe) | under `HR-P1-ha-04` |
| S8 | `internal/web/probe_s_test.go` `TestProbeSAdminTable` | the block in 1.1; `open /api/v1/node/status -> 200 …`, `open /api/v1/nodes -> 200 {"nodes":[]}` |
| S9 | `internal/ha/probe_s2_test.go` `TestProbeSDecodeAllOrNothing` | `devices: err=json: cannot unmarshal number into Go struct field .1.name of type string`; `answer: err=<nil> answer="v=0\r\n\x00�"` |

## 7. Questions for the owner

1. **T6, the image's default listener** (`HR-P1-cfg-12`). Bind or accept?
   **Recommendation: bind** — remove `WEB_LISTEN=0.0.0.0:8099` from the
   image so that image and binary share the loopback default. It is the
   best design (one default, T5 and T6 closed by one line); it changes
   behaviour for image users, who set `WEB_LISTEN` and `WEB_ALLOW`
   themselves.
2. **A selection file moved aside** (`HR-P1-err-06`). **Recommendation:
   hold the exposed set** — while the selection is "recovered", keep what
   was exposed, add nothing, warn until an administrator saves a
   selection. It is the best design because both simple answers do harm:
   "rule only" (today) exposes what was hidden, "expose nothing" removes
   devices a controller has rooms and automations for. If unit G-A finds
   the previous set is not cheaply at hand, the fallback is a lenient
   read of another version plus today's behaviour for a file that is not
   JSON — better than today, not the best.
3. **`HR-P1-auth-01` (held, medium, inferred).** The held file puts
   three steps, in an order. Recommendation: all three, in that order;
   the row stays held until the first is agreed, and X1 (phase 2) or the
   owner's live test owes the measurement.
4. **The camera's bounds** (`HR-P1-bound-02`, `-conc-03`, `-bound-17`).
   The numbers proposed in those rows are behaviour a
   controller can observe. Recommendation: take them as proposed and raise the two
   facade questions at go-fabric as issues; the host's bounds do not wait
   for them. Should these three lows stay held until their fix merges, or
   be published as HR-P0-bound-28 was (decision 18's reasoning)?
   Recommendation: held until the fix. *Owner: published now (decision 23).*
5. **A refused token** (`HR-P1-ha-08`). Stop trying after three
   `auth_invalid` in a row stand-alone, or only slow down?
   **Recommendation: slow down everywhere (minutes, then an hour) with a
   status warning, and stop stand-alone after three** — the token cannot
   change without a restart there, and each try is a failed login in Home
   Assistant.
6. **The add-on as an unprivileged user** (`HR-P1-sup-18`).
   Recommendation: yes, with a profile and a watchdog, as a change of its
   own after the live Ingress test (it touches `/data`'s ownership on
   update), not inside phase 1's fix wave.
