# Phase 4, run 1 — unit P (the plan side)

Reviewer P (Opus). Baseline `c3a9770` (worktree `review/p4`), delta
`d16f3ab..c3a9770` read first (`notes/reads/read-review-p4-delta.md`), then
the full cut. Brief `notes/briefs/2026-11-26-review-p4.md`, "Unit P". IDs
`HR-P4-<theme>-40..59`. Every `file:line` is at `c3a9770`; references at
their pins: `gf:` go-fabric v0.26.0, `gf27:` v0.27.0, `hm:` go-hamapping
v0.8.0, `hq:` go-hamqtt v0.41.0. M = measured (a probe ran), I = inferred
(read only).

## Summary

| ID | sev | one line |
| --- | --- | --- |
| HR-P4-err-40 | low | a refused Replace leaves the old endpoint served with its plan closed: its state, reachability and commands are dead until a retry succeeds (HR-P1-err-31's second bullet, fix incomplete) |
| HR-P4-err-41 | low | the web UI loses the plan's reason for an exposed camera with the camera option off and for a state over 256 KiB: the item is Unmappable with an empty reason, and enabling such an item is refused with an empty reason |
| HR-P4-adpt-42 | low (I) | a failed reassembly is never retried: the next sync has no operation, go-fabric's batch then does not reassemble, and the sync clears its failure (upstream candidate, go-fabric) |
| HR-P4-bound-43 | note | the group signature is built by `+=` in a loop: quadratic in a device's entity count (measured 13.6 s for one `Assess` of 2 000 large selects on one device) |

Counts: 0 high · 0 medium · 3 low · 1 note.

No held file: no new exploitable security item.

## The delta (`d16f3ab..c3a9770`) in this unit

Only `internal/bridge/bridge.go` changed in substance (R7, #78); the rest
of the unit's changes are import paths (`thingfabric.dev`, `sukramj.dev`)
and citation pins (`adapt.go:184`, `input.go:27-28`, `exposure.go:50`,
`bridge.go:15`), each checked against its pin.

- `bridge.go:671-709` (was the per-call error read): the calls' errors are
  kept by index; `ran` records whether `fn` ran, and a batch refused before
  `fn` (`node.ErrStopped`, `gf:node/aggregator.go:549-559`) marks every
  operation failed. Read against `gf:node/aggregator.go:520-614`: correct
  — `Partition.Batch` returns `ErrStopped` before `fn` and changes nothing.
- `bridge.go:809-890` `settle`: what applied is read from
  `Aggregator().Endpoints()` after the batch. Add/Replace applied ⇔ the set
  serves the very `*device.Device` under the key (`:840-843`); Forget
  applied ⇔ its call answered nil and no endpoint serves the key, itself or
  as a part (`:846-849`, `:857-865`, `servedKeys` `:892-906`). Checked
  against every path of `gf:node/aggregator.go` `Batch.Add` (`:622-655`),
  `Batch.Replace` (`:696-747`), `replaceLocked` (`:751-778`),
  `replaceWithheldLocked` (`:788-794`), `Batch.Remove` (`:805-850`) and
  `gf:node/reserve.go:425-462` `Batch.Forget` / `:467-483`
  `commitForgetLocked`:
  - a held Add refused at the commit: the set has no endpoint, or another
    one, under the key → not applied. Right.
  - a Replace held and refused, or applied and undone at the commit
    (decision 585: "a Replace puts the replaced endpoint back"): the set
    holds the old device → not applied. Right — but see HR-P4-err-40 for
    what the bridge then serves.
  - a Replace by a declaration the specification level withholds
    (`replaceWithheldLocked`) drops the key from the set and would leave the
    group asking Replace of a key the set no longer holds (`ErrUnknownKey`
    at every later sync). Not reachable here: `config.Node` sets no
    `Surface` policies, so the level is the pin's (`gf:surface/level.go:
    66-83`) and nothing is withheld (I). No finding.
  - a Forget whose key another endpoint serves at the end: re-recorded.
    A Forget of a key nothing stores is no error (`gf:node/reserve.go:
    395-399`), so Diff's "forget a key never served" cannot loop. Right.
  - the settle comment's two premises hold: the daemon creates no partition
    (one node per registry entry), and a part key `<key>#n`
    (`hm:plan.go:465`) is no entity's key (Home Assistant entity ids and
    registry ids carry no `#`).
- Store-side failures of an applied Forget (the erasure) reach the bridge
  only inside `Batch`'s joined error; that is go-fabric #616's scope ("cannot
  see the store-side failures of an applied one") and not re-reported. With
  v0.27.0 the case becomes concrete (see "v0.27.0").

## Findings

### HR-P4-err-40 — low — a refused Replace leaves the old endpoint served, with its plan closed

- `internal/bridge/bridge.go:714-716` (the replaced groups' old plans
  closed regardless of the outcome), `:872-889` (a refused Replace keeps its
  key with the **new**, unserved device in `g.devices`), `:738-742`,
  `:1086-1101` (states applied to `g.plan`/`g.devices`), `:350-365`
  (`unreachable` marks `g.devices`), `:744-750` (the verdict's endpoint from
  the new plan).
- Claim. When go-fabric refuses a Replace (held and refused at the commit,
  or applied and undone; `gf:node/aggregator.go:419-436`), the node keeps
  serving the **old** device under the key. The bridge has already closed
  the old group's plan (`hm:plan.go:600-608`: its watchers, the source of
  every controller command, are unsubscribed) and from then on carries
  every state onto the new device, which no controller sees. So, until a
  retry's Replace applies:
  - the served endpoint's attribute values and reachability freeze (an
    entity that goes `unavailable` stays reachable on the wire, measured);
  - a Home Assistant outage does not mark it unreachable (`unreachable`
    walks `g.devices`, the new devices; I);
  - a controller's write to it reaches no `call_service` (the old plan's
    watchers are gone; I from `hm:plan.go:600-608`, `:566-579`);
  - the web UI shows the entity without an endpoint (verdict endpoint 0,
    measured) while endpoint 3 is served.
  HR-P1-err-31's second bullet named exactly this ("a failed `Replace`
  leaves the old device served while its plan is closed and the states go
  to the new, unserved devices: the served endpoint's values freeze",
  `phase-1-GA.md:70-72`); its fix (`46a71c7`, `phase-1-fix-F2.md:66-73`)
  added the retry (no signature → rebuilt) but not the served side. With a
  persistent refusal the retry fails with back-off up to 5 min
  (`:442-443`, `:471-487`) and the endpoint stays dead for as long.
- Reach (I). No refusal of a Replace is known that this composition
  produces at go-hamapping v0.8.0 by itself: part keys are `<key>#n`, no
  surface policy is set, no row declares a server that needs the node's
  Matter time. The probes reach it with a foreign endpoint holding the part
  key, as `TestRefusedReplaceIsRetried` does. Hence low.
- M: probe `TestP4RefusedReplaceKeepsServedEndpointLive` (appendix):
  `status endpoints=27 sync_error="node: an endpoint with this StableKey is
  already added: ha:dev-zha-2/reg-binary_sensor-front_door#1"`, `verdict
  endpoint=0 ok=false, served endpoint number=3`, then `never: the served
  endpoint follows the state` (20 s, two retries ran meanwhile).
- Fix. Keep the old group live for the keys whose Replace was refused:
  `settle` returns the group keys with a refused Replace; for each, the old
  group is kept beside the new one (e.g. `group.stale *group`) and its plan
  is **not** closed at `:714-716`; `apply`, `unreachable` and the verdict's
  endpoint use the stale plan for the refused keys; the stale plan is closed
  when a later batch applies those keys' Replace or removes the group. A
  simpler cut (if the fix wave prefers it): a group whose Replace was
  refused keeps the old group whole (old plan open, `Sig` ""), the new plan
  is closed, and keys of that group whose Replace or Add did apply are
  replaced again by the retry. Either way `settle` decides it, not the
  close loop.
- Red test: the probe, extended by one controller write to the served
  endpoint after the refusal that must reach the fake's `call_service`.

### HR-P4-err-41 — low — the web UI loses the plan's reason for a camera with the option off and for a state over 256 KiB

- `internal/plan/plan.go:200-212` sets `Verdict.State`/`Reason` for the two
  pre-Map verdicts (`CameraOff`, `OversizedReason`) but not `Verdict.Full`;
  `internal/exposure/exposure.go:163-179` builds the web UI's item from
  `Full` (`c.Full` when exposed, `v.all[id].Full` when the rule leaves it
  out), so the item is `eligibility.Verdict{}` — `StateUnmappable` (the
  zero value, `gf:contract/matter.go:1155`) with an empty reason.
  `exposure.go:230-231` then refuses enabling such an item as
  `"<id>" cannot be exposed: ` with nothing after the colon.
- Claim. Decision 52 made an oversized entity "unmappable with a reason …
  the operator sees the reason"; `--dump-plan` and the log do
  (`plan.Verdict.Reason`), the web UI's selection page, where an operator
  looks first, shows none. The same for an exposed camera while
  `CAMERA` is off — the page gives no hint that an option exists.
  `docs/operations.md:250-251` promises only the plan line, so no docs
  claim is false; the gap is the page.
- M: probe `TestP4ItemKeepsPlanReason` (appendix): `camera.front_door:
  enabled=true state=unmappable reason=""`, `light.kitchen: enabled=true
  state=unmappable reason=""`, `enable camera.doorbell_still with the
  option off: fabricui: invalid update: "camera.doorbell_still" cannot be
  exposed: `. `TestOversizedUnmappable` (`plan_test.go:299-331`) and
  `TestCameraOff` (`:109-121`) check `Reason`, never `Full`.
- Fix. In `plan.Assess`, set `v.Full = eligibility.Verdict{State:
  eligibility.StateUnmappable, Reason: v.Reason}` in both branches
  (`:201`, `:209`), so `Full` is the one source the page reads; extend both
  plan tests to assert `Full.Reason`.
- Red test: the probe.

### HR-P4-adpt-42 — low (I) — a failed reassembly is never retried

- `internal/bridge/bridge.go:680-702`, `:569-576`; `gf:node/aggregator.go:
  590` (`if (b.changed || len(served) > 0) && started { … reassemble }`),
  `:1352-1353`, `:1403-1409` ("A store that fails a read fails the
  assembly … The topology before stays served"); unchanged at
  `gf27:node/aggregator.go:591`.
- Claim. `Partition.Batch` keeps what applied in the set "whatever fails"
  and reassembles once; if that reassembly fails (a store read error, an
  assembly the bridge layer refuses), the wire keeps the topology before
  while the set holds the change. The bridge logs `bridge.batch`, the sync
  fails and is retried — but the retry's `plan.Diff` is empty (every group
  kept by signature, `settle` found nothing refused because the set holds
  every device), `Batch` sees no change and does not reassemble, and the
  sync succeeds and clears `SyncError` and the `sync_failed` warning
  (`:572-576`). The added or replaced endpoints stay off the wire until an
  unrelated change reassembles; the web UI's reassemble (`Resync`) does not
  help, it is the same empty batch. Status and page say all is well.
- I: read only; no seam in `fabrictest` fails a store read on demand at
  v0.26.0, so no probe. How often: a store that fails a read (disk error)
  or an assembly error; rare.
- Fix. Upstream (go-fabric, theme `adpt`, not a workaround here): the
  Aggregator records that its served topology is behind its set after a
  failed reassembly, and the next `Batch` (also an empty one) or a new
  `Aggregator.Reassemble(ctx)` reassembles; go-ha2fabric then needs nothing
  or one `Reassemble` call on a sync after a failed batch. The issue goes
  with the pinned lines above, as go-fabric's rule asks.
- Guard: go-fabric's own test (a failing store read in one reassembly, an
  empty batch after it reassembles); here an e2e once a seam exists.

### HR-P4-bound-43 — note — the group signature grows quadratically

- `internal/plan/plan.go:241-251`: `sig += "|" + e.Key() + "=" +
  g.Descs[e.Key()]` per entity; each `+=` copies the whole string.
- Claim. A device's signature costs O(n²) in its entity count times the
  description size; `Assess` runs at least twice per sync (`bridge.go:615`,
  `:618` through `Candidates` → `Dry`) and again in each web UI preview and
  apply (`exposure.go:269`, `:303`). The descriptions are untrusted entity
  data (a select's options, up to 256 mapped).
- M: probe `TestP4SignatureGrowth` (appendix), 256-option selects on one
  device on this machine: n=250 0.20 s, 500 0.84 s, 1 000 3.46 s,
  2 000 13.6 s (×4 per doubling); signature 2.4 → 18.9 MB, held per group.
  Realistic devices (tens of entities) cost nothing, hence a note.
- Fix. `strings.Builder` with `Grow`, or a SHA-256 over the parts (a 32-byte
  `Sig`, also bounding the memory held per group); `Sig` is compared only
  for equality (`bridge.go:654`, `plan/diff.go:114`).
- Guard: a benchmark or the probe with a time bound is fragile; the
  Builder/hash is the fix's own proof (no red test proposed).

## Checked, no finding

- **The go-hamapping boundary (decision 9).** `Map` (`plan.go:330`,
  `bridge.go:1034`), `Build` (`:1036`), `Close` (`:715`, `:1067`, `:284`),
  `OnChange`/`OnDiscard` (`:1045-1046`), `Apply` (`:1096`), `DeviceFor`
  (`:746`, `:773`), `Endpoints[i].Options.StableKey` parallel to `Build`'s
  devices (`:1041-1043`; `hm:plan.go:420-451` builds in endpoint order),
  `*BuildError` with `Key`/`Riders` (`:1050-1070`; `hm:plan.go:431`, `:472`).
  `MapOptions`'s stable key (`plan.go:308-322`) and `RemovedKey`
  (`:353-375`) compose the same key as `hm:plan.go:380-395` (UID, "/", the
  registry id, else the entity id); `TestRemovedKey` holds it. A failed
  `Build` registers no watcher (`hm:plan.go:426-433`, watchers only at `:438-450`), so its plan needs no
  `Close` beyond what it gets. No defect inside go-hamapping found.
- **The build loop** (`bridge.go:1033-1082`) ends: each `continue` drops at
  least one entity (`:1057`).
- **Diff** (`plan/diff.go`): kinds per key never collide (a key carries its
  group's UID, `hm:plan.go:393-395`, so no key is in two groups; within a
  group a key is Add or Replace, never both); a failed build's group
  replaces with removes only; forgets of live keys dropped. `TestDiff`
  covers each section.
- **Concurrency.** Worker-owned state (`groups`, `owner`, `known`,
  `hollow`, `retry`, `labelIDs`, `oversized`, `since`) is touched only on the
  worker (`Run`'s closures, `sync`, `stateChanged`); shared state under
  `mu`; `View` hands out `Snapshot` and `All`, which no sync mutates after
  publishing (replaced wholesale at `:778-783`); `Assess` and `adapt` do
  not write the snapshot (`slices.Clone` `plan.go:163`, `maps.Clone`
  `adapt.go:245`). The drop log's timer may log once or twice after a
  bridge stopped (`drops.go:45`, `:60`); harmless.
- **Resource bounds.** Queues 1 024 work, 256 commands (`bridge.go:255-257`)
  with owed syncs; the debounce capped at 5 debounces (`:448-469`); retry
  5 s … 5 min. Endpoint count is Home Assistant's entity count, bounded by
  go-fabric's numbers 2…65 534 (`gf:endpoint/identity.go:58`); `NODE_LIMIT`
  is `config`'s (0…32, `internal/config/config.go:55-56`, `:367-368`) and
  each registry node has its own bridge over one shared snapshot reader.
- **Exposure provider.** `With`/`Without` never write the receiver's maps
  (`input.go:60-89`); `Apply` removes an override that repeats the rule
  (`exposure.go:302-322`); `check` refuses unknown keys. Only the reason
  gap of HR-P4-err-41.
- **The exposure rule** (`internal/expose`): precedence disabled > hidden >
  override > exclude (ids, globs, labels) > include > Assist; labels by
  name case-insensitively, else id, renamed labels followed in memory
  (decision 33, accepted). No new finding.
- **`adapt.go`**: unchanged in substance; control characters cleaned
  (decision 41); `toInt` of a non-integral or out-of-range float is Go's
  conversion (phase 0 data-01 covered the clamps that matter).

## The v0.27.0 question: can this bridge produce `ErrNoStableKey`?

**No** (M + I). go-hamapping composes every top-level key as
`<device UID>/<registry id or entity id>` (`hm:plan.go:380-395`) and every
part key as `<that key>#n` (`hm:plan.go:465`). The UID is never empty for a
device the plan builds: a registry device's first identifier is its id
(`adapt.go:51`), and `config/device_registry/list` refuses an entry without
one (`internal/ha/client.go:671`, `idOf("id", true)` `:774-786`); an
entity without a device gets `ha-entity:<entity id>` (`adapt.go:79-84`,
`hq:model/device.go:92-97`). The only empty input is an empty entity id
from `get_states` (not checked by `listOf`, `client.go:698`); its domain is
"" and no go-hamapping row maps it, so no endpoint is built. Probe
`TestP4NoEmptyKey` (appendix) over the fixtures plus states with ids `""`,
`"."`, `"binary_sensor."`, `"binary_sensor"`, `"light."` and a registry entry
with an empty id, all included, smoke sensors (a part each): 35 endpoints
and parts, every key non-empty, e.g. `ha-entity:binary_sensor./binary_sensor.#1`,
`ha:dev-hue-1/binary_sensor.noid#1`. Should one ever come, v0.27.0
refuses it at the call (`gf27:node/surface.go:336-345`, before any hold),
so `opErrs` and `settle`'s set read both see it.

## v0.27.0

Rows of `gh release view v0.27.0 -R thingfabric/go-fabric` that touch a call
in this unit's files (code at v0.26.0). None needs a code change; two want
a test or a comment line in the lift.

| Row (section) | Call here | Lift needs |
| --- | --- | --- |
| *Changed* `node`: an endpoint whose tree holds a part whose key is nil or renders empty is refused at `Add` and `Replace` with `ErrNoStableKey` | `bridge.go:690` `bt.Replace`, `:692` `bt.Add` | nothing (I, M by `TestP4NoEmptyKey`): no such key is produced; a refusal is a call error that `settle` handles. Optional: keep the probe as a standing guard in `internal/plan` (every built key renders non-empty) |
| *Fixed* `node`: a Batch Replace undone at commit keeps its replacement's parts reserved, so `Forget` of the key erases their numbers | `bridge.go:690` with `:688` in a later sync (a refused Replace followed by the entity's removal) | nothing (I); better behaviour for HR-P4-err-40's case |
| *Fixed* `node`: an Add and a Replace of one key in one batch judge only the Replace | `bridge.go:690`, `:692` | nothing (I): `plan.Diff` never issues both for one key |
| *Fixed* `node`: a Forget whose reservation read fails erases nothing for that key (`gf27:node/reserve.go:619-631`, joined as "the erasure of … is skipped") | `bridge.go:688`; `settle` `:857-865` | a comment, or a test once go-fabric #616 types the error (I): the call answers nil and the key is not served, so `settle` takes the Forget as applied and does not re-record it; the entity, created again, would get its old number. Within #616's scope, not a new finding |
| *Fixed* `endpoint`: an assembly that fails no longer adopts its `Spec.Instance` or writes a key's first state | `bridge.go:680` `Batch` (its reassembly) | nothing (I); does not change HR-P4-adpt-42 — still no next assembly |
| *Fixed* `cluster/core`: BridgedDeviceBasicInformation — `SetReachable`'s last ReachableChanged event matches the attribute | `bridge.go:357` `d.SetReachable` (and go-hamapping's `Apply`) | nothing (I) |
| *Fixed* `bridge`: a quieter attribute of a new instance behind a kept key starts a fresh report throttle | the Replace path `bridge.go:690` (new instance under a kept key) | nothing (I) |

Rows on the camera kit (`Provider.End`, `EndLostPeer`, the kit settings
beside a `HostServer`) touch `internal/bridge/camera.go` and
`internal/camera`, unit S-ha's. No deprecated identifier is used in this
unit's files (`PeerGone` is S-ha's file, if anywhere).

## Not read

- `internal/bridge/camera.go`, `command.go` and the bridge's `_test.go`
  files other than `bridge_r7_e2e_test.go`, `export_test.go` and the
  helpers of `bridge_e2e_test.go:33-205`: not in unit P (S-ha's files; the
  tests are read where a probe reuses them). `internal/adapt/command.go`,
  `adapt_test.go`, `fuzz_test.go`: S-ha's / not named. Everything the unit
  names was read whole: `internal/plan/*` (6 files), `internal/bridge/
  bridge.go`, `drops.go`, `internal/adapt/adapt.go`, `internal/expose/*`
  (2), `internal/exposure/*` (2).
- go-hamapping inside (decision 9): only the called functions and
  `options`/`parts`/`Build`/`Close` at v0.8.0.

## Probes

Copy: `git archive c3a9770` under
`/tmp/claude-1000/-home-markus-Dokumente-GitHub/87ad4fd5-a123-4d9e-9e1b-3f61a436a6f1/scratchpad/p4/P`;
`PATH=$HOME/go/bin:$PATH go test ./<pkg> -run TestP4 -count=1 -v`.

| Probe | Package | Result | For |
| --- | --- | --- | --- |
| `TestP4RefusedReplaceKeepsServedEndpointLive` | `internal/bridge` | FAIL (the defect, 20.2 s) | HR-P4-err-40 |
| `TestP4ItemKeepsPlanReason` | `internal/exposure` | FAIL (the defect) | HR-P4-err-41 |
| `TestP4NoEmptyKey` | `internal/plan` | PASS | v0.27.0 `ErrNoStableKey` |
| `TestP4SignatureGrowth` | `internal/plan` | PASS (timings logged) | HR-P4-bound-43 |

No `-race` or whole-tree run (brief: slow machine).

## Questions for the owner

1. **HR-P4-err-40's shape.** Recommendation: the stale-plan cut (the old
   group stays live for exactly the refused keys, closed when a later batch
   applies them). It is the only shape in which no endpoint the node serves
   is ever without a plan; the simpler "keep the old group whole" leaves
   the keys that did apply without a live plan for one retry. Both are
   small; the first is the better architecture.
2. **HR-P4-adpt-42 upstream.** Recommendation: a go-fabric issue (the
   Aggregator remembers a failed reassembly and the next batch, also an
   empty one, reassembles; or `Aggregator.Reassemble`), no workaround here.
   It belongs with #616 in the same go-fabric wave, not in this fix wave.
3. **The `ErrNoStableKey` probe as a standing guard?** Recommendation: yes,
   cheap (`internal/plan`, 30 ms): it pins the answer to the brief's
   question at every go-hamapping bump.

## Appendix: probe texts

### `internal/bridge/p4_p_probe_test.go`

```go
package bridge_test

import (
	"testing"

	"thingfabric.dev/go-fabric/device"
	"thingfabric.dev/go-fabric/node"

	"thingfabric.dev/go-ha2fabric/internal/expose"
)

// TestP4RefusedReplaceKeepsServedEndpointLive: while a Replace is refused,
// the endpoint the node still serves for the key should follow Home
// Assistant's state. Today the states go to the new, unserved device and
// the old plan is closed.
func TestP4RefusedReplaceKeepsServedEndpointLive(t *testing.T) {
	e := newEnv(t, expose.Rule{Assist: true})
	key := "ha:dev-zha-2/" + e.regID("binary_sensor.front_door")
	e.eventually("served", func() bool { return e.served(key) != nil })
	host, err := device.NewBridgedNode(device.Options{StableKey: device.StringKey("foreign"), Name: "foreign"})
	if err != nil {
		t.Fatal(err)
	}
	part, err := device.NewOnOffPlugInUnit(device.Options{StableKey: device.StringKey(key + "#1"), Name: "foreign part"})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.AddPart(part.Device); err != nil {
		t.Fatal(err)
	}
	if err := e.dev.Act(func(n *node.Node) error { return n.Aggregator().Add(e.ctx, host) }); err != nil {
		t.Fatal(err)
	}
	e.ha.SetState("binary_sensor.front_door", "off", map[string]any{"device_class": "smoke", "friendly_name": "Front door contact Door"})
	if err := e.br.Resync(e.ctx); err == nil {
		t.Fatal("Replace not refused")
	}
	old := e.served(key).(*device.Device)
	if !old.Reachable() {
		t.Fatal("precondition: old endpoint unreachable")
	}
	st := e.br.Status()
	t.Logf("status endpoints=%d sync_error=%q", st.Endpoints, st.SyncError)
	ep, ok := e.endpoint("binary_sensor.front_door")
	n, _ := old.EndpointNumber()
	t.Logf("verdict endpoint=%d ok=%v, served endpoint number=%d", ep, ok, n)
	// Same description, only the state: no re-sync, just apply.
	e.ha.SetState("binary_sensor.front_door", "unavailable", map[string]any{"device_class": "smoke", "friendly_name": "Front door contact Door"})
	e.eventually("the served endpoint follows the state", func() bool { return !old.Reachable() })
}
```

### `internal/exposure/p4_p_probe_test.go`

```go
package exposure

import (
	"context"
	"testing"
	"time"

	fabricui "thingfabric.dev/go-fabric-ui/fabricui"

	"thingfabric.dev/go-ha2fabric/internal/expose"
	"thingfabric.dev/go-ha2fabric/internal/plan"
)

// TestP4ItemKeepsPlanReason: an exposed camera with the camera option off
// and an exposed entity whose state is over the cap are unmappable with a
// reason (plan.CameraOff, plan.OversizedReason); the web UI's item should
// carry that reason.
func TestP4ItemKeepsPlanReason(t *testing.T) {
	in := plan.Input{Rule: expose.Rule{Include: []string{"camera.front_door", "light.kitchen"}}}
	m := synced(t, in, time.Now())
	// light.kitchen's state over the cap, as ha.GetStates leaves it.
	for i := range m.v.Snapshot.States {
		if st := &m.v.Snapshot.States[i]; st.EntityID == "light.kitchen" {
			st.Oversized, st.Attributes = 300000, nil
		}
	}
	m.v.Verdicts, m.v.All = plan.Dry(m.v.Snapshot, in), plan.Candidates(m.v.Snapshot, in)
	p := New(m, Options{Input: func() plan.Input { return in }})
	items, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bad := 0
	for _, it := range items {
		if it.Key != "camera.front_door" && it.Key != "light.kitchen" {
			continue
		}
		t.Logf("%s: enabled=%v state=%s reason=%q", it.Key, it.Enabled, it.Verdict.State, it.Verdict.Reason)
		if it.Verdict.Reason == "" {
			bad++
		}
	}
	_, err = p.Preview(context.Background(), []fabricui.Update{{Key: "camera.doorbell_still", Enabled: true}})
	t.Logf("enable camera.doorbell_still with the option off: %v", err)
	if bad > 0 {
		t.Fatalf("%d items lost the plan's reason", bad)
	}
}
```

### `internal/plan/p4_p_probe_test.go`

```go
package plan_test

import (
	"context"
	"testing"

	hamapping "sukramj.dev/go-hamapping"
	"thingfabric.dev/go-fabric/endpoint"

	"thingfabric.dev/go-ha2fabric/internal/expose"
	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/plan"
)

// TestP4NoEmptyKey: every endpoint and part this bridge builds has a key
// that renders non-empty, also for hostile entity ids and an empty
// registry id (go-fabric v0.27.0 refuses one at Add and Replace).
func TestP4NoEmptyKey(t *testing.T) {
	s := snapshot(t)
	attrs := map[string]any{"device_class": "smoke", "friendly_name": ""}
	for _, id := range []string{"", ".", "binary_sensor.", "binary_sensor", "light."} {
		s.States = append(s.States, ha.State{EntityID: id, State: "off", Attributes: attrs})
	}
	// A registry entry whose id is empty, on a registry device.
	s.Entities = append(s.Entities, ha.EntityEntry{EntityID: "binary_sensor.noid", DeviceID: s.Devices[0].ID})
	s.States = append(s.States, ha.State{EntityID: "binary_sensor.noid", State: "off", Attributes: attrs})
	in := plan.Input{Rule: expose.Rule{Include: []string{"*"}}}
	_, groups := plan.Assess(s, in)
	n := 0
	for _, g := range groups {
		p, _ := hamapping.Map(g.Device, g.Entities, plan.MapOptions(g.Regs, g.Names, nil))
		devs, err := p.Build(context.Background())
		if err != nil {
			continue
		}
		for _, d := range devs {
			stack := []endpoint.Spec{d.Spec()}
			for len(stack) > 0 {
				sp := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				n++
				if sp.StableKey == nil || sp.StableKey.String() == "" {
					t.Errorf("group %q: an empty key", g.Key)
				} else {
					t.Logf("%s", sp.StableKey)
				}
				stack = append(stack, sp.Parts...)
			}
		}
	}
	t.Logf("%d endpoints and parts", n)
}
```

### `internal/plan/p4_p_sig_probe_test.go`

```go
package plan_test

import (
	"fmt"
	"testing"
	"time"

	"thingfabric.dev/go-ha2fabric/internal/expose"
	"thingfabric.dev/go-ha2fabric/internal/ha"
	"thingfabric.dev/go-ha2fabric/internal/plan"
)

// TestP4SignatureGrowth: the time of Assess for one device with n select
// entities of 256 options each (the cap go-hamapping maps).
func TestP4SignatureGrowth(t *testing.T) {
	opts := make([]any, 256)
	for i := range opts {
		opts[i] = fmt.Sprintf("option number %03d of a long list", i)
	}
	for _, n := range []int{250, 500, 1000, 2000} {
		s := snapshot(t)
		dev := s.Devices[0].ID
		for i := range n {
			id := fmt.Sprintf("select.many_%04d", i)
			s.Entities = append(s.Entities, ha.EntityEntry{EntityID: id, ID: fmt.Sprintf("reg-%d", i), DeviceID: dev})
			s.States = append(s.States, ha.State{EntityID: id, State: "x", Attributes: map[string]any{"options": opts, "friendly_name": id}})
		}
		in := plan.Input{Rule: expose.Rule{Include: []string{"select.many_*"}}}
		start := time.Now()
		_, groups := plan.Assess(s, in)
		t.Logf("n=%d: Assess %v, signature %d bytes", n, time.Since(start), len(groups[0].Sig))
	}
}
```
