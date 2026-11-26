# Phase 4 run 1 fix wave — F1 (Go code, tests, the lift)

Brief `notes/briefs/2026-11-26-review-p4-fix.md`, section F1. Branch
`review/p4`, base `ccc4104`; commits `fe9cc38..d7e083f` (code) and this
report. Opus, no sub-agents. `PATH=$HOME/go/bin:$PATH`. Scratch: the
session's scratchpad (`repin.py`, `guards/`).

## Per row

### 1. Decision 69, the lift (`fe9cc38`)

- go-fabric v0.26.0 → **v0.27.0**; `go mod tidy` demanded nothing else:
  go-fabric-ui v0.14.1 and go-hamapping v0.8.0 stay. `licenses/`
  regenerated (`MODULES.txt` only). `golangci-lint` 0 issues (no SA1019).
- **Every citation in my paths re-pinned and re-read.** Files compared
  byte for byte between the two module directories; for an unchanged file
  the pin only, for a changed one the range read at v0.27.0:

  | Citation | At v0.27.0 |
  | --- | --- |
  | `camera.go:16`, `arch_test.go:55` `docs/guides/camera.md:93-110` | `:93-114` (the `End` bullet grew by four lines) |
  | `camera.go:22`, `config/node.go:44`, `arch_test.go:57` `camera.md:133-140` | `:137-144` (the `ErrPayloadTooLarge` paragraph, same text) |
  | `config.go:143` `camera.md:577-599` | `:581-603` ("The ecosystem gate" … "harness as the independent counterpart.") |
  | `camera.go:52` `cluster/camera/provider.go:680-695` | `:712-727` (`maxEndReason`, `endSession`) |
  | `camera.go:103` `provider.go:648-665` | `:680-697` (`provideIceCandidates`) |
  | `camera.go:202` `provider.go:597-601` | `:629-633` (the port's `ProvideOffer`, `upsertSession`) |
  | `camera.go:440` `provider.go:159-165` | `:172-178` (the `serial` handlers) |
  | `camera.go:187` `provider_peer.go:104-148` | `:102-173` (`Offer`'s comment to `End`'s end) |
  | `config/node.go:28` `node/config.go:956-965` | `:960-969` (`checkPBKDF`) |
  | `config.go:494` `node/config.go:1086-1092` | `:1091-1097` (the `MDNS.Interfaces` checks) |
  | `node/node.go:42-46`, `observability.md` (no lines), `webrtc_provider_peers.go:30-51` | files changed, the cited lines identical: pin only |
  | the other 22 (`embed-in-a-daemon.md`, `factory_reset.go`, `basic_information.go`, `node_label.go`, `camera_gen.go`, `fabrictest/device.go`, `value.go`, `values.go`, `definition_gen.go` ×2, `sessionparams.go`, `avstream.go`, `status.go`, `chime.go`, `state.go`, `tcp_listen.go`, coverfloor's `main.go`, the parity test) | files unchanged: pin only |

  Left as they are (history, not a pin): `bridge.go:672` "Since go-fabric
  v0.26.0", `bridge_r7_e2e_test.go:19`, `:78`; `bridge.go:811` reworded
  to "go-fabric (since v0.26.0) holds". `script/cite/main_test.go`'s
  v0.26.0 rows are the test's fake modules.
- **P's failed-reservation Forget note:** a comment at the `bt.Forget`
  call (`bridge.go`) citing `node/reserve.go:615-631 at v0.27.0`: the
  call answers nil, `settle` takes it as applied, the entity created
  again gets its old number; typing the error is go-fabric #616's scope.
- **Difference from the row:** none. `make cite` is red only on F2's
  files (7 citations at v0.26.0 in `docs/`, `README.md`) until F2's
  re-pin lands; `go run ./script/cite cmd internal script testdata` is
  green.

### 2. Decision 72, HR-P4-bound-20 with HR-P4-cite-23 (`ac4e41b`)

- `session` carries the controller's `fabric`; `MaxSessionsPerFabric =
  MaxSessions / 2` (4). A new session of a fabric at its share **or** at
  `MaxSessions` evicts that fabric's own session idle longest (≥
  `EvictIdle`), else RESOURCE_EXHAUSTED (`ErrTooManySessions`, the text
  now names the fabric's count). Another fabric's session is never
  evicted; a re-offer does not count.
- cite-23: `MaxSessions`' comment now says the kit has
  `ProviderConfig.MaxSessions`/`MaxSessionsPerFabric`
  (`cluster/camera/provider.go:65-86 at v0.27.0`) which go-hamapping's
  camera row does not set (`ports.go:59-65 at v0.8.0`), so the host's
  bounds are in force; `MaxSessionsPerFabric` says it goes when
  go-hamapping passes the kit's setting through.
- **Red first:** `TestEvictionStaysInTheFabric` (the probe inverted: a
  third fabric at the bound refused, no `end`), `TestEvictionTakesTheFabricsOwnIdlest`
  (fabric 2's session older, fabric 1's own idlest evicted, `end 1 12`
  only), `TestFabricShareRefused` (4 of fabric 1 refused below 8; the
  re-offer passes; fabric 2 served) — all three failed before.
- **Difference:** the existing tests that fill the camera
  (`TestSessionsPerCameraAreBounded`, `TestIdleSessionEvictedAtTheBound`,
  `TestIgnoredEventsDoNotKeepTheSession`, `close_test.go`'s `fullHost`)
  now fill it with two fabrics (`fill(i)`), as a single fabric can no
  longer hold 8.

### 3. Decision 73, HR-P4-err-40 (`55859bd`, `d7e083f`)

- `group.stale` maps each entity of a refused Replace to the group whose
  plan serves the endpoint the node still holds under the key; `settle`
  calls `keepStale` for every refused Replace key, finding the served
  device (the node's set) in the replaced group or in the groups it kept
  stale itself (a second refusal). `group.live(id)` is that group, used by
  `apply` (states, `restore` after a failed command), the verdict's
  endpoint and `serving` (the command gate); `unreachable` also marks the
  served devices; a camera host is re-attached to the served device's
  provider. `closePlans` closes a plan only when no group of the new set
  uses it, own or stale (also at `Run`'s end). One `WARN
  bridge.replace_refused` per refused key.
- **Red first:** `TestRefusedReplaceKeepsServedEndpointLive` (P's probe
  extended): the read model's endpoint is the served one, `Serving` is
  true, `unavailable` and back reach the served endpoint, so does a Home
  Assistant outage (`d7e083f`); after the part
  key is free the retry replaces, the new device follows, and the old one
  no longer gets states. Failed before at the endpoint check (0 vs 3).
- **Difference:** the controller-write clause is held by construction,
  not by an end-to-end write: the only rows with parts (the condition of
  a refused Replace this composition can be driven to) are the smoke and
  CO binary sensors, which take no controller command. What the test
  holds is the gate (`Serving`, so `command` does not drop it) and the
  plan staying open (its `OnChange` watchers are only removed by
  `Close`, `hm:plan.go:600-608`).

### 4. Decision 74, HR-P4-err-64 with HR-P4-conc-65 and HR-P4-err-66 (`79268ce`, `d7e083f`)

- err-64: `served.close` keeps a node whose `Stop` failed
  (`unstopped`): a second `close` stops it again, a third does nothing;
  the node is listed not enabled from the first close. `fleet.Delete`
  returns the stop's error (go-fabric-ui answers 500), keeps record,
  files and member; a second Delete retries; `fleet.Close` still stops it.
- conc-65: Create's rollback stops with `rollbackContext` (the request's
  values, `stopBudget`); the stop's error is joined into Create's
  (`d7e083f` dropped a separate keep-the-files branch: it left a running
  node outside the fleet either way and cost the floor). `fleet.Close`'s text names "create, rename, erase or delete".
- err-66: an erase of a run with a node an earlier erase kept stale stops
  that node first (10 s within the erase's bound); if it does not stop,
  the erase refuses before touching the run.
- **Red first:** `TestDeleteWhoseStopFailsIsRefused` (D's probe inverted:
  500, alexa listed not enabled, record and store kept, second delete 204,
  port free), `TestCloseRetriesAFailedStop`, `TestEraseStopsAnEarlierStaleNode`
  (panicked before: the erase went on), `TestCreateRollbackBounded`.

### 5. HR-P4-test-60 (`ec824ac`, lint `f9b484b`)

D's shape: `nodeStart = 90 s` for `eventually` and every request;
`do(method, path, body) (code, resp, err)` with "`METHOD path: err after
Xms`", safe from goroutines (no `t`); `call` is `do` for polling; `want`
fails with `do`'s error; `eventually` names the last client error; a
client per daemon (`DisableKeepAlives`); `endpointIDs` returns
`([]int, error)`. Direct calls compared (`/node/fabrics`, the status read
during an erase, the erase in a goroutine) use `do`; the ignored ones
`_, _, _ = d.do`. Red: `TestDaemonDoKeepsTheError` (a compile red first —
`do` did not exist; with the old `call` the probe showed `0, nil`).
`TestRun`'s own client (`main_test.go`) keeps its error and is unchanged.

### 6. HR-P4-test-61, -test-68, -conc-69 (`86f0ae6`)

- test-61: `get_config` and the exposure list answer `clone`s; `fire`
  clones its data under the lock (covers `SetState`'s new state and every
  other event). Red: `TestAnswersAreCopies` (`-race`: DATA RACE before).
- test-68, as core where cheap: `subscribe_events` without a type is
  MATCH_ALL (`"*"`, `commands.py:185` as D cites it) and `fire` serves it;
  `WaitCall` reads `Calls()` from an index, the channel only wakes it;
  the still's `Content-Type` is `http.DetectContentType`. A comment names
  the one left: core answers 401 for an unknown entity without a header,
  the fake 403.
- conc-69: `c.began.After(since)`. Red: `TestSnapshotsSameTickIsNotReused`
  (internal test, a flight at the trigger's instant is not reused, one a
  nanosecond after is).

### 7. HR-P4-test-62, -test-63, -test-67 (`2b72e1a`, `4756103`, `78c4393`, `cb7b540`)

- test-62: `internal/ha 94`; the header in `floors.txt` and
  `main.go`'s comment: spread 0.3–0.6, `internal/ha` 1.8 (94.7–96.5).
  Red: `TestHAFloorHoldsItsSpread` (96.5 failed against 93).
- test-63: `attribute` reports a path as **missing** when the block names
  a module before it, the path has a directory, a `vX.Y.Z` follows within
  reach with no other module named between, and no module (nor this
  repository) has it; counted only with lines; `check` says "not in
  go-fabric at v0.27.0". Red: D's `TestCites` row plus two "no lines / no
  version" rows and one "another module's version" row (a false positive
  in `docs/by_design.md:154` found while running it, fixed before
  commit). D's mutation (`node_labels.go`) now exits 1.
- test-67: **a `debug.ReadBuildInfo` path test**, not a built binary:
  `cmd/ha2fabric/version_test.go` `TestVersionBannerFindsItsModules` —
  the cmd test binary links both modules, so `version.GoFabric()` and
  `GoHamapping()` must equal `go.mod`'s requires. Mutation (the old
  `github.com/SukramJ/go-hamapping` path) fails it. `version_test.go`'s
  check that could not fail is replaced by a comment pointing there.

### 8. HR-P4-err-41, HR-P4-bound-43, the `ErrNoStableKey` guard (`4ec34a3`, `35102ff`, `a42034b`)

- err-41: `Verdict.unmappable(reason)` sets `State`, `Reason` and `Full`
  in both pre-Map branches. Red: `TestCameraOff` and
  `TestOversizedUnmappable` assert `Full`; `TestItemKeepsPlanReason`
  (P's probe, in `internal/exposure`) asserts both items' reasons and the
  refusal's text ending with `plan.CameraOff`.
- bound-43: `strings.Builder`, grown once to the exact size — the same
  string, so `Sig`'s meaning is kept (`TestSignature` holds the formula).
  `BenchmarkAssessManyEntities` (P's probe, n = 1 000): 0.09 s/op here,
  P measured 3.46 s before.
- `TestNoEmptyKey` (P's probe as a standing test, in `internal/plan`);
  it walks parts through `Spec().Parts` so it stays on go-fabric's
  facade (`internal/arch`).

### 9. HR-P4-bound-21, HR-P4-data-22, HR-P4-cfg-05 (`825a55a`, `6c64417`, `507d628`)

- bound-21: `MaxCandidateBytes` counts the line plus `sdpMid`: Home
  Assistant's over it is refused and logged (session stays); a
  controller's over it is refused with CONSTRAINT_ERROR
  (`ErrCandidateTooLong`, new) before Home Assistant is asked. Red:
  `TestCandidateMidIsBounded`.
- data-22: U+2028, U+2029 unsafe; `Clean` makes them a space. Red:
  `TestSafe`/`TestClean` rows.
- cfg-05: `allowedHost` refuses `unicode.IsSpace` ("has white space");
  `FuzzLoad` calls `fabricui.NewHandler` with the accepted hosts and a
  seed with `\v` and U+00A0. Red: four `TestWebAllowedHosts` rows (`\r`,
  U+2003 passed before) and the fuzz seed.

### 10. HR-P4-sup-70 (`923785a`)

The `gosec` / `path: cmd/` exclusion dropped; one
`//nolint:gosec // G101: a warning code naming the passcode file's state,
not a credential` on `warnings.go`'s `warningPasscodeNotKept`.
`golangci-lint run ./...` 0 issues.

## Guards

Full set at `f9b484b` (summary in the scratchpad `guards/`), the
packages `d7e083f` touched (`cmd/ha2fabric`, `internal/bridge`, tests and
one Create branch) again at `d7e083f`:

| Guard | HEAD | Result |
| --- | --- | --- |
| `make ci` | `f9b484b` | build, vet, test, lint green; **cite red on F2's files only** (7 citations at v0.26.0: `docs/operations.md:119`, `:389`, `:411`, `docs/by_design.md:125`, `:142`, `:168`, `README.md:36`) |
| `go run ./script/cite cmd internal script testdata` | `f9b484b` | exit 0 |
| `make licenses-check` | `f9b484b` | red on F2's file only: `THIRD-PARTY-NOTICES.md` lacks "go-fabric pinned v0.27.0" (`licenses/` regenerated and green) |
| `make race` | `f9b484b` | exit 0 (206 s) |
| `make cover-check` | `f9b484b` | **red**: `cmd/ha2fabric` 87.9 < 88, `internal/bridge` 92.8 < 93 → fixed by `d7e083f` |
| `make cover-check` | `d7e083f` | exit 0 (`cmd/ha2fabric` 88.6, `internal/bridge` 93.6, `internal/ha` 94.7 against 94) |
| `make build vet test lint` | `d7e083f` | exit 0, 0 lint issues |
| `go test -race ./cmd/ha2fabric ./internal/bridge` | `d7e083f` | ok |
| `make fuzz-smoke` | `f9b484b` | exit 0 |
| `make vuln licenses secrets` | `f9b484b` | exit 0 each |
| `GOARCH=386 go test ./...` | `f9b484b` | exit 0 |

Not rerun at `d7e083f`: fuzz-smoke, vuln, licenses, secrets, 386 (no
fuzz target, dependency or 32-bit-sensitive code moved).

## Left out

- The add-on image's `S6_CMD_RECEIVE_SIGNALS` guard test
  (`cmd/ha2fabric/image_test.go` is my path, but the Dockerfile is F2's;
  a test against an unchanged Dockerfile would be red on this branch) —
  F2 writes the text, the coordinator places it.
- HR-P4-adpt-42 (decision 75: a go-fabric issue, outside the wave).
- test-68's proxy order (core's 401 before the entity check): a comment,
  not a change — the client always sends the header.

## Questions

1. **Decision 72's share and a third fabric.** With 4 per fabric, two
   fabrics can still fill all 8, and a third is then refused
   (RESOURCE_EXHAUSTED) — the lock-out the share narrows but does not
   remove for three ecosystems. Accepted as decided, or a share of
   `MaxSessions / 3`? (I kept the decision's 4 of 8.)
2. **err-40 and a camera.** A refused Replace of a camera re-attaches its
   host to the served device's provider; a camera's Replace cannot be
   refused by this composition (no parts), so this is untested.

## Changelog lines for F2 (`[Unreleased]`)

```markdown
### Changed

- go-fabric v0.27.0 (from v0.26.0).
- Camera: each controller ecosystem (fabric) holds at most 4 of a
  camera's 8 live-view sessions, and a new view takes the place of an
  idle one of its own ecosystem only — never another ecosystem's;
  otherwise it is refused (RESOURCE_EXHAUSTED).
- Camera: an ICE candidate is bounded with its media id (1024 bytes
  together); a controller's longer one is refused with CONSTRAINT_ERROR.
- Names from Home Assistant: the line and paragraph separators U+2028
  and U+2029 are cleaned like a line break.
- `WEB_ALLOWED_HOSTS`: an entry with white space (a vertical tab, a
  carriage return, a no-break space, …) is refused at start, as the web
  UI would refuse it.

### Fixed

- A device whose replacement go-fabric refuses keeps its served endpoint
  live — states, reachability, controller commands and the endpoint the
  web UI shows — until a later sync replaces it.
- Deleting an additional node whose stop fails answers an error and keeps
  the node, listed not enabled, with its files; deleting it again
  retries.
- The web UI's selection shows why an exposed camera is not served while
  the camera option is off, and why an entity whose state is over
  256 KiB is not mapped.
- A failed node create stops the started node within the stop budget; an
  erase stops a node an earlier failed erase kept before it resets the
  current one.
- A device with very many entities no longer makes each sync slow
  (the signature was quadratic in its entities).
```

## Docs lines for F2

- `docs/operations.md`, camera section: "A camera holds at most 8 WebRTC
  sessions, at most 4 per controller ecosystem (fabric). At either bound
  a new live view of an ecosystem ends that ecosystem's own session idle
  longest (no signalling for a minute); another ecosystem's view is never
  ended. Otherwise the offer is refused (RESOURCE_EXHAUSTED)." (decision
  72; replaces any sentence on the eviction at 8.)
- `docs/operations.md`, node registry / Delete: "A delete whose node does
  not stop answers an error; the node stays listed, not enabled, with its
  store and selection, and a second delete retries the stop." (decision
  74)
- `docs/operations.md`, `WEB_ALLOWED_HOSTS`: add "white space inside an
  entry" to what start refuses.
- `docs/by_design.md` for decision 72: only if matter-hub bounds or
  evicts sessions differently — that is a reference read (Fable's role,
  on Opus) I did not make; the coordinator's call.

## Tokens

About 450 k.

## Fix round (pre-merge IDs 90–99, decisions 76–78)

Base `73546e6` (F2 integrated; F1 owns every path). Commits
`730821f..c018dc6`.

| ID | Commit | What |
| --- | --- | --- |
| test-90 | `730821f` | `TestEvictionTakesTheFabricsOwnIdlest` on a plain `&upstream{}`: no event moves `last`, the offer order is the idle order. `-race -count=50` of `TestEviction*`/`TestFabricShare`: ok |
| test-91 | `6743f75` | Seam `closePlan` (bridge.go); `export_test.go` `PlanCloses` and `Plans` (own and live plan, read on the worker); `TestRefusedReplaceKeepsServedEndpointLive` now holds: the stale plan is not closed while refused (also across the retries), the refused build's plan is closed once, the stale plan once when the Replace applies. The pre-merge's mutation (`used[s.plan] = true` → `_ = s`) fails it ("closed 1 times"). An e2e test through the seam, not an internal one: the refused Replace needs the real node of the env (package `bridge_test`) |
| err-93 | `375c4c4` | Decision 77: a rollback whose stop fails puts the node into `fleet.orphans` (under `op`), keeps its files, logs `ha2fabric.node_stop` at warn with a hint; `fleet.Close` stops the orphans with the members. `TestCreateRollbackBounded` (helper only) became `TestCreateRollbackStopFails`, at the call site: the stop's deadline within `stopBudget` (red with `WithCancel` in place of `WithTimeout`), the files stay (red before), the daemon's stop stops the node again |
| test-99 | `bd88458` | `eventually` clears the last client error first |
| bound-98 | `18bfe8b` | Decision 78: `MaxCandidateBytes`' comment says the bound is the host's (the spec bounds the line not at all and `SdpMid` only below, `definition_gen.go:256-257, 282 at v0.27.0`), CONSTRAINT_ERROR kept |
| doc-97 | `2d53731`, `c018dc6` | `docs/by_design.md`, "Cameras behind an option": matter-hub bounds no session (`webrtc-provider-server.ts:25-34, 88-96, 291-315`, `webrtc-bridge.ts:90, 141, 216`, re-read at `664575a`; the search for `max|limit|evict|ResourceExhausted` over the four plugin files finds only sensor and stream parameters); go-ha2fabric's 8 per camera, 4 per fabric, eviction within the fabric |
| doc-94, -95, -96 | `5a33929` | operations.md: Delete has left the fabrics before a failed stop (listed not enabled, `fabrics_removed=0` on the retry); `addon/DOCS.md` the same; the candidate bullet names line plus `sdpMid`, both directions, the host's bound; `camera.session_evicted` logs `fabric`. changelog: bound-21 both ways; lines for err-41, bound-43, conc-65/err-93/err-66 and the Delete clause (F2 had carried the others) |

Decision 76 needs no code (4 of 8 kept).

### Guards at `c018dc6`

| Guard | Result |
| --- | --- |
| `make ci` | exit 0 (build, vet, test, lint 0 issues, cite, licenses-check) |
| `make cite` | exit 0 |
| `make race` (full) | exit 0 |
| `make cover-check` | exit 0 (`cmd/ha2fabric` 88.6, `internal/bridge` 93.3, `internal/camera` 95.1, `internal/ha` 95.2) |

Not rerun: fuzz-smoke, vuln, licenses, secrets, 386 (no fuzz target,
dependency or 32-bit-sensitive code moved since `f9b484b`).

### Left

Nothing of IDs 90–99.
