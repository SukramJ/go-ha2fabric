# Read map H3 — label selection (matter-hub), HA label registry, go-ha2fabric and go-hamapping insertion points

Sonnet read, 2026-10-25. A map: file:line for every fact, no judgement. Verify before acting.

## 1. home-assistant-matter-hub @664575a (/home/markus/Dokumente/GitHub/home-assistant-matter-hub)
Paths relative to packages/ unless they start with services/ or matter/ (those are under backend/src/).

Filter model, common/src/home-assistant-filter.ts
- :1-19 enum HomeAssistantMatcherType: pattern, regex, domain, platform, label (deprecated, :6), entity_label, device_label, entity_label_regex, device_label_regex, any_field_regex, area, entity_category, device_name, product_name, manufacturer, device_class
- :21-24 HomeAssistantMatcher {type, value: string}
- :26 HomeAssistantFilterMode = "any" | "all"
- :28-33 HomeAssistantFilter {include[], exclude[], includeMode?}; comment :31 default "any"
- common/src/home-assistant-entity-registry.ts:13, home-assistant-device-registry.ts:22, home-assistant-area-registry.ts:7: `labels?: string[]`

Matcher evaluation, services/bridges/matcher/matches-entity-filter.ts (271 lines)
- :17-33 testMatchers: "all" = every (:25-29); else some (:30-32); default "any" (:21)
- :35-89 testMatcher switch; null value → false (:42-44)
  - domain :46-47; label + entity_label one case :48-52 (value → slug, entity.labels.includes); device_label :53-56 (device.labels); entity_label_regex :57-58, device_label_regex :59-60 → testLabelRegex; any_field_regex :61-68; entity_category :69-70; platform :71-72; pattern :73-74 patternToRegex(value).test(entity_id); regex :75-76; area :77-78 (entity.area_id ?? device.area_id); device_name :79-80, product_name :81-82, manufacturer :83-84; device_class :85-86 (entityState.attributes.device_class); unknown → false :88
- Pattern :91-101: split on "*", escapeRegExp parts, join ".*", anchored ^$, case-sensitive
- Regex :103-110: new RegExp(pattern).test(value), unanchored, case-sensitive; bad pattern → false
- testLabelRegex :112-136: empty labels → false (:117-119); bad regex → false (:120-125); per slug: regex on slug (:127) and on the label name by label_id (:130-133)
- resolveLabelValue :255-271: with labels, case-insensitive name match → label_id (:259-264); else slugify (NFD, strip diacritics, lowercase, non [a-z0-9] runs → "_", trim "_") (:265-270); never compared to label_id directly
- entity_label reads entity.labels (:51), device_label reads device.labels (:55); no merged matcher; any_field_regex sees both as entity_labels=, entity_label_names=, device_labels=, device_label_names= (buildEntityHaystack :154-196; :188-191)
- any_field_regex haystack :181-195: entity_id=, domain=, platform=, area=, entity_category=, device_class=, label fields, device_name=, product_name=, manufacturer=
- device name fallback :205, :179: name_by_user ?? name ?? default_name
- device_name/product_name/manufacturer :198-253: case-insensitive; "*" → patternToRegex on lowercased; else substring includes
- Filter application, services/bridges/bridge-registry.ts: matchesFilter :1148-1176 (include only when include.length > 0 with includeMode :1160-1169; exclude always "any" :1170-1174; empty include matches everything); refresh() :1000-1030: disabled_by != null excluded (:1004-1006); hidden_by != null excluded unless featureFlags.includeHiddenEntities (:1009-1012); filter :1015-1017; _states/_devices from survivors :1019-1030; firstEntityMatching :1138-1148; labels from this.registry.labels :1141, :1158

Label registry fetch and refresh
- services/home-assistant/api/get-registry.ts: HomeAssistantLabel {label_id, name, icon?, color?} :31-36; getLabelRegistry "config/label_registry/list" :38-47; entity list :9-18; device list :20-29; area list :49-58
- services/home-assistant/home-assistant-registry.ts: _labels :65-67; runRegistryQueries :192- Promise.all of 5 (:202-217); label query `.catch(() => this._labels)` keeps last labels (:211-213, issue #500); areas fall back to [] (:214-216); fingerprint md5 over entity fields incl. labels (:222-226) and device fields incl. labels (:228-235); labels contribute only label_id (:236) — a rename keeping label_id is not in the fingerprint, names stored at :294 only on change; unchanged fingerprint → states updated (:252-253), returns false (:256-260); _labels assigned :294; untrusted snapshot dropped (:239-250); reload() :140-152 withRetry (10, 2000 ms base, 30000 ms max); enableAutoRefresh :100-131 setInterval refreshInterval*1000 (:130), skip when running (:104-110), onRefresh on change or runningSince change (:117-123); no label_registry_updated subscription — poll only
- interval default: commands/start/start-options-builder.ts:84-89 `home-assistant-refresh-interval` 60 s
- wiring: commands/start/start-handler.ts:176-178 enableAutoRefresh(() => bridgeService.refreshAll())
- API exposes labels: backend/src/api/matter-api.ts:423, :594

Filter change re-mounting endpoints
- services/bridges/bridge-service.ts:318-330 update(): bridge.update, bridgeStorage.add, onBridgeChanged
- services/bridges/bridge.ts:399-403 update(): dataProvider.update, refreshDevices(); :240-241 → endpointManager.refreshDevices()
- bridge-service.ts:284-291 refreshAll() per bridge
- services/bridges/bridge-endpoint-manager.ts:764- refreshDevices(): registry.refresh() (:765); defers if HA not running / registry empty (:777-787); pre-marks composed sub-entities (:795-823); present check ~:940-950; endpoint no longer matched but entity still present → endpoint.close() at once, keeps number/groups (:949-965; #468, #404); entity gone from registry waits ENDPOINT_REMOVAL_GRACE_MS 300_000 (:59) + newer snapshotGeneration, then endpoint.delete() (:970-999); new endpoints created ~:1150-1200, root.add :1196; mappingFingerprints (:101) rebuild existing (:1044)

Naming bridged devices
- matter/behaviors/basic-information-server.ts:44-48 nodeLabel precedence: ellipse(32, state.customName) → ellipse(32, registryName) (:41-43 = entity.registry?.name ?? original_name, only with featureFlags.preferEntityRegistryName) → ellipse(32, friendly_name) → ellipse(32, entity_id)
- :49-52 productNameFromNodeLabel flag; :84-111 patch: vendorName device.manufacturer; productName chain customProductName / productNameFromNodeLabel / model_id / model; serialNumber customSerialNumber / registry serial (flag useHaRegistrySerial) / anchor hash; uniqueId :111 frozenUniqueId(anchor); hw/sw version
- bridge nodeLabel = data.name trimmed to 32: utils/json/create-bridge-server-config.ts:39, matter/endpoints/server-mode-server-node.ts:60
- matter/endpoints/update-entity-state.ts:68: NodeLabel writable, follows immediately

Composing a device from several entities
- bridge-registry.ts:323-325 isAutoComposedDevicesEnabled() = featureFlags.autoComposedDevices
- user composition mapping.composedEntities (services/storage/entity-mapping-storage.ts:227-229); ids at matter/endpoints/entity-endpoint.ts:74-78; built at matter/endpoints/legacy/legacy-endpoint.ts:440-455 UserComposedEndpoint.create; not in server mode (:432-439)
- auto composition air purifier: legacy-endpoint.ts:23, ~:94; bridge-endpoint-manager.ts:808-822 fan.* with matterDeviceType air_purifier pulls temperature/humidity from same device_id
- auto-mapping battery/humidity/pressure/power/energy to same device_id: bridge-registry.ts preCalculateAutoAssignments :1032-1105 (temperature pulls humidity/pressure :1043-1070; battery :1085-1096); sets cleared per refresh (:1002-1008)

Unavailable and unknown
- matter/behaviors/basic-information-server.ts:93-95 reachable = state != null && state !== "unavailable" ("unknown" reachable)
- matter/behaviors/home-assistant-entity-behavior.ts:44-49 isAvailable = not unavailable and not unknown
- basic-information-server.ts:31-33 update() early return without state/attributes
- both as no data: rvc-run-mode-server.ts:165, rvc-operational-state-server.ts:120, legacy/appliance/index.ts:110, legacy/sensor/devices/evse-status.ts:26, services/diagnostics/diagnostic-service.ts:45
- bridge-registry.ts:117-122 (#450) batteryFingerprintFor; legacy-endpoint.ts:595 options fallback

## 2. HA core 2026.10.3 (raw files; base homeassistant/)
components/config/label_registry.py
- :43-50 async_setup registers list/create/delete/update
- :53-57 `config/label_registry/list`, handler :59-67, result list of _entry_dict; not require_admin
- :70-80 create: Required name; Optional color (cv.color_hex | SUPPORTED_LABEL_THEME_COLORS | None), description, icon; @require_admin :81; ValueError → "invalid_info" (:95-96)
- :101-106 delete: Required label_id; @require_admin; KeyError → "invalid_info" (:117-118); empty result (:120)
- :123-134 update: Required label_id; Optional color, description, icon, name; @require_admin :135; handler :137-152
- :155-166 _entry_dict: color, created_at (float), description, icon, label_id, name, modified_at (float)
- :13-40 SUPPORTED_LABEL_THEME_COLORS
helpers/label_registry.py
- :23-26 EVENT_LABEL_REGISTRY_UPDATED = EventType("label_registry_updated"); :27 STORAGE_KEY "core.label_registry"; :28-29 storage 1.2
- :32-41 _LabelStoreData; :50-54 EventLabelRegistryUpdatedData {action: "create"|"remove"|"update", label_id}
- :60-67 LabelEntry(NormalizedNameBaseRegistryEntry): label_id, description, color, icon (name, normalized_name from base)
- :112-126 async_get_label, async_get_label_by_name, async_list_labels; :130-132 _generate_id = labels.generate_id_from_name
- :135-166 async_create (duplicate name ValueError :146-149; event "create" :162-165); :169-180 delete "remove"; :183-224 update (no-op :207-208; "update" :216-222); :275 async_get_missing_label_ids; :285 async_load
helpers/entity_registry.py
- :83 EVENT_ENTITY_REGISTRY_UPDATED; :171-184 event data; :199 ("lb", "labels", True) index; :239 RegistryEntry; :264 `labels: set[str]`; :369 as_partial_dict; :393 `"labels": list(self.labels)`; :2003 labels update key; :2529-2532 async_clear_label_id
helpers/device_registry.py
- :401 BaseDeviceEntry; :412 `labels: set[str]`; :510 DeviceEntry; :589-629 dict_repr; :616 `"labels": list(self.labels)`; :3470 / :3687 labels in update changes
components/config/entity_registry.py:176 Optional("labels") on update; :243-245 filters missing ids. components/config/device_registry.py:161, :180-182 same.

## 3. go-ha2fabric @v0.1.0
internal/expose/expose.go: :1-13 doc (H2 §1.2); :22-28 Rule {Assist, Include, Exclude} (path.Match globs); :31-41 Entity {EntityID, Registered, DisabledBy, HiddenBy, ExposedToAssist} — no label field; :44-47 Decision; :50-57 Validate; :60-80 Decide order disabled (:62), hidden (:64), exclude (:67), include (:70), Assist (:73-79); :82-96 match; :99 Split
internal/bridge/bridge.go: :289-311 Read() ListEntities, ListDevices, ListAreas, GetStates, GetConfig, ListExposed only if rule.Assist (:307); :329- assess() builds expose.Entity{EntityID, Registered, ExposedToAssist} at :363 — DisabledBy/HiddenBy not set there; :444 Dry(); :459-466 sync uses opts.Rule (:62-63); :227-239 Event(): state_changed queued (:228-234), registry events → resyncSoon (:236-237), debounce opts.Debounce (:243-)
cmd/ha2fabric/main.go:45 events = state_changed, entity_registry_updated, device_registry_updated; :61 ha.New; :80 bridge.New(Rule: cfg.Exposure())
internal/ha: message.go:14-26 type constants; :34-38 event constants (no label_registry_updated); client.go:405-415 subscribe_events per type, Options.Events :59; commands :445-451 entity list, :453-459 device list, :461-467 area list, :473 get_states, :481 get_config, :490 expose_entity/list, :498 call_service — no label_registry/list; message.go:150-156 RegistryUpdated; :164-178 EntityEntry (no Labels); :193-208 DeviceEntry (no Labels); :213 Area; :222 Config; :240 Exposed; :251 ServiceCall
addon/config.yaml:48-58 options, :60-71 schema; internal/config/config.go:73-74 Include/Exclude/ExposeAssist; :90-92 Exposure(); :145-148 flags/env; :181 Split; :221-232 Validate

## 4. go-hamapping @v0.1.0
- row.go:137-150 Value.From doc: "state" / "state.<key>" / "extra.<key>" / device_class / unit / desc.min/max/step
- value.go:23-32 fromState, prefixSt, prefixEx; :37-56 lookupState (found when Value != nil || st.Available :44; "state.<key>" via path() then st.Extra :46-56); :59-72 path(); :76-81 extra() = Desc().Extra[key] else catalogDefault; :84-102 catalogDefault via hacatalog.LoadMQTT(); :105-122 describe
- row.go:167-173 Value.Per, Catalog
- go-hamqtt@v0.38.0 model: State state.go:23 (Value, Available :29, Extra :38); Description.Extra description.go:366; Device.Extra device.go:187
- go-ha2fabric fills: internal/adapt/adapt.go:101-138 Entity(); Description.Extra :120/:164; State(st) :215-248 (Extra = attributes clone + colour keys; Available :246; Value = st.State); command extra reads internal/adapt/command.go:185-199
- Stable identity: plan.go:236-259 options(): key := e.Key(); stable = uid + "/" + key when Device uid (:242-246); StableKey device.StringKey(stable) (:257); Name via device.ComposeNodeLabel (:247-256); DeviceAddress uid, ChannelAddress key (:258); part StableKey "%v#%d" (plan.go:309); e.Key() = go-hamqtt model/entity.go:31, :46 Basic.EntityKey, :53; go-ha2fabric EntityKey = EntityID (adapt.go:138); device Identifier{ha, d.ID} (adapt.go:31-33, :49), device-less "ha-entity" (:34-36, :79); plan.go:156, :115, :275-282 keyed by Key
- Method names: row.go:121-122 Method; rowfile.go:181 yaml "method"; binding.go:232-239 model.Command{Method}; literals in rows/*.yaml (climate.yaml:60-92 mode/temperature/temperature_low/temperature_high; cover.yaml:46,50 set_position/tilt; fan.yaml:49 percentage/preset_mode/oscillation/direction); rows_gen.go counts: direction 1, mode 1, oscillation 2, percentage 2, preset_mode 1, set_position 1, temperature 4, temperature_high 2, temperature_low 2, tilt 1; consumer go-ha2fabric internal/adapt/command.go:52-131 literals (:59-125); model.Command.Method go-hamqtt model/entity.go:88-92, Methods() :130; no exported constants in go-hamapping
