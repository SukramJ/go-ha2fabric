# Phase 1 fix wave — F1, the security and daemon paths

Implementer F1 (Opus). Brief `notes/briefs/2026-11-23-review-p1-fix.md`,
section F1. Branch `review/p1`, base `27b68ec`. Rows from `phase-1-S.md`,
`phase-1-GB.md`, `phase-1-verify.md` (verdicts used) and decisions 23–30.
No path of F2's was touched.

## Commits

| Commit | Rows |
| --- | --- |
| `01cec31` fix(ha): unsubscribe an abandoned offer, back-off after short connections and refused tokens, lenient lists, detached writes, joined reader | ha-62 (+ ha-04), ha-07, ha-08, data-09, conc-22, conc-71 |
| `8ce56ed` fix(ha): read the states before the registries | data-67 |
| `36c706e` fix(camera): bound sessions per camera with an idle end, candidates per call and session, one still request; end a failed re-offer and an empty answer | bound-02, conc-03, bound-17, err-05, data-20 (answer) |
| `68d9beb` fix(selection): keep a last-good copy, read another version leniently, say what an empty start exposes; check keys and names | err-06 (+ cfg-35), data-20 (keys, names) |
| `66dc114` fix(nodes): tighten a secret file's wide mode; warn when the registry exceeds NODE_LIMIT | cfg-10, cfg-66 |
| `9c33110` fix(image): the web UI on loopback by default, 5540/tcp declared, a .dockerignore | cfg-12, sup-19 |
| `9ba8ad2` test(cmd): hold every administrator route of the contract at the daemon's wiring | test-16 |
| `3c7f9e2` fix(cmd): one stop budget with the nodes in parallel and timeout 30; keep a node whose reset failed; erase without the status lock; exit 0 on a stop during start | err-64, err-61, conc-63, err-72, test-70 |
| `7b24efe` test(ha): Add and Remove racing Event on the fan-out | test-70 |
| `e1bb75f` fix(cmd): the Supervisor gate's client follows no redirect and takes no proxy | ha-13, ha-14 |
| `8099d1a` fix(config): LABEL as text; the web UI refuses a non-loopback bind under WEB_ADMIN without WEB_ALLOW; label commas documented | cfg-15, data-20 (LABEL), cfg-21 |

Commits group a class where the fixes share one file (`internal/ha/client.go`,
`internal/camera/camera.go`, `cmd/ha2fabric/nodes.go`); interactive
staging is not available, so one commit per row was not practical there.

## Guards

At HEAD `8099d1a` (before and after the run; the report commit is
notes-only, no second run): `make ci` 0, `make race` 0,
`make cover-check` 0 (every package above its floor; floors not raised),
`make fuzz-smoke` 0. Logs:
`/tmp/claude-1000/-home-markus-Dokumente-GitHub/700e1304-db09-4033-9c98-588d8ec93391/scratchpad/F1/`.

Not run: `make docker addon-image` (the Dockerfile, `.dockerignore` and
`addon/config.yaml` changed — the coordinator's pre-PR set should run
it), `make release` / `version-check` (no version moved), `make vuln
licenses secrets` (no dependency changed).

## Per row

"Red" = the regression test was run against the unfixed code and failed.

| Row | Status | Test | Red |
| --- | --- | --- | --- |
| ha-62 (+ ha-04) | fixed: the abandoned subscription's late result is awaited and a successful one is unsubscribed (`session.unsubscribeLate`); sent after the result, so Home Assistant has registered the teardown — an `unsubscribe_events` sent at once could reach it before an `async_response` handler registered it | `TestSubscribeAbandonedIsUnsubscribed`; negative `TestSubscribeAbandonedRefusedSendsNothing` | yes / negative green |
| ha-07 | fixed: the back-off resets only after a connection that stayed up longer than `MaxBackoff`; no jitter added (decision 28 does not ask for it) | `TestClientBackoffGrowsAfterShortConnection`; negative `TestClientBackoffResetsAfterStableConnection` | yes (44 auths/s) |
| ha-08 | fixed (decision 28): `auth_invalid` waits `AuthBackoff` 5 min doubling to 1 h, `ha.token_refused` log, `Client.TokenRefused`, status warning `ha_token_refused`; no exit, add-on and stand-alone alike | `TestClientAuthRefusedBacksOffLong`, `TestClientAuthRefusedClears`, `TestTokenRefusedWarning` (daemon) | yes, both |
| data-09 | fixed: the five lists decode entry by entry (`listOf`), a bad entry is skipped, `ha.entry_skipped` at warn once per list, then debug | `TestListSkipsEntryThatDoesNotDecode` | yes |
| conc-22 | fixed: the write runs on `WithoutCancel(ctx)` bounded by `CallTimeout`; a ctx already ended sends nothing | `TestCallContextKeepsConnection` | yes (after tuning the frame size; timing-based, 24 MB frame against a server that stops reading) |
| conc-71 | fixed: `serve` joins the reader before returning | `TestNoEventAfterDisconnected` | yes |
| data-67 | fixed: `get_states` first. GB's premise (registry entry before the first state) is I and was not read; the order is safe either way | `TestReadSnapshot` (order) | yes |
| bound-02 | fixed: `MaxSessions` 8; at the bound the session idle longest (≥ `EvictIdle`, 1 min) is evicted (`End` 12 sent asynchronously, Home Assistant's session closed), else RESOURCE_EXHAUSTED before Home Assistant is asked; a re-offer does not count | `TestSessionsPerCameraAreBounded`, `TestIdleSessionEvictedAtTheBound` | yes |
| conc-03 | fixed (part b): `IdleTimeout` ends a session without signalling; **default 1 h, not S's 10 min** — once the peer connection stands a live view signals nothing, so 10 min would end every longer live view (question 1). Part a is go-fabric's (below) | `TestIdleSessionEnds`; negative `TestActiveSessionDoesNotEnd` | yes |
| bound-17 | fixed: candidates 64 per call and 256 per session (RESOURCE_EXHAUSTED); stills: one request per camera, shared by waiters of the same size, reused for 1 s, each waiter bounded by its own ctx | `TestCandidatesAfterSessionAreBounded`, `TestSnapshotsShareOneRequest`; negative `TestSnapshotWaitEndsWithContext` | yes (20 requests for 20 snapshots); the negative hung on the old code by construction, not run red |
| err-05 | fixed: a failed re-offer sends `End` (reason 0) | `TestFailedReofferEndsTheSession`; negative `TestFailedFirstOfferSendsNoEnd` | yes |
| err-06 (+ cfg-35) | fixed (decision 25): `selection.json.bak` written before each rename (only from a file that parses), read when the file does not; another version read leniently (`selection.version`); `Recovery.Message()` says what applies, and that turned-off entities are exposed again when neither helps. Registry-node behaviour stated in `docs/operations.md` (its rule is its selection alone, so it exposes nothing) | `TestOtherVersionReadLeniently`, `TestCorruptFallsBackToLastGood`, `TestCorruptBackupIsNotKept`, `TestNothingToRecoverWarns`; `TestRemoveFiles` (the `.bak` goes with a deleted node) | yes |
| ha-07/ha-08 docs | `docs/operations.md` (What the daemon reads and writes, Logs) | — | — |
| cfg-10 | fixed: `nodes.json`, `passcode.json` and a copy moved aside are set to 0600 when read wider (`nodes.mode_tightened`); not on Windows | `TestSecretFileModesTightened` | yes |
| cfg-12 | fixed (decision 27, breaking): the image drops `WEB_LISTEN=0.0.0.0:8099`; README, `docs/operations.md`, Dockerfile comment, `changelog.md` (summary and Changed) | `TestImageDefaults` (file guard on the Dockerfile) | yes |
| test-16 | added: every `x-admin` operation of go-fabric-ui's contract (read at the pinned version through `go list -m`) × client kind, through `webOptions`, which `run` now uses too; a mutation of `admin` (`&&` → `\|\|`) turns 84 cases red | `TestAdminTable` | a guard, green by design; mutation-checked |
| err-61 (note) | fixed: a failed reset stops the node before the come-back; a node that does not stop either is kept (`stale`), reset by the next erase, stopped by `close`. Seams `factoryReset`, `stopNode` | `TestEraseResetFailedKeepsNode`, `TestEraseResetAndStopFailedKeepsNode` | yes (port in use) |
| err-64 | fixed (decision 29): `addon/config.yaml` `timeout: 30`; one 25 s budget (`stopBudget`) for the whole stop; the backend wait bounded by it; the configured node and the registry nodes closed in parallel; `op` taken with the deadline (`lockBy`); an erase now runs on the daemon's base (ends with the stop); `ha2fabric.web_stop` logs the web server's shutdown error | `TestStopWithinBudget`, `TestAddonStopTimeout` | yes (11.5 s against a 3 s budget; no timeout in the manifest) |
| test-70 | added: negatives of 61 and 62 above; the fan-out's `Add`/`Remove` racing `Event` | `TestFanoutConcurrent` | a gap, not a defect: green |
| cfg-11 (note) | not fixed, accept: a node whose record does not start is logged and listed not enabled; refusing the file would move `nodes.json` aside and drop every registry node for one bad entry | — | — |
| ha-13, ha-14 (notes) | fixed: the gate's client refuses redirects and takes no proxy; docs say the Home Assistant connection honours `HTTP_PROXY`/`NO_PROXY` | `TestSupervisorGateNoRedirectNoProxy` | yes |
| cfg-15 (note) | fixed: `web.Options.Loopback`; under `WEB_ADMIN` without `WEB_ALLOW` `Run` refuses a listener that did not bind loopback (`localhost` resolving elsewhere) | `TestRunLoopback` | yes (served the wildcard, the test hung) |
| sup-19 (note) | (a) `.dockerignore`, (c) `EXPOSE 5540/tcp` fixed; (b) no `HEALTHCHECK`: accept for now — distroless has no shell, it needs a new `--health` flag; (d) base images by digest is phase 3's | `TestImageDefaults` | yes |
| data-20 (note) | fixed: an empty answer ends the session (`camera.answer_empty`); selection keys must have an entity id's form (dropped at Open, refused by Set), names ≤ 256 bytes; `LABEL` UTF-8 without control characters | `TestEmptyAnswerEndsTheSession`, `TestEntityKeys`, `TestLabelText` | yes |
| cfg-21 (note) | (a) docs (`docs/operations.md` Labels, `addon/DOCS.md`); (b) accept: the schema's `match()` is a Supervisor fact not read at a pin, and the daemon's start check stays the bound — for the live test | — | — |
| conc-22, conc-71, data-67 | above | | |
| conc-63 (note) | fixed: `served.mu` is held only to take the run out and to install the new one (`launch` / `install`) | `TestStatusDuringErase` | yes (1.9 s) |
| cfg-66 (note) | fixed: `ha2fabric.node_limit` at start; docs and config say the limit bounds creation | `TestNodeRegistry` (lowered limit) | yes |
| err-72 (note) | fixed: a failed start after the signal exits 0; `main` resets the signal handler after the first, so a second signal ends the process | `TestSignalDuringStart` | yes (exit 1) |
| not now | sup-18, HR-P0-cfg-27 (decision 22) | — | — |

Test files also adjusted: `TestCandidatesBeforeSessionAreBounded` (65
candidates after the session event are now refused), `FuzzCaptureSnapshot`
(clears the still reuse per body), `FuzzOpen` (another version is read,
not moved aside), `TestCorruptFileIsMovedAside` (its version case moved to
the lenient test).

## For F2 or the coordinator

- **`internal/exposure/exposure.go:345-352`** (F2's path) must take the
  text from `selection.Recovery.Message()` instead of formatting
  `r.Err`/`r.MovedTo` itself: with the lenient read `MovedTo` is empty and
  the current text would say "was moved to ; … started empty", and with a
  restored copy "started empty" is wrong. One line:
  `Message: r.Message(),`. Until then decision 25's warning text holds in
  the log (`selection.corrupt`/`selection.version` hints) but not on the
  status page.

## go-fabric facade questions (not in code)

- **bound-02**: the kit could take `ProviderConfig.MaxSessions` and answer
  RESOURCE_EXHAUSTED before the port (`gf:cluster/camera/provider.go:210-224`
  bounds only the id space). An `upstream` candidate, theme `adpt`.
- **conc-03 (a)**: the kit or the node could tell the host when no CASE
  session with a WebRTC session's peer is live any more, so a session ends
  on evidence instead of an idle bound.
- **bound-17**: whether CameraAvStreamManagement's invokes are serialised
  like the provider's (`op`) was not read; the host's single request now
  holds either way.

## Changelog lines added (`changelog.md`, 0.7.0)

Summary paragraph: "**The container image's web UI now listens on
loopback** (`127.0.0.1:8099`) …"; the two older mentions of "the image's
`0.0.0.0:8099`" were reworded.

Changed: "**Breaking for the container image:** it no longer sets
`WEB_LISTEN=0.0.0.0:8099` …, declares `5540/tcp` …, a `.dockerignore` …".

Fixed (in order): the back-off after a short connection; the refused
token (`ha.token_refused`, `ha_token_refused`); the abandoned camera
offer; one bad registry or state entry; a command whose caller gave up
mid-write; camera sessions bounded with an idle end; candidates after the
session event; shared stills; a failed re-offer and an empty answer;
`selection.json` last-good copy and lenient read; secret file modes;
`NODE_LIMIT` warning; the stop budget and `timeout: 30`; an erase whose
reset fails; `LABEL` text, the loopback bind under `WEB_ADMIN`, the
Supervisor gate's client and the proxy note.

F2's lines are not added (the coordinator did not ask).

## Differences from the brief

- conc-03's idle bound defaults to 1 h, not 10 min (question 1).
- ha-62 unsubscribes after the late result, not at once (the reason is in
  the row); G-B's alternative.
- err-64: besides the budget, an erase now ends with the daemon (it ran
  on the request's detached context before); this is what lets the stop
  stay inside the budget while an erase runs.
- Docs on the Supervisor's default stop timeout (10 s) rest on the
  verifier's M read (`sup:supervisor/apps/validate.py:558-560`).
- While grepping for the image's old default the output listed one line
  of a file under `held/`; it was not opened or used.

## Questions

1. **conc-03's idle bound**: 1 h (chosen: a live view kept open longer
   must be reopened) or S's 10 min (ends long live views after 10 min),
   or none until go-fabric's facade (a)? Eviction at the bound works
   either way.
2. **The exposure warning text** (above): F2, or the coordinator at the
   integration?
3. sup-19 (b): a `--health` flag for an image `HEALTHCHECK` — a follow-up
   issue, or not wanted?
