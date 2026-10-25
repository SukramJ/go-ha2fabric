# read-h2-ha2fabric: inventory (facts with sources; no judgments)

Reading date 2026-10-25. Note on web facts: pages were fetched through a summarising fetcher (quotes capped near 125 characters per quote; some JSON is therefore split by key or condensed by the fetcher, not verbatim whole). The old developers.home-assistant.io/docs/... URLs answer 301 to contributors.home-assistant.io/... (redirect targets given per page). Gaps are listed explicitly as "NOT on page".

Repository shas: go-fabric main 019277ed; go-zendure2mqtt HEAD 1540851; go-hamapping main 69aa9ca, branch h1/rows a89aa5482e322c120b333ef6e7049b50278b78e0; go-hamqtt HEAD d1bb9ac.

## 1. Home Assistant WebSocket API

Source page: https://developers.home-assistant.io/docs/api/websocket -> 301 -> https://contributors.home-assistant.io/core/internals/websocket-api/ (abbrev. WS).

### 1.1 Headings on WS (verbatim)
How a connection works; Authentication; Enable features; Results and errors; Core commands; Subscribe to events; Unsubscribe; Subscribe to entities; Subscribe to a trigger; Call an action; Fire an event; Get things; Ping; Validate configuration; Work with targets; More commands; Integration commands; List entities for display; Expose entities to voice assistants; Expose or unexpose entities; List exposed entities; Expose new entities by default; Add your own command; Subscriptions.

### 1.2 Auth handshake (WS, "Authentication")
Server first sends `auth_required` with `ha_version`; client replies `type: "auth"` with `access_token`; valid -> `auth_ok` (with `ha_version`); invalid -> `auth_invalid` with a `message`, then the server disconnects.
```json
{"type": "auth_required", "ha_version": "2026.10.0"}
{"type": "auth", "access_token": "<token>"}
{"type": "auth_ok", "ha_version": "2026.10.0"}
```
(fetcher fragments of the page's example: `"access_token": "ABCDEFGH"`.) A heading "Enable features" exists; its content was not returned by the fetcher.

### 1.3 Envelope (WS, "Results and errors")
Success: `type: "result"`, matching `id`, `success: true`, `result`. `ping` gets `pong` instead.
```json
{"id": 6, "type": "result", "success": true, "result": null}
```
Failure: `success: false` and an `error` object with `code` and `message`. Codes listed: `invalid_format`, `not_found`, `not_allowed`, `unauthorized`, `unknown_command`, `timeout`, `unknown_error`. `HomeAssistantError` with translation data also carries `translation_domain`, `translation_key`, `translation_placeholders`.
```json
{"id": 12, "type": "result", "success": false,
 "error": {"code": "invalid_format", "message": "..."}}
```
(page fragment: `"message": "Message incorrectly formatted: required key not provided"`.)

### 1.4 subscribe_events / unsubscribe_events (WS)
`subscribe_events` takes optional `event_type` (omitted = all events); most event types require an admin. Events arrive as `type: "event"` tagged with the subscription `id`; event object holds `event_type`, `data` (for `state_changed`: `entity_id`, `old_state`, `new_state`), `origin`, `time_fired`, `context` (`id`, `parent_id`, `user_id`).
```json
{"id": 18, "type": "subscribe_events", "event_type": "state_changed"}
```
Unsubscribe fragment: `"type": "unsubscribe_events",` `"subscription": 18`.
NOT on WS: the fields inside an `old_state`/`new_state` object (the page's example shows them as empty objects, "shortened"). Same gap on https://contributors.home-assistant.io/core/internals/events/ (only says `EVENT_STATE_CHANGED` is `EventType[EventStateChangedData]`, points to home-assistant.io/docs/configuration/events/ and `homeassistant/core.py`).
State object shape, from https://contributors.home-assistant.io/core/internals/rest-api/ ("States", state object example, quoted per key): `"entity_id": "sun.sun",` `"state": "below_horizon",` `"attributes": {` with `"friendly_name": "Sun",` `"elevation": -17.67`; `"last_changed": "2026-10-10T21:43:29.204838+00:00",` `"last_updated": "2026-10-10T21:50:30.529465+00:00",` `"context": {` `"id": "01J9Z7K1M3S8TQ0Y6E4H2B5N7C",` `"parent_id": null,` `"user_id": null`; the example also has a `last_reported` key.

### 1.5 subscribe_entities (WS, "Subscribe to entities")
```json
{"id": 20, "type": "subscribe_entities", "entity_ids": ["light.kitchen"]}
```
First event holds all readable states; later events only changes. Event map keys: `a` (added, full compressed state), `c` (changed: `+` changed values, `-` removed attributes), `r` (removed entity ids). Compressed state keys: `s` state, `a` attributes, `c` context, `lc` last changed, `lu` last updated.

### 1.6 get_states / get_config / get_services (WS, "Get things")
Descriptions only, no JSON on the page: "`get_states`: all states the user may read." "`get_config`: the core configuration" "`get_services`: all actions, with their descriptions." Shapes of their results: NOT on WS.

### 1.7 call_service (WS, "Call an action")
`domain`, `service` required; `service_data` and `target` optional; result includes the call's `context`; `"return_response": true` adds a `response` field.
```json
{"id": 24, "type": "call_service", "domain": "light", "service": "turn_on",
 "service_data": {"brightness": 255}, "target": {"entity_id": "light.kitchen"}}
```

### 1.8 subscribe_trigger (WS, "Subscribe to a trigger")
Admin only; `trigger` object in the automation trigger format; each firing yields an `event` message with trigger variables.
```json
{"id": 2, "type": "subscribe_trigger",
 "trigger": {"trigger": "state", "entity_id": "binary_sensor.motion", "to": "on"}}
```
"More commands" section names: `render_template`, `subscribe_condition`, `test_condition`, `execute_script` (list truncated by the fetcher).

### 1.9 Ping (WS, "Ping")
`{"id": N, "type": "ping"}` answered by `{"id": N, "type": "pong"}`; page example uses id 19.

### 1.10 Registry commands
- `config/entity_registry/list_for_display` (WS, "List entities for display"): no parameters, any authenticated user; result `entity_categories` (number -> category name) and `entities` (non-disabled registry entries). Short keys: `ei` entity id; `pl` platform; `lb` label ids; `ai` area id set on the entity; `di` device id; `np` name priority (`area` or `device`); `ic` icon; `tk` translation key; `ec` entity category (key of `entity_categories`); `hb` hidden; `hn` has_entity_name; `en` entity name; `dp` sensor decimal places. Keys with no value omitted. Page says: for full registry entries use `config/entity_registry/list` or `config/entity_registry/get_entries`.
- `config/entity_registry/list`, `config/device_registry/list`, `config/area_registry/list`: names only mentioned (entity list/get_entries above); their payload fields are NOT documented on WS. The page does not document `config/device_registry/list` or `config/area_registry/list`.
- Entity registry page https://developers.home-assistant.io/docs/entity_registry_index -> 301 -> https://contributors.home-assistant.io/core/entities/registry/ : headings "The entity registry", "Unique ID", "Linking to a device", "Child devices", "Entity categories", "Enabled and visible by default", "Disabled entities", "Entity options". `disabled_by` values (table): `user`, `integration`, `config_entry`, `device`, `hass`. NOT on page: `platform`, `device_id`, `original_device_class`, `device_class`, `supported_features`, `capabilities`, `unit_of_measurement`, `hidden_by` values; `options` only as per-entity settings grouped by domain read from `self.registry_entry.options`.
- Device registry page https://developers.home-assistant.io/docs/device_registry_index -> 301 -> https://contributors.home-assistant.io/core/integrations/device-registry/ , "Device properties" fields: `id`, `identifiers`, `connections`, `config_entry_id`, `config_subentry_id`, `name`, `name_by_user`, `manufacturer`, `model`, `model_id`, `serial_number`, `sw_version`, `hw_version`, `configuration_url`, `entry_type`, `via_device_id`, `parent_device_id` (WebSocket API reports `null` for other devices), `area_id`, `labels`, `disabled_by`, `created_at`, `modified_at`. No area-registry section on that page; `suggested_area` is a `DeviceInfo` argument.

### 1.11 Exposed entities (WS)
- "Expose or unexpose entities": `homeassistant/expose_entity`, admin only; required `assistants` (list), `entity_ids` (list, not checked against existing entities), `should_expose` (bool); effective immediately; result `null`. Valid assistant ids: `conversation` (Assist), `cloud.alexa`, `cloud.google_assistant`.
```json
{"id": 41, "type": "homeassistant/expose_entity",
 "assistants": ["conversation", "cloud.alexa"],
 "entity_ids": ["light.living_room", "sensor.outside"], "should_expose": true}
```
- "List exposed entities": `homeassistant/expose_entity/list`, no parameters; result `exposed_entities` maps each entity to the assistants it is exposed to, value always `true`; entities exposed to no assistant omitted; lists only stored settings (stored when someone changes it or the first time an assistant checks the entity).
- "Expose new entities by default": `homeassistant/expose_new_entities/get` (`assistant` -> `expose_new`; default `true` for `conversation`, `false` for others) and `homeassistant/expose_new_entities/set` (`assistant`, boolean `expose_new`; result `null`).
```json
{"id": 44, "type": "homeassistant/expose_new_entities/set",
 "assistant": "cloud.alexa", "expose_new": true}
```
  New entity auto-exposed only if no entity category, not hidden, and: domain in `climate`, `cover`, `fan`, `humidifier`, `light`, `media_player`, `scene`, `switch`, `todo`, `vacuum`, `water_heater`; or binary sensor with device class such as `door`, `lock`, `motion`, `window`; or sensor with device class such as `temperature`, `humidity`, `carbon_dioxide`.
- `options` incl. a `conversation` exposure key inside registry entries: NOT documented on the fetched pages.

### 1.12 Long-lived access token
https://developers.home-assistant.io/docs/auth_api -> 301 -> https://contributors.home-assistant.io/core/internals/auth-api/ :
- "Creating a Long-Lived Access Token (User Profile)": created "on their profile, under Security"; "valid for 10 years, and are used exactly like an access token."
- "Creating One via WebSocket": command `auth/long_lived_access_token` (not `auth/create_long_lived_access_token`); `lifespan` in days.
- "Token Handling": "The result is the token. Home Assistant does not store it, so it cannot be shown again."

### 1.13 Add-on route to Core (Supervisor)
https://developers.home-assistant.io/docs/add-ons/communication -> 301 -> https://contributors.home-assistant.io/apps/guides/communication/ :
- "Home Assistant Core API": "The easiest way to call Home Assistant is through the proxy of the Supervisor."; requires `homeassistant_api: true` in `config.yaml`; proxy path `http://supervisor/core/api/` maps to `/api/` on Core.
- "WebSocket API": "The WebSocket API is at ws://supervisor/core/websocket." Authenticate by sending `SUPERVISOR_TOKEN` as `access_token` in the auth message, "like you would a normal access token." "The proxy does not forward commands of the supervisor/ and hassio/ types."
- "Your token": Supervisor provides "its own token, in the environment variable SUPERVISOR_TOKEN"; REST: `Authorization: Bearer` header; S6 scripts need `#!/command/with-contenv bashio` (or `with-contenv bash`) to read it.
- "Security warning": "Through the proxy, your app acts as an administrator of Home Assistant."; "treat your SUPERVISOR_TOKEN as an administrator credential".

## 2. Entity attributes and services per domain

Entity pages: https://developers.home-assistant.io/docs/core/entity/<name> -> 301 -> https://contributors.home-assistant.io/core/entities/<name>/. These pages describe entity-class properties (Python); the page does not state the `state.attributes` keys as such. Action data keys come from https://www.home-assistant.io/actions/<action>/ pages (only those fetched).

### light (https://contributors.home-assistant.io/core/entities/light/ ; headings: Properties, Color modes, Color mode while an effect runs, Supported features, Methods, Turn on, Turn off, Toggle, Scaling brightness, States, Example)
- Properties: `supported_color_modes` (set of `ColorMode`, required); `color_mode` (required while on); `brightness` 1-255; `color_temp_kelvin` (only in `COLOR_TEMP`); `min_color_temp_kelvin` (warmest, default 2000); `max_color_temp_kelvin` (coolest, default 6535); `hs_color` hue 0-360, saturation 0-100 (`HS`); `xy_color` CIE xy (`XY`); `rgb_color` 0-255 each (`RGB`); `effect_list` (needs `EFFECT`); `effect` ("off" / `EFFECT_OFF` when none).
- Color modes: ONOFF (only mode), BRIGHTNESS (only mode), COLOR_TEMP, HS, XY, RGB, RGBW, RGBWW, WHITE, UNKNOWN (valid only as `color_mode`).
- Supported features: EFFECT, FLASH (`flash` = `short`|`long`), TRANSITION (`transition` seconds on turn_on/turn_off). Bit values not on page.
- Turn-on params (page): `flash`, `transition`, brightness, color temperature, `hs_color`, `rgb_color`, `white`, `effect`.
- `light.turn_on` data keys (https://www.home-assistant.io/actions/light.turn_on/), all optional: `transition` (integer), `brightness` (integer, 1-255, 0 turns off), `brightness_pct` (integer), `color_name` (string), `color_temp_kelvin` (integer), `rgb_color` (list of 3 ints 0-255), `hs_color` (list: hue 0-360, sat 0-100), `xy_color` (list of 2 decimals 0-1), `effect` (string), `flash` (`short`|`long`), `profile` (string); requires a `target`. Actions on https://www.home-assistant.io/integrations/light/ : `light.turn_on`, `light.turn_off`, `light.toggle`.

### switch (https://contributors.home-assistant.io/core/entities/switch/)
Properties `is_on` (bool|None), `device_class` (`SwitchDeviceClass`: `OUTLET`, `SWITCH`). Methods `turn_on`, `turn_off`, `toggle` (and async). Actions (https://www.home-assistant.io/integrations/switch/): `switch.toggle`, `switch.turn_off`, `switch.turn_on`; no data keys listed.

### cover (https://contributors.home-assistant.io/core/entities/cover/)
- Properties: `is_closed`, `is_opening`, `is_closing`, `current_cover_position` (0 closed to 100 open), `current_cover_tilt_position` (0-100), `device_class`, `supported_speeds`.
- `CoverDeviceClass`: AWNING, BLIND, CURTAIN, DAMPER, DOOR, GARAGE, GATE, SHADE, SHUTTER, WINDOW.
- `CoverEntityFeature` names: OPEN, CLOSE, SET_POSITION, STOP, OPEN_TILT, CLOSE_TILT, STOP_TILT, SET_TILT_POSITION, SPEED. Numeric bit values NOT on page.
- Data keys (entity methods): `ATTR_POSITION` (0-100), `ATTR_TILT_POSITION` (0-100), `ATTR_SPEED`. Literal strings not given there.
- Actions (https://www.home-assistant.io/integrations/cover/): `cover.close_cover`, `cover.close_cover_tilt`, `cover.open_cover`, `cover.open_cover_tilt`, `cover.set_cover_position`, `cover.set_cover_tilt_position`, `cover.stop_cover`, `cover.stop_cover_tilt`, `cover.toggle`, `cover.toggle_cover_tilt`. `cover.set_cover_position` data (https://www.home-assistant.io/actions/cover.set_cover_position/): `position` (integer, required, 0 closed to 100 open), `speed` (string, optional, a `supported_speeds` value).

### climate (https://contributors.home-assistant.io/core/entities/climate/)
- Properties: `hvac_mode`, `hvac_modes`, `hvac_action`, `native_temperature_unit`, `native_current_temperature`, `native_target_temperature`, `native_target_temperature_high`, `native_target_temperature_low`, `target_temperature_step`, `min_temp`, `max_temp`, `precision`, `current_humidity`, `target_humidity`, `target_humidity_step`, `min_humidity`, `max_humidity`, `preset_mode(s)`, `fan_mode(s)`, `swing_mode(s)`, `swing_horizontal_mode(s)`. Page states `target_temperature`, `target_temp_high`, `target_temp_low`, `current_temperature` are deprecated names that stop working in Home Assistant 2027.11.
- `ClimateEntityFeature`: TARGET_TEMPERATURE, TARGET_TEMPERATURE_RANGE, TARGET_HUMIDITY, FAN_MODE, PRESET_MODE, SWING_MODE, SWING_HORIZONTAL_MODE, TURN_ON, TURN_OFF (bit values not on page).
- `HVACMode` state values: off, heat, cool, heat_cool, auto, dry, fan_only; `hvac_mode` None -> state `unknown`. HVACAction: OFF, IDLE, HEATING, PREHEATING, COOLING, DRYING, FAN, DEFROSTING. Presets: PRESET_NONE, ECO, AWAY, BOOST, COMFORT, HOME, SLEEP, ACTIVITY. Fan modes: FAN_ON, OFF, AUTO, LOW, MEDIUM, HIGH, TOP, MIDDLE, FOCUS, DIFFUSE. Swing: SWING_ON, OFF, BOTH, VERTICAL, HORIZONTAL; SWING_HORIZONTAL_ON, SWING_HORIZONTAL_OFF.
- Actions (https://www.home-assistant.io/integrations/climate/): `climate.set_temperature`, `climate.set_hvac_mode`, `climate.set_fan_mode`, `climate.set_preset_mode`, `climate.set_swing_mode`, `climate.turn_on`, `climate.turn_off`. `climate.set_temperature` data (https://www.home-assistant.io/actions/climate.set_temperature/): `temperature` (float), `target_temp_high` (float), `target_temp_low` (float), `hvac_mode` (string), all optional; temperature not to be combined with high/low. Entity methods: `set_hvac_mode` takes `hvac_mode`, `set_fan_mode` `fan_mode`, `set_preset_mode` `preset_mode`.

### lock (https://contributors.home-assistant.io/core/entities/lock/)
Properties `is_locked`, `is_locking`, `is_unlocking`, `is_open`, `is_opening`, `is_jammed`, `code_format`, `changed_by`. `LockState`: locked, unlocked, jammed, locking, unlocking, open, opening; `unknown` when none apply. `LockEntityFeature`: OPEN. Actions (https://www.home-assistant.io/integrations/lock/): `lock.lock`, `lock.open`, `lock.unlock`. `lock.lock` data (https://www.home-assistant.io/actions/lock.lock/): `code` (string, optional).

### fan (https://contributors.home-assistant.io/core/entities/fan/)
Properties: `percentage` (int|None, 0-100), `preset_modes` (list[str]|None), `preset_mode`, `oscillating` (bool|None), `current_direction` (`DIRECTION_FORWARD` "forward" / `DIRECTION_REVERSE` "reverse"), `speed_count` (default 100), `is_on`. `FanEntityFeature`: SET_SPEED, PRESET_MODE, OSCILLATE, DIRECTION, TURN_ON, TURN_OFF (bit values not on page). Methods: turn_on (params `percentage`, `preset_mode`), turn_off, set_percentage, set_preset_mode, oscillate, set_direction. Actions (https://www.home-assistant.io/integrations/fan/): `fan.decrease_speed`, `fan.increase_speed`, `fan.oscillate`, `fan.set_direction`, `fan.set_percentage`, `fan.set_preset_mode`, `fan.toggle`, `fan.turn_off`, `fan.turn_on`. `fan.set_percentage` data: `percentage` (integer, required, 0-100).

### sensor (https://contributors.home-assistant.io/core/entities/sensor/)
`device_class` (`SensorDeviceClass`|None), `state_class` (`SensorStateClass`|None), `native_unit_of_measurement`; `unit_of_measurement` is not a property in the table. State classes: MEASUREMENT, MEASUREMENT_ANGLE, TOTAL, TOTAL_INCREASING. Device classes, non-numeric: DATE, ENUM, TIMESTAMP, UPTIME; numeric: ABSOLUTE_HUMIDITY, APPARENT_POWER, AQI, AREA, ATMOSPHERIC_PRESSURE, BATTERY, BLOOD_GLUCOSE_CONCENTRATION, CO, CO2, CONDUCTIVITY, CURRENT, DATA_RATE, DATA_SIZE, DISTANCE, DURATION, ENERGY, ENERGY_DISTANCE, ENERGY_STORAGE, FREQUENCY, GAS, HUMIDITY, ILLUMINANCE, IRRADIANCE, MOISTURE, MONETARY, NITROGEN_DIOXIDE, NITROGEN_MONOXIDE, NITROUS_OXIDE, OZONE, PH, PM1, PM25, PM4, PM10, POWER, POWER_FACTOR, PRECIPITATION, PRECIPITATION_INTENSITY, PRESSURE, RADON, REACTIVE_ENERGY, REACTIVE_POWER, SIGNAL_STRENGTH, SOUND_PRESSURE, SPEED, SULPHUR_DIOXIDE, TEMPERATURE, TEMPERATURE_DELTA, VOLATILE_ORGANIC_COMPOUNDS, VOLATILE_ORGANIC_COMPOUNDS_PARTS, VOLTAGE, VOLUME, VOLUME_FLOW_RATE, VOLUME_STORAGE, WATER, WEIGHT, WIND_DIRECTION, WIND_SPEED.

### binary_sensor (https://contributors.home-assistant.io/core/entities/binary-sensor/)
`is_on` (bool|None, required), `device_class` (`BinarySensorDeviceClass`|None). Values: BATTERY, BATTERY_CHARGING, CO, COLD, CONNECTIVITY, DOOR, GARAGE_DOOR, GAS, GLASS_BREAK, HEAT, LIGHT, LOCK, MOISTURE, MOTION, MOVING, OCCUPANCY, OPENING, PLUG, POWER, PRESENCE, PROBLEM, RUNNING, SAFETY, SMOKE, SOUND, TAMPER, UPDATE, VIBRATION, WINDOW.

## 3. Add-on form: /home/markus/Dokumente/GitHub/go-zendure2mqtt (HEAD 1540851)

### 3.1 Listing
`addon/`: CHANGELOG.md, DOCS.md, Dockerfile, README.md, build.yaml, config.yaml, icon.png. Repo root: `repository.yaml`; entrypoint `script/run.sh` (copied in Dockerfile); workflow `.github/workflows/addon-image.yml`.

### 3.2 addon/config.yaml (verbatim)
```yaml
# Home Assistant add-on manifest for go-zendure2mqtt.
#
# version MUST equal the published image tag (ghcr.io/sukramj/go-zendure2mqtt-addon-{arch})
# and is bumped in the release commit that cuts the matching git tag.
name: go-zendure2mqtt
version: "0.11.0"
slug: go-zendure2mqtt
description: >-
  Bridges Zendure devices (e.g. SolarFlow 2400 AC) to MQTT with Home Assistant
  MQTT discovery and a read-only diagnostic web UI. Local (zenSDK HTTP) or
  Zendure cloud transport.
url: https://github.com/SukramJ/go-zendure2mqtt
arch:
  - amd64
  - aarch64
  - armv7

# Per-arch add-on image (HA base + bashio + run.sh) published by
# .github/workflows/addon-image.yml. The Supervisor substitutes {arch} and
# pulls <image>:<version>. This is distinct from the distroless daemon image
# (ghcr.io/sukramj/go-zendure2mqtt), which runs the binary directly and is NOT
# a valid add-on image (no run.sh to apply options). Remove this key to build
# addon/Dockerfile locally instead.
image: "ghcr.io/sukramj/go-zendure2mqtt-addon-{arch}"

# The HA base image ships s6-overlay as its init; don't let the Supervisor
# inject its own init on top.
init: false

# Read-only diagnostic web UI surfaced as an HA panel via Ingress — no port
# needs to be exposed.
ingress: true
ingress_port: 8080
panel_icon: mdi:solar-power
panel_title: go-zendure2mqtt

# Optional host-port exposure (disabled by default — `null` publishes no port,
# Ingress stays the primary path).
ports:
  8080/tcp: null
ports_description:
  8080/tcp: "Diagnostic web UI (normally reached via Ingress)"

# Soft dependency: use the Supervisor's MQTT broker (core-mosquitto) when present.
services:
  - mqtt:want

options:
  connection: local
  refresh: 15
  local_devices: []
  cloud_app_token: ""
  cloud_tls_verify: false
  mqtt_server: ""
  mqtt_port: 1883
  mqtt_login: ""
  mqtt_password: ""
  mqtt_topic: ""
  mqtt_maintenance: true
  mqtt_stats_interval: 60
  hass_enable: true
  language: en
  web_enable: true
  charge_active_value: 1200
  discharge_active_value: 1200
  debug: false

schema:
  connection: "list(local|cloud)"
  refresh: int(5,86400)
  local_devices:
    - sn: str
      host: str
      device_name: "str?"
      model: "str?"
  cloud_app_token: "password?"
  cloud_tls_verify: bool
  mqtt_server: "str?"
  mqtt_port: int(1,65535)
  mqtt_login: "str?"
  mqtt_password: "password?"
  mqtt_topic: "str?"
  mqtt_maintenance: bool
  mqtt_stats_interval: int(0,86400)
  hass_enable: bool
  language: "list(en|de)"
  web_enable: bool
  charge_active_value: int(1,2400)
  discharge_active_value: int(1,2400)
  debug: bool
```
Keys `homeassistant_api`, `hassio_api`, `SUPERVISOR_TOKEN`, `supervisor/core`: no match anywhere in that repo (grep over *.yaml, *.sh, *.md, other files). `services: mqtt:want` is used; `script/run.sh:21-29` reads `bashio::config` and, when `mqtt_server` has no value, borrows the Supervisor broker (comment at run.sh:21).

### 3.3 addon/Dockerfile (facts; full file at /home/markus/Dokumente/GitHub/go-zendure2mqtt/addon/Dockerfile)
- Line ~29 `ARG BUILD_FROM` before first FROM; stage 1 `FROM golang:1.27.2-alpine AS builder`, `WORKDIR /src`, copies go.mod/go.sum, `go mod download`, `COPY . .` (build context = repo root), `ENV CGO_ENABLED=0`, `go build -trimpath -ldflags="-s -w -X ...internal/version.Version=${VERSION} ...Commit=${COMMIT} ...BuildDate=${BUILD_DATE}" -o /out/zendure2mqtt ./cmd/zendure2mqtt`.
- Stage 2 `ARG BUILD_FROM` / `FROM ${BUILD_FROM}`; `COPY --from=builder /out/zendure2mqtt /usr/bin/zendure2mqtt`; `COPY --from=builder /src/zendure.yaml /app/zendure.yaml`; `WORKDIR /app`; `COPY script/run.sh /run.sh`; `RUN chmod +x /run.sh`; `EXPOSE 8080`; `ARG BUILD_ARCH=amd64`, `ARG BUILD_VERSION=0.11.0`; `LABEL io.hass.version="${BUILD_VERSION}" io.hass.type="addon" io.hass.arch="${BUILD_ARCH}"`; `CMD ["/run.sh"]`. Header comment: `docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest -f addon/Dockerfile -t go-zendure2mqtt-addon .`
- `script/run.sh:1` shebang `#!/usr/bin/with-contenv bashio`.

### 3.4 addon/build.yaml (verbatim essentials; used only when config.yaml has no `image:`)
```yaml
build_from:
  aarch64: "ghcr.io/home-assistant/aarch64-base:latest"
  amd64: "ghcr.io/home-assistant/amd64-base:latest"
  armv7: "ghcr.io/home-assistant/armv7-base:latest"
labels:
  org.opencontainers.image.title: "go-zendure2mqtt"
  org.opencontainers.image.source: "https://github.com/SukramJ/go-zendure2mqtt"
  org.opencontainers.image.licenses: "MIT"
```

### 3.5 repository.yaml (verbatim)
```yaml
# Home Assistant add-on repository descriptor. Lets users add this GitHub
# repository under Settings -> Add-ons -> Add-on Store -> Repositories; the
# Supervisor then discovers the add-on in the addon/ subdirectory.
name: go-zendure2mqtt Add-ons
url: https://github.com/SukramJ/go-zendure2mqtt
maintainer: SukramJ <sukramj@icloud.com>
```

### 3.6 addon/DOCS.md headings
L1 `# go-zendure2mqtt add-on`; L3 `## Quickstart`; L21 `## Options reference`; L44 `## Topics`; L73 `### Maintenance topics`; L85 `### Discovery`; L101 `### Downgrading to 0.7.x or earlier`.

### 3.7 .github/workflows/addon-image.yml
- name "Add-on: Build and Push"; triggers `push: tags: '**'` and `workflow_dispatch` (input `tag`); permissions `contents: read`, `packages: write`; concurrency group `addon-${{ github.ref }}`, cancel-in-progress; env `IMAGE: ghcr.io/sukramj/go-zendure2mqtt-addon`.
- One job `build_and_push` (`if: github.repository_owner == 'SukramJ'`, ubuntu-latest, matrix fail-fast false): amd64 / linux/amd64 / ghcr.io/home-assistant/amd64-base:latest; aarch64 / linux/arm64 / aarch64-base:latest; armv7 / linux/arm/v7 / armv7-base:latest.
- Steps: actions/checkout@v7 (fetch-depth 0); "Resolve version + build metadata" (VERSION from input tag, else tag name without `v`, else `git describe`; COMMIT short sha; BUILD_DATE UTC); docker/setup-qemu-action (pinned sha, v4); docker/setup-buildx-action (v4); docker/login-action (v4, ghcr.io, `GITHUB_TOKEN`); docker/build-push-action (v7): `context: .`, `file: ./addon/Dockerfile`, `platforms`, `push: true`, `provenance: false`, build-args `BUILD_FROM`, `BUILD_ARCH`, `BUILD_VERSION`, `VERSION`, `COMMIT`, `BUILD_DATE`; tags `${IMAGE}-${arch}:latest` and `:${VERSION}`; gha cache scoped per arch.
- The `home-assistant/builder` action is NOT used in this workflow (grep of the file: it uses docker/build-push-action).

## 4. go-hamapping surface (branch h1/rows @ a89aa548; main @ 69aa9ca holds only docs/meta files: AI_POLICY.md CLA.md CLAUDE.md CONCEPT.md LICENSE Makefile README.md changelog.md doc.go go.mod)

h1/rows root-package non-test files: binding.go (376 lines), doc.go, measure.go, plan.go (428), row.go (208), rows_gen.go (2352), value.go (353). Also rows/{binary_sensor,button,climate,cover,event,fan,light,lock,number,select,sensor,switch,text}.yaml, internal/gen, internal/rowfile, docs/rows.md, docs/by_design.md, docs/reports/H1.md, notes/reads/read-h1-hamapping.md.

### 4.1 Exported API (grep, non-test)
row.go:17 `type Row`; :53 `Measurement`; :60 `Part`; :66 `Caveat`; :73 `Cluster`; :96 `Feature`; :103 `Attr`; :114 `Command`; :136 `Field`; :144 `Value`; :179 `Linear`; :184 `When`; :200 `Set`.
plan.go:31 `Verdict`; :39 `Endpoint`; :63 `Patch`; :79 `Plan`; :97 `func Map(dev *model.Device, entities []model.Entity) (*Plan, []Verdict)`; :264 `func (p *Plan) Build(_ context.Context) ([]*device.Device, error)`; :336 `func (p *Plan) DeviceFor(key string) *device.Device`; :350 `func (p *Plan) Apply(ctx context.Context, key string, st model.State) []Patch`; :401 `func (p *Plan) OnChange(fn func(model.Command))`; :417 `func (p *Plan) Close()`; :428 `func Rows() []Row`. plan.go consts/vars: `ErrNotBuilt` (error "hamapping: plan not built"), `RideOnPrimary = "primary"`.

### 4.2 Types (plan.go)
- `Verdict` (:31): `Key string`, `Row string`, embedded `eligibility.Verdict` (go-fabric `eligibility`: Mappable/Partial/Unmappable with reason, device type, clusters).
- `Endpoint` (:39): `Key`, `Row`, `Type *device.Type`, `Options device.Options`, `Declarations []device.Declaration`, `Features []string`, `Bindings []model.Binding`, `Riders []string`.
- `Patch` (:63): `Key`, `Cluster string`, `Value any` (typed `<cluster>def.Patch`, nil for reachability), `Reachable *bool`, `Err error`.
- `Plan` (:79): exported `Device *model.Device`, `Endpoints []Endpoint`; unexported bindings/devices/mu/fn/unsub/built.
- `Map` doc (plan.go ~:92-96): every entity gets a verdict, every mapped one an endpoint; a row that rides (`RideOnPrimary`) puts its clusters on the primary endpoint (first mapped entity whose row does not ride); entities taken in order.
- `Row` (row.go:17): `ID` ("<platform>/<name>"), `Platform`, `When When`, `Type *device.Type`, `State eligibility.State`, `Reason` (`{device_class}` placeholder), `Caveats []Caveat`, `Ride string`, `Cites []string`, `Clusters []Cluster`, `RideMeasurement *Measurement`, `Parts []Part`. `When` (row.go:184): `DeviceClass []string`, `Unit []string`, `StateClass []string`, `Features *Set`, `Extra map[string]Set`, `Options *bool`. `Set` (:200): `All, Any, None []string`, `Present *bool`, `True *bool`. `Value` (:144) `From` forms: "state", "state.<key>", "extra.<key>", "device_class", "unit", "desc.min", "desc.max", "desc.step".

### 4.3 Input types (go-hamqtt HEAD d1bb9ac, /home/markus/Dokumente/GitHub/go-hamqtt/model/)
- `model.Device` (device.go:169): `Identity Identity` (Identity at :81 has `IDs []Identifier` :82, `Connections []Connection` :83), `Name Localized`, `Manufacturer`, `Model`, `ModelID`, `SWVersion`, `HWVersion`, `SerialNumber`, `SuggestedArea`, `ConfigURL`, `Via *Identity`, `Extra map[string]any`.
- `model.Entity` interface (entity.go:27): `Key() string`, `Platform() hacatalog.Platform`, `Desc() *Description`, `Bindings() []Binding`. `model.Basic` (entity.go:45): `EntityKey string`, `EntityPlatform hacatalog.Platform`, `Description Description`, `Binds []Binding`.
- `model.Description` (description.go:180) fields with lines: Name 182 (Localized), NameKey 195, NameArgs 212, NameNull 245, DeviceClass 250 (`DeviceClass`), StateClass 251 (`hacatalog.StateClass`), Unit 252 (`Unit`), Icon 256, Category 259, Enabled 264 (*bool), Options 267 (*Enum), Precision 270 (*int), Availability 276, Optimistic 290, JSONAttributesTopic 302, JSONAttributesTemplate 303, ValueTemplate 322, CommandTemplate 349, Multiplier 361 (*float64), Extra 366 (`map[string]any`).
- `model.State` (state.go:23): `Value any`, `Available bool`, `Origin Origin`, `ModifiedAt`, `RefreshedAt time.Time`, `Extra map[string]any`. `model.Command` type at entity.go:80.

### 4.4 Predicate reading of Description.Extra
- binding.go:60 (`When.matches`-style loop): `for key, set := range w.Extra { own, stated := d.Extra[key]; v, _ := extra(e, key); if !set.holds(list(v), stated && own != nil) ...; if set.True != nil { t, _ := v.(bool); t != *set.True } }`; `Set.holds` binding.go ~:75-90: `Present` compares with whether the entity states the key; `All`/`Any`/`None` test list membership; a key left out reads the catalog default (value.go:73 `extra` returns `Description.Extra[key]` (value.go:77), else the catalog's default for the key).
- Features: binding.go:93-97 `features(e)` reads `e.Desc().Extra[keyFeature]` (`supported_features`): an integer bit mask decoded with go-ha-catalog `data/features.json` flag values, or a list of flag names (upper-cased).
- `when:` keys used in rows/*.yaml `extra:` (count): `supported_color_modes` 4 `any`, `color_temp_kelvin` 8, `modes` 8 (+1 `any`), `preset_modes` 4, `fan_modes` 1, `swing_modes` 1, `payload_open` 1 (`present: true`). Also `features: {any: [OPEN]}` (lock), `device_class`, `unit`, `state_class`, `options`.
- Example rows/lock.yaml: row `door_lock`, `when: {}`, `type: DoorLock`, `verdict: mappable`; caveats with `features: {any: [OPEN]}` and `extra: {payload_open: {present: true}}`; attribute `LockState` from `state` with enum locked/unlocked/jammed/open; commands `LockDoor` raw `extra.payload_lock` catalog true, `UnlockDoor` raw `extra.payload_unlock`.
- State keys read by rows: `Value.From` "state.<key>" reads a key of a map state, else `State.Extra` (row.go Value doc).

## 5. go-fabric facade (main @ 019277ed)

### 5.1 node.Config (node/config.go:43 `type Config struct`), field and line
Network 45; Commissioning 48; ProductDescription 52; BasicInformation 55; Store 58; Advertiser 63; DeviceTypeValidation 67 (`DeviceTypeValidationMode`); Attestation 73; OTA 76 (`*OtaConfig`); OTAProvider 82; CommissionerControl 86; FixedLabels 88 (`[]fixedlabeldef.LabelStruct`); Locales 94; Groups 106 (`*Groups`); Topology 116 (`Snapshotter`); EndpointPersistence 122; AggregatorServers 126 (`[]contract.ClusterServer`); SupportedFabrics 132 (`uint8`); DisableTimeSynchronization 141; GeneralDiagnosticsFeatures 148; MaxPathsPerInvoke 156; CapabilityMinima 163; Limits 166; Client 170; BootID 173; Health 176; Observers 180; Metrics 184 (`Metrics` = `contract.Metrics`, metrics.go:12); Diagnostics 187; Logger 191 (`*slog.Logger`); Clock 200 (`clock.Clock`).
- `node.Diagnostics` (node/diagnostics.go:10): `Capacity int`; `DefaultDiagnosticsCapacity = 256` (diagnostics.go:18).
- `node.Health` (node/health.go:27): `Interval time.Duration` (0 = `DefaultProbeInterval`), `OnSample func(HealthSample)`; probe runs when OnSample set or Interval > 0; `HealthSample = bridge.HealthSample` (health.go:16); `func (n *Node) Health() (HealthSample, bool)` (health.go:96; false before first verdict or with no probe).
- `func (n *Node) PairingCodes() (qr, manual string, err error)` (node/node.go:973; from ProductDescription, Commissioning.Discriminator/Passcode); `func (n *Node) PairingCodesFor(creds WindowCredentials) (qr, manual string, err error)` (credentials.go:77).
- `func (n *Node) Aggregator() *Aggregator` (node/node.go:873).

### 5.2 Aggregator API (node/aggregator.go)
`type Endpoint interface` :31; `type Aggregator struct` :86; `type Partition struct` :126; `Aggregator.Partition(name) *Partition` :153; `RetirePartition(ctx, name) error` :211; `Add(ctx, ep Endpoint) error` :243; `Replace(ctx, key device.SourceKey, ep Endpoint) error` :258; `Remove(ctx, key device.SourceKey) error` :268; `MarkComplete()` :278; `Endpoints() []Endpoint` :281; `Batch(ctx, fn func(b *Batch) error) error` :285; Partition has the same Add/Replace/Remove/MarkComplete/Endpoints/Batch (:291-:350); `type Batch struct` :328 with `Add(ep Endpoint) error` :375, `Replace(key device.SourceKey, ep Endpoint) error` :400, `Remove(key device.SourceKey) error` :438, `MarkComplete()` :461; `IncreaseConfigurationVersion(key) error` :515; `IncreaseConfigurationVersions(keys ...device.SourceKey) error` :539.
Batch doc (aggregator.go ~:337-349): each call changes the set at once; none reassembles; after fn a started node with at least one change reassembles exactly once and removed/replaced devices are then unbound; what applied stays applied whatever fails; not a transaction; before `Node.Start` a batch only changes the set; result is errors.Join of fn's error and the reassembly's.

### 5.3 docs/guides/embed-in-a-daemon.md headings (line)
1 `# Embed in a daemon`; 17 `## 1. The dependency`; 33 `## 2. node.Config from the daemon's config`; 71 `## 3. The device: one daemon entity as a typed declaration`; 125 `## 4. The aggregator batch at the device run`; 150 `## 5. Readings to attributes`; 174 `## 6. Lifecycle and reachability`; 197 `## 7. The pairing code into the UI and MQTT`; 214 `## 8. Running it`; 231 `## What you did not have to do`; 247 `## What a controller sees`.

### 5.4 docs/guides/operations.md headings (line)
1 `# Operations`; 12 `## Files`; 49 `## Ports and addresses`; 65 `## mDNS`; 89 `## Logs`; 116 `## Health`; 127 `## Limits`; 141 `## A failed pairing`; 158 `## Factory reset`; 181 `## Upgrading`.
