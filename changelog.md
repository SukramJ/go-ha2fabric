# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.3] - 2026-10-26

The F1 follow-ups (brief F2, issue #10): go-fabric-ui `v0.1.3`.

### Changed

- An entity deleted from Home Assistant's entity registry loses its
  endpoint number (go-fabric's `Forget`, logged as `bridge.forget`): created
  again, it is served under a fresh number, as a new device. A disabled,
  hidden or unexposed entity keeps its number as before.

### Fixed

- A label whose name contains a comma ("Living, dining") is one label in
  the web UI's labels column and filter, not two: the column reads the
  item's list field (`Item.Lists["labels"]`, go-fabric-ui `v0.1.3`).

## [0.2.2] - 2026-10-26

The family bump (brief F1): go-fabric `v0.13.3`, go-fabric-ui `v0.1.2`,
go-hamapping `v0.2.1`.

### Added

- The web UI's status shows the configuration's warnings
  (`fabricui.Options.Warnings`, go-fabric-ui `v0.1.1`): a label value of
  `INCLUDE_LABELS`/`EXCLUDE_LABELS` that names no Home Assistant label
  (`label_unresolved`, the log's `bridge.label_unresolved`), and an
  overrides file that did not parse and was moved aside at start
  (`selection_corrupt`, the log's `selection.corrupt`). Before, both were
  in the log only.

### Changed

- The labels column of the web UI's selection is list-valued
  (`fabricui.Column.Multi`): the filter offers each label and matches an
  entity that carries it, not the combination of an entity's labels
  (H3 Q2).
- A rename in the web UI is the endpoint's node label as typed, without
  the device name (go-hamapping `Options.Name`, verbatim; H3 Q3). Before,
  go-hamapping composed it with the device name.
- An entity whose endpoint is removed and added again — turned off and on
  in the web UI, or left out by the rule and let in again — gets its
  endpoint number back (go-fabric `v0.12.1`, `node.Aggregator.Remove`
  keeps the number); go-fabric `v0.13.0` keeps a stored number until
  `Forget`, so the daemon's first-sync `MarkComplete` call is gone.
- The web UI's event stream observes the node itself (go-fabric-ui
  `v0.1.2`); the daemon no longer wires `Events.Observers` into the node's
  configuration.

## [0.2.1] - 2026-10-25

The add-on icon, and a sync that survives a non-string device identifier.

### Added

- The add-on's icon and logo (`addon/icon.png`, `addon/logo.png`, sources
  beside them as SVG): the home on go-fabric-ui's fabric. The add-on store
  showed the Supervisor's default before.

### Fixed

- `config/device_registry/list` no longer fails the sync
  (`bridge.sync_failed`, "cannot unmarshal number into ... identifiers")
  when an integration registers a device identifier or connection whose
  value is not a string. Home Assistant declares both as pairs of strings
  but does not validate the element types, so a number reaches the wire as
  a JSON number; the daemon now reads it as its decimal text.

## [0.2.0] - 2026-10-25

Labels, the family's web UI, and the stable entity identity (brief H3).

### Added

- Exposure by Home Assistant label: `INCLUDE_LABELS` / `EXCLUDE_LABELS`
  (add-on `include_labels`, `exclude_labels`). A value matches a label's
  name case-insensitively, else its `label_id`; an entity carries its own
  labels and its device's. The order is disabled, hidden, the web UI's
  override, exclude (ids, globs, labels), include (ids, globs, labels),
  Assist. A value that names no label is a warning in the log
  (`bridge.label_unresolved`), not an error.
- `config/label_registry/list` and the `label_registry_updated`
  subscription; a label change re-plans the bridge.
- The web UI is go-fabric-ui `v0.1.0` (`fabricui.Handler`): health,
  pairing and the commissioning window, fabrics, endpoints, diagnostics,
  and the entity selection with preview and per-entity overrides (enable,
  disable, rename) in `selection.json` beside the store
  (`/data/selection.json` in the add-on). Administrator: the Ingress proxy
  in the add-on; stand-alone read-only unless `WEB_ADMIN=true` (the UI's
  token mode).
- The plan (`--dump-plan`, the UI's selection) carries each entity's labels and
  the deciding layer (`source`).

### Changed

- **Endpoint numbers change once at this upgrade.** An endpoint's stable
  key is now the device's registry id and the entity's registry entry id
  (go-hamapping `v0.2.0` `Options.StableKey`), so a renamed entity id keeps
  its endpoint; every existing endpoint gets a new number once, and a
  controller re-learns the bridge. What Apple Home does with the old
  devices is not recorded in the H2 report (a real pairing was the owner's
  step) nor in matter-hub; matter-hub's code says a lost endpoint number
  makes controllers "treat the recreated device as new and lose groups",
  naming Alexa (`packages/backend/src/services/bridges/bridge-endpoint-manager.ts:968-971`,
  #438 at `:55-58`, at `664575a`). Expect to put the devices into rooms and
  groups again.
- go-fabric `v0.12.0`, go-hamapping `v0.2.0`; the command mapping uses
  go-hamapping's method constants.

### Removed

- `GET /api/health` and `GET /api/plan` (0.1.0's JSON routes): the web UI's
  `GET /api/v1/node/status` (the node's health, endpoints, fabrics, window)
  and `GET /api/v1/node/exposure` (every candidate entity with its verdict,
  labels and deciding layer) replace them.
- The page of 0.1.0 and its QR encoder (`internal/web/assets`, its golden
  test): go-fabric-ui's pairing page replaces them.

## [0.1.0] - 2026-10-25

The first release: the daemon and its Home Assistant add-on (brief H2).

### Added

- **The bridge.** Home Assistant's entities, read over its WebSocket API,
  served as the bridged endpoints of one go-fabric node (`v0.11.0`) to
  Matter controllers. go-hamapping (`v0.1.0`) decides every device type and
  cluster: lights (on/off, dimmable, colour temperature, extended colour),
  switches, sensors (temperature, humidity, pressure, illuminance, power,
  voltage, current, energy; a battery rides on its device's primary
  endpoint), binary sensors (contact, motion, occupancy, water leak, smoke,
  carbon monoxide), covers, climate, locks and fans.
- **The Home Assistant client** (`internal/ha`): the auth handshake, the
  id-tagged command envelope, ping keepalive, reconnect with back-off and a
  full re-sync after every reconnect; `config/entity_registry/list`,
  `config/device_registry/list`, `config/area_registry/list`, `get_states`,
  `get_config`, `homeassistant/expose_entity/list`; `state_changed`,
  `entity_registry_updated` and `device_registry_updated` move the topology
  at run time; a controller's command leaves as `call_service`.
- **The exposure rule:** an entity is exposed when it is exposed to Assist
  in Home Assistant, or named in `INCLUDE` (entity ids or globs), and not
  named in `EXCLUDE`; disabled and hidden entities never are. The daemon
  never writes Home Assistant's exposure settings. `--dump-plan` prints
  every candidate entity with the rule's word and go-hamapping's verdict.
- **Reachability:** an entity's `unavailable` or `unknown` state makes its
  endpoint unreachable; while Home Assistant is away every endpoint is.
- **The web UI** (Ingress in the add-on): the pairing QR code, rendered in
  the browser, and the manual code, the commissioned fabrics, the
  candidate entities with their verdicts, the Home Assistant connection
  and the node's health; `GET /api/health`, `GET /api/plan`.
- **Configuration** from the environment and flags (`HA_URL`, `HA_TOKEN`,
  `LISTEN`, `PASSCODE`, `DISCRIMINATOR`, `VENDOR_ID`, `PRODUCT_ID`, `LABEL`,
  `STORE_PATH`, `INCLUDE`, `EXCLUDE`, `EXPOSE_ASSIST`, `LOG_LEVEL`,
  `WEB_LISTEN`, `WEB_ALLOW`), validated as a whole; the token is read from
  the environment only and never logged or stored.
- **Packaging:** the Home Assistant add-on (`addon/`, `repository.yaml`,
  host networking, Ingress, the Supervisor's WebSocket proxy), the
  distroless daemon image, release archives for linux amd64, arm64 and
  arm/v7.
- **Guards:** the import guard (`internal/arch`: go-fabric's facade only),
  per-package coverage floors, a fuzz target on the message decoder, the
  gitleaks history scan, both Docker builds in CI, the release dry run and
  the version check (`internal/version`, `addon/config.yaml` and this
  file agree).

The bridge carries the CSA test vendor and go-fabric's test attestation
chain: it is uncertified, and a controller may warn when pairing it.

[Unreleased]: https://github.com/SukramJ/go-ha2fabric/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/SukramJ/go-ha2fabric/releases/tag/v0.2.1
[0.2.0]: https://github.com/SukramJ/go-ha2fabric/releases/tag/v0.2.0
[0.1.0]: https://github.com/SukramJ/go-ha2fabric/releases/tag/v0.1.0
