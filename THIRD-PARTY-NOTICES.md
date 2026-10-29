# Third-Party Notices

go-ha2fabric is licensed under the Apache License 2.0 (see
[`LICENSE`](LICENSE)). It depends on five Go modules directly; every
further dependency is a decision recorded here (MIT, Apache-2.0, BSD and
the equivalent ISC only, checked by `make licenses`). No third-party
source is copied into this tree. The recorded fixtures under
`testdata/ha/` are shapes of Home Assistant's WebSocket messages written
for the tests (each names the Home Assistant core lines it mirrors), not
copies of its source.

## Direct dependencies

### go-fabric — Apache-2.0

The Matter side: the `node` facade, the device kit (`device`), `contract`,
`eligibility` and the generated `<cluster>def` packages; in tests its
in-process controller (`fabrictest`).

- Source: <https://github.com/SukramJ/go-fabric>, pinned `v0.14.0`.
- Copyright (C) 2026 SukramJ. Apache License 2.0; its own notices
  (matter.js, filippo.io/nistec, modernc.org/sqlite, …) travel in its
  `THIRD-PARTY-NOTICES.md` and `licenses/`.
- Decision: concept v8 decision 0.2 — the daemon embeds go-fabric in
  process.

### go-hamapping — Apache-2.0

The table: which Home Assistant entity becomes which device type and
cluster, and which controller command becomes which entity command.

- Source: <https://github.com/SukramJ/go-hamapping>, pinned `v0.2.1`.
- Copyright (C) 2026 SukramJ. Apache License 2.0.
- Decision: CONCEPT.md, Shape — the mapping is go-hamapping's.

### go-fabric-ui — Apache-2.0

The web interface: the `fabricui` handlers and the built bundle embedded in
the module (Svelte 5 pages compiled at go-fabric-ui's release; no Node.js
in this build).

- Source: <https://github.com/SukramJ/go-fabric-ui>, pinned `v0.2.4`.
- Copyright (C) 2026 SukramJ. Apache License 2.0; the bundle's own
  notices travel in its `THIRD-PARTY-NOTICES.md`.
- Decision: go-fabric concept v9 — one UI for the family's daemons
  (brief H3 §2.8).

### go-hamqtt — MIT

The vocabulary go-hamapping reads: `model.Device`, `model.Entity`,
`model.State`, `model.Command`.

- Source: <https://github.com/SukramJ/go-hamqtt>, pinned `v0.39.0`.
- Copyright (c) 2026 SukramJ. MIT License.

### go-ha-catalog — MIT

Home Assistant's tables; the daemon reads the MQTT schema's payload
defaults the table's commands render.

- Source: <https://github.com/SukramJ/go-ha-catalog>, at `d9582e02`
  (snapshot 2026.10.3; `v0.3.2-0.20261024093528-d9582e022b8f`, the
  version go-hamapping pins).
- Copyright (c) 2026 SukramJ. MIT License.

### github.com/coder/websocket — ISC

The WebSocket transport of the Home Assistant client: pure Go,
context-aware, concurrent writes; the server family's choice.

- Source: <https://github.com/coder/websocket>, `v1.8.15`.
- Copyright (c) 2025 Coder. ISC License, see
  [`licenses/LICENSE-coder-websocket.txt`](licenses/LICENSE-coder-websocket.txt).

## Indirect dependencies

Through go-fabric: filippo.io/nistec (BSD-3-Clause), the pure-Go SQLite
driver modernc.org/sqlite and its modernc.org modules (BSD-3-Clause),
github.com/grandcat/zeroconf (MIT), github.com/miekg/dns (BSD-3-Clause),
golang.org/x/net and golang.org/x/sys (BSD-3-Clause), github.com/google/uuid
(BSD-3-Clause) and the others go-fabric's notices list.

Through go-fabric-ui (since its `v0.2.x` controller API, pinned here from
go-ha2fabric 0.3.0): github.com/oapi-codegen/runtime (Apache-2.0), the
runtime of the generated API types, and github.com/apapsch/go-jsonmerge/v2
(MIT), which it uses. Decision: taken with the go-fabric-ui `v0.2.2` bump
for its Home Assistant theme fixes (2026-10-27). `go mod graph` is the
complete list.
