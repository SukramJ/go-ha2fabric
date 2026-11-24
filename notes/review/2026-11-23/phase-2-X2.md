# Phase 2, unit X2 — hostile entity data

Reviewer X2 (Fable). Baseline `2324fbf`, branch `review/p2`. Flows from
Home Assistant's entity data (decision 11: untrusted) and from a
commissioned controller through `internal/ha`, `internal/plan`,
`internal/adapt`, `internal/bridge`, `internal/camera`, `internal/expose`,
`internal/exposure`, `internal/web/operation.go` and go-hamapping's plan
(boundary, decision 9). References at their pins: `gf:` go-fabric v0.25.0,
`hm:` go-hamapping v0.6.0, `ui:` go-fabric-ui v0.13.0. **M** = measured (the
line was read, or the named probe ran), **I** = inferred.

Probes ran in a `git archive 2324fbf` copy under the session scratchpad
(`…/scratchpad/X2/`), never in the worktree. Their texts are in the
appendix. No live Home Assistant: the fixtures of `internal/ha/hatest`
only.

## Summary

| ID | Severity | Subject |
| --- | --- | --- |
| HR-P2-data-30 | medium | a stream of re-sync requests starves the re-sync (published, decision 36) |
| HR-P2-data-31 | low | control and bidirectional characters reach five attributes and a terminal, not NodeLabel alone (closes HR-P1-data-36) |
| HR-P2-data-32 | low | a select's option is trimmed on its way back to Home Assistant |
| HR-P2-adpt-33 | medium | upstream (go-hamapping): every cover with tilt is called mappable and refused by go-fabric |
| HR-P2-adpt-34 | low | upstream (go-hamapping): a select with 256 options is called mappable and refused by go-fabric |
| HR-P2-data-35 | low | one refused entity takes its device's other entities off the bridge |
| HR-P2-adpt-36 | note | upstream (go-hamapping): ModeSelect labels that are equal after the 64-byte cut |
| HR-P2-bound-37 | low | T16 measured: states over the frame limit, no sync ever, and no page says so |
| HR-P2-bound-38 | low | a frame under the limit decodes to seventeen times its size, and the bridge keeps it twice |
| HR-P2-bound-39 | low | Home Assistant's error text is kept whole in the log ring and the status |
| HR-P2-bound-40 | low | camera: Home Assistant's side of a session is not bounded as the controller's is |
| HR-P2-bound-41 | note | T10 measured: no cap on the connections of an admitted peer |
| HR-P2-test-42 | low | no fuzz target takes entity data through Map, Build and Apply |

## Findings

### HR-P2-data-30 — medium — a stream of re-sync requests starves the re-sync

Published by decision 36 (first held; the owner published it with its fix).

- Flow.
  1. `Bridge.Event` (`internal/bridge/bridge.go:323-351`): an
     `entity_registry_updated`, `device_registry_updated` or
     `label_registry_updated` calls `resyncSoon` (`:344`, `:349`).
  2. `stateChanged` (`:832-853`) calls `resyncSoon` for a state of an
     entity the last sync did not know (`:841-843`: `!b.known[id]`, or a
     removed state) and for a known entity whose description differs
     from the last sync's (`:849-851`).
  3. `resyncSoon` (`:394-407`): `b.timer.Stop()` and a new
     `time.AfterFunc(b.opts.Debounce, …)` on **every** call (`:397-400`).
     A trailing debounce with no latest time: the sync runs only after
     `Debounce` (1 s in the daemon, `:197-198`; `cmd/ha2fabric/nodes.go:143`
     sets none) without any such event.
  4. Nothing else syncs: `sync` runs on a (re)connection (`:283`, `:295`),
     on the web UI's `Resync` (`:759-781`, after an apply or
     "reassemble"), on a full work queue (`:256-262`, `:241-246`) and on this
     timer. There is no periodic sync.
- Claim. While events of step 1 or 2 arrive less than `Debounce` apart,
  no sync runs. The served set stays what the last sync made it:
  - an entity the operator disabled, hid, took out of Assist's exposure
    or put under an exclude label in Home Assistant keeps its endpoint,
    reachable, and a commissioned controller keeps commanding it —
    `command` checks `b.serving`, which only a sync changes
    (`internal/bridge/command.go:75-79`);
  - a removed entity keeps its endpoint; a new one gets none; a renamed
    one keeps its old name.
  Nothing is logged and no page says so (`resyncSoon` is silent; the
  status is that of the last sync).
- Who can make the stream.
  - An entity the last sync did not know that reports more often than
    once a second: it stays unknown because the sync that would learn it
    is the one it puts off. One new fast sensor does it; so does a newly
    added integration whose entities together report several times a
    second (at 20 states a second a gap of a full second practically
    never comes). I: the rates of real integrations.
  - A known, exposed entity whose description attribute (`options`,
    `supported_features`, `min`/`max`, `hvac_modes`, a `friendly_name`
    without a registry name, …; `internal/plan/plan.go:250-256`) differs
    from the last sync's and that reports more often than once a second:
    every state is "description moved" until a sync, which never comes.
  - Whoever can write such states into Home Assistant. Decision 11 takes
    entity data as untrusted: with MQTT discovery a device on the broker
    creates an entity and publishes its state at any rate. It needs no
    Home Assistant account and nothing on this daemon. What it gets:
    the bridge's exposure frozen at the moment it starts — the
    operator's revocation of an entity from the controllers does not
    take effect, silently — for as long as it publishes.
- Not affected: the web UI's own changes (the selection page applies
  through `Resync`, not the timer); states of known entities (they are
  applied, `:852`).
- M: probe `TestX2ProbeResyncStarved` (`internal/bridge/x2_probe_test.go`
  in the copy; `Debounce` 200 ms, `sensor.x2_new` reporting every 50 ms,
  then `switch.coffee` disabled in the registry):
  `switch.coffee, disabled, kept its endpoint for 5 s (25 debounces)
  while sensor.x2_new reported every 50 ms; last sync 5.645s before the
  flood ended`. The endpoint goes once the entity is quiet (the probe's
  last step passes).
- Severity. Medium: an operator's act that takes an entity away from the
  controllers is not applied for an unbounded time and without a sign,
  from a cause that needs no fault and no privilege beyond producing
  state changes. Not high: nothing is exposed that the rule did not
  expose before; a restart, a reconnection or the web UI's reassemble
  ends it.
- Fix. Give the debounce a latest time: keep the first pending request's
  time and fire at `min(last + Debounce, first + maxWait)` (say `maxWait`
  = 5 × `Debounce`), or simply do not re-arm a timer that is pending
  (a fixed delay from the first request). After the sync the unknown
  entity is known, so the first cause ends by itself; for the second (a
  description that never settles) add a floor between two syncs of the
  same cause and a warning (`bridge.resync_storm`, the entity id) so the
  cost of a full read a second does not replace the starvation. A
  low-rate periodic sync (say every 10 min) as a backstop is the owner's
  call.
- Red test. The probe below: with the flood running, the disabled
  entity's endpoint is gone within a few debounces.
- For the verifier. X3's items 1 and 2 (reconnect, `Fanout` and resync
  under load) border on this; if X3 reports the same line, one row.

### HR-P2-data-31 — low — control and bidirectional characters reach five attributes and a terminal (closes HR-P1-data-36)

- Flow. A registry device's `name_by_user`, `name`, `manufacturer`,
  `serial_number` (`internal/ha/message.go:224-233`), an entity's `name`,
  `original_name` (`:203-204`) and a state's `friendly_name` and `options`
  (`:129`) → `adapt.Device` (`internal/adapt/adapt.go:44-72`),
  `adapt.Entity` (`:119-140`), `entityName` (`:161-170`), `plan.Assess`
  (`internal/plan/plan.go:183`, `:186`, `:211-215`) → go-hamapping:
  NodeLabel `hm:plan.go:369-386`, VendorName `hm:plan.go:387`,
  SerialNumber `hm:plan.go:345-350`, ModeSelect Description and labels
  `hm:options.go:126-131` → go-fabric: `gf:endpoint/spec.go:152-157`
  (composes and cuts, nothing else), `gf:device/bridged_basic_information.go:67-73`
  (`spec.CheckWritten`: type and length) → the node's attributes.
- Claim. Every byte Home Assistant's JSON can carry arrives, cut to the
  attribute's length on a character boundary and not otherwise changed:
  NUL, C0 and C1 controls, ESC sequences, U+202E and the other bidi
  controls, zero-width characters. HR-P1-data-36 said so for NodeLabel;
  the same holds for BridgedDeviceBasicInformation VendorName and
  SerialNumber and for ModeSelect Description and every
  SupportedModes label. Nothing is refused: no hostile string fails a
  build. Invalid UTF-8 never arrives (`encoding/json` decodes it as
  U+FFFD). go-hamapping serves no other string of a state (its rows map
  state text through enums only: no `string` attribute in
  `hm:rows_gen.go` reads a state).
- Besides the controller. `--dump-plan` prints the device name, the label
  names and the reasons unquoted to the operator's terminal
  (`cmd/ha2fabric/main.go:405`), so an ESC sequence of a device name runs
  in that terminal. The log quotes (slog's text handler; probe
  `TestX2ProbeErrorTextInRing`: a newline and `msg=forged` in an error
  text stay inside one quoted value). The pages are go-fabric-ui's
  (text interpolation, its T11).
- The operator's own text is not checked either: a web UI name override
  is held to 256 bytes only (`internal/selection/selection.go:298-306`)
  and goes verbatim into NodeLabel (`internal/plan/plan.go:302-305`,
  `hm:plan.go:371-377`); a registry node's label is cut, not checked
  (`cmd/ha2fabric/nodes.go:522-528`). Only `LABEL` refuses control
  characters (`internal/config/config.go:341`).
- M: probe `TestX2ProbeHostileText` (controller's read after the sync):
  `NodeLabel "A\x00B\nC\x1b[31m‮evil⁦​ÄÄÄÄ" (31 bytes)`,
  `VendorName "V\x00\a\x1b]0;x\a‮ACMEÖÖÖÖÖÖÖÖ" (32 bytes)`,
  `SerialNumber "S\x00\r\n‮9999999999999999999999999" (32 bytes)`,
  `ModeSelect Description "Sel\x00‮\nnnn…"`, `label[0] "a\x00b\x1b[2J‮c"`.
  I: what a controller's app makes of them; whether Matter's string type
  forbids C0 controls (not read).
- **Decision on HR-P1-data-36: sanitise locally, in `internal/adapt`; no
  go-fabric `ComposeNodeLabel` candidate.** Reasons: (1) the composer
  (`gf:endpoint/spec.go:152-157`, `gf:device/lifecycle.go:394`) covers
  one of five sinks; VendorName, SerialNumber and the ModeSelect strings
  never pass it. (2) go-fabric takes a host's stated value as the
  host's (`gf:device/bridged_basic_information.go:60-73`: refuse, never
  rewrite); which text is untrusted is this daemon's statement (decision
  11), and `internal/adapt` is the one place Home Assistant's text enters
  the model. (3) It is not a workaround of an upstream defect: neither
  library is wrong to pass a string. One exception is upstream: a
  select's option must go back to Home Assistant byte for byte
  (`hm:options.go:144`), so only its *label* may be cleaned, and that is
  in go-hamapping (`hm:options.go:129`) — an upstream candidate
  (`adpt`), low.
- Fix. One function in `internal/adapt` (say `Text`): drop
  `unicode.IsControl` runes (C0, DEL, C1) and the bidi controls U+061C,
  U+200E, U+200F, U+202A–U+202E, U+2066–U+2069; keep U+200C/U+200D (they
  join Persian, Indic and emoji sequences); trim. Apply it to the device
  name, manufacturer and serial (`adapt.go:45-48`, `:64`, `:69`), the
  entity name (`:161-170`) and the two `friendly_name` reads in
  `plan.go:186`, `:211`. Refuse control characters in a web UI name
  (`selection.Set`) and a registry node label as `config.go:341` does
  for `LABEL`. Quote (`%q`) the free text columns of `--dump-plan`.
- Red test. The appendix's `TestX2ProbeHostileText` (it fails on the
  first control character of each attribute); for the option labels,
  go-hamapping: a mode's label carries no control or bidi character
  while `select_option` carries the option unchanged.

### HR-P2-data-32 — low — a select's option is trimmed on its way back to Home Assistant

- Flow. `options` of a select (`internal/adapt/adapt.go:138-140`;
  `strings_` drops only blank entries, `:281-286`) → ModeSelect, mode =
  index (`hm:options.go:126-130`) → a controller's `ChangeToMode` →
  `b.send(o.Method, l[req.NewMode])` (`hm:options.go:144`), the option
  byte for byte → `adapt.Service`: `payload :=
  strings.TrimSpace(string(cmd.Payload))` (`internal/adapt/command.go:97`)
  → `select_option` with `option: payload` (`:190`).
- Claim. An option with leading or trailing white space reaches Home
  Assistant without it. Home Assistant refuses an option that is not in
  the list, so the mode cannot be chosen; where the trimmed text is
  another option of the list (`"Eco"` beside `"Eco "`), the controller
  chooses one mode and Home Assistant runs the other.
- M: probe `TestX2ProbeOptionTrimmed`: `the controller chose "Eco ",
  select_option carries "Eco"`.
- Fix. For `MethodSelectOption` use `string(cmd.Payload)` untrimmed (the
  trim is for the numeric and catalog payloads).
- Red test. The probe.

### HR-P2-adpt-33 — medium — upstream (go-hamapping): every cover with tilt is called mappable and refused by go-fabric

- Flow. A cover's `supported_features` with OPEN_TILT, CLOSE_TILT or
  SET_TILT_POSITION and `device_class` blind, shade, shutter, curtain,
  awning or none (`internal/adapt/adapt.go:121`, `:176-180`) → row
  `cover/window_covering`: features Lift always and Tilt
  (`hm:rows/cover.yaml:26-30`, `hm:rows_gen.go:1405`), Type from the
  device class, Rollershade for blind, shade and none, Shutter, Drapery,
  Awning (`hm:rows/cover.yaml:32-37`, `hm:rows_gen.go:1439-1455`) →
  `device.New` → go-fabric holds Type and EndProductType to the
  FeatureMap (`gf:cluster/cover/windowcovering_server.go:385-399`;
  Rollershade is `LF & !TL`, Shutter `LF & !TL | !LF & TL`,
  `gf:cluster/spec/windowcovering/definition_gen.go:273-281`) →
  `ErrEnumValue` → `Build` fails for the whole plan (`hm:plan.go:402-405`)
  → `bridge.build_failed`, no endpoint (`internal/bridge/bridge.go:788-799`).
- Claim. No hostile value is needed: a venetian blind as most
  integrations state it (tilt features, `device_class: blind`) has no
  endpoint at this pin, and with HR-P2-data-35 neither has any other
  entity of its Home Assistant device. `--dump-plan` and the web UI's
  preview call it mappable (they do not build). The fixtures have no
  cover with tilt (`testdata/ha/get_states.json`: features 15, 3, 7), so
  no test of this repository or of go-hamapping's rows sees it. Only
  garage, gate, door, damper and window (Type Unknown) build with tilt.
- M: probe `TestX2ProbeCoverBuild`: `36 cover shapes are mappable and do
  not build` (six device classes × six feature sets with tilt), e.g.
  `device_class blind features 255: mappable, refused: … cover: Type or
  EndProductType not allowed under the FeatureMap`. Found by the fuzz
  probe's fifth seed (HR-P2-test-42).
- Fix. Upstream, go-hamapping: Type and EndProductType by the features
  too (Lift and Tilt: TiltBlindLift; the EndProductType allowed with
  both), and a check in `Map` that what it calls mappable builds. Not a
  workaround here (decision 9). Until the pin moves, the release notes
  should say so if a tag is cut.
- Red test. go-hamapping: a cover with `device_class: blind` and features
  255 builds. Here: the probe, and a fixture cover with tilt in the end
  to end test.

### HR-P2-adpt-34 — low — upstream (go-hamapping): a select with 256 options is called mappable and refused by go-fabric

- Flow. `options` with 256 entries → `checkOptions` admits up to
  `maxModes = 256` (`hm:options.go:81`, `:107`) → `device.New` →
  `SupportedModes length 256 violates "max 255"` (go-fabric's generated
  constraint) → `Build` fails (`hm:plan.go:402-405`).
- Claim. Off by one: 257 options are unmappable with a reason, 256 are
  "mappable" in `--dump-plan` and the web UI's preview and refused by the
  node. The bridge's verdict says why (`bridge.go:794`).
- M: probe `TestX2Probe256Options`: `dry (web preview, --dump-plan):
  mappable` against `bridge: unmappable: go-fabric refused the device: …
  SupportedModes length 256 violates "max 255"`.
- Fix. Upstream: `maxModes = 255`.
- Red test. go-hamapping: 256 options are unmappable; the probe here.

### HR-P2-data-35 — low — one refused entity takes its device's other entities off the bridge

- Flow. One Home Assistant device is one group and one go-hamapping plan
  (`internal/plan/plan.go:206-232`) → `hp.Build` (`internal/bridge/bridge.go:788`)
  returns at the first endpoint go-fabric refuses (`hm:plan.go:402-405`)
  → the group has no devices, every entity's verdict becomes
  "go-fabric refused the device" (`bridge.go:789-798`) → the Diff
  removes the endpoints the group served.
- Claim. An entity whose attributes make go-fabric refuse it — today a
  cover with tilt (HR-P2-adpt-33) or a select with 256 options
  (HR-P2-adpt-34); tomorrow whatever a row lets through — removes the
  endpoints of its siblings, which a controller then loses with their
  automations. The trigger is an integration's data (decision 11), the
  effect is on entities that are sound.
- M: probe `TestX2ProbeSiblingKnockedOut`: `switch.coffee: endpoint 0,
  unmappable: go-fabric refused the device: hamapping: entity
  cover.living_blind …`.
- Fix. Local: on a build failure, map and build the group again without
  the refused entity (go-hamapping names it in the error; better a typed
  error — an upstream candidate: `Build`'s error carries the entity
  key), so the device keeps its sound endpoints and only the refused
  entity is unmappable.
- Red test. The probe.

### HR-P2-adpt-36 — note — upstream (go-hamapping): ModeSelect labels that are equal after the 64-byte cut

- `hm:options.go:129`: `Label: cut(s, maxLabel)`. Options that differ
  only after their 64th byte give modes a controller cannot tell apart;
  the option sent is still the right one by index (`:144`).
- M: probe `TestX2ProbeHostileText`: `255 modes, 2 distinct labels`;
  `select_option carries the list's option 5, 4069 bytes`.
- Fix. Upstream: number a label that repeats (as far as 64 bytes allow),
  or call such a select unmappable.

### HR-P2-bound-37 — low — T16 measured: states over the frame limit, no sync ever, and no page says so

- Flow. One state's attributes push a `state_changed` frame or the
  `get_states` result over `DefaultReadLimit` (64 MiB,
  `internal/ha/client.go:58`, `:286`) → `ws.Read` fails (`:405-408`) →
  the session ends, `Disconnected` → every endpoint unreachable
  (`internal/bridge/bridge.go:290-296`, `:300-320`) → reconnect,
  `Connected` → `sync` → `get_states` (`internal/ha/snapshot.go:55`) is
  over the limit again → the connection drops in the read →
  `bridge.sync_failed error="get_states: ha: connection dropped"`
  (`bridge.go:465-468`) → back-off, again.
- Claim. The threat model's T16 (I) holds, measured: while the states
  are over the limit the daemon never syncs. Every endpoint stays
  unreachable; a registry change made meanwhile (an entity disabled) is
  not applied. The entity need not be exposed: any state counts. What the
  operator gets: `ha.disconnected error="ha: read: failed to read:
  websocket: message too big: read limited at 67108865 bytes"` and
  `bridge.sync_failed error="get_states: ha: connection dropped"` in the
  log, each once per attempt; the sync's own error names neither the
  limit nor an entity. No page shows it: `Status.SyncError` is read by
  `Resync` only (`bridge.go:777`), the web UI's warnings are the label,
  selection and erase ones (`internal/exposure/exposure.go:349-364`).
  With the production back-off (a connection that drops within
  `MaxBackoff` doubles it, `client.go:231-243`) Home Assistant serialises
  the whole result once a minute.
- M: probe `TestX2ProbeOversizedStates` (a 65 MiB attribute on
  `sensor.outdoor`): `after 6 s: connected true, authenticated 8 times,
  last sync moved false, sync error "get_states: ha: connection
  dropped"`; `after the operator disabled switch.coffee: still served
  true`. (The fixture's back-off is 20–100 ms, hence eight attempts.)
- Fix. Cannot be read in parts (`get_states` is one result). So: say it —
  a read that ends on the limit is its own error (`ha: Home Assistant's
  message exceeds READ_LIMIT …`) that the sync's error keeps, a web UI
  warning for a sync that keeps failing (code `sync_failed`, since), and
  the limit as an option; `docs/operations.md` names the symptom. With
  HR-P2-bound-38's per-state cap the offending entity is named in the
  log when it is one state of a result that fits.
- Red test. The probe, asserting the warning and the error's text.

### HR-P2-bound-38 — low — a frame under the limit decodes to seventeen times its size, and the bridge keeps it twice

- Flow. A frame of up to 64 MiB → `Decode` into `Message` (the result
  copied as `json.RawMessage`, `internal/ha/message.go:47-57`, `:76-95`)
  → `listOf` splits it into `[]json.RawMessage` and decodes each entry
  (`internal/ha/client.go:736-765`) → `State.Attributes map[string]any`
  (`message.go:129`) → the bridge keeps every state of every entity,
  exposed or not, in `b.states` and in `b.last` (`internal/bridge/bridge.go:481-485`,
  `:609`), and the next sync builds the next generation while the last
  is still held.
- Claim. The read limit bounds the frame, not the memory. A legal 60 MiB
  `get_states` of small numbers holds about 1 GiB once decoded, and a
  sync in progress holds two generations; on the add-on's hardware that
  is the out-of-memory killer and a restart loop that the next sync
  repeats. The attributes of entities no rule exposes are kept whole.
- M: probe `TestX2ProbeDecodeAmplification`: `frame 60 MiB; heap in use
  after decode, before GC 1142 MiB (total allocated 2949 MiB); kept by
  the 1571 states alone 1044 MiB`; the probe ran 25 s. I: the add-on's memory
  at that point (not run).
- Fix. A cap per state (say 256 KiB of raw JSON): a state over it is
  kept without its attributes and logged once with its entity id — the
  operator learns which entity — and the same cap for a `state_changed`.
  Then lower `DefaultReadLimit`'s meaning to "the sum" and document it.
- Red test. The probe with a bound on the heap; a state over the cap is
  planned without attributes and logged.

### HR-P2-bound-39 — low — Home Assistant's error text is kept whole in the log ring and the status

- Flow. A failed `call_service` answers with an error whose message is
  the integration's exception text (`internal/ha/message.go:61-66`) →
  `Status.LastCommand = sc.String() + ": " + err.Error()`
  (`internal/bridge/command.go:99`) and `bridge.call_service_failed`
  with the text as an attribute (`:103`) → the log ring, 1000 records,
  no bound on a record (`internal/web/operation.go:20`,
  `gf:logring/logring.go:55`) → the Operation page and its stream
  (`operation.go:53-68`, `:81-87`). The same for `camera.ha_error`'s
  `message` (`internal/camera/camera.go:706`), `camera.offer_failed`
  (`:442`) and `bridge.sync_failed` (`internal/bridge/bridge.go:466`).
- Claim. Quoting holds (T15: one line, the newline and the forged
  `msg=` escaped inside the value); length does not. A 1 MiB error text
  is a 1 MiB record, a 1 MiB stderr line and a 1 MiB status string; a
  controller that repeats the failing command fills the ring with a
  thousand of them, and every Operation page load carries them.
- M: probe `TestX2ProbeErrorTextInRing`: `status.LastCommand 1048703
  bytes; ring record attributes 1048701 bytes; stderr line 1048801
  bytes`.
- Fix. Cut Home Assistant's text where it becomes an error
  (`ha.Error.Error`, the camera's event message): 512 bytes and "… (n
  more bytes)". A ring-side bound per record is a go-fabric candidate
  (`gf:logring`), note.
- Red test. The probe.

### HR-P2-bound-40 — low — camera: Home Assistant's side of a session is not bounded as the controller's is

- Flow. Events of a `camera/webrtc/offer` subscription
  (`internal/ha/subscription.go:25`, a buffer of 64, drained by the
  relay) → `relay` (`internal/camera/camera.go:644-656`): every event
  that decodes touches the session (`:652`) before its type is looked at
  → a `candidate` event becomes one `IceCandidates` invoke to the
  controller (`:691-704`), of any size and number; an unknown type is
  ignored (`:709-710`).
- Claim. Phase 1's caps (HR-P1-bound-02, -17; decision 31) bound what a
  controller can ask of Home Assistant: 64 candidates a call, 256 a
  session, 8 sessions a camera, one still request. The other direction
  has none: a camera integration (untrusted data, decision 11) signals
  any number of candidates of any length to the controller in one
  session, and any event, even one the relay ignores, puts off the idle
  end of decision 31 and makes the session the last to be evicted
  (`idlest`, `:477-488`). Eight such sessions hold the camera's bound
  for good. The controller's side holds under the probe-free read: the
  three refusals precede any Home Assistant call (`:545-557`), a
  re-offer closes the old subscription first (`:436-439`), a still is
  one request shared (`:886-924`) and at most 64 000 bytes
  (`internal/ha/camera.go:182-188`).
- M: probe `TestX2ProbeHACandidatesUnbounded`: `5000 candidates of 4010
  bytes each signalled to the controller in one session (the
  controller's own are held to 256); after 1.2 s of ignored events with
  IdleTimeout 300 ms the session ended: false`.
- Fix. Count Home Assistant's candidates per session against
  `MaxSessionCandidates` and end the session beyond it (ICE failed);
  refuse a candidate longer than a bound (a real one is under 300
  bytes); touch the session only for the event types the relay acts on.
- Red test. The probe.
- Observation, no finding: at eight sessions a controller's offer evicts
  the session idle longest once it has been idle a minute; a standing
  live view signals nothing, so it is that session. Decision 31 took
  this (a commissioned controller is a fabric administrator).

### HR-P2-bound-41 — note — T10 measured: no cap on the connections of an admitted peer

- `internal/web/web.go:115-119`: header, read and idle timeouts, no
  `MaxHeaderBytes` (Go's 1 MiB), no limit on connections. go-fabric-ui
  caps its event streams (`ui:fabricui/handler.go:269-274`) and its
  bodies (`ui:fabricui/handlers_controller.go:188`).
- M: probe `TestX2ProbeConnections`: `800 of 800 connections served and
  held open; goroutines 4 -> 804`; each is held for `IdleTimeout` (120
  s) after one request. The process shares its descriptors with the
  Matter node and the Home Assistant client. In the add-on the admitted
  peer is the Ingress proxy alone; stand-alone it is whoever `WEB_ALLOW`
  admits (X1's table).
- Fix. `netutil.LimitListener` (say 256) around the listener; note.

### HR-P2-test-42 — low — no fuzz target takes entity data through Map, Build and Apply

- The thirteen `Fuzz…` functions of the tree and what the eight on this
  unit's path reach:

  | Input | Target | Reaches |
  | --- | --- | --- |
  | a frame | `FuzzDecode` (`internal/ha/message_test.go:68`) | the envelope |
  | an event's data | `FuzzEventData` (`internal/ha/fuzz_test.go:18`) | the decode of `state_changed`, the registry updates, a WebRTC event; not `Bridge.Event` nor the camera's relay |
  | the snapshot's results | `FuzzSnapshotResults` (`:57`) | the decode of each list |
  | a state and its attributes | `FuzzEntityState` (`internal/adapt/fuzz_test.go:21`) | `adapt.Entity`, `adapt.State`; stops before go-hamapping |
  | a command | `FuzzService` (`:44`) | `adapt.Service` |
  | a candidate, both ways | `FuzzCandidateFromHA`, `FuzzCandidateToHA` (`internal/camera/fuzz_test.go:27`, `:67`) | the conversion |
  | a still | `FuzzCaptureSnapshot` (`:108`) | the JPEG check |

- Not reached by any: entity data through `plan.Assess`, go-hamapping's
  `Map`, `Build` and `Apply` — where the text and the numbers meet
  go-fabric's constraints; the device entry's text; `listOf`'s skip path;
  the relay's `event` with a hostile sequence of events.
- M: probe `FuzzX2PlanBuild` (state, attributes, device name, vendor,
  serial → `Assess` → `Map` → `Build` → `Apply`; "what the dry plan
  calls mappable builds, nothing panics"): its fifth seed failed at once
  (HR-P2-adpt-33), HR-P2-adpt-34 is the same property; with those two
  known refusals skipped it ran 120 s without a failure or a panic.
- Fix. Land the target in `internal/bridge` (package `plan` must not
  call `Build`, `internal/arch`), in `make fuzz-smoke`, with the two
  refusals as red seeds until the pin moves.
- Red test. The target itself.

## What holds (no finding)

- **T12, the four text values.** A controller's bytes reach none of
  them. `hvac_mode`, `fan_mode` and `preset_mode` are the row's
  constants: the written enum is looked up in the row's map and the
  command is dropped when it is not there (`hm:binding.go:324-330`;
  `hm:rows_gen.go:816`, `:1194` mode; `:1021-1025` fan_mode; `:1678`
  preset_mode; `:5266` a water heater's operation_mode, which
  `internal/adapt/command.go:262-278` turns into `"off"` or an entry of
  the entity's own `operation_list`). `option` is the entity's own
  option by the controller's index, bounds checked
  (`hm:options.go:140-144`), so it is Home Assistant's text, not the
  controller's — but see HR-P2-data-32. The numbers are parsed and
  clamped as the threat model says (`command.go:116-121`, `:159-162`,
  `:226-229`, `:333-355`). M: read; `TestX2ProbeHostileText` (the option
  of mode 5 arrives as the list's); `FuzzService` stands.
- **T13, the lock.** The row declares DoorLock without a feature
  (`hm:rows_gen.go:2931-2944`), so no PIN credential exists and
  RequirePINforRemoteOperation is not served: an `UnlockDoor` without a
  PIN is admitted (`gf:cluster/lock/doorlock_server.go:1107-1111`,
  `:1232-1236`), one with a PIN is refused as a wrong code (`:1113-1116`).
  The lock commands are timed (`gf:cluster/spec/doorlock/definition_gen.go:80-83`).
  The controller's PIN never leaves go-fabric: the row's port emits the
  handle with no field (`hm:rowports.go:98-103`), and the payload is the
  catalog's constant — `adapt.description` states no `payload_lock` or
  `payload_unlock` from a state's attributes (`internal/adapt/adapt.go:174-228`),
  so an integration cannot move it. `lock.unlock` goes without a `code`
  (`command.go:46`, `:109-112`): a lock that needs one refuses, in Home
  Assistant. The threat model's T13 row stands as written (an exposed
  lock is the controller's to unlock; phase 3 docs). M: read.
- **Entity ids and services.** The entity id of a call is the plan's
  (`command.go:92-96`); the queue is bounded (256,
  `internal/bridge/bridge.go:208`, `internal/bridge/command.go:40-45`); a
  queued command of an entity that left the bridge is dropped
  (`command.go:75-79`).
- **Lengths.** NodeLabel 32 bytes (`gf:endpoint/spec.go:170-179`),
  VendorName and SerialNumber 32 (`hm:plan.go:349`, `:387`), ModeSelect
  label and description 64 (`hm:options.go:129-131`), all on character
  boundaries; a web UI name 256 (`selection.go:303`). A list of options
  over 256 is unmappable with a reason; long lists elsewhere
  (`hvac_modes`, `supported_color_modes`, identifiers) reach predicates
  and the signature only. M: `TestX2ProbeHostileText`.
- **T15, quoting.** slog's text handler quotes every hostile value the
  probes sent (`TestX2ProbeErrorTextInRing`); the ring's attributes are
  the raw strings for the page to interpolate as text (`operation.go:81-87`).
- **Decode.** A registry entry that does not decode is skipped by id and
  not exposed (HR-P1-data-90, read again: `client.go:736-785`,
  `plan.go:139-146`, `:191-196`); a state that does not decode is left
  out (`client.go:657-659`).

## Counts

| Severity | Count | IDs |
| --- | --- | --- |
| high | 0 | — |
| medium | 2 | data-30 (published, decision 36), adpt-33 (upstream) |
| low | 9 | data-31, data-32, adpt-34 (upstream), data-35, bound-37, bound-38, bound-39, bound-40, test-42 |
| note | 2 | adpt-36 (upstream), bound-41 |

Held: none (HR-P2-data-30 was held at first and published by decision 36). HR-P1-data-36: closed by
HR-P2-data-31 (sanitise locally in `internal/adapt`; no go-fabric
candidate; one go-hamapping candidate for the option labels).

Upstream candidates (go-hamapping, `adpt`): adpt-33, adpt-34, adpt-36,
the option label of data-31, a typed build error for data-35. go-fabric:
a per-record bound in `logring` (bound-39), note.

## Not read, and why

- go-fabric's camera kit (`gf:cluster/camera/provider.go`) beyond the
  ports this host implements: whether it holds a session to its fabric
  and its peer is go-fabric's review; this host trusts the session id it
  is handed (`camera.go:410`, `:536`, `:584`).
- go-fabric's DoorLock server beyond `authorizeLocked` and `requirePin`;
  its wrong-code lockout was not followed.
- go-hamapping's rows one by one: the four text methods, the lock, the
  select and the cover rows only (boundary, decision 9); the fuzz probe
  ran fifteen domains' seeds.
- go-fabric-ui's pages (how they show a name with bidi characters): its
  own review (T11).
- `internal/expose` beyond its call from `plan.Assess`, `internal/nodes`,
  `internal/config`: X1 and X3; `cmd/ha2fabric` beyond `--dump-plan`'s
  print and the label cut.
- The other reviewers' reports of this phase (independence).
- Home Assistant's own limits on an attribute's size and an error's
  text (`core:` not read for it): the bounds above assume none.

## Probes

In the copy `…/scratchpad/X2/` (`git archive 2324fbf`), `go test -count=1`:

| Probe | File | Result |
| --- | --- | --- |
| `TestX2ProbeHostileText` | `internal/bridge/x2_probe_test.go` | FAIL (data-31, adpt-36; T12 option passes) |
| `TestX2Probe256Options` | same | FAIL (adpt-34) |
| `TestX2ProbeOversizedStates` | same | FAIL (bound-37) |
| `TestX2ProbeErrorTextInRing` | same | FAIL on length (bound-39); quoting passes |
| `TestX2ProbeSiblingKnockedOut` | same | FAIL (data-35) |
| `TestX2ProbeCoverBuild` | `internal/bridge/x2_cover_test.go` | FAIL (adpt-33) |
| `FuzzX2PlanBuild` | `internal/bridge/x2_fuzz_test.go` | FAIL at seed 5 (adpt-33); 120 s clean with the two known refusals skipped |
| `TestX2ProbeOptionTrimmed` | `internal/adapt/x2_probe_test.go` | FAIL (data-32) |
| `TestX2ProbeDecodeAmplification` | `internal/ha/x2_probe_test.go` | PASS (a measurement, bound-38) |
| `TestX2ProbeHACandidatesUnbounded` | `internal/camera/x2_probe_test.go` | FAIL (bound-40) |
| `TestX2ProbeConnections` | `internal/web/x2_probe_test.go` | FAIL (bound-41) |
| the HR-P2-data-30 probe | see "Probe of HR-P2-data-30" | FAIL (data-30) |

## Questions for the owner

1. **HR-P2-data-31 / HR-P1-data-36.** Sanitise in `internal/adapt`
   (drop controls and bidi controls, keep the joiners), or refuse such a
   name and fall back to the entity id? Recommendation: drop — a refusal
   would rename a device for one stray character, and the name is shown,
   never compared.
2. **HR-P2-adpt-33.** Every cover with tilt is off the bridge at this
   pin. Hold the `0.7.0` tag (decision 21 names it) for a go-hamapping
   release that builds them, or tag with a known issue? Recommendation: fix in go-hamapping first and lift
   the pin in the fix wave; it is a one-row change and the fuzz target of
   HR-P2-test-42 guards it.
3. **HR-P2-data-35.** A partial device (the sound entities served, the
   refused one unmappable) or all-or-nothing as today? Recommendation:
   partial, with the refused entity's reason in its verdict.
4. **HR-P2-bound-38.** A per-state cap (256 KiB proposed) drops the
   attributes of an entity over it, which may be an entity the operator
   exposed. Acceptable with a log line and a verdict reason?
   Recommendation: yes; no row reads an attribute of that size.
5. **HR-P2-bound-37.** Add a web UI warning for a sync that keeps
   failing (new code beside `label_unresolved`)? Recommendation: yes; it
   also serves every other cause of a failing sync, which no page shows
   today.
6. **The held row.** After its fix, publish it with the fix wave's pull
   request as decision 23 did for phase 1's camera rows, or keep it to
   the tag (decision 21)? Recommendation: publish with the fix.

## Appendix: probe texts

As they ran in the copy; each file's package and imports included. The
fix wave reuses them as the red tests.

### `internal/bridge/x2_probe_test.go`

```go
// X2 probes (review phase 2, unit X2). Not part of the tree.
package bridge_test

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	bridgedbidef "thingfabric.dev/go-fabric/cluster/spec/bridgeddevicebasicinformation"
	modeselectdef "thingfabric.dev/go-fabric/cluster/spec/modeselect"
	onoffdef "thingfabric.dev/go-fabric/cluster/spec/onoff"
	"thingfabric.dev/go-fabric/controller/client"
	"thingfabric.dev/go-fabric/fabrictest"

	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/web"
)

// hostile reports the first control, format (bidi) or private character.
func hostile(s string) (rune, bool) {
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return r, true
		}
	}
	return 0, false
}

// TestX2ProbeHostileText drives hostile registry and state text from the
// fake Home Assistant through the sync to what a commissioned controller
// reads.
func TestX2ProbeHostileText(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	e.commission()
	plug := e.mustEndpoint("switch.coffee")

	e.ha.UpdateDevice("dev-shelly-1", map[string]any{
		"name_by_user":  "A\x00B\nC\x1b[31m‮evil⁦​" + strings.Repeat("Ä", 40),
		"manufacturer":  "V\x00\a\x1b]0;x\a‮ACME" + strings.Repeat("Ö", 40),
		"serial_number": " \tS\x00\r\n‮" + strings.Repeat("9", 80),
	})
	var st bridgedbidef.State
	e.eventually("the hostile device name is served", func() bool {
		values, err := fabrictest.ReadCluster(e.ctx, e.pair, plug, bridgedbidef.ClusterID)
		if err != nil {
			return false
		}
		st, _ = bridgedbidef.StateFrom(values)
		return strings.HasPrefix(st.NodeLabel, "A")
	})
	t.Logf("NodeLabel    %q (%d bytes)", st.NodeLabel, len(st.NodeLabel))
	t.Logf("VendorName   %q (%d bytes)", st.VendorName, len(st.VendorName))
	t.Logf("SerialNumber %q (%d bytes)", st.SerialNumber, len(st.SerialNumber))
	for name, v := range map[string]string{"NodeLabel": st.NodeLabel, "VendorName": st.VendorName, "SerialNumber": st.SerialNumber} {
		if len(v) > 32 {
			t.Errorf("%s is %d bytes", name, len(v))
		}
		if r, bad := hostile(v); bad {
			t.Errorf("%s carries %U: %q", name, r, v)
		}
	}

	// A select: 256 options that share their first 64 bytes, the first one
	// with control and bidi characters; the entity's name hostile too.
	opts := make([]any, 255)
	for i := range opts {
		opts[i] = strings.Repeat("o", 64) + fmt.Sprintf("-%03d-", i) + strings.Repeat("x", 4000)
	}
	opts[0] = "a\x00b\x1b[2J‮c"
	e.ha.UpdateEntity("select.hall_scene", map[string]any{"name": "Sel\x00‮\n" + strings.Repeat("n", 100)})
	e.ha.SetState("select.hall_scene", opts[7].(string), map[string]any{"options": opts, "friendly_name": "x"})
	var ms modeselectdef.State
	var lastErr error
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && len(ms.SupportedModes) != 255 {
		time.Sleep(100 * time.Millisecond)
		ep, ok := e.endpoint("select.hall_scene")
		if !ok {
			lastErr = fmt.Errorf("no endpoint")
			continue
		}
		values, err := fabrictest.ReadCluster(e.ctx, e.pair, ep, modeselectdef.ClusterID)
		if err != nil {
			lastErr = err
			continue
		}
		ms, _ = modeselectdef.StateFrom(values)
		lastErr = fmt.Errorf("read ok, %d values", len(values))
	}
	if len(ms.SupportedModes) != 255 {
		for _, v := range e.br.Plan() {
			if v.EntityID == "select.hall_scene" {
				t.Logf("verdict %s %s endpoint %d", v.State, v.Reason, v.Endpoint)
			}
		}
		t.Fatalf("255 modes never read: %d modes, last error %v; status %+v", len(ms.SupportedModes), lastErr, e.br.Status())
	}
	sel := e.mustEndpoint("select.hall_scene")
	labels := map[string]int{}
	for _, m := range ms.SupportedModes {
		labels[m.Label]++
	}
	t.Logf("ModeSelect Description %q; %d modes, %d distinct labels; label[0] %q; CurrentMode %d", ms.Description, len(ms.SupportedModes), len(labels), ms.SupportedModes[0].Label, ms.CurrentMode)
	if r, bad := hostile(ms.Description); bad {
		t.Errorf("Description carries %U: %q", r, ms.Description)
	}
	if r, bad := hostile(ms.SupportedModes[0].Label); bad {
		t.Errorf("SupportedModes[0].Label carries %U: %q", r, ms.SupportedModes[0].Label)
	}
	if len(labels) != len(ms.SupportedModes) {
		t.Errorf("%d modes carry %d distinct labels", len(ms.SupportedModes), len(labels))
	}
	values, _ := fabrictest.ReadCluster(e.ctx, e.pair, sel, bridgedbidef.ClusterID)
	st, _ = bridgedbidef.StateFrom(values)
	t.Logf("select NodeLabel %q", st.NodeLabel)
	if r, bad := hostile(st.NodeLabel); bad {
		t.Errorf("the select's NodeLabel carries %U: %q", r, st.NodeLabel)
	}

	// T12: the option that reaches Home Assistant is the list's, by index.
	if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(sel, modeselectdef.ClusterID, modeselectdef.CmdChangeToMode), modeselectdef.ChangeToModeRequest{NewMode: 5}); err != nil {
		t.Fatal(err)
	}
	call, err := e.ha.WaitCall(e.ctx, func(c ha.ServiceCall) bool { return c.Service == "select_option" })
	if err != nil {
		t.Fatal(err)
	}
	if call.Data["option"] != opts[5] {
		t.Errorf("option %.80q", call.Data["option"])
	}
	t.Logf("select_option carries the list's option 5, %d bytes", len(call.Data["option"].(string)))

	// 257 options: unmappable, the endpoint goes.
	e.ha.SetState("select.hall_scene", "x", map[string]any{"options": append(opts, "one more", "two more"), "friendly_name": "x"})
	e.eventually("the select with 257 options has no endpoint", func() bool {
		_, ok := e.endpoint("select.hall_scene")
		return !ok
	})
	for _, v := range e.br.Plan() {
		if v.EntityID == "select.hall_scene" {
			t.Logf("257 options: %s: %s", v.State, v.Reason)
		}
	}
}


// TestX2Probe256Options: go-hamapping's Map takes a select with 256
// options (maxModes), go-fabric's ModeSelect refuses the 256th.
func TestX2Probe256Options(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	opts := make([]any, 256)
	for i := range opts {
		opts[i] = fmt.Sprintf("o%d", i)
	}
	e.ha.SetState("select.hall_scene", "o1", map[string]any{"options": opts, "friendly_name": "x"})
	e.eventually("the sync after the options", func() bool {
		for _, v := range e.br.Plan() {
			if v.EntityID == "select.hall_scene" && v.State != "mappable" {
				return true
			}
		}
		return false
	})
	var live, dry string
	for _, v := range e.br.Plan() {
		if v.EntityID == "select.hall_scene" {
			live = v.State + ": " + v.Reason
		}
	}
	if c := e.br.View().All["select.hall_scene"]; c != nil {
		dry = c.State + ": " + c.Reason
	}
	t.Logf("bridge: %s", live)
	t.Logf("dry (web preview, --dump-plan): %s", dry)
	if strings.HasPrefix(dry, "mappable") {
		t.Errorf("the dry plan calls a select with 256 options %q; the node refuses it: %s", dry, live)
	}
}

// TestX2ProbeOversizedStates (T16): one entity whose attributes push
// get_states over the client's 64 MiB frame limit.
func TestX2ProbeOversizedStates(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	first := e.br.Status().LastSync
	e.ha.SetState("sensor.outdoor", "1", map[string]any{"friendly_name": "Outdoor", "blob": strings.Repeat("x", 65<<20)})
	time.Sleep(6 * time.Second)
	s := e.br.Status()
	t.Logf("after 6 s: connected %v, authenticated %d times, last sync moved %v, sync error %.200q", s.HAConnected, e.ha.Authenticated(), s.LastSync.After(first), s.SyncError)
	e.ha.UpdateEntity("switch.coffee", map[string]any{"disabled_by": "user"})
	time.Sleep(3 * time.Second)
	_, served := e.endpoint("switch.coffee")
	s = e.br.Status()
	t.Logf("after the operator disabled switch.coffee: still served %v; sync error %.200q", served, s.SyncError)
	if served {
		t.Errorf("no sync while get_states is over the frame limit: switch.coffee, disabled, is still served; status %.300q", s.SyncError)
	}
}

// TestX2ProbeErrorTextInRing (T15): Home Assistant's error text of a
// failed call_service, as the status and a log record keep it.
func TestX2ProbeErrorTextInRing(t *testing.T) {
	var buf strings.Builder
	var mu sync.Mutex
	lw := writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) })
	log, ring := web.NewLogger(lw, new(slog.LevelVar))
	e := newEnv(t, expose.Rule{Assist: true}, func(s *setup) { s.Logger = log })
	e.commission()
	msg := "boom\n\x1b[2Jtime=2026-01-01T00:00:00Z level=INFO msg=forged‮" + strings.Repeat("M", 1<<20)
	e.ha.Fail = func(ha.ServiceCall) *ha.Error { return &ha.Error{Code: "home_assistant_error", Message: msg} }
	light := e.mustEndpoint("light.kitchen")
	if _, err := fabrictest.Invoke(e.ctx, e.pair, client.CommandAt(light, onoffdef.ClusterID, onoffdef.CmdOff), nil); err != nil {
		t.Fatal(err)
	}
	e.eventually("the failed call is counted", func() bool { return e.br.Status().CommandErrs > 0 })
	time.Sleep(100 * time.Millisecond)
	s := e.br.Status()
	var rec int
	for _, r := range ring.Records(0, 0) {
		if r.Message == "bridge.call_service_failed" {
			for _, a := range r.Attrs {
				rec += len(a.Value.String())
			}
		}
	}
	mu.Lock()
	out := buf.String()
	mu.Unlock()
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "bridge.call_service_failed") {
			line = l
		}
	}
	t.Logf("status.LastCommand %d bytes; ring record attributes %d bytes; stderr line %d bytes, starts %.160q", len(s.LastCommand), rec, len(line), line)
	if strings.Count(out, "msg=forged") > 0 && !strings.Contains(line, `\n`) {
		t.Errorf("the stderr line is not quoted")
	}
	if len(s.LastCommand) > 4096 || rec > 4096 {
		t.Errorf("Home Assistant's error text is kept whole: status.LastCommand %d bytes, the ring's record %d bytes (the ring keeps 1000 records)", len(s.LastCommand), rec)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// TestX2ProbeSiblingKnockedOut: one entity go-fabric refuses takes the
// other entities of its Home Assistant device off the bridge.
func TestX2ProbeSiblingKnockedOut(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	if _, ok := e.endpoint("switch.coffee"); !ok {
		t.Fatal("no endpoint")
	}
	// A cover with tilt joins the plug's device (a blind actuator with a
	// relay): go-hamapping v0.6.0 maps it, go-fabric v0.25.0 refuses it.
	e.ha.UpdateEntity("cover.living_blind", map[string]any{"device_id": "dev-shelly-1"})
	e.ha.SetState("cover.living_blind", "open", map[string]any{"device_class": "blind", "supported_features": 255, "current_position": 70, "current_tilt_position": 50, "friendly_name": "Blind"})
	e.eventually("the cover's verdict", func() bool {
		for _, v := range e.br.Plan() {
			if v.EntityID == "cover.living_blind" && strings.Contains(v.Reason, "refused") {
				return true
			}
		}
		return false
	})
	for _, v := range e.br.Plan() {
		if v.EntityID == "switch.coffee" || v.EntityID == "cover.living_blind" {
			t.Logf("%s: endpoint %d, %s: %.120s", v.EntityID, v.Endpoint, v.State, v.Reason)
		}
	}
	if _, ok := e.endpoint("switch.coffee"); !ok {
		t.Errorf("switch.coffee lost its endpoint to its device's refused cover")
	}
}
```

### `internal/bridge/x2_cover_test.go`

```go
// X2 probe (review phase 2). Not part of the tree.
package bridge_test

import (
	"context"
	"fmt"
	"testing"

	hamapping "github.com/SukramJ/go-hamapping"

	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// TestX2ProbeCoverBuild: which covers go-hamapping calls mappable and
// go-fabric refuses, by device_class and supported_features.
func TestX2ProbeCoverBuild(t *testing.T) {
	bad := 0
	for _, dc := range []any{nil, "blind", "shutter", "curtain", "awning", "garage", "shade", "window", "door", "damper", "gate"} {
		for _, feat := range []int{3, 11, 15, 143, 240, 255, 128, 139, 191} {
			attrs := map[string]any{"supported_features": float64(feat), "current_position": 50.0, "current_tilt_position": 50.0}
			if dc != nil {
				attrs["device_class"] = dc
			}
			snap := &ha.Snapshot{States: []ha.State{{EntityID: "cover.a", State: "open", Attributes: attrs}}}
			in := plan.Input{Rule: expose.Rule{Include: []string{"*"}}, Ports: camera.DryPorts}
			_, groups := plan.Assess(snap, in)
			for _, g := range groups {
				hp, mv := hamapping.Map(g.Device, g.Entities, plan.MapOptions(g.Regs, g.Names, in.Ports))
				_, err := hp.Build(context.Background())
				if err != nil && mv[0].State.String() == "mappable" {
					bad++
					t.Logf("device_class %v features %d: mappable, refused: %.160s", dc, feat, fmt.Sprint(err))
				}
				hp.Close()
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d cover shapes are mappable and do not build", bad)
	}
}
```

### `internal/bridge/x2_fuzz_test.go`

```go
// X2 probe (review phase 2). Not part of the tree.
package bridge_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	hamapping "github.com/SukramJ/go-hamapping"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/camera"
	"github.com/SukramJ/go-ha2fabric/internal/expose"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
	"github.com/SukramJ/go-ha2fabric/internal/plan"
)

// FuzzX2PlanBuild takes one entity's hostile state, attributes and device
// text the whole way the sync takes it without a node: plan.Assess,
// go-hamapping's Map, Build and Apply. Nothing panics, and an entity the
// dry plan calls mappable builds.
func FuzzX2PlanBuild(f *testing.F) {
	f.Add("select.a", "x", []byte(`{"options":["a","b"],"friendly_name":"S"}`), "Dev", "ACME", "SN1")
	f.Add("light.a", "on", []byte(`{"brightness":180,"color_temp_kelvin":2702,"hs_color":[28.4,65.7],"supported_color_modes":["hs","color_temp"],"min_color_temp_kelvin":0,"max_color_temp_kelvin":1e30}`), "D\x00", "V‮", "")
	f.Add("climate.a", "heat", []byte(`{"hvac_modes":["off","heat","cool","heat_cool"],"min_temp":-1e30,"max_temp":1e30,"target_temp_step":0,"temperature":1e300,"fan_modes":["auto"],"supported_features":4095}`), "", "", "")
	f.Add("sensor.a", "1e400", []byte(`{"device_class":"temperature","unit_of_measurement":"°C","state_class":"measurement"}`), "x", "", " ")
	f.Add("cover.a", "open", []byte(`{"current_position":-5,"current_tilt_position":1e9,"supported_features":255}`), "x", "y", "z")
	f.Add("fan.a", "on", []byte(`{"percentage":1e9,"preset_modes":["auto",""],"supported_features":63}`), "x", "y", "z")
	f.Add("media_player.a", "playing", []byte(`{"volume_level":-3,"is_volume_muted":"x","supported_features":1e18,"device_class":"tv"}`), "x", "y", "z")
	f.Add("water_heater.a", "eco", []byte(`{"min_temp":1e9,"max_temp":-1e9,"operation_list":["off"],"supported_features":15,"temperature":null}`), "x", "y", "z")
	f.Add("number.a", "5", []byte(`{"min":1e308,"max":-1e308,"step":0}`), "x", "y", "z")
	f.Add("vacuum.a", "cleaning", []byte(`{"supported_features":65535,"battery_level":1e9}`), "x", "y", "z")
	f.Add("lock.a", "jammed", []byte(`{"supported_features":1}`), "x", "y", "z")
	f.Add("valve.a", "open", []byte(`{"current_position":1e9,"supported_features":15}`), "x", "y", "z")
	f.Add("binary_sensor.a", "on", []byte(`{"device_class":"door"}`), "x", "y", "z")
	f.Add("event.a", "x", []byte(`{"event_types":["a",1],"event_type":"a","device_class":"button"}`), "x", "y", "z")
	f.Add("camera.a", "idle", []byte(`{"supported_features":3,"frontend_stream_type":"web_rtc"}`), "x", "y", "z")
	f.Fuzz(func(t *testing.T, id, state string, attrs []byte, dev, vendor, serial string) {
		if !strings.Contains(id, ".") || len(attrs) > 1<<16 {
			return
		}
		var a map[string]any
		_ = json.Unmarshal(attrs, &a)
		snap := &ha.Snapshot{
			States:   []ha.State{{EntityID: id, State: state, Attributes: a}},
			Entities: []ha.EntityEntry{{ID: "reg1", EntityID: id, DeviceID: "d1", HasEntityName: true, OriginalName: dev}},
			Devices:  []ha.DeviceEntry{{ID: "d1", Name: dev, Manufacturer: vendor, SerialNumber: serial}},
		}
		in := plan.Input{Rule: expose.Rule{Include: []string{"*"}}, Cameras: true, Ports: camera.DryPorts}
		verdicts, groups := plan.Assess(snap, in)
		for _, g := range groups {
			hp, mv := hamapping.Map(g.Device, g.Entities, plan.MapOptions(g.Regs, g.Names, in.Ports))
			plan.MergeVerdicts(verdicts, mv)
			devs, err := hp.Build(context.Background())
			if err != nil {
				for _, v := range mv {
					if v.State.String() == "mappable" {
						if strings.Contains(err.Error(), "FeatureMap") || strings.Contains(err.Error(), "max 255") {
							continue // HR-P2-adpt-33, -34
						}
						t.Fatalf("%s is mappable and does not build: %v", id, err)
					}
				}
				continue
			}
			_ = devs
			for _, p := range hp.Apply(context.Background(), id, adapt.State(&snap.States[0])) {
				_ = p.Err
			}
			hp.Close()
		}
	})
}
```

### `internal/adapt/x2_probe_test.go`

```go
// X2 probe (review phase 2). Not part of the tree.
package adapt_test

import (
	"testing"

	hamapping "github.com/SukramJ/go-hamapping"
	"github.com/SukramJ/go-hamqtt/model"

	"github.com/SukramJ/go-ha2fabric/internal/adapt"
	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// TestX2ProbeOptionTrimmed: the option go-hamapping delivers is the
// entity's own, byte for byte; Service trims it.
func TestX2ProbeOptionTrimmed(t *testing.T) {
	st := &ha.State{EntityID: "select.a", State: "Eco", Attributes: map[string]any{"options": []any{"Eco", "Eco ", " Boost"}}}
	e := adapt.Entity(adapt.Input{EntityID: "select.a", State: st, Device: adapt.OwnDevice("select.a", "a")})
	for _, opt := range []string{"Eco ", " Boost"} {
		sc, err := adapt.Service(model.Command{Entity: e, Method: hamapping.MethodSelectOption, Payload: []byte(opt)}, st)
		if err != nil {
			t.Fatal(err)
		}
		if got := sc.Data["option"]; got != opt {
			t.Errorf("the controller chose %q, select_option carries %q", opt, got)
		}
	}
}
```

### `internal/ha/x2_probe_test.go`

```go
// X2 probe (review phase 2). Not part of the tree.
package ha_test

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/SukramJ/go-ha2fabric/internal/ha"
)

// TestX2ProbeDecodeAmplification: the heap a get_states frame under the
// 64 MiB limit holds once decoded as the client decodes it (Decode, then
// the result as []json.RawMessage, then each ha.State).
func TestX2ProbeDecodeAmplification(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"id":1,"type":"result","success":true,"result":[`)
	for i := 0; b.Len() < 60<<20; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"entity_id":"sensor.e%d","state":"1","attributes":{"l":[`, i)
		for j := 0; j < 20000; j++ {
			if j > 0 {
				b.WriteByte(',')
			}
			b.WriteString("0")
		}
		b.WriteString(`]}}`)
	}
	b.WriteString(`]}`)
	frame := []byte(b.String())
	b.Reset()
	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	msgs, err := ha.Decode(frame)
	if err != nil {
		t.Fatal(err)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(msgs[0].Result, &raw); err != nil {
		t.Fatal(err)
	}
	states := make([]ha.State, len(raw))
	for i := range raw {
		if err := json.Unmarshal(raw[i], &states[i]); err != nil {
			t.Fatal(err)
		}
	}
	peak := new(runtime.MemStats)
	runtime.ReadMemStats(peak)
	raw, msgs = nil, nil
	runtime.GC()
	runtime.ReadMemStats(&m1)
	t.Logf("frame %d MiB; heap in use after decode, before GC %d MiB (total allocated %d MiB); kept by the %d states alone %d MiB", len(frame)>>20,
		(peak.HeapAlloc-m0.HeapAlloc)>>20, (peak.TotalAlloc-m0.TotalAlloc)>>20, len(states), (m1.HeapAlloc-m0.HeapAlloc)>>20)
	runtime.KeepAlive(states)
	runtime.KeepAlive(frame)
}
```

### `internal/camera/x2_probe_test.go`

```go
// X2 probe (review phase 2). Not part of the tree.
package camera_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SukramJ/go-ha2fabric/internal/camera"
)

// TestX2ProbeHACandidatesUnbounded (T14, Home Assistant's side): the
// camera integration's candidates and other events of one session, to
// the controller and against the idle end.
func TestX2ProbeHACandidatesUnbounded(t *testing.T) {
	up := &upstream{}
	h := boundedHost(t, up, camera.Options{IdleTimeout: 300 * time.Millisecond})
	sig := &signaller{}
	h.Attach(sig)
	ctx := context.Background()
	if _, err := h.ProvideOffer(ctx, offer(5)); err != nil {
		t.Fatal(err)
	}
	st := up.stream(0)
	st.push(session("ha-1"))
	const n = 5000 // the controller's side is held to camera.MaxSessionCandidates (256)
	huge := "candidate:" + strings.Repeat("A", 4000)
	for i := 0; i < n; i++ {
		st.push(candidate(huge, "0", 0))
	}
	eventually(t, "the candidates are relayed", func() bool { return len(sig.seen()) >= n })
	cands := 0
	for _, c := range sig.seen() {
		if strings.HasPrefix(c, "candidate ") {
			cands++
		}
	}
	// An event of a type the relay ignores still counts as activity.
	for i := 0; i < 12; i++ {
		time.Sleep(100 * time.Millisecond)
		st.push(map[string]any{"type": "noise"})
	}
	ended := false
	for _, c := range sig.seen() {
		ended = ended || strings.HasPrefix(c, "end ")
	}
	t.Logf("%d candidates of %d bytes each signalled to the controller in one session (the controller's own are held to %d); after 1.2 s of ignored events with IdleTimeout 300 ms the session ended: %v",
		cands, len(huge), camera.MaxSessionCandidates, ended)
	if cands > camera.MaxSessionCandidates {
		t.Errorf("Home Assistant's candidates of one session are not bounded: %d signalled", cands)
	}
	if !ended {
		t.Errorf("an event the relay ignores keeps the session from its idle end")
	}
}
```

### `internal/web/x2_probe_test.go`

```go
// X2 probe (review phase 2). Not part of the tree.
package web

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"runtime"
	"testing"
	"time"
)

// TestX2ProbeConnections (T10): how many connections one admitted peer
// holds open at once, each after one served request (kept alive for
// IdleTimeout), and what it costs.
func TestX2ProbeConnections(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := mustNew(t, Options{Listen: addr, Allow: []string{"127.0.0.1"}})
	go func() { _ = s.Run(ctx) }()
	time.Sleep(200 * time.Millisecond)
	before := runtime.NumGoroutine()
	const want = 800
	var conns []net.Conn
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	for i := 0; i < want; i++ {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			break
		}
		conns = append(conns, c)
		_, _ = c.Write([]byte("GET /api/v1/node/status HTTP/1.1\r\nHost: " + addr + "\r\n\r\n"))
		res, err := http.ReadResponse(bufio.NewReader(c), nil)
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("connection %d: %v %v", i, res, err)
		}
		_ = res.Body.Close()
	}
	time.Sleep(200 * time.Millisecond)
	t.Logf("%d of %d connections served and held open; goroutines %d -> %d", len(conns), want, before, runtime.NumGoroutine())
	if len(conns) == want {
		t.Errorf("no cap: one admitted peer holds %d connections for IdleTimeout", len(conns))
	}
}
```

### Probe of HR-P2-data-30

In `internal/bridge/x2_probe_test.go` of the copy, beside the probes of
the tracked report's appendix (same package, imports and `newEnv`):

```go
// TestX2ProbeResyncStarved: a registry change (the operator disables an
// exposed entity) while an entity the last sync did not know reports
// faster than the debounce.
func TestX2ProbeResyncStarved(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true}, func(s *setup) { s.Debounce = 200 * time.Millisecond })
	e.commission()
	if _, ok := e.endpoint("switch.coffee"); !ok {
		t.Fatal("no endpoint")
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(50 * time.Millisecond):
				e.ha.SetState("sensor.x2_new", fmt.Sprint(i), map[string]any{"friendly_name": "new"})
			}
		}
	}()
	time.Sleep(300 * time.Millisecond)
	e.ha.UpdateEntity("switch.coffee", map[string]any{"disabled_by": "user"})
	gone := func() bool { _, ok := e.endpoint("switch.coffee"); return !ok }
	deadline := time.Now().Add(5 * time.Second) // 25 debounces
	for time.Now().Before(deadline) && !gone() {
		time.Sleep(20 * time.Millisecond)
	}
	starved := !gone()
	last := e.br.Status().LastSync
	close(stop)
	<-done
	e.eventually("the endpoint goes once the entity is quiet", gone)
	if starved {
		t.Errorf("switch.coffee, disabled, kept its endpoint for 5 s (25 debounces) while sensor.x2_new reported every 50 ms; last sync %s before the flood ended", time.Since(last).Round(time.Millisecond))
	}
}
```
