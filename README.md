# go-ha2fabric

[![ci](https://github.com/SukramJ/go-ha2fabric/actions/workflows/ci.yml/badge.svg)](https://github.com/SukramJ/go-ha2fabric/actions/workflows/ci.yml)
[![codeql](https://github.com/SukramJ/go-ha2fabric/actions/workflows/codeql.yml/badge.svg)](https://github.com/SukramJ/go-ha2fabric/actions/workflows/codeql.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)

A Home Assistant installation seen by Apple Home, Google Home and Amazon
Alexa as one bridge: the entities Home Assistant exposes become endpoints
of a bridged node. The daemon embeds [go-fabric](https://github.com/SukramJ/go-fabric) in
process: the Home Assistant WebSocket API (entity and device registry,
`state_changed`, `call_service`) on one side, go-fabric's device kit and
`node` facade speaking Matter to the controllers on the other, one
process and one store. Which
Home Assistant component becomes which device type and cluster is the
table of [`go-hamapping`](https://github.com/SukramJ/go-hamapping).

Twin of [`go-hamapping`](https://github.com/SukramJ/go-hamapping); both are built on
[go-fabric](https://github.com/SukramJ/go-fabric) and share its project
setup (pure Go, `CGO_ENABLED=0`, Apache-2.0 with a contributor licence
agreement, the same guards in CI).

> **Status: 0.2.3.** The daemon and its Home Assistant add-on run; a real
> pairing with Apple Home is the owner's manual step before the release is
> announced. The design is in [`CONCEPT.md`](CONCEPT.md).

## Why "Matter" is not in the name

"Matter" and the Matter logo are trademarks of the Connectivity Standards
Alliance. Their use in a product, project or repository name, as on a
device, is reserved to Alliance members and certified products under the
*Connectivity Standards Alliance Trademark and Logo Usage Guidelines and
Terms* (<https://csa-iot.org/resources/governing-documents/>; the terms are
restated in go-fabric's
[`licenses/NOTICE-matter.js.txt`](https://github.com/SukramJ/go-fabric/blob/main/licenses/NOTICE-matter.js.txt):
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
  device types and clusters: lights (on/off, dimmable, colour temperature,
  extended colour), switches, sensors (temperature, humidity, pressure,
  illuminance, power, voltage, current, energy, with a device's battery on
  its main endpoint), binary sensors (contact, motion, occupancy, water
  leak, smoke, carbon monoxide), covers, climate, locks and fans.
- **Both directions.** `state_changed` becomes an attribute report to every
  subscribed controller; a controller's command becomes a `call_service`.
- **Live topology.** A renamed, disabled, hidden or newly exposed entity
  moves the bridge at run time; the other endpoints keep their numbers.
- **Reachability.** An `unavailable` or `unknown` entity is unreachable;
  while Home Assistant is away, every endpoint is.
- **The family's web UI** ([go-fabric-ui](https://github.com/SukramJ/go-fabric-ui)
  `v0.1.2`, through Ingress in the add-on): health, the pairing code and the
  commissioning window, the fabrics, the endpoints, diagnostics, and the
  entity selection — every candidate entity with its labels, the layer of
  the rule that decided and go-hamapping's verdict, with per-entity
  overrides. `--dump-plan` prints the same table.
- **Pure Go**, `CGO_ENABLED=0`: a static binary, a distroless image, a Home
  Assistant add-on for amd64, aarch64 and armv7.

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

1. **Settings → Add-ons → Add-on Store**, **⋮ → Repositories**, add
   `https://github.com/SukramJ/go-ha2fabric`.
2. Install **go-ha2fabric**, change its `passcode`, start it.
3. Open its **Web UI** and pair the QR code from Apple Home, Google Home or
   Alexa.

[`addon/DOCS.md`](addon/DOCS.md) has the options.

### Stand-alone

Create a long-lived access token for an administrator (the user's profile,
**Security**) and run the daemon on a host in the controllers' LAN, with
host networking:

```sh
docker run --network host -v ha2fabric:/data \
  -e HA_URL=ws://homeassistant.local:8123/api/websocket \
  -e HA_TOKEN=<token> -e PASSCODE=<8 digits> \
  ghcr.io/sukramj/go-ha2fabric
```

or the release binary:

```sh
HA_URL=ws://homeassistant.local:8123/api/websocket HA_TOKEN=<token> ./ha2fabric
./ha2fabric --dump-plan     # what would be served, and why
```

The pairing code is in the log (`ha2fabric.pairing`) and on the web UI
(`WEB_LISTEN`, default `127.0.0.1:8099`).

## Configuration

Every setting is an environment variable with a flag of the same name
(`LISTEN` and `--listen`); a flag wins. The token is read from the
environment only and never logged or stored.

| Variable | Default | Meaning |
| --- | --- | --- |
| `HA_URL` | add-on: `ws://supervisor/core/websocket` | Home Assistant's WebSocket API. |
| `HA_TOKEN` | add-on: `SUPERVISOR_TOKEN` | An administrator's long-lived access token. |
| `LISTEN` | `:5540` | UDP address of the Matter node. |
| `PASSCODE` | `20202021` | Setup passcode; trivial ones are refused. Change it. |
| `DISCRIMINATOR` | `3840` | 12-bit discriminator. |
| `VENDOR_ID`, `PRODUCT_ID` | `0xFFF1`, `0x8001` | The CSA test vendor and product. |
| `LABEL` | `Matter Bridge` | The bridge's node label (at most 32 bytes). |
| `STORE_PATH` | `ha2fabric.db`; add-on `/data/ha2fabric.db` | The node's SQLite store. |
| `EXPOSE_ASSIST` | `true` | Follow Home Assistant's expose-to-Assist setting. |
| `INCLUDE`, `EXCLUDE` | empty | Entity ids or globs. |
| `INCLUDE_LABELS`, `EXCLUDE_LABELS` | empty | Home Assistant labels, by name or id (comma- or newline-separated). |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. |
| `WEB_LISTEN` | `127.0.0.1:8099` | The web UI; empty disables it. |
| `WEB_ALLOW` | empty (all) | Client addresses the web UI serves. |
| `WEB_ADMIN` | `false` | Stand-alone: let the web UI write (selection, window, fabrics) and show the pairing code, with the UI's per-process token. The add-on ignores it: the Ingress proxy is the administrator. |

[`docs/operations.md`](docs/operations.md) covers the files, ports, mDNS,
the token's role, logs, health, pairing and a factory reset.

## Development

```sh
make build     # go build ./... (the daemon's main is a placeholder until the first release)
make test      # go test -count=1 ./...
make race      # go test -race (CGO_ENABLED=1 for this run only)
make lint      # gofumpt + golangci-lint + go mod tidy -diff
make ci        # build vet test lint
make cover-check fuzz-smoke   # the coverage floors, the decoder's fuzz target
make docker addon-image       # both images (amd64)
make release                  # the release dry run into dist/
```

The guards, the tool versions and the workflow files follow the sibling
repositories. `internal/arch` holds the import rule: go-fabric's facade
(`node`, `device`, `contract`, `eligibility`, the `<cluster>def` packages)
and nothing below it. The tests run against a fake Home Assistant built
from recorded WebSocket fixtures (`testdata/ha/`) and go-fabric's in-process
controller (`fabrictest`); no live Home Assistant in CI.

## Acknowledgements

Built on [go-fabric](https://github.com/SukramJ/go-fabric), a semantic
port of [matter.js](https://github.com/matter-js/matter.js) (Apache-2.0),
and on the [Home Assistant](https://www.home-assistant.io/) APIs. The
embedding follows go-fabric's cookbook "Embed in a daemon", written
against go-zendure2mqtt.

## License

Apache-2.0, see [LICENSE](LICENSE). Contributions are accepted under the
[contributor licence agreement](CLA.md); the use of AI tools is governed by
[`AI_POLICY.md`](AI_POLICY.md).
