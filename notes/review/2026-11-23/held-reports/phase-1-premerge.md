# Phase 1 fix wave — pre-merge read (Fable)

Read: `git diff 27b68ec..2fe4944 -- . ':!notes'` (F1 and F2), the commit
messages `27b68ec..HEAD`, decisions 23–34, both fix reports. Lines at
`2fe4944`. Probes in a `git archive 2fe4944` copy
(`…/scratchpad/PM/src`, files `internal/ha/probe_pm_test.go`,
`internal/selection/probe_pm_test.go`); nothing in the worktree was run or
changed except this file. M = read or probe ran, I = inferred.

## Verdict

**Not ready to merge.** Two fixes fail open on the exposure
(HR-P1-data-90, HR-P1-err-91, both measured by a probe, both contradicted
by a sentence `docs/operations.md` gains in this wave), and one status
warning does not clear as the docs and decision 28 say (HR-P1-ha-92).
All three are small. Everything else read holds; the rest are notes.

## Findings

| ID | Severity | Where | M/I | What | Fix |
| --- | --- | --- | --- | --- | --- |
| HR-P1-data-90 | medium | `internal/ha/client.go:683-721` (`listOf`, skip at `:697`), `internal/plan/plan.go:132-146` | M | A registry entry that does not decode is dropped from the list, but its entity still has a state, so `Assess` takes it as an unregistered candidate: `hidden_by`, its labels and its device's labels are gone, and the rule decides without them. A skipped device entry likewise drops the device's labels for all its entities; a skipped label entry makes an exclude label unresolved. Before the fix the sync failed (closed). `docs/operations.md:216-217` says "that entity is left out" — it is not | The skip must be neutral for exposure: keep the lenient read for `get_states` and the area list (a missing state is "no state yet", a missing area a name); for the entity, device and label registries either stay strict, or decode the id of a skipped entry (`entity_id`, `id`, `label_id`), carry the skipped ids in `Snapshot`, and have `Assess` give an entity whose own or whose device's entry was skipped a not-exposed verdict ("registry entry unreadable"); a skipped label entry fails the sync while the rule has label values. Red test: the probe below |
| HR-P1-err-91 | medium | `internal/selection/selection.go:132-142` (restore), `:115-117` (missing file) | M | After a corrupt `selection.json` is moved aside and the `.bak` is read, nothing writes the restored content back: the file is missing. The next start (no `Set` in between) takes the missing-file path — no overrides, no `.bak` read, no warning. Decision 25's protection lasts one start; the second one exposes every entity turned off, silently | In `Open`, after a restore that parsed, `atomicWrite(path, bb)` (log a failure, keep the Recovery); and/or read `.bak` when the file is missing and a `.bak` exists, with a Recovery. Red test: the probe below |
| HR-P1-ha-92 | low | `internal/ha/client.go:228-230` | M | `setRefused(false)` runs only after `serve` returns, so after `auth_invalid` followed by a good authentication `TokenRefused` stays true for as long as that connection lives (days): the status page shows `ha_token_refused` ("the bridge has no connection") while connected. `docs/operations.md:212-213` says "until an authentication succeeds". F1's `TestClientAuthRefusedClears` never had a refusal first | Clear it in `serve` right after `c.auth` succeeds; keep Run's reset of `authBackoff`. Red test: the probe below |
| HR-P1-conc-93 | low | `internal/bridge/bridge.go:340-346`, `:470-474`; `internal/plan/diff.go:72-75`, `:88-93` | I | `noteRemoved` now records at the event, so a removal that arrives while a sync is between its registry read and the `forget` swap is consumed by that sync, whose snapshot still plans the entity; `Diff` drops such a forget silently, and the following sync removes the key instead of forgetting it (the stored number stays). Before, the queue ordered it behind the sync | In `sync`, put back under `mu` every forget whose key `next` still serves (unless a newer one is recorded); or accept with bound-39 |
| HR-P1-err-94 | low | `cmd/ha2fabric/nodes.go:268`, `:290-300`; `gf:node/factory_reset.go:51-55` | I | An erase now ends with the daemon's base. A stop during the reset can leave what go-fabric documents for an ended context: "the fabrics are removed, the node is not stopped and keeps its store, and a later FactoryReset finishes the reset" — after the restart nothing finishes it. Before, the erase ran out (30 s) and the stop waited | Bound the erase by the stop budget instead of cancelling it at once (`close` already waits with `lockBy`), or accept and say in the docs that an erase interrupted by a stop must be repeated |
| HR-P1-cfg-95 | note | `internal/selection/selection.go:91-92`, `:118-127` | M (read) | The lenient read accepts any JSON object or `null` (`{}` is "version 0"); a file of another version whose entries this version does not understand loads zero overrides, and the message says its overrides "apply" — not that entities turned off may be exposed again | When the file is not empty and no entry was understood (or always for a higher version), use the "exposed again" wording |
| HR-P1-conc-96 | note | `internal/bridge/bridge.go:231-233`, `:242-247` | I | `enqueue`'s failed send and its `owed.Store(true)` are two steps; if the worker drains the whole queue between them the flag is seen only at the next piece of work. Needs 1024 closures applied between two statements; after a dropped `Disconnected` the next work is the reconnect | A one-slot wake channel beside `owed`, or accept |
| HR-P1-adpt-97 | note | `internal/bridge/command.go:122-141`, `:158-168` | M (read) | The range last sent stands in for 30 s unless a state shows exactly that range; a different range set in Home Assistant inside the window is overridden by the held bound on the controller's next single-bound write. Decision 32 accepts the 30 s | None now; goes with go-hamapping's range command |
| HR-P1-bound-98 | note | `internal/camera/camera.go:413-435`, `:477-489` | M (read) | At the cap the "idle longest" session is any standing live view older than a minute (a view signals nothing), of any controller or fabric: a ninth offer ends the oldest view. Documented (`docs/operations.md`, Cameras) and within decision 31 | None; the go-fabric follow-up (decision 34) replaces the idle measure |

## Probes (output lines)

- HR-P1-data-90, `TestProbePMSkippedEntryExposure` (rule `Include: *`,
  `ExcludeLabels: secret`; the entity hidden by the user and labelled):
  `hidden+labelled: automation.nachtlicht exposed=false reason="hidden in Home Assistant (by user)"`,
  then with `original_name: 5` on the same entry:
  `hidden+labelled, entry does not decode: automation.nachtlicht exposed=true reason="included by *"`.
- HR-P1-err-91, `TestProbePMSecondStartAfterRestore`:
  `start 1: entries=map[light.a:{false }] recovered=true`,
  `after start 1: selection.json exists=false`,
  `start 2: entries=map[] recovered=false err=<nil>`.
- HR-P1-ha-92, `TestProbePMRefusedThenConnected` (refused once, then
  `auth_ok` and kept up): `connected=true TokenRefused=true`.

## Checked and holding

- **Client** (`internal/ha/client.go`): the back-off resets only after
  `up > MaxBackoff`; `up` is set by the deferred function over the explicit
  zero returns; `auth_invalid` takes the long delay and leaves the short
  one untouched. The detached write is bounded by `CallTimeout`, a ctx
  already ended sends nothing. `unsubscribeLate` waits on the buffered
  result channel, which `fail` closes on a drop; a refusal sends nothing;
  the stream is dropped before, so late events go nowhere. The joined
  reader ends on `CloseNow`; no path leaves it blocked. The token is in no
  new log line.
- **Supervisor gate**: cloned transport without a proxy, redirect refused;
  an error keeps the gate where it was (closed at start).
- **config / web / nodes**: `LABEL` check; `Loopback` closes the listener
  before `Serve` and fails closed on a non-TCP or non-loopback address, only
  stand-alone under `WEB_ADMIN` without `WEB_ALLOW`; `tighten` only narrows.
- **Selection copy**: `.bak` is written 0600 by temp file and rename, only
  from a file that parses, always one generation behind; it goes with a
  deleted node. It can bring back at most the last change (stated in the
  warning) — apart from HR-P1-err-91.
- **Image and manifest**: no `go:embed`, the build needs nothing the
  `.dockerignore` drops; the image takes the binary's loopback default.
  Not built here (below).
- **Camera against go-fabric v0.25.0**: the port contract names a total
  bound as the port's with RESOURCE_EXHAUSTED
  (`gf:cluster/spec/webrtctransportprovider/server_gen.go:253-257`);
  `portError` passes a status-carrying error through
  (`gf:cluster/camera/chime.go:306-311`); `Provider.End` removes the session
  whether or not the command arrives and never takes `op`
  (`gf:cluster/camera/provider_peer.go:137-146`, `provider.go:82-86`), so
  the synchronous `tell` inside `ProvideOffer` (a failed re-offer) cannot
  deadlock; it holds the camera's commands for at most `SignalTimeout`. The
  kit keeps a re-offered session on a port error (`provider.go:560-563`), so
  the host's `End` is what unlists it. Idle timers are set and reset under
  `h.mu`, a fired timer checks `own`; the still's goroutine is counted in
  `h.wg` and runs on the host's context with the client's own timeout.
- **Bridge queue**: the owed sync after the queue drains; `Disconnected`
  as a sync that marks unreachable while the upstream is away; `forget`
  under `mu` on every access; `settle` and `serving` consistent with
  `plan.Diff` (apart from HR-P1-conc-93, -96).
- **adpt-30 against core 2026.10.3**: `climate/services.py:121-131` refuses
  only `target_low_temp > target_high_temp` (strict), so low == high passes
  core's service; `:48-62` has both bounds `Inclusive`. An integration's own
  refusal of an equal range is not core's and was not read (I).
- **Wording (decisions 21, 24)**: no tracked line added in
  `27b68ec..HEAD` and no commit message speaks of the held row or of the
  proxy's address beyond the check as it is; `cmd/ha2fabric/admin_test.go`'s
  cases name peers on other addresses being refused, and the proxy by its
  header. No attribution lines.

## Before the push

1. Fix HR-P1-data-90 and HR-P1-err-91, each with its red test; fix
   HR-P1-ha-92 (three lines and a test). The docs sentences at
   `docs/operations.md:212-213` and `:216-217` then hold as written (or
   follow the fix chosen for -90).
2. Decide HR-P1-conc-93 and HR-P1-err-94 (fix or one register line each).
3. Run `make docker addon-image` and `make version-check`: F1 changed
   `Dockerfile`, `.dockerignore` and `addon/config.yaml` and did not run
   them.
