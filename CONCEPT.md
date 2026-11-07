# go-ha2fabric: concept

Status: implemented in 0.1.0 (brief H2, 2026-10-25); labels, the web UI
from go-fabric-ui and the stable entity identity in 0.2.0 (brief H3). Written 2026-10-24
from go-fabric's concept v6 part B (2026-10-20) and the owner's decisions
of 2026-10-24 (go-fabric concept v8: `go-fabric-server`; evening: this
daemon embeds go-fabric). Changes to this document follow those decisions.

## Purpose

Home Assistant entities as a bridge for Matter controllers (Apple Home,
Google Home, Amazon Alexa): the role
[home-assistant-matter-hub](https://github.com/RiDDiX/home-assistant-matter-hub)
plays on matter.js, as a static Go binary and a Home Assistant add-on.
matter-hub is the helper reference for how a real Home Assistant bridge
behaves (cited `file:line` at a pinned commit, `docs/by_design.md`); it is
never a reference for Matter behaviour, which is go-fabric's.

## Shape (decisions of 2026-10-24, evening)

The daemon **embeds go-fabric in process**, the way go-zendure2mqtt does in
go-fabric's cookbook "Embed in a daemon" (wave CE): one process, one store,
one add-on, no protocol between two Go programs.

- **Home Assistant side.** The Home Assistant WebSocket API: the entity
  and device registries and the exposure list decide what is exposed;
  `state_changed` events carry the values; `call_service` executes a
  controller's command. The exposure rule is decided below.
- **Matter side.** go-fabric's `node` facade and device kit: each exposed
  entity is a typed declaration under the node's Aggregator; a controller's
  invoke or write reaches the daemon as a handler call and becomes a
  `call_service`. Pairing, fabrics, sessions, subscriptions, persistence
  and mDNS are the library's.
- **The mapping.** Which component with which attributes becomes which
  device type and cluster, and which Matter command becomes which service
  call, is the table of `go-hamapping` (a Go module on go-hamqtt's
  vocabulary); this daemon maps a registry entity onto that vocabulary
  with a small adapter and applies the table.

`go-fabric-server` is a different product: the Matter **controller** as a
service (the role Home Assistant's Matter integration talks to) with a
remote API, plus a bridge door for hosts that are not written in Go. A
Go daemon does not need it; this one does not use it.

## Tests

Against go-fabric's in-process test package (`fabrictest`, an in-process
controller): the bridge is commissioned, a Home Assistant entity appears as an
endpoint, a controller's invoke becomes a `call_service`, a
`state_changed` becomes an attribute report. A recorded Home Assistant
WebSocket session is the fixture (`testdata/ha/`, each file citing the
Home Assistant core lines it mirrors); no live Home Assistant in CI.

## Packaging

A distroless, non-root image; a Home Assistant add-on (`addon/`,
`repository.yaml`) in the form of the `go-*2mqtt` family; release
binaries for linux amd64, arm64 and arm/v7. The add-on runs on the host's
network (mDNS and UDP 5540) and serves its web UI through Ingress only.

## Name

`go-ha2fabric` follows the family's `go-<source>2<target>` pattern
(go-zendure2mqtt): Home Assistant as the source, the Matter fabric as the
target. The repository was created as `go-habridge` on 2026-10-24 and
renamed the same day; GitHub redirects the old name.

## Sequence

1. go-fabric closes milestone 4 (`v0.11.0`); its cookbook wave has shown
   one device type end to end on the kit from a `go-*2mqtt` daemon.
2. `go-hamapping` publishes the table's Go form with its first rows.
3. This repository starts: the Home Assistant client, the registry
   adapter onto go-hamqtt's vocabulary, the embedding, the add-on.

## Decided (brief H2, 2026-10-25)

- **Exposure.** An entity is exposed when it is exposed to Assist
  (`conversation`) in `homeassistant/expose_entity/list`, or named in the
  `include` option (entity ids or globs), and not named in `exclude`; a
  disabled or hidden registry entry never is. Home Assistant has no
  Matter-specific expose flag (its assistants are `conversation`,
  `cloud.alexa` and `cloud.google_assistant`, core
  `homeassistant/exposed_entities.py:25` at 2026.10.3); Assist's list is
  the one every installation has. The daemon never writes exposure
  settings. `--dump-plan` and the web UI show every candidate's verdict.
- **The component set** is go-hamapping's table at `v0.1.0`: `light`,
  `switch`, `cover`, `climate`, `lock`, `fan`, `sensor`, `binary_sensor`;
  the daemon adds none of its own.

## Decided (owner, 2026-11-05)

Against go-fabric's architecture standard (its `CLAUDE.md`, "The
architecture standard"). The first bullet is built (A1, report
`docs/reports/A1.md`). The two remaining bullets are not built yet;
`docs/roadmap.md` tranche A2 carries them, and the sections above describe
today's state until it lands.

- **The internal cut** (built in A1). `cmd/ha2fabric` is the one composition root;
  `internal/plan` a pure core (no node, no goroutines); `internal/bridge`
  only the reconciler; `internal/exposure` the web UI's provider. An
  incremental mirror of the registries was rejected: it gives up what a
  full read gives after a reconnect, for a gain nobody has measured.
- **No Matter types in the adapter.** `internal/adapt` imports nothing of
  go-fabric. What it knows of Matter today (`command.go:15-16`, `:208-209`,
  `:276-284`) moves to go-hamapping, where "The mapping" above already
  places it.
- **go-hamapping's own vocabulary.** The `go-*2mqtt` daemons are not planned
  as consumers. go-hamqtt's vocabulary was chosen so that one table serves
  them too (go-hamapping `CONCEPT.md:20-22, 26-29`); without them it is a
  detour: a WebSocket entity renamed into MQTT terms (`adapt.go:11-15`) and
  a command rendered as an MQTT payload that `command.go` parses back. The
  table gets a vocabulary of its own with structured commands.

## Better, before, the right cut

The three questions of the standard, for the target above. They compare
design, not measured behaviour.

- **Better than matter-hub?** In its form and in the single place for a
  device's shape: one static binary embedding the protocol in process, and
  one table that decides what a device becomes. Not everywhere: matter-hub
  offers user-defined composed entities, go-ha2fabric composes nothing
  (`docs/by_design.md`, Composition). That is a recorded decision, not an
  advantage.
- **Better than before?** Yes. Before A1, at `34075a6`, `Bridge.sync` held
  planning, the diff, the forget and replace rules, the batch and the
  status in one method (`internal/bridge/bridge.go:688-880`), so those
  rules were reachable only through a node. The rule with its overrides
  existed in the bridge (`bridge.go:883-903`), in `--dump-plan`
  (`cmd/ha2fabric/main.go:219-226`) and twice more, varied, in the provider
  (`provider.go:196-213`, `:255-259`), and the bridge imported `fabricui`
  and, for `NodeConfig` alone, `config` (`node.go:39`; called for the
  configured node at `main.go:97` and for each registry node at
  `nodes.go:163`). Since A1 the rules are `plan.Diff`'s, tested without a
  node; the rule meets its overrides in `plan.Effective` alone; the
  bridge imports neither `fabricui` nor `config`.
- **The right cut?** go-fabric owns Matter. go-hamapping owns what an entity
  and a command become, including the Matter half of a command. This
  repository owns Home Assistant's wire, the exposure rule and the
  reconciliation. go-fabric-ui owns the web UI, behind `internal/exposure`.
  go-fabric-server is not used (Shape, above). Since U6b (#36) one host
  serves several nodes: each has its own bridge and, after A1, its own plan
  input and exposure provider, composed per node by `cmd/ha2fabric`'s
  `serve` (`nodes.go`); the registry of those nodes (`internal/nodes`) and
  the fan-out of the one Home Assistant connection (`internal/ha`) are
  daemon state and stay here.

## References

- go-fabric: [README](https://github.com/SukramJ/go-fabric#readme),
  [roadmap](https://github.com/SukramJ/go-fabric/blob/main/docs/roadmap.md),
  [concept v6](https://github.com/SukramJ/go-fabric/blob/main/docs/history/concept-v6-embedding-layer.md)
  part B (the origin of this document),
  [concept v7](https://github.com/SukramJ/go-fabric/blob/main/docs/concept-v7-operation-and-packaging.md)
  decisions 0.4, 0.6, 0.9.
- matterjs-server: the WebSocket client pattern Home Assistant already
  uses for Matter ([repository](https://github.com/matter-js/matterjs-server)).
