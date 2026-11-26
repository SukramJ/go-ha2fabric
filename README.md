# go-ha2fabric

[![ci](https://github.com/thingfabric/go-ha2fabric/actions/workflows/ci.yml/badge.svg)](https://github.com/thingfabric/go-ha2fabric/actions/workflows/ci.yml)
[![codeql](https://github.com/thingfabric/go-ha2fabric/actions/workflows/codeql.yml/badge.svg)](https://github.com/thingfabric/go-ha2fabric/actions/workflows/codeql.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Open your Home Assistant instance and add this add-on repository.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fthingfabric%2Fgo-ha2fabric)

A Home Assistant installation seen by Apple Home, Google Home and Amazon
Alexa as a bridge — or as one bridge per ecosystem: the entities Home Assistant exposes become endpoints
of a bridged node. The daemon embeds [go-fabric](https://github.com/thingfabric/go-fabric) in
process: the Home Assistant WebSocket API (entity and device registry,
`state_changed`, `call_service`) on one side, go-fabric's device kit and
`node` facade speaking Matter to the controllers on the other, one
process. Which
Home Assistant component becomes which device type and cluster is the
table of [`go-hamapping`](https://github.com/SukramJ/go-hamapping).

Twin of [`go-hamapping`](https://github.com/SukramJ/go-hamapping); both are built on
[go-fabric](https://github.com/thingfabric/go-fabric) and share its project
setup (pure Go, `CGO_ENABLED=0`, Apache-2.0 with a contributor licence
agreement, the same guards in CI).

> **Status: 0.7.0, released.** The daemon and its Home Assistant add-on
> run. Cameras stay an opt-in (brief R6) until a real pairing with Apple
> Home has passed, which is also the owner's step before the project is
> announced. The design is in [`CONCEPT.md`](CONCEPT.md).

## Why "Matter" is not in the name

"Matter" and the Matter logo are trademarks of the Connectivity Standards
Alliance. Their use in a product, project or repository name, as on a
device, is reserved to Alliance members and certified products under the
*Connectivity Standards Alliance Trademark and Logo Usage Guidelines and
Terms* (<https://csa-iot.org/resources/governing-documents/>; the terms are
restated in go-fabric's
[`licenses/NOTICE-matter.js.txt`](https://github.com/thingfabric/go-fabric/blob/v0.27.0/licenses/NOTICE-matter.js.txt)
at v0.27.0, shipped in this project's `licenses/` tree:
"Only the Alliance and its members may use Alliance trademarks and logos,
including, without limitation, the Matter trademarks and logos"). This
project is neither, so the mark is not part of its name; the word appears
only descriptively, as in "a bridge for Matter controllers", and never to
suggest certification or endorsement. Nothing built with this project is
certified by the Alliance, and using it does not convey any right to
describe a device, product or service as Matter compliant or certified.
See go-fabric's README, section "Not certified".

## Features

- **One bridge for every exposed entity.** Each Home Assistant device
  becomes bridged endpoints of one go-fabric node; go-hamapping decides the
  device types and clusters ([go-hamapping v0.8.0](https://github.com/SukramJ/go-hamapping/blob/v0.8.0/docs/rows.md)): lights (on/off, dimmable, colour
  temperature, extended colour), switches, scenes, scripts, automations
  and buttons, sensors (temperature, humidity, pressure, illuminance,
  power, voltage, current, energy, air quality and more, with a device's
  battery on its main endpoint), binary sensors (contact, motion,
  occupancy, water leak, freeze, smoke, carbon monoxide), covers, climate,
  locks, fans, media players, vacuums, valves, water heaters, selects,
  events and, opt-in, cameras.
- **Several nodes, one per ecosystem.** Beside the configured node the
  web UI creates further nodes — one for Apple Home, one for Alexa —
  each with its own port, pairing code, store and entity selection, on
  the one Home Assistant connection; created, renamed and deleted at run
  time, started with the daemon (`NODE_LIMIT`, default 8;
  [operations](docs/operations.md#nodes)).
- **Both directions.** `state_changed` becomes an attribute report to every
  subscribed controller; a controller's command becomes a `call_service`.
- **Live topology.** A renamed, disabled, hidden or newly exposed entity
  moves the bridge at run time; the other endpoints keep their numbers.
- **Reachability.** An `unavailable` or `unknown` entity is unreachable;
  while Home Assistant is away, every endpoint is.
- **The family's web UI** ([go-fabric-ui](https://github.com/thingfabric/go-fabric-ui)
  `v0.14.1`, through Ingress in the add-on): health, the pairing code and the
  commissioning window, the fabrics, the endpoints, diagnostics, and the
  entity selection — every candidate entity with its labels, the layer of
  the rule that decided and go-hamapping's verdict, with per-entity
  overrides. `--dump-plan` prints the same table. The terms the pages use
  are defined in go-fabric's [glossary](https://docs.thingfabric.dev/reference/glossary/).
- **Pure Go**, `CGO_ENABLED=0`: a static binary, a distroless image, a Home
  Assistant add-on for amd64 and aarch64 (no armv7 after 0.7.0: Home
  Assistant publishes no armv7 base image any more; the release archives
  and the plain image still cover linux/arm/v7).

## The exposure rule

An entity is served when it is exposed to Assist in Home Assistant
(**Settings → Voice assistants → Expose**; `EXPOSE_ASSIST=true`, the
default) **or** named in `INCLUDE` **or** carries a label of
`INCLUDE_LABELS`, **and** not named in `EXCLUDE` and carries no label of
`EXCLUDE_LABELS`, **and** neither disabled nor hidden in Home Assistant.
`INCLUDE` and `EXCLUDE` take entity ids or globs (`light.*`,
`sensor.*_temperature`), comma- or newline-separated. A label value is a
label's name (case-insensitive) or its id; an entity carries its own labels
and its device's. An override set in the web UI sits above the lists and
below disabled and hidden. go-hamapping then maps what is served, or says
why it cannot. The daemon never writes Home Assistant's exposure settings.

Example: put the label **Matter** on the device "Living room climate
sensor" in Home Assistant (**Settings → Devices → the device → Labels**) and
set `INCLUDE_LABELS=Matter` (add-on: `include_labels: [Matter]`). Its
temperature and battery entities are served — the battery on the
temperature sensor's endpoint — and the plan says "included by label
'Matter'". Removing the label from the device removes the endpoint within a
second; a label named in the options that does not exist yet is a warning,
not an error, and matches once it is created.

## Quickstart

### As a Home Assistant add-on

1. **Settings → Apps → Install app**, **⋮ → Repositories** (formerly
   *Settings → Add-ons → Add-on Store*), add
   `https://github.com/thingfabric/go-ha2fabric`.
2. Install **go-ha2fabric** and start it (its passcode is random unless you
   set one).
3. Open its **Web UI** and pair the QR code from Apple Home, Google Home or
   Alexa.

[`addon/DOCS.md`](addon/DOCS.md) has the options.

### Stand-alone

Create a long-lived access token for an administrator (the user's profile,
**Security**) and run the daemon on a host in the controllers' LAN, with
host networking:

```sh
docker run --network host --stop-timeout 30 -v ha2fabric:/data \
  -e HA_URL=ws://homeassistant.local:8123/api/websocket \
  -e HA_TOKEN=<token> -e PASSCODE=<8 digits> \
  ghcr.io/thingfabric/go-ha2fabric
```

The images moved with 0.7.0: `ghcr.io/sukramj/go-ha2fabric` and
`ghcr.io/sukramj/go-ha2fabric-addon-<arch>` stay at 0.6.3 and receive no
update; pull `ghcr.io/thingfabric/go-ha2fabric`. From the release after
0.7.0, the archives and images carry a build provenance and an SBOM attestation
(`gh attestation verify oci://ghcr.io/thingfabric/go-ha2fabric:<version> -R thingfabric/go-ha2fabric`;
for an archive, its file instead of the `oci://` reference) and the
licence texts of everything linked (`licenses/`, in the images
`/usr/share/doc/go-ha2fabric/`).

The image's web UI listens on loopback (`127.0.0.1:8099`); to reach it from
another machine add `-e WEB_LISTEN=0.0.0.0:8099`, and with `WEB_ADMIN=true`
also `-e WEB_ALLOW=<the administrators' addresses>`
([`docs/operations.md`](docs/operations.md), The web UI).

`ws://` sends the token unencrypted to that host; use `wss://` where Home
Assistant has TLS (the daemon warns about `ws://` to another host,
[`docs/operations.md`](docs/operations.md), The token).

or the release binary:

```sh
HA_URL=ws://homeassistant.local:8123/api/websocket HA_TOKEN=<token> ./ha2fabric
./ha2fabric --dump-plan     # what would be served, and why
```

The pairing code is in the log (`ha2fabric.pairing`) until the first
controller pairs, and on the web UI's pairing page (`WEB_LISTEN`, default
`127.0.0.1:8099`, with `WEB_ADMIN`).

## Configuration

Every setting is an environment variable with a flag of the same name
(`LISTEN` and `--listen`); a flag wins. The token is read from the
environment only and never logged or stored.

| Variable | Default | Meaning |
| --- | --- | --- |
| `HA_URL` | add-on: `ws://supervisor/core/websocket` | Home Assistant's WebSocket API; `wss://` where Home Assistant has TLS — with `ws://` to another host the token crosses the network unencrypted (a warning, `token_in_clear`). No user or password in the URL; a redirect is not followed. |
| `HA_TOKEN` | add-on: `SUPERVISOR_TOKEN` | An administrator's long-lived access token. |
| `LISTEN` | `:5540` | UDP address of the Matter node. |
| `MDNS_INTERFACES` | empty (every interface) | Host interfaces the mDNS responder announces on (comma- or newline-separated), for a dual-homed host behind an mDNS reflector ([`docs/operations.md`](docs/operations.md), mDNS). |
| `PASSCODE` | unset (random) | Setup passcode; trivial ones are refused. Unset, the configured node gets a random one on its first start, kept in `passcode.json` beside the store; the public test value 20202021 is honoured with a warning (`passcode_test_value`). Set it in the environment: the flag shows in the process list. |
| `DISCRIMINATOR` | `3840` | 12-bit discriminator. |
| `VENDOR_ID`, `PRODUCT_ID` | `0xFFF1`, `0x8001` | The CSA test vendor and product. |
| `LABEL` | `Matter Bridge` | The bridge's node label (at most 32 bytes). |
| `STORE_PATH` | `ha2fabric.db`; add-on `/data/ha2fabric.db` | The node's SQLite store. |
| `EXPOSE_ASSIST` | `true` | Follow Home Assistant's expose-to-Assist setting. |
| `CAMERA` | `false` | Map cameras with a live stream and relay their WebRTC signalling; also turns on Matter over TCP. Off until a live test with Apple Home has passed ([`docs/operations.md`](docs/operations.md), Cameras). |
| `INCLUDE`, `EXCLUDE` | empty | Entity ids or globs. |
| `INCLUDE_LABELS`, `EXCLUDE_LABELS` | empty | Home Assistant labels, by name or id (comma- or newline-separated). |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. |
| `NODE_LIMIT` | `8` | How many nodes the web UI may create beside the configured one, 0 to 32 ([`docs/operations.md`](docs/operations.md#nodes)). |
| `WEB_LISTEN` | `127.0.0.1:8099` | The web UI; empty disables it. |
| `WEB_ALLOW` | empty (all) | Client addresses (IP addresses) the web UI serves. An address filter, not authentication. |
| `WEB_ALLOWED_HOSTS` | empty | Stand-alone: DNS names the web UI answers under besides `localhost` and IP addresses (comma-separated host names, no port). The add-on needs none. |
| `WEB_ADMIN` | `false` | Stand-alone: make the web UI's user the administrator (every page, the writes, the pairing code), with the UI's per-process token; without it the UI is the status page only. Plain HTTP: whoever reaches the port is the administrator, so on a `WEB_LISTEN` that is not loopback (`0.0.0.0:8099`) it is refused without `WEB_ALLOW`. The add-on ignores it: there a Home Assistant administrator, through Ingress, is the administrator (Supervisor 2026.10.4 or later, [`addon/DOCS.md`](addon/DOCS.md)). |

[`docs/operations.md`](docs/operations.md) covers the files, ports, mDNS,
the token's role, logs, health, pairing, Erase and a factory reset.

## Development

```sh
make build     # go build ./... and the daemon into bin/
make test      # go test -count=1 ./...
make race      # go test -race (CGO_ENABLED=1 for this run only)
make lint      # gofumpt + golangci-lint + go mod tidy -diff
make ci        # build vet test lint
make cover-check fuzz-smoke   # the coverage floors; every fuzz target of the Makefile's FUZZ_TARGETS
make docker addon-image       # both images (amd64)
make release                  # the release dry run into dist/
```

The guards, the tool versions and the workflow files follow the sibling
repositories. `internal/arch` holds the import rule: go-fabric's facade
(`node`, `device`, `contract`, `eligibility`, `logring`, the `<cluster>def`
packages under `cluster/spec/`) and nothing below it; the camera host
alone also `cluster/camera`, and the pure core (`internal/plan`) not even
`node`. The tests run against a fake Home Assistant built
from recorded WebSocket fixtures (`testdata/ha/`) and go-fabric's in-process
controller (`fabrictest`); no live Home Assistant in CI.

## Acknowledgements

Built on [go-fabric](https://github.com/thingfabric/go-fabric), a semantic
port of [matter.js](https://github.com/matter-js/matter.js) (Apache-2.0),
and on the [Home Assistant](https://www.home-assistant.io/) APIs. The
embedding follows go-fabric's cookbook "Embed in a daemon", written
against go-zendure2mqtt.

## License

Apache-2.0, see [LICENSE](LICENSE). Contributions are accepted under the
[contributor licence agreement](CLA.md); the use of AI tools is governed by
[`AI_POLICY.md`](AI_POLICY.md).
