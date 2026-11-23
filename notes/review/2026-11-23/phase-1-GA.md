# Phase 1, unit G-A — the plan side

Reviewer G-A (Opus). Baseline `5eefc0a`, branch `review/p1`. Files:
`internal/plan`, `internal/bridge`, `internal/adapt`, `internal/expose`,
`internal/exposure` and their tests. References at their pins: `gf:`
go-fabric v0.25.0, `hm:` go-hamapping v0.6.0, `core:` Home Assistant
2026.10.3. M = measured (read, or a probe ran), I = inferred.

Probes ran in a `git archive 5eefc0a` copy under the session scratchpad
(`…/scratchpad/GA/`), never in the worktree.

## Findings

### HR-P1-adpt-30 — medium — a climate range setpoint completes from a stale bound

- `internal/adapt/command.go:134-149`; `internal/bridge/command.go:56-61`;
  `internal/bridge/bridge.go:645-652`.
- Claim. go-hamapping delivers the two bounds of a heat/cool range as two
  commands, `temperature_low` and `temperature_high`, for a write and for
  a change (`hm:rows_gen.go:818`, `:820`, `:823-824`), so a controller
  writing both setpoints, or a `SetpointRaiseLower` in mode Both, gives two
  commands. `adapt.Service` fills the other bound from `cur`, the bridge's
  last-known state (`command.go:145`), which only the worker updates when
  Home Assistant's `state_changed` arrives (`bridge.go:645-652`). The
  command goroutine runs the second command right after the first one's
  call returns, and nothing waits for the first one's state. Two results:
  (a) the second call sends the old value of the first bound and undoes
  it; (b) moving the range up past the old high, the first call carries
  `low > high` and Home Assistant refuses it (`core:homeassistant/
  components/climate/services.py:121-131`, `low_temp_higher_than_high_temp`),
  so the low is lost even with no race. The controller saw its write
  succeed in both cases.
- M: code read, and probe `TestGAProbeRangeStale`
  (`internal/adapt/ga_probe_test.go` in the copy):
  `first map[target_temp_high:24 target_temp_low:20]; second map[target_temp_high:26 target_temp_low:18]`.
  I: how often a controller writes both bounds in one action (a dual
  slider in Auto does), and how long the state takes (a cloud thermostat
  answers after a poll).
- Fix. The right place is go-hamapping (theme `adpt`, `upstream`
  candidate): a range command that carries both bounds, read from the
  Thermostat server's two attributes, which already hold the controller's
  write. Until then, here: the command loop keeps, per climate entity,
  the bounds it last sent and uses them as `other` until a
  `state_changed` shows them or a short time passes. Two
  `temperature_low`/`temperature_high` commands queued back to back for
  one entity can also be merged into one call.
- Red test. A bridge end-to-end test: in `heat_cool`, the controller
  writes `OccupiedHeatingSetpoint` and `OccupiedCoolingSetpoint` in one
  write request (new low above the old high), and the fake Home Assistant
  delays its `state_changed`. Expected: the last `climate.set_temperature`
  carries both new values, and no call has `low > high`. Today the last
  call carries the old low.

### HR-P1-err-31 — low — a failed batch operation is recorded as served and never retried

- `internal/bridge/bridge.go:427-452`, `:489-497`, `:590-593`.
- Claim. `node.Batch` is not a transaction: each `Add`, `Replace`,
  `Remove` and `Forget` applies or fails on its own, and the failure is
  joined into the result (`gf:node/aggregator.go:361-427`; refusals in
  `Add` `:431-469`, `Replace` `:471-519`: `ErrDuplicateKey`,
  `ErrPolicyMismatch`, `errNoMatterTime`, `ErrUnknownKey`, `ErrStopped`).
  The bridge logs the joined error (`:443-445`), then sets
  `b.groups = next` anyway (`:452`), closes the replaced groups' old plans
  (`:449-451`), counts the endpoints from `next` (`:461-467`) and clears
  `SyncError` (`:496`). So:
  - a group whose `Add` failed is held as served with its signature; the
    next syncs keep it (`:414-418`) and never add it again until the
    signature moves, and then `plan.Diff` asks for `Replace` of a key the
    node never held, which fails `ErrUnknownKey` again;
  - a failed `Replace` leaves the old device served while its plan is
    closed and the states go to the new, unserved devices: the served
    endpoint's values freeze;
  - `Resync` (the web UI's reassemble) returns nil (`:590-593`) and the
    status shows no error and the wrong endpoint count.
- M: code read and `gf` read. I: which refusal fires in practice — the
  composition gives none today by construction (keys carry the group's
  UID; the node's policies are the devices'); a reassembly error is the
  likeliest.
- Fix. Run the operations one by one inside the batch and keep each
  error by key; a group with a failed operation goes into `b.groups` with
  an empty signature (so the next sync rebuilds and adds it again) or
  with the keys that did apply only; set `SyncError` (or a status
  warning) to the joined error, so `Resync` reports it.
- Red test. Bridge test: before a sync, the node's Aggregator holds an
  endpoint under the key the bridge will add for one entity; `Resync`
  returns an error naming the key, and after the foreign endpoint is
  removed the next `Resync` adds it.

### HR-P1-conc-32 — low — worker work is dropped on a full queue: the unreachable marking, the forget, the re-sync

- `internal/bridge/bridge.go:214-221`, `:236-259`, `:280-283`, `:313-325`,
  `:367-370`.
- Claim. `enqueue` drops the closure when the 1024-slot queue is full and
  calls `resyncSoon` instead. That covers a state change, but not:
  (a) `Disconnected`'s marking of every endpoint unreachable — the re-sync
  it schedules returns at once while Home Assistant is away (`:368-370`),
  so the endpoints stay reachable with stale values through the outage;
  (b) `noteRemoved` — the removed entity's endpoint number is removed, not
  forgotten, and a re-created entity gets the old number back (the case
  `:292-301` exists to prevent). And `resyncSoon`'s timer drops its sync
  silently when the queue is full at the moment it fires (`:320-323`),
  with no log and no retry, so a registry change waits for the next event.
- M: code read. I: a full queue needs a burst of more than 1024 events
  faster than the worker applies them (the log line `bridge.queue_full`
  names it as expected).
- Fix. Level-triggered state instead of droppable closures: `sync`'s
  not-connected path marks the endpoints unreachable (it is the re-sync
  `enqueue` falls back to); `Event` records the removed entity under
  `b.mu` (or a small mutex) rather than through the queue; the timer
  re-arms itself when the queue is full.
- Red test. Package test with a long debounce: fill `b.work` while the
  worker is blocked, call `Disconnected`, release; expect every endpoint
  `Reachable false`. Same for a `remove` event: expect a `bridge.forget`.

### HR-P1-conc-33 — low — a queued command runs after its entity left the exposure

- `internal/bridge/command.go:25-32`, `:45-89`; `internal/bridge/bridge.go:175`.
- Claim. Commands wait in a 256-slot queue and run one at a time, each up
  to 30 s (`command.go:21`). `command` calls `adapt.Service` and
  `CallService` without asking whether the entity is still served. A
  command queued before the operator turned the entity off (web UI,
  exclude list, label) still reaches Home Assistant after the sync that
  took its endpoint away. Decision 11 bounds a controller to the exposed
  entities' services; this holds at the time of the request, not at the
  time of the call.
- M: code read. I: the window is the queue's backlog, seconds to minutes
  only with a slow service.
- Fix. The sync publishes the set of served entity ids under `b.mu`
  (beside `states`); `command` drops, logs and counts a command whose
  entity is not in it.
- Red test. Hold the fake's `call_service` open, queue a command for
  entity B behind one for A, turn B off and `Resync`, release; expect no
  call for B.

### HR-P1-cfg-34 — low — an exclude label named by its name fails open when the label is renamed

- `internal/expose/expose.go:89-98`, `:151-153`; `internal/plan/input.go:93-103`.
- Claim. A label value resolves by name, case-insensitively, else is used
  as a label id as given (D1, `docs/by_design.md` "Unresolved label
  names"). For an include label that is fail-closed; for an exclude label
  it is fail-open: after a rename in Home Assistant (same `label_id`,
  `core:helpers/label_registry.py:183-224`) the configured name resolves
  to nothing, the exclusion stops and the Assist rule (default on) can
  expose the entity to every controller. The status warning
  `label_unresolved` appears, but its text (`input.go:100`) does not say
  that an exclusion is no longer in force.
- M: probe `TestGAProbeExcludeLabelRenamed`
  (`internal/expose/ga_probe_test.go` in the copy):
  `before rename: exposed=false "excluded by label 'No Matter'"; after rename: exposed=true "exposed to Assist"; unresolved=[No Matter]`.
- Fix (owner question 2). Minimum: the warning names an unresolved
  *exclude* value as "excludes nothing now", and `docs/operations.md`
  recommends the `label_id` for exclude labels. Further: keep the label
  id each value resolved to in the last sync and keep using it while the
  name is gone (a rename keeps the id), which closes the gap without
  slug guessing.
- Red test. `expose`/`plan` test: rule with `ExcludeLabels: ["No Matter"]`,
  the label renamed; expect the entity still excluded (with the further
  fix) or a warning whose text names the lapsed exclusion (minimum).

### HR-P1-cfg-35 — low — a corrupt `selection.json` exposes what the operator turned off on the configured node

- `internal/selection/selection.go:12-14`, `:95-101`;
  `cmd/ha2fabric/main.go:83`; `cmd/ha2fabric/nodes.go:422`;
  `internal/exposure/exposure.go:344-351`.
- Claim. A file that does not parse is moved aside and the store starts
  empty. The configured node's input is the configured rule with the
  overrides (`main.go:83`), so every entity the operator turned *off* in
  the web UI goes back to the rule — with Assist on, a lock or cover that
  Assist exposes reaches the controllers again. A registry node's rule is
  empty (`nodes.go:422`), so it exposes nothing: fail-closed there. The
  web UI shows `selection_corrupt`; nothing else says what is exposed
  now. (Overlap: unit S, item 4 / T23 owns the file; this row is the
  exposure side.)
- M: code read. I: a corrupt file needs a fault outside the daemon (the
  write is a temporary file and a rename, `selection.go:173-194`).
- Fix (owner question 3). Either accept and document it in
  `docs/operations.md` (the warning says "entities turned off here are
  exposed by the rule again until you restore or re-apply them"), or
  make the configured node fail closed while recovered: expose only the
  entities the last good file enabled — which needs a last-good copy
  beside the file (`selection.json.bak` written before each rename).
- Red test. Bridge test with a corrupt file and an entity Assist exposes:
  today it is served; with the fail-closed fix it is not, and the
  warning text names it.

### HR-P1-test-38 — low — negative cases missing on the plan side

- `internal/plan/diff_test.go:56-163`, `internal/plan/plan_test.go`,
  `internal/bridge/*_test.go`, `internal/adapt/adapt_test.go:220-403`.
- Claim. Not covered by a test today: an entity moved to another device
  (its key changes: the old key is removed and keeps its number, the new
  one added) in `TestDiff` and end to end; a batch with a failed
  operation (HR-P1-err-31); the two range setpoints in sequence
  (HR-P1-adpt-30); a command for an entity no longer served
  (HR-P1-conc-33); a dropped worker closure (HR-P1-conc-32); an exclude
  label renamed (HR-P1-cfg-34); `Assess`'s independence of the input
  order (a probe holds it, see below; no standing test).
- M: test lists read (`grep -n '^func Test'`, the `TestDiff` case names).
- Fix. One test each, the red ones with their findings; the order test
  as a standing guard (shuffle states, entities and devices; compare
  keys and signatures).

### HR-P1-data-36 — note — control and bidirectional characters reach NodeLabel (upstream candidate)

- `internal/adapt/adapt.go:312-321`; `hm:plan.go:358-388`;
  `gf:endpoint/spec.go:152-177`.
- Claim. An entity's name (untrusted, decision 11) reaches the
  endpoint's NodeLabel with NUL, newline, ESC and U+202E intact; go-fabric
  cuts it to 32 bytes on a rune boundary and nothing filters control or
  bidi characters. Invalid UTF-8 never arrives: `encoding/json` replaces
  it with U+FFFD on decode. Vendor and serial are cut to 32 bytes by
  go-hamapping (`hm:plan.go:349`, `:387`); model, versions, area and
  configuration URL are carried in `model.Device` but served nowhere
  (`grep` of `hm` finds no reader), only in the signature.
- M: probe `TestGAProbeHostileLabel` (`internal/plan/ga_probe_test.go` in
  the copy): `FriendlyName "A\x00B\nC\x1b[31m‮evil��ÄÖÜä" (31 bytes)`.
  I: whether Matter's character strings forbid C0 controls (not read);
  what a controller's UI makes of them.
- Fix. Upstream: go-fabric's `ComposeNodeLabel` (one place for every
  host) drops C0/C1 controls and bidi overrides before the cut; not a
  workaround here.
- Red test. go-fabric: `ComposeNodeLabel("A\x00B‮C", "")` has no
  control or bidi character.

### HR-P1-data-37 — note — some controller values reach Home Assistant unclamped; Home Assistant validates them

- `internal/adapt/command.go:131-133`, `:137-149`, `:203-206`, `:332-342`.
- Claim. `temperature`, `target_temp_low/high`, `hs_color` and `xy_color`
  pass as parsed (finite only). Home Assistant refuses out-of-range
  values (`core:homeassistant/components/climate/services.py:117-160`;
  `core:homeassistant/components/light/const.py:174-195`, where
  `hs_color`, `xy_color`, `rgb_color` and `color_temp_kelvin` are
  also `Exclusive` in one colour group, so a payload with two of them
  would be refused; go-hamapping's rows send one group per change,
  `hm:rows_gen.go:2264-2267`). Not a defect; recorded so the asymmetry
  with the clamped values is a decision.
- M: probe `TestGAProbeRangeStale` (second half):
  `huge temperature: map[temperature:1e+308] <nil>`;
  `light colour: map[hs_color:[-1e+09 1e+09] xy_color:[5 -5]] <nil>`.
- Fix. None needed; optionally a test that pins "one colour group per
  call".

### HR-P1-bound-39 — note — a moved entity's old key keeps its endpoint number for good

- `internal/plan/diff.go:94-102`, `:130-134`.
- Claim. An entity whose registry device changes gets a new stable key
  (`hm:plan.go:358-370`: the device UID, "/", the registry id). `Diff`
  drops the old key with `Remove`, which keeps its number reserved in the
  store (`gf:node/aggregator.go:272-282`); only a registry `remove` event
  leads to `Forget`. Reservations grow with device moves and
  un-exposures, bounded by Home Assistant's churn.
- M: code read. I: the size is small in practice.
- Fix. `Diff` forgets a dropped key whose entity id is planned under
  another key in `next` (the entity moved); a test case in `TestDiff`.

## The five items of unit G-A

### 1. `plan`: purity, determinism, overrides, diff, input validation

- Pure: no goroutines, no node; `Assess`, `Dry`, `Candidates`, `Diff`
  read their arguments only (M). `Input.With`/`Without` clone both maps
  (`input.go:59-88`, M).
- Determinism (M): candidates in sorted entity id order
  (`plan.go:222-226`), groups in first-entity order, entities in id order
  within a group, the signature from `json.Marshal` (sorted map keys);
  `Diff` sorts removed groups and trailing forgets (`diff.go:103`,
  `:136`); `Labels.Resolve` iterates sorted ids (`expose.go:91`). Probe
  `TestGAProbeAssessDeterministic`: `groups=28 stable over 50 shuffles`
  (states, entities and devices shuffled; keys and signatures equal).
- Overrides: above the lists, below disabled and hidden
  (`expose.go:136-147`, M); `Effective` takes both from the store
  (`input.go:48-55`).
- `Diff` (M, `TestDiff` and the e2e tests): **renamed** entity id — same
  registry id, same key, `Replace`, number kept (`TestRenameKeepsTheEndpoint`);
  **moved** to another device — new key: `Remove` old, `Add` new
  (HR-P1-bound-39, untested: HR-P1-test-38); **re-typed** — signature
  moves, `Replace` under the same key, the Descriptor the change a
  controller sees (`gf:node/aggregator.go:257-268`); **removed** —
  forgotten when the registry said `remove`, else removed and kept;
  **removed and re-added** within the debounce — the forget is dropped,
  the endpoint kept (`diff.go:88-93`); after a sync Home Assistant
  returns the old registry id and the key gets a fresh number. Correct.
- Input validation: patterns are checked at configuration
  (`expose.Rule.Validate`); override keys and names are not validated
  here (unknown ids are ignored; names are cut downstream). No finding.

### 2. `bridge` against go-fabric v0.25.0

- One worker owns `groups`, `owner`, `known`, `forget`, `hosts`; `b.mu`
  guards `states`, `verdicts`, `last`, `status` and is never held across
  a node call, a Home Assistant call or a plan call (M). No lock order
  with go-fabric's locks: `onCommand` and `onDiscard`, which run inside a
  controller's request, take only `b.mu` briefly and never block (M).
- Add/remove/replace in one `Aggregator().Batch`; not a transaction, and
  the bridge treats it as one: HR-P1-err-31.
- While a controller is subscribed: `Replace` keeps the number and the
  PartsList, reassembles once, unbinds the old device after
  (`gf:node/aggregator.go:257-268`, `:361-427`); the old plan is closed
  after the batch, so a command already queued from it still runs
  (HR-P1-conc-33 for the unexposed case).
- Half-way failure: HR-P1-err-31. Dropped worker closures: HR-P1-conc-32.
- Camera wiring: a host per camera entity on Map's first ask, attached
  after the build, closed when no built device carries it
  (`camera.go:113-161`, M). A camera host attached to a device whose
  `Add` failed stays open (follows HR-P1-err-31). The host's session and
  subscription handling is unit S's.
- Commands: queued (256), one goroutine, 30 s each, failure re-applies
  Home Assistant's state (`command.go:77-86`, M).

### 3. Controller command → `adapt` → `call_service`: the complete reach

The entity id is always the plan's (`cmd.Entity.Key()`,
`command.go:92`), the domain its prefix, and every service is a constant
of `services`/`payloads` or a `case` below; anything else is
`ErrCommand` (`command.go:225`). Every text value is a go-hamapping
constant or an entity's own list (the enums at `hm:rows_gen.go:816`,
`:1021-1025`, `:1678`; the select option from `desc.options`,
`:3654`), never a controller's bytes. M.

| Domain | Service | Data keys (value, bound) | Source |
| --- | --- | --- | --- |
| button | press | — | `command.go:55` |
| climate | turn_on, turn_off, toggle | — | `:59` |
| climate | set_hvac_mode | `hvac_mode` (row enum: heat_cool, cool, dry, fan_only, heat, off) | `:124-128` |
| climate | set_temperature | `temperature` (finite float) | `:129-133` |
| climate | set_temperature | `target_temp_low` + `target_temp_high` (one from the controller, one from the state) | `:134-149` |
| climate | set_fan_mode | `fan_mode` (row enum: auto, low, medium, high, off) | `:173-177` |
| vacuum | pause, start, return_to_base, stop | — | `:62`, `:183-199` |
| valve | open_valve, close_valve | — | `:65` |
| media_player | turn_on, turn_off, toggle, media_play, media_pause, media_stop, volume_up, volume_down | — | `:70-73`, `:280-301` |
| media_player | volume_mute | `is_volume_muted` (bool) | `:207-211`, `:291-293` |
| media_player | volume_set | `volume_level` (0…1, two places, from level 0…254) | `:212-221` |
| script | turn_on | — | `:75` |
| scene | turn_on | — | `:78` |
| automation | trigger | — (skip_condition defaults true) | `:81` |
| switch | turn_on, turn_off | — | `:39` |
| fan | turn_on, turn_off | — | `:41` |
| fan | set_percentage | `percentage` (int 0…100, clamped) | `:150-154` |
| fan | set_preset_mode | `preset_mode` (row enum) | `:155-159` |
| fan | oscillate | `oscillating` (bool) | `:160-167` |
| fan | set_direction | `direction` (forward, reverse) | `:168-172` |
| cover | open_cover, close_cover, stop_cover | — | `:44` |
| cover | set_cover_position | `position` (int 0…100, clamped) | `:114-118` |
| cover | set_cover_tilt_position | `tilt_position` (int 0…100, clamped) | `:119-123` |
| lock | lock, unlock | — (no code) | `:46` |
| select | select_option | `option` (one of the entity's options) | `:178-182` |
| water_heater | set_operation_mode | `operation_mode` ("off" or the first non-off of the entity's list) | `:246-272` |
| water_heater | turn_on, turn_off | — | `:258-263` |
| water_heater | set_temperature | `temperature` (finite float) | `:202-206` |
| light | turn_off | — | `:319-322` |
| light | turn_on | `brightness` (int 0…255, clamped), `color_temp_kelvin` (int 15…1 000 000, clamped, >0 only), `hs_color` [h, s], `xy_color` [x, y] (unclamped), `rgb_color` (ints 0…255, clamped) | `:308-351` |

Bounded to the exposed entity and its services (decision 11): yes, at the
time of the request; HR-P1-conc-33 for the time of the call. Conversions:
`clampInt` clamps in float before converting (HR-P0-data-01's fix holds:
`command.go:402-416`, `TestServiceClampsOutOfRange`); rounding half away
from zero; temperatures in the installation's unit, the row converting
from Matter's 0.01 °C (`hm:rows_gen.go:818-824`; `adapt.go:280-285`).
Unknown method or payload: `ErrCommand`, logged and counted
(`bridge/command.go:61-66`). Out-of-range: clamped or left to Home
Assistant's validation (HR-P1-data-37). The range: HR-P1-adpt-30.

### 4. `expose` and `exposure`

- Order (M, `expose.go:135-167`): disabled, hidden, override, exclude
  (ids, globs, labels), include (ids, globs, labels), Assist. An entity's
  labels are its own and its device's (`plan.go:239-246`). Labels:
  HR-P1-cfg-34.
- Selection **missing** or **empty**: no overrides, the rule alone.
  **Corrupt**: moved aside, the rule alone on the configured node,
  nothing on a registry node, warning `selection_corrupt`: HR-P1-cfg-35.
- Agreement with `selection`: `Apply` stores an override only where it
  differs from the rule without it, or carries a name (`exposure.go:288-317`);
  `check` refuses unknown ids and enabling an unmappable entity
  (`:215-233`); the preview is `plan.Dry` on the same input with the
  updates (`:252-274`). Consistent (M, `TestProviderApply`).

### 5. Hostile entity data into go-fabric attributes

NodeLabel: 32 bytes, cut on a rune boundary by go-fabric
(`gf:endpoint/spec.go:152-177`), verbatim names too (`hm:plan.go:375-388`).
VendorName and SerialNumber: 32 bytes by go-hamapping (`hm:plan.go:349`,
`:387`). Select option labels: 64 bytes, 256 options, by go-hamapping
(`hm:options.go:77-84`). UniqueID: a hash of the stable key, not the text
(`gf:endpoint/materialize.go:250`). Invalid UTF-8: replaced on JSON
decode. Control and bidi characters: pass (HR-P1-data-36). The other
string attributes a row fills from states are go-hamapping's (boundary,
decision 9; G-B).

## Counts

| Severity | Count | IDs |
| --- | --- | --- |
| high | 0 | — |
| medium | 1 | adpt-30 |
| low | 6 | err-31, conc-32, conc-33, cfg-34, cfg-35, test-38 |
| note | 3 | data-36, data-37, bound-39 |

No held item: nothing here is an open exploitable weakness (cfg-34 and
cfg-35 need the operator's own action or a fault, and carry a warning).

## Not read, and why

- `internal/camera/camera.go` beyond `Host.Close` (unit S).
- go-hamapping's row machinery beyond the rows and option limits cited
  (decision 9; G-B owns the boundary).
- go-fabric's reassembly under a live subscription beyond the `Batch` and
  `Replace` docs (go-fabric's own review).
- The bridge's end-to-end tests line by line (read their names and the
  reachability and resync paths).

## Probes

In the copy `…/scratchpad/GA/` (`git archive 5eefc0a`):

- `internal/adapt/ga_probe_test.go` `TestGAProbeRangeStale` (adpt-30,
  data-37).
- `internal/expose/ga_probe_test.go` `TestGAProbeExcludeLabelRenamed`
  (cfg-34).
- `internal/plan/ga_probe_test.go` `TestGAProbeHostileLabel` (data-36),
  `TestGAProbeAssessDeterministic` (item 1).

## Questions for the owner

1. HR-P1-adpt-30: fix in go-hamapping (a range command carrying both
   bounds) with a local stop-gap (the last sent bounds per entity), or the
   stop-gap only? **Recommendation: both** — the issue at go-hamapping is
   the right cut; the stop-gap fixes users now and goes when the row
   delivers both bounds.
2. HR-P1-cfg-34: the exclude label after a rename — warning text and docs
   only, or keep the last resolved id? **Recommendation: keep the last
   resolved id** (no slug guessing, so D1 stands) plus the warning text.
3. HR-P1-cfg-35: a corrupt overrides file on the configured node — accept
   and document, or fail closed with a last-good copy? **Recommendation:
   accept and document** in phase 1 (the write is atomic; a corrupt file
   is a fault outside the daemon), with the warning text naming what is
   exposed again; revisit if unit S's T23 answer differs.
