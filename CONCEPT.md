# go-habridge: concept

Status: concept, not started. Written 2026-10-24 from go-fabric's concept
v6 part B (2026-10-20) and the owner's decisions of 2026-10-24 (go-fabric
concept v8: `go-fabric-server`). Changes to this document follow those
decisions; the go-fabric roadmap names the milestone in which work starts.

## Purpose

Home Assistant entities as a bridge for Matter controllers (Apple Home,
Google Home, Amazon Alexa): the role
[home-assistant-matter-bridge](https://github.com/Nabu-Casa/home-assistant-matter-bridge)
plays on matter.js, as a static Go binary and a Home Assistant add-on.

## Shape (decision of 2026-10-24)

The daemon is a **thin client between two WebSockets** and contains no
Matter protocol code.

- **Home Assistant side.** The Home Assistant WebSocket API: the entity
  and device registries decide what is exposed; `state_changed` events
  carry the values; `call_service` executes a controller's command. The
  exposure follows Home Assistant's own "expose" settings where they
  exist, with an allow list in the add-on configuration as the fallback.
- **Matter side.** `go-fabric-server`, the stand-alone node of go-fabric
  with a remote protocol (WebSocket, JSON, a schema number): the bridge
  sends a **declaration** of its endpoints (device type, clusters, initial
  values by name), **state deltas** as entities change, and the **result**
  of a command the node forwarded; it receives **commands** (an invoke or
  a write with a correlation id and a deadline) and **events** (pairing
  state, a fabric added or removed). The node owns pairing, fabrics,
  sessions, subscriptions, persistence and mDNS.
- **The mapping.** Which component with which attributes becomes which
  device type and cluster, and which Matter command becomes which service
  call, is the table of `go-hafabric`; this daemon applies it and holds no
  table of its own.

In go-fabric's concept v6 part B the daemon embedded go-fabric in
process through its device kit. That route still exists for Go daemons
(go-fabric's cookbook "Embed in a daemon"); for this product the owner
chose the out-of-process shape so that the Matter stack, its persistence
and its image have one home (`go-fabric-server`) and this repository is
small enough to be a Home Assistant add-on with a short review surface.

## Tests

Against go-fabric's in-process test package (`fabrictest`, an in-process
controller) through `go-fabric-server`'s own test harness once it exists:
the bridge is commissioned, a Home Assistant entity appears as an
endpoint, a controller's invoke becomes a `call_service`, a
`state_changed` becomes an attribute report. A recorded Home Assistant
WebSocket session is the fixture; no live Home Assistant in CI.

## Packaging

A distroless, non-root image; a Home Assistant add-on (`addon/`,
`repository.yaml`) in the form of the `go-*2mqtt` family; release
binaries for linux amd64, arm64 and arm/v7. The add-on directory and its
workflows exist here as disabled placeholders until the first release.

## Sequence

1. go-fabric closes milestone 4 (`v0.11.0`).
2. go-fabric concept v8 fixes the remote protocol of `go-fabric-server`
   (message names, the declaration document, the schema number); its
   first release carries the device side of the protocol.
3. `go-hafabric` publishes the table in a form this daemon can apply.
4. This repository starts: the Home Assistant client, the protocol
   client, the add-on.

## Open decisions

- Which Home Assistant exposure settings are honoured and in which order
  (the Matter integration's "expose" flags, the conversation exposure,
  an add-on allow list).
- Whether the add-on runs `go-fabric-server` as a second process in the
  same container or expects it as its own add-on.
- The first supported component set (concept v6 named `light`, `switch`,
  `cover`, `climate`, `lock`, `fan`, `sensor`, `binary_sensor`; the table
  decides).

## References

- go-fabric: [README](https://github.com/SukramJ/go-fabric#readme),
  [roadmap](https://github.com/SukramJ/go-fabric/blob/main/docs/roadmap.md),
  [concept v6](https://github.com/SukramJ/go-fabric/blob/main/docs/history/concept-v6-embedding-layer.md)
  part B (the origin of this document),
  [concept v7](https://github.com/SukramJ/go-fabric/blob/main/docs/concept-v7-operation-and-packaging.md)
  decisions 0.4, 0.6, 0.9.
- matterjs-server: the WebSocket client pattern Home Assistant already
  uses for Matter ([repository](https://github.com/matter-js/matterjs-server)).
