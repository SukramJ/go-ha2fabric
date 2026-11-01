# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.5.1] - 2026-11-01

Dependency release: go-fabric `v0.15.6`, go-fabric-ui `v0.2.9`, go-hamapping
`v0.4.1`.

### Changed

- go-hamapping `v0.4.1` (was `v0.4.0`): its go-fabric `v0.15.6` bump, no
  row changes.
- go-fabric `v0.15.6` (was `v0.15.5`), which answers the two questions of
  go-fabric #356: `internal/camera` takes `camera.ErrPayloadTooLarge` from
  `cluster/camera` and no longer imports go-fabric's `bridge` (the import
  guard holds it to `cluster/camera`), and a camera's refused `SolicitOffer`
  answers INVALID_IN_STATE instead of FAILURE, since the kit passes a port's
  `contract.StatusError` through.
- go-fabric-ui `v0.2.9` (was `v0.2.8`; oapi-codegen runtime `v1.8.0`): the
  web UI's client no longer rejects the whole `GET /node/surface` answer
  when go-fabric adds a field to the surface report (go-fabric-ui #27).

## [0.5.0] - 2026-10-31

The camera relay (brief R6), on go-hamapping `v0.4.0`'s camera row.

### Added

- Cameras, opt-in (`CAMERA`, add-on option `camera`, default off): a
  camera entity with the `STREAM` feature becomes a Matter Camera. The
  daemon relays the WebRTC signalling — the controller's offer and
  candidates to `camera/webrtc/offer` and `camera/webrtc/candidate`, Home
  Assistant's answer and candidates to the controller's requestor,
  `EndSession` as `unsubscribe_events` — and serves a snapshot from the
  camera proxy. The stream is Home Assistant's (go2rtc or a native WebRTC
  camera). Off by default under go-fabric's ecosystem gate, until a live
  test with Apple Home has passed.
- With `CAMERA` on, the node also accepts Matter over TCP on its port: a
  signal carrying an SDP needs the controller's TCP session.

### Changed

- go-hamapping `v0.4.0`.

- go-fabric `v0.15.5`: the camera's signals to the controller's requestor
  (`camera.Provider.Offer`, `Answer`, `IceCandidates`, `End`; go-fabric #351,
  the blocker of roadmap R6), colour conversions with the same values on
  arm64 as on amd64 (the add-on's aarch64 image), and test fixes.
- The add-on's side panel is titled **Matter Server** (`panel_title` in
  `addon/config.yaml`); the bridge label controllers show stays `Matter Bridge`.
  The Supervisor reads the title from `config.yaml` only (contributors
  reference, apps/configuration: "The title of the app in the sidebar"), so
  there is no option for it.

## [0.4.0] - 2026-10-31

The roadmap rows R1–R5 (brief R1–R5): the entities go-hamapping `v0.3.0`
maps now reach a controller, and a controller's commands on them reach
Home Assistant.

### Added

- Sensors: carbon dioxide, carbon monoxide, PM1, PM2.5, PM10, volatile
  organic compounds, nitrogen dioxide, ozone and radon as an air quality
  sensor; apparent power, reactive power, frequency and power factor as an
  electrical sensor; volume flow rate as a flow sensor; moisture as a soil
  sensor; a `cold` binary sensor as a water freeze detector.
- `button`: an on/off plug whose On presses the button (`button.press`).
- `event`: a generic switch; every new event is a short press.
- `select`: a mode select with the entity's options; choosing a mode calls
  `select.select_option`.
- `climate` with fan modes, turn on and turn off: a room air conditioner —
  power (`climate.turn_on`, `climate.turn_off`), the thermostat, the fan
  mode (`climate.set_fan_mode`) and the current humidity.
- `vacuum` with start: a robotic vacuum cleaner — idle or cleaning
  (`vacuum.start`, `vacuum.return_to_base`), pause, resume and go home
  (`vacuum.pause`, `vacuum.start`, `vacuum.return_to_base`).
- `valve`: a water valve — open and close (`valve.open_valve`,
  `valve.close_valve`).
- `water_heater`: a heating-only thermostat — off and on
  (`water_heater.set_operation_mode`, or `water_heater.turn_on` /
  `turn_off` where the heater has them) and the target temperature
  (`water_heater.set_temperature`); the limits from `min_temp` and
  `max_temp` in the installation's unit.
- `media_player`: a TV is a video player (power, play, pause, stop, the
  volume and power keys); every other player a speaker (power or mute,
  and the volume as a level: `media_player.volume_set`, rounded to two
  places — `docs/by_design.md` "Volume as a level").

### Changed

- go-hamapping `v0.2.1` → `v0.3.0`: the rows above, each with what it
  leaves out as a caveat (`docs/roadmap.md` R1–R5).

## [0.3.7] - 2026-10-30

Dependency release: go-fabric `v0.15.4`, go-fabric-ui `v0.2.8`.

### Changed

- go-fabric `v0.15.3` → `v0.15.4`: `node.Observers.OnSurfaceChanged` announces
  an endpoint whose served surface an assembly changed; a withheld key's
  record leaves `Node.Surface().Withheld` on `Remove` and `Forget`; two test
  flakes (#306). Nothing composed changes.
- go-fabric-ui `v0.2.7` → `v0.2.8`: `surface_changed` comes from the node's
  observer, so a reassembly the host runs on its own reaches the stream too;
  the status page's warning count text is neutral; the Surface page shows the
  root endpoint.

## [0.3.6] - 2026-10-30

Dependency release: go-fabric `v0.15.3`, go-fabric-ui `v0.2.7`.

### Changed

- go-fabric `v0.15.2` → `v0.15.3`: `surface.Warn`, the verdict of a policy that
  keeps an element and flags it; a controller profile's Reported or Inherited
  row is a `Warn` decision. Nothing composed changes.
- go-fabric-ui `v0.2.6` → `v0.2.7`: the node's surface report gets its own page
  (`GET /node/surface`: effective surface per endpoint, decisions, withheld
  elements, announced changes), a Surface tab in the shell, the stream event
  `surface_changed` and a marker on the Endpoints page for an endpoint whose
  surface changed; the status page counts warnings by the `Warn` verdict.

## [0.3.5] - 2026-10-30

Dependency release: go-fabric `v0.15.2`, go-fabric-ui `v0.2.6`.

### Changed

- go-fabric `v0.15.1` → `v0.15.2`: `surface.Report.Extensions` lists the enabled ahead-of-matter.js register IDs in the node's surface report, for go-fabric-ui's status page (its 0.2.6). Nothing composed changes.
- go-fabric-ui `v0.2.5` → `v0.2.6`: the node status page shows the surface
  policies the node composes under — the specification level with its
  origin, the controller profiles, the enabled extensions — and, when
  non-zero, the counts of withheld elements, profile warnings and
  announced surface changes (`GET /node/status` gains `surface`).

## [0.3.4] - 2026-10-30

The store's fabrics in the log at start, and what a lost store means.

### Added

- `ha2fabric.fabrics` at start: the number of fabrics the store holds, and
  per fabric its index, vendor id and label; with none, a hint that no
  controller has paired or the store was lost. Until now an empty store
  showed only through the commissioning window it opens. `docs/operations.md`
  says how a lost store comes back (the add-on's backup) and what a new
  pairing means instead.

## [0.3.3] - 2026-10-30

Dependency release: go-fabric `v0.15.1`, go-fabric-ui `v0.2.5`.

### Changed

- go-fabric `v0.14.0` → `v0.15.1`. The bridge builds and its tests pass
  without a code change. go-fabric v0.15.0 serves each endpoint's surface
  (device types, clusters, features, the global lists) as a value under
  node-level policies (its ADR 0030): a specification level, opt-ins for
  the features it implements ahead of matter.js, and controller profiles,
  all in `node.Config.Surface`, which this bridge leaves at its zero value
  — nothing it composes changes, and go-hamapping's plans select no
  ahead-of-matter.js feature. What this bridge gains: when an endpoint
  comes back after a restart with a different surface (a feature or
  cluster that was not there before), the node raises its and the root's
  ConfigurationVersion in one change and bumps the changed clusters'
  data versions, so a paired controller reloads it; before, such a change
  was silent on the wire. v0.15.1: a bridged endpoint whose device has no
  name keeps its Descriptor (this bridge always names its devices); three
  race-job test flakes fixed in go-fabric.
- go-fabric-ui `v0.2.4` → `v0.2.5`: the selection page hides items whose
  verdict is unmappable — here an entity go-hamapping has no plan for, or
  one without a state — until the toggle above the list shows them, with
  their reason as before.

## [0.3.2] - 2026-10-29

Dependency release: go-fabric `v0.14.0`, go-fabric-ui `v0.2.4`.

### Changed

- go-fabric `v0.13.6` → `v0.14.0`. The bridge builds and its tests pass
  without a code change: none of the breaks in between (`core.Binding`'s
  entry type, `camera.SessionKey`, `jfadef.CommissionerReach`, the removed
  `controller/thread`) touches what it composes. For this bridge: the
  node answers TimeSynchronization's SetUTCTime with connectedhomeip's
  statuses instead of accepting every request (the clock is still not
  set). The node's session intervals stay the Matter defaults. The
  camera, Thread Border Router, Joint Fabric, TLS and
  AmbientContextSensing servers it adds are not composed by this bridge.
- go-fabric-ui `v0.2.3` → `v0.2.4` (the go-fabric `v0.14.0` build and the
  exported `NodeObservers`, which this bridge does not use).

## [0.3.1] - 2026-10-27

Dependency release: go-fabric `v0.13.6`, go-fabric-ui `v0.2.3`, go-hamqtt
`v0.39.0`.

### Changed

- go-fabric `v0.13.3` → `v0.13.6`. For this bridge: the OTA provider's
  Operate ACL entry no longer emits an AccessControlEntryChanged event
  (the node installs it as a local change, as matter.js does). The other
  additions (network-infrastructure and camera cluster servers, the
  `ble/esphome` sub-module) are not composed by this bridge.
- go-fabric-ui `v0.2.2` → `v0.2.3` (build-tooling updates; the bundle is
  byte-identical).
- go-hamqtt `v0.38.0` → `v0.39.0` (new lawn-mower discovery fields; go-mqtt
  `v1.7.0` underneath). No change for the bridge.

## [0.3.0] - 2026-10-27

go-fabric-ui `v0.2.2`: the web UI looks like Home Assistant from the first
frame.

### Changed

- The web UI follows Home Assistant's theme without a flash of the wrong
  palette: inside the Ingress panel the first frame already carries Home
  Assistant's colours and light/dark mode; a theme switch is followed live,
  including a variable a theme stops defining; the label on a primary
  button uses the theme's `--text-primary-color`; checkboxes and
  scrollbars take the theme's colours; the tab row no longer shows a
  scrollbar. Opened outside Home Assistant it uses Home Assistant's default
  light or dark palette by the system setting.
- go-fabric-ui `v0.1.3` → `v0.2.2`. It brings two indirect modules,
  github.com/oapi-codegen/runtime (Apache-2.0) and
  github.com/apapsch/go-jsonmerge/v2 (MIT), recorded in
  `THIRD-PARTY-NOTICES.md`.
- The add-on's side panel is titled **Matter Bridge** (`panel_title`),
  instead of the slug `go-ha2fabric`.
- The default bridge label (`label` option, `LABEL`) is `Matter Bridge`,
  the name controllers show for the bridge. Only the default changes: an
  add-on whose configuration was saved keeps the label stored there; one
  that never saved its configuration runs on the add-on's defaults (the
  Supervisor merges the saved options over them) and takes the new label
  after the update, as does a daemon started without `LABEL`. Either way a
  store created under the old label keeps its UniqueID (go-fabric pins it
  at the first boot), so paired controllers keep the same bridge; a
  controller may show the new name.

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
