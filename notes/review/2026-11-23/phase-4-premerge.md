# Phase 4 run 1 — pre-merge read of the fix wave

Reader: Opus in the Fable role (owner, 2026-11-24). Branch `review/p4` at
`4424a66`; diff `c3a9770..4424a66 -- . ':!notes' ':!licenses'`. Context:
brief `notes/briefs/2026-11-26-review-p4-fix.md`, decisions 68–75, F1's and
F2's reports. Probes in a `git archive 4424a66` copy (scratchpad
`p4/PM`), package tests only; the worktree untouched but for this file.
Pins: go-fabric v0.27.0 (module cache, `gf27:`), go-hamapping v0.8.0
(`hm:`), matter-hub `664575a` (`hub:`), core 2026.10.3 (`core:`).

## Findings

### HR-P4-test-90 — low — `TestEvictionTakesTheFabricsOwnIdlest` is flaky

- **Where:** `internal/camera/bounds_test.go:450-480`.
- **What:** the test orders the sessions by offer time (2 ms apart), but
  `answeringUp` makes every relay act on a `session` and an `answer` event,
  and each act moves `last` (`camera.go` `touch`, `event`). When a relay
  goroutine runs late, session 1's `last` is newer than session 2's and
  the eviction takes session 2.
- **Evidence:** `go test -race -count=30 -run 'TestEviction|TestFabricShare|TestCandidateMid' ./internal/camera/`,
  5 runs: one failed with `bounds_test.go:479: ends ["end 2 12"], want fabric 1's idlest (session 1) only`
  (about 1 in 150 locally; a loaded CI runner more often).
- **Fix:** a plain `&upstream{}` (no script, so `last` is the insert time
  alone), or wait for each session's answer before the next offer, as
  `TestEvictionStaysInTheFabric` does.

### HR-P4-test-91 — low — decision 73's "plan stays open" is not held by a test

- **Where:** `internal/bridge/bridge.go:966-984` (`closePlans`);
  `internal/bridge/bridge_p4_e2e_test.go`.
- **What:** F1's report (row 3) says the test holds "the plan staying open".
  It does not: the mutation `used[s.plan] = true` → `_ = s` (the stale
  plan closed at the sync that keeps it) passes
  `TestRefusedReplaceKeepsServedEndpointLive`. `Plan.Close` removes only
  the change hooks (`hm:plan.go:599-608`); `Apply` still works, so states
  and reachability pass while a controller's command to the served
  endpoint would be dropped silently — the one clause of decision 73
  ("commands to `call_service`") this composition cannot drive end to end.
- **Evidence:** probe in the copy: `close-stale-plans` → `ok`; the other
  three mutations (`live` no-op, `g.served` dropped, second refusal not
  searched) fail the test.
- **Fix:** a seam `closePlan = (*hamapping.Plan).Close` and an internal
  test that counts the closes over a refused Replace: none for the plan
  kept stale, one for the refused build's own plan, and the stale plan's
  close after the Replace finally applies.

### HR-P4-err-93 — low — a Create whose rollback stop fails still leaves the node running outside the fleet

- **Where:** `cmd/ha2fabric/nodes.go:680-686`.
- **What:** when `reg.Add` fails and `s.close(rctx)` fails too, the files
  are removed (`RemoveFiles`) under the running node and the node is in no
  member, so `fleet.Close` never stops it — the shape decision 74 removed
  for Delete. F1 dropped the keep-the-files branch (`d7e083f`) for the
  floor. Narrow (a registry write failure and a stop held by a bridge
  lifecycle), so low.
- **Fix:** on `cerr != nil` keep the files and hand `s` to the fleet's
  close list (a member without a record, or a `f.orphans` slice that
  `Close` stops), logged at warn; or record the decision to accept it.

### HR-P4-doc-94 — low — "Delete … is not deleted" omits that the node has already left its fabrics

- **Where:** `docs/operations.md:675-678`, `addon/DOCS.md:104-105`; code
  `cmd/ha2fabric/nodes.go:762-764`.
- **What:** `Delete` calls `leave` (every fabric removed, the controllers
  lose it) **before** `s.close`; a stop that then fails keeps record,
  files and member, but the node has no fabric left. "Is not deleted" /
  "is kept" reads as if the controllers still had it. The second Delete
  logs `fabrics_removed=0`. This answers F2's open question 2 ("whether
  Delete still leaves the fabrics before a failed stop"): yes.
- **Fix:** one clause in both: "it has left its fabrics already; its record
  and files stay and a second Delete tries the stop again".

### HR-P4-doc-95 — low — bound-21's controller side and three of F1's lines are missing from the docs and changelog

- **Where:** `docs/operations.md:461-467`; `changelog.md` `[Unreleased]`.
- **What:** `ProvideIceCandidates` now refuses a candidate whose line plus
  `sdpMid` exceeds 1024 bytes with CONSTRAINT_ERROR (`camera.go:597-603`);
  the Limits bullet names the 1 KiB bound for Home Assistant's side only
  and not that `sdpMid` counts, and the changelog line names only Home
  Assistant's side. F1's draft lines for HR-P4-err-41 (the web UI shows
  why a camera with the option off or an oversized state is not mapped),
  HR-P4-bound-43 (quadratic signature) and HR-P4-conc-65/err-66 (bounded
  Create rollback, erase stops an earlier stale node) are not in
  `changelog.md` (F1 report lines 272-279). CLAUDE.md: a user-visible
  change gets a line.
- **Fix:** the bullet: "A controller's candidate over 1 KiB with its
  `sdpMid` is answered CONSTRAINT_ERROR; Home Assistant's … (line and
  `sdpMid` together)"; the four changelog lines from F1's draft.

### HR-P4-doc-96 — note — `camera.session_evicted` logs `fabric` too

- **Where:** `docs/operations.md:329`; `camera.go:488-489`.
- **Fix:** "(`session`, `for`, `fabric`; Cameras)".

### HR-P4-doc-97 — low — decision 72 needs a `docs/by_design.md` paragraph

- **What:** matter-hub serves cameras and bounds no WebRTC session, per
  camera or per fabric (fact note below); go-ha2fabric bounds 8 per camera
  and 4 per fabric, evicts only within the offering fabric and refuses
  with RESOURCE_EXHAUSTED. A difference in how a bridge behaves, going
  ahead of matter-hub: by CLAUDE.md it is recorded. Decision 23's cap had
  no row either; one paragraph covers both, under "Cameras behind an
  option".

### HR-P4-bound-98 — note — the controller's over-long candidate answers CONSTRAINT_ERROR for a host bound

- **Where:** `camera.go:597-603`; `gf27:cluster/spec/webrtctransportprovider/definition_gen.go:256-257, 274, 282`.
- **What:** the spec bounds neither `Candidate` nor `SdpMid` above (`SdpMid`
  "min 1" only); 1024 is the host's resource bound, and the host's other
  bounds (candidate counts, sessions) answer RESOURCE_EXHAUSTED. Not wrong
  — chip answers CONSTRAINT_ERROR for an out-of-range field — but the
  comment should say it is the host's bound, not the spec's. Accept or
  switch; no real candidate is near it (below).

### HR-P4-test-99 — note — `eventually` can name a stale client error

- **Where:** `cmd/ha2fabric/nodes_test.go:166-176`.
- **What:** `lastErr` is the last error of the daemon's whole life, not of
  this wait; after a later success the message names an old error.
- **Fix:** `d.note(nil)` at the start of `eventually`, or clear it on a
  successful `do`.

## Per check of the brief

1. **Fixes against rows and decisions.**
   - **Decision 72 (`internal/camera`).** Matches the decision: the share
     (`MaxSessionsPerFabric = MaxSessions/2`), eviction only within the
     offering fabric at either bound, RESOURCE_EXHAUSTED otherwise, a
     re-offer not counted (old session removed before the count). The
     fabric comes from the kit's `OfferArgs.FabricIndex`, which a
     re-offer cannot change (`gf27:cluster/camera/provider.go:310`). The
     tests bite: `idlest` without the fabric filter fails two, the share
     condition dropped fails two; "re-offer counted" is an equivalent
     mutation (the old session is deleted before the count). Flake:
     test-90. cite-23's comment holds (`gf27:cluster/camera/provider.go:65-86`,
     `MaxSessions` … `MaxSessionsPerFabric`).
   - **Decision 73 (`internal/bridge`).** Correct and complete for states,
     reachability (own and `unreachable`'s `g.served`), the read model's
     endpoint and the command gate; second refusal finds the served
     device through `old.stale`; `closePlans` closes every plan no group
     of `next` uses, own or stale, also at `Run`'s end — no leak found
     (refused build's own plan closed at the next sync; the stale plan
     closed when the Replace applies, the group is removed or forgotten).
     Concurrency: `stale`, `served`, `closePlans` run on the worker only;
     `serving` is still published under `b.mu`. Side effect, for the
     better: a forgotten group's plan was not closed before (only
     Replaced and Removed were) and is now. Test gap: test-91.
   - **Decision 74, conc-65, err-66 (`cmd/ha2fabric/nodes.go`).** Locks:
     `close` keeps `op` then `mu`, as before; `erase` drops `mu` around
     the stale node's stop and holds `op` (no `close` meanwhile). The
     retry relies on `Node.Stop` being resumable, which go-fabric states
     ("the next Stop finishes the work", `gf27:node/node.go:1556-1563`,
     `stopBegun` `:1608-1611`), and idempotent on a stopped node
     (`:1601-1603`), so a retry after a failed stale stop re-stopping the
     stopped run answers nil. `stopRun` is not repeated on a retry (the
     run's cancel is gone). Deadlines: rollback `stopBudget`; err-66's
     stop 10 s within `eraseTimeout`. Mutations: Delete ignoring the
     close error, `close` without retry, erase without the stale stop —
     each fails its test; the rollback reverted to `WithoutCancel`
     passes (`TestCreateRollbackBounded` tests the helper alone — a note,
     with err-93). `-race -count=3` on the new tests: clean.
   - **test-60.** `do` returns method, path, error and elapsed; `want`
     fails with it; `eventually` names the last client error (test-99);
     one client per daemon with `DisableKeepAlives`, the only constructor
     is `newDaemon`; `endpointIDs` returns its error; `TestRun` keeps its
     own client.
   - **`script/cite`'s missing-file check.** Bites: in the copy,
     `node/node_label.go:39` → `node_labels.go:39` and
     `cluster/camera/provider.go` → `providr.go` each give exit 1 with
     "not in go-fabric at v0.27.0"; `CITE_PATHS` at HEAD green. Limits, by
     design and stated: a path with no module named before it in the
     block, or without lines or a `v` pin, is not checked.
   - **sdpMid bound.** Both directions counted (line + mid; `candidateBytes`
     for Home Assistant's, inline for the controller's, `SdpMid.Value`
     empty when null); both mutations fail `TestCandidateMidIsBounded`. A
     valid core candidate is not refused: core passes the candidate dict
     through unbounded (`core:homeassistant/components/camera/webrtc.py:71-81, 325-328`),
     its line is under 300 bytes (RFC 8839 §5.1, the comment) and its
     `sdpMid` is the SDP's media id ("0", "1"); a controller's mid
     likewise. Status: bound-98.
2. **New defects.** err-93 (Create rollback) only; no race (`-race` on
   camera, bridge, cmd new tests), no deadlock, no fail-open found.
3. **Docs and changelog against the code.** Camera sessions (operations
   Cameras bullet, changelog *Changed*) match `ProvideOffer`; Delete:
   doc-94; bound-21 and F1's missing lines: doc-95; log row: doc-96.
   `operations.md:727` (a stale registry node's Delete stops it first,
   500 while it cannot) matches `close`. cfg-01/02 text matches
   `addon/Dockerfile`, `script/run.sh` and `TestAddonStopSignal`.
4. **Citations at v0.27.0** (read, each the construct named): 
   `docs/guides/camera.md:93-114` (the four signals through `End`),
   `:137-144` (the payload paragraph, `ErrPayloadTooLarge`), `:581-603`
   (The ecosystem gate); `cluster/camera/provider.go:65-86` (`MaxSessions`,
   `MaxSessionsPerFabric`), `:172-178` (`serial` handlers), `:629-633`
   (`ProvideOffer`, `upsertSession`), `:680-697` (`provideIceCandidates`,
   no count or length bound), `:712-727` (`maxEndReason`, `endSession`);
   `provider_peer.go:56-67` (`ErrPayloadTooLarge`), `:102-173` (`Offer` to
   `End`'s end); `state.go:13-28` (`serial`); `chime.go:302-311`
   (`portError`); `im/status.go:94` (`StatusInvalidInState`);
   `node/config.go:960-969` (`checkPBKDF`), `:1091-1097`
   (`MDNS.Interfaces`); `node/reserve.go:615-631` ("the erasure of … is
   skipped"); `node/factory_reset.go:51-55`; `node/node.go:42-46`
   (`ErrStopped`); `node/node_label.go:39` (`SetNodeLabel`);
   `cluster/core/basic_information.go:651` (32 bytes);
   `cluster/camera/avstream.go:871-888` (`videoStreamAllocate`'s checks);
   `docs/guides/operations.md:72, 76`; `levelcontrol/values.go:14-44`
   (`LevelFromUnit`, `UnitFromLevel`); `script/coverfloor/main.go:73-81`.
   All hold (24 ranges).
5. **Fact note, matter-hub `664575a` and camera sessions.** matter-hub
   serves cameras (a built-in camera plugin, `hub:packages/backend/src/plugins/builtin/camera/`,
   `camera-plugin.ts`, `webrtc-provider-server.ts`, `webrtc-bridge.ts`) and
   bounds no WebRTC session: `provideOffer` mints an id
   (`webrtc-provider-server.ts:25-34, 88-96`) and appends to
   `currentSessions` (`:300-301`) with no count, per camera or per fabric
   (the fabric index is read only to fill the entry, `:291-315`); the
   relay's `#sessions` map is unbounded (`webrtc-bridge.ts:90, 141, 216`);
   no eviction and no RESOURCE_EXHAUSTED anywhere in the plugin (search
   for `max|limit|evict|ResourceExhausted` over the four files: only
   sensor and stream parameters). The audio `referenceCount` is clamped
   to 255 (`av-stream-server.ts:221-230`), a counter, not a bound. So
   decision 72 (and decision 23's cap) is a by_design paragraph: doc-97.
6. **F1's open question (4 per fabric, a third fabric locked out).** The
   decision's premise holds: an established live view signals nothing,
   so "idle" does not mean abandoned, and cross-fabric eviction ended live
   views (bound-20). The residual cost is real but narrower than "8
   viewers": a controller that abandons its views while staying on Matter
   leaves sessions only its own fabric can now reclaim, so two such
   ecosystems at 4 each lock a third out until the node proves them lost.
   **Recommendation: keep 4 of 8 for this wave.** `MaxSessions/3` is 2
   (integer), halves each ecosystem's viewers and still locks out a
   fourth fabric (Matter nodes support at least 5). The lasting answer
   belongs in the go-hamapping issue: the kit's `MaxSessionsPerFabric`
   has the same shape, so ask there for the total to follow the share
   (share × SupportedFabrics), or for a fair-share eviction at the total
   bound (a fabric below `MaxSessions / fabrics holding sessions` may
   evict the idlest session of the fabric holding most). Not a finding.

## Counts

0 high · 0 medium · 6 low (test-90, test-91, err-93, doc-94, doc-95,
doc-97) · 3 note (doc-96, bound-98, test-99).

## Ready to merge?

**Yes, after the small lows.** No high or medium, so run 1 stays the
convergence run (decision on `phase-4-verify.md`'s count). Per decision 75
the lows and notes are fixed in phase 4's pull request: test-90 (a
flaky test reaches CI otherwise), doc-94, doc-95, doc-96 and doc-97 are a
few lines each; test-91 and err-93 one small change each, or recorded as
accepted.

## Owner questions

1. **Decision 72's third fabric** (check 6). Recommendation: keep 4 of 8;
   carry the lock-out into the go-hamapping issue as the reason for a
   share-aware total or a fair-share eviction.
2. **err-93: fix or accept?** Recommendation: fix (hand the unstopped node
   to the fleet's close list, keep its files) — it is the shape decision
   74 just removed for Delete.
3. **bound-98: CONSTRAINT_ERROR or RESOURCE_EXHAUSTED** for a controller's
   over-long candidate? Recommendation: keep CONSTRAINT_ERROR (a field
   out of the host's range, as chip answers one) and say in the comment
   that it is the host's bound.

## Token use

About 190 k.
