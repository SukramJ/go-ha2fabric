# Phase 4, run 1 — unit D: the daemon side and the tooling

Reviewer D (Opus). Baseline `c3a9770` (go-fabric v0.26.0, go-fabric-ui
v0.14.1, go-hamapping v0.8.0); brief `notes/briefs/2026-11-26-review-p4.md`,
"Unit D". Delta first (`git diff d16f3ab c3a9770` of the unit's files:
import paths, `cmd/ha2fabric/main.go:167-175` the add-on bind,
`internal/ha/hatest/hatest.go:451-470` `RemoveDevice`,
`internal/version/version.go:33`, `Makefile` `cite`/`licenses-check`,
`.golangci.yaml:108`, `script/cite` and `script/coverfloor` new or
rewritten), then the full cut. Probes in
`scratchpad/p4/D` (a `git archive c3a9770`) and, for the guard mutations,
`scratchpad/p4/Dmut`; their text is in the appendix. M = measured, I =
inferred. References: `gf:` go-fabric v0.26.0, `gf27:` v0.27.0, `ui:`
go-fabric-ui v0.14.1, `core:` Home Assistant 2026.10.3.

No held file: nothing found is an exploitable open security item.

## Findings

### HR-P4-test-60 — low (M) — the carried row of decision 70: `daemon.call` drops the client's error, and its 30 s bound is below the node start it waits for

- `cmd/ha2fabric/nodes_test.go:75-96`: `call` sets a 30 s request
  deadline (`:77`) and, on any client error, returns `0, nil` (`:89-91`)
  — the dial error, the deadline and a reset are all "status 0". `want`
  (`:98-105`) then fails with "`0, want 201`" and no cause.
- The bound is wrong too: `POST /api/v1/nodes` starts the node
  synchronously inside the request (`nodes.go:635` `f.start` →
  `serve` → `launch` → `node.New`/`Start`, `:129-161`), and so do an
  erase's come-back (`:366`) and `TestNodeErase*`'s erases. The test file
  itself knows a node start on the Windows runner takes close to 30 s
  (`main_test.go:93-97`, TestRun's 90 s; `nodes_test.go:109`
  `eventually`'s 90 s "the slowest runner's node start"). Phase 3's
  Windows failure of `TestNodeRegistry` was exactly this (a 30 s client
  timeout on the create; memory note of #82).
- Every helper of that kind in `cmd/ha2fabric`'s tests (read in full):
  `want` (`nodes_test.go:98`), `nodes()` (`:130`), `status()` (`:320`),
  `endpointIDs` (`:341-357`, any failure → `nil`), the direct `call`s
  whose code is compared (`nodes_test.go:219`, `:392`, `:423`, `:457`,
  `:474`; `stop_test.go:95`, `:104`), the ignored ones
  (`stop_test.go:137`, `:181`), `startDaemon`'s readiness probe
  (`nodes_test.go:45-59`); `TestRun` (`main_test.go:102`, `:117`, `:130`)
  keeps its error and is fine; `supervisor_test.go`'s `eventually`
  (`:68-77`) polls state, not HTTP. `http.DefaultClient` is shared across
  a daemon's restart on the same `WEB_LISTEN` (`nodes_test.go:243-246`,
  `:272-280`): a pooled connection of the old server is a further
  "status 0" source on a runner where the reset arrives late.
- **Proposed shape** (one helper, no new dependency):
  - `const nodeStart = 90 * time.Second` beside `eventually`, used by
    `eventually` and as `call`'s request bound (a write that starts a node
    must be allowed what a node start takes);
  - `func (d *daemon) do(method, path, body string) (int, []byte, error)`
    returning the client's error, wrapped with the method, path and the
    elapsed time; `call` stays for polling and keeps `do`'s last error in
    `d.lastErr`;
  - `want` fails with `do`'s error (`"%s %s: %v after %v"`) and the log;
    `eventually` names `d.lastErr` when it gives up;
  - a client per daemon (`&http.Client{Transport: &http.Transport{DisableKeepAlives: true}}`,
    or `CloseIdleConnections` in `stop`), so a restart never meets the old
    server's connection;
  - `endpointIDs` returns `([]int, error)`, the `eventually` callers
    ignore the error, a direct caller fails on it.
- Red test: `TestProbeCallDropsError` (appendix A1) shows `call` on a
  closed port answers `code=0 body=""`; with the shape, the same call
  fails with "dial tcp …: connection refused". Standing guard: none beyond
  the helper itself (a flake class, not a product defect).

### HR-P4-test-61 — low (M) — the fake Home Assistant marshals its live maps outside its lock: a data race under `-race`

- `internal/ha/hatest/hatest.go:272-275`: `get_config` answers
  `s.config` and `homeassistant/expose_entity/list` answers
  `map[string]any{"exposed_entities": s.exposed}` — the live maps, not a
  `clone` as every other list (`:263-271`) — and `c.send` marshals them
  after `s.mu.Unlock()` (`:316-317`, `:331-338`). `SetExposed`
  (`:548-557`) and `RenameEntity` (`:488-491`) write `s.exposed` under
  the lock meanwhile. The same holds for `fire`'s `new_state`
  (`:388-399`): `next` is stored in `s.states` and marshalled by `fire`
  outside the lock, while `RenameEntity` writes `st["entity_id"]`
  (`:483-487`).
- Who meets it: every bridge test that changes the Assist exposure while
  a sync reads the list (`internal/bridge/bridge_p1_e2e_test.go:39`,
  `:50`, `:111`; `bridge_test.go:75`) under CI's `go test -race` on three
  runners (`.github/workflows/ci.yml:110`). A race report fails the test
  binary; a concurrent map write during the read can also abort it
  ("concurrent map read and map write").
- M: `TestProbeExposedRace` (appendix A2), `-race`: `WARNING: DATA RACE`
  in `reflect.maplen` from `hatest.(*conn).send` ← `(*Server).command`
  `hatest.go:317`.
- Fix: `result = clone(s.config)` and `clone(map[string]any{"exposed_entities": s.exposed})`
  under the lock; `fire` takes a `clone` of `old`/`next` made under the
  lock (`SetState` already holds it at `:377-398`). Red test: the probe.

### HR-P4-test-62 — low (M) — `internal/ha`'s floor 93 is outside the window the stale-floor check leaves: the required `coverage` job goes red on a high run (HR-P3-test-95's fix incomplete)

- `script/coverfloor/main.go:41` holds a floor at most 3.0 points below
  the measurement (`:100-107`); `floors.txt:13` sets `internal/ha 93`.
  Phase 3 measured `internal/ha` at 96.5, 95.9, 94.7 and 95.2
  (`phase-3-fix-F1.md:64`, `:194-196`). A floor that holds both ends of
  that spread must lie in [96.5 − 3, 94.7] = [93.5, 94.7]: only **94**.
  The pre-merge read's lowering 94 → 93 (`2b9da12`, HR-P3-test-95) traded
  the low-run failure for a high-run one: at 96.5 the check fails
  "3.5 points above its floor (more than 3.0)". `coverage` is a required
  check (decision 57).
- The header's "spread 0.3-0.5 points" (`floors.txt:4-5`, and
  `main.go:37-38`) is still stale for `internal/ha` (the second half of
  HR-P3-test-95's fix was not made).
- M: `TestProbeHAFloorSpread` (appendix A3) runs `check` with the
  repository's floor at the four measured values: 96.5 → `ok=false`,
  the other three pass.
- Fix: `internal/ha 94` with the measured spread named in the commit; the
  header says "0.3–0.6 points, `internal/ha` 1.8"; better still, find
  the timing-dependent branches of `internal/ha` that move 1.8 points
  between runs (a reconnect or back-off path reached or not) and make
  their test deterministic, so the package's spread is the others'.
  Standing guard: the check itself, once the floor sits in its window.

### HR-P4-test-63 — low (M) — `script/cite` silently drops a citation whose file is not in the module at `go.mod`'s version: the bump case decision 61 exists for

- `script/cite/main.go:416-440` (`attribute`): a path after a module
  mention counts only when it resolves in that module's tree at
  `go.mod`'s version (`:423`); otherwise it falls through to the exact
  tree search (`:430-439`) and, finding nothing, returns `""` — no
  citation, no problem. A go-fabric bump that renames, moves or deletes
  a cited file (or a typo in the path) makes the citation vanish from
  the check while it still names the old version.
- M (appendix A4, mutation of a copy): `cmd/ha2fabric/nodes.go:665`
  `go-fabric node/node_label.go:39 at v0.26.0` → `node_labels.go`:
  `go run ./script/cite …` exits 0, and `-list` shows one citation fewer
  in `nodes.go` (9 → 8). The other mutations bite: lines past the end
  (`:9999`) and an old pin (`v0.25.0`) exit 1 with the expected line.
- Fix: in `cites`, when the block names a module before the path, the
  path carries a line span and a `vX.Y.Z` follows within reach, and the
  path resolves neither in the module nor in this repository, report
  "not in <module> at <go.mod version>" instead of skipping (Home
  Assistant and matter-hub ranges carry no `vX.Y.Z`, so `TestCites`'
  case "core services.py:137-148" stays unattributed). Red test: a
  `TestCites` row `"go-fabric node/gone.go:12 at v0.26.0"` → a problem.

### HR-P4-err-64 — low (M) — a registry node whose stop fails is deleted anyway: its record, files and fleet entry go, the node keeps running and nothing stops it again

- `cmd/ha2fabric/nodes.go:717-742`: `Delete` logs `s.close`'s error
  (`:722-724`), sets `m.s = nil` (`:728-730`), removes the store and
  selection (`:732`) and the record (`:735`), and drops the member
  (`:738-740`); `fleet.Close` skips a member without `s` (`:831-834`),
  and the member is gone anyway. `served.close` has already set
  `closed` (`:463`), so no second close would act either. A node whose
  `Stop` failed (`gf:node/node.go:1594-1619`: "the node is not stopped:
  the advertiser, the lifetime context and the store stay") stays bound
  to its port, its store file open, without a fabric (they were removed
  by `leave`, `:720`) — a running, unlisted node until the process ends.
  The doc comment (`:705-706`) promises the opposite for a file that
  cannot be removed ("leaves the record, listed as not enabled, for a
  second Delete").
- M: `TestProbeDeleteKeepsUnstoppedNode` (appendix A5): with the
  `stopNode` seam failing, `DELETE /api/v1/nodes/alexa` answers 204, the
  list has one node, and alexa's UDP port is still bound afterwards
  ("bind: address already in use").
- Fix: when `s.close` fails, keep the record and the member (listed not
  enabled, `m.s` kept so `fleet.Close` stops it), and return the error so
  the UI answers 500 and a second Delete retries the stop; only after a
  stop that succeeded remove the files and the record. Red test: the
  probe with "204 and port bound" turned into "500, listed, and a second
  Delete with the real stop answers 204 and frees the port".

### HR-P4-conc-65 — note (I) — Create's rollback stops the started node without a deadline, holding the fleet's lock

- `cmd/ha2fabric/nodes.go:640-643`: when `reg.Add` fails, the new node
  is closed with `context.WithoutCancel(ctx)` — the request's values,
  no deadline. `served.close` → `stopRun` waits for the bridge's worker
  until that context ends (`:186-208`), and `stopNode` waits for
  go-fabric's lifecycle slot until it ends (`gf:node/node.go:1577-1589`).
  A worker held by a camera's sessions (the HR-P2-conc-60 case) holds
  `f.op` (`:623-624`) for good: every later Create, Rename, Erase and
  Delete blocks, and at the stop `fleet.Close` gives up on the lock
  (`:821-823`) and stops no registry node. Its log text there ("a
  create, rename or delete still runs") also omits Erase, the operation
  most likely to hold `f.op` (up to `eraseTimeout`, `:87`).
- Fix: `context.WithTimeout(context.WithoutCancel(ctx), eraseTimeout)`
  (or the stop budget); the log text names "a create, rename, erase or
  delete". Test: a rollback with a worker that never ends answers within
  the bound (the `TestCloseBoundedByBridge` shape).

### HR-P4-err-66 — note (I) — an erase of a run replaces a kept stale node without stopping it

- `cmd/ha2fabric/nodes.go:313-323`, `:353-355`: with a run, the erase
  resets `s.n` and then writes `s.stale = stale` — `nil` after a reset
  that worked — dropping a node an earlier erase kept as stale
  (`:340-351`) without a stop. That earlier node exists only when its
  reset and its stop failed *and* the come-back still started (a `Stop`
  that failed after its transport closed, `gf:node/node.go:1607-1619`,
  so the port was free); the kept node then still holds its store.
- Fix: in `erase`, stop a pending `s.stale` (bounded by `ectx`) before
  the reset of the run, or keep both until `close`. Test: seams that
  fail reset and stop once while the come-back starts, then a second
  erase; the first node's `Stop` must have been called.

### HR-P4-test-67 — note (M) — nothing checks that the version banner finds its modules

- `internal/version/version.go:29-49`: `GoFabric`/`GoHamapping` look the
  module path up in the build info and answer `"unknown"` when it is
  absent. The delta renamed the go-hamapping path (`:33`); a missed rename
  prints "unknown" silently. `version_test.go:26-28` only checks for an
  empty string — and in a test binary both answer "unknown" anyway (M:
  `TestProbeDeps`, appendix A6: `GoFabric="unknown" GoHamapping="unknown"`).
  CI's build job runs `bin/ha2fabric --version` (`ci.yml:148`) without
  looking at the output; the binary built here prints
  `go-fabric v0.26.0, go-hamapping v0.8.0` (M).
- Fix: the build job's step fails when `--version` prints `unknown`
  (`bin/ha2fabric --version | tee /dev/stderr | grep -qv unknown` or an
  explicit `grep -q 'go-hamapping v'`); drop or reword the unit test's
  check that cannot fail.

### HR-P4-test-68 — note (M) — the fake differs from core where no test looks yet

- `internal/ha/hatest/camera.go:269-295` (`cameraProxy`): core checks the
  entity before the authentication and answers 401 for an unknown entity
  of an unauthenticated request (`core:homeassistant/components/camera/__init__.py:784-787`);
  the fake answers 403 without a header. It also serves every still as
  `image/jpeg`, where core sends the image's own type (`:838`). The
  client always sends the header and does not branch on the type, so no
  test is wrong today.
- `internal/ha/hatest/hatest.go:276-280`: `subscribe_events` without
  `event_type` is `MATCH_ALL` in core
  (`core:homeassistant/components/websocket_api/commands.py:185`); the
  fake files it under `""`, which `fire` never matches.
- `hatest.go:290-293`, `:613-629` (`WaitCall`): `callCh` holds 64 calls and
  drops the rest; a test with more than 64 calls before a `WaitCall`
  can miss a call made between the `Calls()` scan and the drain.
- Fix (when a test needs it): order the proxy's checks as core's;
  `"*"`/empty as every type in `fire`; `WaitCall` polls `Calls()` from an
  index instead of the lossy channel.

### HR-P4-conc-69 — note (I) — `Snapshots.Read` reuses a read that began in the same clock tick as the trigger

- `internal/ha/snapshot.go:134-137`: a flight is reused when
  `!c.began.Before(since)`, so one that began at the same `time.Time`
  as the trigger counts as having seen it. On a monotonic clock with a
  coarse tick a read sent just before the event was received can carry
  the same timestamp; that read did not see the event, and the bridge
  keeps the older state until the next trigger. Linux's clock makes it
  practically unreachable; it is the CI's Windows runner where it could
  show.
- Fix: `c.began.After(since)` (an equal timestamp costs one read more,
  never a stale one). Test: a `Snapshots` with an injected clock that
  returns the same instant for trigger and flight.

### HR-P4-sup-70 — note (M) — `gosec` is off for all of `cmd/`

- `.golangci.yaml:88-90` excludes `gosec` under `cmd/` (since the
  template, `b2b49d2`). `cmd/ha2fabric` is where the daemon builds its
  Supervisor HTTP client and request (`supervisor.go`), reads
  `SUPERVISOR_TOKEN` into headers and writes the passcode path to the
  log — the code the credential rule of `CLAUDE.md` covers. `script/`
  runs `gosec` and carries `//nolint:gosec` with reasons; `cmd/` has none.
  M (appendix A10): with the exclusion dropped, `golangci-lint run
  --enable-only gosec ./cmd/...` reports one issue, a false positive
  (`warnings.go:25` G101 on the warning code `passcode_not_kept`).
- Fix: drop the exclusion; one reasoned `//nolint:gosec // a warning
  code, not a credential` on `warnings.go:25`.

## Guards of phase 3 — do they bite, what slips, where they run

| Guard | Mutation (copy `Dmut`) | Result | Runs in |
| --- | --- | --- | --- |
| `TestPure` (`internal/arch/ast_test.go:147-164`) | `_ "sync"` in `internal/plan/plan.go` | red: "imports sync: internal/plan is pure" | `go test ./...` (`make test`, CI `test` ×3) |
| `TestPure` | `func init() { go func() {}() }` in `plan.go` | red: "starts a goroutine" | same |
| `TestEnvironment` (`:169-178`) | `os.Getenv("X")` in `internal/web/web.go` | red: "calls os.Getenv" | same |
| `adaptGoFabric` (`arch_test.go:71`, `:144-146`) | `_ ".../cluster/spec/onoff"` in `internal/adapt/adapt.go` | red: "below go-fabric's facade" | same |
| `TestFuzzTargets` (`ast_test.go:241-257`) | `FuzzSlug` dropped from `Makefile:82` | red: "is not in the Makefile's FUZZ_TARGETS" | same |
| `make cite` (`script/cite`) | `node_label.go:39` → `:9999` | red, exit 1 | `make ci`, CI `lint` (`ci.yml:60`), `TestRepository` |
| `make cite` | `v0.26.0` → `v0.25.0` | red, exit 1 | same |
| `make cite` | `node_label.go` → `node_labels.go` | **green, exit 0** (HR-P4-test-63) | same |
| `script/coverfloor` stale/missing floor | `check` at the measured `internal/ha` 96.5 with floor 93 | red (HR-P4-test-62: the repository's own floor) | CI `coverage` (`ci.yml:122`), not `make ci` |

What slips past, by reading (already accepted rows not repeated:
HR-P3-api-99's dot import and `x/sys/unix`): `TestEnvironment` sees only
`os.X`/`syscall.X` selectors on the import's name — `exec.Cmd.Environ()`
(the process environment when `Env` is nil) passes; `TestPure` checks the
pure layers' own imports and `go` statements, not what a callee does
(by design: go-hamapping and go-fabric's `device` are allowed);
`script/cite` checks that lines exist, not what they say (decision 61's
scope) — a re-pin after a bump that shifted the cited construct passes
unless a human reads it (v0.27.0's `docs/guides/camera.md` is such a
case, below). `cover-check` is not in `make ci` (`Makefile:144`) but is
its own CI job and a required check (decision 57) — as the plan says.

## Life cycle (reconnect, `Fanout`, resync, registry, shutdown): read and held

- `main.go:148-234`: one stop budget (`stopBudget`, 25 s) for web and
  connection first, nodes in parallel after; the second signal kills
  (`:50-56`); a stop during the start exits 0 (`:152-156`); a web error
  exits 1 after the stop (`:193-197`). Held.
- `Fanout` (`internal/ha/fanout.go:45-98`): `Add` while connected hands
  `Connected` under the lock, so a node created at run time syncs at once
  (`TestNodeRegistry`'s selection read, `nodes_test.go:218-221`); `Remove`
  without `Disconnected`, as `stopRun` needs. The bridge's handlers only
  enqueue (`internal/bridge/bridge.go:329-346`, `:373-400`). Held.
- `Subscription` (`internal/ha/subscription.go`): closed once with its
  reason; `Unsubscribe` on a dropped connection sends nothing. Held.
- Registry create/rename/erase/delete under `f.op`; status reads under
  `f.mu`/`s.mu` only; erase detached with `eraseGrace` (decision 29).
  Held but for HR-P4-err-64, -conc-65, -err-66.
- Flakes (M, Linux): `TestNodeRegistry`, `TestStatusDuringErase`,
  `TestStopWithinBudget` `-race -count=20` pass (266 s); `internal/ha`'s
  snapshot, fanout, subscription and reconnect tests `-race -count=20`
  pass. The Windows failure is not reproducible here; its cause by
  reading is HR-P4-test-60's 30 s bound. `TestStatusDuringErase`'s 1 s
  bound on one status read (`stop_test.go:105`) is the next candidate
  on a slow runner (I).

## v0.27.0

`gh release view v0.27.0 -R thingfabric/go-fabric`, rows that touch a call
in this unit's files:

| Row | Call here | Lift needs |
| --- | --- | --- |
| *Changed* `node`: `Config.TLS` is a configuration error at `Validate`/`New` below 1.5.1 | `cmd/ha2fabric/nodes.go:130` `newNode(base, s.ncfg)`; the configuration (`internal/config/node.go`) sets no `TLS` (grep) | nothing (I) |
| *Changed* `node`: `OpenOwnWindow` refuses a negative timeout; *Changed* `bridge`: window refusal order | not called in this unit (go-fabric-ui's pairing page calls it) | nothing |
| *Changed* `node`: `ErrNoStableKey` at `Add`/`Replace` | not in this unit (`internal/bridge`, unit P) | — |
| *Deprecated* `camera.Provider.PeerGone` | not called anywhere here (the release names go-ha2fabric "no use") | nothing |
| *Fixed* `im/subscription` panic recovered | none | nothing |

Citations in this unit that `make cite` turns red at the lift (each names
`v0.26.0`; M, by `cmp` of the cited files between the two module
directories): `cmd/ha2fabric/nodes.go:38` (`node/node.go:42-46`, file
changed, the lines unchanged), `:126`, `:281` (`node/factory_reset.go`,
unchanged), `:550-551` (`cluster/core/basic_information.go`, unchanged),
`:665` (`node/node_label.go`, unchanged), `main.go:144`
(`docs/guides/embed-in-a-daemon.md`, unchanged), `internal/arch/arch_test.go:42`
(`docs/guides/observability.md`, changed, no lines), `:49`
(`cluster/spec/value.go:62`, unchanged), `:55-57`
(`docs/guides/camera.md:93-110`, `:133-140`: **content moved** — v0.27.0
inserts three lines into the `End` bullet at 111, so the
`ErrPayloadTooLarge` sentence is `:137-144` and the `End` bullet ends at
`:114`), `script/coverfloor/main.go:35` (go-fabric's
`script/coverfloor/main.go:73-81`, unchanged). Re-pin to `v0.27.0`, and
`:133-140` → `:137-144` (a comment change, M after the bump).

## Counts

0 high · 0 medium · 5 low (test-60, test-61, test-62, test-63, err-64) ·
6 note (conc-65, err-66, test-67, test-68, conc-69, sup-70) = 11.

## Not read

Nothing of the unit's file list was skipped. Read in full: `cmd/ha2fabric/`
`main.go`, `nodes.go`, `warnings.go` and its ten `_test.go`
(`admin`, `exits`, `image`, `main`, `nodes`, `stop`, `supervisor`,
`warnings`, `web`); `internal/ha/subscription.go`, `fanout.go`,
`snapshot.go`; `internal/ha/hatest/hatest.go`, `camera.go`;
`internal/version/*`; `internal/arch/*`; `script/cite/*`;
`script/coverfloor/*`; `Makefile`; `.golangci.yaml`. `supervisor.go` was
read only where `main.go` calls it (S-web's file). The tests of
`internal/ha` itself (`fanout_test.go`, `snapshot_test.go`, …) are not in
the unit's list and were only run, not reviewed.

## Probes run

| Probe | Where | Result |
| --- | --- | --- |
| A1 `TestProbeCallDropsError` | `cmd/ha2fabric` | pass — shows `code=0 body=""` |
| A2 `TestProbeExposedRace` `-race` | `internal/ha/hatest` | **fail: DATA RACE** (test-61) |
| A3 `TestProbeHAFloorSpread` | `script/coverfloor` | pass — 96.5 → `ok=false` (test-62) |
| A4 guard mutations (9) | `Dmut` | 8 red as intended, 1 green (test-63) |
| A5 `TestProbeDeleteKeepsUnstoppedNode` | `cmd/ha2fabric` | pass — 204, port still bound (err-64) |
| A6 `TestProbeDeps` | `internal/version` | pass — both "unknown" in a test binary (test-67) |
| A7 `--version` of the built binary | — | `go-fabric v0.26.0, go-hamapping v0.8.0` |
| A8 flakes `-race -count=20` | `internal/ha` (snapshot, fanout, subscription, reconnect tests) | pass (14 s) |
| A9 flakes `-race -count=20` | `cmd/ha2fabric` `TestNodeRegistry`, `TestStatusDuringErase`, `TestStopWithinBudget` | pass, 20/20 each (266 s) |
| A10 `gosec` on `cmd/` | `Dmut`, exclusion dropped | 1 issue, a false positive (`warnings.go:25` G101) |

## Questions for the owner

1. **HR-P4-test-62, the `internal/ha` floor.** Recommendation: **94**
   now (the only whole percent inside the window the 3-point check leaves
   for the measured 94.7–96.5), and make the package's timing-dependent
   test branches deterministic in a later wave; alternative: a per-package
   slack in `floors.txt` (a second column) — rejected by F1 for phase 3,
   and it widens the ratchet for the one package that drifts.
2. **HR-P4-err-64, a Delete whose stop fails.** Recommendation: **refuse
   the delete** (500, record and member kept, listed not enabled, a second
   Delete retries); alternative: delete the record and keep the node in an
   orphan list that `fleet.Close` stops — it frees the name at once but
   leaves a running node the UI cannot show.
3. **HR-P4-sup-70, `gosec` on `cmd/`.** Recommendation: **enable** it in
   the fix wave (measured: one false positive to annotate).

## Appendix: probe texts

### A1 — `cmd/ha2fabric/probe_p4d_test.go` (part)

```go
func TestProbeCallDropsError(t *testing.T) {
	d := &daemon{t: t, web: freePort(t, "tcp"), logs: &safeBuffer{}}
	code, b := d.call(http.MethodGet, "/api/v1/node/status", "")
	t.Logf("call on a closed port: code=%d body=%q (the dial error is gone)", code, b)
	if code != 0 || b != nil {
		t.Fatalf("code %d body %q", code, b)
	}
}
```

Output: `call on a closed port: code=0 body="" (the dial error is gone)`.

### A2 — `internal/ha/hatest/probe_p4d_test.go`

```go
package hatest_test

type up struct{ once sync.Once; ch chan struct{} }

func (u *up) Connected(string)   { u.once.Do(func() { close(u.ch) }) }
func (u *up) Event(ha.Event)     {}
func (u *up) Disconnected(error) {}

func TestProbeExposedRace(t *testing.T) {
	srv := hatest.New(t)
	c := ha.New(ha.Options{URL: srv.URL(), Token: hatest.Token})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &up{ch: make(chan struct{})}
	go func() { _ = c.Run(ctx, h) }()
	select {
	case <-h.ch:
	case <-time.After(10 * time.Second):
		t.Fatal("no connection")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			srv.SetExposed("light.kitchen", i%2 == 0)
		}
	}()
	for i := 0; i < 200; i++ {
		if _, err := c.ListExposed(ctx); err != nil {
			t.Fatal(err)
		}
	}
	<-done
}
```

`CGO_ENABLED=1 go test -race -run TestProbeExposedRace ./internal/ha/hatest/`:
`WARNING: DATA RACE` — read in `reflect.maplen` … `encoding/json.Marshal`
← `hatest.(*conn).send` `hatest.go:332` ← `(*Server).command`
`hatest.go:317`; the write is `SetExposed`.

### A3 — `script/coverfloor/probe_p4d_test.go`

```go
func TestProbeHAFloorSpread(t *testing.T) {
	floors, err := readFloors("floors.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, pct := range []float64{96.5, 95.9, 95.2, 94.7} {
		blocks := map[string]block{
			module + "internal/ha/a.go:1.1,2.2": {stmts: int(pct * 10), covered: true},
			module + "internal/ha/a.go:3.1,4.2": {stmts: 1000 - int(pct*10), covered: false},
		}
		ok, out := run(t, blocks, map[string]float64{"internal/ha": floors["internal/ha"]})
		t.Logf("internal/ha at %.1f, floor %.0f: ok=%v %s", pct, floors["internal/ha"], ok, out)
	}
}
```

Output: `internal/ha at 96.5, floor 93: ok=false … ✗ internal/ha is 3.5
points above its floor (more than 3.0): raise it to 96`; 95.9, 95.2,
94.7: `ok=true`.

### A4 — guard mutations (`Dmut`, each reverted after its run)

```sh
# arch: go test -count=1 ./internal/arch/
M1 internal/plan/plan.go: + _ "sync"                          -> FAIL TestPure "imports sync"
M2 internal/plan/plan.go: + func init() { go func() {}() }    -> FAIL TestPure "starts a goroutine"
M3 internal/web/web.go:   + "os"; var _ = os.Getenv("X")      -> FAIL TestEnvironment "calls os.Getenv"
M4 internal/adapt/adapt.go: + _ ".../cluster/spec/onoff"      -> FAIL TestImports "below go-fabric's facade"
M5 Makefile: - ./internal/nodes/:FuzzSlug                     -> FAIL TestFuzzTargets
# cite: go run ./script/cite cmd internal script testdata
M7 nodes.go:665 node_label.go:39 -> :9999      -> exit 1 "lines 9999-9999 are not in node/node_label.go (41 lines)"
M8 nodes.go:665 node_label.go -> node_labels.go -> exit 0; -list: 9 -> 8 citations in nodes.go
M9 nodes.go:665 v0.26.0 -> v0.25.0             -> exit 1 "names v0.25.0, go.mod has v0.26.0"
```

Red test for HR-P4-test-63 (a `TestCites` row):
`{"go-fabric node/gone.go:12 at v0.26.0", []string{"gf node/gone.go:12 PROBLEM: not in go-fabric at v0.26.0"}}`.

### A5 — `cmd/ha2fabric/probe_p4d_test.go` (part)

```go
func TestProbeDeleteKeepsUnstoppedNode(t *testing.T) {
	restoreSeams(t)
	env, web := eraseEnv(t)
	d := startDaemon(t, env, web)
	d.want(http.MethodPost, "/api/v1/nodes", `{"label":"Alexa"}`, http.StatusCreated)
	dir := filepath.Dir(func() string { v, _ := env("STORE_PATH"); return v }())
	alexa := readRegistry(t, dir)[0].Listen
	stopNode = func(*node.Node, context.Context) error { return errors.New("probe: stop fails") }
	d.want(http.MethodDelete, "/api/v1/nodes/alexa", "", http.StatusNoContent)
	stopNode = (*node.Node).Stop
	if l := d.nodes(); len(l.Nodes) != 1 {
		t.Fatalf("nodes after the delete %+v", l)
	}
	if c, err := net.ListenPacket("udp", alexa); err != nil {
		t.Logf("alexa's port %s after its delete: still bound (%v)", alexa, err)
	} else {
		_ = c.Close()
		t.Logf("alexa's port %s after its delete: free", alexa)
	}
}
```

Output: `alexa's port 127.0.0.1:33428 after its delete: still bound (listen
udp 127.0.0.1:33428: bind: address already in use)`. As a red test: want
the delete answered 500 and alexa listed; a second delete with the real
stop answers 204 and the port is free.

### A6 — `internal/version/probe_p4d_test.go`

```go
func TestProbeDeps(t *testing.T) {
	t.Logf("GoFabric=%q GoHamapping=%q stale=%q", GoFabric(), GoHamapping(), dep("github.com/SukramJ/go-hamapping"))
}
```

Output: `GoFabric="unknown" GoHamapping="unknown" stale="unknown"`.

### A9 — flakes

```sh
CGO_ENABLED=1 go test -race -count=20 -timeout 580s \
  -run 'TestNodeRegistry$|TestStatusDuringErase|TestStopWithinBudget' ./cmd/ha2fabric/
# ok  thingfabric.dev/go-ha2fabric/cmd/ha2fabric  266.473s
CGO_ENABLED=1 go test -race -count=20 \
  -run 'TestReadSnapshot|TestSnapshots|TestFanout|Subscri|Reconnect' ./internal/ha/
# ok  thingfabric.dev/go-ha2fabric/internal/ha  14.357s
```

### A10 — `gosec` on `cmd/`

```sh
# .golangci.yaml with the `gosec` / `path: cmd/` exclusion removed
golangci-lint run --enable-only gosec ./cmd/...
# cmd/ha2fabric/warnings.go:25:2: G101: Potential hardcoded credentials (gosec)
# 1 issues
```
