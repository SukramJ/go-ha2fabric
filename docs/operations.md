# Operations

What an operator of go-ha2fabric needs: the files it keeps, the ports and
addresses, mDNS, the token, the logs, the health endpoint and pairing. The
node's side of each is go-fabric's
[operations guide](https://github.com/SukramJ/go-fabric/blob/v0.11.0/docs/guides/operations.md);
this page is the daemon's version.

## Files

The daemon keeps one file: the node's SQLite store, `STORE_PATH`.

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
| `subscribe_events` `state_changed`, `entity_registry_updated`, `device_registry_updated` | each connection | `websocket_api/commands.py:180-219` |
| `config/entity_registry/list`, `config/device_registry/list`, `config/area_registry/list`, `get_states`, `get_config`, `homeassistant/expose_entity/list` | each connection and each registry change | `config/entity_registry.py:39-64`, `config/device_registry.py:75-101`, `config/area_registry.py:23-37`, `websocket_api/commands.py:369-385, 658-666`, `homeassistant/exposed_entities.py:414-442` |
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

## Health

`GET /api/health` answers 200 with `"status": "ok"`, or 503 with
`"status": "degraded"` and the problems: Home Assistant not connected or
not read yet, the node's health probe unhealthy, the fabrics unreadable.
It also carries the
pairing codes, the commissioned fabrics and the bridge's counters
(endpoints, candidates, service calls and their failures).
`GET /api/plan` is every candidate entity with the exposure rule's word and
go-hamapping's verdict, the same as `--dump-plan` prints.

## Pairing

The pairing code is in the log at start (`ha2fabric.pairing`), on the web
UI with its QR code, and in `/api/health`. The node opens its
commissioning window when no controller has paired; the first controller
closes it. To pair a second ecosystem, open a pairing window from the first
controller (its multi-admin sharing of the bridge), which shows a code of
its own.

The defaults are public test values: passcode 20202021, discriminator 3840,
the CSA test vendor 0xFFF1 and product 0x8001. Change the passcode before
pairing for real. The bridge carries go-fabric's test attestation chain; a
controller may warn that it is an uncertified device and needs a
confirmation to go on.

## Factory reset

Stop the daemon and delete the store file (and a `-journal` next to it).
The next start pairs from nothing; the controllers keep a dead bridge until
they are told to remove it. Home Assistant is untouched.
