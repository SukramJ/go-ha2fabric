# Review phase 0, unit I: inventory

A map of the production Go (no `_test.go`, no `hatest`) plus `script/run.sh`,
`addon/config.yaml`, `Dockerfile`, `addon/Dockerfile`, at baseline `10ee7b4`
(worktree `review/p0`). No judgement. Every row is read from the file named;
a row marked "(go-fabric)" or "(go-fabric-ui)" names behaviour of a pinned
dependency that was not read for this unit.

Abbreviations: `cfg` = `internal/config/config.go`; `main` =
`cmd/ha2fabric/main.go`; `nodes.go` = `cmd/ha2fabric/nodes.go`; `run.sh` =
`script/run.sh`; `addon` = `addon/config.yaml`; `Dockerfile` = root image;
`addonDF` = `addon/Dockerfile`; `node.go` = `internal/config/node.go`.
Every variable also has a flag of the same meaning (`cfg:181-187`: the flag
default is the environment value; flags are parsed after, `cfg:211`).

## 1. Environment variables

Read by `str(...)` at `cfg:188-208` through `get` (`cfg:172-175`, trimmed with
`strings.TrimSpace`) and the two direct reads at `cfg:219,221`. No other
`Getenv`/`LookupEnv` in the production Go (`cfg:158` is the only reader).

| Variable | Read | Default | Check | `run.sh` sets | add-on option (schema) |
| --- | --- | --- | --- | --- | --- |
| `HA_URL` | cfg:188 | "" (add-on with `SUPERVISOR_TOKEN`: `ws://supervisor/core/websocket`, cfg:33,226-228) | required outside add-on (cfg:249-250); prefix `ws://` or `wss://` only (cfg:251-252); no parse of host/userinfo | no | none |
| `HA_TOKEN` | cfg:221 (env only, no flag, cfg:6-7) | "" | required if no `SUPERVISOR_TOKEN` (cfg:254-256) | no | none |
| `SUPERVISOR_TOKEN` | cfg:219 (env only) | "" | none; non-empty sets `Addon` (cfg:220), is the token when `HA_TOKEN` empty (cfg:222-224) | not set; inherited via `with-contenv` (run.sh:1,8) | none |
| `LISTEN` | cfg:189 | `:5540` (cfg:42) | `host:port`, port 1..65535 (cfg:257-259, 342-352) | `":$(port)"` run.sh:16 | `port: port`, 5540 (addon:49,66) |
| `MDNS_INTERFACES` | cfg:190 | "" (every interface) | split on `,` and newline; empty name or duplicate refused (cfg:380-397); names not checked against the host | run.sh:61-65 (from `mdns_interfaces`, list one per line) | `list(str)`, `[]` (addon:63,84-85) |
| `PASSCODE` | cfg:191 | 20202021 (cfg:43) | `ParseUint(...,10,32)` (cfg:260); `node.ValidateSetupPIN` (cfg:262) (go-fabric) | run.sh:17 | `int(1,99999998)`, 20202021 (addon:50,67) |
| `DISCRIMINATOR` | cfg:192 | 3840 (cfg:44) | `ParseUint(...,0,16)` and `<= node.MaxDiscriminator` (cfg:267-271) | run.sh:18 | `int(0,4095)`, 3840 (addon:51,68) |
| `VENDOR_ID` | cfg:193 | `0xFFF1` (cfg:193) | `id16`: base 0 ParseUint 16 bit, non-zero (cfg:272,333-340) | run.sh:19 | `int(1,65535)`, 65521 (addon:52,69) |
| `PRODUCT_ID` | cfg:194 | `0x8001` (cfg:194) | `id16` (cfg:273) | run.sh:20 | `int(1,65535)`, 32769 (addon:53,70) |
| `LABEL` | cfg:195 | `Matter Bridge` (cfg:47) | trimmed; non-empty (cfg:274-275); at most 32 bytes (cfg:276-279) | run.sh:21 | `str`, `Matter Bridge` (addon:54,71) |
| `STORE_PATH` | cfg:196 | "" then `ha2fabric.db` (cfg:55,233-235); add-on `/data/ha2fabric.db` (cfg:57,235-237) | directory (if not `.`) must exist and be a dir (cfg:281-285); the file itself not checked | run.sh:24 `/data/ha2fabric.db` | none |
| `INCLUDE` | cfg:197 | "" | split by `expose.Split` on `,` space newline tab (expose:199-208, cfg:239); each entry checked with `path.Match(p, "")` (expose:109-116, cfg:286-288) | run.sh:32-36 | `list(str)`, `[]` (addon:57,74-75) |
| `EXCLUDE` | cfg:198 | "" | as `INCLUDE` | run.sh:37-41 | `list(str)` (addon:58,76-77) |
| `INCLUDE_LABELS` | cfg:199 | "" | split on `,` and newline only, items trimmed, empties dropped (cfg:399-409); resolved later against HA labels (expose:82-98, 120-131); unresolved is a warning, not an error (bridge:564) | run.sh:42-46 | `list(str)` (addon:59,78-79) |
| `EXCLUDE_LABELS` | cfg:200 | "" | as `INCLUDE_LABELS` | run.sh:47-51 | `list(str)` (addon:60,80-81) |
| `EXPOSE_ASSIST` | cfg:201 | `true` | `strconv.ParseBool` (cfg:289-293) | run.sh:27 | `bool`, true (addon:55,72) |
| `LOG_LEVEL` | cfg:202 | `info` | `slog.Level.UnmarshalText` (cfg:309-311) | run.sh:53 | `list(debug\|info\|warn\|error)`, info (addon:61,82) |
| `WEB_LISTEN` | cfg:203 | `127.0.0.1:8099` (cfg:48) | empty disables (cfg:102 doc, main:111); else `host:port` (cfg:312-316) | run.sh:70 `0.0.0.0:8486` (fixed) | `ingress_port: 8486` (addon:44) |
| `WEB_ALLOW` | cfg:204 | "" (empty allows every client, cfg:104-105) | `expose.Split`; each entry `net.ParseIP` (cfg:317-321) | run.sh:71 `172.30.32.2` (fixed) | none |
| `WEB_ALLOWED_HOSTS` | cfg:205 | "" | `expose.Split`; each entry `allowedHost` refuses IP, scheme, port/brackets, path, wildcard (cfg:322-326, 359-373) | not set | none |
| `WEB_ADMIN` | cfg:206 | `false` | `ParseBool` (cfg:294-298) | not set (add-on ignores it, cfg:117-118, main:196) | none |
| `CAMERA` | cfg:207 | `false` | `ParseBool` (cfg:299-303) | run.sh:29 | `bool`, false (addon:56,73) |
| `NODE_LIMIT` | cfg:208 | 8 (cfg:53) | `Atoi`, 0..32 (cfg:304-308; `MaxNodeLimit` cfg:54) | run.sh:56-58 only if the option has a value | `int(0,32)`, 8 (addon:62,83) |

Other inputs of `config.Load` (cfg:209-216): flags `-dump-plan` (cfg:209),
`-version` (cfg:210); extra arguments refused (cfg:214-216). `-h` returns
`ErrHelp` (cfg:411-415, main:57-59).

Container images (not `run.sh`): `Dockerfile:31` sets `STORE_PATH=/data/ha2fabric.db`
and `WEB_LISTEN=0.0.0.0:8099`; `Dockerfile:33` runs as `nonroot:nonroot`;
`addonDF:44` `CMD ["/run.sh"]` (runs as the HA base image's user).
`run.sh` exports no `HA_URL`, `HA_TOKEN`, `WEB_ALLOWED_HOSTS`, `WEB_ADMIN`
(run.sh:15-75).

## 2. Listeners and outbound connections

### 2.1 Listeners

| What | Address / protocol | Where | Timeouts, limits | By mode |
| --- | --- | --- | --- | --- |
| Web UI | TCP `net.ListenConfig.Listen("tcp", WebListen)` (web:111-115) | `internal/web/web.go:106-129`, started `main:111-125` | `ReadHeaderTimeout` 10 s, `ReadTimeout` 30 s, `IdleTimeout` 120 s (web:108-109); no `WriteTimeout`, no `MaxHeaderBytes`, no `TLSConfig` set (web:106-110); shutdown 5 s (web:121) | add-on `0.0.0.0:8486` (run.sh:70); container `0.0.0.0:8099` (Dockerfile:31); stand-alone default `127.0.0.1:8099` (cfg:48); empty `WEB_LISTEN` disables (main:111) |
| Web client filter | `RemoteAddr` host must equal one of `Allow` IPs, else 403 (web:86-99); empty `Allow` is no filter (web:87-89) | web:57-76 | none | add-on `172.30.32.2` (run.sh:71); others "" by default |
| Matter node, UDP | `node.Network.Listen = cfg.Listen` (node.go:43) (go-fabric) | `served.start` nodes.go:98-105 | `ncfg` of go-fabric; none set here | default `:5540` (cfg:42); add-on `:<port>` (run.sh:16) |
| Matter node, TCP | `node.TCPConfig{Incoming: cfg.Camera}` (node.go:44) (go-fabric) | same | none set here | only with `CAMERA=true` (node.go:44) |
| mDNS responder | `node.MDNS{Interfaces: cfg.MDNSInterfaces}` (node.go:45) (go-fabric) | same | none set here | every multicast interface when the list is empty (cfg:79-82) |
| Health probe | `node.Health{Interval: node.DefaultProbeInterval}` (node.go:66) (go-fabric: checks the node's own operational listener, not a network listener of ha2fabric) | same | 30 s (go-fabric health.go:18-20) | all |
| Registry nodes' UDP/TCP | per node `rec.Listen` from `nodes.json` or `Allocate` (nodes.go:398-410 cmd, nodes.go:312-338 internal); the same `node.Config` form via `c.Node(log)` (cmd nodes.go:409) | `internal/nodes/nodes.go:312-338` | probe-bind `net.ListenPacket("udp")` and `net.Listen("tcp")` on candidate ports, closed at once (nodes:341-356); ports `base+1..65535` | up to `NODE_LIMIT` (cmd nodes.go:463) |
| Port probe | `bindable` (nodes:341-356) | nodes:333 | none | each `Create` |

`nodes.json` entries' `Listen` is checked non-empty only (nodes:151-152);
registry `Node.Listen` is not passed through `hostPort`.

### 2.2 Outbound connections

| What | Address / protocol | Where | Timeouts, limits |
| --- | --- | --- | --- |
| Home Assistant WebSocket | `websocket.Dial(dctx, c.opts.URL, nil)` (ha:197), `github.com/coder/websocket`, dial options nil (default HTTP client, default redirect policy, system TLS roots for `wss`) | `internal/ha/client.go:195-255`; URL from `cfg.HAURL` (main:74) | dial `CallTimeout` 30 s (ha:46-47,196); read limit 64 MiB (`DefaultReadLimit`, ha:52, `SetReadLimit` ha:202); auth `CallTimeout` (ha:260); ping 30 s (ha:46,236) with `CallTimeout` (ha:246); backoff 1 s to 60 s (ha:48-49,150-171) |
| Auth frame | `{"type":"auth","access_token":<token>}` text frame (ha:269-275) | ha:259-288 | as above |
| Camera still | HTTP GET `HTTPBase(URL)/camera_proxy/<entity>?width&height` with `Authorization: Bearer <token>` (camera.go:131-153), `c.http.Do` (ha camera.go:154), client `http.Client{Timeout: CallTimeout}` (client.go:139), default redirect policy | `internal/ha/camera.go:126-180` | 30 s; body `io.LimitReader(ImageLimit+1)` (camera.go:168), `DefaultImageLimit` 64000 (camera.go:87); scheme from `ws`->`http`, `wss`->`https` (camera.go:100-107) |
| Camera WebRTC signalling | `camera/webrtc/offer` subscription and `camera/webrtc/candidate` over the one WebSocket (ha camera.go:55-67) | `internal/ha/camera.go:55-67` | `CallTimeout`; per camera `SignalTimeout` 10 s, `Retries` 10, `RetryInterval` 50 ms (camera.go:202-209) |
| Port probes | local only (see 2.1) | nodes:341-356 | none |
| go-fabric peers | the Matter node's own sockets: Matter sessions with controllers, mDNS multicast (go-fabric, not read here) | n/a | go-fabric's |

Nothing else in the production Go dials (`grep` for `http.Get`, `net.Dial`,
`websocket.Dial`, `http.Client` finds only `ha/client.go:139,197`).
`script/coverfloor/main.go` makes no network call.

## 3. go-fabric-ui `Options` set

`fabricui.Options` is built once, at `main:112-120`, inside `web.Options.UI`
(web.go:44). `web.New` fills `UI.Logger` if nil (web:62-64) and calls
`fabricui.NewHandler(opts.UI)` (web:71). Only `Admin` and `AllowHost` differ
by mode. Admin choice: `admin(cfg)` (main:194-203); host choice:
`allowHost(cfg)` (main:212-217).

| Field | Value | Line | add-on | stand-alone read-only (default) | stand-alone `WEB_ADMIN=true` |
| --- | --- | --- | --- | --- | --- |
| `Node` | `def.sw` (`NodeSwitch` over the configured node) | main:113, nodes.go:133-137 | same | same | same |
| `Exposure` | `def.prov` (`exposure.Provider`) | main:113, nodes.go:84 | same | same | same |
| `Admin` | add-on: `web.From("172.30.32.2")`; `WEB_ADMIN`: `nil`; else `web.ReadOnly` (always false) | main:113,194-203; web:136-149 | `From(172.30.32.2)` | `ReadOnly` | `nil` (UI's token mode, per-process token, cfg:113-119) |
| `Reassemble` | `def.Resync` | main:113, nodes.go:162-169 | same | same | same |
| `Events` | `def.events` | main:113, nodes.go:83 | same | same | same |
| `Warnings` | `warnings(def, fl)` | main:113,152-156 | same | same | same |
| `Counters` | `counters(def, fl)` (one counter `discards`) | main:113,166-170 | same | same | same |
| `Erase` | `def.erase` (factory reset, 30 s, nodes.go:77,220-260) | main:113 | same | same | same |
| `Logger` | `logger` | main:113 | same | same | same |
| `Operations` | `web.NewOperations(ring, version.Version, started)` (log ring read, level set, info) | main:114; web/operation.go:28-77 | same | same | same |
| `Registry` | `fl` (`*fleet`) | main:115, nodes.go:343-369 | same | same | same |
| `NodeConfig` | `nodeConfig(cfg)` (values, "never the passcode", main:172-188) | main:115 | add-on note | flags note | flags note |
| `AllowedHosts` | `cfg.WebAllowedHosts` | main:116 | "" unless set (run.sh does not set it) | per `WEB_ALLOWED_HOSTS` | same |
| `AllowHost` | add-on `web.From("172.30.32.2")`; else `nil` | main:116,212-217 | `From(172.30.32.2)` | nil | nil |
| `Done` | `rctx.Done()` | main:119 | same | same | same |
| not set | any other `Options` field (none named in main) | main:112-120 | n/a | n/a | n/a |

Registry node entries (`fleet.entry`, nodes.go:424-451) carry per node: `Source`
(`s.sw`, only while running), `Exposure`, `Reassemble`, `Events`, `Erase`
(`f.Erase(ctx,key)`); `Config` view with `Source: Registry`, `Store:
rec.StoreFile()` (nodes.go:427-433). `fleet` methods exposed to the UI:
`List` 413, `Create` 457, `Rename` 506, `Delete` 542, `Erase` 579.

`web.Server` wrapping: the `Allow` IP filter runs before the UI handler
(web:75, 86-99); `web.Options.Allow = cfg.WebAllow` (main:112).

## 4. Parse sites of external input

| # | Input | Site | Parser | Bound / validation at the site |
| --- | --- | --- | --- | --- |
| P1 | WebSocket frame from HA (every frame, auth included) | ha/message.go:76-108 `Decode`; called from ha/client.go:295 (`readOne`) and 311 (`read`) | `json.Unmarshal` into `Message` (object, message.go:84-85) or `[]Message` (array, 89-92) | frame <= 64 MiB (client.go:52,202); `TrimSpace`, first byte `{`/`[` (82-95); empty array refused (96-98); empty `Type` and negative `ID` refused (99-106). `Result`, `Event` kept as `json.RawMessage` |
| P2 | `auth_required`/`auth_ok`/`auth_invalid` reply | ha/client.go:259-288 | `Decode` (P1) then `reply.Type`, `reply.HAVersion`, `reply.Message` | `HAVersion` and `Message` strings unbounded (message.go:54-56); `Message` embedded in `ErrAuthInvalid` text (client.go:284) |
| P3 | `event` messages of own subscriptions | ha/client.go:316-338 | `json.Unmarshal(m.Event, &ev)` into `ha.Event` (message.go:112-117) | `ours := s.subs[m.ID]` (client.go:320,330); error -> `ha.event_refused` log (335) |
| P4 | `state_changed` event data | bridge/bridge.go:262-270 | `json.Unmarshal(ev.Data, &sc)` into `ha.StateChanged{EntityID, OldState, NewState *State}` (message.go:144-148); `State.Attributes map[string]any` (message.go:129) | none beyond JSON; `NewState == nil` delete+resync (bridge:647-656); error -> `bridge.event_refused` log (267) |
| P5 | `entity_registry_updated` event data | bridge/bridge.go:271-283 | anonymous struct `{action, entity_id}` (276-280); only `remove` with an `entity_id` acts (280-281) | error ignored (280); triggers `resyncSoon` (283) |
| P6 | `device_registry_updated`, `label_registry_updated` events | bridge/bridge.go:284-288 | data not parsed | `resyncSoon` only |
| P7 | Command results, one-shot: `config/entity_registry/list`, `config/device_registry/list`, `config/area_registry/list`, `config/label_registry/list`, `get_states`, `get_config`, `homeassistant/expose_entity/list` | ha/client.go:457-480 (`Call`), 484-538 (typed callers); structs `EntityEntry`/`DeviceEntry`/`Area`/`Label`/`State`/`Config`/`Exposed` (message.go:189-317) | `json.Unmarshal(m.Result, out)` (client.go:476); `Tuple.UnmarshalJSON` for identifiers/connections (message.go:260-285) | bound is the 64 MiB frame; list lengths not bounded; error -> `ha: <type> result: ...` (client.go:477) |
| P8 | HA command error | ha/message.go:61-66; client.go:427-432 | `Error{Code, Message}`; `m.Error` returned as the call's error | strings unbounded; text `home assistant: <code>: <message>` |
| P9 | Camera WebRTC event | camera/camera.go:456-474 `relay` | `json.Unmarshal(raw, &ev)` into `ha.WebRTCEvent` (ha camera.go:31-38) | stream channel 64 deep (`SubscriptionBuffer`, subscription.go:25; overflow drops the stream, client.go:324-327); error -> `camera.event_refused` log (camera.go:461) |
| P10 | WebRTC `answer` SDP from HA | camera.go:493-494 | none; passed to `Signaller.Answer(ctx, ss.id, ev.Answer)` (go-fabric checks size: `ErrPayloadTooLarge`, camera.go:563) | string unbounded here |
| P11 | WebRTC ICE candidate from HA | camera.go:495-508, `fromHA` 420-435 | none for the string; `SdpMLineIndex` int range 0..65535 checked (429-431); empty `SdpMid` -> null (423) | empty candidate string ignored (496-500) |
| P12 | WebRTC `session` id from HA | camera.go:479-492 | `ss.haID = ev.SessionID` | unbounded string, no check |
| P13 | Camera still, bytes | ha/camera.go:154-179; camera/camera.go:653-673 `CaptureSnapshot` | `Content-Type` via `mime.ParseMediaType` (ha camera.go:175-178); must equal `image/jpeg` (camera.go:659); `image.DecodeConfig` must be `jpeg` (662-665); width/height 1..65535 (666-668) | <= `ImageLimit` 64000 bytes (ha camera.go:87,168-174) |
| P14 | Controller's WebRTC offer (SDP, session id, stream usage, peer ids) | camera.go:310-349 `ProvideOffer`; `args.Sdp` to `Subscribe` (322); `streams()` 354-362 | no parse here; go-fabric decodes the Matter command (go-fabric) | `SdpMLineIndex`/`SdpMid` of controller candidates: `toHA` 407-418 (no range check, int of a uint16) |
| P15 | Controller's ICE candidates | camera.go:367-391 `ProvideIceCandidates`, `toHA` 407-418 | none; sent to HA as `camera/webrtc/candidate` (ha camera.go:62-67); queued in `ss.pending` before the HA session id (380) | `pending` unbounded here (380) |
| P16 | Matter command -> `model.Command` (`Method`, `Payload []byte`) | bridge/command.go:56-61 -> adapt/command.go:88-225 `Service` | `strings.TrimSpace(string(cmd.Payload))` (97); `strconv.ParseFloat` (`number`, 376-379; NaN/Inf refused); `integer` clamps to range (381-392); `ParseBool` (209); `ParseUint(payload,10,8)` (284); light payload `json.Unmarshal` into `{state, brightness, color_temp, color{string->float64}}` (308-317) | see section 5 for what reaches HA |
| P17 | Entity attributes read for descriptions and state | adapt/adapt.go:113-154 `Entity`, 174-229 `description`, 235-266 `State`; helpers `str`, `strings_`, `toFloat`, `toInt`, `num`, `floats` (268-334) | type assertions on `map[string]any` | `strings_` drops empty and whitespace strings (283); no length bound |
| P18 | Device registry fields into the device model | adapt/adapt.go:44-72 `Device` | field copy; `Identifiers` kept if `len==2 && id[1]!=""` (51), `Connections` if `len==2` (57) | unbounded strings (name, manufacturer, model, serial, `ConfigURL`) |
| P19 | `selection.json` (and `selection-<key>.json`) | selection/selection.go:74-108 `Open` | `os.ReadFile` (82), `json.Unmarshal` into `file{Version, Entities map[string]Entry}` (89-90); version must be 1 (90-92) | no size or entry-count limit; on error rename to `<path>.corrupt-<unix>` (95-98), log `selection.corrupt` (99) |
| P20 | `nodes.json` | nodes/nodes.go:113-140 `Open`, `file.check` 144-161 | `os.ReadFile` (118), `json.Unmarshal` into `file{Nodes []Node}` (126) | `check`: key valid (<=64, `[A-Za-z0-9._-]`, first alnum, not reserved, unique; nodes:269-280), `Listen` non-empty, `Discriminator <= MaxDiscriminator`, `ValidateSetupPIN(Passcode)`; no node-count limit on load (the limit is in `Create`, cmd nodes.go:463); on error rename to `nodes.json.corrupt-<unix>` (130-133), log `nodes.corrupt` (134) |
| P21 | Web UI exposure updates (key, enabled, name) | exposure/exposure.go:215-233 `check`, 237-246 `proposed`, 280-318 `Apply` | `[]fabricui.Update`; key must exist in the current verdicts (224-227); enabling an unmappable refused (228-230); name `strings.TrimSpace` (242,303) | name length not checked here; written to `selection.json` (313) |
| P22 | Web UI node create / rename | cmd/ha2fabric/nodes.go:457-487 `Create`, 506-535 `Rename` | `spec.Label` / `label` to `Allocate` (nodes:196-214), `Slug` (243-265, key <= 56 + suffix), `nodeLabel` cuts to 32 bytes at a rune boundary (nodes.go:388-394) | `Rename` stores `label` unchanged in the registry (nodes.go:512-519, internal nodes:410-424); node limit (nodes.go:463) |
| P23 | Environment and flags | config.go:171-331 | see section 1 | whole-config validation, all issues in one error (cfg:144-152,327-329) |
| P24 | Build metadata | version/version.go:56-70 `Software` | `strconv.ParseUint` per part, <= 999 (62-65) | non-matching version -> 0 |
| P25 | Coverage profile and floors file (build tool, not the daemon) | script/coverfloor/main.go:83,114 | `os.Open`, `os.ReadFile` of the named profile / floors file | `os.Args[1]` only (34-39) |
| P26 | Catalog payload defaults | adapt/command.go:356-373 `catalogPayload`; entity `Desc().Extra[key]` (357) then `hacatalog.LoadMQTT()` defaults (360-372) | type assertion to string | none |

## 5. `call_service` / outbound Home Assistant commands

Every outbound HA command is one of two sets.

### 5.1 `call_service`

Single place: `ha.Client.CallService` (ha/client.go:543-552) builds
`{"type":"call_service","domain","service","target":{"entity_id"},"service_data":...}`
(544-550); the only caller is `Bridge.command` (bridge/command.go:69) with the
`ServiceCall` of `adapt.Service` (adapt/command.go:88-225); timeout
`commandTimeout` 30 s (bridge/command.go:21,67). The domain is always the
entity id's domain (`ha.Domain(id)`, command.go:93), except `water_heater`
(command.go:248), `media_player` (282), `light` (318), which fix it
(the entity is of that domain in those paths). Target is always the one
entity id of the command (command.go:95,248,282,318); queue: 256 commands
(bridge:175), full -> drop with log (command.go:26-31).

| Domain | Controller method / payload | Service | `service_data` the controller supplies | Line |
| --- | --- | --- | --- | --- |
| button | `press` | `press` | none | command.go:55,102-103 |
| climate | turn_on / turn_off / toggle | `turn_on` / `turn_off` / `toggle` | none | command.go:59 |
| vacuum | pause / resume / go_home | `pause` / `start` / `return_to_base` | none | command.go:62 |
| valve | open_valve / close_valve | `open_valve` / `close_valve` | none | command.go:65 |
| media_player | turn_on/off, toggle, media_play/pause/stop | `turn_on`, `turn_off`, `toggle`, `media_play`, `media_pause`, `media_stop` | none | command.go:70-73 |
| script | turn_on | `turn_on` | none | command.go:75 |
| scene | turn_on | `turn_on` | none | command.go:78 |
| automation | trigger | `trigger` | none | command.go:81 |
| switch | payload equal to `payload_on`/`payload_off` (entity extra or catalog default) | `turn_on` / `turn_off` | none | command.go:37-39,108-113 |
| fan | same | `turn_on` / `turn_off` | none | command.go:41,108-113 |
| cover | payload equal to open/close/stop | `open_cover` / `close_cover` / `stop_cover` | none | command.go:44,108-113 |
| lock | payload equal to lock/unlock | `lock` / `unlock` | none | command.go:46,108-113 |
| light | method "" , JSON payload | `turn_off` if `state` equals "OFF" case-insensitively, else `turn_on` | `brightness` (clamped 0..255), `color_temp_kelvin` (int, only if >0, not clamped above), `hs_color` [h,s] (floats, not clamped), `xy_color` [x,y] (floats, not clamped), `rgb_color` [r,g,b] (each clamped 0..255) | command.go:106-107,308-351 |
| cover | `set_position` | `set_cover_position` | `position` int 0..100 (clamped) | command.go:114-118 |
| cover | tilt | `set_cover_tilt_position` | `tilt_position` int 0..100 | command.go:119-123 |
| climate | mode | `set_hvac_mode` | `hvac_mode` = trimmed payload string, non-empty, unbounded, not checked against the entity's modes | command.go:124-128 |
| climate | temperature | `set_temperature` | `temperature` float64 (finite, not range-checked) | command.go:129-133 |
| climate | temperature_low / temperature_high | `set_temperature` | `target_temp_low` or `target_temp_high` = payload float, plus the other from the entity's current attribute (`toFloat`); error if the other is absent | command.go:134-149 |
| climate | fan_mode | `set_fan_mode` | `fan_mode` = trimmed payload string, non-empty | command.go:173-177 |
| fan | percentage | `set_percentage` | `percentage` int 0..100 | command.go:150-154 |
| fan | preset_mode | `set_preset_mode` | `preset_mode` = trimmed payload string, non-empty | command.go:155-159 |
| fan | oscillation | `oscillate` | `oscillating` bool, only if payload equals `payload_oscillation_on`/`off` | command.go:160-167 |
| fan | direction | `set_direction` | `direction` only `forward` or `reverse` | command.go:168-172 |
| select | select_option | `select_option` | `option` = trimmed payload string, non-empty, not checked against the entity's options | command.go:178-182 |
| vacuum | run_mode | `return_to_base` / `stop` (payload "0", by `supported_features` bit 16 / 8) or `start` (payload "1") | none | command.go:183-199, 229-232 |
| water_heater | operation_mode | `set_operation_mode` / `turn_off` / `turn_on` | `operation_mode`: the literal `off` or an entry of the entity's `operation_list` (payload `on`/`off` only) | command.go:200-201,246-272 |
| water_heater | temperature | `set_temperature` | `temperature` float64 | command.go:202-206 |
| media_player | volume_mute | `volume_mute` | `is_volume_muted` bool (`ParseBool`) | command.go:207-211 |
| media_player | volume_level | `volume_set` | `volume_level` float: payload rounded, clamped 0..LevelMax, scaled to 0..1 | command.go:212-221 |
| media_player | key | `volume_up` / `volume_down` / `volume_mute` / `toggle` | for mute `is_volume_muted` = negation of the entity's current attribute; payload is a CEC key code `ParseUint(...,10,8)`, only Volume Up/Down, Mute, Power act | command.go:222-223,280-301 |
| any other | none | error `ErrCommand`, nothing sent | n/a | command.go:225 |

Failure path: `bridge.call_service_failed` log (command.go:78); on failure the
entity state is re-applied from the cache (command.go:81-85). Counters:
`Commands`, `CommandErrs`, `LastCommand` (command.go:70-76).

### 5.2 Other outbound HA commands (not `call_service`)

| Command | Where | Data |
| --- | --- | --- |
| `auth` | ha/client.go:269 | the token |
| `subscribe_events` (4 types: `state_changed`, `entity_registry_updated`, `device_registry_updated`, `label_registry_updated`) | ha/client.go:448; list main:53 | event type only |
| `ping` | ha/client.go:247 | none |
| `config/entity_registry/list`, `config/device_registry/list`, `config/area_registry/list`, `config/label_registry/list` | ha/client.go:486,494,502,511 | none |
| `get_states`, `get_config` | ha/client.go:519,527 | none |
| `homeassistant/expose_entity/list` | ha/client.go:536 (only when `Assist` rule on, snapshot.go:59) | none |
| `camera/webrtc/offer` | ha/camera.go:56 via `Subscribe` (subscription.go:104-127) | `entity_id`, `offer` = the controller's SDP |
| `camera/webrtc/candidate` | ha/camera.go:62-67 | `entity_id`, `session_id` (HA's), the controller's candidate |
| `unsubscribe_events` | ha/subscription.go:148 | the subscription id |
| HTTP GET `camera_proxy/<entity>` | ha/camera.go:126-180 | `entity_id` path-escaped (131), `width`, `height` of the controller's snapshot request (camera.go:654) |

## 6. Log calls whose attributes carry configuration, tokens, passcodes, entity data, or errors that may embed them

Logger: `logring` handler over `slog.TextHandler(stderr)` at `LOG_LEVEL`
(web/operation.go:19-22); every record enters the ring that the web UI
reads (`Operations.Log`, operation.go:53-60, attrs rendered with
`Value.String()`, 81-86) and writes to stderr. Records of all levels pass
`lvl` (main:70-72). Column "Carries": C config, T token, P passcode or
pairing code, E entity data, X error text that may embed external text.

| Message | Level | Attrs | Carries | Line |
| --- | --- | --- | --- | --- |
| `ha2fabric.start` | Info | `version`, `ha_url` (the full `HAURL` string), `addon`, `camera` | C (URL as given, including any userinfo or query) | main:86 |
| `ha.connected` | Info | `ha_version`, `url` (full) | C | ha/client.go:225 |
| `ha.disconnected` | Warn | `error` (dial, read, ping, auth errors), `retry_in` | X (dial error text from the library; `auth_invalid` message from HA) | ha/client.go:160 |
| `ha.frame_refused` | Warn | `error`, `bytes` | X | ha/client.go:313 |
| `ha.event_refused` | Warn | `error` | X | ha/client.go:335 |
| `ha.subscription_overflow` | Warn | `id` | none | ha/client.go:325 |
| `ha2fabric.pairing` | Info | `manual_code`, `qr_payload`, `hint` | P (manual code and QR payload, which encode the passcode and discriminator) | cmd nodes.go:312-314 |
| `ha2fabric.pairing_codes` | Warn | `error` | X | cmd nodes.go:309 |
| `ha2fabric.fabrics` | Info/Warn | `count`, per fabric `fabric_<n>{vendor_id,label}`, `hint`; or `error` | fabric label set by a controller | cmd nodes.go:271,276-282 |
| `ha2fabric.mdns_interface` | Warn | `code`, `severity`, `message` (from go-fabric) | host interface names | cmd nodes.go:301 |
| `ha2fabric.node_erased` | Info/Warn | `fabrics_removed`, `elapsed`, `error` | X | cmd nodes.go:243-247 |
| `ha2fabric.node_lost` | Error | `error`, `hint` | X | cmd nodes.go:256-257 |
| `ha2fabric.selection`, `.nodes`, `.node`, `.web`, `.node_stop` | Error/Warn | `error` | X (file paths in `os` errors) | main:77,90,103,132,143 |
| `ha2fabric.dump_plan` | Error | `error` | X | main:259,265 |
| `ha2fabric.label_unresolved` | Warn | `label` (configured name) | C | main:272 |
| `ha2fabric.node_start` | Error | `node`, `listen`, `error` | C (registry listen) | cmd nodes.go:378 |
| `ha2fabric.node_created` | Info | `node`, `listen`, `discriminator` | C (no passcode) | cmd nodes.go:485 |
| `ha2fabric.node_renamed`, `.node_deleted`, `.node_fabrics`, `.node_remove_fabric`, `.node_stop` | Info/Warn | `node` key, `error`, counts | X | cmd nodes.go:526,530,533,558,573,635,641,663 |
| `web.listening` | Info | `addr`, `allow` (count) | C | web/web.go:116 |
| `selection.corrupt` | Warn | `file`, `moved_to`, `error` (json error text) | X | selection.go:99 |
| `nodes.corrupt` | Error | `moved_to`, `error`, `hint` | X | nodes.go:134 |
| `bridge.selection_applied` | Info | `set`, `removed` (counts) | none | exposure.go:316 |
| `bridge.event_refused` | Warn | `event`, `error` (json error text) | X | bridge.go:267 |
| `bridge.sync_failed` | Error | `error` (wrapped HA error: `home assistant: <code>: <message>`) | X | bridge.go:374 |
| `bridge.batch` | Error | `error` | X | bridge.go:444 |
| `bridge.forget` | Info | `entity`, `key` | E | bridge.go:447 |
| `bridge.synced` | Info | six counts | none | bridge.go:505-506 |
| `bridge.queue_full` | Error | `hint` | none | bridge.go:218 |
| `bridge.camera_disabled` | Info | `skipped`, `enable` | none | bridge.go:361 |
| `bridge.label_unresolved` | Warn | `warning` (text of a warning built from configured labels) | C | bridge.go:564 |
| `bridge.build_failed` | Warn | `device` (device name from HA), `error` | E, X | bridge.go:603 |
| `bridge.unreachable` | Debug | `key`, `error` | E (stable key) | bridge.go:253 |
| `bridge.upstream_unusable` | Warn | `hint` | none | bridge.go:257 |
| `bridge.apply` | Debug | `entity`, `cluster`, `error` | E | bridge.go:637 |
| `bridge.camera_unattached` | Error | `entity`, `row`, `error` | E | bridge.go:47 (bridge/camera.go) |
| `bridge.command_dropped` | Error | `entity`, `hint` | E | bridge/command.go:29 |
| `bridge.command_discarded` | Info | `entity`, `cluster`, `command` | E | bridge/command.go:40 |
| `bridge.command_unmapped` | Warn | `entity`, `error` (text contains method and the controller's payload, command.go:225; or `payload %q` in light, command.go:316) | E, controller-supplied payload | bridge/command.go:63; adapt/command.go:90,147,225,271,300,316 |
| `bridge.call_service_failed` | Warn | `call` (`domain.service entity`), `error` (HA's error text) | E, X | bridge/command.go:78 |
| `bridge.call_service` | Debug | `call` | E | bridge/command.go:88 |
| `camera.session` | Info | `session`, `peer_node` | none | camera.go:347 |
| `camera.offer_failed` | Warn | `session`, `error` | X | camera.go:324 |
| `camera.session_ended`, `camera.session_lost` | Info | `session`, `reason` or `error` | X | camera.go:401,471 |
| `camera.event_refused` | Warn | `session`, `error` | X | camera.go:461 |
| `camera.candidate_failed`, `camera.candidate_refused` | Warn | `session`, `error` / `sdp_mline_index` | X | camera.go:490,503 |
| `camera.ha_error` | Warn | `session`, `code`, `message` (HA's text) | X | camera.go:510 |
| `camera.event_ignored` | Debug | `session`, `type` (HA's) | X | camera.go:514 |
| `camera.signal_failed`, `camera.signal_too_large` | Error/Warn | `session`, `command`, `error`, `hint` | X | camera.go:539,564,567 |
| `camera.snapshot_failed` | Warn | `error` (includes entity id and status) | E, X | camera.go:656 |
| `camera.end`, `camera.unsubscribe` | Debug | `session`, `error` | X | camera.go:286,443,587 |
| every camera log | all | `entity_id` added by `log.With` | E | camera.go:214 |
| registry node logs | all | `node` key added by `log.With` | none | cmd nodes.go:403, 555 |

Token handling facts: `HAToken` is used at `ha/client.go:269` (auth frame)
and `ha/camera.go:151-153` (header) and `main:74` (options); it is never an
attribute of any `slog` call above; `nodeConfig` omits the passcode
(main:172-188). `PASSCODE` is carried as a number in `Config.Passcode`
and in the QR/manual code (`ha2fabric.pairing`). `run.sh:73` logs the
listen address and store path through bashio. `run.sh` echoes no secret.
The go-fabric node receives `cfg.Node(logger)` with the daemon's logger
(node.go:67), so go-fabric's own log attributes go to the same ring.

## 7. Goroutines, locks, channels, subscriptions

### 7.1 Goroutine starts

| Start | Where | Ends |
| --- | --- | --- |
| signal context | main:46 `signal.NotifyContext` | `stop()` main:48 |
| HA client run loop | main:109 `wg.Go(client.Run(rctx, fan))` | `rctx` cancel (ha/client.go:153-155,163-165); `wg.Wait` main:138 |
| web server | main:124 `wg.Go(ws.Run(rctx))`; inner `go srv.Serve(ln)` web.go:118 | `ctx.Done` -> `Shutdown` 5 s (web:119-123); `wg.Wait` main:138 |
| per node bridge worker | cmd nodes.go:124-127 `go br.Run(ctx)`; one for the configured node (`serve`) and one per registry node | `s.cancel()` then `<-s.done` (nodes.go:141-147); in `close` and `erase` |
| bridge command loop | bridge.go:188 `wg.Go(commandLoop)` | `ctx.Done` (command.go:48-49); `defer wg.Wait` (bridge:189) |
| HA session reader | ha/client.go:213 `go c.read(rctx, s, h)` | read error -> `s.fail` (client.go:306-309); `stop()` deferred (212) cancels `rctx`; `fail` closes the socket (371) |
| camera relay | camera.go:337 `go h.relay(h.ctx, ss)` (one per WebRTC session, `wg.Add(1)` camera.go:335) | stream channel closed (`Subscription.close`), or `relay` returns after `event` false (camera.go:464-466); `Host.Close` calls `h.cancel()` and `h.wg.Wait()` (camera.go:291-292) |
| dump-plan client | main:242-245 `go client.Run(ctx, w)` | `cancel(); <-done` (main:246-249); ctx timeout 30 s (main:238) |
| debounce timer | bridge.go:319 `time.AfterFunc(Debounce=1 s, ...)` (bridge.go:165-167) | `timer.Stop()` at `Run` end (bridge:194-196) and on each re-arm (316-318) |
| go-fabric goroutines | node transport, mDNS, health probe, sessions (go-fabric) | `Node.Stop` (nodes.go:333, main:142) |

### 7.2 Locks

| Lock | Guards | Where |
| --- | --- | --- |
| `Client.mu` (`sync.Mutex`) | `cur` session | ha/client.go:104,221-223,230-232,458-460 |
| `session.writeMu` | WebSocket write and id allocation | ha/client.go:184,387 |
| `session.mu` | `pending`, `subs`, `streams`, `err` | ha/client.go:186,319-322,340-343,354-372,391-407 |
| `Subscription.mu` | `closed`, `err`, channel send/close | ha/subscription.go:37,56,64,79 |
| `Fanout.mu` | handler list, `connected`, `version`; held while calling each handler | ha/fanout.go:24,33,46,53,60,70,79 |
| `Bridge.mu` | `states`, `verdicts`, `last`, `all`, `status`, `warnings`, `timer`, `synced` | bridge.go:151, 193,303,314,328,335,342,386-401,489-504,536 |
| Bridge fields without lock | `groups`, `owner`, `known`, `unit`, `forget`, `hosts`, `since`, `warned`, `cameraLogged`: accessed from the one work goroutine (`Run`, bridge.go:206-208) | bridge.go:134-149 |
| `Host.mu` (camera) | `sig`, `sessions`, `closed`, session `haID`/`pending` | camera.go:164,227,268,311,368,396,449,480,579,594 |
| `served.op` (`Mutex`) | serialises `erase` and `close`; `closed` | cmd nodes.go:53-54,221,321 |
| `served.mu` (`RWMutex`) | run (`n`, `br`, `cancel`, `done`, `lost`, `lostAt`, `discarded`) | cmd nodes.go:59-66,152-154,163,173,185,197-200,229,254,327 |
| `served.live` (`atomic.Bool`) | mirrors `n != nil` | cmd nodes.go:66,129,242,179 |
| `fleet.op` (`Mutex`) | serialises Create, Rename, Erase, Delete, Close | cmd nodes.go:365,458,507,543,580,652 |
| `fleet.mu` (`RWMutex`) | `members` | cmd nodes.go:366,414,481,515,560,570,585,602,620,655 |
| `selection.Store.mu` | `entries` and file write | selection.go:55,123,130,142,157 |
| `nodes.Registry.mu` | `nodes` and `nodes.json` write | nodes.go:101,168,175,197,396,411,428 |
| `waiter.mu` / `once` | dump-plan connect and error | main:221-233 |
| `Client.conn` (`atomic.Bool`), `Bridge.connected` (`atomic.Bool`) | connected flags | ha/client.go:106; bridge.go:131 |

Lock held across external calls: `served.erase` holds `s.mu.Lock` over
`stopRun`, `FabricInfos`, `FactoryReset` (30 s timeout) and `start`
(cmd nodes.go:229-254); `served.close` holds `s.mu` over `stopRun` and
`n.Stop` (nodes.go:327-333); `Fanout` holds `mu` over handler callbacks
(fanout.go:59-85).

### 7.3 Channels

| Channel | Capacity | Producer -> consumer | Full / closed behaviour | Where |
| --- | --- | --- | --- | --- |
| `Bridge.work` (`func(ctx)`) | 1024 | `enqueue`, `resyncSoon`, `Resync` -> `Run` | `enqueue` full: log `bridge.queue_full` and `resyncSoon` (bridge:214-221); timer: drop (321-324); `Resync` blocks until `ctx` (577-584) | bridge.go:128,174 |
| `Bridge.cmds` (`model.Command`) | 256 | `onCommand` -> `commandLoop` | full: `bridge.command_dropped`, `CommandErrs++` (command.go:26-31) | bridge.go:129,175 |
| `Bridge.synced` | unbuffered, closed once | `sync` -> `Synced()` readers | closed under `mu` (bridge:499-503) | bridge.go:159,179 |
| `session.pending[id]` (`Message`) | 1 each | `read` -> `callWith` | removed on result; closed by `fail` (client.go:363-366); ignored when no waiter (344-346) | ha/client.go:386,399 |
| `session.done` | closed once | `fail` -> `serve` loop, `callWith` | `fail` idempotent (356-360) | ha/client.go:190,210 |
| `Subscription.ch` (`json.RawMessage`) | 64 (`SubscriptionBuffer`) | `deliver` (read goroutine) -> `relay` | full: `deliver` false -> `ha.subscription_overflow`, `dropStream` with `ErrSubscriptionOverflow` (client.go:324-327); closed by `close` | ha/subscription.go:25,121 |
| `Host.ready` | closed once | `Attach` -> `signaller` | closed in `Attach` (camera.go:237-241) | camera.go:162,215 |
| `served.done` | closed on bridge exit | `br.Run` goroutine -> `stopRun` | nodes.go:123,125,146 | cmd nodes.go:123 |
| `webErr` | 1 | web start error or `ws.Run` result -> `run` | main:110,122,124,130 | main:110 |
| `waiter.ok` | closed once | `Connected` -> `dumpPlan` | main:227,251 | main:240 |
| `errc` | 1 | `srv.Serve` -> `Run` | web.go:117-124 | web.go:117 |
| `fabricui.Events` | go-fabric-ui's | `s.events.Publish` from `OnChange` (cmd nodes.go:113-120) | go-fabric-ui's (not read) | cmd nodes.go:83 |

### 7.4 Subscriptions

| Subscription | Created | Ended |
| --- | --- | --- |
| HA events, 4 types (`s.subs`) | each connect, `subscribe` (ha/client.go:215-220, 445-452) | session end (`fail`/connection close); no `unsubscribe_events` is sent for them |
| HA `camera/webrtc/offer` stream (`Subscription`) | `ProvideOffer` -> `Subscribe` (camera.go:322; subscription.go:104-127) | `Unsubscribe` (subscription.go:136-156) by `unsubscribe` on end, replace, `Host.Close`, `Attach` of a new signaller (camera.go:243-245,319-321,281-283), or session `fail` (client.go:367-370) |
| Bridge on the fan-out | `s.fan.Add(br)` (cmd nodes.go:130) | `s.fan.Remove(s.br)` (nodes.go:144) in `stopRun` |
| Web UI log observer | `Operations.ObserveLog` (operation.go:63-68) (called by go-fabric-ui) | the returned func (go-fabric-ui) |
| go-fabric `Node.Observe` | go-fabric-ui (not here; nodes.go:109-110 comment) | go-fabric-ui |
| Handlers `OnChange`, `OnDiscard` | `hp.OnChange(b.onCommand)`, `hp.OnDiscard(b.onDiscard)` (bridge.go:619-620) | `g.plan.Close()` (bridge.go:199,450) |

### 7.5 Ends in order (daemon stop)

`run` after `ctx.Done` or a web error: `cancel()` (main:137), `wg.Wait()`
(138), then `fl.Close` and `def.close` under a 10 s context (main:139-144).
Registry nodes are started before the HA client runs (main:101-109); the
cookbook order is stated in main:94-97.

## 8. Files written

| File | Mode | Directory | Writer | Notes |
| --- | --- | --- | --- | --- |
| node store `ha2fabric.db` (SQLite) | go-fabric's | `filepath.Dir(STORE_PATH)`: add-on `/data` (run.sh:24); container `/data` (Dockerfile:31); stand-alone `.` unless set (cfg:233-235) | go-fabric `node.Store{Path}` (node.go:65) | also `-journal`, `-wal`, `-shm` removed on delete (nodes:447) |
| `selection.json` | `os.CreateTemp` default 0600, then `os.Rename` | beside the store (cfg:241,58-59) | `selection.Store.write` (selection.go:173-201): temp `selection.json.tmp-*`, `Sync`, `Close`, `Rename`; temp removed on error (197) | written by `Set` from the web UI `Apply` (exposure.go:313) |
| `selection.json.corrupt-<unix>` | rename of the original (original's mode) | same | selection.go:95-98 | on parse failure or wrong version |
| `selection-<key>.json` | as `selection.json` | `reg.Path(...)` = the store's directory (cmd main:88, nodes.go:404) | `selection.Store` for a registry node | removed on `Delete` (nodes:447) |
| `selection-<key>.json.corrupt-*` | rename | same | selection.go:95-98 | removed by glob on `Delete` (nodes:448-453) |
| `nodes.json` | 0600 via `CreateTemp` + `Rename` | the store's directory (main:88) | `Registry.write` (nodes.go:459-487): `nodes.json.tmp-*`, `Sync`, `Close`, `Rename`, temp removed on error (483); contains `passcode` per node in clear (nodes.go:70-77) | on `Add`, `Rename`, `Remove` |
| `nodes.json.corrupt-<unix>` | rename | same | nodes.go:130-133 | on parse or check failure |
| `ha2fabric-<key>.db` (+ journal files) | go-fabric's | same | the registry node's `node.Store{Path}` (cmd nodes.go:402) | created by go-fabric at `Start`; erased and removed by `Delete` |
| `/data` volume | n/a | Dockerfile:25,29-30 (`mkdir -p /out/data`, `chown nonroot`) | build | the only writable path in the distroless image |

No `os.WriteFile`, `os.Create`, `os.OpenFile`, `os.MkdirAll`, `os.Chmod` in the
production Go (`grep`: only `os.CreateTemp` at selection.go:181 and
nodes.go:467). `STORE_PATH`'s directory must pre-exist (cfg:281-285).
`coverfloor` writes nothing (script/coverfloor/main.go).

## 9. Bounds

| Bound | Value | Where |
| --- | --- | --- |
| HA WebSocket frame read limit | 64 MiB | ha/client.go:52,126-128,202 |
| HA call timeout (dial, auth, ping, call, subscribe) | 30 s | ha/client.go:47,114-116,196,246,260,446,466 |
| HA ping interval | 30 s | ha/client.go:46,236 |
| HA reconnect backoff | 1 s doubling to 60 s | ha/client.go:48-49,150-171 |
| HA command (service call) timeout | 30 s | bridge/command.go:21,67 |
| Camera still size | 64000 bytes | ha/camera.go:87,168-174 |
| Camera still dimensions | 1..65535 each | camera.go:666 |
| Camera still content type | `image/jpeg` only | camera.go:659-665 |
| Camera `SignalTimeout` | 10 s | camera.go:208-209 |
| Camera signalling retries / interval | 10 / 50 ms | camera.go:202-207 |
| Camera sessions per host | keyed by controller `SessionID` (uint16); replacing an id ends the old (camera.go:313-321); no count limit here | camera.go:166,311-337 |
| Camera pending ICE candidates before HA session id | unbounded | camera.go:380 |
| Subscription buffer | 64 events | ha/subscription.go:25 |
| Bridge work queue | 1024 | bridge.go:174 |
| Bridge command queue | 256 | bridge.go:175 |
| Resync debounce | 1 s | bridge.go:165-167 |
| `NODE_LIMIT` | 0..32, default 8 | cfg:53-54,304-308; cmd nodes.go:463 |
| Registry key length | <= 64; slug <= 56 chars + `-n` suffix, `n` < 1000 | nodes.go:59,257,269-296 |
| Registry port search | `base+1 .. 65535` | nodes.go:328 |
| Registry discriminator / passcode draws | 1000 tries each | nodes.go:364,382 |
| Registry passcode range | 1..99999998 | nodes.go:378,387 |
| `LABEL` | 1..32 bytes | cfg:274-279; registry label cut at 32 bytes (cmd nodes.go:388-394) |
| `MDNS_INTERFACES`, `INCLUDE`, `EXCLUDE`, label lists, `WEB_ALLOW`, `WEB_ALLOWED_HOSTS` | no count or length limit | cfg:230,239-244,380-409 |
| `selection.json`, `nodes.json` | no size limit on read | selection.go:82; nodes.go:118 |
| Selection override name | no length check at `Apply` | exposure.go:242,303 |
| Web: read-header / read / idle timeout | 10 s / 30 s / 120 s | web.go:108-109 |
| Web: write timeout, header size, body size | not set here (go-fabric-ui's for the handler) | web.go:106-110 |
| Web: shutdown wait | 5 s | web.go:121 |
| Erase | 30 s | cmd nodes.go:77,226 |
| Daemon stop | 10 s | main:139 |
| Dump-plan | 30 s | main:238 |
| Light `brightness` | clamp 0..255 | command.go:325 |
| Light `rgb_color` | each 0..255 | command.go:345 |
| Light `color_temp_kelvin` | int of the rounded float; no upper bound | command.go:327-331 |
| Light `hs_color`, `xy_color` | floats unclamped | command.go:333-341 |
| Cover position / tilt, fan percentage | 0..100 | command.go:116,121,152 |
| Climate / water heater temperature | finite float, no range | command.go:131,137,204; 376-379 |
| Media volume | 0..`LevelMax`, scaled to 0..1 | command.go:216-220 |
| Climate mode, fan mode, preset mode, select option | non-empty trimmed string, unbounded | command.go:126,157,175,180 |
| `Tuple` identifiers | no length bound; `len==2` only | adapt.go:51,57 |
| Version for `SoftwareVersion` | each part <= 999 | version.go:62-65 |
| Matter session counts, message sizes | go-fabric's (not set here: `node.Config` carries no such field, node.go:41-67) | node.go:41-67 |
| Add-on option ranges | `port`, `passcode` 1..99999998, `discriminator` 0..4095, `vendor_id`/`product_id` 1..65535, `node_limit` 0..32 | addon:65-85 |

## 10. Build and deploy facts used above

| Fact | Where |
| --- | --- |
| Container image runs distroless `nonroot`, entrypoint `/ha2fabric`, exposes 5540/udp, 8099/tcp | Dockerfile:27,32-34 |
| Container sets `WEB_LISTEN=0.0.0.0:8099`; no `WEB_ALLOW` or `WEB_ADMIN` | Dockerfile:31 |
| Add-on image: HA base, binary `/usr/bin/ha2fabric`, `/run.sh`; no `USER` line | addonDF:35-44 |
| Add-on `host_network: true`, `homeassistant_api: true`, `ingress: true`, `ingress_port: 8486`, `backup: cold`, `init: false` | addon:26,31,37,41,43-44 |
| Add-on arch: amd64, aarch64, armv7 | addon:13-16 |
| `run.sh` ends with `exec /usr/bin/ha2fabric` (PID 1) | run.sh:75 |
| `run.sh` `set -e` | run.sh:11 |
| Add-on version `0.7.0` | addon:7 |
| Both Dockerfiles `CGO_ENABLED=0`, `-trimpath`, Go `1.27.4-alpine` build stage | Dockerfile:10,18-19; addonDF:16,24-25 |
