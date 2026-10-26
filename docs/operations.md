# Operations

What an operator of go-ha2fabric needs: the files it keeps, the ports and
addresses, mDNS, the token, the logs, the health endpoint and pairing. The
node's side of each is go-fabric's
[operations guide](https://github.com/SukramJ/go-fabric/blob/v0.11.0/docs/guides/operations.md);
this page is the daemon's version.

## Files

The daemon keeps two files: the node's SQLite store, `STORE_PATH`, and
beside it `selection.json`, the web UI's per-entity overrides.

| Where | Path |
| --- | --- |
| Add-on | `/data/ha2fabric.db`, the add-on's persistent data directory (`script/run.sh`) |
| Daemon image | `/data/ha2fabric.db`, the image's volume (`Dockerfile`) |
| Stand-alone | `ha2fabric.db` in the working directory, unless `STORE_PATH` says otherwise |

The store holds the fabrics with their operational keys, the access control
lists, the subscriptions to re-establish and the endpoint numbers of the
bridged entities. Losing it unpairs the bridge from every controller, and
the entities come back under new endpoint numbers. Keep it on storage that
survives restarts and upgrades, protect it like a key (it holds private
keys unencrypted), back it up only with the daemon stopped (the add-on
sets `backup: cold`, so the Supervisor stops it for a backup), and never
run two copies of one store.

`selection.json` is `{"version": 1, "entities": {"<entity_id>": {"enabled":
true, "name": "…"}}}`, written atomically (a temporary file, then a rename)
and read at start. A file that does not parse is renamed to
`selection.json.corrupt-<unix seconds>`, logged (`selection.corrupt`) and
the daemon starts without overrides. The add-on's cold backup holds it with
the store. An override keyed by an entity id does not follow a rename of
that id.

The daemon stores nothing else: not the token, not Home Assistant's
registry, not the states. Everything it knows about Home Assistant it reads
again on every connection.

## Ports and addresses

| What | Default | Notes |
| --- | --- | --- |
| UDP `LISTEN` | `:5540` | Every Matter message. The add-on's `port` option sets it; change it when another Matter node on the same host uses 5540. |
| UDP 5353, `224.0.0.251`, `ff02::fb` | — | mDNS, below. |
| IPv6 on the LAN | required | Matter's operational network is IPv6. |
| TCP `WEB_LISTEN` | `127.0.0.1:8099`; add-on `0.0.0.0:8486`, image `0.0.0.0:8099` | The web UI. Empty disables it. |
| Outbound WebSocket | `HA_URL`; add-on `ws://supervisor/core/websocket` | Home Assistant's WebSocket API. |

**Host networking.** Controllers find the bridge over mDNS and talk to it
over UDP on the LAN, so the daemon needs the host's network: the add-on
sets `host_network: true`, the image runs with `--network host`. A
consequence for the add-on: its web UI listens on the host too. Home
Assistant's Ingress requires that only its proxy reaches it
([add-on presentation, "Ingress"](https://developers.home-assistant.io/docs/apps/presentation#ingress):
"Only connections from `172.30.32.2` must be allowed"), so the add-on sets
`WEB_ALLOW=172.30.32.2` and the daemon refuses every other client with 403.
Stand-alone, `WEB_ALLOW` takes any list of client addresses.

## mDNS

The node runs its own DNS-SD responder: `_matterc._udp` while its
commissioning window is open, `_matter._tcp` once a controller has paired,
and the discriminator subtype Apple Home and Google Home browse for. A host
with avahi-daemon keeps it running beside it. Without multicast between the
daemon and the controllers (a container without host networking, a VLAN
boundary, a Wi-Fi network that filters multicast) no controller finds the
bridge.

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
so it never shows in the process list — sent in the WebSocket auth message,
and never logged or written to disk. The daemon only reads Home Assistant
and calls the services of the entities it serves; it never writes the
exposure settings or the registries.

## What the daemon reads and writes

| Command | When | Home Assistant core at 2026.10.3 |
| --- | --- | --- |
| `auth` | each connection | `websocket_api/auth.py:25-46` |
| `subscribe_events` `state_changed`, `entity_registry_updated`, `device_registry_updated`, `label_registry_updated` | each connection | `websocket_api/commands.py:180-219`; `helpers/label_registry.py:23-26` |
| `config/entity_registry/list`, `config/device_registry/list`, `config/area_registry/list`, `config/label_registry/list`, `get_states`, `get_config`, `homeassistant/expose_entity/list` | each connection and each registry change | `config/entity_registry.py:39-64`, `config/device_registry.py:75-101`, `config/area_registry.py:23-37`, `config/label_registry.py:53-67`, `websocket_api/commands.py:369-385, 658-666`, `homeassistant/exposed_entities.py:414-442` |
| `call_service` | a controller's command | `websocket_api/commands.py:266-295` |
| `ping` | every 30 s | `websocket_api/commands.py:730-736` |

A connection that drops is retried with a back-off from one second to one
minute; after each reconnect the daemon reads everything again.

## Logs

The daemon logs to standard error in slog's text form, at `LOG_LEVEL`
(`debug`, `info`, `warn`, `error`); the node logs through the same handler.
The lines to know:

| Message | Level | Meaning |
| --- | --- | --- |
| `ha2fabric.pairing` | info | The manual code and the QR payload, at start. |
| `ha.connected` / `ha.disconnected` | info / warn | The Home Assistant connection. |
| `bridge.synced` | info | A sync: candidates, devices, endpoints added, replaced, removed. |
| `bridge.upstream_unusable` | warn | Home Assistant is away; every endpoint is unreachable. |
| `bridge.build_failed` | warn | go-fabric refused a device go-hamapping planned; its entities have no endpoint (the plan says why). |
| `bridge.call_service_failed` | warn | A controller's command failed in Home Assistant; the entity's state goes back onto its endpoint. |
| `bridge.command_unmapped` | warn | A command the daemon's service table has no service for. |
| `bridge.label_unresolved` | warn | A label of `INCLUDE_LABELS`/`EXCLUDE_LABELS` names no Home Assistant label; it is used as a label id. Logged when the set changes; also on the web UI's status page. |
| `bridge.selection_applied` | info | The web UI stored overrides. |
| `bridge.forget` | info | An entity removed from Home Assistant's registry: its endpoint number is forgotten (`entity`, `key`); created again, it gets a fresh one. |
| `selection.corrupt` | warn | `selection.json` did not parse and was moved aside; also on the web UI's status page. |

## Labels

`INCLUDE_LABELS` and `EXCLUDE_LABELS` (add-on `include_labels`,
`exclude_labels`) name Home Assistant labels. A value matches the label
whose name equals it, ignoring case, else the label whose id it is; a value
that matches neither is used as a label id as given, so it takes effect
once such a label is created. Such a value is a warning: in the log
(`bridge.label_unresolved`, when the set of unresolved values changes) and
on the web UI's status page and above its selection list
(`label_unresolved`, with the time it was first seen), until the label
exists or the value is corrected. The plan's `labels` column and the web UI
show an entity's labels, its own and its device's; the reason says which
label decided ("included by label 'Matter'"). The daemon subscribes to
`label_registry_updated`: creating, renaming or removing a label, or adding
one to an entity or a device (`entity_registry_updated`,
`device_registry_updated`), re-plans the bridge after a one-second
debounce. The order: disabled, hidden, the web UI's override, `EXCLUDE`
and `EXCLUDE_LABELS`, `INCLUDE` and `INCLUDE_LABELS`, Assist.

## The web UI

go-fabric-ui's interface (`v0.1.2`) at `WEB_LISTEN`, in the add-on through
Ingress. Who may write — change the selection, open a commissioning window,
remove a fabric — and read the pairing code: in the add-on the Ingress
proxy (`172.30.32.2`), which has authenticated the Home Assistant user;
stand-alone nobody, unless `WEB_ADMIN=true`, which selects the UI's token
mode (every write carries a per-process token the page reads from
`/api/v1/node/status`, so anyone who can reach the port can write — keep
it on a trusted network). The selection page lists every candidate entity
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
(`label_unresolved`) and an overrides file moved aside at start
(`selection_corrupt`).

## Health

`GET /api/v1/node/status` (go-fabric-ui's contract) is the node's health
sample, its listen address, its endpoint and fabric counts and whether the
commissioning window is open; `GET
/api/v1/node/exposure` is every candidate entity with the exposure rule's
word, its labels, the deciding layer and go-hamapping's verdict, the same
as `--dump-plan` prints, and stays empty until Home Assistant has been read
once. The Home Assistant connection is in the log (`ha.connected`,
`ha.disconnected`, `bridge.synced`). 0.1.0's `GET /api/health` and `GET
/api/plan` are gone since 0.2.0.

## Pairing

The pairing code is in the log at start (`ha2fabric.pairing`), on the web
UI's pairing page with its QR code (for an administrator). The node opens its commissioning window when no controller
has paired; the first controller closes it. To pair a second ecosystem,
open a pairing window from the first controller (its multi-admin sharing
of the bridge), or from the web UI's pairing page, which opens one with
fresh credentials and shows their codes.

The defaults are public test values: passcode 20202021, discriminator 3840,
the CSA test vendor 0xFFF1 and product 0x8001. Change the passcode before
pairing for real. The bridge carries go-fabric's test attestation chain; a
controller may warn that it is an uncertified device and needs a
confirmation to go on.

## Factory reset

Stop the daemon and delete the store file (and a `-journal` next to it).
The next start pairs from nothing; the controllers keep a dead bridge until
they are told to remove it. Home Assistant is untouched.
