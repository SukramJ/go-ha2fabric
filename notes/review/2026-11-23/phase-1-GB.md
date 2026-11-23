# Phase 1 — unit G-B, the daemon side

Reviewer G-B (Opus). Baseline `5eefc0a`. Files: `cmd/ha2fabric/main.go`
(except the web and admin wiring), `cmd/ha2fabric/nodes.go`,
`internal/ha/subscription.go`, `fanout.go`, `snapshot.go`, the
go-hamapping boundary (decision 9), and the goroutines the inventory lists.
Read for context only: `internal/bridge/bridge.go` (the `ha.Handler`
methods, the queue, `build`), `internal/ha/client.go` (`Run`, `serve`,
`read`, `callWith`), `internal/camera/camera.go` (its use of a
`Subscription`), `internal/web/web.go:105-133`, `internal/plan/plan.go`
(`Assess`, `MapOptions`, `Dry`, `RemovedKey`).

M = measured (read at the pin, or a probe ran); I = inferred.
Probes ran in a `git archive 5eefc0a` copy under the session's scratchpad
(`GB/src`), never in the worktree.

No finding is an open exploitable security weakness: no held file.

## Findings

### HR-P1-conc-60 — medium — a full worker queue loses the re-sync it promises, and the Disconnected marking

- `internal/bridge/bridge.go:214-221` (`enqueue`), `:313-325`
  (`resyncSoon`), `:236-258` (`Disconnected`), `:228-232` (`Connected`).
  The file is G-A's. It is reported here because brief item 3 (a slow
  consumer, resync after missed events) is G-B's. The verifier
  de-duplicates.
- Claim: when the 1024-slot queue is full, `enqueue` drops the work, logs
  `bridge.queue_full` ("a re-sync follows") and calls `resyncSoon`. The
  debounce timer's send is non-blocking too (`:320-323`, `default:`). If
  the queue is still full when the timer fires (the worker is inside a
  long sync while Home Assistant keeps sending), the re-sync is dropped
  and no timer is left. The dropped `state_changed` values then stay
  wrong on the controllers until some unrelated registry event or a
  reconnect. The same drop hits `Disconnected`'s closure. It has already
  swapped `connected` to false (`:237`), so a second `Disconnected` does
  nothing, and the endpoints stay reachable with stale values while Home
  Assistant is away. That is what cookbook §6 "Upstream down", cited at
  `:234-235`, is meant to prevent. A dropped `Connected` closure means a
  reconnect never re-reads.
- Reference: the bridge's own contract at `:212-213` and the log hint at
  `:218`.
- M, by probe `TestProbeGBResyncLostOnFullQueue`
  (`internal/bridge/probe_gb_internal_test.go` in the copy): 1100 events
  with no worker draining, then `Disconnected`, then the timer fires, then
  the queue drains. Output:
  `queued after flood: 1024 (cap 1024)` /
  `after drain: queued 0, re-sync timer pending false, connected flag false` /
  `PROBE: no re-sync queued or pending after the overflow; the Disconnected work was dropped too`.
  How likely the trigger is in a real house is I: a sync longer than the
  debounce (1 s) while a few hundred events per second arrive.
- Fix: keep a worker-owned "re-sync owed" flag (`atomic.Bool`). `enqueue`'s
  overflow and the timer set it. The worker runs `sync` after the
  function in hand whenever the flag is set (`Swap(false)`). Keep
  `Disconnected`'s unreachable marking out of the droppable queue: a flag
  the worker reads, or a dedicated 1-slot channel.
- Red test: the probe as a regression. Flood past the capacity, call
  `Disconnected`, let the timer fire, start `Run` on a fake upstream, and
  assert one sync ran and every device was set unreachable.

### HR-P1-err-61 — low — an erase whose `FactoryReset` did not stop the node orphans it

- `cmd/ha2fabric/nodes.go:240-242`: `rerr = s.n.FactoryReset(ectx)`, then
  `s.n, s.br, s.cancel, s.done = nil, …` whatever `rerr` is.
- Claim: go-fabric's `FactoryReset` can return without stopping the node.
  This happens when the lifecycle slot is not won before ctx ends
  (`gf:node/node.go:1501-1512`), or when the stop does not complete
  (`gf:node/factory_reset.go:83-91`; doc `:51-55`: "the node is not
  stopped and keeps its store, and a later FactoryReset finishes the
  reset"). ha2fabric drops its only reference to that node. The node
  keeps serving its port and store, still paired in the first case, with
  no bridge behind it. The come-back (`s.start`) then fails on the taken
  port, so `node_lost` is set. The documented remedy "Erase again"
  (`docs/operations.md:545`) only calls `start` again (`s.n == nil`), and
  it fails the same way. `close` (`:343-345`) no longer sees the node, so
  the daemon never stops it, and only a restart recovers.
- Reference: `gf:node/factory_reset.go:51-55`, `:83-91`;
  `gf:node/node.go:1480-1493` (`Stop` finishes an unfinished stop).
- M for the code path, I for reachability. `stopRun` has already ended
  the reconciler, so no host bridge `Start` or `Stop` holds the slot. The
  trigger is narrow today: an `ectx` that ends inside the reset.
- Fix: when `rerr != nil`, keep `s.n` (and `lost`). Before `start`, try
  `s.n.Stop(ectx)` and start only once the old node is stopped. A later
  erase then calls `FactoryReset` again on the kept node, as go-fabric
  documents.
- Red test: a seam `factoryReset = (*node.Node).FactoryReset` that the
  daemon test swaps for one returning an error without stopping. Assert
  that a second erase succeeds and the old port is free, and that `stop`
  ends the old node (no listener left).

### HR-P1-ha-62 — low — a `Subscribe` whose context ends before Home Assistant's result leaves Home Assistant's subscription open

- `internal/ha/subscription.go:122-125` with `internal/ha/client.go:452-456`.
- Claim: when ctx ends after the command was written, `callWith` returns
  `ctx.Err()`. `Subscribe` only drops its local stream and sends no
  `unsubscribe_events`. Home Assistant registers the subscription's
  teardown before it answers (`core:homeassistant/components/camera/webrtc.py:275-280`,
  the result at `:280`) and runs it only on unsubscribe or when the
  connection closes (`core:homeassistant/components/websocket_api/connection.py:258-268`).
  So every timed-out `camera/webrtc/offer` leaves a provider WebRTC
  session open in Home Assistant for the rest of the connection's life.
  The camera host calls `Subscribe` with the controller's request context
  (`internal/camera/camera.go:340`). Its error path logs
  `camera.offer_failed` and keeps no handle to unsubscribe (`:341-343`).
- M, by probe `TestProbeGBSubscribeTimeoutLeaves`
  (`internal/ha/probe_gb_test.go` in the copy; a local WebSocket server
  answers the offer after 300 ms, the caller's deadline is 100 ms). Output:
  `Subscribe: ha: camera/webrtc/offer: context deadline exceeded; offer id 1; unsubscribe_events sent for []`.
- Fix: when the write succeeded and ctx ended, send
  `unsubscribe_events` for the id, best-effort and detached with
  `CallTimeout`. The alternative is to keep the stream registered and
  unsubscribe when the late result arrives.
- Red test: the probe as a regression. Assert `unsubscribe_events` for
  the offer's id within the call timeout.

### HR-P1-conc-63 — note — an erase holds `served.mu` across the reset and the come-back; every status read waits

- `cmd/ha2fabric/nodes.go:229-254`: `s.mu.Lock()` covers `stopRun`,
  `FactoryReset` (up to `eraseTimeout`, 30 s) and `start`
  (`node.New` + `Start`).
- Claim: `View`, `Resync`, `node`, `Discards` and `Warnings` take
  `s.mu.RLock` (`:151-209`). The host-wide `warnings` and `counters`
  (`main.go:189-221`) call the configured node's and every registry
  node's, so every node's status page waits for any node's erase.
  `fleet.Warnings` also holds `f.mu.RLock` (`:615-629`) while it waits.
  The comment at `:56-58` promises only that `List` does not wait, and
  it holds. No deadlock was found: no path takes `s.mu` under the
  fan-out's lock, and the bridge's handlers do not block.
- M, by probe `TestProbeGBConcurrentWrites` (`cmd/ha2fabric/probe_gb_test.go`
  in the copy, under `-race`): three erases of each node, reassembles,
  renames and status reads at once. Output:
  `slowest status read during the writes: 4.706373207s`, `stop took 1.021433219s`,
  no `DATA RACE`.
- Fix: take the run out under `mu` (`s.n`, `s.br` into locals; `live`
  false), run `FactoryReset` and `start`'s slow part unlocked (`op`
  already serialises erase and close), and install the new run under
  `mu`.
- Red test: `newNode` swapped for one that sleeps 2 s before
  `node.New`. Assert that a status read during the erase answers in under
  200 ms.

### HR-P1-err-64 — low — shutdown can outlast the add-on's stop timeout; the web server's shutdown error is dropped

- `cmd/ha2fabric/main.go:173-181`; `internal/web/web.go:125-127`.
- Claim: after `cancel()`, `wg.Wait` waits for the web server's
  `Shutdown` (up to 5 s) and for the client's `ws.Close` (a close write
  and a close handshake of up to 5 s each, `coder/websocket@v1.8.15 close.go:185`,
  `:199`). Only then does the 10 s node budget start (`:176`). That
  budget is spent serially: every registry node (`fl.Close`), then the
  configured node last with what is left. `fl.Close` and `def.close`
  first take `op` with no deadline (`nodes.go:335`, `:666`). A registry
  Erase (`eraseTimeout` 30 s) or a Delete (go-fabric-ui's detached
  2 min, `ui:fabricui/handlers_node.go:713`, `ui:fabricui/registry.go:342-344`)
  still running after the web server gave up holds them. The Supervisor
  stops an add-on with the manifest's `timeout`, default 10 s
  (`sup:supervisor/apps/validate.py:558-560`,
  `sup:supervisor/docker/app.py:204-206`). `addon/config.yaml` sets none,
  so a SIGKILL can land inside a node's `Stop` or `FactoryReset`. Also,
  on the signal path `ws.Run`'s return (`main.go:161`) is never read, so
  a shutdown that hit its deadline is not logged.
- M for the code and the Supervisor lines, I for how long each step takes
  in practice. The probe's stop of two idle nodes took 1.0 s.
- Fix: one shutdown deadline for the whole stop, below the Supervisor's
  (or set `timeout:` in `addon/config.yaml`, which allows 10–300). Close
  the nodes in parallel, or the configured node first. Take `op` with the
  deadline (a `TryLock` loop, or end in-flight writes through the base
  context). Drain and log `webErr` after `wg.Wait`.
- Red test: a daemon test with two registry nodes and an erase whose
  `newNode` blocks. Cancel, then assert `run` returns within the budget.

### HR-P1-cfg-66 — note — `NODE_LIMIT` bounds creation, not the nodes that start

- `cmd/ha2fabric/nodes.go:387-396` (every record starts), `:477` (only
  `Create` checks); `internal/nodes/nodes.go:145-162` (`check` does not
  count records).
- Claim: a limit lowered below the nodes already created, or a
  `nodes.json` with more records, starts every record. The docs say the
  limit "bounds the nodes beside the configured one"
  (`docs/operations.md:516-518`; `internal/config/config.go:144-146`).
- M.
- Fix: the docs say "how many the web UI may create", plus a start-up
  warning when `reg.Len() > NodeLimit`. Or refuse the excess records,
  which is a behaviour change for the owner.
- Red test: `NODE_LIMIT=0` with one record in `nodes.json`. Assert the
  warning, or the refusal.

### HR-P1-data-67 — note — the snapshot reads the registries before the states

- `internal/ha/snapshot.go:282-296`; `internal/plan/plan.go:110-123`,
  `:159-161`, `:173-182`.
- Claim: an entity registered and given its first state between
  `config/entity_registry/list` and `get_states` is in the states but not
  the registry. `Assess` plans it as unregistered: its own device
  (`adapt.OwnDevice`), stable key = entity id (`MapOptions`, `:261-266`).
  If the rule exposes it, it is built under that key, and the next re-sync
  (triggered by its registry event) rebuilds it under the registry key.
  The controllers see a transient endpoint.
- I. It rests on Home Assistant creating the registry entry before the
  first state write. That is not cited and needs the Fable read if it is
  kept.
- Fix: read `get_states` first. An entity registered later then has no
  state yet and stays out ("no state yet", `plan.go:159-161`) until its
  `state_changed` re-syncs it.
- Red test: a `Source` fake whose registry list contains an entity that
  its `GetStates` (read first) does not. Assert that no group carries it.

### HR-P1-adpt-68 — note — the primary endpoint follows entity-id order; riders move on a rename

- `internal/plan/plan.go:118-123` (ids sorted), `:191-192` (entities
  appended in that order); `hm:plan.go:145-150` ("the primary endpoint is
  the first mapped entity whose row does not ride"); riders:
  `hm:rows/sensor.yaml:556` (battery, `ride: primary`).
- Claim: the battery's PowerSource rides on whichever mapped entity of
  the device sorts first by entity id. Two things move it to another
  endpoint: an entity-id rename, which `StableKey` is meant to make
  transparent (`plan.go:249-253`), or exposing another of the device's
  entities that sorts earlier. Whether the diff then rebuilds the old
  primary is G-A's (I).
- M for the order, I for the controller-visible effect.
- Fix: order each group's entities by stable key (the registry id) in
  `Assess`, which does not change on a rename. Or (go-hamapping
  candidate) a primary rule in `Map` that is independent of input order,
  e.g. an `Options.Primary` hook or a row priority.
- Red test: a device with `binary_sensor.door` and a battery. Rename to
  `binary_sensor.zz_door` with a second entity `binary_sensor.a_tamper`,
  and assert that PowerSource stays on the same stable key.

### HR-P1-adpt-69 — note — `RemovedKey` re-derives go-hamapping's unexported stable-key format

- `internal/plan/plan.go:304-323` builds `UID + "/" + registry id`, the
  format of `hm:plan.go:360-368` (`Plan.options`, unexported).
- Claim: the boundary has a duplicated private format.
  `internal/plan/plan_test.go:189-212` guards it against go-hamapping's
  output, so drift goes red. The duplication is the gap, not a defect.
- M.
- Fix: go-hamapping candidate: export `hamapping.StableKey(dev, entity,
  opts)` (or `Plan.StableKeyFor`) and use it here.
- Red test: the existing `TestRemovedKey` stays the guard.

### HR-P1-test-70 — low — the negative cases behind 60, 61 and 62 are not tested

- `cmd/ha2fabric/nodes_test.go:426-485` (`TestNodeEraseFailed`) fails only
  `newNode`, never `FactoryReset`.
- `internal/ha/camera_test.go:54-192` covers subscribe, refusal,
  reconnect and overflow, but not a ctx that ends before the result.
- The bridge tests do not overflow the work queue.
- `internal/ha/fanout_test.go:32-62` is sequential, with no `Add`/`Remove`
  racing `Event`. It is clean under `-race` in the daemon tests, but has
  no direct test.
- M.
- Fix and red tests: the regressions named in 60, 61 and 62.

### HR-P1-conc-71 — note — the connection's reader goroutine is not joined

- `internal/ha/client.go:229-231` (`go c.read(rctx, s, h)`; `serve`
  returns on `defer stop()` without waiting for it).
- Claim: after `serve` returns, the reader can still hand one decoded
  frame's events to `h.Event` (through the fan-out to the bridges). That
  can happen after `Run` called `Disconnected`, or after `run`'s
  `wg.Wait`. It is harmless today: `Bridge.Event` only enqueues without
  blocking, and the bridges whose `Run` ended drop it. But the
  `Handler` order "Connected, Event, Disconnected" (`fanout.go:20-22`)
  is not guaranteed across that edge.
- I (no probe; the window is one frame and the back-off is at least
  `MinBackoff`).
- Fix: a `readDone` channel. `serve` waits for it after `stop()` and
  `CloseNow`.
- Red test: a fake whose `Event` records after `Disconnected`. Drop the
  connection while the server sends a burst, and assert that no `Event`
  follows `Disconnected` until the next `Connected`.

### HR-P1-err-72 — note — a signal during start-up exits 1; a second signal is swallowed

- `cmd/ha2fabric/main.go:47-49`, `:132-139`.
- Claim: SIGTERM between `run`'s start and the main `select` cancels
  `rctx`. `serve` then builds the node on a cancelled context, which is
  likely to fail (I), and `run` logs `ha2fabric.node` and returns 1 for
  an ordinary stop. `stop()` runs only after `run` returns, so a second
  SIGINT during a long shutdown (see 64) does not end the process.
- M for the flow, I for `Start` on a cancelled context.
- Fix: return 0 when `ctx.Err() != nil` after a failed start. Call
  `stop()` as soon as `ctx` is done (a goroutine on `ctx.Done()`), so the
  next signal takes the default action.
- Red test: `run` with an already-cancelled context and a valid
  configuration returns 0.

## The five items of the unit

### 1. `main.go`: start order, shutdown, exit codes, signals, a failed component

- **Start**:
  1. `config.Load`: exit 2, or 0 for `-h` and `--version`.
  2. The logger, then the client (built, not run), then `selection.Open`
     (exit 1).
  3. `--dump-plan` branches off here.
  4. The start-up warnings, then the Supervisor gate (built), then
     `nodes.Open` (exit 1), then the configured passcode (exit 1, or a
     status warning when it cannot be kept).
  5. `serve` of the configured node (exit 1), then `newFleet` (a failing
     registry node is logged and listed as not enabled).
  6. Then the backend: `client.Run`, `gate.run`, and the web server
     (`web.New` failing is exit 1 through `webErr`).

  Nodes before the backend, as the comment cites (`main.go:128-131`).
- **Shutdown**: on a signal (exit 0) or a web error (exit 1): `cancel()`
  (bridges' workers, client, gate and web end), `wg.Wait`, then the
  registry nodes, then the configured node, inside one 10 s budget
  (`:176-181`). Bounds and ordering: HR-P1-err-64.
- **Signals**: SIGINT and SIGTERM through `signal.NotifyContext`; a second
  one is swallowed (HR-P1-err-72). SIGHUP is not caught, so the default
  terminate applies: no clean stop, outside the add-on's normal flow.
- **A failed component**:
  - Home Assistant unreachable: `client.Run` retries forever with
    back-off and never returns an error (`client.go:167-189`), so
    ignoring its return at `main.go:143` is correct. The bridges mark
    every endpoint unreachable, unless the queue is full (HR-P1-conc-60).
  - The web listener: the whole daemon stops with exit 1
    (`exits_test.go:27`).
  - A registry node: listed as not enabled.
  - The configured node: exit 1 at start. After a failed erase the daemon
    runs on with `node_erase_failed` (HR-P1-err-61 for one edge).
  - `--dump-plan`: 30 s bound, joined goroutine, exit 1 on no connection
    or a read error.

### 2. `nodes.go`: the registry fleet

- `fleet.op` serialises Create, Rename, Erase, Delete and Close. `f.mu`
  guards `members` for `List`, `Warnings` and `Discards`. Every write of
  `m.s` and `m.rec` is under both. Reads of them hold one or the other,
  consistently.
- `served.op` serialises erase and close. `served.mu` guards the run.
- Lock order is always `f.op → s.op → s.mu → fan.mu → bridge.mu`. No
  reverse path was found: the fan-out's handlers and the bridge's
  `OnChange` never take `s.mu`.
- **Create**: the `NODE_LIMIT` check, then `Allocate`, then `start` (on
  the daemon's context, not the request's), then `Add`. Each failure
  rolls back the node and the files (`exits_test.go:53`). On `Add`
  failing, `s.close` uses an uncancelled context with no deadline: a
  note, bounded by go-fabric's `Stop`.
- **Delete**: go-fabric-ui runs it detached for 2 min
  (`ui:fabricui/registry.go:340-344`). A failed fabric leave is logged
  and the files go anyway: "final", as `docs/operations.md:504-507` says.
- **Rename**: the request's context bounds `SetNodeLabel`, and a refusal
  is logged and leaves the registry's name, as documented.
- **Erase**: see 61 and 63.
- **Restart**: every record starts with the daemon.
- **Per-node goroutines**: see item 5.
- **NODE_LIMIT**: HR-P1-cfg-66.
- **Concurrent web writes against a running resync**: `Resync` holds
  `s.mu.RLock` while it waits for the worker (go-fabric-ui detaches it
  for 2 min, `ui:fabricui/handlers_node.go:356-372`). An erase waits for
  it, with no deadlock. Probe `TestProbeGBConcurrentWrites`: erases,
  reassembles, renames and reads together. Clean under `-race`, clean
  stop, status reads stalled up to 4.7 s (HR-P1-conc-63).
- The existing `make race` set for `./cmd/ha2fabric/` and
  `./internal/ha/` is clean in the copy.

### 3. `subscription`, `fanout`, `snapshot`

- **Life cycle across a reconnect**: an event subscription
  (`subscribe_events`) is renewed per connection in `serve`
  (`client.go:233-238`). The bridges re-read everything on `Connected`,
  so missed events need no path of their own. That holds while the
  queue is not full (HR-P1-conc-60).
- A command subscription (`Subscription`) ends with the connection
  (`fail`, `client.go:385-388`, `ErrDisconnected`) and is not renewed, as
  documented (`subscription.go:30-31`). The camera relay ends the
  session (`camera.go:494-498`).
- A late result: HR-P1-ha-62.
- **A slow or dead consumer**:
  - A `Subscription` is bounded at 64 events. On overflow it is dropped
    locally and logged (`client.go:342-345`). The relay then ends the
    session, and `Unsubscribe` still sends `unsubscribe_events` because
    the session is alive (`subscription.go:136-148`), so Home Assistant's
    teardown runs. Correct.
  - The `Handler` path never blocks the reader: `Fanout` calls under its
    lock, and every bridge method only enqueues (`bridge.go:214-221`).
  - The bridge's queue is bounded (1024) and drops, with the gap in 60.
  - The command queue (256) drops with a status count
    (`command.go:25-31`).
  - No unbounded queue was found in these files.
- **Snapshot consistency**: seven sequential reads, not one atomic view
  (`snapshot.go:279-306`). Events that arrive during the read are queued
  behind the sync and applied after it, in order, so states converge.
  Registry changes during the read re-sync. One transient edge:
  HR-P1-data-67.
- **Fanout**: `Add` hands `Connected` at once while up. `Remove` sends no
  `Disconnected`, which is correct for `stopRun`, which ends the worker
  itself. The connected flag is set and cleared only by the client's
  calls, so a bridge added between `subscribe` and `Connected` gets
  exactly one `Connected` (`client.go:233-245`, `fanout.go:17-22`, `:30-42`, `:58-66`).
  Tested sequentially (`fanout_test.go:32-62`).

### 4. The go-hamapping boundary (decision 9)

Calls (production code):

- `hamapping.Map` (`bridge.go:599`, `plan.go:281` in `Dry`).
- `Plan.Build`, `Plan.Endpoints[i].Options.StableKey`, `Plan.Apply`,
  `Plan.OnChange`, `Plan.OnDiscard`, `Plan.Close` (`bridge.go:600-628`,
  `:199`).
- `Options{Ports, StableKey, Name}` (`plan.go:259-273`).
- `Verdict{Key, Row, eligibility.Verdict}` (`plan.go:225-238`).
- `Discard` (`command.go:39`).
- The `Method*` constants (`adapt/command.go:55-222`; G-A reads the
  mapping to services).
- `version.GoHamapping` (`version.go:33`).

What the types carry: go-hamqtt `model.Device`/`model.Entity` in, and
go-fabric `device.Device`, `device.Options`, `device.SourceKey` and
`eligibility.Verdict` out.

Assumptions, each checked at `hm:` v0.6.0:

- `Build` returns devices in `Endpoints` order (`hm:plan.go:399-414`).
  `bridge.go:614-616` relies on it for `g.keys`. Holds.
- One verdict per entity with a non-nil `Desc`, keyed by `e.Key()`
  (`hm:plan.go:166-172`). `adapt.Entity` always sets `Desc`
  (`adapt.go:119-125`), and `MergeVerdicts` indexes by entity id, which
  is unique in a snapshot. Holds.
- Stable keys are unique per device: `UID + "/" + registry id or entity
  id` (`hm:plan.go:360-368`). Holds.
- The 32-byte NodeLabel cut is go-hamapping's (`hm:plan.go:375-389`).
  G-A reads lengths.
- `OnChange` and `OnDiscard` run inline in the controller's request and
  must not block (`hm:plan.go:557-560`). Both bridge callbacks are
  non-blocking (`command.go:25-42`). Holds.
- `Build` is single-use (`hm:plan.go:396-398`). The bridge maps afresh
  for every build. Holds.
- The primary endpoint depends on input order: HR-P1-adpt-68.
- The stable-key format is duplicated in the host: HR-P1-adpt-69.

No defect inside go-hamapping was found. Two issue candidates are below.

### 5. Goroutines and contexts: each one's end

| Where | What | Ends |
| --- | --- | --- |
| `main.go:143` (`wg.Go`) | `client.Run` | `rctx` cancelled; joined by `wg.Wait` |
| `main.go:145` (`wg.Go`) | the Supervisor gate | `rctx`; joined |
| `main.go:161` (`wg.Go`) | `ws.Run` | `Shutdown` (5 s); joined. Its handlers can outlive it (64) |
| `main.go:299` | `--dump-plan`'s `client.Run` | `cancel()` then `<-done` (`:303-306`) |
| `nodes.go:124` | each run's `br.Run` | `stopRun` (`cancel`, `<-done`) on erase, close, Delete, the Create rollback, and the daemon's stop. A run orphaned by 61 is the one leak (its node, not this goroutine) |
| `bridge.go:188` (`wg.Go`) | `commandLoop` | the run's ctx; joined by `Run`'s `defer wg.Wait()` |
| `client.go:231` | the session reader | `rctx` or the socket's close. **Not joined** (71) |
| `camera.go:355` | the relay | the subscription's channel closing; `h.wg` (S's) |
| `web.go:123` | `srv.Serve` | `Shutdown` |
| `bridge.go:319` | the debounce timer | stopped at `Run`'s end. A later `resyncSoon` (a straggling event) arms one more that fires once into a buffered channel: no leak |

- **Contexts**: every `WithTimeout` and `WithCancel` in the unit's files
  has its `cancel` deferred or called. `erase` detaches from the request
  and bounds the reset with `eraseTimeout`. The come-back runs on the
  daemon's base, which `contextcheck` notes. `Subscribe` and
  `Unsubscribe` bound a call without a deadline by `CallTimeout`.
- **Leak probes**: none beyond `-race` on the copy's
  `./cmd/ha2fabric/` and `./internal/ha/` (clean) and the three probes
  above. No `goleak` is in the module.

## Counts

| Severity | Count | IDs |
| --- | --- | --- |
| high | 0 | — |
| medium | 1 | conc-60 |
| low | 4 | err-61, ha-62, err-64, test-70 |
| note | 7 | conc-63, cfg-66, data-67, adpt-68, adpt-69, conc-71, err-72 |

No phase 0 row is re-reported. None of them touches these files' defects
(`phase-0-register.md`).

## go-hamapping issue candidates (decision 9)

1. **An exported stable-key helper** (HR-P1-adpt-69):
   `hamapping.StableKey(dev *model.Device, e model.Entity, opts Options)`,
   so a host that forgets a removed entity's endpoint does not re-derive
   `UID + "/" + id` (`hm:plan.go:360-368`).
2. **A primary endpoint independent of input order** (HR-P1-adpt-68):
   `Map` takes the first mapped non-riding entity (`hm:plan.go:145-150`).
   Either document that the host must give a stable order, or offer an
   `Options.Primary` hook or a row priority.

## Not read, and why

- `cmd/ha2fabric/supervisor.go`, `warnings.go` and the web and admin
  wiring of `main.go`: unit S's. They were read only where `main.go` calls
  them.
- `internal/bridge` beyond the handler methods, the queue and `build`,
  and `internal/plan`'s diff: unit G-A's.
- `internal/camera` beyond its use of `Subscription`: unit S's.
- go-fabric's node beyond `FactoryReset`, `Stop` and `enterLifecycle`.
- go-hamapping beyond `plan.go` and the battery row: decision 9.
- The Home Assistant fact under HR-P1-data-67 (registry entry before the
  first state) was not read in core. It is marked I.

## Probes

All are in `<scratchpad>/GB/src` (`git archive 5eefc0a`), none in the
worktree.

- `internal/bridge/probe_gb_internal_test.go`
  `TestProbeGBResyncLostOnFullQueue`: FAIL as expected (HR-P1-conc-60).
- `internal/ha/probe_gb_test.go` `TestProbeGBSubscribeTimeoutLeaves`:
  FAIL as expected (HR-P1-ha-62).
- `cmd/ha2fabric/probe_gb_test.go` `TestProbeGBConcurrentWrites` under
  `-race`: PASS, no race, slowest status read 4.7 s (HR-P1-conc-63).
- `CGO_ENABLED=1 go test -race -count=1 ./cmd/ha2fabric/ ./internal/ha/`
  (probes skipped): `ok` / `ok`.

## Questions for the owner (recommendation first)

1. **HR-P1-conc-60's fix shape**: a worker-owned "re-sync owed" flag that
   overflow and the timer set and the worker honours after every function
   (**recommended**: it never blocks the reader and never loses the
   re-sync), or a blocking timer send. The file is G-A's; the fix belongs
   in one change with G-A's bridge findings.
2. **HR-P1-err-64, the stop budget**: set `timeout: 30` in
   `addon/config.yaml` and keep one deadline for the whole stop, with the
   configured node stopped in parallel with the registry nodes
   (**recommended**), or squeeze everything under the default 10 s.
3. **go-hamapping**: file candidate 1 (the exported stable-key helper) as
   an issue now, and put candidate 2 to go-hamapping as a question first
   (**recommended**: candidate 2 may be better solved here, by ordering
   each group by stable key).
