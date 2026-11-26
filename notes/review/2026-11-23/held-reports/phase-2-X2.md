# Phase 2, unit X2 — held row

Untracked (`held/` is in `.git/info/exclude`). Baseline `2324fbf`. Marks
and prefixes as in `notes/review/2026-11-23/phase-2-X2.md`: **M** measured,
**I** inferred. The probe ran only in the reviewer's `git archive` copy
(`…/scratchpad/X2/`); nothing was run against a live system.

One row, medium. The tracked report names it by ID and severity only.

| ID | Sev. | Subject |
| --- | --- | --- |
| HR-P2-data-30 | medium | events that ask for a re-sync faster than the debounce put the re-sync off without end: a registry change (an entity disabled, hidden, unexposed, excluded by a label) is not applied while they last |

---

## HR-P2-data-30 — medium — a stream of re-sync requests starves the re-sync

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

## Probe text

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
