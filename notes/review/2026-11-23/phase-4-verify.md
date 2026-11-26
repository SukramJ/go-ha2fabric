# Phase 4, run 1 — verification

Verifier: Opus in Fable's role (owner rule 2026-11-24). Baseline `c3a9770`;
reports `phase-4-S-web.md`, `phase-4-S-ha.md`, `phase-4-P.md`,
`phase-4-D.md` as committed in `606c3b0`. Probes in a `git archive c3a9770`
copy under the session's scratchpad (`p4/V`), the reviewers' appendix
probes reused unchanged; the s6 measurement in `p4/Vs6`.
`PATH=$HOME/go/bin:$PATH`. References at their pins: go-fabric v0.26.0
(module cache), the pinned Home Assistant base image
`amd64-base:3.24-2026.11.1@sha256:d4d8bc2b…`. M = measured (read or run
here), I = inferred.

Scope (brief, "After the reviews"): run 1 has no high and no medium, so
the four lows the coordinator named for a grade challenge (cfg-01, err-40,
bound-20, test-62), a sample of the other lows (S-web sup-03; P err-41,
adpt-42; D test-60, test-61, test-63, err-64), a skim of every note for a
re-grade, and the citations met on the way.

## Grade challenge: could any low be medium?

### HR-P4-cfg-01 (add-on stop budget ~3 s under s6) — **confirmed low**

- **Measurement re-run (M).** The appendix script P1a/b against the pinned
  base image: plain `docker stop -t 30` returned after 4 s, the CMD got its
  TERM and printed `alive 1`, `alive 2`, then was killed (exit not clean);
  with `-e S6_CMD_RECEIVE_SIGNALS=1` the same CMD ran its 20 s and printed
  `clean exit` (stop 24 s). Decision 29's premise (one 25 s budget inside
  `timeout: 30`) does not hold in the add-on. `addon/config.yaml` has
  `init: false` (`:25`), so `/init` is PID 1; `addon/Dockerfile:41-57`
  sets no `S6_*`.
- **Data loss or corruption? No (M by reading, I for SQLite's crash
  guarantee).** Every durable write the daemon and go-fabric make survives
  a SIGKILL:
  - the node store is SQLite (`gf:store/sqlite.go:16-56`, modernc.org,
    transactional); a process kill cannot tear a committed transaction,
    and an uncommitted one is rolled back at the next open;
  - go-fabric is built to restart after a crash: forgets a crash left
    half done are re-run (`gf:node/reserve.go:167-170`, `:209`,
    `gf:node/node.go:627`), a fail-safe interrupted is rolled back at the
    next start (`gf:node/observers.go:58`, `:82`);
  - `nodes.json`, `passcode.json` and `selection.json` are temp file,
    sync, rename (S-ha's verdict, `nodes.go:532-557`,
    `selection.go:366-389`).
  What the 3 s cut costs is **orderly**, not durable state: no clean
  close of the controllers' sessions and subscriptions (they recover by
  their own timeouts after the restart), no camera session ends (Home
  Assistant ends them when the WebSocket drops), and an **erase in
  progress** is cut — `gf:store/erase.go:60-95`: the erase transaction is
  atomic, but a kill after the fabrics were removed and before the reset
  leaves the store to be erased again, which the docs already name
  ("must be repeated after the next start", `docs/operations.md:719-722`);
  until then the removed rows' bytes stay in free pages (the scrub is the
  erase's last step). Only an operator who stops the add-on during an
  erase meets that, and the docs tell them what to do.
- **Grade.** A broken decision premise and two false sentences
  (`script/run.sh:7` "becomes PID 1 and receives the signals";
  `addon/config.yaml:44-46` "The daemon stops within 25 s"; plus
  `docs/operations.md:726-727`) with a slower recovery and no data
  loss: **low**, as HR-P1-err-64 was. Fix as proposed
  (`ENV S6_CMD_RECEIVE_SIGNALS=1` in the final stage), and correct the
  three sentences — the config comment was not in the report's fix list.
- **Citation corrections** to the report (wrong at `c3a9770`, the files
  are 57 and 97 lines): `addon/Dockerfile:92-108` → `:41-57`;
  `addon/config.yaml:129-132` → `:44-47`. Also in S-web's "Full cut":
  `dependabot-auto-merge.yml:149-152` and sup-04's `:140` name lines of a
  42-line file; the patch gate is `:29-30`. The verdicts do not change.

### HR-P4-err-40 (refused Replace leaves the served endpoint without a live plan) — **confirmed low**

- **M.** The appendix probe `TestP4RefusedReplaceKeepsServedEndpointLive`
  re-run: `sync_error="node: an endpoint with this StableKey is already
  added: ha:dev-zha-2/reg-binary_sensor-front_door#1"`, verdict endpoint 0
  while endpoint 3 is served, `never: the served endpoint follows the
  state` (20.2 s). The code reads as claimed: `bridge.go:714-716` closes
  every replaced group's old plan after `settle` (`:830-890`), which only
  takes the signature and refused Adds back, never the old group.
- **Reach and trigger (I).** A Replace is refused (a) at the call by
  `checkSurface`, `checkMatterTime`, `recordWithheld` (a store write) or
  `attach` (`gf:node/aggregator.go:696-778`), or (b) held and refused at
  the commit with `ErrDuplicateKey` when another endpoint serves one of its
  keys (`:417-436`). This composition sets no surface policy and maps no
  row that needs Matter time, so (a) is a store write error; (b) needs a
  part key `<key>#n` that another endpoint of the same node serves, which
  one bridge per node with group-UID-prefixed keys does not produce (the
  probe has to insert a foreign endpoint). So the trigger is a storage
  fault, rare and not driven from outside; the effect is bounded to the
  replaced devices and ends with the first retry that applies. An entity
  whose controller writes are dropped and whose `unavailable` is not
  shown is a real functional defect, but with no reach from a controller
  or from Home Assistant: **low**. It is HR-P1-err-31's (low) fix
  incomplete, rightly re-reported under the brief's rule.

### HR-P4-bound-20 (camera eviction across fabrics after 1 min idle) — **confirmed low; the fix needs one more piece**

- **M.** `TestProbeEvictionCrossesFabrics` re-run: fabric 2's offer at
  the bound ends a quiet, answered session of fabric 1 (`["end 2 12"]` —
  session 2, not 1 as in the report: the eight sessions' `last` tie and
  `idlest` takes the first in map order; same finding). `camera.go:452-476`
  and `:514-527` read as claimed; `session` (`:237-258`) carries no fabric.
- **Reach.** Only a commissioned controller sends `ProvideOffer`
  (decision 11's trusted principal; the kit checks fabric and node on the
  re-offer, ICE and End commands, S-ha's verdict). Its harm is
  availability of another ecosystem's live view, never data or control.
  And the cross-fabric reach is **not new with the eviction**: without
  it, a controller that holds all 8 sessions refuses every other fabric
  with RESOURCE_EXHAUSTED; with it, the other fabric's view is ended
  instead of refused. Decision 23 was taken on exactly that reach
  ("only a commissioned controller reaches them") and published the row
  for it.
- **Decision 23's premise.** "The idlest ≥ 1 min is an abandoned view" was
  never exact — `last` moved only on signalling before R7 too; R7 removed
  the 1 h idle end, which made the peer-gone path (decision 31) the end of
  lost controllers and the eviction the only end of an abandoned view.
  S-ha is right that the premise moved; the grade does not: **low**.
- **Adjustment of the fix.** "Evict only within the offering fabric, else
  refuse" stops a fabric from ending another's view, but leaves the
  lock-out above: fabric B with 8 sessions still refuses fabric A. The
  fix needs a per-fabric share as well (the kit's `MaxSessionsPerFabric`
  through go-hamapping's camera row, `hm:ports.go:59-65`, decision 9; or
  the host's own share until it lands). See owner question 4.

### HR-P4-test-62 (`internal/ha` floor 93 vs the 3-point window) — **confirmed low**

- **M.** `TestProbeHAFloorSpread` re-run: 96.5 → `ok=false` ("3.5 points
  above its floor"), 95.9 / 95.2 / 94.7 pass. The four values are F1's
  (`phase-3-fix-F1.md:62-66`, `:192-197`); `floors.txt:13` is `93`,
  `main.go:41` the 3.0 slack, and the header (`floors.txt:4-5`,
  `main.go:37-38`) still says 0.3–0.5.
- **Grade.** A required check (`coverage`, decision 57) red on about one
  run in four with no change — a CI flake that blocks merges, not a product
  defect, and the gate fails closed (it refuses, it does not pass bad
  code). The medium bar of this review (a defect a user or the gate's
  purpose meets) is not reached: **low**. 94 is the only whole floor in
  [93.5, 94.7], so the window is 0.7 points wide; the deterministic test
  of the timing-dependent branches is the real fix, 94 the stopgap.

## Sample of the other lows

| ID | Verdict | Basis |
| --- | --- | --- |
| HR-P4-sup-03 | **confirmed low** | M: `Dockerfile:1` `# syntax=docker/dockerfile:1`; the lint grep (`ci.yml:66-67`) matches `^FROM ` only; `addon/Dockerfile` has no directive and builds. A moving frontend tag runs inside an attested build, outside decision 62's pin. |
| HR-P4-err-41 | **confirmed low** | M: `TestP4ItemKeepsPlanReason` re-run: both items `state=unmappable reason=""`, the refusal ends at the colon. Decision 52's "the operator sees the reason" fails on the page. |
| HR-P4-adpt-42 | **confirmed low (I)** | Read: `gf:node/aggregator.go:590` reassembles only when `b.changed \|\| len(served) > 0`; `bridge.go:680` calls an empty Batch on the retry, which then clears `SyncError`. No seam to measure; an upstream candidate as filed. |
| HR-P4-test-60 | **confirmed low** | M by reading `nodes_test.go:75-96`: any client error → `0, nil`, 30 s bound on a request that starts a node synchronously. The carried row of decision 70; the proposed helper shape is right. |
| HR-P4-test-61 | **confirmed low** | M by reading `hatest.go:272-275` (live `s.config`, `s.exposed`, not `clone`d as `:268-271` are) and `:316-317` (marshalled after `s.mu.Unlock()`); probe not re-run (a `-race` build on this machine). Test-only, a `-race` flake class in CI. |
| HR-P4-test-63 | **confirmed low** | M: mutation `nodes.go:665` `node_label.go` → `node_labels.go` in `p4/V`: `go run ./script/cite cmd internal script testdata` exits 0, `-list` counts 8 citations in `nodes.go` instead of 9; restored after. |
| HR-P4-err-64 | **confirmed low** | M by reading `nodes.go:707-743`: `s.close`'s error is logged (`:722-724`), then `m.s = nil`, files, record and member removed — against the doc comment `:705-706`. Probe not re-run (a daemon start). Needs `Stop` to fail (its context ends while a bridge stop runs): rare. |

## Notes skimmed — re-grades

None re-graded. Considered:

- **HR-P4-cfg-02** (container image, Docker's 10 s): the same cut as
  cfg-01, but in a setting the operator controls (`--stop-timeout`,
  `stop_grace_period`) and with the same harmless outcome; a docs line —
  **note** stands. Fix it with cfg-01 so `docs/operations.md:717-728`
  becomes true for both.
- **HR-P4-sup-04** (four homes of the Go version): Dependabot's `docker`
  ecosystem covers `/` and `/addon` (`dependabot.yml:58-73`), so the first
  `golang` patch makes the drift real, and the SBOM then names a standard
  library the image does not carry. Close to low; kept **note** because
  no released artefact carries it yet (decision 71: no tag) and the
  lint-job check removes it before one does.
- bound-21, data-22, cite-23, doc-24, bound-43, conc-65, err-66, test-67,
  test-68, conc-69, sup-70: each grade holds as written (each needs a
  trusted party's hostile data, a rare double failure, or is tooling
  hygiene).

## New (80–89)

None. No code defect was met beyond the reports' rows. The report-text
corrections (S-web's line numbers above, bound-20's session number) are
not findings.

## Run 1: any confirmed high or medium?

**No.** Every row of run 1 is low or note after verification (0 high,
0 medium, 11 low, 14 note). By README "Done" (`README.md:235-241`), run 1
is the convergence run: the review ends after its fix wave (decisions
69, 70, 71), with no run 2 — provided the fix wave's pre-merge read
(IDs 90–99) finds no new high or medium.

## Table

| ID | Reviewer | Verifier | Verdict | One line |
| --- | --- | --- | --- | --- |
| HR-P4-cfg-01 | low | low | confirmed | 3 s kill re-measured; no store loss (SQLite, temp+rename, go-fabric crash recovery); citations and the config comment corrected |
| HR-P4-sup-03 | low | low | confirmed | `# syntax=` frontend unpinned, grep checks `FROM` only |
| HR-P4-bound-20 | low | low | confirmed, fix adjusted | cross-fabric end re-measured; reach is a commissioned controller and exists as lock-out without the eviction; fix needs a per-fabric share |
| HR-P4-err-40 | low | low | confirmed | probe re-run; trigger is a store fault, no outside reach |
| HR-P4-err-41 | low | low | confirmed | probe re-run; empty reason on the page |
| HR-P4-adpt-42 | low | low | confirmed (I) | empty retry batch does not reassemble; upstream |
| HR-P4-test-60 | low | low | confirmed | `call` drops the error, 30 s below a node start |
| HR-P4-test-61 | low | low | confirmed (read) | live maps marshalled outside the lock |
| HR-P4-test-62 | low | low | confirmed | 96.5 fails against floor 93; CI flake, fails closed |
| HR-P4-test-63 | low | low | confirmed | moved path silently dropped by `script/cite` (mutation re-run) |
| HR-P4-err-64 | low | low | confirmed (read) | failed stop still deletes record and member |
| HR-P4-cfg-02, sup-04, cfg-05, bound-21, data-22, cite-23, doc-24, bound-43, conc-65, err-66, test-67, test-68, conc-69, sup-70 | note | note | confirmed (skim) | sup-04 close to low, kept note (no release before the fix) |

## Owner questions (the four reports merged)

1. **cfg-01, the add-on's stop** (S-web 1). `ENV S6_CMD_RECEIVE_SIGNALS=1`
   with the guard test, rather than a fixed `S6_KILL_GRACETIME`.
   **Agree**: it puts decision 29's one budget back in the daemon;
   also correct `run.sh:7`, `addon/config.yaml:44-46` and the docs, and
   fold cfg-02's docs line into the same change.
2. **sup-03** (S-web 2). Drop the directive rather than pin it. **Agree**;
   extend the lint grep to `# syntax=` so it cannot return unpinned.
3. **sup-04** (S-web 3). Lint-job agreement check, keep the `golang` patch
   auto-merge. **Agree**.
4. **bound-20** (S-ha 1). Reviewer: evict only within the offering fabric,
   else refuse. **Agree, plus a per-fabric share**: alone it still lets one
   fabric's 8 sessions refuse every other. Best cut: the kit's
   `MaxSessionsPerFabric` (go-fabric is the gold standard for the bound)
   through a go-hamapping issue for the camera row (decision 9); in this
   fix wave the host's own fabric-scoped eviction and a share (e.g. 4 of
   8) until go-hamapping carries the kit's setting.
5. **err-40's shape** (P 1). The stale-plan cut (the old group live for
   exactly the refused keys). **Agree**.
6. **adpt-42 upstream** (P 2). A go-fabric issue beside #616, not in this
   fix wave. **Agree**.
7. **`ErrNoStableKey` probe as a standing guard** (P 3). **Agree** (30 ms,
   pins the answer at every go-hamapping bump; useful with the v0.27.0
   lift).
8. **test-62, the `internal/ha` floor** (D 1). 94 now, deterministic
   branches later. **Agree**; fix the header's spread in the same commit.
9. **err-64, a Delete whose stop fails** (D 2). Refuse the delete (500,
   record and member kept, a second Delete retries). **Agree**.
10. **sup-70, `gosec` on `cmd/`** (D 3). Enable it with one reasoned
    `nolint`. **Agree**.

## Probes run

| Probe | Where | Result at `c3a9770` |
| --- | --- | --- |
| s6 stop, pinned base, plain / `S6_CMD_RECEIVE_SIGNALS=1` | `p4/Vs6` | killed after `alive 2` (4 s) / `clean exit` (24 s) |
| `TestP4RefusedReplaceKeepsServedEndpointLive` | `internal/bridge` | FAIL as reported (20.2 s) |
| `TestProbeEvictionCrossesFabrics`, `TestProbeSdpMidUnbounded` | `internal/camera` | FAIL as reported (`end 2 12`; 65 536-byte `sdpMid`) |
| `TestP4ItemKeepsPlanReason` | `internal/exposure` | FAIL as reported |
| `TestProbeHAFloorSpread` | `script/coverfloor` | 96.5 `ok=false`, others `ok=true` |
| cite mutation `node_label.go` → `node_labels.go` | `p4/V` | exit 0, 9 → 8 citations (restored) |

## Tokens

About 150 k (reports and decisions read once, slices of the code and
go-fabric, six probe runs).
