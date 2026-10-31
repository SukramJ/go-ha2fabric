# Read map R6 — go-ha2fabric: the WebSocket client's subscriptions, the bridge's plan application, config and web (Sonnet read, 2026-10-31; a map: file:line for every fact, no judgement; verify before acting)

Source: /Users/markus/Documents/GitHub/go-ha2fabric, branch chore/go-fabric-0.15.5 (clean; main plus 2 commits per the session's git status). go.mod: go-fabric v0.15.5 (go.mod:6), go-fabric-ui v0.2.8 (:7), go-hamapping v0.3.0 (:9). Layout follows `notes/reads/read-r0-ha2fabric-pipeline.md` (sections 2, 3, 5 extended; not repeated). All paths relative to the repo root.

## 1. internal/ha client (Q1) — internal/ha/client.go (515 lines), message.go (334 lines)

### 1.1 Request and result matching (the pending map)
- `session` struct: `ws`, `writeMu`, `nextID int64`, `mu`, `pending map[int64]chan Message`, `subs map[int64]bool`, `done`, `err` (client.go:166-177). Created per connection in `serve` (client.go:195).
- `(*session).call(ctx, cmd map[string]any, sub bool) (Message, error)` (client.go:352): makes a `chan Message` (cap 1, :353), takes `writeMu`, `nextID++`, sets `cmd["id"]` (:355-358), registers `s.pending[id] = ch` (:366) and, if `sub`, `s.subs[id] = true` (:367-369), marshals and writes (:371-373), waits on `ch` / `ctx.Done()` (:386-407). On `ctx.Done()` the pending entry is deleted (:398-400); no unsubscribe is sent there.
- Result path: the reader `(*Client).read` (client.go:289) decodes frames with `Decode` (:296); for each message `switch m.Type`: `case TypeEvent` (:305), `default` (:314) looks up `s.pending[m.ID]`, deletes it, sends the message on the channel (:317-323). So only the first non-event message per id is delivered; the pending entry is removed on delivery.
- Result evaluation in `call`: `m.Type == TypePong` returns (:389); `m.Success != nil && !*m.Success` returns `m.Error` or "ha: command failed" (:392-397); otherwise `m` (:398-ish, see :386-398).
- Public entry: `(*Client).Call(ctx, cmd, out any) error` (client.go:420): takes `c.cur` session under `c.mu` (:421-424), `ErrNotConnected` if nil (:425-427), applies `CallTimeout` if ctx has no deadline (:428-432), `s.call(ctx, cmd, false)` (:433) — `sub` is always false from `Call`; decodes `m.Result` into `out` (:437-443). There is no exported method that passes `sub=true`; `subscribe` (client.go:408) is the only caller with `sub=true` (:413).
- Message envelope: `Message{ID, Type, Success *bool, Result, Error *Error, Event json.RawMessage, HAVersion, Message}` (message.go:47-57). `Event{EventType, Data json.RawMessage, Origin, TimeFired}` (message.go:112-117).
- Message type constants: auth_required, auth, auth_ok, auth_invalid, result, event, ping, pong (message.go:19-28). `Decode(b)` returns `[]Message` (a frame may hold an array; used at client.go:281, :296).

### 1.2 Subscriptions and event delivery
- Subscription command: `subscribe` sends `{"type":"subscribe_events","event_type":<et>}` with `sub=true` (client.go:408-416; citation of commands.py:180-219 in its comment :405-407).
- Which event types: `Options.Events []string`, "subscribed after every authentication" (client.go:59-60); `serve` loops over `c.opts.Events` calling `subscribe` (client.go:206-211). main.go passes `events` (cmd/ha2fabric/main.go:65; the list itself defined elsewhere in main.go — not read here). Test env lists `state_changed`, `entity_registry_updated`, `device_registry_updated`, `label_registry_updated` (internal/bridge/bridge_e2e_test.go:70-71). Constants message.go:36-41.
- Dispatch switch (the event branch), client.go:305-322: `case TypeEvent:` takes `ours := s.subs[m.ID]` under `s.mu` (:306-308); if not ours -> `continue` (the event is dropped silently, :309-311); else `json.Unmarshal(m.Event, &ev)` into `ha.Event` (:313-318; failure logs `ha.event_refused` and continues); then `h.Event(ev)` (:319).
- Delivery type: the `Handler` interface — `Connected(haVersion string)`, `Event(ev Event)`, `Disconnected(err error)` (client.go:67-77); "Every method runs on the client's goroutines and must not block" (:65-66). Single handler for the whole client, passed to `Run(ctx, h)` (client.go:135).
- Per-id event channel: NOT found. Events of every subscribed id go to the one `Handler.Event`, keyed by decoded `Event.EventType` only; the id is not passed to the handler (client.go:319, `h.Event(ev)` takes no id). The decode target is `ha.Event` with fields `event_type`/`data`/`origin`/`time_fired` (message.go:112-117); a payload whose `event` object has other fields (e.g. a different shape) would decode into those four fields only (json.Unmarshal; unknown fields dropped) — what the shape of `camera/webrtc/offer` events is: not read from HA in this task (not found in this repo; docs/roadmap.md:239-241 only names the commands).
- `s.subs` (map id -> bool) is the only per-id record; set only via `call(..., sub=true)` (client.go:367-369); never cleared on a result or cancel (no `delete(s.subs` in client.go — grep of `delete(s.subs` returns nothing in client.go; verified by the sed read of :289-407).

### 1.3 unsubscribe_events
- Not found. `grep -rn 'unsubscribe' internal/ha cmd` returned no matches in non-test files (and none in the sources read). The only subscription teardown is the session's end: `fail` closes `done`, fails pending, `ws.CloseNow()` (client.go:330-349).

### 1.4 Reconnect behaviour
- `Run` (client.go:135-158): loop `c.serve(ctx, h)`; on return `h.Disconnected(err)` (:142), logs `ha.disconnected` with `retry_in` (:146), waits `backoff` (:147-153); backoff starts `MinBackoff`, resets to MinBackoff after a connection that authenticated (:143-145), doubles up to `MaxBackoff` otherwise (:154-156). Defaults 1s/1min (client.go:39-42).
- Each `serve` creates a fresh `session` with empty `pending` and `subs` (client.go:195) and re-subscribes only `c.opts.Events` (:206-211). Any other subscription (none exists in this repo's code) is not carried over: `subs` is per session, so ids from the old session do not exist in the new one.
- On drop: `session.fail` closes every pending channel (`close(ch)`, client.go:340-343) so a waiting `call` returns `ErrDisconnected` (:387-389 `!ok`); `c.cur` is cleared and `conn` false in the deferred block of `serve` (:212-220); `Call` then returns `ErrNotConnected` (:425-427).
- After re-auth and subscriptions: `h.Connected(version)` (client.go:211-ish `h.Connected(version)`), the bridge responds with a full `sync` (internal/bridge/bridge.go:235-240). Disconnected marks every device unreachable (bridge.go:243-257).
- Ping keepalive: ticker `PingInterval` (default 30s, client.go:38) sending `{"type":"ping"}` via `s.call(..., false)` (client.go:226-236).

### 1.5 call_service path
- `(*Client).CallService(ctx, sc ServiceCall)` (client.go:506-515): builds `{"type":"call_service","domain","service"}`, `target.entity_id` if set, `service_data` if `len(sc.Data) > 0` (:507-513), `c.Call(ctx, cmd, nil)` (:514).
- Bridge side: `Upstream` interface includes `CallService` (internal/bridge/bridge.go:61). `onCommand` queues a `model.Command` on `b.cmds` (internal/bridge/command.go:23-31); `commandLoop` (:34-43) -> `command` (:45-): `adapt.Service(c, cur)` (:52) -> `b.up.CallService(cctx, sc)` with `commandTimeout` 30s (command.go:19, :64). On failure the entity's state is re-applied (command.go:75-80).

### 1.6 HTTP side, base URL and token
- The client holds only `Options.URL` (a ws:// or wss:// WebSocket endpoint) and `Options.Token` (client.go:51-58). No `net/http` import in internal/ha non-test files (client.go imports: context, encoding/json, errors, fmt, log/slog, sync, sync/atomic, time, coder/websocket, :16-27); no HTTP base URL field; no `camera_proxy` string anywhere in internal/ha, internal/config, cmd (grep returned nothing). An HTTP GET helper: not found.
- Token use: only `json.Marshal(map[string]string{"type": TypeAuth, "access_token": c.opts.Token})` in `auth` (client.go:254). Package doc: "held in memory only: it is sent in the auth message and never logged" (client.go:10-12); `Options.Token` comment "It is never logged." (client.go:57).
- Where the token lives in config: `Config.HAToken` ("HA_TOKEN, else SUPERVISOR_TOKEN. It is never logged or persisted", internal/config/config.go:64-66); `SupervisorTokenEnv = "SUPERVISOR_TOKEN"` (config.go:33); set at config.go:184-189 (`get("HA_TOKEN")`, fallback to `supervisor`); required outside the add-on (config.go:217-219). Package comment: token "read from the environment only — a flag ..." (config.go:7; the rest of that sentence not read).
- How it reaches the client: `ha.New(ha.Options{URL: cfg.HAURL, Token: cfg.HAToken, Events: events, Logger: logger})` (cmd/ha2fabric/main.go:65). `HAURL` defaults to `SupervisorURL = "ws://supervisor/core/websocket"` in an add-on (config.go:30, :191-194).
- No flag carries the token (flags listed config.go:~135-155 include no token flag; the `str` helper reads env keys; `ha-url` flag exists, no `ha-token` flag).
- Logging of connection data: `ha.connected` logs `ha_version` and `url` (client.go:210); main logs `ha_url` (cmd/ha2fabric/main.go:74). Neither logs the token. Test that fixes it: internal/ha/client_test.go:189-190 (`strings.Contains(logs.String(), hatest.Token)` -> "the token reached the log"); and :224 "the token is in the error" for a wrong token (assert at :221-225). Redaction code (a masking function): not found; the guard is absence of logging.
- `auth_invalid` error includes `reply.Message` (client.go:~268, `fmt.Errorf("%w: %s", ErrAuthInvalid, reply.Message)`), not the token.

## 2. hatest (Q2) — internal/ha/hatest/hatest.go (582 lines)

- Fixture load: `Fixtures()` is `testdata/ha` relative to the file via `runtime.Caller` (hatest.go:71-76); `Fixture(tb, name)` reads the file, requires a `_mirrors` citation key (fatal "no _mirrors citation"), returns `result` else `message` (hatest.go:79-100). `New(tb)` decodes `entity_registry_list.json`, `device_registry_list.json`, `area_registry_list.json`, `label_registry_list.json`, `get_states.json`, `get_config.json`, plus `expose_entity_list.json` (hatest.go:113-130); starts `httptest.NewServer` (:131); cleanup `Down`+`Close` (:132-135).
- Files in testdata/ha: area_registry_list.json, auth_invalid.json, auth_ok.json, auth_required.json, device_registry_list.json, entity_registry_list.json, expose_entity_list.json, get_config.json, get_states.json, label_registry_list.json (ls). `grep camera testdata/ha/*.json` — no match (no camera entity in the fixtures).
- Token the fake accepts: `const Token = "hatest-token"` (hatest.go:33-34); handshake checks `auth.AccessToken != Token` -> `auth_invalid` (hatest.go:220-243).
- URL: `ws://.../api/websocket` (hatest.go:137).
- `case "..."` strings of `command` (hatest.go:244-): "ping" (:249), "config/entity_registry/list" (:253), "config/device_registry/list" (:255), "config/area_registry/list" (:257), "config/label_registry/list" (:259), "get_states" (:261), "get_config" (:263), "homeassistant/expose_entity/list" (:265), "subscribe_events" (:267), "call_service" (:272). `default:` answers `{"success":false,"error":{"code":"unknown_command","message":"Unknown command."}}` (hatest.go:292-298 region; the default block follows the call_service case). So `camera/webrtc/*` and `unsubscribe_events` would receive `unknown_command`.
- subscribe_events record: `c.subs[et] = append(c.subs[et], id)` where `c.subs map[string][]int64` is keyed by event type (hatest.go:267-271, field :67); the success result is `result: nil`-valued generic (falls to the common reply at the end of `command`, :301-304).
- Scripted `event` push: `(*Server).fire(eventType string, data any)` (hatest.go:338-359, unexported): builds the event object (`event_type`, `data`, `origin`, `time_fired`, `context`) and sends `{"id": id, "type":"event","event": ev}` to every id in `c.subs[eventType]` on every connection. The push is keyed by event type, not by an arbitrary id; no exported method to send an event to a given subscription id and no way to push an event shaped other than that object: not found. Exported mutators that call `fire` internally: `SetState` (:362), `UpdateEntity` (:404), `UpdateDevice` (:423), `RenameEntity` (:442), `UpdateLabel` (:468), `RemoveLabel` (:483), `SetExposed` (:500), `RemoveEntity` (:516), `RestoreEntity` (:544); other exported: `URL` (:137), `Authenticated` (:140), `Down` (:148), `Up` (:162), `State` (:390), `Calls` (:559), `WaitCall` (:566). (Whether each of those calls `fire` was read for `RestoreEntity` at :555 only; the others not individually verified.)
- call_service recording: `sc := serviceCall(cmd)` (domain, service, `service_data`, `target.entity_id`; hatest.go:307-316); appended to `s.calls` (:273-274); non-blocking send to `callCh` (cap 64, :122; send :277-281); hooks `OnCall func(ha.ServiceCall)` (:57-59) and `Fail func(ha.ServiceCall) *ha.Error` (:61) run after recording; success reply `{"id","type":"result","success":true,"result":{"context":{...}}}` (hatest.go:~288-292).
- Assertion API: `Calls() []ha.ServiceCall` (:559-564), `WaitCall(ctx, match)` (:566-582): scans recorded calls first, then reads `callCh` until match or ctx done.
- A test cannot reach HTTP: the fake serves only the WebSocket handler (`httptest.NewServer(http.HandlerFunc(s.serve))`, hatest.go:131); handlers for other paths (e.g. `/api/camera_proxy/...`): not found. `s.serve` (hatest.go:168) — path-handling inside not read.

### 2.1 Bridge e2e pattern (internal/bridge/bridge_r5_e2e_test.go, 254 lines; harness internal/bridge/bridge_e2e_test.go)
- Harness `newEnv(t, rule, edit ...func(*bridge.Options))` (bridge_e2e_test.go:54-103): `hatest.New(t)` (:58), `fabrictest.NewSite(t, fabrictest.Options{Clock: clock.System{}, Logger: logger()})` (:59), `site.AddNode(bridge.NodeConfig(&config.Config{...}, logger()))` (:65-67), `ha.New(ha.Options{URL: srv.URL(), Token: hatest.Token, Events: [...4 events], MinBackoff 20ms, MaxBackoff 100ms})` (:68-73), `bridge.New(dev.Node(), cl, opts)` (:78), `go br.Run`, `go cl.Run(rctx, br)` (:80-82), wait `<-br.Synced()` (:90-94). `e.commission()` creates a controller and pairs (bridge_e2e_test.go:105-113). Note: the node is the site's `dev.Node()` (comment bridge_e2e_test.go:38-42).
- R5 test: fixture-load is implicit in `newEnv` (hatest.New decodes the fixtures); `e := newEnv(t, expose.Rule{Assist: true})` and `e.commission()` (bridge_r5_e2e_test.go:63-64); endpoints by `e.mustEndpoint("select.hall_scene")` etc. (:66-72); controller drive: `fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(sel, modeselectdef.ClusterID, modeselectdef.CmdChangeToMode), ...)` (:148), `fabrictest.Write(...)` (:221), `fabrictest.ReadCluster` (:75-112), `fabrictest.Subscribe` (:122-134); state push: `e.ha.SetState("climate.bedroom_ac", "cool", e.attrsWith(...))` (:215-216 region, line :214 in the read slice); service assertion: helper `e.waitCall(entityID, service)` using `e.ha.WaitCall(e.ctx, func(c ha.ServiceCall) bool {...})` (:49-57), and inline `e.ha.WaitCall(...)` matching `c.EntityID`, `c.Service`, `c.Data["volume_level"] == 0.5` (:206-211).
- The test imports `fabrictest`, `controller/client`, `cluster/spec` (r5_e2e_test.go:9-18); allowed for tests by the arch guard (section 6).

## 3. internal/bridge (Q3)

### 3.1 Files and sizes
bridge.go (1012), node.go (91), command.go (80), provider.go (286); tests bridge_test.go (175), bridge_e2e_test.go (282), bridge_f2_e2e_test.go (102), bridge_h3_e2e_test.go (414), bridge_r5_e2e_test.go (254).

### 3.2 Call chain: Map result -> go-fabric declarations
- `Read(ctx, up, rule)` collects the snapshot (bridge.go:394); `assess(snap, rule, names)` (bridge.go:438) decides exposure with `rule.Decide(e, labels)` (bridge.go:485 region, line :485 `d := rule.Decide(e, labels)`) and groups entities per HA device into `planned` (bridge.go:423-436 type).
- `sync(ctx)` (bridge.go:633): `Read` -> `assess` -> for each planned group: reuse if `old.sig == p.sig` (:663-667) else `b.build(ctx, p)` (:668).
- `build` (bridge.go:939-965): `hamapping.Map(p.device, p.entities, mapOptions(p.regs, p.names))` (:940) returns `(plan, verdicts)`; `devs, err := plan.Build(ctx)` (:945) returns `[]*device.Device`; on error all verdicts become `eligibility.StateUnmappable` with reason "go-fabric refused the device: ..." (:946-957); `g.devices = devs`, `g.keys` from `plan.Endpoints[i].Options.StableKey` (:959-961); `plan.OnChange(b.onCommand)` registers the command callback (:962).
- Node attachment: `b.n.Aggregator().Batch(ctx, func(bt *node.Batch) error {...})` (bridge.go:715) with `bt.Remove(k)`, `bt.Forget(k)`, `bt.Replace(k, dev)`, `bt.Add(dev)` (bridge.go:708-713 `drop`; :724-746 Replace/Add; :752 Add for `added`). Types used: `node.Node`, `node.Batch`, `device.Device`, `device.SourceKey` (imports bridge.go, node.go:11).
- `Options` for `Map` (`mapOptions`, bridge.go:593-608); `Dry` (bridge.go:610-620) runs `hamapping.Map` only.

### 3.3 State into cluster attributes
- `apply(ctx, g, id)` (bridge.go:967-985): reads `b.states[id]`, `g.plan.Apply(ctx, id, adapt.State(st))` (:979); per-result errors logged at Debug `bridge.apply` (:979-983).
- `stateChanged` (bridge.go:987-1008): updates `b.states` (:988-994); `resyncSoon` if the entity is removed/unknown (:995-998) or its description changed (:1003-1005); then `apply`.
- `Event` (bridge.go:259-296): `state_changed` -> `b.enqueue(stateChanged)`; `entity_registry_updated` -> `noteRemoved` for action "remove" and `resyncSoon`; `device_registry_updated` / `label_registry_updated` -> `resyncSoon` (:290-294).
- After each sync: `b.apply` for every entity of every group (bridge.go:772-781 region, "Every state onto its device").

### 3.4 Commands back
- `plan.OnChange(b.onCommand)` (bridge.go:962); `onCommand` queues on `b.cmds` cap 256 (bridge.go:146 field; command.go:23-31); `commandLoop` + `command` (command.go:34-80) call `adapt.Service(c, cur)` -> `b.up.CallService`.
- `internal/adapt/command.go` first 60 lines: `ErrCommand` (:19); `payloads` table keyed per domain `{key, service}` for `switch`, `fan`, `cover`, `lock` (:26-40); `services` table: named-method map `map[string]map[string]string` starting at `"button": {hamapping.MethodPress: "press"}` (:45-56) then `climate`, `vacuum` (:56-60). Domain list of the rest of the table: not read here (past line 60). Comments cite `services.yaml` / `services.py` lines at 2026.10.3 per entry. The file imports `keypadinputdef`, `levelcontroldef` from `go-fabric/cluster/spec/...` (adapt/command.go:12-13).
- No `camera` domain key within the first 60 lines; `grep -n -i camera internal/adapt/*.go internal/bridge/*.go internal/expose/*.go` produced no match.

### 3.5 Lifecycle
- `New(n *node.Node, up Upstream, opts Options) *Bridge` — "The node may be started or not; Run serves it" (bridge.go:173-195).
- `Run(ctx)` (bridge.go:197-219): `commandLoop` in a goroutine (:199-200); worker loop executing `b.work` closures (:205-216); on ctx.Done stops the timer and `g.plan.Close()` for each group (:204-213); returns nil.
- `enqueue` (bridge.go:221-229): non-blocking, `bridge.queue_full` log and `resyncSoon` when full (cap 1024, bridge.go:182).
- `Connected` (bridge.go:235-240): sets status, enqueues `sync`. `Disconnected` (bridge.go:243-257): sets every device `SetReachable(ctx, false)`.
- `resyncSoon` debounced timer (bridge.go:337-349; default Debounce 1s, bridge.go:176-178). `Resync(ctx)` (bridge.go:913) is the web UI's Reassemble hook (cmd/ha2fabric/main.go:117).
- Endpoint add/remove: only through `sync`'s Batch (bridge.go:715-760): removed groups -> `drop`; replaced -> `Replace`/`Add`/`drop`; new -> `Add`; closing plans `g.plan.Close()` (bridge.go:763-765).
- main.go order: `node.New(bridge.NodeConfig(cfg, logger))` (cmd/ha2fabric/main.go:81), `n.Start(ctx)` (:86), `LogFabrics`, `LogPairing` (:91-92), `bridge.New(n, client, ...)` (:93), `br.Run` and `client.Run(rctx, br)` goroutines (:106-107), web (:109-114), `n.Stop(sctx)` after `wg.Wait()` (:128-132).
- What touches `node.Node` after `Start`: `b.n.Aggregator().Batch(...)` (bridge.go:715) at every sync, i.e. after `n.Start`; `LogFabrics` calls `n.FabricInfos(ctx)` (internal/bridge/node.go:~58), `LogPairing` calls `n.PairingCodes()` (node.go:~79); the web UI receives `Node: n` (cmd/ha2fabric/main.go:112-115). No other `node.Node` method call found in internal/bridge non-test files (grep `n\.[A-Z]` outside `b.n.Aggregator` returned only json/plan hits). Any call that attaches a camera peer or other non-aggregator thing to the node: not found.
- `NodeConfig(c *config.Config, logger)` builds `node.Config` with Network, Commissioning, ProductDescription, BasicInformation, Store, Health, Logger (internal/bridge/node.go:32-54). No camera-related field in it.

## 4. internal/config (Q4) — config.go

### 4.1 Options: env var, flag, add-on key (add-on key from addon/config.yaml `options`, :48-60; schema :62-79; mapping from script/run.sh)
| Config field | Env / flag (config.go) | Default | Add-on option (schema type) | run.sh mapping |
| --- | --- | --- | --- | --- |
| HAURL | HA_URL / --ha-url (config.go:~137) | `SupervisorURL` in add-on (:191-194) | none | not set; daemon defaults from SUPERVISOR_TOKEN |
| HAToken | HA_TOKEN, else SUPERVISOR_TOKEN (:186-189) | - | none (SUPERVISOR_TOKEN env from with-contenv, run.sh:8) | - |
| Listen | LISTEN / --listen | ":5540" (:37 DefaultListen) | `port` (port) | `LISTEN=":$(bashio::config 'port')"` run.sh:16 |
| Passcode | PASSCODE | 20202021 | `passcode` int(1,99999998) | run.sh:17 |
| Discriminator | DISCRIMINATOR | 3840 | `discriminator` int(0,4095) | run.sh:18 |
| VendorID | VENDOR_ID | 0xFFF1 | `vendor_id` int(1,65535) | run.sh:19 |
| ProductID | PRODUCT_ID | 0x8001 | `product_id` int(1,65535) | run.sh:20 |
| Label | LABEL | "Matter Bridge" | `label` str | run.sh:21 |
| StorePath | STORE_PATH | "ha2fabric.db"; "/data/ha2fabric.db" in add-on (:196-201) | none | run.sh:24 fixed `/data/ha2fabric.db` |
| Include / Exclude | INCLUDE / EXCLUDE | empty | `include`, `exclude` (list str) | run.sh:30-39, `bashio::config.has_value` guard |
| IncludeLabels / ExcludeLabels | INCLUDE_LABELS / EXCLUDE_LABELS | empty | `include_labels`, `exclude_labels` (list str) | run.sh:40-49 |
| ExposeAssist | EXPOSE_ASSIST (string "true") | true | `expose_assist` bool | run.sh:27 |
| LogLevel | LOG_LEVEL | info | `log_level` list(debug\|info\|warn\|error) | run.sh:51 |
| WebListen | WEB_LISTEN | "127.0.0.1:8099" (:43) | none | run.sh:56 `0.0.0.0:8486` fixed |
| WebAllow | WEB_ALLOW | empty | none | run.sh:57 `172.30.32.2` fixed |
| WebAdmin | WEB_ADMIN (string "false") | false | none (add-on ignores it, config.go:98-100 comment) | not set |
| SelectionPath | derived: `selection.json` beside the store (config.go:~203) | - | none | - |
| DumpPlan / Version | flags `--dump-plan`, `--version` only | false | none | - |
Flag/env definitions: config.go lines of `str(...)` calls are in the block starting `haURL := str("ha-url", "HA_URL", ...)`; `fs.Bool` for the two flags (config.go:~139-158 region; exact line numbers of each call were not individually printed — the block is at config.go:~139-158 of the slice read; unverified per-line).
- `addon/config.yaml` has no `camera`-related option. Add-on `options:` block at addon/config.yaml:48-60; `schema:` at :62-79.

### 4.2 Pattern for a boolean option
- Config string flag with default string `"true"`/`"false"` via `str(...)`: `assist := str("expose-assist", "EXPOSE_ASSIST", "true", ...)` and `webAdmin := str("web-admin", "WEB_ADMIN", "false", ...)` (config.go, both in the `str` block); parsed with `strconv.ParseBool`, error text `"<ENV> must be true or false, got %q"` (config.go:~244-255: `strconv.ParseBool(*assist)` -> `ExposeAssist`; `strconv.ParseBool(*webAdmin)` -> `WebAdmin`). Struct field bool (config.go:~76 `ExposeAssist bool`, :~100 `WebAdmin bool`).
- Add-on: schema `expose_assist: bool` (addon/config.yaml:68), options `expose_assist: true` (:55); run.sh `export EXPOSE_ASSIST="$(bashio::config 'expose_assist')"` (script/run.sh:27; bashio prints true/false).
- Tests pass env via `MapEnv(map[string]string{...})` (config.go:~118-125; config_test.go:17, :30).

### 4.3 Add-on option -> env mapping
- `script/run.sh` (80 lines? file read through line 61): shebang `#!/usr/bin/with-contenv bashio` (run.sh:1); `bashio::config '<key>'` per option, `export`, then `exec /usr/bin/ha2fabric` (run.sh:61). The add-on image copies it as `/run.sh` and `CMD ["/run.sh"]` (addon/Dockerfile:`COPY script/run.sh /run.sh`, `CMD`). `init: false` (addon/config.yaml:~34) ("The HA base image ships s6-overlay as its init"); no s6 service directory (no `run` file under addon/rootfs; `find addon -name 'run*'` returned nothing).
- Add-on manifest flags: `homeassistant_api: true` (addon/config.yaml:~39), `host_network: true`, `ingress: true`, `ingress_port: 8486`, `backup: cold`, `panel_title: Matter Server` (addon/config.yaml:~42-57; exact lines approximate).

## 5. internal/expose (Q5) — internal/expose/expose.go (208 lines)

- The rule is not by domain. Types: `Rule{Assist, Include, Exclude []string, IncludeLabels, ExcludeLabels []string, Overrides map[string]bool}` (expose.go:30), `Entity{EntityID, Registered, DisabledBy, HiddenBy, ExposedToAssist, Labels}`, `Decision{Exposed, Reason, Source}`, source constants `disabled`, `hidden`, `override:on`, `override:off`, `rule:exclude`, `rule:include`, `rule:label`, `rule:assist` (expose.go, const block above `Decision`).
- `(Rule).Decide(e, l)` (expose.go:135): order: disabled -> hidden -> web-UI override -> exclude glob -> exclude label -> include glob -> include label -> `Assist && ExposedToAssist` -> not exposed. `match` uses `path.Match` globs on the entity id (e.g. `camera.*` would be a glob; no domain table).
- Domain list / table in `internal/expose`: not found (`grep -n 'domain\|Domain\|camera' internal/expose/expose.go` returned nothing).
- Camera: no domain table that includes or excludes it. Whether a `camera.*` entity is exposed is decided by the same rule (e.g. exposed-to-Assist from `homeassistant/expose_entity/list`) and then by go-hamapping's `Map` verdict (internal/bridge/bridge.go:940, verdict states `eligibility.StateUnmappable`, :946-957). Test fixing camera behaviour: not found (`grep -i camera` over internal/expose/expose_test.go, internal/bridge/*_test.go, internal/adapt/*_test.go — no match; testdata/ha/*.json — no match). What go-hamapping v0.3.0 does with a `camera` entity: not read in this task (docs/roadmap.md:246 says go-hamqtt's model has no camera with a stream; not verified here).
- Labels: `IncludeLabels`/`ExcludeLabels` resolved by `Labels.Resolve` (case-insensitive name or id); a device's labels apply to its entities (bridge.go:476-484 region). Overrides: `Rule.Overrides` from the web UI's `selection.Store.Enabled()` (internal/bridge/bridge.go:826-833 `rule()`, `r.Overrides = b.opts.Selection.Enabled()`).
- `Split(s)` splits on comma, space, newline, tab (expose.go:199).

## 6. internal/arch/arch_test.go (Q6)

Constants: `module = "github.com/SukramJ/go-ha2fabric"`, `goFabric = "github.com/SukramJ/go-fabric/"` (arch_test.go:20-23).
- go-fabric allowed: `var facade = map[string]bool{"node": true, "device": true, "contract": true, "eligibility": true}` (arch_test.go:25-26); `isDef(rel)`: `strings.HasPrefix(rel, "cluster/spec/") && !strings.Contains(strings.TrimPrefix(rel, "cluster/spec/"), "/")` i.e. exactly one level under `cluster/spec/` (arch_test.go:~82-84). Tests additionally: `var testOnly = map[string]bool{"fabrictest": true, "controller/client": true, "clock": true, "cluster/spec": true}` (arch_test.go:~32-33). A go-fabric package outside these (e.g. `cluster/camera`, `bridge`, `endpoint`, `im`, `tlv`, `mdns`) is reported "below go-fabric's facade"; `TestGuardRefuses` lists refused deps `cluster/spec` (non-test), `cluster/spec/onoff/internal`, `bridge`, `endpoint`, `im`, `tlv`, `mdns`, `fabrictest`/`controller/client` (non-test), `controller`, `secure/setup` (test) (arch_test.go:~142-160).
- Layers (arch_test.go:~36-48), each package -> module packages it may import: `internal/version` {}; `internal/expose` {}; `internal/selection` {}; `internal/ha` {}; `internal/ha/hatest` {internal/ha}; `internal/adapt` {internal/ha}; `internal/config` {internal/expose}; `internal/bridge` {internal/ha, internal/adapt, internal/expose, internal/config, internal/selection, internal/version}; `internal/web` {}; `internal/arch` {}; `cmd/ha2fabric` {internal/bridge, internal/config, internal/ha, internal/selection, internal/version, internal/web}. `testLayers`: `internal/ha` + `internal/ha/hatest`; `internal/bridge` + `internal/ha/hatest`; `cmd/ha2fabric` + `internal/ha/hatest`. A package without a layer entry (other than root `.`) fails: "package %s has no layer entry".
- Third party allowed (arch_test.go thirdParty): `github.com/SukramJ/go-hamapping`, `github.com/SukramJ/go-hamqtt/model`, `github.com/SukramJ/go-ha-catalog`, `github.com/coder/websocket`, `github.com/SukramJ/go-fabric-ui/fabricui`; any other import whose first path element contains a dot is refused (arch_test.go case `strings.Contains(strings.Split(p, "/")[0], ".")`). Standard library: not checked by the guard (no rule on `net/http`).
- Which packages may import go-hamapping: the guard has no per-package rule on third-party imports; any package may import an allowed third-party (go-hamapping is imported by internal/adapt (adapt/command.go:16), internal/bridge (bridge.go)). Which packages import go-fabric today: internal/bridge, internal/adapt (cluster/spec defs), cmd/ha2fabric (node), internal/config (node, config.go imports node.ValidateSetupPIN).
- Layer note: `internal/ha` may import no module package; `internal/web` {}.
- Quote of the header comment: "go-fabric is used through its facade only — node, device, contract, eligibility and the <cluster>def packages under cluster/spec/ — and the daemon's own packages keep their layers." (arch_test.go:5-8).

## 7. Guards (Q7)

- `script/coverfloor/floors.txt` (9 floors; header says "A ratchet"): cmd/ha2fabric 75; internal/adapt 95; internal/bridge 82; internal/config 92; internal/expose 95; internal/ha 80; internal/selection 90; internal/version 75; internal/web 85. No floor for internal/arch or internal/ha/hatest. A new package would need a line: whether the tool fails for a package without a floor is not read (script/coverfloor source not read).
- Makefile targets (line): `build` (43-45: `CGO_ENABLED=0 go build ./...` and the daemon to bin/ with ldflags); `vet` (47); `test` (50-51: `CGO_ENABLED=0 go test -count=1 -timeout 300s ./...`); `race` (53-54: `CGO_ENABLED=1 go test -race -count=1 -timeout 600s ./...`); `fmt` (56); `fmt-check` (59); `lint` (62, deps fmt-check; gofumpt check + golangci-lint + go mod tidy -diff + go mod verify per its help text); `tidy-check` (68); `cover` (72-73: `-coverpkg=./... -coverprofile=coverage.out`); `cover-check` (75-76, deps cover: `go run ./script/coverfloor coverage.out`); `fuzz-smoke` (78-79: runs only `-fuzz '^FuzzDecode$$'` `-fuzztime $(FUZZTIME)` on `./internal/ha/`; FUZZTIME default `10000x`, Makefile:24); `vuln` (81); `licenses` (88); `secrets` (92-93: `go run github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION) git --no-banner --redact .`); `docker` (95); `addon-image` (98); `version-check` (101-105); `release` (107, deps version-check); `ci: build vet test lint` (124).
- `version-check` (Makefile:101-105): compares `RELEASE_VERSION` (awk of `Version = "..."` in `internal/version/version.go`, Makefile:18; currently "0.4.0", version.go:20) with `version:` in `addon/config.yaml` (awk, first `version:` line; currently "0.4.0", addon/config.yaml:7), and requires `grep -q "^## \[$v\]" changelog.md`. Three files: internal/version/version.go, addon/config.yaml, changelog.md. (addon/CHANGELOG.md exists too, addon/ listing; not checked by the target.)

## 8. Web (extra, from Q-list context "config and web")
- internal/web/web.go: `web.New(Options{Listen, Allow, Logger, UI fabricui.Options})` (web.go:48), `Handler()` (:66), client filter `filter` (:74), `Run(ctx)` (:90), `From(ip)` (:124), `ReadOnly` (:135). main passes `fabricui.Options{Node: n, Exposure: br.Exposure(), Admin: admin(cfg), Reassemble: br.Resync, Events: uiEvents, Warnings: br.Warnings, Logger}` (cmd/ha2fabric/main.go:110-114). Any camera/stream handler on this server: not found (file bodies beyond the signatures not read).


## 9. Line corrections (grep -n, measured after the draft; these override the earlier cites)
- internal/ha/client.go: `type Handler interface` :79, `Event(ev Event)` :84; subscribe loop `for _, et := range c.opts.Events` :200; `h.Connected(version)` :211; auth `json.Marshal(... "access_token": c.opts.Token)` :254; dispatch `case TypeEvent:` :303, `ours := s.subs[m.ID]` :305, `h.Event(ev)` :315; `func (s *session) fail` :330, `close(ch)` :342; `s.pending[id] = ch` :366, `s.subs[id] = true` :368; `Call`: `if s == nil` :424, `ErrNotConnected` :425. Other client.go cites in section 1 were derived from unnumbered slices and may be off by up to 5 lines; function-start lines from `grep -n 'func '` (New :102, Run :135, serve :180, auth :244, read :289, call :352, subscribe :408, Call :420, CallService :506) are exact.
- internal/ha/message.go: `TypeEvent` :25, `type Event struct` :112.
- internal/ha/hatest/hatest.go: `default:` of the command switch :279 (another :295).
- internal/bridge/bridge.go (grep -n): `rule.Decide(e, labels)` :485; `b.n.Aggregator().Batch` :715; `hamapping.Map(...)` :940; `plan.Build(ctx)` :945; `plan.OnChange(b.onCommand)` :962; `g.plan.Apply(...)` :977.
- internal/config/config.go (grep -n): `haURL := str("ha-url", "HA_URL", ...)` :157, `listen` :158, `passcode` :159, `webAdmin := str("web-admin", ...)` :173; `strconv.ParseBool(*assist)` :252, `strconv.ParseBool(*webAdmin)` :257.
- `grep -ri camera internal/` returns nothing (no camera reference in any internal/ file).

## Summary table of unverified / not found

| Item | Status |
| --- | --- |
| Generic per-id event channel in internal/ha | Not found; events go to one `Handler.Event(ev Event)` without the id (client.go:305-322) |
| `unsubscribe_events` sent anywhere | Not found (no grep match) |
| `subs` entries ever deleted | Not found in client.go (only set at :367-369) |
| HTTP client / base URL / `camera_proxy` | Not found in internal/ha, internal/config, cmd |
| Token redaction function | Not found; guard is absence of logging + test client_test.go:189-190 |
| Shape of `camera/webrtc/*` event messages | Not read from HA in this task (docs/roadmap.md:239-241 only names commands) |
| Per-subscription-id event push in hatest | Not found; `fire` is unexported and keyed by event type (hatest.go:338) |
| Handlers for `camera/*` or `unsubscribe_events` in hatest | Not found; default answers `unknown_command` (hatest.go ~:292) |
| HTTP routes in hatest (`serve` internals) | Not read (hatest.go:168-219) |
| Domain table in internal/expose; camera test | Not found |
| What go-hamapping v0.3.0 does with `camera.*` | Not read here |
| `adapt/command.go` services table past line 60 | Not read |
| Exact line numbers for each `str(...)`/flag definition in config.go and for the main.go `events` list | Approximate (block at config.go ~139-158); unverified per line |
| Several line numbers marked `~` or "region" (config.go bool parsing, run.sh-adjacent addon/config.yaml flags, arch_test.go isDef/testOnly/layers/TestGuardRefuses, node.go FabricInfos/PairingCodes, hatest.go default case) | Approximate from sed slices; re-check with grep -n before citing |
| Whether each exported hatest mutator calls `fire` | Only inferred from names except where cited; `SetState` firing state_changed per its comment only |
| script/coverfloor behaviour for a package without a floor | Not read |
| Anything touching node.Node besides `Aggregator().Batch`, `FabricInfos`, `PairingCodes`, web `Node: n`, Start/Stop | Not found |
