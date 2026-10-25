# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

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

[Unreleased]: https://github.com/SukramJ/go-ha2fabric/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/SukramJ/go-ha2fabric/releases/tag/v0.1.0
