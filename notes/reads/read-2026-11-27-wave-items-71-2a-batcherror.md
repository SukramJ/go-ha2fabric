# Wave 83/71 — facts and verdicts for #71, follow-up 2a and `BatchError` (read-only)

Read on 2026-11-27. Pins: go-ha2fabric `main` = `b0cfc99`; go-fabric
`v0.28.0` and go-fabric-ui `v0.16.0` (module cache,
`thingfabric.dev/…@<tag>`); go-hamapping `v0.9.0`. Short names: `h2f:` =
go-ha2fabric at `b0cfc99`, `gf:` = go-fabric at v0.28.0, `ui:` =
go-fabric-ui at v0.16.0. Everything not marked **unverified** was read at
the line given. #83 is `read-2026-11-27-issue-83-camera-eviction.md`;
section 4 here is its delta at `b0cfc99`.

## 1. #71 — `--health`

### 1.1 Facts

| Fact | Where |
| --- | --- |
| The issue: connect to the web listener (`WEB_LISTEN`), exit 0 when the daemon answers and the bridge is connected, 1 otherwise; no credentials, prints no configuration; a test per case (daemon up, listener down, Home Assistant away); `docs/operations.md` and changelog; `addon/config.yaml`: "consider `watchdog` together with the unprivileged add-on issue" | `gh issue view 71`, 2026-11-27 |
| The plain image is `gcr.io/distroless/static-debian12:nonroot`; the binary is `/ha2fabric`; `ENTRYPOINT ["/ha2fabric"]`; `USER nonroot:nonroot`; no `HEALTHCHECK` | `h2f:Dockerfile:38, :39, :52, :51` |
| `WEB_LISTEN` / `--web-listen`, default `127.0.0.1:8099`, empty disables the UI | `h2f:internal/config/config.go:51, :251, :297`, `:119-120` |
| `--version` returns from `Load` before the validation (`HA_URL is required…` follows it); `WebListen` is already set there | `h2f:internal/config/config.go:306-308`, `:297`, `:309-310` |
| `run`: `Load`, then `Version`, then the logger, the Home Assistant client, the selection file | `h2f:cmd/ha2fabric/main.go:72-95` |
| One Home Assistant client for the daemon, fanned out to every node's bridge; `Client.Connected()` = "a connection is authenticated now" | `h2f:cmd/ha2fabric/main.go:92, :151-166`; `h2f:internal/ha/client.go:214-215` |
| A bridge's `Status.HAConnected` follows the fan-out's `Connected`/`Disconnected` | `h2f:internal/bridge/bridge.go:96, :342-344, :353-357` |
| **The web listener serves the Home Assistant connection nowhere.** `bridge.Status` is used in `cmd/` only for `Discards`; `docs/operations.md` says "The Home Assistant connection is in the log" | `h2f:cmd/ha2fabric/nodes.go:254-266` (the only `Status()` use found by grep over `cmd/`, `internal/web/`); `h2f:docs/operations.md:612-613` |
| `GET /api/v1/node/status` carries: node health, counts, window, `warnings` (codes only without admin), `counters`, `csrf_token` in token mode; it reads the fabrics and lists the exposure on every call | `ui:fabricui/handlers_node.go:150-192, :193-240`, `:30-62` (`CSRFToken` `:59`) |
| The warning codes this host sets: `passcode_test_value`, `token_in_clear`, `passcode_not_kept`, `ha_token_refused`, `label_unresolved`, `selection_corrupt`, `sync_failed` (from the second failed sync in a row), `node_erase_failed`, the Supervisor gate's. None says "Home Assistant not connected" | `h2f:cmd/ha2fabric/warnings.go:16-30`, `h2f:cmd/ha2fabric/main.go:251-266`, `h2f:internal/exposure/exposure.go:330-348, :355-380`, `h2f:internal/bridge/bridge.go:1046-1053` |
| The client filter answers 403 to every client outside `WEB_ALLOW`, before go-fabric-ui's handler; the Dockerfile's own example is `WEB_LISTEN=0.0.0.0:8099 WEB_ALLOW=192.0.2.10` | `h2f:internal/web/web.go:95-109, :84`; `h2f:Dockerfile:11-15` |
| go-fabric-ui's Host check admits `localhost`, `*.localhost` and an IP literal without a zone | `ui:fabricui/host.go:49-60` |
| 0.1.0's `GET /api/health` was removed by owner decision 2026-10-25; `/api/v1/node/status` replaced it | `h2f:internal/web/web.go:10-12`; `h2f:docs/operations.md:613-614` |
| In the add-on `WEB_LISTEN=0.0.0.0:8486`, `WEB_ALLOW=172.30.32.2`; the daemon replaces the unspecified host by the address the Supervisor gives the add-on, read from the Supervisor at start; `WEB_ALLOW` is forced to the Ingress proxy | `h2f:script/run.sh:82-83`; `h2f:cmd/ha2fabric/supervisor.go:349-355`, `h2f:cmd/ha2fabric/main.go:169-175`; `h2f:internal/config/config.go:299-302, :386-387` |
| The add-on image is `ghcr.io/home-assistant/{arch}-base`, binary `/usr/bin/ha2fabric`, started by `/run.sh` | `h2f:addon/Dockerfile` (the `FROM`, `COPY --from=builder`, `CMD` lines; line numbers **not taken**) |
| #72 (open): unprivileged user, AppArmor, and "`watchdog` in `addon/config.yaml`, using the health endpoint of the `--health` issue"; "When: after the owner's live Ingress test of 0.7.0" | `gh issue view 72`, 2026-11-27 |
| `TestImageDefaults` reads the `Dockerfile`'s instructions | `h2f:cmd/ha2fabric/image_test.go:22-40` |

### 1.2 Verdicts

- **What "bridge connected" is:** `ha.Client.Connected()` — the daemon's one
  authenticated Home Assistant connection (`client.go:214-215`). Not a
  bridge's first sync, not `sync_failed`, not a node's state: the issue
  names the connection only. "Home Assistant away" is `Connected() == false`,
  which includes a refused token (`client.go:196-200`).
- **Which route:** none of today's routes can answer. `/api/v1/node/status`
  does not carry the connection (1.1), is behind `WEB_ALLOW` (403 for a
  loopback probe in the Dockerfile's own example), hands out `csrf_token`
  in token mode and reads fabrics and the exposure list per call. So the
  probe needs **a route of the host's own**: `GET /healthz` in
  `internal/web`, ahead of the client filter and of go-fabric-ui's
  handler, answered only to a loopback client address; `200` with body
  `ok` when `Connected()`, `503` with body `unavailable` otherwise; no
  JSON, no header beyond the defaults. Any other client address falls
  through to the filter and the handler as today. This is the "health
  endpoint" #72 expects. It re-adds a health route after the owner removed
  `GET /api/health` (web.go:10-12) — **owner question Q1**.
- **Where the probe connects:** the host and port of `WEB_LISTEN` as `Load`
  reads it (flag over environment). An empty or unspecified host
  (`0.0.0.0`, `::`) is dialled on loopback (`127.0.0.1`, `::1`); a
  loopback host as given. **Not performable, exit 1 with a line on
  stderr:** `WEB_LISTEN` empty (no listener); a specific non-loopback host
  (the probe's source address would not be loopback; admitting "source
  equals the listener's own address" rests on a kernel property that was
  **not read** — left out, Q2); the add-on (`SUPERVISOR_TOKEN` set: the
  listener's address comes from the Supervisor at start and loopback is
  not bound, `supervisor.go:349-355`).
- **No credentials, no configuration:** `--health` returns from `Load` where
  `--version` does (`config.go:306-308`), so it needs no `HA_URL`/`HA_TOKEN`
  and validates nothing; `run` answers it before the logger and the client
  exist (`main.go:81-91`). stdout stays empty; stderr gets one fixed line
  per failure class without the address or any value read.
- **Dockerfile:** `HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 CMD ["/ha2fabric", "--health"]`
  (exec form: distroless has no shell). The numbers are a proposal, not
  read from anywhere. Docker runs it in the container's namespaces, so
  loopback reaches the daemon also with `--network host` — Docker's
  behaviour, **unverified** here (not read from Docker's source or docs).
- **Add-on `watchdog`: not in this wave.** The issue says "consider …
  together with the unprivileged add-on issue"; that issue (#72) is open,
  names the watchdog as its own item and waits for the live Ingress test.
  Besides, the probe is not performable in the add-on as built (above),
  and which address the Supervisor's watchdog probes from is **unverified**
  (Supervisor source not read). #72 gets a note: `/healthz` exists, its
  client rule is loopback only and must be widened there for the
  Supervisor's probe.
- **Consequence to document:** the container is `unhealthy` while Home
  Assistant is away, although a restart of the daemon does not bring Home
  Assistant back; an operator who restarts on `unhealthy` restarts for
  nothing. The issue's acceptance wants exit 1 there; it stays.

## 2. Follow-up 2a — `endpoints_changed` after a failed batch

### 2.1 Facts

| Fact | Where |
| --- | --- |
| The batch's error is logged, the sync goes on: settle, groups, states, status, `bridge.synced`, then `OnChange` | `h2f:internal/bridge/bridge.go:698-727` (log `:725-727`), `:728-823` |
| `changed` is computed from the **diff** (`ch.Added/Replaced/Removed`, group keys), not from what applied; `OnChange(moved, changed)` runs whatever `err` is; the sync returns `err` | `h2f:internal/bridge/bridge.go:824-828`; `h2f:internal/plan/diff.go:58-59` |
| `err` at `:828` is the batch's: the only assignment after the snapshot read is `:698` | `h2f:internal/bridge/bridge.go:617, :619, :698` (grep of `err =`/`err :=` in `:611-830`) |
| `OnChange`'s second argument becomes `endpoints_changed` on the UI's stream | `h2f:cmd/ha2fabric/nodes.go:156-163` |
| The web UI's reassemble is `Resync`, whose error is the sync's | `h2f:cmd/ha2fabric/main.go:177`, `h2f:cmd/ha2fabric/nodes.go:232-242`, `h2f:internal/bridge/bridge.go:1085-1100` |
| A failed sync is retried with a back-off while Home Assistant stays connected | `h2f:internal/bridge/bridge.go:583-608`, `:489-496` |
| settle clears the signature of a group with a refused Add or Replace, so the next sync's diff holds those operations again | `h2f:internal/bridge/bridge.go:893-918`, doc `:831-843` |
| The batch always runs, with an empty diff too (no guard before `:698`) | `h2f:internal/bridge/bridge.go:696-724` |
| go-fabric: "What applied stays applied whatever fails" | `gf:node/aggregator.go:579-580` |
| go-fabric: a failed reassembly leaves the set with the batch's changes while "the node still serves the topology before"; the next batch, "an empty one included", reassembles | `gf:node/batch_error.go:71-82`; `gf:node/aggregator.go:612-622`, `:694-698` (`reassembleIfBehind`) |
| go-fabric: an applied call whose store work failed is in `Calls` with `Applied` true; the set holds it | `gf:node/batch_error.go:122-132`; `gf:node/reserve.go:147-155` |
| go-fabric-ui: its routes publish `endpoints_changed` only after `reassemble` returned nil | `ui:fabricui/handlers_exposure.go:217-229`; `ui:fabricui/handlers_node.go:356-373` |
| go-fabric-ui's contract for the host: "A host that rebuilds on its own publishes `endpoints_changed` through `Options.Events` after its rebuild returned nil" | `ui:docs/embedding.md:152-154`; same in `ui:docs/provider.md:89-91`, `ui:docs/contract.md:267-270`, `ui:changelog.md:48-49` |
| The page: on `endpoints_changed` it sets `exposureNotInEffect = false`, reloads the status and moves `topologyEpoch`; its comment: "A rebuild returned nil: … so the stored selection is in effect" | `ui:ui/src/lib/store.svelte.ts:480-490` |
| The page sets the notice from the apply's answer (`reassembled === false`) | `ui:ui/src/lib/store.svelte.ts:296, :304`; shown `ui:ui/src/lib/pages/node/Selection.svelte:234, :338` |
| `surface_changed` also reloads the status and moves `topologyEpoch`, and leaves the notice | `ui:ui/src/lib/store.svelte.ts:491-501` |
| go-fabric raises `OnSurfaceChanged` "after an assembly changed an endpoint's served surface against the one persisted for its key" | `gf:node/observers.go:97-106`. Whether a newly added endpoint (nothing persisted for its key) raises it is **unverified** |
| This repo's changelog states the bridge publishes it "after a sync that changed the endpoints" | `h2f:changelog.md:55-57` |

### 2.2 Verdict

**Publishing after a failed batch is wrong.** In go-fabric-ui v0.16.0 the
event has one stated meaning — a rebuild returned nil, the stored
selection is in effect — and the page acts on it by clearing the notice
(`store.svelte.ts:480-490`). The bridge's sync is that rebuild (`Resync`),
and it publishes on the diff, so a batch refused in part publishes at
every retry (the refused group's operations are in each retry's diff,
`bridge.go:893-918`) and clears a notice that is still true. "Something
applied" does not rescue it: the event cannot say "partly", and with
`BatchError.Reassembly` nothing reached the wire at all
(`batch_error.go:71-82`). Making it depend on what `settle` found is the
wrong axis for the same reason — settle reads the set, and the set holds
changes the node does not serve yet.

But "only when `err == nil`" alone loses an event: after a failed
reassembly, or a store error of an applied call, settle finds everything
applied, the groups keep their signatures, and the healing sync has an
empty diff — its (empty) batch reassembles and returns nil
(`aggregator.go:694-698`) with `changed == false`.

**The rule:** the bridge owes one `endpoints_changed` from the first sync
whose diff changes endpoints until a sync whose batch returned nil, and
publishes it then:

```go
// worker-owned, like b.retry
b.endpointsOwed = b.endpointsOwed || changed   // changed as today, from the diff
endpoints := err == nil && b.endpointsOwed
if endpoints { b.endpointsOwed = false }
if b.opts.OnChange != nil && (len(moved) > 0 || endpoints) { b.opts.OnChange(moved, endpoints) }
```

`exposure` (`moved`) is published as today, also after a failed batch: it
says a decision moved, not that it is served. A batch refused before `fn`
(`node.ErrStopped`, `bridge.go:728-734`) is a failed batch like any other.

**Residual, to state not to fix:** between a batch refused in part and the
sync that heals it, an open Endpoints or topology page refetches only on
`surface_changed` (`store.svelte.ts:491-501`), which go-fabric may not
raise for every applied change (2.1, **unverified**); the status page
shows `sync_error` at once and `sync_failed` from the second failure
(`bridge.go:811-813, :1050-1052`). An event that says "the topology moved"
without "the selection is in effect" does not exist in go-fabric-ui
v0.16.0; asking for one is a go-fabric-ui question, not this wave's.

## 3. Follow-up — the typed `*node.BatchError`

### 3.1 Facts (go-fabric v0.28.0)

| Fact | Where |
| --- | --- |
| `Partition.Batch` returns nil, fn's error as it is when nothing failed after fn, `ErrStopped`, or a `*BatchError` | `gf:node/aggregator.go:589-592`, `:820-826` (`batchResult`), `:641-643` |
| `BatchError{Fn, Calls []*BatchCallError, Store, Reassembly}`; `Unwrap` is the non-nil parts, so `errors.Is` finds every sentinel | `gf:node/batch_error.go:57-83, :95-110`, doc `:48-56` |
| `Calls`: "the calls of the batch that failed after fn returned — a held call refused or failed at its judgement, an applied one undone, or the store work of a call — one entry per call, in call order" | `gf:node/batch_error.go:60-64` |
| `BatchCallError{Op BatchOp, Key device.SourceKey, Applied bool, Err error}`; `Key` is "the call's own key, as the host passed it: the endpoint's StableKey for an Add, the key argument otherwise"; a part-key collision is named in `Err`'s text only | `gf:node/batch_error.go:113-136`; `BatchOp` `:17-29` |
| **No call index.** The place of a call (`callOrigin.order`) is unexported; `Calls` is sorted by it and holds failed calls only | `gf:node/batch_error.go:144-151, :192-203`; `gf:node/aggregator.go:1121-1127` |
| `Applied` false: "the set is as it was without the call — refused at its judgement or undone". True: the call changed state and its store work failed ("an Add stays in the set, a Forget's device out of it, and the Forget can be made again"), or a held Replace by a withheld declaration | `gf:node/batch_error.go:122-132` |
| "A call that returned nil succeeded unless Calls names it, or a later call of the batch on the same key superseded or dropped it" | `gf:node/batch_error.go:51-54`; `gf:node/aggregator.go:592-597` |
| A call refused **at the call** returns its error there and is not in `Calls`; what fn returns is `Fn` | `gf:node/batch_error.go:58-59, :113-114`; e.g. `gf:node/reserve.go:497-519` |
| The skipped erasure — a Forget whose reservation cannot be read after fn — is recorded for its call with `applied` true: "node: the erasure of %q is skipped", "the Forget can be made again" | `gf:node/reserve.go:687-693`, doc `:668-672` |
| So are "the erasure of %q is not recorded" (after a failed reassembly) and "endpoint number reservation" (an Add's, Remove's or Forget's store work) | `gf:node/aggregator.go:761`; `gf:node/reserve.go:147-155` |
| An erasure left by an **earlier** batch has no call in this one: its error is `Store` | `gf:node/aggregator.go:676-678`; `gf:node/batch_error.go:65-70` |
| Held calls: refused → `applied` false; applied with an error → `applied` only for a `changedError` | `gf:node/aggregator.go:1228, :1341, :1207-1214` |
| Since v0.28.0, for go-fabric #616 | `gf:changelog.md:22, :30`; `gf:node/aggregator.go:611` |

Host side: the comment "its refusal joined into Batch's error without
saying which call it was" (`h2f:internal/bridge/bridge.go:689-695`); the
Forget comment "Typing that error so it can be recorded again is go-fabric
#616's scope" (`:704-712`); `opErrs` by index (`:696, :718-720, :723`),
filled with the batch's error when fn did not run (`:728-734`); settle
uses `errs[i]` for a Forget only (`:881-890`) and reads Adds and Replaces
from the node's set (`:855-880`). `plan.Diff`'s operations: `h2f:internal/plan/diff.go:77-141`;
that no two of them share kind and key was **not verified** (it holds if
no two planned groups hold one key — not read). An Add's `op.Key` is the
endpoint's `Options.StableKey` (`h2f:internal/bridge/bridge.go:1137`,
compared with `Spec().StableKey` at `:858-866`).

### 3.2 Verdict

- **"Without saying which call it was" is no longer true.** At v0.28.0 the
  batch's error names every call that failed after fn by kind and own key,
  in call order, and says whether it changed state.
- **It does not replace `opErrs`; it completes it.** `BatchError` carries
  no call index and not the calls refused at the call (those are only in
  what the call returned; `Fn` is the host's own join). The index slice
  stays as the host's record of call-time errors, and the entries of
  `Calls` are merged into it by `(Op, Key)`. Because the uniqueness of
  `(kind, key)` in a diff was not verified, an entry is attached to
  **every** operation of that kind and key — never "the n-th".
- **What the code can now know that it could not:** a Forget that answered
  nil and whose erasure was skipped or not recorded. It is a `Calls` entry
  with `Op == BatchForget` (`Applied` true), so after the merge
  `errs[i] != nil` and settle records the Forget again (`bridge.go:881-890`)
  — the gap the comment at `:704-712` names ("the entity, created again,
  gets its old number") closes. A refused held Forget (`ErrUnknownKey`,
  `Applied` false) is covered twice, by the entry and by `served`.
- **settle keeps reading the set for Add and Replace.** `Calls` cannot
  replace it: a call superseded or dropped by a later call on the same key
  is neither in `Calls` nor applied (`batch_error.go:51-54`), and the set
  is what decides whether a group keeps its signature.
- **`Store` and `Reassembly` need no new handling** beyond 2.2's rule: the
  sync has failed, is retried, and the next batch — an empty one too —
  runs what was left (`aggregator.go:612-622`).
- Whether a Forget made again while its erasure is recorded as "in
  progress" is harmless was **not read** (`reserve.go:283-415`,
  `finishForgets`/`cancelForget`); go-fabric's own text says "the Forget
  can be made again" (`batch_error.go:124-126`, `reserve.go:669-672`).

## 4. #83 at `b0cfc99` — delta to the #83 note

- The note was read at `04c205a`; #86 (`b0cfc99`) changed cites only in
  `internal/camera/camera.go` (30 lines), `docs/operations.md`,
  `docs/by_design.md` (`git diff --stat 04c205a b0cfc99`). The anchors of
  note 3.2 hold at `b0cfc99`: `:97-100`, `:140-154`, `:156-167`, `:171-173`,
  `:237-240`, `:268-269`, `:287`, `:303`, `:332-334`, `:477` (ProvideOffer),
  `:491-499`, `:547`, `:559`, `:573`, `:651`, `:747, :769, :798` (grep).
- **Note 3.3's go-fabric re-pins are already in the tree** (v0.28.0 at
  `camera.go:16, :23, :52, :71, :106, :107, :136, :144, :148, :214, :229, :378, :452, :471`,
  `internal/ha/camera.go:83, :85`, `docs/by_design.md:142`,
  `bridge_r6_e2e_test.go:24, :56`). Two differ from the note's mapping by a
  line: `camera.go:148` has `:613-617, :724-728` (note: `:612-618, :723-728`)
  — the sentence is rewritten by #83 anyway; `camera.go:214` has
  `:102-187` (note: `:102-186`; `provider_peer.go:185` is the function's
  `return`, `:186` its `}` — both ranges cover it).
- **Left to re-pin:** every `at v0.9.0` (go-hamapping) cite — 9 in 7 files
  outside `notes/`, `changelog.md`, `licenses/`: `docs/by_design.md`,
  `internal/adapt/adapt.go`, `internal/adapt/command.go`,
  `internal/bridge/bridge_c1_e2e_test.go`, `internal/bridge/bridge_p2r2_e2e_test.go`,
  `internal/camera/camera.go` (`:146`, `:308`), `internal/plan/input.go`
  (`git grep 'at v0\.9\.0'`). `make cite` fails on a cite that does not
  hold at `go.mod`'s version (`Makefile:98`), so the go-hamapping bump
  carries all nine. Their lines at v0.10.0: **to be read at v0.10.0**
  (not in the module cache on 2026-11-27).
- Doc anchors at `b0cfc99`: `docs/operations.md:448-460` (the bounds
  bullet), `docs/by_design.md:146-156`, `decisions.md:178` (72), `:187`
  (76), `phase-4-register.md:54`.

## 5. Not read

- Docker's HEALTHCHECK execution (namespaces, `--network host`); the
  Supervisor's watchdog (what it probes, from which address).
- Whether a packet with a local source address can arrive from the
  network (the premise of admitting "source equals the listener's
  address").
- go-fabric `node/reserve.go:213-470` (the erasure's store work) beyond
  the signatures; `OnSurfaceChanged`'s raising site.
- go-fabric-ui's debounce of the stream events; the Endpoints page's
  refetch beyond `topologyEpoch`.
- go-hamapping v0.10.0 (not released when this was written).
- This repo's test harness for a failing store (whether `fabrictest`
  offers one).
