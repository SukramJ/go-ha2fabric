# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Home Assistant entities as a bridge for Matter controllers, in pure Go on go-fabric. Twin of `go-hamapping`; both are built on
[go-fabric](https://github.com/SukramJ/go-fabric) and share its project
setup. **State: 0.2.0** (brief H3): the daemon `cmd/ha2fabric` and its
add-on. `CONCEPT.md` is the design, `docs/operations.md` the operator's
page, `docs/by_design.md` the differences from matter-hub,
`docs/reports/H2.md` and `H3.md` the waves' reports. Layout: `internal/ha`
(the WebSocket client, `hatest` its fake on the fixtures in `testdata/ha/`),
`internal/expose` (the exposure rule with labels and overrides),
`internal/selection` (the web UI's overrides file), `internal/adapt` (HA
onto go-hamqtt's model, a command back to `call_service`),
`internal/bridge` (the composition on go-fabric's node and go-hamapping's
plans, and the web UI's exposure provider), `internal/config`,
`internal/web` (go-fabric-ui's handler behind the client filter),
`internal/arch` (the import guard).

## Commands

```sh
make ci        # build vet test lint — what the pipeline runs
make race      # go test -race (CGO_ENABLED=1 for this run only)
make fmt       # gofumpt -w .
```

Go 1.27.3, `CGO_ENABLED=0`. Dependencies: go-fabric, go-fabric-ui,
go-hamapping, go-hamqtt (`model`), go-ha-catalog, coder/websocket — `THIRD-PARTY-NOTICES.md`.
Every further dependency is a decision: Apache-2.0 / MIT / BSD / ISC only,
and it updates the notices.

More guards: `make cover-check` (floors in `script/coverfloor/floors.txt`,
a ratchet), `make fuzz-smoke`, `make secrets`, `make docker addon-image`,
`make release` (the dry run; `make version-check` holds
`internal/version`, `addon/config.yaml` and `changelog.md` together).

Home Assistant facts come from `home-assistant/core` at a pinned tag
(2026.10.3), read from its raw files and cited `file:line`; never run.
How a real Home Assistant bridge behaves: the helper reference is
[home-assistant-matter-hub](https://github.com/RiDDiX/home-assistant-matter-hub)
at a pinned commit (`664575a`), cited `file:line` in `docs/by_design.md` —
never a gold standard for Matter behaviour, which stays go-fabric's.

## Rules

- **Naming.** "Matter" is a trademark of the Connectivity Standards
  Alliance and is not part of this project's name, its packages, its
  binaries or its images; it appears only descriptively. See the README.
- **No mapping of our own.** Which entity becomes which device type and
  cluster is go-hamapping's `Map`; the daemon translates Home Assistant's
  registry and states into go-hamqtt's model (`internal/adapt`) and
  applies the plan. A gap there is a question to go-hamapping, not a
  workaround here.
- **Credentials** (the long-lived token, `SUPERVISOR_TOKEN`) come from the
  environment only, and are never logged or persisted.
- **go-fabric is the gold standard** for everything on the Matter side:
  device types, clusters, the facade (`node`, `device`, `contract`, the
  `<cluster>def` packages). This repository never reimplements a
  cluster or reaches below go-fabric's facade.
- **Which model does which work** (as go-fabric's CLAUDE.md): analysis,
  planning, anything that becomes a fact about Matter or Home Assistant
  in the repository is the strongest model's; implementation of a
  finished plan with guards behind it may use a cheaper one; a
  generated or formatted artefact a test checks may use the cheapest.
- **Official documents in English**: issues, pull requests, docs, notes.
- **One pull request per wave**, squash-merged; CI is the gate.
- **No attribution lines** in commits or pull request bodies.
- A user-visible change gets a `changelog.md` entry under `[Unreleased]`.
