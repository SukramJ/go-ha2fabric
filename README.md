# go-habridge

[![ci](https://github.com/SukramJ/go-habridge/actions/workflows/ci.yml/badge.svg)](https://github.com/SukramJ/go-habridge/actions/workflows/ci.yml)
[![codeql](https://github.com/SukramJ/go-habridge/actions/workflows/codeql.yml/badge.svg)](https://github.com/SukramJ/go-habridge/actions/workflows/codeql.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)

A Home Assistant installation seen by Apple Home, Google Home and Amazon
Alexa as one bridge: the entities Home Assistant exposes become endpoints
of a bridged node. The daemon is a thin client between two WebSockets. On
one side the Home Assistant WebSocket API (entity and device registry,
`state_changed`, `call_service`); on the other side `go-fabric-server`,
the stand-alone node of [go-fabric](https://github.com/SukramJ/go-fabric)
with a remote protocol, which speaks Matter to the controllers. Which
Home Assistant component becomes which device type and cluster is the
table of [`go-hafabric`](https://github.com/SukramJ/go-hafabric).

Twin of [`go-hafabric`](https://github.com/SukramJ/go-hafabric); both are built on
[go-fabric](https://github.com/SukramJ/go-fabric) and share its project
setup (pure Go, `CGO_ENABLED=0`, Apache-2.0 with a contributor licence
agreement, the same guards in CI).

> **Status: concept, not started.** The design is in [`CONCEPT.md`](CONCEPT.md).
> Work begins after go-fabric closes its milestone 4 and writes the concept
> of `go-fabric-server`; the go-fabric
> [roadmap](https://github.com/SukramJ/go-fabric/blob/main/docs/roadmap.md)
> names the milestone. Nothing in this repository runs yet.

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

Planned; see [`CONCEPT.md`](CONCEPT.md). Nothing is implemented.

## Quickstart

There is nothing to run yet.

## Configuration

Defined with the first release.

## Development

```sh
make build     # go build ./... (the daemon's main is a placeholder until the first release)
make test      # go test -count=1 ./...
make race      # go test -race (CGO_ENABLED=1 for this run only)
make lint      # gofumpt + golangci-lint + go mod tidy -diff
make ci        # build vet test lint
```

The guards, the tool versions and the workflow files follow the sibling
repositories; what a concept repository cannot run yet is disabled in the
workflow with a comment that says when it is switched on.

## Acknowledgements

Built on [go-fabric](https://github.com/SukramJ/go-fabric), a semantic
port of [matter.js](https://github.com/matter-js/matter.js) (Apache-2.0),
and on the [Home Assistant](https://www.home-assistant.io/) APIs. The
pattern of a thin client between Home Assistant and a Matter process is
the one Home Assistant uses for its own Matter support
([matterjs-server](https://github.com/matter-js/matterjs-server)).

## License

Apache-2.0, see [LICENSE](LICENSE). Contributions are accepted under the
[contributor licence agreement](CLA.md); the use of AI tools is governed by
[`AI_POLICY.md`](AI_POLICY.md).
