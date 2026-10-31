# Roadmap — what a Home Assistant entity cannot become yet

Written 2026-10-31 from a survey of three sources: Home Assistant's
platform vocabulary as go-ha-catalog carries it (`data/mqtt.json`,
`data/device_classes.json`), go-hamapping's row table (`rows/*.yaml`,
`rows_gen.go` at v0.2.1) and go-fabric's device kit (`device/*_gen.go`,
`cluster/`). Each item names the repository that owns it; by this
repository's rules ("No mapping of our own", `CLAUDE.md`) most of the
work is a row in go-hamapping, and go-ha2fabric follows with its
adapter and fixtures. Items are ordered by value per row; work them
top down, one tranche per wave, one pull request per repository.

Every claim below carries its source. A line marked **unverified** is a
reading question for the brief, not a fact.

Legend: `[ ]` open, `[~]` in progress (brief named), `[x]` done (release
named).

## How a tranche runs

1. A Sonnet read of the sources the item names, written to
   `notes/reads/read-<tranche>.md` with `file:line` for every fact.
2. A brief in go-hamapping (`notes/briefs/`) naming the rows, their
   device types and the Home Assistant keys each predicate reads; the
   owner verifies every cited line.
3. The rows with their round trip through go-fabric's in-process
   controller, then go-ha2fabric: `internal/adapt` (state and command),
   a fixture in `testdata/ha/`, a `hatest` case, `docs/operations.md`
   when the operator sees a difference, `changelog.md`.
4. The item here moves to `[x]` with the release.

## R1 — Sensor device classes the kit already serves

Owner: go-hamapping. Largest gap between kit and table: nine sensor
classes are mapped (`rows/sensor.yaml:10-191`), about fifty are listed
as unmappable "in H1" (`rows/sensor.yaml:221-224`), and go-fabric has
device types for a good part of them.

- [~] **Air quality**: `carbon_dioxide`, `carbon_monoxide`, `pm1`, `pm25`,
  `pm10`, `volatile_organic_compounds`, `nitrogen_dioxide`, `ozone`,
  `radon`, `aqi` → go-fabric `device/air_quality_sensor_gen.go` with the
  concentration clusters (`cluster/measurement/concentration.go`).
  Reading question: which concentration cluster each class takes, the
  unit each cluster measures in, and what `aqi` becomes (AirQuality
  enum from a numeric index is a mapping decision, not a fact).
  Landed in go-hamapping v0.3.0 / go-ha2fabric 0.4.0 (`rows/sensor.yaml`
  carbon_dioxide … radon, AirQualitySensor). Left: `aqi`, which stays
  unmappable — a numeric index onto the AirQuality enum is a mapping
  decision.
- [~] **Electrical**: `apparent_power`, `power_factor`, `frequency`,
  `reactive_power`, `reactive_energy` → `device/electrical_sensor_gen.go`
  (ElectricalPowerMeasurement, ElectricalEnergyMeasurement). The
  existing `power`, `voltage`, `current`, `energy` rows
  (`rows/sensor.yaml:91-190`) are the pattern; check whether they already
  declare these clusters so the new classes become optional attributes
  on the same device rather than new rows.
  Landed in go-hamapping v0.3.0 / go-ha2fabric 0.4.0: apparent_power,
  reactive_power, frequency, power_factor on the ElectricalSensor. Left:
  `reactive_energy`.
- [x] **Flow**: `volume_flow_rate` → `device/flow_sensor_gen.go`
  (`cluster/measurement/flow.go`). Unit conversion from Home Assistant's
  flow units to the cluster's is the reading question.
  Done in go-hamapping v0.3.0 / go-ha2fabric 0.4.0 (FlowSensor).
- [x] **Soil**: `moisture` (sensor, percentage) →
  `device/soil_sensor_gen.go` (`cluster/measurement/soil.go`).
  **Unverified**: whether Home Assistant's `moisture` sensor class means
  soil moisture or general humidity in the integrations that use it.
  Done in go-hamapping v0.3.0 / go-ha2fabric 0.4.0 (SoilSensor;
  go-hamapping `docs/by_design.md`).
- [x] **Leave unmappable, with the reason written into the row**: `date`,
  `timestamp`, `duration`, `monetary`, `enum`, `data_rate`, `data_size`,
  `signal_strength`, `uptime`, `distance`, `speed`, `weight`, `volume`,
  `area`, the weather classes (`precipitation*`, `wind_*`,
  `irradiance`), `blood_glucose_concentration`, `ph`, `conductivity`.
  No Matter device type exists for these in go-fabric's kit; the row's
  verdict should say "no Matter device type" without the "in H1" dating.
  Done in go-hamapping v0.3.0 / go-ha2fabric 0.4.0:
  `sensor/no_device_type` lists only the classes no row maps, without
  "in H1".

## R2 — Buttons, events and selects

Owner: go-hamapping. The platforms `button`, `event`, `select`, `number`
and `text` have an explicit unmapped row each (`rows_gen.go:419, 851,
1879, 1885`; `rows/button.yaml`, `event.yaml`, `number.yaml`,
`select.yaml`).

- [x] **`button`** → `device/generic_switch_gen.go` as a momentary switch:
  a press from the controller side has no meaning (a button is an input
  to Home Assistant, not an output), so the row is state-only: Home
  Assistant's `button` state is the last press timestamp; a change
  emits InitialPress and ShortRelease. Reading question: the GenericSwitch
  event semantics go-fabric serves (`contract.SwitchEventEmitter`,
  `CHANGELOG.md:174`) and how a controller shows a bridged switch that
  never has a "current position".
  Done in go-hamapping v0.3.0 / go-ha2fabric 0.4.0, as matter-hub does
  it rather than as written here: an OnOffPlugInUnit whose On presses
  the button (`press`), its OnOff always off (go-hamapping
  `docs/by_design.md`).
- [~] **`event`** → GenericSwitch too, with `event_types` as the
  positions when the type count fits (MultiPressMax), else a
  single-position switch per type. **Unverified**: whether Apple Home
  and Google Home show a bridged GenericSwitch at all; matter-hub's
  behaviour at `664575a` is the first thing to read.
  Landed in go-hamapping v0.3.0 / go-ha2fabric 0.4.0: a GenericSwitch
  whose every new event is a short press (InitialPress, ShortRelease).
  Left: long and multi presses from the event type.
- [x] **`select`** → `device/mode_select_gen.go` (ModeSelect): options
  become modes with generated mode numbers, the current option is
  CurrentMode, ChangeToMode calls `select.select_option`. Needs the
  stable option order from Home Assistant's `options` attribute.
  Done in go-hamapping v0.3.0 / go-ha2fabric 0.4.0 (the options in Home
  Assistant's order, ChangeToMode → `select_option`).
- [~] **`number`**: no direct Matter device type. Leave unmappable with
  the reason; revisit only if a consumer names a use.
  The verdict says so without "in H1" (go-hamapping v0.3.0 /
  go-ha2fabric 0.4.0). Left: a use a consumer names.
- [~] **`text`**: no Matter device type; stays unmappable.
  Unchanged in go-hamapping v0.3.0 / go-ha2fabric 0.4.0. Left: nothing
  unless Matter gains a counterpart.

## R3 — Climate features

Owner: go-hamapping; go-ha2fabric's `internal/adapt` (`command.go:78-104`)
follows. The thermostat row (`rows/climate.yaml:11-25`) excludes four
things with caveats "not mapped in H1".

- [ ] **Preset modes** → Thermostat Presets (go-fabric
  `device/thermostat_hold_declaration_test.go` shows the declaration).
  Home Assistant `preset_modes` → presets with generated handles;
  `climate.set_preset_mode` on SetActivePresetRequest.
  Open after go-hamapping v0.3.0: go-fabric's Thermostat server requires
  a heating or cooling setpoint on every preset, and Home Assistant's
  preset modes carry none (the caveat of `rows/climate.yaml`).
- [ ] **HVAC mode `auto`** (the device's own schedule): no SystemMode
  value; the Thermostat Schedules feature
  (`device/thermostat_schedules_declaration_test.go`) is the candidate.
  Reading question: whether a schedule the device owns and the bridge
  cannot read can be declared honestly. If not, the caveat stays.
- [x] **Fan modes** on a climate entity: a second endpoint with
  FanControl, as a composed device. Reading question: whether the
  Thermostat device type allows a Fan part or needs an Aggregator-level
  sibling; matter.js's room-air-conditioner element
  (`device/room_air_conditioner_gen.go`) is the pattern to check.
  Done in go-hamapping v0.3.0 / go-ha2fabric 0.4.0 for an entity with
  FAN_MODE, TURN_ON and TURN_OFF: a RoomAirConditioner with FanControl
  (`fan_mode`) and OnOff (`turn_on`, `turn_off`). Without turn_on and
  turn_off the fan modes stay a caveat.
- [ ] **AC FanMode Off**: the air conditioner's FanMode Off write sends
  `set_fan_mode off`, which Home Assistant refuses unless `off` is in
  `fan_modes` (`climate/__init__.py:470-488` at 2026.10.3); the
  `air_conditioner` row should gate the Off write on `fan_modes`
  containing `off` or map it to `turn_off`. Owner: go-hamapping.
- [~] **Swing modes**: no Matter counterpart in go-fabric's kit; stays a
  caveat with the reason.
  Still a caveat in go-hamapping v0.3.0 / go-ha2fabric 0.4.0
  (`climate/air_conditioner`, `climate/thermostat`). Left: a Matter
  counterpart.
- [~] **Humidity on the thermostat**: `current_humidity` →
  RelativeHumidityMeasurement as an optional part. Small; bundle with
  presets.
  Landed in go-hamapping v0.3.0 / go-ha2fabric 0.4.0 on the
  RoomAirConditioner (RelativeHumidityMeasurement from
  `current_humidity`). Left: the plain thermostat.

## R4 — Switch and lock details

Owner: go-hamapping.

- [x] **`switch` with `device_class: switch`** stays an OnOffPlugInUnit
  (`rows/switch.yaml:6-8`). Owner decision 2026-10-31: follow matter-hub,
  which creates a plug-in unit for every `switch` and reads no
  `device_class` (`packages/backend/src/matter/endpoints/legacy/switch/index.ts:13-22,
  34-68` at `664575a`). Documented in go-hamapping's `docs/by_design.md`
  (wave R1–R5).
- [x] **Lock users and PINs**: closed without a code of our own (owner
  decision 2026-10-31). Home Assistant keeps a per-lock default code in
  the entity registry's lock options and inserts it when a `lock`,
  `unlock` or `open` call carries none (`lock/__init__.py:113-133,
  280-295` at 2026.10.3); the daemon's calls carry none
  (`internal/adapt/command.go:43, 62-67`), so the operator sets the code
  in Home Assistant. `docs/operations.md` §Locks with a code says where.
  A code never passes through this daemon, its logs or its selection
  file. Matter-side PIN credentials (matter-hub stores the controller's
  PIN and forwards it as `data.code`, `behaviors/lock-server.ts:841-870`)
  stay out of scope.

## R5 — New domains with a kit device type

Owner: go-hamapping, each a larger row with command structure; one
domain per wave.

- [~] **`vacuum`** → `device/robotic_vacuum_cleaner_gen.go` (RvcRunMode,
  RvcOperationalState, RvcCleanMode, ServiceArea in
  `cluster/servicearea`). Home Assistant's `vacuum` states and
  `supported_features` (`data/features.json`) are the predicate.
  Landed in go-hamapping v0.3.0 / go-ha2fabric 0.4.0:
  RoboticVacuumCleaner with RvcRunMode and RvcOperationalState for an
  entity with START. Left: RvcCleanMode (fan speed) and ServiceArea.
- [~] **`valve`** → `device/water_valve_gen.go` (ValveConfigurationAndControl
  in `cluster/valve`): open, close, position when `set_valve_position`
  is supported.
  Landed in go-hamapping v0.3.0 / go-ha2fabric 0.4.0: WaterValve, open
  and close. Left: the valve position (go-fabric's valve server serves
  no Level feature).
- [~] **`water_heater`** → `device/water_heater_gen.go`
  (`cluster/energy/waterheater.go`): modes and target temperature.
  Landed in go-hamapping v0.3.0 / go-ha2fabric 0.4.0 as a heating-only
  Thermostat (SystemMode Off/Heat, setpoint, limits). Left: the
  WaterHeater device type (modes, boost).
- [x] **`media_player`** → `device/speaker_gen.go` for an entity with
  volume only, `device/basic_video_player_gen.go` /
  `casting_video_player_gen.go` for one with media control. Reading
  question: which of Home Assistant's `media_player` features map onto
  MediaPlayback, MediaInput, KeypadInput and LevelControl; a reference
  behaviour exists in matter-hub at `664575a`.
  Done in go-hamapping v0.3.0 / go-ha2fabric 0.4.0: a tv is a
  BasicVideoPlayer (OnOff, MediaPlayback, KeypadInput power and volume
  keys), every other player a Speaker (OnOff, LevelControl from
  `volume_level`). Sources and tracks are a caveat.
- [ ] **`cover` details**: the row is complete for lift and tilt
  (`rows/cover.yaml:20-49`). Nothing open; listed so the survey is
  complete.
- [ ] **`light` effects** (`effect_list`) and **`fan` `speed_count`**:
  no Matter counterpart for effects; the fan's step count could set
  SpeedMax when the controller benefits. Low value; last in R5.

## R6 — Camera

Owner: go-hamqtt first, then go-hamapping, then go-ha2fabric; go-fabric
only for a convenience default. The facts:

- go-fabric serves the whole camera surface (`cluster/camera`, device
  types `camera`, `snapshot_camera`, `floodlight_camera`,
  `video_doorbell`, `chime`) and holds "no media stack: the host runs
  the WebRTC peer connection" (`cluster/camera/doc.go`, package comment).
- The Camera device type makes CameraAvStreamManagement (VDO, ADO, SNP)
  and WebRtcTransportProvider mandatory (`device/camera_gen.go:34-45`).
- Home Assistant at 2026.10.3 offers `camera/webrtc/offer`,
  `camera/webrtc/candidate`, `camera/webrtc/get_client_config`
  (`homeassistant/components/camera/webrtc.py:254, 335, 308`); the
  answer and candidates arrive as subscription messages (`:40-97`).
  Camera capabilities name only the stream types WEB_RTC and HLS
  (`camera/__init__.py:744-755`); no codec, resolution or frame rate.
- go-hamqtt's model has no camera with a stream; go-ha-catalog's MQTT
  `camera` is an image topic (`data/mqtt.json`, `image_topic`).

Finding of 2026-10-31 (`notes/reads/read-r6-camera.md`, owner-verified
on the go-fabric lines):

- chip's camera controller starts a live view with ProvideOffer
  (`examples/camera-controller/commands/liveview/LiveViewCommands.cpp:40`,
  checkout 1dc3b69ed5e); the specification makes both SolicitOffer and
  ProvideOffer mandatory on the provider (`WebRTC_Provider.xml:88, 199`).
  A relay can serve ProvideOffer and refuse SolicitOffer.
- chip's AllocateVideoStream validation reads only the request's ranges
  and the declared stream usages, no sensor parameters
  (`CameraAVStreamManagementCluster.cpp:2066-2141`); go-fabric checks
  the same (`cluster/camera/avstream.go:708-731`). A placeholder stream
  declaration is possible.
- H264 is the mandatory codec (`CameraAVStreamManagement.xml:173-185`,
  data model 1.7); Home Assistant's go2rtc provider produces it
  (`homeassistant/components/go2rtc/__init__.py:414`).
- **Blocker:** go-fabric has no facade route for the camera to invoke
  `Answer`, `IceCandidates` and `End` on the controller's
  WebRtcTransportRequestor. `docs/guides/camera.md:38-58` covers the
  requestor side only, `cluster/camera/provider.go` has no outbound
  call, `Node.Peers()` is a seam without a facade route
  (`docs/guides/advanced-seams.md:37`). Filed as go-fabric #351.

R6 is therefore not built in the R1–R5 run (owner decision 2026-10-31:
build only on a green reading). Steps, in order, once go-fabric #351 is
released:

- [x] **Read**: the three questions above are answered; see the finding.
- [ ] **go-hamqtt**: a camera entity with a stream capability beyond
  MQTT discovery. A question to that repository.
- [ ] **go-hamapping**: a row kind that binds a port (`camera.ProviderPort`)
  instead of a service call; the row format knows commands with a
  payload only (`model.Command`: Method, Payload).
- [ ] **go-fabric** (necessary, #351): the signalling route — the provider
  invokes `Answer`, `IceCandidates` and `End` on the session's peer node,
  fabric and originating endpoint, as chip's provider manager does
  (`webrtc-provider-manager.cpp:1189-1242` at 1dc3b69ed5e). Sensible
  beside it: a kit default for a signalling-only camera that declares
  one opaque H264 video stream, beside `device/defaults_camera.go`.
- [ ] **go-ha2fabric**: the WebRTC subscription handling in
  `internal/ha`, the relay of offer, answer and candidates, snapshot via
  the camera proxy for SnapshotStream.

## Not planned, with the reason

- `alarm_control_panel`, `siren`, `humidifier`, `lawn_mower`, `scene`,
  `script`, `device_tracker`, `update`, `image`, `text`, `number`: no
  Matter device type in go-fabric's kit (`device/*_gen.go` surveyed
  2026-10-31). matter-hub may expose some as OnOff helpers; whether a
  consumer wants that is a product decision, not a gap.
- `binary_sensor` classes `battery`, `battery_charging`, `connectivity`,
  `gas`, `glass_break`, `heat`, `light`, `lock`, `moving`, `plug`,
  `power`, `problem`, `running`, `safety`, `sound`, `tamper`, `update`,
  `vibration` (`rows/binary_sensor.yaml:130`): no Matter type. `cold`
  has one (`device/water_freeze_detector_gen.go`) and could join R1 if
  a consumer asks. OnOffSensor as a generic fallback is a decision for
  go-hamapping's `docs/by_design.md`, not a default.
- Node capabilities of go-fabric a Home Assistant bridge does not need:
  Thread Border Router, OTA, TLS, Joint Fabric, Network Infrastructure.
- Bluetooth commissioning in the add-on: deferred to a later bundle S4 by
  owner decision (brief F2, 2026-10-26, head).

## Sources

- go-ha-catalog `data/mqtt.json`, `data/device_classes.json` (platform
  and class lists).
- go-hamapping v0.2.1: `rows/*.yaml`, `rows_gen.go`, `CONCEPT.md:21-22`
  (the vocabulary is go-hamqtt's), §Sequence.
- go-fabric: `device/*_gen.go`, `cluster/camera/doc.go`,
  `cluster/measurement/`, `cluster/energy/`, `CHANGELOG.md:148-176`
  (milestone 6).
- Home Assistant `home-assistant/core` at tag 2026.10.3, read with
  `git show`, never run.
