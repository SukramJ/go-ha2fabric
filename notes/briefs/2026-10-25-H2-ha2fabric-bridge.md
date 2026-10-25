# Brief H2 — go-ha2fabric: the Home Assistant bridge daemon (HA WebSocket in, go-hamapping, go-fabric in process) and its add-on

Date 2026-10-25. Concept v8 (`go-fabric/docs/concept-v8-fabric-server.md`
B.1 go-ha2fabric's role and boundary, B.3, D row H2, decision 0.2 — the
daemon embeds go-fabric in process), the repository's own `CONCEPT.md`
(`/home/markus/Dokumente/GitHub/go-ha2fabric`, main: Shape, Tests,
Packaging, Open decisions), the read map `read-h2-ha2fabric.md` (copy to
`notes/reads/read-h2-ha2fabric.md` in the first commit; HA WebSocket API
facts with their page URLs, the add-on form of go-zendure2mqtt with
file:line, go-hamapping's surface at `h1/rows`, go-fabric's facade — a
map with named gaps; verify before acting), go-fabric's cookbook
`docs/guides/embed-in-a-daemon.md` (the eight steps the daemon follows)
and `docs/guides/operations.md`. Dependencies: go-fabric `v0.11.0`
(bump to `v0.12.0` when tagged), go-hamapping `v0.1.0` (the launcher says
whether the tag exists; else the merge commit of its PR #4 as a
pseudo-version until the tag — bump then), go-hamqtt (the `model` types
go-hamapping takes), go-ha-catalog; `github.com/coder/websocket` (ISC) for
the HA client (the server family's choice); `gopkg.in/yaml.v3` only if the
config needs it (the add-on passes options as JSON — read how
go-zendure2mqtt's `script/run.sh` turns `bashio::config` into the daemon's
config and copy that route). Branch `h2/bridge` in a worktree
`/home/markus/Dokumente/GitHub/go-ha2fabric/.claude/worktrees/h2`,
conventional commits, one pull request, no auto-merge; the owner merges
and tags `v0.1.0` (`internal/version` constant and `changelog.md` section
`## [0.1.0] - <date>` cut in the PR — the release pipeline refuses a tag
that does not match). English. Never add attribution lines. go-fabric's
`CLAUDE.md` token and model rules apply.

**Model:** Opus. **Hard rules:** (1) imports: go-fabric's `node`,
`device`, `contract`, `eligibility`, the `<cluster>def` packages (only
where go-hamapping's `Plan` hands them over), go-hamapping, go-hamqtt
`model`, go-ha-catalog — nothing below go-fabric's facade; an import
guard (`internal/arch`) holds it. (2) No Matter knowledge and no HA
mapping knowledge of your own: which entity becomes which device type is
go-hamapping's `Map`; the daemon translates HA's registry and states into
go-hamqtt's `model` types and applies the plan. A gap in go-hamapping or
go-fabric is reported under Questions with the symbol needed and worked
around only by a documented stub. (3) **Home Assistant facts come from HA
core at a pinned release**, read from GitHub raw files (the read map's
pages lack the registry and state payloads): pin `home-assistant/core`
tag `2026.10.3` (the catalog's snapshot) and read
`homeassistant/components/config/entity_registry.py`,
`homeassistant/components/config/device_registry.py`,
`homeassistant/components/websocket_api/commands.py`,
`homeassistant/components/homeassistant/exposed_entities.py`,
`homeassistant/core.py` (`State.as_dict`, `EVENT_STATE_CHANGED`),
`homeassistant/helpers/entity_registry.py` (`RegistryEntry` fields,
`as_partial_dict`/`extended_dict`), citing `file:line` at that tag in
code comments and the report; never run anything from those files.
(4) The daemon acts as an administrator through the Supervisor proxy;
every credential (the long-lived token outside the add-on,
`SUPERVISOR_TOKEN` inside) is read from the environment or the add-on
options, never logged, never persisted by the daemon.

## 1 The daemon

- **1.1 HA client** (`internal/ha`): WebSocket to `ws://supervisor/core/websocket`
  (add-on; token `SUPERVISOR_TOKEN`) or `HA_URL`/`HA_TOKEN` (stand-alone);
  the auth handshake (`auth_required` → `auth` → `auth_ok`/`auth_invalid`,
  map §1.2), the `id`-tagged envelope (§1.3), `ping` keepalive (§1.9),
  reconnect with back-off and a full re-sync after reconnect. Reads:
  `config/entity_registry/list`, `config/device_registry/list`,
  `config/area_registry/list` (payload fields from HA core at the pin),
  `get_states`, `homeassistant/expose_entity/list` (§1.11);
  `subscribe_events` for `state_changed` (§1.4) and the registry-updated
  events (`entity_registry_updated`, `device_registry_updated` — confirm the
  event names in HA core) so a renamed, disabled or newly exposed entity
  moves the topology at run time. Writes: `call_service` (§1.7) with
  `target.entity_id`.
- **1.2 Exposure** (concept's open decision, decided here): an entity is
  exposed when it is exposed to Assist (`conversation`) in
  `homeassistant/expose_entity/list` **or** named in the add-on option
  `include` (entity ids or globs), and not in `exclude`; disabled and
  hidden registry entries never. The daemon never writes exposure
  settings. A `--dump-plan` flag (and the same in the web UI, §1.6) prints
  every candidate entity with its go-hamapping verdict and reason, so an
  operator sees why something is not in Matter.
- **1.3 Adapter HA → go-hamqtt** (`internal/adapt`): a registry entry plus
  its state becomes `model.Device` (from the device registry: name,
  manufacturer, model, sw_version, identifiers; entities without a device
  get a device of their own) and `model.Basic` (`EntityKey` = entity_id,
  `EntityPlatform` = domain, `Description`: device_class from
  `device_class`/`original_device_class`, unit from `unit_of_measurement`,
  state_class, options, min/max/step from attributes, name; `Extra`:
  `supported_features` (the integer bit mask — go-hamapping decodes it
  with the catalog), `supported_color_modes`, `color_temp_kelvin` bounds,
  `hvac_modes`, `fan_modes`, `preset_modes`, `swing_modes` under the
  catalog's key names as go-hamapping's rows read them (map §4.4);
  bindings: `state` and `command` roles with a Slot the daemon
  understands (entity_id as address). States become `model.State`
  (`Value` from `state`, `Extra` from `attributes`, `Available` =
  state not `unavailable`/`unknown` — read how go-hamapping's rows read
  `state.<key>` and `extra.<key>`, map §4.2).
- **1.4 Composition** (`internal/bridge`): `node.New` from the daemon's
  config (cookbook step 2: listen, passcode, discriminator, vendor/product
  — the test vendor as in CE, store path under `/data` in the add-on,
  logger, metrics, limits as go-fabric defaults), `hamapping.Map` per HA
  device, `Plan.Build`, `Aggregator.Batch` for the whole exposed set at
  start and on every registry change (cookbook step 4), `Plan.Apply` on
  `state_changed`, `Plan.OnChange` → `call_service` (the `model.Command`'s
  method and payload map to a service and its data per domain: read
  go-hamapping's command rows for the method names and the catalog's
  command keys; the service table per domain is the daemon's one table,
  cited from HA's action pages / core at the pin — e.g. `light.turn_on`
  with `brightness`, `color_temp_kelvin`, `hs_color`; `cover.set_cover_position`
  `position`; `climate.set_temperature`, `set_hvac_mode`; `lock.lock/unlock`;
  `fan.set_percentage`, `turn_on/off`; `switch.turn_on/off`), reachability
  from availability and from the HA connection (`UpstreamUsable` → every
  endpoint unreachable while HA is away). Pairing codes to the log, the web
  UI and the add-on's DOCS (cookbook step 7).
- **1.5 Config**: environment and flags (`HA_URL`, `HA_TOKEN`, `LISTEN`,
  `PASSCODE`, `DISCRIMINATOR`, `VENDOR_ID`, `PRODUCT_ID`, `LABEL`,
  `STORE_PATH`, `INCLUDE`, `EXCLUDE`, `EXPOSE_ASSIST` bool default true,
  `LOG_LEVEL`, `WEB_LISTEN`), validated like go-zendure2mqtt's `internal/config`
  (read it for the pattern); the add-on's `run.sh` maps options to them.
- **1.6 Web UI** (`internal/web`, Ingress): one read-only page — pairing
  QR (rendered client-side from the payload text, no image dependency),
  manual code, commissioned fabrics, the exposed entities with their
  verdicts and reasons, HA connection state, node health. `GET /api/health`,
  `GET /api/plan` (JSON).
- **1.7 Add-on**: `addon/` in the form of go-zendure2mqtt (`config.yaml`
  with `homeassistant_api: true`, `ingress: true`, options/schema from
  1.5, `image: ghcr.io/sukramj/go-ha2fabric-addon-{arch}`, `init: false`),
  `addon/Dockerfile` (two stages as the sibling's; `BUILD_FROM`), `script/run.sh`
  (`#!/usr/bin/with-contenv bashio`, options → env, `SUPERVISOR_TOKEN` is
  already in the environment), `repository.yaml`, `addon/DOCS.md`,
  `.github/workflows/addon-image.yml` (the sibling's, renamed; it runs on
  a tag — the version in `addon/config.yaml` equals `internal/version`),
  the distroless `Dockerfile` and `docker-build-push.yml` for the plain
  image. Host networking: the add-on needs the host's network for mDNS and
  UDP 5540 — `host_network: true` in `config.yaml` (read the Supervisor's
  add-on configuration docs for the key and its consequence for Ingress;
  cite).

## 2 Tests

- `internal/ha`: a fake HA WebSocket server in the test (auth handshake,
  registry lists, `get_states`, `state_changed` events, `call_service`
  recording) built from recorded fixtures: JSON files under `testdata/ha/`
  with the shapes taken from HA core at the pin (cite the file:line each
  fixture mirrors).
- `internal/adapt`: table tests entity → `model.Basic`/`model.State` for
  every domain go-hamapping maps (light with each color mode set, switch,
  the sensor and binary_sensor device classes, cover, climate, lock, fan)
  and the unavailable/unknown states.
- End-to-end in process with go-fabric's `fabrictest`: the fake HA, the
  daemon's bridge on a site (read how the server's S2 did it — a
  daemon-composed `node.New` cannot join a site before go-fabric's CJ; use
  `fabrictest.Device.Node()` as S2 does and say so), a site controller
  commissions it, a light and a sensor from the fake registry appear as
  endpoints, a `state_changed` reaches the controller's subscription, a
  controller `On` becomes `light.turn_on` on the fake HA, a registry
  disable removes the endpoint, HA going away flips reachability.
- Guards: the sibling standard with coverage and fuzz-smoke switched on
  (a fuzz target on the HA message decoder), `internal/arch`, `docker
  build` of both Dockerfiles, the add-on image build locally for amd64
  (`docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest -f addon/Dockerfile .`),
  the release dry run.

## 3 Docs and report

`README.md` (status to first release; Features; Quickstart as add-on and
stand-alone; the exposure rule; the trademark paragraph stays),
`addon/DOCS.md`, `docs/operations.md` (files under `/data`, ports, mDNS on
host network, the token's role, logs, health, pairing), `CONCEPT.md`
status and the decided exposure rule, `changelog.md` `## [0.1.0]`,
`internal/version`. Report `docs/reports/H2.md` (and PR body): **HA client**
(commands used with HA core citations), **Adapter** (the field table),
**Composition**, **Add-on**, **Tests**, **Guards** with HEAD,
**Differences**, **Decisions**, **Left out** (areas, scenes, media_player,
climate fan/preset modes, …), **Questions** (go-hamapping, go-fabric,
go-hamqtt gaps with symbols), `by_design` candidates (where the daemon
departs from home-assistant-matter-bridge's behaviour as far as you know
it — say what you compared against). A real pairing with Apple Home is
the owner's manual step: list it, do not claim it. Copy this brief to
`notes/briefs/2026-10-25-H2-ha2fabric-bridge.md` in the first commit.
