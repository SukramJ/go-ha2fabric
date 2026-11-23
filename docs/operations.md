# Operations

What an operator of go-ha2fabric needs: the files it keeps, the ports and
addresses, mDNS, the token, the logs, the health endpoint, pairing and the
nodes created in the web UI. The
node's side of each is go-fabric's
[operations guide](https://github.com/thingfabric/go-fabric/blob/v0.25.0/docs/guides/operations.md);
this page is the daemon's version. Terms such as fabric, commissioning
window, bridged node and served surface are defined in go-fabric's
[glossary](https://docs.thingfabric.dev/reference/glossary/).

## Files

The daemon keeps two files for its configured node: the node's SQLite
store, `STORE_PATH`, and beside it `selection.json`, the web UI's
per-entity overrides. Each node created in the web UI (Nodes, below) adds
its pair in the same directory, `ha2fabric-<key>.db` and
`selection-<key>.json`, and `nodes.json` lists those nodes. Without `PASSCODE`, `passcode.json`
beside them holds the configured node's random passcode.

| Where | Path |
| --- | --- |
| Add-on | `/data/ha2fabric.db`, the add-on's persistent data directory (`script/run.sh`) |
| Daemon image | `/data/ha2fabric.db`, the image's volume (`Dockerfile`) |
| Stand-alone | `ha2fabric.db` in the working directory, unless `STORE_PATH` says otherwise |

The store holds the fabrics with their operational keys, the access control
lists, the subscriptions to re-establish and the endpoint numbers of the
bridged entities. Losing it unpairs the bridge from every controller, and
the entities are numbered afresh from the first bridged number (2), as on
a new installation. Keep it on storage that
survives restarts and upgrades, protect it like a key (it holds private
keys unencrypted), back it up only with the daemon stopped (the add-on
sets `backup: cold`, so the Supervisor stops it for a backup), and never
run two copies of one store.

**The upgrade to 0.7.0 is one-way.** go-fabric v0.25.0 upgrades the store's
schema from version 7 to 12 at the first start, and an older daemon is not
expected to open the upgraded file. Take a backup before the update; to go
back to 0.6.x, restore the add-on together with its data from that backup
rather than installing the older version over the upgraded store.

A store that has lost its fabrics shows at start as `ha2fabric.fabrics
count=0` with the commissioning window open (`commissioning.own_window.open`),
although the entities are still listed. Restoring the add-on from a backup
taken while it was paired brings the store and with it the pairing back; the
entities keep their endpoint numbers. Pairing again instead gives the bridge
a new fabric, and every controller sees a new device.

`selection.json` is `{"version": 1, "entities": {"<entity_id>": {"enabled":
true, "name": "…"}}}`, written atomically (a temporary file, then a rename)
and read at start; before each write the file it replaces is kept as
`selection.json.bak`, the last-good copy. A file of another version (a
restored backup, a downgrade) is read leniently: its entities in this
shape apply, the rest is ignored, and the log (`selection.version`) and
the status page (`selection_corrupt`) say so until the next start; saving
the selection rewrites it as version 1; when nothing in it is understood
(`{}`, `null`), the warning says the overrides started empty. A file that
does not parse is renamed to `selection.json.corrupt-<unix seconds>`,
logged (`selection.corrupt`), and the last-good copy is read instead and
written back as `selection.json`, which loses at most the last change; a
missing `selection.json` beside a last-good copy is restored the same way
(`selection.missing`). Only when that does not parse either does the
configured node start without overrides — and then entities turned off in
the web UI are exposed again wherever `EXPOSE_ASSIST`, `INCLUDE` and the
labels expose them; the status warning says so. A web UI node's rule is
its selection alone, so without overrides it exposes nothing. Keys that are
not entity ids are ignored and names longer than 256 bytes cut. The
add-on's cold backup holds the file with the store. An override keyed by
an entity id does not follow a rename of that id.

`nodes.json` is the node registry, `{"nodes": [{"key", "label",
"listen", "discriminator", "passcode", "created_at"}]}`: written
atomically with mode 0600 — it holds the nodes' passcodes, protect it like
the store — and read at start. A file that does not parse, or holds a node
whose key, address, discriminator or passcode is unusable, is renamed to
`nodes.json.corrupt-<unix seconds>`, logged (`nodes.corrupt`), and the
daemon starts with its configured node only; the nodes' files stay, and no
new node takes a key whose files exist, so restoring the file brings them
back. A `nodes.json` or `passcode.json` that is there with a wider mode —
restored from a backup, copied from another host — is set to 0600 when it
is read, and so is one moved aside as corrupt (`nodes.mode_tightened`).
On Windows, which has no POSIX file modes, the ACL of the data
directory protects it, as it does the store: that is the service's
concern.

`passcode.json` is `{"passcode", "created_at"}`, written the same way
(atomically, 0600) on the first start without `PASSCODE`. One that does not
parse or holds an invalid passcode is renamed to
`passcode.json.corrupt-<unix seconds>`, logged (`nodes.passcode_corrupt`),
and a new one is drawn; a paired node keeps its fabrics, since the passcode
opens only the node's own window, which opens without a fabric. One that
cannot be read, moved aside or written does not stop the daemon: the node
runs on a random passcode for that start only, logged at error
(`ha2fabric.passcode`) and shown as the status warning `passcode_not_kept`,
so an unpaired node's codes change at the next start until the directory
is fixed or `PASSCODE` set.

The daemon stores nothing else: not the token, not Home Assistant's
registry, not the states. Everything it knows about Home Assistant it reads
again on every connection.

## Ports and addresses

| What | Default | Notes |
| --- | --- | --- |
| UDP `LISTEN` | `:5540` | Every Matter message. The add-on's `port` option sets it; change it when another Matter node on the same host uses 5540. |
| UDP each node's `listen` | the next free port above `LISTEN`'s | A node created in the web UI (Nodes, below); the add-on's host networking needs no mapping. |
| TCP `LISTEN` | off; on with `CAMERA` | Matter over TCP on the same port, for a camera's signals (Cameras, below; go-fabric `docs/guides/operations.md:54, 58` at v0.15.5). |
| UDP 5353, `224.0.0.251`, `ff02::fb` | — | mDNS, below. |
| IPv6 on the LAN | required | Matter's operational network is IPv6. |
| TCP `WEB_LISTEN` | `127.0.0.1:8099`, the image's too; add-on `0.0.0.0:8486` | The web UI. Empty disables it. The image keeps it on loopback: `-e WEB_LISTEN=0.0.0.0:8099` (with `WEB_ALLOW` for `WEB_ADMIN`) opens it to the network. Reached by a DNS name, the name must be in `WEB_ALLOWED_HOSTS` (The web UI, below). |
| Outbound WebSocket | `HA_URL`; add-on `ws://supervisor/core/websocket` | Home Assistant's WebSocket API. |
| Outbound HTTP | derived from `HA_URL` | The camera proxy's stills, only with `CAMERA` (Cameras, below). |

**Host networking.** Controllers find the bridge over mDNS and talk to it
over UDP on the LAN, so the daemon needs the host's network: the add-on
sets `host_network: true`, the image runs with `--network host`. A
consequence for the add-on: its web UI listens on the host too. Home
Assistant's Ingress requires that only its proxy reaches it
([add-on presentation, "Ingress"](https://developers.home-assistant.io/docs/apps/presentation#ingress):
"Only connections from `172.30.32.2` must be allowed"), so the add-on sets
`WEB_ALLOW=172.30.32.2` and the daemon refuses every other client with 403.
In add-on mode (`SUPERVISOR_TOKEN` set) the daemon holds that itself:
`WEB_ALLOW` defaults to `172.30.32.2`, and any other value is refused at
start. Stand-alone, `WEB_ALLOW` takes any list of client addresses.

## mDNS

The node runs its own DNS-SD responder: `_matterc._udp` while its
commissioning window is open, `_matter._tcp` once a controller has paired,
and the discriminator subtype Apple Home and Google Home browse for. A host
with avahi-daemon keeps it running beside it. Without multicast between the
daemon and the controllers (a container without host networking, a VLAN
boundary, a Wi-Fi network that filters multicast) no controller finds the
bridge.

**Interfaces** (`MDNS_INTERFACES`, the add-on's `mdns_interfaces`; empty is
every multicast-capable interface). The responder announces and answers on
the named host interfaces alone, with their addresses (go-fabric
`node.MDNS.Interfaces` at v0.19.1); every node of the daemon, the
configured one and the registry's, uses the same list. The UDP listen
socket is not scoped by it. The case is a dual-homed host behind an mDNS
reflector (go-ha2fabric#40: a Home Assistant OS host on a home VLAN and an
IoT VLAN, whose gateway's mDNS proxy relays the record set it hears on one
VLAN onto the other): the controllers then see the bridge with both
networks' addresses and may try the one they cannot reach. Naming the
interface on the controllers' network keeps the other network's addresses
out of the records. An empty or doubled name is refused at start; a name
the host does not have is allowed (an interface may appear later), and
go-fabric's diagnosis (`Node.DiagnoseMDNS`) reports it, as it reports a
named container bridge (`docker0`, `hassio`, which stays filtered) and a
named interface without an address: at start as one
`ha2fabric.mdns_interface` warning per finding (Logs, below), and on the
web UI's mDNS page.

## The token

The daemon reaches Home Assistant as an administrator:

- **In the add-on** through the Supervisor's proxy
  (`homeassistant_api: true`), authenticated with `SUPERVISOR_TOKEN`, which
  the Supervisor puts into the add-on's environment. "Through the proxy,
  your app acts as an administrator of Home Assistant"
  ([add-on communication](https://developers.home-assistant.io/docs/apps/communication)).
- **Stand-alone** with `HA_TOKEN`, a long-lived access token created on a
  Home Assistant user's profile (Security tab). The user needs to be an
  administrator: `homeassistant/expose_entity/list` is an administrator's
  command (`homeassistant/exposed_entities.py:414-418` at 2026.10.3); the
  three events are on the subscribe allowlist
  (`auth/permissions/events.py:30-49`).

Either token is read from the environment only — there is no flag for it,
so it never shows in the process list — sent in the WebSocket auth message
(and, with `CAMERA`, as the camera proxy's `Authorization: Bearer` header),
and never logged or written to disk. The daemon only reads Home Assistant
and calls the services of the entities it serves; it never writes the
exposure settings or the registries.

The token is only as private as the hop it crosses. `HA_URL` with
`wss://` sends it over TLS, with the certificate verified; with `ws://` it
crosses the network unencrypted, and anyone who can read or answer for that
host on the network gets an administrator's token. The add-on's hop is the
Supervisor's internal name (`ws://supervisor/core/websocket`). Stand-alone,
use `wss://` where Home Assistant has TLS, or run the daemon on Home
Assistant's host (`ws://127.0.0.1:8123/api/websocket`); with `ws://` to
any other host the daemon starts but logs `ha2fabric.token_in_clear` and
shows the warning `token_in_clear` on every status page. Neither the
WebSocket handshake nor the camera proxy's request follows a redirect: an
`HA_URL` that answers with one fails to connect (`ha.disconnected`, naming
the redirect), so set it to the final address. `HA_URL` must name a host and carry no user or password; the log
shows it without a query. The WebSocket handshake and the camera proxy's
request take a proxy from the environment (`HTTP_PROXY`, `HTTPS_PROXY`,
`NO_PROXY`, Go's rules): set one only for a proxy trusted with the token,
or list Home Assistant's host in `NO_PROXY`. The add-on's call to the
Supervisor (`GET /info`) takes no proxy and follows no redirect.

## What the daemon reads and writes

| Command | When | Home Assistant core at 2026.10.3 |
| --- | --- | --- |
| `auth` | each connection | `websocket_api/auth.py:25-46` |
| `subscribe_events` `state_changed`, `entity_registry_updated`, `device_registry_updated`, `label_registry_updated` | each connection | `websocket_api/commands.py:180-219`; `helpers/label_registry.py:23-26` |
| `config/entity_registry/list`, `config/device_registry/list`, `config/area_registry/list`, `config/label_registry/list`, `get_states`, `get_config`, `homeassistant/expose_entity/list` | each connection and each registry change | `config/entity_registry.py:39-64`, `config/device_registry.py:75-101`, `config/area_registry.py:23-37`, `config/label_registry.py:53-67`, `websocket_api/commands.py:369-385, 658-666`, `homeassistant/exposed_entities.py:414-442` |
| `call_service` | a controller's command | `websocket_api/commands.py:266-295` |
| `ping` | every 30 s | `websocket_api/commands.py:730-736` |
| `camera/webrtc/offer`, `camera/webrtc/candidate`, `unsubscribe_events` | a controller's camera session, with `CAMERA` | `camera/webrtc.py:252-303, 333-348`, `websocket_api/commands.py:246-263` |
| `GET /api/camera_proxy/<entity_id>` (HTTP) | a controller's snapshot, with `CAMERA` | `camera/__init__.py:782-838` |

A connection that drops is retried with a back-off that doubles from one
second to one minute; it starts again at one second only after a
connection that stayed up longer than a minute, so a connection that
authenticates and drops at once is not retried every second. A token Home
Assistant refuses (`auth_invalid`) is retried after five minutes, doubling
to hourly, and shown as the status warning `ha_token_refused` until an
authentication succeeds (`ha.token_refused` in the log); the daemon keeps
running and its nodes serve their last state. After each reconnect the
daemon reads everything again, the states first. A registry or state entry
that does not decode (a field of an unexpected type) is skipped, not the
whole list: that entity is left out and `ha.entry_skipped` is logged once
per list.

## Logs

The daemon logs to standard error in slog's text form, at `LOG_LEVEL`
(`debug`, `info`, `warn`, `error`); the node logs through the same handler.
The web UI's Operation page shows the newest records the daemon keeps in
memory (go-fabric's log ring) and streams new ones; an admin can change the
level there, which moves standard error's level too, until the daemon
restarts.
The lines to know:

| Message | Level | Meaning |
| --- | --- | --- |
| `ha2fabric.fabrics` | info | The fabrics the store holds, at start: `count`, and per fabric its index, vendor id and label. `count=0` means no controller has paired — or the store was lost; see Files. |
| `ha2fabric.pairing` | info | At start and after an erase: the manual code and the QR payload while the node has no fabric; a paired node logs only `fabrics=<n>` and that the codes are on the administrator's pairing page. |
| `ha2fabric.passcode_created` | info | Without `PASSCODE`, the configured node's random passcode was drawn and written to `passcode.json` (`file`); once, on the first start. |
| `ha2fabric.passcode_test_value` | warn | `PASSCODE` is the public test value 20202021 (Pairing). |
| `ha2fabric.token_in_clear` | warn | `HA_URL` is `ws://` to a host that is neither loopback nor the Supervisor (`host`; The token). |
| `ha2fabric.supervisor` | info, warn | Add-on: the Supervisor's `version` and whether the web UI has an `admin` (2026.10.4 or later; a pre-release of 2026.10.4 counts as older); at warn when it is older, read again every 10 minutes (`recheck_in`) until it is not. |
| `ha2fabric.supervisor_unreadable` | warn, then debug | Add-on: `GET /info` failed (`error`); retried every 30 s (every 10 minutes once a version below 2026.10.4 was read), the web UI is the status page only meanwhile. |
| `ha2fabric.node_erased` | info | A node was erased in the web UI (`node=<key>` for a registry node): `fabrics_removed`, `elapsed`; the node comes back next, with `ha2fabric.fabrics count=0` and `ha2fabric.pairing`. At warn with `error` when the reset failed part-way. |
| `ha2fabric.node_limit` | warn | At start: `nodes.json` holds more nodes (`nodes`) than `NODE_LIMIT` (`limit`) allows; all start, the web UI creates no more (Nodes). |
| `nodes.mode_tightened` | warn | `nodes.json` or `passcode.json` (`file`) had a mode wider than 0600 (`was`) and was set to 0600 (Files). |
| `ha2fabric.stop_budget` | warn | At stop: the web server or the Home Assistant connection did not end within the budget; the nodes stop anyway (Erase and factory reset). |
| `ha2fabric.web_stop` | warn | At stop: the web server's shutdown failed (`error`), e.g. a request still open at its deadline. |
| `ha2fabric.node_lost` | error | The node did not come back after an erase (`node=<key>` for a registry node). The daemon keeps running; the status page shows `node_erase_failed`. See Erase and factory reset. |
| `ha2fabric.mdns_interface` | warn | At start, with `MDNS_INTERFACES` set: go-fabric's diagnosis of the list (`code`: `interface_not_found`, `interface_filtered`, `interface_no_addresses`; `node=<key>` for a registry node). See mDNS. |
| `ha.connected` / `ha.disconnected` | info / warn | The Home Assistant connection. |
| `ha.token_refused` | warn | Home Assistant refused the access token (`auth_invalid`); retried after `retry_in` (five minutes, doubling to an hour). Also the status warning `ha_token_refused`. |
| `ha.entry_skipped` | warn, then debug | An entry of a list (`command`) did not decode and was skipped (`skipped` of `of`, the first `error`); at warn the first time per list. |
| `bridge.synced` | info | A sync: candidates, devices, endpoints added, replaced, removed. |
| `bridge.upstream_unusable` | warn | Home Assistant is away; every endpoint is unreachable. |
| `bridge.build_failed` | warn | go-fabric refused a device go-hamapping planned; its entities have no endpoint (the plan says why). |
| `bridge.call_service_failed` | warn | A controller's command failed in Home Assistant; the entity's state goes back onto its endpoint. |
| `bridge.command_unserved` | warn | A controller's command waited in the queue while a sync took its entity off the bridge; it was dropped and counted as a command error. |
| `bridge.batch` | error | go-fabric refused part of a sync's batch (the operation and the key); what it refused is tried again at the next sync, and the status shows it as `sync_error` until a sync succeeds. |
| `bridge.command_unmapped` | warn | A command the daemon's service table has no service for. |
| `bridge.command_discarded` | info | A controller's command the entity has no action for (`entity`, `cluster`, `command`): an Off or OffWithEffect on a scene's, script's, automation's or button's plug. It was answered with success and nothing was called — the controller's "done" changed nothing. Counted as `discards` in the status JSON (`counters`) and on the status page, over every node (Health). |
| `bridge.label_unresolved` | warn | A label of `INCLUDE_LABELS`/`EXCLUDE_LABELS` names no Home Assistant label; it is used as a label id, or follows a renamed label by its id; an exclude label that names no label excludes nothing. Logged when the set changes; also on the web UI's status page. |
| `bridge.selection_applied` | info | The web UI stored overrides. |
| `bridge.forget` | info | An entity removed from Home Assistant's registry: its endpoint number is forgotten (`entity`, `key`); created again, it gets a fresh one. |
| `selection.corrupt` | warn | `selection.json` did not parse and was moved aside (`moved_to`); `restored` names the last-good copy read instead, empty when none could be (Files); also on the web UI's status page. |
| `selection.version` | warn | `selection.json` is of another version (`version`) and was read leniently (Files); also on the web UI's status page. |
| `selection.missing` | warn | `selection.json` was missing beside its last-good copy; `restored` names the copy read and written back, empty when it could not be read (Files); also on the web UI's status page. |
| `selection.restore_failed` | warn | The last-good copy read at start could not be written back as `selection.json` (`error`); the next start reads the copy again. |
| `bridge.camera_disabled` | info | Once per start: exposed cameras skipped because `CAMERA` is off (`skipped`). |
| `camera.session` / `camera.session_ended` | info | A controller opened or ended a camera session (`entity_id`, `session`). |
| `camera.ha_error` | warn | Home Assistant reported an error for a session (`code`, `message`, e.g. `go2rtc_webrtc_offer_failed`); the session ends. |
| `camera.signal_too_large` | error | A signal with an SDP did not fit the controller's UDP session; the session ends. See Cameras. |
| `camera.signal_failed` | warn | A signal did not reach the controller (`command`, `error`); the session ends. |
| `camera.snapshot_failed` | warn | A controller's snapshot failed at the camera proxy (`error`: off, refused, too large). |
| `camera.session_lost` | info | The Home Assistant connection dropped; its camera sessions ended with it. |
| `camera.session_evicted` / `camera.session_idle` | info | A session ended to make room for a new offer at the camera's 8 (`session`, `for`), or after an hour without signalling (Cameras). |
| `camera.answer_empty` | warn | Home Assistant's answer carried no SDP; the session ends. |

## Labels

`INCLUDE_LABELS` and `EXCLUDE_LABELS` (add-on `include_labels`,
`exclude_labels`) name Home Assistant labels. A value matches the label
whose name equals it, ignoring case, else the label whose id it is; a value
that matches neither is used as a label id as given, so it takes effect
once such a label is created. The values are split at commas and
newlines, so a label whose name holds a comma is named by its id. Such a
value is a warning: in the log
(`bridge.label_unresolved`, when the set of unresolved values changes) and
on the web UI's status page and above its selection list
(`label_unresolved`, with the time it was first seen), until the label
exists or the value is corrected. When Home Assistant renames a label, a
value that named it follows its label id while the daemon runs, and the
warning names the id and the new name; a restart resolves the names afresh,
so update the value (or configure the label id) after a rename. An exclude
label that names no label excludes nothing, and its warning says so. The plan's `labels` column and the web UI
show an entity's labels, its own and its device's; the reason says which
label decided ("included by label 'Matter'"). The daemon subscribes to
`label_registry_updated`: creating, renaming or removing a label, or adding
one to an entity or a device (`entity_registry_updated`,
`device_registry_updated`), re-plans the bridge after a one-second
debounce. The order: disabled, hidden, the web UI's override, `EXCLUDE`
and `EXCLUDE_LABELS`, `INCLUDE` and `INCLUDE_LABELS`, Assist.

## Scripts, scenes and automations

A served `script`, `scene` or `automation` appears as a plug
(OnOffPlugInUnit) that always reads off; switching it on runs it —
`script.turn_on`, `scene.turn_on` or `automation.trigger`, without data
(go-hamapping `docs/by_design.md`, "A script, scene or automation is a
plug-in unit that always reads off").

Switching it off does nothing: the command is answered with success,
logged `bridge.command_discarded` and counted as `discards` (Logs,
Health). Observed on an Alexa fabric (go-ha2fabric#51, 2026-11-09): since
the plug always reads off, the app's tile drifts to the state it last
commanded, so tapping it sends Off again and lands in the discard path —
"the tile does nothing" —, and one voice command may arrive as two Offs
about a second apart, counted twice. Voice "turn on" is the control that
works there.

## Locks with a code

The daemon calls `lock.lock`, `lock.unlock` and `lock.open` without a
`code`. A lock whose integration requires one is driven through Home
Assistant's own setting: open the lock entity's settings in Home
Assistant and enter the default code; Home Assistant stores it in the
entity registry's lock options and inserts it into every call that
carries none (`homeassistant/components/lock/__init__.py:113-133,
280-295` at 2026.10.3). Without a stored code such a call fails in Home
Assistant with "The code for <entity> doesn't match pattern
<code_format>" (`lock/strings.json:106`), and the daemon logs
`bridge.call_service_failed`. No code passes through the daemon, its
log or its selection file.

## Cameras (opt-in)

With `CAMERA=true` (the add-on option `camera`) a camera entity with the
`STREAM` feature (`camera/const.py:71-75` at 2026.10.3) becomes a Matter
Camera (device type 0x0142) through go-hamapping's camera row; a camera
without it stays unmapped. The option is **off by default**: go-fabric's
ecosystem gate keeps camera clusters out of host devices until a live test
with Apple Home has passed (go-fabric `docs/guides/camera.md:414-425` at
v0.15.5), and the daemon follows it until that test is recorded.

What the daemon does — signalling only:

- A controller's `ProvideOffer` goes to Home Assistant as
  `camera/webrtc/offer`; Home Assistant's answer and ICE candidates go
  back to the controller's WebRtcTransportRequestor as `Answer` and
  `IceCandidates`, one per candidate as they arrive; the controller's
  candidates go to `camera/webrtc/candidate`; `EndSession` unsubscribes the
  offer, which closes Home Assistant's session
  (`camera/webrtc.py:252-303, 333-348`, `websocket_api/commands.py:246-263`).
- Home Assistant's WebRTC provider — go2rtc, or a camera integration with
  native WebRTC — produces the stream and talks to the controller directly
  (`camera/__init__.py:742-755`); no media passes through the daemon.
- A controller's `CaptureSnapshot` is the camera proxy's still,
  `GET /api/camera_proxy/<entity_id>` with the token as a Bearer header
  (`camera/__init__.py:782-838`, `http/auth.py:226-230`); a camera that is
  off answers 503 there and the snapshot fails.

**TCP.** A signal carries an SDP, which rarely fits one UDP message (1178
bytes of payload); go-fabric sends it over the controller's TCP session
and refuses it on a UDP session (go-fabric `docs/guides/camera.md:121-128`).
With `CAMERA` on, the node therefore accepts Matter over TCP on its port;
let TCP 5540 through a firewall. A controller that reached the bridge over
UDP cannot receive the answer: the log says `camera.signal_too_large` and
the session ends.

**The live test must confirm one thing in the add-on.** The camera proxy's
URL is derived from the WebSocket URL: `ws` becomes `http` (`wss` `https`),
the path's `websocket` is dropped and `api` follows. Stand-alone,
`ws://host:8123/api/websocket` gives `http://host:8123/api/camera_proxy/…`;
in the add-on, `ws://supervisor/core/websocket` gives
`http://supervisor/core/api/camera_proxy/…`, the Supervisor's proxy of the
core API. A 401 or a connection error in `camera.snapshot_failed` means
this derivation is wrong for the installation.

Limits:

- `SolicitOffer` is refused: Home Assistant takes the client's offer only
  (`camera/webrtc.py:252-258`); the answer is INVALID_IN_STATE.
- The controller's ICE servers are not relayed; Home Assistant's provider
  uses its own (`camera/__init__.py:708-718`).
- The stream declaration is nominal: Home Assistant names no codec,
  resolution or frame rate (`camera/__init__.py:742-755`); the daemon
  allocates one video, one audio and one snapshot stream and changes
  nothing on a controller's parameters.
- A still is at most 64000 bytes (go-fabric's large-message bound) and
  Home Assistant scales a JPEG down only, by fixed factors
  (`camera/img_util.py:18-36`): a controller that asks for a large
  resolution of a large camera may get no snapshot.
- A camera holds at most 8 WebRTC sessions. A further offer ends the
  session idle longest — no command and no Home Assistant event for a
  minute — with `End` (reason 12) to its controller
  (`camera.session_evicted`), or, when every session is more recent, is
  answered RESOURCE_EXHAUSTED. A session with no signalling for an hour
  ends the same way (`camera.session_idle`): a controller that went away
  never ends its session, and a live view signals nothing once it stands,
  so a view kept open longer than an hour must be opened again.
- A controller's `ProvideIceCandidates` carries at most 64 candidates and
  a session at most 256; beyond, the call is answered RESOURCE_EXHAUSTED.
  Before Home Assistant names its session at most 64 wait.
- One camera proxy request runs per camera at a time: snapshots that
  arrive meanwhile share its still, and a still younger than a second is
  handed to a further snapshot of the same size.
- A re-offer whose new Home Assistant session fails ends the session with
  `End` (reason 0); an answer without an SDP ends it the same way
  (`camera.answer_empty`).

## The web UI

go-fabric-ui's interface (`v0.13.0`) at `WEB_LISTEN`, in the add-on through
Ingress. The administrator — who may write (change the selection, open a
commissioning window, remove a fabric, erase a node) and read every page
but the status: the selection, fabrics, endpoints, diagnostics, pairing
with its code and the Operation page — is in the add-on a Home Assistant
administrator: a request of the Ingress proxy (`172.30.32.2`) whose
`X-Remote-User-Groups`, which the Supervisor sets from the Home Assistant
user, lists `system-admin` (an owner's groups include it); without the
header the request is not an administrator's. The check needs Supervisor
2026.10.4 or later: the add-on reads the Supervisor's version at start
(`GET http://supervisor/info` with `SUPERVISOR_TOKEN`,
`ha2fabric.supervisor` in the log) and, below 2026.10.4 or until it has
read it, has no administrator — the status page only, with the warning
`supervisor_too_old` or `supervisor_unknown`; while it is below, it reads
the version again every 10 minutes and gets its administrator once the
Supervisor is updated. The manifest sets
`panel_admin: true`, so the panel is an administrator's as well.
Stand-alone nobody, unless `WEB_ADMIN=true`, which selects the UI's token
mode (every request carries a per-process token the page reads from
`/api/v1/node/status`, so anyone who can reach the port is the
administrator). Stand-alone without `WEB_ADMIN` the UI is the status page
only.

`WEB_ADMIN=true` on a `WEB_LISTEN` that is not a loopback address
(`0.0.0.0:8099`, for example) is refused at start (exit 2) unless
`WEB_ALLOW` names the clients: either bind the UI to loopback
(`127.0.0.1:8099`; `localhost` is accepted by name, and the web UI
refuses to serve when it resolves to an address that is not loopback) and
reach it through a proxy of one's own, or list the
administrators' addresses in `WEB_ALLOW`. The page is plain HTTP — the
pairing code and the token mode's token cross the network unencrypted —
and an address filter is not authentication: any host that has, or can
take, a listed address is the administrator. `WEB_ALLOW` entries must be
IP addresses; one that is not fails the start. `WEB_ALLOW` sees the address
the connection comes from: in a container on a bridge network with a
published port (rather than `--network host`), that can be the bridge
network's gateway for every client, and a `WEB_ALLOW` naming it then admits
everyone who reaches the published port.

Every request's `Host` is checked: loopback names (`localhost`) and IP
addresses pass; a DNS name passes only when `WEB_ALLOWED_HOSTS` lists it
(comma-separated host names, no scheme or port, for example
`WEB_ALLOWED_HOSTS=bridge.lan`); any other is answered 403
`host_not_allowed`. The add-on needs no list: Home Assistant's Ingress
forwards the browser's `Host` (Home Assistant's own name), and the daemon
admits every request of the Ingress proxy whatever its name. A proxy of
one's own in front of a stand-alone daemon either forwards the browser's
`Host` or replaces it with the daemon's address — then the Host check is
the proxy's, which must serve its own names only; the name the browser uses
goes into `WEB_ALLOWED_HOSTS` in both cases, because an administrator's
request is refused (403 `origin_not_allowed`) when its `Origin` names a
host that is not listed. An administrator's request with an `Origin`
passes only when the Origin names a host the check admits (`localhost`, an
IP address, a listed name); in the add-on it must equal the request's
`Host`.

The selection page lists every candidate entity
with entity id, name, domain, device, area, labels, Assist exposure and
integration, grouped by device, filterable by domain, area, a single label, verdict
and the deciding layer (`rule:assist`, `rule:include`, `rule:label`,
`rule:exclude`, `override:on`, `override:off`, `disabled`, `hidden`).
Preview shows the verdicts a change would bring (go-hamapping's `Map`
without building); apply stores the overrides in `selection.json` and
re-plans the bridge before it answers. Turning an entity back to what the
rule decides removes its override; an entity turned off and on again gets
its endpoint number back — as does one disabled, hidden or left out by the
rule and then brought back. Only an entity deleted from Home Assistant's
entity registry loses its number (`bridge.forget`): Home Assistant gives a
re-created entity its old registry id, but the bridge serves it under a
fresh number, as a new device. One deleted and re-created within the
re-sync's debounce keeps its endpoint. A rename is the endpoint's name in the controller
as typed (its node label, at most 32 bytes), without the device name
go-hamapping puts in front of an entity name otherwise. The status page
lists the configuration's warnings: unresolved label values
(`label_unresolved`), an overrides file moved aside at start
(`selection_corrupt`), the token sent unencrypted (`token_in_clear`,
The token), the token refused by Home Assistant (`ha_token_refused`,
What the daemon reads and writes), the public test passcode (`passcode_test_value`, Pairing), a
passcode that could not be kept (`passcode_not_kept`, Files) and,
in the add-on, a Supervisor too old or not read yet for an administrator
(`supervisor_too_old`, `supervisor_unknown`); under them the daemon's counters (`discards`,
Health).

The Operation page shows the daemon's version and uptime, its recent log
and the log level (see Logs); changing the level is a write.

## Health

`GET /api/v1/node/status` (go-fabric-ui's contract) is the node's health
sample, its listen address, its endpoint and fabric counts and whether the
commissioning window is open, and `counters`, the daemon's own counts
since it started (go-fabric-ui contract 0.8.0), host-wide as the warnings
are, so every node's status carries the same list: `discards`, the
controller commands answered with success that called nothing
(`bridge.command_discarded`), summed over every node, the runs an erase
ended included. `GET
/api/v1/node/exposure` is every candidate entity with the exposure rule's
word, its labels, the deciding layer and go-hamapping's verdict, the same
as `--dump-plan` prints, and stays empty until Home Assistant has been read
once. The Home Assistant connection is in the log (`ha.connected`,
`ha.disconnected`, `bridge.synced`). 0.1.0's `GET /api/health` and `GET
/api/plan` are gone since 0.2.0.

## Pairing

The pairing code is on the web UI's pairing page with its QR code (for an
administrator), and in the log at start (`ha2fabric.pairing`) while the node
has no fabric; a paired node does not log its codes, which then open
nothing but the window that opens again once its last fabric goes. The node opens its commissioning window when no controller
has paired; the first controller closes it. To pair a second ecosystem,
open a pairing window from the first controller (its multi-admin sharing
of the bridge), or from the web UI's pairing page, which opens one with
fresh credentials and shows their codes. On a node without a fabric, the
node's own window on the configured codes opens again when the page's
window closes or expires, so closing it from the page once leaves the
own window open; a second close closes that too (go-fabric-ui
`Options.ConfiguredWindow`, which the daemon leaves off).

The configured node's passcode is `PASSCODE` (the add-on's `passcode`
option) or, unset, a random valid one drawn on the first start and kept in
`passcode.json` beside the store (`ha2fabric.passcode_created` in the log),
as the registry nodes get theirs. The public test value 20202021 — the
default before 0.7.0, which add-on options saved before 0.7.0 keep — is
honoured, with the log line `ha2fabric.passcode_test_value` and the status
warning `passcode_test_value`: anyone in reach of the commissioning window
can pair with it. An add-on installation that never saved its options has
no `passcode` after the update and gets a random one, so an unpaired
bridge's codes change; a paired bridge stays paired either way. The other defaults are public test
values too: discriminator 3840, the CSA test vendor 0xFFF1 and product
0x8001. The bridge carries go-fabric's test attestation chain; a
controller may warn that it is an uncertified device and needs a
confirmation to go on.

## Nodes

The first node is the daemon's configuration: `LISTEN`, `PASSCODE`,
`DISCRIMINATOR`, `VENDOR_ID`, `PRODUCT_ID`, `LABEL`, `STORE_PATH` (the
add-on's options). Every further node is registry state, created,
renamed and deleted in the web UI's node selector (go-fabric-ui contract
0.6.0, "Node registry") — one node per ecosystem when Apple Home and
Alexa should see different devices: a Matter node has one endpoint tree
for all its fabrics, so a different scope is a different node.

- **Create** allocates the node's key (a slug of its name, unique;
  `alexa`, `apple-home`), its port (the first above `LISTEN`'s port that
  no node names and that binds now, over TCP too with `CAMERA`), its
  discriminator (random in 12 bits, unique among the nodes) and its
  passcode (random, valid), starts it, and writes `nodes.json`; the log
  says `ha2fabric.node_created`. The node shares the vendor and product
  ids and `CAMERA` with the configured node.
- **Its scope** is its own selection page alone: a new node serves
  nothing until its entities are picked there. `EXPOSE_ASSIST`,
  `INCLUDE`, `EXCLUDE` and the label lists are the configured node's.
- **Pairing**: each node has its own pairing page and codes, and opens
  its commissioning window while no controller has paired it; until then
  its codes are in the log at its start (`ha2fabric.pairing node=<key>`).
- **Rename** changes the name in the selector and the status page, and
  the running node's `NodeLabel` (cut to 32 bytes), at once: the node
  persists it and reports it to its controllers' subscriptions (go-fabric
  `Node.SetNodeLabel`), so a rename reaches the controllers without a
  restart (`ha2fabric.node_renamed`; at warn with `error` when the node
  refused it, which leaves the registry's name standing). A controller
  that writes a name later wins in turn.
- **Delete** is final: the node leaves every fabric it holds (its
  controllers lose it), stops, and its store, its selection (with its
  `.bak` and any copy moved aside) and its record are removed (`ha2fabric.node_deleted`). The configured node
  cannot be deleted (409 `node_is_default`).
- **Erase** starts the node over for the controllers and keeps it: as
  for the configured node (Erase and factory reset), its fabrics, its
  store and its endpoint records are erased, and its record, its
  selection and its pairing codes stay. Delete, by contrast, ends the
  node and removes its files.
- **Restart**: every registry node starts with the daemon. One that
  cannot (its port taken) is logged (`ha2fabric.node_start`) and listed
  as not enabled; deleting it removes its files.
- **Limit**: `NODE_LIMIT` (`--node-limit`, the add-on's `node_limit`,
  default 8, 0 to 32) bounds how many nodes the web UI may create beside
  the configured one; one more is refused with 409 `node_limit`. Nodes
  already in `nodes.json` all start when the limit is lowered below their
  number, with the log line `ha2fabric.node_limit`.

Every node's bridge rides the daemon's one Home Assistant connection; a
registry change re-plans every node. Each status page shows its node's
configuration block — where its values come from, its address,
discriminator, ids and store — never the passcode.

## Erase and factory reset

**Erase**, on a node's Fabrics page (an administrator; two
confirmations), starts the node over. It erases the fabrics — every
paired controller loses the node, through the Leave event —, the store
(ACL, group keys, resumption records, subscriptions, settings and
counters, the pinned UniqueID; a fresh one is pinned) and the endpoint
records with their number counter (go-fabric `Node.FactoryReset`, which
calls `endpoint.Store.EraseEndpoints` since v0.19.0), so at the next sync
the entities are numbered from the first bridged number (2) again, as on
a new installation; the daemon test measures it (`TestNodeErase`: the
fixtures' entities take the numbers they had before the erase). The
controllers see a new device either way: the fabric is new. It keeps the configuration, `selection.json` (a registry node's
`selection-<key>.json`), a registry node's record, the passcode and the
discriminator, so the pairing codes stay the same. The node comes back
at once, with its commissioning window open: the log says
`ha2fabric.node_erased`, then `ha2fabric.fabrics count=0` and
`ha2fabric.pairing`. The controllers keep a dead bridge until it is
removed there. A node that does not come back (`ha2fabric.node_lost`)
leaves the daemon running, and every node's status page shows
`node_erase_failed`: Erase again, or restart the daemon, which comes up
on the erased store. A reset that fails (`ha2fabric.node_erased` at warn
with `error`) can leave the node running on its port and store; the
daemon stops it before the come-back, so the node comes back as it was
and the erase answers the error — Erase again. When it cannot be stopped
either, it is kept: the come-back fails on the port it holds, and the
next Erase resets that node again. An erase runs while the status pages
keep answering, and ends when the daemon stops. A registry node in that
state is listed as not enabled; its Delete works.

A stop (SIGTERM, the add-on's stop) has one budget of 25 seconds: the web
server and the Home Assistant connection end first, then every node stops
at once; an erase in progress ends with the daemon, and a create, rename
or delete still running is not waited for beyond the budget. The add-on's
manifest gives the stop 30 seconds before the Supervisor kills it. A
second signal ends the process at once; a signal during the start is an
ordinary stop (exit 0).

The manual way: stop the daemon and delete the store file (and a `-journal` next to it);
for every node, delete the `ha2fabric-*.db` and `selection-*.json` files
and `nodes.json` too, or delete one node in the web UI.
The next start pairs from nothing; the controllers keep a dead bridge until
they are told to remove it. Home Assistant is untouched.
