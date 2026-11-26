# Map: citation-bearing lines at 0a47095 (script, not a citation)

Script: `grep` over the tree outside `notes/`, `held/`, changelogs (session scratchpad `cites.sh`). 253 lines.

```
AI_POLICY.md:5:[Open Home Foundation AI policy](https://developers.home-assistant.io/docs/ai_policy) and adapted
CLA.md:42:   matter.js or connectedhomeip behaviour names the upstream file and
CLAUDE.md:11:page, `docs/by_design.md` the differences from matter-hub,
CLAUDE.md:53:Home Assistant facts come from `home-assistant/core` at a pinned tag
CLAUDE.md:54:(2026.10.3), read from its raw files and cited `file:line`; never run.
CLAUDE.md:56:[home-assistant-matter-hub](https://github.com/RiDDiX/home-assistant-matter-hub)
CLAUDE.md:57:at a pinned commit (`664575a`), cited `file:line` in `docs/by_design.md` —
CLAUDE.md:105:  going ahead of matter-hub is allowed and recorded in `docs/by_design.md`,
CLAUDE.md:109:  target (matter-hub, for how a bridge behaves; never for Matter)? Better than
CLAUDE.md:165:statement about Home Assistant or matter-hub that lands in the repo is cited
CLAUDE.md:173:| **Planning and decisions:** the wave plan and its brief; the decisions put to the owner, with the architecture reasoning ([The architecture standard](#the-architecture-standard)); *which* guard gets built and its bite line; a question to go-hamapping or go-fabric; the verdict where `home-assistant/core`, matter-hub, go-hamapping and go-fabric contradict each other | **Fable**, as a targeted agent |
CLAUDE.md:174:| **Reading the reference and judging:** what `home-assistant/core` (2026.10.3) and matter-hub (`664575a`) do at the pin, and whether a difference is a deliberate divergence or a gap — for `docs/by_design.md`, `CONCEPT.md`, a report's decisions, every comment that cites `home-assistant/core` or matter-hub `file:line`, and the Home Assistant facts the translation in `internal/adapt` and the exposure rule in `internal/expose` rest on | **Fable** gives the verdict with the lines as a short fact note; **Opus** writes the entry or the comment in the file's form; **Fable** reads the finished text once against its note. Every reference statement comes from Fable |
CLAUDE.md:175:| **Implementation**, also on `internal/adapt`, `internal/expose` and the credential handling (`internal/config`, the token's path to the client); test ports; a dependency bump with its notices; a `changelog.md` entry, the wave report and docs; rebases and merge conflicts | **Opus** (also what fast mode runs on) — with the Fable read of the next row before it merges where it touches one of those paths or mirrors Home Assistant or matter-hub |
CLAUDE.md:176:| **Review** of a wave's pull request | **Opus** does the code review — correctness, concurrency, error paths, tests, the fixed report shape. **Fable** reads on top, bounded: behaviour against `home-assistant/core` 2026.10.3, matter-hub `664575a` and go-fabric at its tag, the paths of the row above, and every finding of high or medium severity for confirmation before it reaches an implementer |
CLAUDE.md:177:| **Citations** | **Opus** checks every range (the file, the lines, the construct the sentence names). **Fable** reads every citation that is the reason for behaviour ("as Home Assistant does", "as matter-hub"), every doubtful one and a sample of the rest |
CLAUDE.md:178:| Work a test or formatter checks immediately: `gofumpt`, regenerating the coverage floors, adding a test case to an existing pattern; search and first reading of a source tree (`home-assistant/core`, matter-hub, go-hamapping, this repo) whose result a stronger model or the owner reads again | **Sonnet**, including as the model of a delegated search subagent |
CLAUDE.md:180:Sonnet never writes a fact about Home Assistant or matter-hub into the
CLAUDE.md:194:- **Delegate first reads.** A source tree's inventory (`home-assistant/core`
CLAUDE.md:195:  at the pinned tag, matter-hub at the pinned commit, go-hamapping, this
CLAUDE.md:198:  load-bearing lines and verifies every Home Assistant or matter-hub fact
CONCEPT.md:13:[home-assistant-matter-hub](https://github.com/RiDDiX/home-assistant-matter-hub)
CONCEPT.md:14:plays on matter.js, as a static Go binary and a Home Assistant add-on.
CONCEPT.md:15:matter-hub is the helper reference for how a real Home Assistant bridge
CONCEPT.md:84:  `homeassistant/exposed_entities.py:25` at 2026.10.3); Assist's list is
CONCEPT.md:120:- **Better than matter-hub?** In its form and in the single place for a
CONCEPT.md:122:  one table that decides what a device becomes. Not everywhere: matter-hub
CONCEPT.md:157:- matterjs-server: the WebSocket client pattern Home Assistant already
CONCEPT.md:158:  uses for Matter ([repository](https://github.com/matter-js/matterjs-server)).
README.md:36:[`licenses/NOTICE-matter.js.txt`](https://github.com/thingfabric/go-fabric/blob/main/licenses/NOTICE-matter.js.txt):
README.md:200:port of [matter.js](https://github.com/matter-js/matter.js) (Apache-2.0),
THIRD-PARTY-NOTICES.md:22:  (matter.js, filippo.io/nistec, modernc.org/sqlite, …) travel in its
THIRD-PARTY-NOTICES.md:62:  (snapshot 2026.10.3; `v0.3.2-0.20261024093528-d9582e022b8f`, the
cmd/ha2fabric/main.go:332:// ingress.py:268-272 at 2026.10.3, Supervisor supervisor/api/ingress.py:
cmd/ha2fabric/main.go:333:// 400-427 at fe91b8db; notes/reads/read-2026-11-23-ingress-host-facts.md);
cmd/ha2fabric/supervisor.go:26:// api/root.py:40-44, at fe91b8db).
cmd/ha2fabric/supervisor.go:32:// supervisor/api/ingress.py from that tag on). Below it the add-on's web
cmd/ha2fabric/supervisor_test.go:24:		"2026.10.3": false, "2026.10.2": false, "2026.9.9": false, "2025.12.0": false,
cmd/ha2fabric/supervisor_test.go:48:		_, _ = fmt.Fprintf(w, `{"result":"ok","data":{"supervisor":%q,"homeassistant":"2026.10.3"}}`, version)
cmd/ha2fabric/supervisor_test.go:88:	old, _ := fakeSupervisor(t, "2026.10.3", 0)
cmd/ha2fabric/supervisor_test.go:91:	if g.open() || len(g.Warnings()) != 1 || g.Warnings()[0].Code != warningSupervisorTooOld || !strings.Contains(g.Warnings()[0].Message, "2026.10.3") {
cmd/ha2fabric/supervisor_test.go:108:	version.Store("2026.10.3")
cmd/ha2fabric/supervisor_test.go:151:	version.Store("2026.10.3")
docs/by_design.md:4:[home-assistant-matter-hub](https://github.com/RiDDiX/home-assistant-matter-hub),
docs/by_design.md:6:`file:line` at its commit `664575a`, paths under `packages/` (and
docs/by_design.md:7:`packages/backend/src/` for `services/` and `matter/`). matter-hub is never a
docs/by_design.md:12:matter-hub filters with matchers: sixteen types (pattern, regex, domain,
docs/by_design.md:27:matter-hub resolves a label value by name, case-insensitively, else
docs/by_design.md:33:(`helpers/label_registry.py:130-132` at 2026.10.3) only for a label created
docs/by_design.md:38:matter-hub polls the registries every 60 s by default
docs/by_design.md:49:matter-hub excludes a hidden entity unless the `includeHiddenEntities`
docs/by_design.md:57:matter-hub keeps an entity reachable while its state is `unknown`
docs/by_design.md:62:`homeassistant/const.py:303,313`; H2's decision): a controller shows a
docs/by_design.md:67:matter-hub keeps an endpoint 300 s after its entity leaves the registry
docs/by_design.md:79:`homeassistant/helpers/entity_registry.py:171-175`, `:1728-1775` at
docs/by_design.md:80:2026.10.3): the bridge calls go-fabric's `Forget` for its key on the next
docs/by_design.md:88:matter-hub names a bridged device by a custom name, then the registry
docs/by_design.md:98:matter-hub composes devices: user-defined composed entities
docs/by_design.md:108:matter-hub stores a bridge's filter with the bridge in its own storage and
docs/by_design.md:119:2026.10.3) is LevelControl's CurrentLevel 0 to 254 (go-fabric
docs/by_design.md:121:`UnitFromLevel`), as matter-hub maps it
docs/by_design.md:123:165-166`). One difference: matter-hub sends `level / 254` as it is;
docs/by_design.md:131:matter-hub exposes the cameras its camera plugin lists by entity id, a
docs/by_design.md:134:(`camera/const.py:71-75` at 2026.10.3) through go-hamapping's camera row,
docs/by_design.md:143:matter-hub runs two WebRTC peer connections per session, one to Home
docs/by_design.md:158:matter-hub accepts `SolicitOffer` and starts a session whose deferred offer
docs/by_design.md:171:matter-hub uses the controller's ICE servers on its own controller-side
docs/operations.md:126:([add-on presentation, "Ingress"](https://developers.home-assistant.io/docs/apps/presentation#ingress):
docs/operations.md:176:  ([add-on communication](https://developers.home-assistant.io/docs/apps/communication)).
docs/operations.md:180:  command (`homeassistant/exposed_entities.py:414-418` at 2026.10.3); the
docs/operations.md:211:| Command | When | Home Assistant core at 2026.10.3 |
docs/operations.md:215:| `config/entity_registry/list`, `config/device_registry/list`, `config/area_registry/list`, `config/label_registry/list`, `get_states`, `get_config`, `homeassistant/expose_entity/list` | each connection and each registry change, one read shared by every node on the connection; a sync that failed is retried after 5 s, doubling to 5 minutes | `config/entity_registry.py:39-64`, `config/device_registry.py:75-101`, `config/area_registry.py:23-37`, `config/label_registry.py:53-67`, `websocket_api/commands.py:369-385, 658-666`, `homeassistant/exposed_entities.py:414-442` |
docs/operations.md:363:carries none (`homeassistant/components/lock/__init__.py:113-133,
docs/operations.md:364:280-295` at 2026.10.3). Without a stored code such a call fails in Home
docs/operations.md:373:`STREAM` feature (`camera/const.py:71-75` at 2026.10.3) becomes a Matter
docs/reports/E1.md:90:  so an erased node numbers from 2 again? I have not checked matter.js's
docs/reports/E1.md:98:- matter-hub's factory reset of a bridge: unverified, to be read at
docs/reports/E1.md:99:  `664575a` first.
docs/reports/F2.md:18:- Home Assistant facts at 2026.10.3: remove event data
docs/reports/G1.md:37:- Add-on: `AllowHost: web.From(config.IngressProxy)`. Core 2026.10.3 and the
docs/reports/G1.md:112:  not matter-hub).
docs/reports/H2.md:8:`home-assistant/core` tag `2026.10.3`, read from raw files
docs/reports/H2.md:9:(`https://raw.githubusercontent.com/home-assistant/core/2026.10.3/<path>`),
docs/reports/H2.md:34:| `homeassistant/expose_entity/list` | Assist exposure (admin) | `components/homeassistant/exposed_entities.py:414-442` |
docs/reports/H2.md:131:(developers.home-assistant.io `docs/apps/configuration.md:154` at
docs/reports/H3.md:7:go-ha-catalog unchanged. Home Assistant facts: `home-assistant/core` tag
docs/reports/H3.md:8:`2026.10.3`, raw files, paths under `homeassistant/`. matter-hub facts:
docs/reports/H3.md:9:`home-assistant-matter-hub` at `664575a`.
docs/reports/H3.md:77:report records no real pairing (the owner's step), and matter-hub's code
docs/reports/H3.md:83:## 2.6 The matter-hub comparison — `5e7bf89`
docs/reports/H3.md:87:the selection lives — one paragraph each, cited at `664575a`. `CONCEPT.md`
docs/reports/H3.md:88:§Purpose and `CLAUDE.md` name matter-hub as the helper reference; the
docs/reports/H3.md:242:- **Q1 (owner).** Removal grace and number reservation. matter-hub closes
docs/reports/H3.md:272:  (matter-hub `services/bridges/matcher/matches-entity-filter.ts:255-271`
docs/reports/H3.md:274:- **Labels by subscription, not a 60 s poll** (matter-hub
docs/reports/H3.md:277:- **Hidden never served, no flag** (matter-hub
docs/reports/H3.md:279:- **Unknown is unreachable** (matter-hub reachable on unknown,
docs/reports/H3.md:281:- **Immediate removal, no grace** (matter-hub 300 s,
docs/reports/H3.md:283:- **No composition beyond go-hamapping's riders** (matter-hub
docs/reports/H3.md:286:  (matter-hub `common/src/home-assistant-filter.ts:1-33`,
docs/reports/H3.md:289:  (matter-hub stores the filter with the bridge,
docs/roadmap.md:96:  Done in go-hamapping v0.3.0 / go-ha2fabric 0.4.0, as matter-hub does
docs/roadmap.md:103:  and Google Home show a bridged GenericSwitch at all; matter-hub's
docs/roadmap.md:104:  behaviour at `664575a` is the first thing to read.
docs/roadmap.md:143:  sibling; matter.js's room-air-conditioner element
docs/roadmap.md:151:  `fan_modes` (`climate/__init__.py:470-488` at 2026.10.3); the
docs/roadmap.md:171:  (`rows/switch.yaml:6-8`). Owner decision 2026-10-31: follow matter-hub,
docs/roadmap.md:174:  34-68` at `664575a`). Documented in go-hamapping's `docs/by_design.md`
docs/roadmap.md:180:  280-295` at 2026.10.3); the daemon's calls carry none
docs/roadmap.md:184:  file. Matter-side PIN credentials (matter-hub stores the controller's
docs/roadmap.md:216:  behaviour exists in matter-hub at `664575a`.
docs/roadmap.md:239:- Home Assistant at 2026.10.3 offers `camera/webrtc/offer`,
docs/roadmap.md:241:  (`homeassistant/components/camera/webrtc.py:254, 335, 308`); the
docs/roadmap.md:343:  2026.10.3); `automation` joins `script` and `scene`, which leave "Not planned".
docs/roadmap.md:344:  Parity with matter-hub as go-hamapping's `docs/by_design.md` cites it
docs/roadmap.md:346:  `legacy/automation/index.ts:8-14` at `664575a`).
docs/roadmap.md:353:  2026-10-31). matter-hub may expose some as OnOff helpers; whether a
docs/roadmap.md:376:- Home Assistant `home-assistant/core` at tag 2026.10.3, read with
internal/adapt/adapt.go:16:// Assistant core line it reads (tag 2026.10.3) and the row that reads it.
internal/adapt/adapt.go:43:// (helpers/device_registry.py:589-629 at 2026.10.3). Its first identifier
internal/adapt/adapt.go:240:// unavailable and unknown (homeassistant/const.py:303,313 at 2026.10.3).
internal/adapt/adapt_test.go:411:// 2026.10.3); the controller's next write of the other bound follows.
internal/adapt/command.go:31:// Home Assistant core's at 2026.10.3, cited per entry from the domain's
internal/adapt/command.go:413:// cv.positive_int at helpers/config_validation.py:185, 2026.10.3); both
internal/bridge/bridge.go:384:		// (homeassistant/helpers/entity_registry.py:171-175, :1728-1775 at
internal/bridge/bridge.go:385:		// 2026.10.3): its endpoint number is forgotten, not kept. An
internal/bridge/bridge.go:410:// (homeassistant/helpers/entity_registry.py:1728-1775, :1624 at
internal/bridge/bridge.go:411:// 2026.10.3), so without Forget a re-added entity would get its old
internal/bridge/bridge_c1_e2e_test.go:71:// feature flags are Home Assistant's (cover/const.py:44-54 @2026.10.3).
internal/bridge/bridge_e2e_test.go:89:	// TCP only (go-fabric cluster/camera/requestor_parity_matterjs_test.go:
internal/bridge/bridge_e2e_test.go:206:	// (exposed_entities.py:304-312 at 2026.10.3); the include list adds it.
internal/bridge/bridge_p2r2_e2e_test.go:45:// cover/const.py:44-54 @2026.10.3) — and their commands reach Home
internal/bridge/bridge_r5_e2e_test.go:26:// does (core.py State._as_dict at 2026.10.3).
internal/bridge/command.go:21:// at 2026.10.3, blocking=True).
internal/bridge/queue_internal_test.go:172:// Assistant does (climate/services.py:121-131 at 2026.10.3).
internal/bridge/sync_internal_test.go:131:	b.Connected("2026.10.3")
internal/camera/camera.go:78:	// 2026.10.3, "offer" required).
internal/config/config.go:32:// add-on's environment (https://developers.home-assistant.io/docs/apps/
internal/config/config.go:70:	// (developers.home-assistant.io docs/apps/presentation.md "Ingress").
internal/config/node.go:25:// window each draw a fresh 32-byte salt when they open, as matter.js
internal/expose/expose.go:57:	// (homeassistant/helpers/entity_registry.py:137-151 at 2026.10.3).
internal/expose/expose.go:94:// created later matches by that id (D1; matter-hub slugifies an unknown
internal/ha/camera.go:19:// after "WebRTC" (camera/webrtc.py:40-53, 56-90 at 2026.10.3).
internal/ha/camera.go:28:// 56-90 at 2026.10.3): "session" with session_id, "answer" with the SDP
internal/ha/camera.go:42:// by requirements.txt at 2026.10.3; the aliases are the JSON keys,
internal/ha/camera.go:54:// 2026.10.3). Unsubscribing closes the session.
internal/ha/camera.go:61:// (camera/webrtc.py:333-348 at 2026.10.3).
internal/ha/camera.go:73:	// (camera/__init__.py:794-804 at 2026.10.3).
internal/ha/camera.go:76:	// (camera/__init__.py:806-808 at 2026.10.3).
internal/ha/camera.go:134:// camera/__init__.py:817-838 at 2026.10.3) with the access token as a
internal/ha/client.go:5:// (https://developers.home-assistant.io/docs/api/websocket) with its auth
internal/ha/client.go:10:// taken from Home Assistant core at the pinned release 2026.10.3; the
internal/ha/client.go:364:// (websocket_api/auth.py:25-46, 86-127 at 2026.10.3).
internal/ha/client.go:513:// before it sends the result (camera/webrtc.py:275-280 at 2026.10.3) and
internal/ha/client.go:604:// (websocket_api/commands.py:180-219 at 2026.10.3; an event outside
internal/ha/client.go:644:// (homeassistant/components/config/entity_registry.py:39-64 at 2026.10.3).
internal/ha/client.go:653:// (homeassistant/components/config/device_registry.py:75-101 at 2026.10.3).
internal/ha/client.go:662:// (homeassistant/components/config/area_registry.py:23-37 at 2026.10.3).
internal/ha/client.go:670:// administrator (homeassistant/components/config/label_registry.py:53-67
internal/ha/client.go:671:// at 2026.10.3). Entries that do not decode come back as a
internal/ha/client.go:679:// (websocket_api/commands.py:369-385 at 2026.10.3). A state that does not
internal/ha/client.go:709:// 2026.10.3).
internal/ha/client.go:717:// command (homeassistant/components/homeassistant/exposed_entities.py:
internal/ha/client.go:718:// 414-442 at 2026.10.3).
internal/ha/client.go:727:// (websocket_api/commands.py:266-295 at 2026.10.3).
internal/ha/client_hostile_test.go:107:		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
internal/ha/client_p1_test.go:43:		out(`{"type":"auth_ok","ha_version":"2026.10.3"}`)
internal/ha/client_p1_test.go:173:		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
internal/ha/client_p1_test.go:256:		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
internal/ha/client_p1_test.go:316:		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
internal/ha/client_test.go:140:	if err != nil || cfg.TemperatureUnit() != "°C" || cfg.Version != "2026.10.3" {
internal/ha/fanout_test.go:37:	f.Connected("2026.10.3")
internal/ha/fanout_test.go:53:		a: {"connected 2026.10.3", "event state_changed", "event label_registry_updated"},
internal/ha/fanout_test.go:54:		b: {"connected 2026.10.3", "event label_registry_updated", "disconnected", "connected 2026.10.4"},
internal/ha/fanout_test.go:69:	f.Connected("2026.10.3")
internal/ha/fanout_test.go:100:		if n == 0 || got[0] != "connected 2026.10.3" {
internal/ha/fuzz_test.go:64:		`{"version":"2026.10.3","location_name":"Home","unit_system":{"temperature":"°F"}}`,
internal/ha/hatest/camera.go:53:// (camera/webrtc.py:56-60, 292 at 2026.10.3).
internal/ha/hatest/camera.go:58:// AnswerEvent is the answer event (camera/webrtc.py:63-67 at 2026.10.3).
internal/ha/hatest/camera.go:62:// 2026.10.3; the keys are webrtc-models 0.3.0's aliases,
internal/ha/hatest/camera.go:68:// ErrorEvent is an error event (camera/webrtc.py:85-90 at 2026.10.3).
internal/ha/hatest/camera.go:99:// camera that is off (camera/__init__.py:806-808 at 2026.10.3).
internal/ha/hatest/camera.go:157:// (websocket_api/commands.py:246-263 at 2026.10.3: a known subscription's
internal/ha/hatest/camera.go:216:// 782-838 at 2026.10.3): 401 for a wrong Authorization header, 403
internal/ha/hatest/hatest.go:13:// Home Assistant core lines (tag 2026.10.3) whose shape it mirrors.
internal/ha/hatest/hatest.go:350:// 2026.10.3).
internal/ha/hatest/hatest.go:416:// (homeassistant/helpers/entity_registry.py:178-184 at 2026.10.3).
internal/ha/hatest/hatest.go:434:// (homeassistant/helpers/device_registry.py:78-80 and its update event
internal/ha/hatest/hatest.go:435:// data, device_id and changes, at 2026.10.3).
internal/ha/hatest/hatest.go:453:// old_entity_id (homeassistant/helpers/entity_registry.py:171-189,
internal/ha/hatest/hatest.go:454:// :2051-2061 at 2026.10.3).
internal/ha/hatest/hatest.go:479:// action update (homeassistant/helpers/label_registry.py:183-224 at
internal/ha/hatest/hatest.go:480:// 2026.10.3).
internal/ha/hatest/hatest.go:508:// (homeassistant/helpers/label_registry.py:169-180; the registries clear
internal/ha/hatest/hatest.go:509:// the id, helpers/entity_registry.py:2529-2532, at 2026.10.3).
internal/ha/hatest/hatest.go:541:// (homeassistant/helpers/entity_registry.py:171-175, :1728-1775 at
internal/ha/hatest/hatest.go:542:// 2026.10.3).
internal/ha/hatest/hatest.go:569:// entry id (homeassistant/helpers/entity_registry.py:1624 at 2026.10.3)
internal/ha/message.go:17:// Assistant core at 2026.10.3:
internal/ha/message.go:18:// homeassistant/components/websocket_api/auth.py:25-28 (auth,
internal/ha/message.go:32:// The events the daemon subscribes to. homeassistant/const.py:286
internal/ha/message.go:33:// (state_changed), homeassistant/helpers/entity_registry.py:83-85
internal/ha/message.go:34:// (entity_registry_updated), homeassistant/helpers/device_registry.py:78-80
internal/ha/message.go:35:// (device_registry_updated), homeassistant/helpers/label_registry.py:23-26
internal/ha/message.go:36:// (label_registry_updated), all at 2026.10.3. Each is in the
internal/ha/message.go:37:// SUBSCRIBE_ALLOWLIST (homeassistant/auth/permissions/events.py:30-50;
internal/ha/message.go:62:// at 2026.10.3 (code and message; translation fields are ignored).
internal/ha/message.go:97:// (websocket_api/http.py:152-179 at 2026.10.3; the client never enables
internal/ha/message.go:135:// (homeassistant/core.py:1397-1412 at 2026.10.3).
internal/ha/message.go:144:// (homeassistant/core.py:1988-2014 at 2026.10.3). Attributes hold the
internal/ha/message.go:146:// is available (homeassistant/helpers/entity.py:1115-1129), plus
internal/ha/message.go:194:// The two states that carry no value: homeassistant/const.py:303,313 at
internal/ha/message.go:195:// 2026.10.3.
internal/ha/message.go:201:// StateChanged is state_changed's data: homeassistant/core.py:129-142 at
internal/ha/message.go:202:// 2026.10.3 (entity_id, old_state, new_state; new_state null when the
internal/ha/message.go:211:// (homeassistant/helpers/entity_registry.py:171-189: action create, remove
internal/ha/message.go:213:// device_registry_updated (device_id instead of entity_id), at 2026.10.3.
internal/ha/message.go:224:// (homeassistant/helpers/label_registry.py:50-54 at 2026.10.3). A rename
internal/ha/message.go:232:// (homeassistant/components/config/label_registry.py:155-166 at
internal/ha/message.go:233:// 2026.10.3; color, created_at, description, icon, label_id, name,
internal/ha/message.go:244:// .as_partial_dict (homeassistant/helpers/entity_registry.py:369-403 at
internal/ha/message.go:245:// 2026.10.3). The list carries no device_class, unit, capabilities or
internal/ha/message.go:281:// .dict_repr (homeassistant/helpers/device_registry.py:589-629 at
internal/ha/message.go:282:// 2026.10.3); a child device's entry (:739-755) carries no manufacturer,
internal/ha/message.go:348:// (homeassistant/helpers/area_registry.py:88-98 at 2026.10.3).
internal/ha/message.go:355:// configuration (homeassistant/core_config.py:685-717 at 2026.10.3) with
internal/ha/message.go:356:// its unit_system (homeassistant/util/unit_system.py:213-224: temperature
internal/ha/message.go:374:// (homeassistant/components/homeassistant/exposed_entities.py:419-442 at
internal/ha/message.go:375:// 2026.10.3).
internal/ha/message.go:381:// (exposed_entities.py:25 KNOWN_ASSISTANTS at 2026.10.3).
internal/ha/message.go:386:// 2026.10.3).
internal/ha/message_test.go:19:		{`{"type":"auth_required","ha_version":"2026.10.3"}`, 1, false},
internal/ha/message_test.go:70:		`{"type":"auth_required","ha_version":"2026.10.3"}`,
internal/ha/state_cap_test.go:62:		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
internal/ha/state_cap_test.go:105:		send(ctx, ws, `{"type":"auth_ok","ha_version":"2026.10.3"}`)
internal/ha/subscription.go:29:// 276-292 at 2026.10.3: the result, then event_message(msg["id"], ...)).
internal/ha/subscription.go:48:// subscription (websocket_api/messages.py:107-109 at 2026.10.3). It is
internal/ha/subscription.go:136:// 276-278, camera/__init__.py:730-733 at 2026.10.3). On a connection that
internal/web/web.go:93:// the page (developers.home-assistant.io docs/apps/presentation.md
internal/web/web.go:160:// (supervisor/api/ingress.py:412-419, const.py:115, :625, :653-659 at
internal/web/web.go:161:// fe91b8db; from Supervisor 2026.10.4).
testdata/ha/area_registry_list.json:2: "_mirrors": "homeassistant/components/config/area_registry.py:23-37 (config/area_registry/list) and homeassistant/helpers/area_registry.py:88-104 (AreaEntry.json_fragment) @2026.10.3",
testdata/ha/auth_invalid.json:2: "_mirrors": "homeassistant/components/websocket_api/auth.py:44-46,126 (auth_invalid_message) @2026.10.3",
testdata/ha/auth_ok.json:2: "_mirrors": "homeassistant/components/websocket_api/auth.py:38 (AUTH_OK_MESSAGE) @2026.10.3",
testdata/ha/auth_ok.json:5:  "ha_version": "2026.10.3"
testdata/ha/auth_required.json:2: "_mirrors": "homeassistant/components/websocket_api/auth.py:39-41 (AUTH_REQUIRED_MESSAGE) @2026.10.3",
testdata/ha/auth_required.json:5:  "ha_version": "2026.10.3"
testdata/ha/device_registry_list.json:2: "_mirrors": "homeassistant/components/config/device_registry.py:75-101 (config/device_registry/list) and homeassistant/helpers/device_registry.py:589-629 (DeviceEntry.dict_repr) @2026.10.3; dev-tuya-1 carries an identifier whose value is a JSON number, which the registry never rejects (helpers/device_registry.py:411, no type validator)",
testdata/ha/entity_registry_list.json:2: "_mirrors": "homeassistant/components/config/entity_registry.py:39-64 (config/entity_registry/list) and homeassistant/helpers/entity_registry.py:369-403 (RegistryEntry.as_partial_dict); the C2 actions' unique_id per script/__init__.py:567-568 (the script's key), homeassistant/scene.py:354-358 (the scene's id) and automation/__init__.py:515 (the automation's id) @2026.10.3",
testdata/ha/expose_entity_list.json:2: "_mirrors": "homeassistant/components/homeassistant/exposed_entities.py:414-442 (homeassistant/expose_entity/list) @2026.10.3",
testdata/ha/get_config.json:2: "_mirrors": "homeassistant/components/websocket_api/commands.py:658-666 (get_config) and homeassistant/core_config.py:685-717, homeassistant/util/unit_system.py:213-224 @2026.10.3",
testdata/ha/get_config.json:36:  "version": "2026.10.3",
testdata/ha/get_states.json:2: "_mirrors": "homeassistant/components/websocket_api/commands.py:369-385 (get_states) and homeassistant/core.py:1988-2014 (State._as_dict); attributes per homeassistant/helpers/entity.py:1115-1176 and the domains' state attributes (light/__init__.py:447-469,609-660 capability and state attributes, light/const.py:102-122, climate/const.py:114-136,169-191, cover/const.py:18-23, fan/const.py:26-31); the R1-R5 entities per sensor/const.py:192,268,512 (classes), button/__init__.py:101-106, event/__init__.py:178-206, select/__init__.py:108-122, climate/__init__.py:239-341, vacuum/__init__.py:156-173, valve/entity.py:71-105, water_heater/__init__.py:164-225, media_player/__init__.py:858-899 and the feature masks of climate/const.py:194-205, vacuum/const.py:53-69, valve/const.py:36-42, water_heater/const.py:61-67, media_player/const.py:196-221; the cameras per camera/__init__.py:380-390 (state idle), 410-416 and 110 (entity_picture), 610-627 (state_attributes: access_token) and camera/const.py:71-75 (STREAM = 2); the C1 covers per cover/const.py:44-54 (CoverEntityFeature OPEN 1, CLOSE 2, SET_POSITION 4, STOP 8: cover.scene_blind 3, cover.position_blind 7); the C2 actions per script/__init__.py:590-610 (last_triggered, mode, current; on only while it runs) and helpers/script.py:119,126,135 (mode single by default), scene/__init__.py:104-110,118-120 (the state is the last activation's isoformat timestamp) and homeassistant/scene.py:362-367 (entity_id, id), automation/__init__.py:336-342 (id), 517-536 (last_triggered, mode, current; on while enabled) and automation/const.py:10-22 @2026.10.3",
testdata/ha/label_registry_list.json:2: "_mirrors": "homeassistant/components/config/label_registry.py:53-67 (config/label_registry/list) and :155-166 (_entry_dict) @2026.10.3",
```
