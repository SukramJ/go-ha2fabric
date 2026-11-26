# Phase 3 — unit C (citations)

Reviewer: Opus in Fable's role (owner rule 2026-11-24). Baseline `0a47095`
(read from `git archive 0a47095` in the scratch copy, never the worktree).
References only at their pins: core `2026.10.3` (`git show`), matter-hub
`664575a` (`git show`), go-fabric `v0.26.0`, go-fabric-ui `v0.14.1`,
go-hamapping `v0.8.0` (module cache), Supervisor `fe91b8db` (`gh api …?ref=`).
Prefixes: `core:` `hub:` `gf:` `ui:` `hm:` `sup:`. M = measured (read at the
pin), I = inferred.

No new open exploitable security item: **no held file**.

## Method

The map `notes/reads/read-review-p3-cites.md` lists 253 lines chosen by the
pins' names (`2026.10.3`, `664575a`, `fe91b8db`, `matter-hub`). It misses
most citations: `internal/adapt/command.go` alone carries 57, the map 2.
The probe (appendix A) therefore extracts every `path:line[-line][, …]`
token (with bare `:N` continuations) from every `.go`, `.md`, `.json`,
`.yaml` and `.sh` file outside `notes/` and `changelog.md`, resolves the
path at the pin the sentence names (or, for go-fabric, go-fabric-ui and
go-hamapping without a pin, at `go.mod`'s), checks the range exists and
dumps the cited lines. Every dumped range was read against the construct
the sentence names; the ones the script could not resolve (services.yaml,
Supervisor, ambiguous names, self-citations) were read by hand.

| | count |
| --- | --- |
| citation tokens extracted | 545 (417 outside `docs/reports/`, 128 in the reports) |
| by source, outside the reports | core 259 · go-fabric 44 · go-hamapping 19 · matter-hub 29 · Supervisor 9 · go-fabric-ui 7 · services.yaml / guides / other 45 · webrtc-models 5 |
| parser false positives (not citations), struck | 6 (`hatest/hatest.go:349` `:1397-1412` belongs to `core.py`; `ha/camera.go:136` `:826-827` to `camera/__init__.py`; `hatest/camera.go:216` "): 401"; `roadmap.md:261` and `by_design.md:166` are `.xml`/`.cpp` lines; `command.go:410-411` are go-hamapping's, not core's) |
| range checked against the construct | 417 (all outside the reports) + 128 report tokens (range and construct for core, matter-hub and go-fabric; the reports' self-citations of this repository see Not read) |
| read in full | 74: every matter-hub citation in `docs/by_design.md` and `docs/roadmap.md` (29), every Supervisor citation (9) and the Supervisor's option validation for HR-P1-cfg-21, every core citation that is the reason for behaviour (call_service, the registry events and their removal/restore, label rename, expose list admin, subscribe allow list, camera WebRTC and proxy, climate range refusal, lock default code, camera features: 31), the doubtful ones (5) |

Result: **every Home Assistant core, matter-hub and Supervisor citation
holds at its pin** except the shifts of HR-P3-cite-65 and the misstatement
of HR-P3-cite-63. The defects are concentrated in the go-fabric,
go-fabric-ui and go-hamapping citations, which name tags several bumps old
and in most cases no longer point at the construct (HR-P3-cite-60..62).

## Findings

### HR-P3-cite-60 — low — go-fabric citations at older tags; 15 of 26 no longer hold at v0.26.0

- **Where:** the sites in the table. **Reference:** `gf:` v0.26.0 as listed.
- **Claim (M):** 26 citations name a go-fabric tag before `go.mod`'s
  v0.26.0 (v0.11.0, v0.15.5, v0.15.6, v0.18.0, v0.19.1, v0.25.0); 8 more
  name no tag and are read at v0.26.0. The brief counts an old pin as
  `cite`. At v0.26.0, 15 of the pinned and 6 of the unpinned ranges point
  at other code; the facts themselves still hold at the new ranges.
- **Fix:** re-pin every one at v0.26.0 with the range in the last column
  (one scripted edit); unpinned ones get "at v0.26.0".
- **Guard:** the citation check of appendix A in `make ci`: every
  `gf:`/`ui:`/`hm:` citation names `go.mod`'s tag and its range exists;
  red at `0a47095` with exactly these rows.

| Site | Cited | At v0.26.0 |
| --- | --- | --- |
| `internal/ha/camera.go:83` | `secure/sessionparams/sessionparams.go:44` @v0.15.5 | holds, re-pin |
| `internal/ha/camera.go:85` | `cluster/camera/avstream.go:77-83` (no pin) | shifted → `:79-85` (`DefaultMaxSnapshotSize` at :85) |
| `internal/config/node.go:28` | `node/config.go:958-966` @v0.25.0 | holds (`checkPBKDF` :956-965), re-pin |
| `internal/config/node.go:43-44` | `docs/guides/camera.md:121-128`, `bridge/tcp_listen.go:20-31` @v0.15.5 | guide wrong → `camera.md:133-140`; `tcp_listen.go` holds (`:18-31`) |
| `internal/config/config.go:63` | `fabrictest/device.go:35` @v0.25.0 | shifted → `:36` |
| `internal/config/config.go:142` | `docs/guides/camera.md:414-425` @v0.15.5 | wrong → `:577-599` ("The ecosystem gate") |
| `internal/config/config.go:337-338` | `cluster/core/basic_information.go:853` @v0.11.0 | wrong → `:649-651` (NodeLabel `MaxBytes: 32`) |
| `internal/camera/camera.go:15-16` | `docs/guides/camera.md:84-87` @v0.15.5 | wrong → `:93-110` ("The camera's signals") |
| `internal/camera/camera.go:22-23` | `docs/guides/camera.md:122-127` @v0.15.6 | wrong → `:133-140` |
| `internal/camera/camera.go:52` | `cluster/camera/provider.go:587-588` @v0.15.5 | wrong → `:680-695` (`maxEndReason` :682, the check :693) |
| `internal/camera/camera.go:71` | `im/status.go:94` @v0.15.5 | holds, re-pin |
| `internal/camera/camera.go:102` | `cluster/spec/webrtctransportprovider/definition_gen.go:696` @v0.25.0 | shifted → `:721` |
| `internal/camera/camera.go:103` | `cluster/camera/provider.go:603-617` (no pin) | wrong → `:648-665` (`provideIceCandidates`, no count bound) |
| `internal/camera/camera.go:187` | `cluster/camera/provider_peer.go:84-120` @v0.15.5 | wrong → `:104-148` (`Offer`, `Answer`, `IceCandidates`, `End`) |
| `internal/camera/camera.go:202` | `cluster/camera/provider.go:522-526` @v0.15.5 | wrong → `:597-601` (port returns :597, `upsertSession` :601) |
| `internal/camera/camera.go:348` | `node/webrtc_provider_peers.go:43-56` @v0.15.5 | shifted → `:30-51` (`attachCameraPeers`) |
| `internal/camera/camera.go:422` | `cluster/camera/chime.go:291-299` @v0.15.6 | wrong → `:302-311` (`portError`, `spec.CarriesStatus`) |
| `internal/camera/camera.go:439-440` | `cluster/camera/provider.go:125-131`, `state.go:15-26` @v0.25.0 | shifted → `provider.go:159-165`, `cluster/camera/state.go:13-28` |
| `internal/arch/arch_test.go:54-57` | `docs/guides/camera.md:84-87` @v0.15.6, `:122-127` | wrong → `:93-110`, `:133-140` |
| `internal/adapt/command.go:293-294` | `cluster/spec/keypadinput definition_gen.go:95-98` (no pin) | wrong → `:91-94` (Power 64, VolumeUp 65, VolumeDown 66, Mute 67) |
| `internal/bridge/bridge_r6_e2e_test.go:23` | `device/camera_gen.go:20-24` @v0.15.5 | holds, re-pin |
| `internal/bridge/bridge_e2e_test.go:89-90` | `cluster/camera/requestor_parity_matterjs_test.go:465-480` @v0.15.5 | holds, re-pin |
| `internal/bridge/bridge_e2e_test.go:103-104` | `fabrictest/device.go:314-316` @v0.15.5, `:297-299` | wrong → `:351-356` (`programConfig` clears TCP), `:335-337` (`Configure` after the defaults) |
| `internal/bridge/bridge.go:15` | `docs/guides/embed-in-a-daemon.md` @v0.11.0 (no lines) | re-pin |
| `cmd/ha2fabric/stop_test.go:35`, `nodes.go:281` | `node/factory_reset.go:51-55` @v0.25.0 | holds (file identical), re-pin |
| `cmd/ha2fabric/nodes.go:37-38` | `node/node.go:39-41` @v0.18.0 | wrong → `:42-46` (`ErrStopped`) |
| `cmd/ha2fabric/nodes.go:664` | `node/node_label.go:39` @v0.19.1 | holds, re-pin |
| `docs/operations.md:114` | `docs/guides/operations.md:54, 58` @v0.15.5 | wrong → `:72, :76` |
| `docs/operations.md:377`, `docs/by_design.md:137` | `docs/guides/camera.md:414-425` @v0.15.5 | wrong → `:577-599` |
| `docs/operations.md:399` | `docs/guides/camera.md:121-128` (no pin) | wrong → `:133-140` |
| `docs/by_design.md:163`, `docs/roadmap.md:308` | `cluster/camera/chime.go:291-299` (no pin; "since v0.15.6") | wrong → `:302-311` |

Unpinned go-fabric citations that hold at v0.26.0 (M):
`cluster/spec/levelcontrol/values.go:14-44`, `:21,41-44`, `:34-39`
(`by_design.md:120`, `command.go:234`, `bridge_r5_e2e_test.go:104`);
`cluster/camera/provider_peer.go:51-63` (`roadmap.md:306`);
`device/camera_gen.go:34-45` (`roadmap.md:238`); the R7 report's
`node/aggregator.go:523-548, :619-654, :944-997, :1004-1039`,
`node/reserve.go:412-462, :464-483, :491`, `cluster/camera/provider_gone.go:33-40`.

### HR-P3-cite-61 — low — go-fabric-ui citations at v0.7.1 and v0.8.0 point at a route table

- **Where:** `cmd/ha2fabric/nodes.go:766`, `cmd/ha2fabric/main.go:232`
  (`fabricui/handler.go:401` @v0.7.1), `cmd/ha2fabric/main.go:266`
  (`fabricui/handler.go:393` @v0.8.0), `cmd/ha2fabric/nodes.go:602`
  (`fabricui/registry.go:163-177` @v0.7.1), `internal/config/config.go:469`
  (`fabricui/host.go:101-121` @v0.14.0), `cmd/ha2fabric/nodes.go:432`
  (`fabricui/operations.go:25-26` @v0.14.0).
- **Claim (M):** at `ui:` v0.14.1, `handler.go:401` is the closing brace of
  a route map and `:393` a `DELETE /{node}` route entry; the fact the
  comments state — a registry node's handler takes the default's warnings
  and counters — is `handler.go:601-610` (`sibling` copies `warnings`
  :609, `counters` :610). `registry.go:163-177` is the cache sweep; the
  handler kept across an erase is `managedHandler`, `registry.go:172-199`.
  `host.go` and `operations.go` are identical in v0.14.0 and v0.14.1:
  holds, re-pin.
- **Fix:** re-pin at v0.14.1 with those ranges. **Guard:** as cite-60.

### HR-P3-cite-62 — low — go-hamapping citations at v0.4.3 and v0.6.0 no longer hold at v0.8.0

- **Where:** `internal/plan/input.go:27`, `internal/camera/camera.go:278`
  (`plan.go:120-128` @v0.4.3); `internal/adapt/command.go:408-410`
  (`rows_gen.go:2265` @v0.6.0, `:2248-2249`).
- **Claim (M):** at `hm:` v0.8.0, `plan.go:120-128` is a struct's fields
  and the `Discard` doc; `Options.Ports` is `plan.go:160-168`.
  `rows_gen.go:2265` and `:2248-2249` are a fan row's `watch`; the
  `ColorTemperatureMireds` "reciprocal" with scale 1e6 and the bounds
  1..65279 are `rows_gen.go:2365` (and the second light row's `:2816`).
  `value.go:277-280` (no pin) holds.
- **Fix:** re-pin at v0.8.0 with those ranges. **Guard:** as cite-60.

### HR-P3-cite-63 — low — `docs/by_design.md` misstates what gates matter-hub's composition

- **Where:** `docs/by_design.md:98-101` (Composition).
- **Claim (M):** the page says matter-hub pulls battery, humidity,
  pressure, power and energy onto a device's main entity "behind
  `autoComposedDevices`" (`hub:services/bridges/bridge-registry.ts:1032-1105`).
  At `664575a` only the battery is opt-in (`autoBatteryMapping` or
  `autoComposedDevices`, `:308-312`); humidity and pressure are on by
  default (`autoHumidityMapping !== false`, `:345-350`;
  `autoPressureMapping !== false`, `:446-451`); power and energy of a
  `switch` or `light` are assigned with no flag at all (`:1068-1094`);
  `autoComposedDevices` decides whether the result is a real composed
  device rather than extra clusters on one endpoint (`:314-325`). The
  conclusion ("go-ha2fabric composes nothing") stands, but the reference
  fact behind the comparison is wrong: matter-hub composes by default.
- **Fix:** "matter-hub composes devices: user-defined composed entities
  (`…entity-mapping-storage.ts:227-229`) and, by default, humidity and
  pressure onto a temperature sensor and power and energy onto a switch
  or light, the battery behind `autoBatteryMapping`
  (`services/bridges/bridge-registry.ts:308-312, 345-350, 446-451,
  1040-1106`); `autoComposedDevices` makes them composed devices
  (`:314-325`)." Opus writes it from this note; Fable reads it once.
- **Guard:** none mechanical (a reference statement); the verifier's read.

### HR-P3-cite-64 — low — `CONCEPT.md`'s "what the adapter knows of Matter today" points at other code

- **Where:** `CONCEPT.md:104-107`.
- **Claim (M):** "What it knows of Matter today (`command.go:15-16`,
  `:208-209`, `:276-284`)". At `0a47095`, `internal/adapt/command.go:15-16`
  are the go-ha-catalog and go-hamapping imports, `:208-209` the vacuum's
  run mode, `:276-284` the water heater's mode. The go-fabric knowledge is
  `command.go:18-19` (the `keypadinput` and `levelcontrol` imports),
  `:236-237` (`levelcontroldef.LevelMax`) and `:304-312` (the CEC key
  codes). The same paragraph's `adapt.go:11-15` holds; the pre-A1
  citations of `CONCEPT.md:126-135` hold at `34075a6` (sampled: `bridge.go:688-880`,
  `:883-903`, `main.go:97`, `:219-226`, `provider.go:196-213`, `:255-259`,
  `node.go:39`, `nodes.go:163`).
- **Fix:** the three ranges above, "at `0a47095`" or symbol names
  (`levelcontroldef.LevelMax`, the `keypadinputdef` switch), which do not
  drift. **Guard:** the check of appendix A for self-citations of
  `internal/` paths at HEAD.

### HR-P3-cite-65 — note — core citations off by a line or naming the weaker construct

All at `core:` 2026.10.3 (M); every fact holds, the ranges are narrow:

| Site | Cited | Better |
| --- | --- | --- |
| `internal/ha/message.go:201`, `docs/reports/H2.md:36` | `core.py:129-142` (state_changed data: entity_id, old_state, new_state) | `:130-143` (`old_state` is :143) |
| `internal/ha/message.go:385` | `websocket_api/commands.py:265-274` | `:266-275` (as `client.go:727`'s `:266-295`) |
| `testdata/ha/get_states.json:2` | `camera/__init__.py:380-390 (state idle)` | add `:561-567` (`state` → `CameraState.IDLE`) |
| `docs/operations.md:180`, `docs/reports/H2.md:230` | `exposed_entities.py:414-418` (an administrator's command) | `:417-423` (`require_admin` :418) |
| `docs/roadmap.md:151` | `climate/__init__.py:470-488` (set_fan_mode refused unless in `fan_modes`) | add `:511` (the call with `self.fan_modes`) |
| `cmd/ha2fabric/supervisor.go:30-32` | "the header constant is in `supervisor/api/ingress.py` from that tag on" | the constant is defined in `sup:supervisor/const.py` (`:113` at tag 2026.10.4, `:115` at fe91b8db) and used by `api/ingress.py`; 2026.10.3 lacks it, 2026.10.4 has it, fe91b8db is 79 commits ahead of 2026.10.4 (M, `gh api compare`) |

Fix: the edits above. No guard (a range that exists but is a line short
is not mechanically checkable).

### HR-P3-cite-66 — note — `docs/roadmap.md`'s survey citations are pinned only in "Sources" and do not hold at go.mod's tags

- **Where:** `docs/roadmap.md:35-36, 54, 84, 124-125, 222, 259, 266-269, 358`.
- **Claim (M):** the roadmap's go-hamapping citations are the survey's
  (its "Sources" pin go-hamapping v0.2.1, `roadmap.md:371`), but the
  lines carry no pin, so a reader takes `go.mod`'s. At `hm:` v0.8.0:
  `rows/sensor.yaml:221-224` (unmappable "in H1") is a mappable VOC row —
  the unmappable verdicts are `:587-590`; `rows_gen.go:419, 851, 1879,
  1885` (explicit unmapped rows) are unrelated code; `rows/climate.yaml:11-25`
  ("the thermostat row") is the `air_conditioner` row — `thermostat` is
  `:159-180`; `rows/binary_sensor.yaml:130` (the unmapped classes) is a
  ContactSensor's `verdict: mappable` — the unmappable row is `:142`;
  `rows/cover.yaml:20-49` is the row's `cites:` — the row is `:9-143`. The
  go-fabric `cluster/camera/avstream.go:708-731` (AllocateVideoStream
  checks, `roadmap.md:259`) is the ref-count code at v0.26.0. Items marked
  `[x]` are history; the open ones (R3 climate, the binary_sensor
  fallback, cover details) are read as current.
- **Fix:** an inline pin on each survey citation ("at go-hamapping
  v0.2.1"), and the v0.8.0 ranges above on the open items.

### HR-P3-cite-67 — note — pins stated in the docs against `go.mod`

`2026.10.3`, `664575a`, `fe91b8db` are stated alike everywhere (M:
`CLAUDE.md:53-57`, `docs/by_design.md:6`, `docs/roadmap.md:376`,
`docs/reports/H2.md:8`, `H3.md:7-9`, every `_mirrors` line);
`docs/operations.md:7, :37` name go-fabric v0.26.0 = `go.mod:10`. Three
drifts (M):

- `CLAUDE.md:9` (State): the list ends "brief R7 — go-fabric v0.26.0,
  go-fabric-ui v0.14.1, go-hamapping v0.7.1"; `go.mod:8` is
  `sukramj.dev/go-hamapping v0.8.0` (with `go-hamqtt v0.41.0`,
  `go-ha-catalog v0.4.0`, #80, `changelog.md:12`).
- `THIRD-PARTY-NOTICES.md:61-63`: go-ha-catalog "at `d9582e02` (snapshot
  2026.10.3; `v0.3.2-0.20261024093528-d9582e022b8f`, the version
  go-hamapping pins)"; `go.mod:7` is `sukramj.dev/go-ha-catalog v0.4.0`
  (overlaps unit B's notices check).
- `internal/config/config.go:70` cites developers.home-assistant.io
  `presentation.md` "Ingress" without the pin `internal/web/web.go:93-95`
  gives it (`50b5188`; the fact holds there, `presentation.md:138`, read
  with `gh api` at that ref). The plan's pin table
  (`notes/review/2026-11-23/README.md:20-32`) is phase 0's by design; its
  "Supervisor … `cmd/ha2fabric/main.go:205-209`" is `:332-333` today.

Fix: one line in `CLAUDE.md` (go-hamapping v0.8.0 and the vanity paths),
the notices' catalog entry, the pin in `config.go:70`.

### HR-P3-cfg-68 — note — carried HR-P1-cfg-21 (b): the Supervisor's option schema read at `fe91b8db`

- **Reference (M):** the Supervisor validates the options at every start
  (`sup:supervisor/apps/app.py:1449-1450` → `write_options` `:980-993`)
  against the merged defaults and user values (`:512-516`), with
  `AppOptions` (`sup:supervisor/apps/options.py:89-110`); unknown keys are
  dropped with a warning (`:95-102`), a missing non-optional key refuses
  (`:291-308`). The types (`:31-45`, `:150-212`): `str` and `str(min,max)`
  are `vol.All(str, vol.Length(…))` — characters, not bytes, and no bound
  without the parenthesis (`:167-170`); `int(a,b)` is `Coerce(int)` and
  `Range` (`:171-172`; `Coerce` truncates a float); `bool` is
  `vol.Boolean()` (`:175-176`); `port` 1..65535 (`sup:supervisor/validate.py:68`);
  `match(re)` is `vol.Match` on `str(value)` (`:183-184`, `re.match`,
  `.` excludes newline); `list(a|b)` is `vol.In` (`:185-186`). A failing
  value refuses the start in the Supervisor with the message in its log
  and the options form.
- **Bound per option (M; `addon/config.yaml:72-91`, `script/run.sh`,
  `internal/config/config.go`):**

| Option | Supervisor bounds | Daemon bounds | Tighter |
| --- | --- | --- | --- |
| `port` | 1..65535 | `hostPort` 1..65535 (`:442-452`) | equal |
| `passcode` | `int(1,99999998)?` | `ValidateSetupPIN` refuses trivial values (`:318-326`) | daemon |
| `discriminator` | 0..4095 | 0..`node.MaxDiscriminator` (`:327-331`) | equal |
| `vendor_id`, `product_id` | 1..65535 | 1..0xFFFF (`id16`, `:433-440`) | equal |
| `label` | `str`: any length, any characters | trimmed, not empty, ≤ 32 bytes, `text.Safe` (`:284`, `:334-342`) | daemon |
| `expose_assist`, `camera` | `bool` | `ParseBool` (`:351-365`) | equal (JSON true/false) |
| `include`, `exclude`, `*_labels` | list of unbounded `str` | split on newline and comma, `Rule.Validate` (`:292`, `:348-350`) | daemon (a comma in a label name splits it: cfg-21 (a), documented) |
| `log_level` | `list(debug\|info\|warn\|error)` | `slog.Level.UnmarshalText` (`:371-373`) | Supervisor |
| `node_limit` | 0..32 | 0..`MaxNodeLimit` 32 (`:366-370`) | equal |
| `mdns_interfaces` | list of `str` (`""` allowed) | no empty or repeated name (`:494-511`) | daemon |

- **Claim:** the manifest never admits what the daemon would serve
  wrongly; the daemon's start check stays the bound for `label` (bytes),
  `passcode` (trivial values) and empty interface names, as phase 1
  accepted. What phase 1 could not state is now read: the schema can
  carry `str(1,32)` (`options.py:38`, `:167-170`) — simpler than the
  proposed `match(^.{1,32}$)` and with the same character count — which
  moves a too-long label's refusal into the options form for every
  ASCII label; a multibyte label of ≤ 32 characters and > 32 bytes still
  reaches the daemon's check. Side observation (M): the defaults merge
  (`app.py:512-516`) means an installation that never saved `node_limit`
  or `mdns_interfaces` still gets the manifest's value, so
  `run.sh:60-71`'s "from before 0.6.0" guards do not fire under this
  Supervisor; harmless.
- **Fix:** `label: str(1,32)` in `addon/config.yaml`; keep the daemon's
  check. Optional: `mdns_interfaces: - str(1,)`. **Test:**
  `TestImageDefaults`-style manifest test that the schema's `label` names
  a length bound ≤ 32.

## Counts

| high | medium | low | note |
| --- | --- | --- | --- |
| 0 | 0 | 5 (cite-60, -61, -62, -63, -64) | 4 (cite-65, -66, -67, cfg-68) |

No `upstream` candidate: every go-fabric, go-fabric-ui and go-hamapping
fact still holds at the new pin; only this repository's ranges drifted.

## Not read, and why

- **connectedhomeip `1dc3b69ed5e`, matter.js, `CameraAVStreamManagement.xml`**
  (`internal/camera/camera.go:52`, `docs/by_design.md:166`,
  `docs/roadmap.md:259-262, 284`, `internal/config/node.go:25`): not a
  pinned source of the brief.
- **webrtc-models 0.3.0** (`internal/ha/camera.go:41-44`,
  `hatest/camera.go:62-63`, `internal/camera/camera.go:714`): the pin
  holds (`core:requirements.txt:61`, `homeassistant/package_constraints.txt:76`
  `webrtc-models==0.3.0`), the library's source is not a pinned source.
- **go-ha-catalog data** (`data/mqtt.json`, `data/sensor.json` in rows and
  the roadmap): not a pinned source; outside the unit's list.
- **Self-citations in `docs/reports/R7.md`, `A1.md`, `E1.md`** (this
  repository at the wave's commit, 16 tokens) and the go-fabric/go-fabric-ui
  citations of `E1.md` at v0.7.1/v0.18.0: historical records of their wave
  that no current doc cites by line; their core/matter-hub/go-fabric
  v0.26.0 citations were checked (all hold).
- **`docs/roadmap.md`'s go-fabric `CHANGELOG.md:148-176, :174` and
  `docs/guides/camera.md:38-58`, `advanced-seams.md:37`**: survey history
  of done items (R2, R6 blocker); not checked line by line (covered by
  cite-66's pin fix).
- **developers.home-assistant.io** citations other than `presentation.md`
  at `50b5188` (`docs/operations.md:126, 176`, `config.go:32`, H2's
  `configuration.md:154`, `:195`): URLs to the live site, no pin; not a
  pinned source.

## Probes

| Probe | Result |
| --- | --- |
| A: `cites.py` over `git archive 0a47095` (appendix A) | ran; 545 tokens, 417 + 128; the rows of cite-60..62, -64, -66 are its red output |
| B: Supervisor option validation read at `fe91b8db` (appendix B) | read; cfg-68 |
| `go test` | not needed: no behaviour claim of this unit depends on running code |

## Questions for the owner

1. **Pin policy for go-fabric, go-fabric-ui and go-hamapping citations.**
   Recommendation: every such citation names `go.mod`'s tag, and the
   citation check (appendix A, made a `script/` tool in `make ci`) fails a
   citation whose tag differs from `go.mod`'s or whose range does not
   exist; each dependency bump re-pins them in its own pull request (as
   `no-deprecated` lifts do). The alternative — keep the tag a fact was
   read at — leaves 21 ranges pointing at other code today.
2. **cfg-68: `label: str(1,32)` in the manifest?** Recommendation: yes;
   it is the form's early refusal for the common case at no cost; the
   daemon's byte check stays the bound.
3. **Roadmap survey citations (cite-66):** inline pin at the survey's
   v0.2.1 (history kept as read) or re-cite at v0.8.0? Recommendation:
   inline pin for `[x]` items, v0.8.0 ranges for the open ones.

## Appendix A — the citation probe

Run: `git -C /home/markus/Dokumente/GitHub/go-ha2fabric-review archive 0a47095 | tar -x -C <scratch>/tree`;
`git -C …/core ls-tree -r --name-only 2026.10.3 > <scratch>/core-files.txt`;
`git -C …/home-assistant-matter-hub ls-tree -r --name-only 664575a > <scratch>/hub-files.txt`;
`CAP=12 python3 -I cites.py <scratch>/tree <scratch>` → `cites.tsv`
(site, line, source, path, range, pin, resolved, in-range/out-of-range/missing)
and `dump.txt` (the cited lines). Its "missing", "ambiguous" and
"unchecked-kind" rows were resolved by hand above; its in-range rows
were read for the construct. As a guard it needs one addition: compare
the pin of each `gf:`/`ui:`/`hm:` token with `go.mod`.

```python
#!/usr/bin/env python3
"""Extract file:line citations from the baseline tree and resolve them at their pins.

Usage: cites.py <tree> <outdir>
Writes <outdir>/cites.tsv (one row per citation) and <outdir>/dump.txt (cited lines).
"""
import os, re, subprocess, sys, functools

TREE, OUT = sys.argv[1], sys.argv[2]
CORE = "/home/markus/Dokumente/GitHub/core"
HUB = "/home/markus/Dokumente/GitHub/home-assistant-matter-hub"
MOD = "/home/markus/go/pkg/mod"
GF = MOD + "/thingfabric.dev/go-fabric@{}"
UI = MOD + "/thingfabric.dev/go-fabric-ui@{}"
HM = MOD + "/sukramj.dev/go-hamapping@{}"

core_files = set(open(OUT + "/core-files.txt").read().split())
hub_files = set(open(OUT + "/hub-files.txt").read().split())

@functools.lru_cache(None)
def git_show(repo, ref, path):
    r = subprocess.run(["git", "-C", repo, "show", f"{ref}:{path}"], capture_output=True, text=True)
    return r.stdout.split("\n") if r.returncode == 0 else None

@functools.lru_cache(None)
def read(path):
    try:
        return open(path).read().split("\n")
    except OSError:
        return None

TOK = re.compile(r"((?:[A-Za-z0-9_.\-]+/)*[A-Za-z0-9_.\-]+\.(?:py|ts|go|yaml|yml|md|json|txt|js))?:\s*((?:\d+(?:-\d+)?)(?:\s*,\s*(?:\d+(?:-\d+)?)(?![\d.]))*)")
PIN = re.compile(r"(2026\.10\.3|664575a|fe91b8db|v\d+\.\d+\.\d+|0\.3\.0)")

def blocks(path):
    """Yield (start_line, [(lineno, text)]) comment/paragraph blocks."""
    lines = open(path, errors="replace").read().split("\n")
    go = path.endswith(".go")
    if path.endswith(".json"):
        for i, l in enumerate(lines, 1):
            if "_mirrors" in l: yield [(i, l)]
        return
    cur = []
    for i, l in enumerate(lines, 1):
        if go:
            m = re.match(r"\s*//\s?(.*)", l)
            if m:
                cur.append((i, m.group(1)))
                continue
            if "//" in l:
                if cur: yield cur; cur = []
                yield [(i, l.split("//", 1)[1])]
                continue
            if cur: yield cur; cur = []
        else:
            if l.strip() == "" or l.startswith("|") or l.startswith("#"):
                if cur: yield cur; cur = []
                if l.startswith("|"):
                    yield [(i, l)]
                continue
            cur.append((i, l))
    if cur: yield cur

def ranges(s):
    out = []
    for part in re.split(r"\s*,\s*", s):
        if "-" in part:
            a, b = part.split("-"); out.append((int(a), int(b)))
        else:
            out.append((int(part), int(part)))
    return out

def classify(path, ctx, pin):
    if path.endswith(".py"):
        if "supervisor" in ctx.lower() or pin == "fe91b8db": return "sup"
        if path.startswith("webrtc_models"): return "ext"
        return "core"
    if path.endswith(".ts"): return "hub"
    if path.endswith(".go"):
        if "go-hamapping" in ctx: return "hm"
        if "fabricui/" in path or "go-fabric-ui" in ctx: return "ui"
        return "gf"
    if path.endswith((".yaml", ".yml")) and path.startswith("rows/"): return "hm"
    return "other"

def core_resolve(p):
    cands = [p, "homeassistant/" + p, "homeassistant/components/" + p, "homeassistant/helpers/" + p]
    hits = [c for c in cands if c in core_files]
    if not hits:
        hits = [f for f in core_files if f.endswith("/" + p)]
    return hits

def hub_resolve(p):
    cands = [p, "packages/" + p, "packages/backend/src/" + p]
    hits = [c for c in cands if c in hub_files]
    if not hits:
        hits = [f for f in hub_files if f.endswith("/" + p)]
    return hits

def mod_resolve(root, p):
    if not os.path.isdir(root): return None, []
    full = os.path.join(root, p)
    if os.path.isfile(full): return full, [p]
    hits = []
    for d, _, fs in os.walk(root):
        for f in fs:
            rel = os.path.relpath(os.path.join(d, f), root)
            if rel.endswith("/" + p): hits.append(rel)
    return (os.path.join(root, hits[0]) if len(hits) == 1 else None), hits

rows = []
dump = open(OUT + "/dump.txt", "w")
for d, _, fs in os.walk(TREE):
    if "/notes" in d[len(TREE):] or "/.git" in d: continue
    for f in fs:
        if f == "changelog.md" or not f.endswith((".go", ".md", ".json", ".yaml", ".yml", ".sh")): continue
        path = os.path.join(d, f); rel = os.path.relpath(path, TREE)
        for blk in blocks(path):
            text = " ".join(t for _, t in blk)
            offs = []; pos = 0
            for ln, t in blk:
                offs.append((pos, ln)); pos += len(t) + 1
            last = None
            for m in TOK.finditer(text):
                p, rs = m.group(1), m.group(2)
                if p is None:
                    # bare continuation ":123" — only after a path in this block
                    if last is None: continue
                    p = last
                    if not re.match(r"\s*\d", rs): continue
                else:
                    last = p
                ln = max(l for o, l in offs if o <= m.start())
                after = text[m.end():m.end() + 260]
                pm = PIN.search(after)
                pin = pm.group(1) if pm else ""
                ctx = text[max(0, m.start() - 200):m.end() + 200]
                src = classify(p, ctx, pin)
                rows.append([rel, ln, src, p, rs, pin, ctx])

with open(OUT + "/cites.tsv", "w") as tsv:
    for n, (rel, ln, src, p, rs, pin, ctx) in enumerate(rows, 1):
        lines = None; resolved = ""; note = ""
        if src == "core":
            hits = core_resolve(p)
            if len(hits) == 1:
                resolved = hits[0]; lines = git_show(CORE, "2026.10.3", hits[0])
            else:
                note = f"ambiguous {hits[:4]}" if hits else "missing"
        elif src == "hub":
            hits = hub_resolve(p)
            if len(hits) == 1:
                resolved = hits[0]; lines = git_show(HUB, "664575a", hits[0])
            else:
                note = f"ambiguous {hits[:4]}" if hits else "missing"
        elif src in ("gf", "ui", "hm"):
            base = {"gf": GF, "ui": UI, "hm": HM}[src]
            cur = {"gf": "v0.26.0", "ui": "v0.14.1", "hm": "v0.8.0"}[src]
            ver = pin if pin.startswith("v") else cur
            full, hits = mod_resolve(base.format(ver), p)
            if full is None and not os.path.isdir(base.format(ver)):
                note = f"version {ver} not in cache"
                full, hits = mod_resolve(base.format(cur), p); ver = cur + "(fallback)"
            if full:
                resolved = f"{src}@{ver}:{hits[0]}"; lines = read(full)
            else:
                note = (note + f" ambiguous {hits[:4]}") if hits else (note + " missing")
        else:
            note = "unchecked-kind"
        status = ""
        if lines is not None:
            n_lines = len(lines) - (1 if lines and lines[-1] == "" else 0)
            bad = [f"{a}-{b}" for a, b in ranges(rs) if b > n_lines or a > b]
            status = "OUT-OF-RANGE " + ",".join(bad) if bad else "in-range"
            dump.write(f"\n===== #{n} {rel}:{ln} -> {resolved}:{rs} pin={pin} [{status}] len={n_lines}\n")
            dump.write("CTX: " + ctx.replace("\n", " ") + "\n")
            for a, b in ranges(rs):
                for i in range(max(1, a - 1), min(n_lines, b + 1, a + int(os.environ.get("CAP","45"))) + 1):
                    dump.write(f"{i:5d}| {lines[i-1]}\n")
                dump.write("  ---\n")
        tsv.write("\t".join(map(str, [n, rel, ln, src, p, rs, pin, resolved, status or note])) + "\n")
print(len(rows), "citations")
```

## Appendix B — the Supervisor reads for cfg-68

```sh
for p in supervisor/apps/options.py supervisor/apps/app.py supervisor/validate.py \
         supervisor/api/ingress.py supervisor/const.py \
         supervisor/api/middleware/security.py supervisor/api/root.py; do
  gh api "repos/home-assistant/supervisor/contents/$p?ref=fe91b8db" \
    -H 'Accept: application/vnd.github.raw' > "sup/$(echo $p | tr / _)"
done
gh api repos/home-assistant/supervisor/compare/2026.10.4...fe91b8db --jq '.status,.ahead_by'   # ahead 79
gh api "repos/home-assistant/supervisor/contents/supervisor/const.py?ref=2026.10.4" \
  -H 'Accept: application/vnd.github.raw' | grep -n REMOTE_USER_GROUPS             # 113
```

Supervisor citations checked there (all hold, M): `web.go:160-161`
(`api/ingress.py:412-419`, `const.py:115`, `:625`, `:653-659`),
`supervisor.go:23-26` (`api/middleware/security.py:107-116` `/info` in
`api_bypass`, `:447-451` the bypass for an app; `api/root.py:40-44`
`ATTR_SUPERVISOR`), `main.go:332-333` (`api/ingress.py:400-427`
`_init_header` copies every header but `_REQUEST_HEADERS_FILTER`, which
holds no `Host`; core `hassio/ingress.py:268-272` likewise with
`INIT_HEADERS_FILTER`, `:28-37`).
