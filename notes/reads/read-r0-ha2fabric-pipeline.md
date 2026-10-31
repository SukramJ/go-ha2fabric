# Read map R0 — go-ha2fabric: adapter, fake HA, bridge, config, release (Sonnet read, 2026-10-31; a map: file:line for every fact, no judgement; verify before acting)

Paths are relative to the repo root. Line numbers inside long functions (`sync`, `assess`, `Event`) were spot-checked by grep for call sites; ranges ending in `-` are starts only. Line numbers differ from the brief's guesses (command.go table is :34-44, Method switch :58-134, there is no `extra` read block at :185-199; see 1.3). go-hamapping lines are in the module cache `/Users/markus/go/pkg/mod/github.com/!sukram!j/go-hamapping@v0.2.1/` (HM/).

## 1. internal/adapt

### 1.1 adapt.go (324 lines)
- `Entity(in Input) *model.Basic` adapt.go:113. `Input{EntityID, Registry *ha.EntityEntry, State *ha.State, Device *model.Device, TemperatureUnit, Name}` :85-111.
- Entity builds `model.Description{Name, DeviceClass(attrs.device_class), StateClass(attrs.state_class), Unit(attrs.unit_of_measurement), Extra: description(domain, attrs)}` :124-130; Name override :131-133; climate Unit = installation unit :134-138; Category from registry :139-141; `Options` enum from attrs.options :142-144; Min/Max/Step from attrs min/max/step :145.
- Bindings: state (Read) and command (Write), slot = `{Address: Device.UID(), Channel: EntityID, Bucket: BucketValues, Path: [role]}` :146-158. EntityPlatform = `hacatalog.Platform(domain)` :152.
- `description(domain, attrs)` adapt.go:173-223. Common key: `ex["supported_features"]` (int from attrs.supported_features) :175-179. Domain switch :180-221, ONLY light, climate, fan (no lock/cover/switch/sensor case):
  - light :181-195: `ex["supported_color_modes"]=strings_(attrs.supported_color_modes)` :186; `ex["color_temp_kelvin"]=true` :191; `copyNum(ex,"min_kelvin",attrs,"min_color_temp_kelvin")` :192; `copyNum(ex,"max_kelvin",attrs,"max_color_temp_kelvin")` :193; `ex["brightness_scale"]=255` :195.
  - climate :196-211: `ex["modes"]=strings_(attrs.hvac_modes)` :202; copyNum `min_temp`, `max_temp` :204-205; `temp_step` from `target_temp_step` :206; `fan_modes`, `preset_modes`, `swing_modes` (non-empty lists) :207-210.
  - fan :212-217: `ex["preset_modes"]=strings_(attrs.preset_modes)` :215.
- Helpers: `str` :258, `strings_` :263, `toFloat` :280, `toInt` :292, `num` :297, `floats` :304, `copyNum(dst,key,src,from)` :320.
- `Device(d *ha.DeviceEntry, areas map[string]string) *model.Device` adapt.go:44; `OwnDevice(entityID, name) *model.Device` :77 (device of an entity without a registry device). Namespaces `NamespaceDevice="ha"`, `NamespaceEntity="ha-entity"` :30-37.
- `State(st *ha.State) model.State` adapt.go:225-256. `Extra = maps.Clone(st.Attributes)` :230. Only `light` normalised :234-253: delete `color_temp`; `color_temp = color_temp_kelvin` :238-239; `color.h/.s` from `hs_color` :242-244; `color.x/.y` from `xy_color` :245-247; set/delete `color` :248-252. Climate/fan/lock/cover: attributes pass through unchanged (no branch).
- `Available := st.State != ha.StateUnavailable && st.State != ha.StateUnknown` :254; return `model.State{Value: st.State, Available, Extra: ex}` :255.
- Nothing in adapt.go knows a per-entity setting (Input has no code/options field).

### 1.2 command.go (224 lines)
- `ErrCommand = errors.New("adapt: command has no service")` command.go:23.
- `payloads = map[string][]struct{key, service string}` :34-44:
  - switch :36 `{"payload_on","turn_on"},{"payload_off","turn_off"}`; fan :38 same; cover :41 `payload_open→open_cover, payload_close→close_cover, payload_stop→stop_cover`; lock :43 `{"payload_lock","lock"},{"payload_unlock","unlock"}`.
- `Service(cmd model.Command, cur *ha.State) (ha.ServiceCall, error)` :49. Local `call(service, data)` builds `ha.ServiceCall{Domain: ha.Domain(id), Service, EntityID: id, Data}` :54-57. `payload := strings.TrimSpace(string(cmd.Payload))` :58.
- Switch :59-134 (cases, with the payload decode each uses):
  - :60 `domain=="light" && Method==""` -> `light(id, cmd.Payload)` (JSON decode :140-179: state OFF→turn_off; brightness clampInt 0..255; color_temp>0 → `color_temp_kelvin` int; color h,s→`hs_color`; x,y→`xy_color`; r,g,b→`rgb_color`).
  - :62 `Method==""` (any other domain): loop `payloads[domain]`, `catalogPayload(cmd.Entity, p.key)` equals payload → `call(p.service, nil)`.
  - :68 cover `MethodSetPosition`: `integer(payload,0,100)` → `set_cover_position {position}`.
  - :73 cover `MethodTilt`: `integer(0,100)` → `set_cover_tilt_position {tilt_position}`.
  - :78 climate `MethodMode`: non-empty → `set_hvac_mode {hvac_mode: payload}`.
  - :83 climate `MethodTemperature`: `number(payload)` → `set_temperature {temperature}`.
  - :88-107 climate `MethodTemperatureLow|High`: number; other bound read from `cur.Attributes[target_temp_high|low]` via `toFloat` :99-102; missing → ErrCommand :103; → `set_temperature {target_temp_low, target_temp_high}` :105.
  - :108 fan `MethodPercentage`: `integer(0,100)` → `set_percentage {percentage}`.
  - :113 fan `MethodPresetMode`: non-empty → `set_preset_mode {preset_mode}`.
  - :118 fan `MethodOscillation`: payload vs `catalogPayload(e,"payload_oscillation_on"/"_off")` → `oscillate {oscillating: bool}`.
  - :126 fan `MethodDirection`: "forward"|"reverse" → `set_direction {direction}`.
  - fallthrough error :134 `fmt.Errorf("%w: %s method %q payload %q", ErrCommand, id, cmd.Method, payload)`.
- `catalogPayload(e model.Entity, key string) (string, bool)` :188-205: `e.Desc().Extra[key].(string)` else `hacatalog.LoadMQTT().Platforms[platform].Keys[key].Default`. This is the only `Extra` read in command.go.
- `number` :207, `integer(s,lo,hi)` :212, `clampInt` :221.
- `hamapping.Method*` constants (HM/rows_gen.go:37-49), the complete list; the Service switch handles all ten:
```
MethodDirection="direction" MethodMode="mode" MethodOscillation="oscillation"
MethodPercentage="percentage" MethodPresetMode="preset_mode" MethodSetPosition="set_position"
MethodTemperature="temperature" MethodTemperatureHigh="temperature_high"
MethodTemperatureLow="temperature_low" MethodTilt="tilt"
```
- No lock Method case exists. Lock row (HM/rows/lock.yaml:20-28): commands `LockDoor` `raw: {from: extra.payload_lock, catalog: true}`, `UnlockDoor` `raw: {from: extra.payload_unlock, catalog: true}` (no `method:`/PIN field in the row at v0.2.1; Command.Method is "" for these, so they hit command.go:62 branch).
- `model.Command{Entity, Role, Slot, Method, Payload []byte}` (go doc github.com/SukramJ/go-hamqtt/model.Command).

### 1.3 adapt tests (internal/adapt/adapt_test.go)
- helpers `state(id,s,attrs)` :21, `entity(id,st)` :25, `cmd(id,method,payload)` :191; `TestEntitiesMeetTheirRows` :33 (table, lock rows :74-75), `TestEntityDescription` :114, `TestState` :163, `TestService` :196 (table rows `{cmd, cur, want ha.ServiceCall}` e.g. :223 lock LOCK→`ha.ServiceCall{Domain:"lock",Service:"lock",EntityID:"lock.a"}`), `TestAppliedStateIsNotACommand` :255.

## 2. internal/ha and hatest

### 2.1 client (internal/ha/client.go)
- `Options{URL, Token, Events []string, PingInterval, CallTimeout, MinBackoff, MaxBackoff, ReadLimit, Logger}` :53-77; Token doc "It is never logged." :57.
- `Handler interface{ Connected(haVersion string); Event(ev Event); Disconnected(err error) }` :79-90.
- `New(opts) *Client` :102; `(*Client).Connected() bool` :129; `Run(ctx, h Handler) error` :135. Subscriptions = `Options.Events` re-subscribed after each auth (private `subscribe` :408); there is no public `Subscribe`.
- `Call(ctx, cmd map[string]any, out any) error` :420; `ListEntities` :447; `ListDevices` :455; `ListAreas` :463; `ListLabels` :472; `GetStates` :480; `GetConfig` :488; `ListExposed` :497; `CallService(ctx, sc ServiceCall) error` :506 (sends `{"type":"call_service","domain","service","target":{"entity_id"},"service_data"}` :507-514).
- Errors: ErrAuthInvalid, ErrNotConnected, ErrDisconnected :30-40. Auth message built :254.
- message.go: `ServiceCall{Domain, Service string; Data map[string]any; EntityID string}` :327-332, `String()` :334; `State{EntityID, State, Attributes, LastChanged, LastUpdated}` :126-; `StateUnknown` :137 (`StateUnavailable` adjacent); `StateChanged{EntityID, OldState, NewState *State}` :144; `EntityEntry{ID, EntityID, UniqueID, Platform, DeviceID, AreaID, DisabledBy, HiddenBy, EntityCategory, HasEntityName, Name, OriginalName, Options map[string]map[string]any, Labels}` :189-210; `Domain(entityID) string` :215; `(*EntityEntry).Domain()` :212; event constants `EventStateChanged/EntityRegistryUpdated/DeviceRegistryUpdated/LabelRegistryUpdated` :38-41.
- Event list the daemon subscribes: cmd/ha2fabric/main.go:49.

### 2.2 hatest fake (internal/ha/hatest/hatest.go, 582 lines)
- Fixtures: `Fixtures() string` :73 (testdata/ha beside repo root via runtime.Caller); `Fixture(tb, name) json.RawMessage` :79 (reads "result" or "message"; fails if `_mirrors` missing :93-95); `decode[T]` :102.
- `New(tb testing.TB) *Server` :113: loads entity_registry_list, device_registry_list, area_registry_list, label_registry_list, get_states, get_config, expose_entity_list :116-127; cleanup stops it.
- Methods: `URL() string` :137; `Authenticated() int` :140; `Down()` :148; `Up()` :162; `SetState(entityID, state string, attrs map[string]any)` :362 (attrs nil keeps old attrs :371-373; unknown id is APPENDED :383-385, fires `state_changed` :387); `State(entityID) map[string]any` :390; `UpdateEntity(entityID, changes map[string]any)` :404 (fires entity_registry_updated action update); `UpdateDevice(deviceID, changes)` :423; `RenameEntity(oldID,newID)` :442; `UpdateLabel(labelID,name)` :468; `RemoveLabel(labelID)` :483; `SetExposed(entityID, exposed bool)` :500; `RemoveEntity(entityID)` :516 (keeps entry for restore); `RestoreEntity(entityID)` :544; `Calls() []ha.ServiceCall` :559; `WaitCall(ctx, match func(ha.ServiceCall) bool) (ha.ServiceCall, error)` :566.
- No `AddEntity` method. A new entity exists only by (a) adding it to the fixture JSON files, or (b) `SetState` for an id with no registry entry (appended state only), or (c) `RestoreEntity` of a removed one. `serviceCall(cmd)` decoder :307; `command()` handler :244; `serve` :168.
- Example of a test creating a new entity by SetState: internal/bridge/bridge_test.go:127 `e.ha.SetState("cover.garage","closed", map[string]any{"device_class":"garage","supported_features":3,"friendly_name":"Garage"})` (rule includes `cover.garage` :109).

### 2.3 testdata/ha/ fixtures
- Files: area_registry_list.json, auth_invalid.json, auth_ok.json, auth_required.json, device_registry_list.json, entity_registry_list.json, expose_entity_list.json, get_config.json, get_states.json, label_registry_list.json. Each has a `_mirrors` citation key (required by hatest.Fixture) and a `result` (or `message`) payload.
- Entities in fixtures (both registry and states): light.kitchen, sensor.living_temperature, sensor.living_battery, switch.coffee, binary_sensor.front_door, cover.living_blind, climate.hall, lock.front, fan.ceiling, light.cellar, light.attic, sensor.processor_use, sensor.outdoor; sensor.legacy_yaml (states only/registry-less per bridge_test).
- Registry entity shape (entity_registry_list.json:1-31, light.kitchen):
```
{"area_id":null,"categories":{},"config_entry_id":"ce-hue","config_subentry_id":null,"created_at":1761350400.0,
 "device_id":"dev-hue-1","disabled_by":null,"entity_category":null,"entity_id":"light.kitchen","has_entity_name":true,
 "hidden_by":null,"icon":null,"id":"reg-light-kitchen","labels":[],"modified_at":1761350400.0,"name":null,
 "next_name_part":null,"options":{"conversation":{"should_expose":true}},"original_name":null,"platform":"hue",
 "translation_key":null,"unique_id":"hue-1-light"}
```
  (`_mirrors` at :2 cites config/entity_registry.py:39-64 and helpers/entity_registry.py:369-403 @2026.10.3.) lock.front registry entry at entity_registry_list.json:205; its expose entry expose_entity_list.json:23.
- State shape (get_states.json, lock.front at :166, single-line form via json):
```
{"entity_id":"lock.front","state":"locked","attributes":{"friendly_name":"Front door lock","supported_features":1},
 "last_changed":"2026-10-25T08:00:00.000000+00:00","last_reported":"...","last_updated":"...",
 "context":{"id":"01JB0000000000000000000000","parent_id":null,"user_id":null}}
```
  fan.ceiling state: attrs `preset_modes:null, oscillating:false, percentage:50, percentage_step:33.33…, preset_mode:null, friendly_name:"Ceiling fan", supported_features:51`.
- bridge_test.go:62 expects lock.front verdict row `lock/door_lock`, `partially_mappable` (supported_features 1 = OPEN, HM/rows/lock.yaml:13-16).

## 3. internal/bridge

- `Upstream` interface (bridge.go:52-62): Connected, ListEntities, ListDevices, ListAreas, ListLabels, GetStates, GetConfig, ListExposed, CallService (*ha.Client satisfies it).
- `Options{Rule expose.Rule; Debounce time.Duration (0 = 1 s); Logger; Selection *selection.Store; OnChange func(exposure []string, endpoints bool)}` bridge.go:65-83. Defaults set in `New(n *node.Node, up Upstream, opts Options) *Bridge` :175-195.
- `Verdict` struct bridge.go:85-: `EntityID, Name, Device, Exposed, ExposeReason, Source, Labels, Row, State, Reason, DeviceType uint16, Endpoint uint16` (json tags) + private `verdict eligibility.Verdict`.
- `Status` :106-120; `group` :122-137; `Bridge` :139-173 (channels `work`, `cmds`; maps `groups, owner, known, states`).
- `Run(ctx) error` :197; `enqueue` :221; `Synced() <-chan struct{}` :231; `Connected(string)` :235; `Disconnected(error)` :243; `Event(ev ha.Event)` :259-: state_changed → `enqueue(stateChanged)` :261-267; entity_registry_updated → `noteRemoved` if action remove :268-278, then `resyncSoon()` :280; device/label registry updated → `resyncSoon()` :281-285.
- Debounce: `resyncSoon()` bridge.go:337-349 (`time.AfterFunc(opts.Debounce, …work<-sync)`, resets timer each call).
- `Snapshot{Entities, Devices, Areas, Labels, States, Config, Exposed}` :372; `Read(ctx, up, rule) (*Snapshot, error)` :394; `Plan() []Verdict` :365; `Status()` :358.
- `assess(s, rule, names) ([]Verdict, []*planned)` :438-549: per entity `rule.Decide` :485; device = `adapt.Device` :510 or `adapt.OwnDevice` :516; entity = `adapt.Entity(adapt.Input{EntityID, Registry, State, Device, TemperatureUnit: unit, Name: names[id]})` :526; `g.descs[id]=describe(ent)` :532; group `sig` = device JSON + `key=desc` per entity :539-541. `describe(e)` :551-557 = JSON of platform + `*model.Description` (so any Extra change changes the sig and replaces the device).
- Map call: `mapOptions(regs, names) hamapping.Options` bridge.go:593-607 — `Options{StableKey: regs[e.Key()].ID, Name: names[e.Key()]}` only these two; `hamapping.Map(g.device, g.entities, mapOptions(g.regs, g.names))` at :613 (Dry) and `hamapping.Map(p.device, p.entities, mapOptions(p.regs, p.names))` :940 (build). go doc: `func Map(dev *model.Device, entities []model.Entity, opts ...Options) (*Plan, []Verdict)` HM/plan.go:124.
- `sync(ctx)` bridge.go:633-: Read :637; assess :645; build for new/changed groups (sig differs) :675; `b.n.Aggregator().Batch(ctx, func(bt *node.Batch) error {…})` :715- using `bt.Remove`, `bt.Forget`, `bt.Replace(k, dev)`, `bt.Add(dev)`; `g.plan.Close()` on closing groups :763; `b.apply` of every state :777; log `bridge.synced` :817; `OnChange` call :820-821.
- `build(ctx, p *planned) (*group, []hamapping.Verdict)` :939-: `hamapping.Map` :940, `plan.Build(ctx)`, on error log `bridge.build_failed` and mark verdicts unmappable, `plan.OnChange(b.onCommand)` :962.
- `apply(ctx, g, id)` :967-985: `g.plan.Apply(ctx, id, adapt.State(st))`; per-patch errors logged debug `bridge.apply`.
- `stateChanged(ctx, sc ha.StateChanged)` :987-1008: updates `b.states`; unknown/removed → `resyncSoon`; compares `g.descs[id]` with `descOf(...)` (:1010-1012, rebuilds adapt.Entity) → `resyncSoon` if description moved; then `apply`.
- Command path (internal/bridge/command.go, 80 lines): plan's `OnChange` = `b.onCommand(c model.Command)` :24 (non-blocking enqueue to `b.cmds` size 256; full → log `bridge.command_dropped`); `commandLoop` :34; `command(ctx, c)` :45-: `cur := b.states[id]`; `sc, err := adapt.Service(c, cur)` :50; error → log `bridge.command_unmapped` :52, `CommandErrs++`; `b.up.CallService(cctx 30 s timeout, sc)` :58 (`commandTimeout` :20); error → log `bridge.call_service_failed` :67 and re-`apply` state; ok → debug `bridge.call_service` :77.
- Log events emitted in bridge package: bridge.apply, bridge.batch, bridge.build_failed, bridge.call_service, bridge.call_service_failed, bridge.command_dropped, bridge.command_unmapped, bridge.event_refused, bridge.forget, bridge.label_unresolved, bridge.queue_full, bridge.sync_failed, bridge.synced, bridge.upstream_unusable (+ bridge.selection_applied provider.go:284).
- Node config: `NodeConfig(c *config.Config, logger) node.Config` bridge/node.go:35-62.
- Exposure provider (provider.go): `(*Bridge).Exposure() fabricui.ExposureProvider` :28; `provider{b}` :30; field consts `entity_id, name, domain, device, area, labels, assist, platform, verdict, source` :33-44; `schema` fabricui.Schema columns Entity, Name, Domain(filter), Device, Area(filter), Labels(filter, Multi), Exposed to Assist, Integration; `Groups: true`; filters Verdict and Decided by :46-78; `Schema` :80; `view` struct (`snap, verdicts, all, reg, devs, areas, names`) :83-91; `view.item(c *Verdict) fabricui.Item` :114 (Item{Key, Name, Group, Enabled, Source, Verdict…}); `List` :163; `check` :175; `proposed` :196; `Preview` :219; `Apply(ctx, updates []fabricui.Update) (int, error)` :247-; `current := p.b.opts.Selection.Entries()` :265; builds `selection.Entry{Enabled: u.Enabled, Name: name}` only :278; `Selection.Set(set, remove)`; logs `bridge.selection_applied` :284.
- Update fields used: `u.Key`, `u.Enabled`, `u.Name *string` (provider.go:265-278). No other per-entity field is read.

## 4. Tests: pattern

- No bridge e2e test drives lock, fan, cover or climate through a node (grep of internal/bridge/*_test.go: lock/cover/climate appear only in bridge_test.go:60-63 Dry verdicts and :109-129 a cover.garage re-sync). Existing e2e coverage is `light.kitchen` (OnOff), `sensor.living_temperature` (TemperatureMeasurement), switch.coffee.
- End-to-end test: `TestEndToEnd` internal/bridge/bridge_e2e_test.go:151-267. Env: `newEnv(t, rule, edit ...func(*bridge.Options)) *env` :54-92 (hatest.New → `fabrictest.NewSite(t, {Clock: clock.System{}, Logger})` → `site.AddNode(bridge.NodeConfig(&config.Config{Passcode:20202021, Discriminator:3840, VendorID:0xFFF1, ProductID:0x8000, Label:"go-ha2fabric"}, …))` → `ha.New(ha.Options{URL: srv.URL(), Token: hatest.Token, Events:[4 events], MinBackoff 20ms…})` → `bridge.New(dev.Node(), cl, opts{Debounce 20ms})`; runs `br.Run` and `cl.Run(rctx, br)`; waits `<-br.Synced()`).
- Helpers: `(e *env) commission()` :94-102 (`site.AddController(fabrictest.ControllerOptions{})`, `ctrl.Commission(ctx, dev, fabrictest.CommissionOptions{})` → `e.pair`); `endpoint(entityID) (uint16, bool)` :104-113 (via `br.Plan()`); `regID` :115; `eventually(what, cond)` :130 (20 s poll); `reachable(ep)` :141; `waitReport(reports, what, match)` :269-281.
- Quote, read an attribute (bridge_e2e_test.go:168-181):
```go
values, err := fabrictest.ReadCluster(e.ctx, e.pair, light, onoffdef.ClusterID)
if err != nil { t.Fatal(err) }
if st, _ := onoffdef.StateFrom(values); !st.OnOff { t.Fatalf("the kitchen light is off: %+v", st) }
values, err = fabrictest.ReadCluster(e.ctx, e.pair, temp, temperaturedef.ClusterID)
...
if st, _ := temperaturedef.StateFrom(values); st.MeasuredValue == nil || *st.MeasuredValue != 2150 {
```
  signature: `fabrictest.ReadCluster(ctx, p *Pair, endpoint uint16, cluster uint32) (spec.AttributeValues, error)` go-fabric@v0.15.4/fabrictest/wire.go:105. Cluster packages: `github.com/SukramJ/go-fabric/cluster/spec/<cluster>` imported as `<x>def` (e.g. `onoffdef`).
- Quote, state change reaches a subscription (:185-206): `fabrictest.Subscribe(e.ctx, e.pair, fabrictest.SubscribeRequest{Attributes: []fabrictest.AttributePath{client.AttributeAt(ep, cluster, attr)…}, Updated: func(r *fabrictest.Report){reports<-r}})` (`Subscribe` wire.go:200); `e.ha.SetState("sensor.living_temperature","22.25",nil)`; `e.waitReport(reports, …, func(r){ st,_ := temperaturedef.StateFrom(r.Values(ep, cluster)); _, ok := r.Value(ep, cluster, attr) … })`.
- Quote, inject a command from the controller side and assert the HA call (:208-219): 
```go
if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(light, onoffdef.ClusterID, onoffdef.CmdOff), nil); err != nil { t.Fatal(err) }
call, err := e.ha.WaitCall(e.ctx, func(c ha.ServiceCall) bool { return c.EntityID == "light.kitchen" && c.Service == "turn_off" })
if err != nil || call.Domain != "light" { t.Fatalf("turn_off %+v %v", call, err) }
```
  `fabrictest.Invoke(ctx, p, path CommandPath, fields spec.Encodable) (Result, error)` wire.go:176 (`InvokeTimed` :182). Controller is an in-process `fabrictest.Controller` on the site. Existing calls pass `nil` fields (OnOff has none); a DoorLock LockDoor carries fields (whether it includes a PIN field in `lockdoor`/`doorlockdef`: UNVERIFIED, not read).
- Registry change test: `e.ha.UpdateEntity("switch.coffee", map[string]any{"disabled_by":"user"})` :228, `e.dev.EndpointOf("ha:dev-shelly-1/"+e.regID("switch.coffee"))` :223.
- Final status assertion `e.br.Status()` :260.
- Other e2e files: bridge_f2_e2e_test.go (TestRemovedEntityIsForgotten :17, TestLabelWithAComma :92), bridge_h3_e2e_test.go (TestLabels :32, TestRenameKeepsTheEndpoint :72, TestDisabledAndHiddenNeverAppear :95, newUI :119, TestWebUI :246, TestUIWarnings :369); unit: bridge_test.go (snapshot :23, byID :37, TestDryPlan :47, TestNodeConfig :93, TestResync :108). Log toggle: env `HA2FABRIC_TEST_LOG` bridge_e2e_test.go:28-33.
- Pattern for a new-domain test = fixture entity (JSON in testdata/ha/get_states.json + entity_registry_list.json [+ device_registry_list.json, expose_entity_list.json]) → `newEnv(t, rule)` → `e.commission()` → `e.endpoint(id)` → `ReadCluster`/`StateFrom` → `e.ha.SetState(...)` + `Subscribe`/`waitReport` → `fabrictest.Invoke` + `e.ha.WaitCall`. Adapter unit tests add rows to adapt_test.go tables (:33-112, :196-253). Note: a new fixture entity changes `TestDryPlan` expectations (bridge_test.go:47-91) and the `sensor`/candidate counts: UNVERIFIED which asserts count candidates.

## 5. internal/config, add-on options

- Load order: environment first, each overridden by its flag; package doc config.go:4-9 ("access token is read from the environment only … never printed"). `Load(args []string, env Env, out io.Writer) (*Config, error)` :140; `Env func(key) (string,bool)` :124; `OSEnv` :127; `MapEnv(map)` :130; helper `str(name, envKey, def, usage)` :150 registers `flag` with env default.
- Flags/env (each :157-173): ha-url/HA_URL, listen/LISTEN, passcode/PASSCODE, discriminator/DISCRIMINATOR, vendor-id/VENDOR_ID, product-id/PRODUCT_ID, label/LABEL, store-path/STORE_PATH, include/INCLUDE, exclude/EXCLUDE, include-labels/INCLUDE_LABELS, exclude-labels/EXCLUDE_LABELS, expose-assist/EXPOSE_ASSIST, log-level/LOG_LEVEL, web-listen/WEB_LISTEN, web-allow/WEB_ALLOW, web-admin/WEB_ADMIN; bool flags `-dump-plan` :174, `-version` :175 (no env).
- No HA token flag. Token read: `supervisor := get(SupervisorTokenEnv)` config.go:184, `c.HAToken = get("HA_TOKEN")` :186, else supervisor :187-188 (`SupervisorTokenEnv="SUPERVISOR_TOKEN"` :33). Field doc: "never logged or persisted" :64-66; missing-token issue :217.
- `Config` struct config.go:60-: HAURL, HAToken, Addon bool, Listen, Passcode uint32, Discriminator uint16, VendorID, ProductID uint16, Label, StorePath, Include, Exclude, IncludeLabels, ExcludeLabels []string, ExposeAssist bool, SelectionPath, LogLevel slog.Level, WebListen, WebAllow []string, WebAdmin bool, DumpPlan, Version bool. `Exposure() expose.Rule` :109. Validation issues aggregated into `*ValidationError` :114. `SplitLabels` :304 (comma or newline split). `Passcode` read from env/flag and validated :228 (not logged: main.go:74 logs only version, ha_url, addon).
- Token never logged: ha/client.go:57 `// Token is the access token. It is never logged.`; used only in auth message client.go:254; docs/operations.md:98-100 "Either token is read from the environment only … and never logged or written to disk"; main.go:74 `logger.Info("ha2fabric.start", version, ha_url, addon)`. No slog redaction handler (`ReplaceAttr`) found: logger is `slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))` cmd/ha2fabric/main.go:64. The ha.Client logger gets the same logger (:65).
- Config → bridge: cmd/ha2fabric/main.go:51 `config.Load`; `bridge.NodeConfig(cfg, logger)` :80; `bridge.New(n, client, bridge.Options{Rule: cfg.Exposure(), Logger, Selection: sel, OnChange…})` :94-105 (bridge.Options carries only Rule, Debounce, Logger, Selection, OnChange: bridge.go:65-83). Config is not passed to bridge directly.
- Add-on options → env: addon/config.yaml `options:` :48-60, `schema:` :62-78 (port, passcode, discriminator, vendor_id, product_id, label, expose_assist, include[], exclude[], include_labels[], exclude_labels[], log_level); version :7. `script/run.sh` (61 lines) reads `/data/options.json` through `bashio::config` and exports env: `LISTEN` :16, `PASSCODE` :17, `DISCRIMINATOR` :18, `VENDOR_ID` :19, `PRODUCT_ID` :20, `LABEL` :21, `STORE_PATH` :24, `EXPOSE_ASSIST` :27, `INCLUDE/EXCLUDE/INCLUDE_LABELS/EXCLUDE_LABELS` via `bashio::config.has_value` :30-49, `LOG_LEVEL` :51, `WEB_LISTEN` :56, `WEB_ALLOW` :57; then `exec /usr/bin/ha2fabric` :61.
- Flow for a new option (as the existing ones do): addon/config.yaml options+schema → script/run.sh `export X="$(bashio::config 'x')"` → config.go `str("flag","ENV",def,usage)` + Config field + validation → main.go passes into bridge.Options (or adapt) → docs. 
- addon/translations/: DOES NOT EXIST (`ls addon/translations` fails; addon/ holds CHANGELOG.md, DOCS.md, Dockerfile, README.md, build.yaml, config.yaml, icons). Option labels: addon/DOCS.md (UNVERIFIED which lines list options; not read).
- A list-typed option arrives newline-separated (run.sh:28-29 comment). No option in config.yaml is a `password` schema type (all listed above).

## 6. internal/selection

- File `{"version":1,"entities":{"<entity_id>":{"enabled":true,"name":"…"}}}` selection.go:6-9 (doc); `const Version = 1` :31; file path = `filepath.Join(Dir(StorePath), "selection.json")` config.go:205 (`SelectionFile` :52; add-on `/data/selection.json`).
- Struct (selection.go:34-):
```go
type Entry struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name,omitempty"`
}
type file struct { Version int `json:"version"`; Entities map[string]Entry `json:"entities"` }
```
- API: `Open(path, log) (*Store, error)` :74; `Recovered() (Recovery, bool)` :111; `Path()` :119; `Entries() map[string]Entry` :122; `Enabled() map[string]bool` :129; `Set(updates map[string]Entry, remove []string) error` :142; atomic `write` :159 (temp file + fsync + rename, perms from os.CreateTemp = 0600; indent 2). A wrong version or parse failure → renamed `<path>.corrupt-<unix>`, log `selection.corrupt` :84-97.
- Entry has no field for a per-entity setting besides Enabled and Name; `Set` replaces whole Entry values; provider.Apply builds `selection.Entry{Enabled, Name}` (bridge/provider.go:278) so an added field would be dropped unless Apply copies `current[u.Key]` (`current := ...Entries()` :265). Selection is read by bridge via `Selection.Enabled()` (bridge.go:826 rule()) and `names()` (:835).
- Operator doc of the file: docs/by_design.md "Where the selection lives" :106.
- Store file is plaintext JSON; docs/reports/ and roadmap.md:132-134 state "the code never lands in a log or the selection file" (docs/roadmap.md R4, lock users and PINs).

## 7. Release process

- `make version-check` (Makefile:101-105): `RELEASE_VERSION` = first `Version = "…"` in internal/version/version.go (Makefile:18), must equal `version:` in addon/config.yaml (:7) and `changelog.md` must have `## [<v>]`.
- `make release` (:107-122): depends on version-check; cross-compiles linux/amd64, linux/arm64, linux/arm/v7 (RELEASE_TARGETS :17) into `dist/`, payload README.md LICENSE changelog.md THIRD-PARTY-NOTICES.md (:19), notes by `script/extract-release-notes.sh <v>` → dist/RELEASE_NOTES.md :121. Other targets: help, setup, build, vet, test, race, fmt, fmt-check, lint, tidy-check, cover, cover-check, fuzz-smoke, vuln, licenses, secrets, docker, addon-image, ci (= build vet test lint :124), clean, version.
- internal/version/version.go: `Version = "0.3.7"` :23; Commit, BuildDate vars :24-27; `GoFabric()`, `GoHamapping()` via debug.ReadBuildInfo :30-33.
- changelog.md head: preamble lines 1-5 (Keep a Changelog / SemVer), `## [Unreleased]` :7 (empty), `## [0.3.7] - 2026-10-30` :9 with intro line "Dependency release: go-fabric `v0.15.4`, go-fabric-ui `v0.2.8`." :11, `### Changed` :13 with two bullets (:15-22); `## [0.3.6] - 2026-10-30` :24, same shape (intro :26, `### Changed` :28).
- addon/CHANGELOG.md: header :1-7, entries `## 0.3.7` :9 (one bullet), `## 0.3.6` :14, `## 0.3.5` :19 — plain version heading, one-paragraph bullets.
- README.md status line :22-24: "> **Status: 0.2.3.** The daemon and its Home Assistant add-on run; …" (CLAUDE.md also says "State: 0.2.3"; version.go says 0.3.7).
- script/coverfloor/floors.txt: comment lines (`#`) then `<package path><spaces><integer floor>`: cmd/ha2fabric 75, internal/adapt 90, internal/bridge 82, internal/config 92, internal/expose 95, internal/ha 80, internal/selection 90, internal/version 75, internal/web 85 (floors.txt lines 1-12, ratchet).
- Workflows (.github/workflows): ci.yml `name: ci` with jobs lint (:34), security (:57), test (:79), coverage (:97), fuzz-smoke (:109), build (:121), docker (:137); codeql.yml job analyze (:28); addon-image.yml "Add-on: Build and Push"; docker-build-push.yml "Docker: Build and Push"; release-on-tag.yml "Release: Create from Tag"; dependabot-auto-merge.yml job auto-merge.
- CLAUDE.md rules touching a change: user-visible change → changelog `[Unreleased]` entry; one PR per wave; English docs; no attribution lines.

## 8. Docs structure

- docs/operations.md headings: `# Operations` :1; `## Files` :9; `## Ports and addresses` :48; `## mDNS` :68; `## The token` :78; `## What the daemon reads and writes` :100 (table of WS commands incl. `call_service` row :107); `## Logs` :113 (events table :119-132: ha2fabric.fabrics, ha2fabric.pairing, ha.connected/ha.disconnected, bridge.synced, bridge.upstream_unusable, bridge.build_failed, bridge.call_service_failed, bridge.command_unmapped, bridge.label_unresolved, bridge.selection_applied, bridge.forget, selection.corrupt); `## Labels` :134; `## The web UI` :153; `## Health` :183; `## Pairing` :195; `## Factory reset` :210.
- docs/by_design.md headings: `# By design` :1; `## Exposure model` :10; `## Unresolved label names` :25; `## Label refresh` :36; `## Hidden entities` :47; `## Unknown state` :55; `## Removal grace` :65; `## Naming` :86; `## Composition` :96; `## Where the selection lives` :106.
- docs/roadmap.md: R1 sensor classes :32; R2 buttons, events, selects :68; R3 climate :96; R4 switch and lock details :122 (lock PIN item :130-135: "A Home Assistant lock that requires a `code` cannot be driven… the mapping would carry one credential the operator configures… Fable work… the code never lands in a log or the selection file"); R5 new domains :140 (vacuum, valve, water_heater, media_player, cover details, light effects/fan speed_count; "Owner: go-hamapping, each a larger row"); R6 camera :167; Not planned :206; Sources :225. Roadmap "How a tranche runs" :19-31.
- docs/reports/: F1.md, F2.md, H2.md, H3.md.

## 9. go.mod and Map

- go.mod: `go 1.27.3`; go-fabric v0.15.4; go-fabric-ui v0.2.8; go-ha-catalog v0.3.2-0.20261024093528-d9582e022b8f; go-hamapping v0.2.1; go-hamqtt v0.39.0; coder/websocket v1.8.15 (go.mod:5-12).
- Map is called with `hamapping.Options{StableKey, Name}` only (bridge.go:593-607); `Options` doc (go doc github.com/SukramJ/go-hamapping.Options) lists exactly those two fields at v0.2.1.
- hamapping rows dir (HM/rows/): binary_sensor, button, climate, cover, event, fan, light, lock, number, select, sensor, switch, text .yaml. Row mechanism for a domain: `platform`, `rows[].{id, when, type, verdict, caveats, clusters[].{attributes, commands[].{handle, raw|method…}}}` (lock.yaml as example). Per CLAUDE.md "No mapping of our own": new-domain rows belong in go-hamapping; this repo adds adapt (`description` case, `State` normalisation, `payloads`/`Service` case) and tests.
- Where a new domain touches this repo (grep-derived): internal/adapt/adapt.go:180 domain switch (Extra keys); adapt.go:234 State normalisation (if any); internal/adapt/command.go:34-44 `payloads` and :59-134 Method switch; testdata/ha fixtures; bridge tests; docs/operations.md log table only if a new log event; changelog.md `[Unreleased]`. The bridge package itself has no per-domain code.

## 10. Facts relevant to a per-lock `code` option (measured only)

- Current lock path: `payloads["lock"]` command.go:43 → `call("lock"|"unlock", nil)` command.go:62-67 → `ha.ServiceCall{Data: nil}` → `CallService` sends no `service_data` when `len(Data)==0` (client.go:511).
- `ha.ServiceCall.Data` is `map[string]any` message.go:330, so `{"code": "…"}` can be carried without client changes.
- `adapt.Service(cmd, cur)` receives only `model.Command` and the current `*ha.State` (command.go:49); it has no access to config, selection, or the bridge (called from bridge/command.go:50). A per-lock code would need a new parameter or lookup passed through `bridge.command` (UNVERIFIED design; not read from any source).
- Config holds no lock/code field (config.go:60-107); `run.sh` exports none; `addon/config.yaml` has no such option.
- Selection `Entry` has Enabled/Name only (selection.go:34-); file is plaintext JSON, 0600 via CreateTemp; roadmap R4 :132-135 says the code must not land in a log or the selection file.
- Logging of commands: `bridge.call_service` (debug) and `bridge.call_service_failed` (warn) log `sc.String()` = `domain.service entity_id` (message.go:334) — `Data` is not part of `String()`. `bridge.command_unmapped` logs `err.Error()` which contains method and payload (command.go:134 format; payload of the Matter command, not the code).
- HA lock service fields (code): not read in this pass; command.go:42 cites lock/services.yaml:3,25 for lock/unlock; whether `code` is a field there is UNVERIFIED (HA core is read from raw files at tag 2026.10.3, never run — CLAUDE.md).
- Whether go-fabric's DoorLock LockDoor/UnlockDoor carry a PINCode field to the host and how go-hamapping would deliver it (the lock row has `raw: {from: extra.payload_lock}` only, HM/rows/lock.yaml:25-28) is UNVERIFIED: go-hamapping would need to deliver it, owner per roadmap R4 is go-hamapping.
