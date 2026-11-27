# Third-Party Notices

go-hamapping is licensed under the Apache License 2.0 (see
[`LICENSE`](LICENSE)). It depends on four Go modules directly; every
further dependency is a decision recorded here (MIT, Apache-2.0 and BSD
only, checked by `make licenses`). No third-party source is copied into this
tree.

## Direct dependencies

### go-fabric — Apache-2.0

The Matter side: the device kit (`device`), `contract`, `eligibility` and
the generated `<cluster>def` packages the rows are typed by; in tests its
in-process controller (`fabrictest`).

- Source: <https://github.com/thingfabric/go-fabric>, module `thingfabric.dev/go-fabric`, pinned `v0.28.0`.
- Copyright (C) 2026 SukramJ. Apache License 2.0; its own notices
  (matter.js, filippo.io/nistec, modernc.org/sqlite, …) travel in its
  `THIRD-PARTY-NOTICES.md` and `licenses/`.
- Decision: concept v8 B.1, brief H1 — the one place of Matter knowledge.

### go-hamqtt — MIT

The Home Assistant vocabulary the table reads: `model.Device`,
`model.Entity`, `model.State`, `model.Command`.

- Source: <https://github.com/SukramJ/go-hamqtt>, pinned `v0.38.0`.
- Copyright (c) 2026 SukramJ. MIT License.
- Decision: concept v8 decision 0.4, brief H1 — the input is go-hamqtt's
  vocabulary.

### go-ha-catalog — MIT

Home Assistant's tables: platforms, device classes, units, feature flags,
the MQTT schema with its defaults.

- Source: <https://github.com/SukramJ/go-ha-catalog>, pinned at
  `d9582e02` (snapshot 2026.10.3; `v0.3.2-0.20261024093528-d9582e022b8f`).
- Copyright (c) 2026 SukramJ. MIT License.
- Decision: brief H1 hard rule 2 — Home Assistant facts come from the
  catalog's tables.

### gopkg.in/yaml.v3 — MIT and Apache-2.0

The row files (`rows/*.yaml`), read by `internal/rowfile` for the generator
and its tests only; the generated Go form does not import it.

- Source: <https://github.com/go-yaml/yaml>, `v3.0.1`.
- Copyright (c) 2006-2011 Kirill Simonov (MIT), Copyright (c) 2011-2019
  Canonical Ltd (Apache-2.0).
- Decision: brief H1 — YAML rows. A new module in the graph: neither
  go-fabric nor go-hamqtt carries it.

## Indirect dependencies

The modules go-fabric and go-hamqtt bring (`go list -m all`); their
licences are checked by `make licenses` and listed in go-fabric's notices.
