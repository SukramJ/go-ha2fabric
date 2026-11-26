# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- The licence and notice texts of everything linked into the daemon —
  each Go module's, Go's own, go-fabric's `NOTICE` with matter.js's, the
  npm licences of go-fabric-ui's web bundle — in a generated `licenses/`
  tree, shipped in the release archives and in both images under
  `/usr/share/doc/go-ha2fabric/`; CI fails when it is stale
  (HR-P3-sup-37).
- Release archives and both images carry a build provenance and an SBOM
  (CycloneDX) attestation, checkable with `gh attestation verify`; the
  archives' SBOM is also a release asset (HR-P3-sup-36).
- `SECURITY.md`: report a vulnerability privately through GitHub's
  private vulnerability reporting (HR-P3-sup-44).

### Changed

- Dependencies moved to their vanity import paths: `sukramj.dev/go-ha-catalog` v0.4.0, `sukramj.dev/go-hamapping` v0.8.0, `sukramj.dev/go-hamqtt` v0.41.0.
- Add-on: the web UI binds to the address the Supervisor gives the add-on
  (the Supervisor network's gateway, where the Ingress proxy connects),
  read at start; when it cannot be read, every interface as before, with
  the warning `ha2fabric.web_listen`. The client filter is unchanged
  (HR-P0-cfg-27).
- Add-on: the `label` option is `str(1,32)` in the schema, so the
  Supervisor's options form refuses an empty or a longer label
  (HR-P3-cfg-68).
- Base images are pinned by digest: the add-on's Home Assistant base
  `3.24-2026.11.1` instead of `latest`; a local add-on build takes
  `BUILD_ARCH`, not `BUILD_FROM` (HR-P3-sup-34).
- Images and archives are published only for a tag on `main`; there is no
  manual image build any more (a published image is never rebuilt, a base
  update ships as a patch release), and `:latest` moves only with the
  newest release (HR-P3-sup-32).

### Removed

- **The add-on for armv7.** Home Assistant publishes no armv7 base image
  after `3.22-2025.11.1`, so the add-on is built for amd64 and aarch64
  only; an armv7 installation keeps 0.7.0. The release archives and the
  plain image still cover linux/arm/v7.

### Fixed

- A Home Assistant reply that is not a result with `success` fails its
  command instead of passing as a success (HR-P3-ha-09).
- Docs: the add-on's "Mapped today" lists what go-hamapping v0.8.0 maps
  (scenes, media players, an air conditioner's fan modes and more were
  listed as not mapped); `THIRD-PARTY-NOTICES.md` names `go.mod`'s
  modules and versions; the co-installed add-ons an operator should trust
  (`docs/operations.md`, The web UI).

## [0.7.0] - 2026-11-25

The move to go-fabric `v0.26.0` under its new module path
`thingfabric.dev/go-fabric`, with go-fabric-ui `v0.14.1` (now
`thingfabric.dev/go-fabric-ui`) and go-hamapping `v0.7.1`; the project moved
to `github.com/thingfabric/go-ha2fabric`. **Back up before updating:** the
store's upgrade is one-way.
**The images and the add-on repository moved**: pull
`ghcr.io/thingfabric/go-ha2fabric`, and in Home Assistant add the add-on
repository `https://github.com/thingfabric/go-ha2fabric` (Changed).
**Stand-alone with `WEB_ADMIN=true` on a listener that is not loopback
and no `WEB_ALLOW`, the daemon no longer starts**:
set `WEB_ALLOW` or a loopback `WEB_LISTEN` before updating (Changed).
**The container image's web UI now listens on loopback** (`127.0.0.1:8099`):
to reach it from the network, run the image with
`-e WEB_LISTEN=0.0.0.0:8099` (and `WEB_ALLOW` for `WEB_ADMIN`) (Changed).

### Added

- `WEB_ALLOWED_HOSTS` (`--web-allowed-hosts`): the DNS names a stand-alone
  web UI answers under besides `localhost` and IP addresses; any other name
  is answered 403 `host_not_allowed` (go-fabric-ui's Host check).

### Changed

- **The project moved to the thingfabric organisation.** The repository is
  `github.com/thingfabric/go-ha2fabric` (the old URLs redirect) and the
  module path `thingfabric.dev/go-ha2fabric`. Breaking for anyone who pulls
  an image by its old name: the images
  are `ghcr.io/thingfabric/go-ha2fabric` and
  `ghcr.io/thingfabric/go-ha2fabric-addon-{arch}` (no `0.7.0` is published
  under `ghcr.io/sukramj/`), and a new installation adds the add-on
  repository `https://github.com/thingfabric/go-ha2fabric`. An add-on
  installed from the old address keeps it and updates in place (the old
  address redirects); removing the old repository and installing from the
  new one installs a different add-on with empty data, so its controllers
  must be paired again.
- **The configured node's passcode is random unless set.** Without
  `PASSCODE` (the add-on's `passcode` option, now optional without a
  default) the node draws a valid passcode on its first start and keeps it
  in `passcode.json` beside the store, as the web UI's nodes do. An
  explicit `20202021`, the public test value and the old default — which
  add-on options saved before 0.7.0 keep — is honoured with the log line
  `ha2fabric.passcode_test_value` and the status warning
  `passcode_test_value`; an add-on installation that never saved its
  options gets a random passcode with the update. A paired node stays
  paired either way. A `passcode.json` that cannot be read or written does
  not stop the daemon: the node runs on a random passcode for that start,
  with the status warning `passcode_not_kept`.
- **In the add-on the web UI's administrator is a Home Assistant
  administrator**: the daemon reads the user's groups the Supervisor sets
  on each Ingress request (`X-Remote-User-Groups`, `system-admin`), which
  needs Supervisor 2026.10.4 or later; the daemon reads the Supervisor's
  version at start (`GET /info`). On an older Supervisor (a pre-release of
  2026.10.4 counts as older), or until the version is read, the web UI is
  the status page only, with the warning `supervisor_too_old` or
  `supervisor_unknown`; below 2026.10.4 the daemon reads the version again
  every 10 minutes, so an updated Supervisor needs no restart. The manifest sets
  `panel_admin: true`.
- **Breaking:** `WEB_ADMIN=true` on a `WEB_LISTEN` that is not loopback
  (`0.0.0.0:8099`) is refused at start without `WEB_ALLOW`
  (exit 2: the whole daemon, the bridge included, not only the web UI);
  the docs say the page is plain HTTP and an address filter is not
  authentication. A
  `WEB_ALLOW` entry that is not an IP address fails the web UI instead of
  being dropped. In the add-on `WEB_ALLOW` defaults to the Ingress proxy
  and takes no other value.
- **Breaking for the container image:** it no longer sets
  `WEB_LISTEN=0.0.0.0:8099`; the web UI takes the binary's default,
  `127.0.0.1:8099`, so the status page is not answered to the whole
  network. Run it with `-e WEB_LISTEN=0.0.0.0:8099` to reach the web UI
  from another machine, with `WEB_ALLOW` naming the administrators'
  addresses for `WEB_ADMIN`. The image declares `5540/tcp` (Matter over
  TCP with `CAMERA`), and a `.dockerignore` keeps the notes and the
  repository's history out of the build context.
- The pairing codes are logged only while the node has no fabric; a paired
  node's `ha2fabric.pairing` line points to the pairing page.
- `HA_URL` must parse as a `ws://` or `wss://` URL with a host (an IPv6
  address in brackets) and no user or password; the log shows it without
  a query. Neither the WebSocket handshake nor the camera proxy's request
  follows a redirect. With `ws://` to a
  host that is neither loopback nor the Supervisor the daemon logs
  `ha2fabric.token_in_clear` and shows the status warning `token_in_clear`.
- `-h` no longer prints an environment `PASSCODE` or `HA_URL` as a flag's
  default.

- go-fabric `v0.22.2` → `v0.26.0`, now `thingfabric.dev/go-fabric`
  (repository `github.com/thingfabric/go-fabric`). The store's schema is
  upgraded from version 7 to 12 at the first start, one-way: an older
  daemon is not expected to open it, so going back means restoring a
  backup taken before the update (`docs/operations.md`, Files). go-fabric
  no longer derives a SerialNumber from the unique id: the root serves none
  (owner decision: none stated), a bridged endpoint its device's (below),
  and the commissionable advertisement now names the node an
  Aggregator explicitly (`DT`), rather than the first bridged device's type
  go-fabric would infer. GeneralCommissioning's `LocationCapability` is
  IndoorOutdoor, go-fabric's new default (was Indoor). A device the bridge
  rebuilds under its key (a changed entity or device) is announced as
  new: its clusters get new data versions and subscribed controllers are
  sent its values, so a controller's cache no longer hides them
  (go-fabric #589). go-fabric's node ends the camera sessions of a
  controller it proved lost (an exchange the node opened went unanswered
  and no session with the controller is left); the daemon closes Home
  Assistant's session with them.
- go-fabric-ui `v0.9.2` → `v0.14.1`, now `thingfabric.dev/go-fabric-ui`
  (repository `github.com/thingfabric/go-fabric-ui`); a problem's `type`
  URI is `https://thingfabric.dev/go-fabric-ui/problems#problem-<code>`,
  its `code` is unchanged. Every request's `Host` is checked; the
  add-on admits every request of the Ingress proxy, whose `Host` is the
  browser's. **Stand-alone without `WEB_ADMIN` the web UI is the status
  page only**: the selection, fabrics, endpoints, diagnostics, pairing and
  the Operation page are the administrator's (owner decision). An
  administrator's request is refused when its `Origin` names a host the
  check does not admit (in the add-on: any other than the page's own). The page's
  event streams end when the daemon stops.
- go-hamapping `v0.4.12` → `v0.7.1`: a bridged endpoint's SerialNumber is
  its Home Assistant device's `serial_number` (cut to 32 bytes; none when
  the device has none) — for a bridge already paired, the attribute
  appears where none was served; a fan's percentage maps to its mode in
  three bands and off to 0 %, a climate entity's fan mode to FanControl's
  percentage, an `auto` preset to a null percentage; labels and the vendor
  are cut by UTF-8 bytes, the vendor name to 32. A cover with tilt is
  served (before, go-fabric refused it and its device's other entities
  with it, HR-P2-adpt-33): Type TiltBlindLift with the end product type its
  device class allows; a tilt-only cover's open, close and position move
  its tilt, and a tilt without `SET_TILT_POSITION` opens
  (`open_cover_tilt`) at 0 % only and closes (`close_cover_tilt`)
  otherwise, as a lift without `SET_POSITION` does. A select with 256 options and a thermostat whose `min_temp` is
  above its `max_temp` are unmappable with a reason instead of refused by
  the node (HR-P2-adpt-34, HR-P2-adpt-43); select labels that repeat are
  numbered.

### Fixed

- A CASE session is installed from the handshake step that established
  it (go-fabric `v0.26.0`): with Matter over TCP on, which `CAMERA` turns
  on, go-fabric names a second route by which one handshake's keys could
  end up under another peer's identity (inferred there, not run).
- A web UI node's key from a long label could be 57 characters, one over
  the documented bound of 56.
- A light command's brightness, colour and colour temperature, a cover's
  position or tilt and a fan's percentage clamp to their range on every
  architecture; an out-of-range value from a controller could become the
  opposite bound (or, for `color_temp_kelvin`, a negative number), notably
  on 386 and arm/v7.
- The camera proxy's base URL is derived only from an `HA_URL` that names a
  host; an unbracketed IPv6 literal such as `ws://::/websocket` is refused
  instead of giving a URL that cannot be fetched.
- A WebRTC session queues at most 64 of the controller's ICE candidates
  until Home Assistant names its session; a call beyond that is answered
  `RESOURCE_EXHAUSTED`.

- A failed reachability change while Home Assistant is away is logged
  (`bridge.unreachable`, Debug) instead of dropped.
- A Home Assistant connection that authenticates and drops at once is
  retried with a back-off that keeps doubling to a minute, instead of every
  second; the back-off starts again only after a connection that stayed up
  longer than a minute.
- A token Home Assistant refuses (`auth_invalid`) is retried after five
  minutes, doubling to hourly, instead of every minute, with the log line
  `ha.token_refused` and the status warning `ha_token_refused`, which
  clears as soon as Home Assistant accepts the token again; the daemon
  keeps running.
- A camera offer whose controller gave up before Home Assistant answered no
  longer leaves Home Assistant's WebRTC session open until the connection
  drops: the subscription is ended once the answer arrives.
- One registry or state entry with a field of an unexpected type (a name
  that is a number) no longer fails every sync: that entry is skipped and
  logged once per list (`ha.entry_skipped`). An entity whose entity or
  device registry entry is skipped is not exposed until it reads again (its
  hidden flag and labels are unknown), and a skipped label entry fails the
  sync while `INCLUDE_LABELS` or `EXCLUDE_LABELS` name labels.
- A command whose caller gave up while it was being written no longer
  closes the Home Assistant connection every node shares.
- A camera holds at most 8 WebRTC sessions: a further offer ends the
  session idle longest (a minute or more), or is answered
  `RESOURCE_EXHAUSTED`. A session has no idle end of its own, so a live
  view stays open as long as its controller keeps it; the sessions of a
  controller that went away end through go-fabric's node (above) or make
  room by the eviction.
- A controller's ICE candidates are bounded after Home Assistant's session
  event too: 64 per call, 256 per session (`RESOURCE_EXHAUSTED` beyond).
- Concurrent snapshots of one camera share one camera proxy request, and a
  still younger than a second is reused for the same size.
- A camera re-offer whose new Home Assistant session fails ends the
  session and tells the controller, instead of leaving it listed without a
  stream; an answer without an SDP ends the session as Home Assistant's
  error does.
- An overrides file (`selection.json`) that does not parse no longer turns
  every entity hidden in the web UI back on: each write keeps the file it
  replaces as `selection.json.bak`, which is read instead and written back
  (losing at most the last change), also when `selection.json` is missing;
  a file of another version is read leniently. Only when neither helps —
  or nothing in the file is understood — does the configured node start
  without overrides, and the status warning now says that entities turned
  off are exposed again. Keys that are not entity ids are ignored, names cut to 256 bytes.
- A `nodes.json` or `passcode.json` that is there with a mode wider than
  0600 (a restored backup, a copy) is set to 0600 when read, and so is a
  copy moved aside as corrupt (`nodes.mode_tightened`).
- `NODE_LIMIT` is documented as what it is, the bound on the nodes the web
  UI creates; a registry with more nodes than the limit starts them all
  and says so (`ha2fabric.node_limit`).
- The stop has one budget of 25 seconds — the web server and the Home
  Assistant connection, then the configured node and the web UI's nodes at
  once — and the add-on's manifest gives it 30 (`timeout: 30`; the
  Supervisor's default 10 could kill it inside a node's stop). An erase in
  progress may finish its reset for up to 20 of those seconds and does not
  bring the node back; the web server's shutdown error is logged
  (`ha2fabric.web_stop`). A second signal ends the process at once; a
  signal during the start exits 0 instead of 1.
- An erase whose reset fails without stopping the node no longer orphans
  it on its port: the node is stopped and comes back, or, when it cannot
  be stopped, is kept for the next erase to reset. Status pages no longer
  wait for an erase.
- `LABEL` must be UTF-8 text without control characters. With `WEB_ADMIN`
  and no `WEB_ALLOW`, the web UI refuses to serve when `WEB_LISTEN`
  (`localhost`, by name) bound an address that is not loopback. The
  add-on's Supervisor check follows no redirect and takes no proxy from
  the environment; the docs say that the Home Assistant connection does
  take one (`HTTP_PROXY`).
- A controller that moves a thermostat's heat/cool range writes two
  setpoints; the second call now carries the first one's new bound instead
  of the stale state's, and a bound that crosses the other moves it along,
  so Home Assistant no longer refuses the first call (HR-P1-adpt-30).
- A burst of Home Assistant events that overflows the bridge's queue no
  longer loses the promised re-sync, the unreachable marking of an outage or
  the forgetting of a removed entity (HR-P1-conc-32).
- An endpoint go-fabric refuses in a sync's batch is no longer counted as
  served: the status shows `sync_error`, the web UI's reassemble returns it,
  and the next sync adds it again (HR-P1-err-31).
- A controller command still queued when its entity leaves the bridge is
  dropped (`bridge.command_unserved`), not sent to Home Assistant
  (HR-P1-conc-33).
- An include or exclude label configured by its name keeps working after
  the label is renamed in Home Assistant, while the daemon runs; the warning
  of an exclude label that names no label says it excludes nothing now
  (HR-P1-cfg-34).
- A stream of Home Assistant events closer than the one-second debounce
  no longer puts the bridge's re-sync off without end: it runs at the
  latest five seconds after the first event (HR-P2-data-30). A sync that
  fails while Home Assistant stays connected is tried again after 5
  seconds, doubling to 5 minutes (HR-P2-err-61); from the second failure
  in a row the status page shows the warning `sync_failed` with the error
  (HR-P2-bound-37). The web UI's reassemble reports a sync that did not
  run, instead of a success (HR-P2-err-65). The nodes on the one Home
  Assistant connection share one read of it per event instead of one each
  (HR-P2-bound-62).
- The bridge holds only its served entities' states beside the last
  snapshot, not a second copy of every state (HR-P2-bound-38).
- A stop or an erase no longer waits past its budget for a bridge whose
  camera sessions do not end (`ha2fabric.bridge_stop_overrun`), an erase
  no longer holds the status pages while it waits, and a sync no longer
  waits for a camera's sessions to end (HR-P2-conc-60).
- A controller's command that the full queue drops, or that has no
  service, puts Home Assistant's state back on the device, as a failed
  call does (HR-P2-err-63).
- `bridge.queue_full` and `bridge.command_dropped` are one line per burst
  and then a count every 10 seconds, not one Error line per dropped item
  (HR-P2-bound-64). A sync or a command a stop cancels is no longer logged
  as failed (HR-P2-err-67).
- An entity go-fabric refuses no longer takes its Home Assistant device's
  other entities off the bridge: it (with what rides on its endpoint) is
  unmappable with go-fabric's reason, and the rest of the device is built
  without it (`bridge.build_refused`) (HR-P2-data-35).
- A camera's WebRTC sessions end together, within the stop's budget: a
  controller that went away no longer holds the stop for ten seconds per
  session (HR-P2-conc-60).
- Camera: Home Assistant's ICE candidates are bounded as the
  controller's are — 256 per session, 1 KiB each — and an event the
  relay ignores no longer counts as activity for the eviction
  (HR-P2-bound-40).
- Home Assistant's names, manufacturers, models, versions and serials
  reach the controllers without control and bidirectional control
  characters; a web UI name, a node's label and `LABEL` carrying one are
  refused; `--dump-plan` prints such a cell quoted (HR-P2-data-31, closes
  HR-P1-data-36). A registry name made only of such characters falls back
  to the entity's other names, and a node's label already in `nodes.json`
  is used cleaned and logged (`nodes.label_refused`) (HR-P2-data-94).
- A select's option, a fan's preset and a climate's fan mode go back to
  Home Assistant untrimmed (HR-P2-data-32).
- A state over 256 KiB is read without its attributes and logged
  (`ha.state_oversized`, also for a `state_changed`), and its plan line
  says so (HR-P2-bound-38).
- An entity whose state is over 256 KiB is unmappable ("state over
  256 KiB") until a state under the cap, which brings its endpoint back on
  its number; it is no longer mapped from the state without attributes as
  another device type (HR-P2-bound-90).
- A message from Home Assistant over the read limit is named in the
  sync's error, not only as a dropped connection (HR-P2-bound-37).
- Home Assistant's error text is cut to 512 bytes in errors, the log and
  the status (HR-P2-bound-39).
- The add-on's Supervisor check reads the version every 10 minutes also
  after it passed; a Supervisor downgraded below 2026.10.4 takes the web
  UI's administrator away again (HR-P2-cfg-03).
- A web UI apply racing a node's delete no longer writes the deleted
  node's `selection-<key>.json` again (HR-P2-conc-68).

## [0.6.3] - 2026-11-12

The bump to go-fabric `v0.22.2`, go-fabric-ui `v0.9.2` and go-hamapping
`v0.4.12`.

### Changed

- go-fabric `v0.22.1` → `v0.22.2` (patch, go-fabric PR 512): a fix in the
  nested module `ble/esphome` only (a C1 write an ESPHome Bluetooth proxy
  answers with a GATT error is reported as the peripheral's disconnect); the
  root module has no code change and this bridge does not use
  `ble/esphome`. Alignment only, no behaviour change.
- go-fabric-ui `v0.9.1` → `v0.9.2`, go-hamapping `v0.4.11` → `v0.4.12`: the dependency only (each on go-fabric `v0.22.2` and Go 1.27.4; the UI contract stays 0.9.0).

## [0.6.2] - 2026-11-12

The bump to go-fabric `v0.22.1`, go-fabric-ui `v0.9.1` and go-hamapping
`v0.4.11`.

### Changed

- Built with Go 1.27.4: `GO-2026-6859` (`net/http`, fixed in 1.27.4); `go.mod`, the workflows and the Dockerfiles move together.
- go-fabric `v0.22.1`; `v0.22.0` is milestone 7 (BLE and Wi-Fi commissioning on the
  device side, `node.Network.BLE` and `node.Network.WiFi.Driver`; not wired
  here, the node stays on the host's network); the mDNS responder joins
  interfaces that come up after start; the removed
  `fabrictest.UseWiFiCommissioning` is not used here. No source change.
- go-fabric `v0.22.1` (patch, issue 507): the reconnect fix in `client/peer`
  for a Thread node behind a border router whose SRV set flips to the border
  router's own host; it reaches this bridge through `node`'s binding client
  (a bound Thread target), the daemon controls no nodes itself.
- go-fabric-ui `v0.9.0` → `v0.9.1`, go-hamapping `v0.4.10` → `v0.4.11`: the dependency only (each on go-fabric `v0.22.1` and Go 1.27.4; the UI contract stays 0.9.0).

## [0.6.1] - 2026-11-10

The bump to go-fabric `v0.21.0`, go-fabric-ui `v0.9.0` and go-hamapping
`v0.4.10`; an erase no longer waits on a controller that never answers.

### Changed

- go-fabric `v0.21.0`: `v0.19.2` bounds the fabric-leaving flush of
  `Node.FactoryReset` and `Node.RemoveFabric` (one report per
  subscription, the Leave event with the attributes owed, without
  retransmissions; go-fabric issue 473); `v0.20.0` and `v0.21.0` are
  controller-side (go-fabric issues 478, 483, 488), and `node`'s
  diagnostics types are now aliases of the new `diag` package, with no
  change for this daemon.
- go-fabric-ui `v0.9.0`: the controller family's Diagnostics tab is not
  mounted here; the node family's `SessionInfo` gains optional fields the
  node side leaves absent.
- go-hamapping `v0.4.10`: the version bump alone.

### Fixed

- An erase no longer waits ~14 s for a report the controller never
  answers (go-fabric 0.19.2, issue 473).

## [0.6.0] - 2026-11-09

The mDNS interface allow-list, `discards` in the status JSON, an erase
that numbers from 2 again and a rename that reaches the controllers at
once, over go-fabric `v0.19.1`, go-fabric-ui `v0.8.0` and go-hamapping
`v0.4.8`.

### Added

- `MDNS_INTERFACES` (`--mdns-interfaces`; the add-on's `mdns_interfaces`):
  the host interfaces the mDNS responder announces and answers on, for
  every node; empty is every interface (go-fabric `node.MDNS.Interfaces`).
  For a dual-homed host behind an mDNS reflector, whose controllers would
  otherwise cache the far network's address too (#40, #41). An empty or
  doubled name is refused at start; go-fabric's diagnosis of the list — a
  name the host lacks, a named container bridge, a named interface without
  an address — is logged at start as `ha2fabric.mdns_interface`.
- `discards` in the status JSON (`counters` on `/api/v1/node/status` and
  every `/api/v1/nodes/<key>/status`) and on the status page, through
  go-fabric-ui `v0.8.0`'s host counters: the controller commands answered
  with success that called nothing (`bridge.command_discarded`), summed
  over every node since the daemon started. 0.5.7's notes promised the
  count on the status page; only the log line was there (#51).

### Changed

- An erased node numbers its bridged endpoints from the first bridged
  number (2) again, as a new installation does, instead of above the old
  high-water mark: go-fabric `v0.19.1`'s `FactoryReset` erases the
  numbering with the endpoint records (go-fabric issue 457, #50). The
  daemon test asserts it.
- Renaming a registry node in the web UI renames the running node's own
  NodeLabel at once — persisted and reported to its controllers —
  instead of at its next start (go-fabric `Node.SetNodeLabel`).
- go-fabric `v0.19.1`, go-fabric-ui `v0.8.0`, go-hamapping `v0.4.8`.

## [0.5.8] - 2026-11-08

The lock relocks after UnlockWithTimeout; Toggle on a mute plug acts, over
go-hamapping `v0.4.7`.

### Fixed

- A lock's UnlockWithTimeout relocks when the timeout is up: go-hamapping
  `v0.4.7` makes the lock row go-fabric's DoorLock server's actuator, so
  the server keeps the auto-relock timer and calls `lock.lock` through the
  row when it fires; `AutoRelockTime` works the same way. The endpoint's
  LockState is the entity's state — null until known, NotFullyLocked for
  `jammed` — and a controller's command changes it only once Home
  Assistant reports it (go-hamapping #23).
- Toggle on a speaker's mute plug calls `volume_mute` with the inverse of
  what the controller sees instead of being discarded (go-hamapping #24);
  `bridge.command_discarded` is left to the action plugs' off family.

### Changed

- go-hamapping `v0.4.7`.

## [0.5.7] - 2026-11-08

Every command of an action plug is bound, over go-hamapping `v0.4.6` (#51).

### Fixed

- Off, Toggle, OffWithEffect, OnWithRecallGlobalScene and OnWithTimedOff on
  a scene's, script's, automation's or button's plug were answered with
  success and reached nothing: no service call, no log line, and Toggle
  stored OnOff true on the endpoint (#51). go-hamapping `v0.4.6` binds the
  on family to the action (`scene.turn_on`, `script.turn_on`,
  `automation.trigger`, `button.press`) and discards the off family
  visibly: the daemon logs `bridge.command_discarded` (`entity`, `cluster`,
  `command`) at info and counts `discards` on the status page
  (`docs/operations.md`). The same release binds Toggle on an air
  conditioner and on media players to `climate.toggle` and
  `media_player.toggle` (`climate/services.yaml:145-151`,
  `media_player/services.yaml:17-24` at 2026.10.3; the service table in
  `internal/adapt`), discards Toggle on a speaker's mute plug, and makes a
  lock's UnlockWithTimeout an `unlock`.

### Changed

- go-hamapping `v0.4.6`.

## [0.5.6] - 2026-11-08

Scripts, scenes and automations as plug-in units, over go-hamapping `v0.4.5`.

### Added

- Scripts, scenes and automations as plugs (the web UI finding of
  2026-11-08): go-hamapping `v0.4.5` maps a `script`, `scene` or
  `automation` to an OnOffPlugInUnit that always reads off; a
  controller's On calls `script.turn_on`, `scene.turn_on` or
  `automation.trigger` on the entity, without data
  (`script/services.yaml:4-8`, `scene/services.yaml:3-14`,
  `automation/services.yaml:22-30` at 2026.10.3).

### Changed

- go-hamapping `v0.4.5` (was `v0.4.4`): the rows `script/plug`,
  `scene/plug` and `automation/plug` with the method `trigger`.

## [0.5.5] - 2026-11-08

Erase for the configured node and the registry nodes, over go-fabric-ui `v0.7.1` (#45).

### Added

- Erase (#45, go-fabric-ui contract 0.7.0): `POST /api/v1/node/maintenance/erase`
  and `/api/v1/nodes/<key>/maintenance/erase` (an administrator, body
  `{"confirm":"erase-node"}`), the Fabrics page's **Erase**, for the
  configured node and every registry node. go-fabric's `FactoryReset`
  removes the node's fabrics (the controllers lose it), erases its store
  and its endpoint records and stops it; the node comes back at once on
  the erased store with its commissioning window open, the same pairing
  codes and its selection, its entities under new endpoint numbers. [Corrected
  in 0.6.0: since go-fabric `v0.19.0` the erase restarts the numbering, and
  the entities come back under the first bridged numbers again.] The
  web UI follows the new node through go-fabric-ui's `NodeSwitch`. Logged
  `ha2fabric.node_erased`; a node that does not come back is logged
  `ha2fabric.node_lost`, the daemon keeps running, and the status pages
  show `node_erase_failed` until the next Erase or a restart brings it
  back.

### Changed

- go-fabric-ui `v0.7.1` (was `v0.6.1`). From 0.7.0: a Re-announce
  button on the diagnostics page, `attestation_error` and `boot_reason`
  on the status page, the sessions' parameters on the diagnostics page,
  and the node stream's `fabric_updated`, `commissioning_complete` and
  `cluster_event`. From 0.7.1: `fabricui.NodeSwitch`, which the erase
  uses; its Commission-page fix (Bluetooth LE) is the controller
  family's, which this daemon does not serve.

## [0.5.4] - 2026-11-07

Covers with OPEN and CLOSE only, over go-hamapping `v0.4.4` (#40, #42).

### Fixed

- Covers with OPEN and CLOSE only (#40, #42): a cover without
  SET_POSITION got `set_cover_position` from a controller's
  GoToLiftPercentage and one without STOP got `stop_cover` from
  StopMotion, which Home Assistant refuses with `ServiceNotSupported` —
  a template cover with open and close scenes could not be moved.
  go-hamapping `v0.4.4` gates the cover's commands on its features:
  without SET_POSITION, GoToLiftPercentage 0 calls `open_cover` and any
  other value `close_cover`; without STOP, StopMotion answers success
  and calls nothing.

### Changed

- go-hamapping `v0.4.4` (was `v0.4.3`): the cover rows' command gates
  above, no other row changes.

## [0.5.3] - 2026-11-07

Several nodes created in the web UI and its Operation page; go-fabric
`v0.18.0`, go-fabric-ui `v0.6.1`, go-hamapping `v0.4.3`.

### Added

- Several nodes, created in the web UI (#36, go-fabric-ui contract 0.6.0,
  concept U6): beside the configured node — the add-on options, the
  flags — the web UI's node selector creates, renames and deletes further
  nodes, one per ecosystem with its own device scope. Each is a go-fabric
  node of its own on the next free port above the configured one, with a
  random 12-bit discriminator unique among the nodes, a random valid
  passcode, its store `ha2fabric-<key>.db` and its selection
  `selection-<key>.json`; the registry is `nodes.json` beside the store
  (`{nodes: [{key, label, listen, discriminator, passcode, created_at}]}`,
  `internal/nodes`). Every node's bridge rides the one Home Assistant
  connection (`ha.Fanout`); a new node serves nothing until its entities
  are picked on its selection page. Registry nodes start with the daemon;
  deleting one removes its fabrics (its controllers lose it), its files
  and its record. Each node's status page shows its configuration block,
  never the passcode. `NODE_LIMIT` (`--node-limit`, the add-on's
  `node_limit`, default 8, 0 to 32) bounds how many the UI may create.

- The Operation page of the web UI (go-fabric-ui `v0.5.0`): the daemon's
  recent log, its live stream and a run-time log level (#34). The daemon's
  slog handler tees into go-fabric's `logring` ring in front of the stderr
  text handler; one `slog.LevelVar`, set from `LOG_LEVEL` at start, gates
  both, and `fabricui.Options.Operations` reads and moves it. A level set
  on the page (admin) holds until the daemon restarts.

### Changed

- go-fabric `v0.18.0` (was `v0.17.0`), no code change here. What a
  controller of this daemon sees (go-fabric changelog 0.18.0): the
  OccupancySensing server behind an OccupancySensor is `cluster/sensing`'s
  `Occupancy`, its FeatureMap still the one go-hamapping's row declares (the
  `0x202` default belongs to `cluster/measurement`'s projected server, which
  no row uses; go-hamapping changelog 0.4.3); an mDNS goodbye names only the
  records no published service holds any more, and the node follows the
  host's network interfaces, re-announcing on an interface whose addresses
  changed; a controller's SetUTCTime sets the node's UTC as an offset over
  its clock, never the host clock. golang.org/x/net moves to `v0.62.0` (was
  `v0.61.0`), required by all three modules below.
- go-fabric-ui `v0.6.1` (was `v0.5.0`): `v0.6.0` brought the node registry
  (`fabricui.NodeRegistry`, `POST /nodes`, `PUT /nodes/{key}/label`,
  `DELETE /nodes/{key}`), per-node selection and the configuration block;
  `v0.6.1` is its go-fabric `v0.18.0` bump with no route or contract change
  and no migration (go-fabric-ui changelog 0.6.1).
- go-hamapping `v0.4.3` (was `v0.4.2`): its go-fabric `v0.18.0` bump, no
  row changes.

## [0.5.2] - 2026-11-05

Dependency release: go-fabric `v0.17.0`, go-fabric-ui `v0.5.0`, go-hamapping
`v0.4.2`.

### Changed

- go-fabric `v0.17.0` (was `v0.15.6`). The removed symbols took the routes
  of go-fabric's `docs/guides/migrating-to-the-facade.md` (the facade
  review table): `node.New(cfg)` is `node.New(ctx, cfg)`; `Config.Health`
  is a `*node.Health` (nil off); `Node.PairingCodes()` is
  `Node.SetupPayload()` with `.QR` and `.Manual`; in the end-to-end tests a
  subscription's `Updated` returns an error (go-fabric changelog 0.17.0,
  `SubscribeRequest.Updated`). A cover without `SET_POSITION` (a garage
  door, a Lift-only WindowCovering) now builds and gets its endpoint,
  partially mappable, where go-fabric refused the device before (report
  H2, Q1). go-fabric answers DNS-SD with its own responder, so
  github.com/grandcat/zeroconf and github.com/cenkalti/backoff leave the
  module graph.
- go-fabric-ui `v0.5.0` (was `v0.2.9`). `*node.Node` remains the
  `NodeSource`: the methods the interface gained (`ReadCluster`,
  `ReadAttribute`, `Events`, `Endpoints`, `OnHealthSample`,
  `OnDiagnosticEvent`) are go-fabric `v0.17.0`'s own, so the web UI shows
  each endpoint's attributes and the node's event log with no change
  here. The daemon serves one node (the default key) and attaches no
  `Operations` port, so the Operation page's tab stays hidden.
- go-hamapping `v0.4.2` (was `v0.4.1`): its go-fabric `v0.17.0` bump, no
  row changes.
- The add-on's side panel is titled **Fabric Bridge** (was `Matter Server`,
  `panel_title` in `addon/config.yaml`): the add-on is the bridge, not the
  server Home Assistant's Matter integration talks to, and "Matter" stays out
  of names (README, "Why 'Matter' is not in the name").

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

[Unreleased]: https://github.com/thingfabric/go-ha2fabric/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/thingfabric/go-ha2fabric/releases/tag/v0.2.1
[0.2.0]: https://github.com/thingfabric/go-ha2fabric/releases/tag/v0.2.0
[0.1.0]: https://github.com/thingfabric/go-ha2fabric/releases/tag/v0.1.0
