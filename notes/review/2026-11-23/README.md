# Whole-codebase review, November 2026

Owner decision 2026-11-17 (go-fabric-ui's `notes/review/2026-11-17/`,
decisions 1–6), the third repository of the three after go-fabric-ui
(done, `v0.12.0`) and before go-fabric-server (order changed by the owner on
2026-11-23: go-ha2fabric first). Goal: every weakness of go-ha2fabric found
and eliminated, over every aspect. The model is go-fabric's Review II
(`go-fabric/notes/review/2026-11-13/`) as go-fabric-ui adapted it: this
repository is a daemon that reads Home Assistant, plans with go-hamapping
and serves a bridge node on go-fabric, with go-fabric-ui's pages behind its
own listener. The cuts are the Home Assistant client, the plan and the
reconciler, the camera relay, the web wiring, the configuration and the
add-on, not a protocol stack and not the pages.

This file is the plan. It holds no finding. Every statement about the code
carries a `file:line` read at the baseline; every size is an estimate and
says so.

## Pins and baseline

| Pin | Value | Source |
| --- | --- | --- |
| Baseline HEAD | `10ee7b4fb57ca6e9889ecf9e1189bbcd91212025` (origin/main after #67, 2026-11-23) | `git rev-parse origin/main` after `git fetch` |
| Review branch | `review/p0`, worktree `../go-ha2fabric-review` | — |
| Version | `0.7.0`, unreleased (no tag; decision 8) | `internal/version`, `addon/config.yaml:7` |
| go-fabric | `thingfabric.dev/go-fabric v0.25.0` | `go.mod:11` |
| go-fabric-ui | `v0.13.0` | `go.mod:6` |
| go-hamapping | `v0.6.0` | `go.mod:8` |
| go-hamqtt | `v0.40.0`; go-ha-catalog `v0.3.2-0.20261024093528-d9582e022b8f` | `go.mod:7`, `:9` |
| Home Assistant core | `2026.10.3` (read, never run) | `CLAUDE.md`, "Commands" |
| home-assistant-matter-hub (helper reference) | `664575a` | `CLAUDE.md`, `docs/by_design.md` |
| Supervisor (Ingress facts) | `fe91b8db` | `cmd/ha2fabric/main.go:205-209`, `notes/reads/read-2026-11-23-ingress-host-facts.md` |

Reviewers read go-fabric, go-fabric-ui, go-hamapping, Home Assistant core
and the helper reference as `git archive` exports or module-cache copies of
these pins, never a sibling checkout's working tree.

In flight and not part of the baseline: PR #68
(`docs/spake2-salt-comment`, comment-only, closes #64; owner merges). What
lands on main during the review is read as delta by the next phase.

## Owner decisions

`decisions.md` carries them: rows 1–6 are go-fabric-ui's decisions 1–6
(2026-11-17) restated for this repository, rows 7–12 the owner's wizard of
2026-11-23.

| Decision | Choice |
| --- | --- |
| Cut | **Adapted, five phases**: 0 basis, 1 package pass, 2 flows and adversarial, 3 architecture, tests, docs and supply chain, 4 convergence. No matter.js parity phase |
| Baseline | origin/main **now** (`10ee7b4`); each phase reads what landed since as delta |
| Models | **Opus by default** for reviews and implementation; **Fable** for the security paths, the verification of every high and medium, every reference citation (go-fabric, go-fabric-ui, Home Assistant core, Supervisor, the helper reference) and planning; **Sonnet or a script** for inventories |
| Process | Per phase: inventory → reviewer per unit → one Fable verifier in batches; **at most five agents per phase for review and verification, at most two for a fix wave**; one bundled fix pull request per phase, squash, auto-merge on green CI; open security items held back until their fix merges |
| Sessions | **One fresh session per phase**; go-fabric-server's plan is written while this review is in phase 4 |
| Phase 1 | **One phase**, not halves (decision 7) |
| Releases | **No tag during the review** (decision 8): fixes land in the unreleased `0.7.0`; a tag only on the owner's release go, with the owed live Ingress test |
| go-hamapping | **Boundary only** (decision 9): how this repository calls it; a defect inside it becomes a go-hamapping issue |
| Fuzz | **Standing targets** (decision 10): every target written in phase 0 is committed and joins `make fuzz-smoke` |
| Trust | **Home Assistant trusted, its data not** (decision 11) |
| Live Home Assistant | **Fixtures only** (decision 12) |

## What is in this directory, and what is not

The plan (this file), the owner's decisions (`decisions.md`), the
dispositions of every finding (`phase-<N>-register.md`), the threat model
(`threat-model.md`, phase 0) and the reviewers' and fix agents' working
reports (`phase-<N>-<unit>.md`). Inventories go to
`notes/reads/read-review-p<N>-<unit>.md`.

A report that mentions a security item still open is not committed until
the pull request that fixes it; the register then says how many rows are
held back. Meanwhile held files live in `held/` of the review worktree,
untracked and listed in `.git/info/exclude` (go-fabric-ui decision 14).

## The tree the phases are sized from

Measured at the baseline with `git ls-files | xargs wc -l`.

| Area | Files | Lines | Read by |
| --- | --- | --- | --- |
| `cmd/ha2fabric` (composition root: `main.go`, `nodes.go`) | 2 | 954 | phase 1 |
| `internal/ha` (WebSocket client, messages, subscriptions, `Fanout`, snapshot, camera calls) | 6 | 1 374 | phase 1 |
| `internal/ha/hatest` (the fake Home Assistant, test support) | 2 | 838 | phase 3 (tests) |
| `internal/camera` (WebRTC signalling and stills relay) | 1 | 682 | phase 1 |
| `internal/bridge` (reconciler on go-fabric's node) | 3 | 826 | phase 1 |
| `internal/plan` (pure core: assessment, overrides, diff) | 3 | 580 | phase 1 |
| `internal/adapt` (HA → go-hamqtt model, command → `call_service`) | 2 | 726 | phase 1 |
| `internal/config` (environment, `SUPERVISOR_TOKEN`/`HA_TOKEN`, node config) | 2 | 484 | phase 1 |
| `internal/nodes` (node registry, `nodes.json`) | 1 | 487 | phase 1 |
| `internal/exposure`, `internal/expose`, `internal/selection` | 3 | 763 | phase 1 |
| `internal/web` (go-fabric-ui behind the client filter, Operation page) | 2 | 236 | phase 1 |
| `internal/version` | 1 | 75 | phase 3 |
| Go tests (`*_test.go`, 33 files) and `internal/arch` | 34 | 6 400 (about) | phase 3 (quality); phase 1 reads the tests of the unit it reviews |
| `testdata/` (Home Assistant fixtures) | 10 | — | phase 3 |
| `addon/` (manifest, Dockerfile, DOCS), `script/run.sh`, `Dockerfile` | — | — | phase 1 (S: `run.sh`, manifest), phase 3 (images) |
| `docs/`, `README.md`, `CONCEPT.md`, `changelog.md`, `addon/*.md`, `CLAUDE.md` | — | 4 500 (about) | phase 3 |
| `.github/workflows/` (6), `dependabot`, `Makefile`, `.golangci*`, `script/` | — | 1 230 (about) | phase 3 |

Facts the cut rests on, read at the baseline:

- **Web gate.** `admin` (`cmd/ha2fabric/main.go:190-202`): in the add-on
  `web.From(config.IngressProxy)` (peer address `172.30.32.2`,
  `internal/config/config.go:60-63`); stand-alone with `WEB_ADMIN`
  go-fabric-ui's token mode (nil); otherwise `web.ReadOnly`. Host check:
  `AllowedHosts` from `WEB_ALLOWED_HOSTS` and, in the add-on, `AllowHost`
  by the Ingress source (`main.go:116`, `:204-215`).
- **Client filter.** `internal/web/web.go:83-98` refuses a peer outside
  `Allow` (`WEB_ALLOW`); empty `Allow` means no filter (`:84-86`); an entry
  that does not parse is dropped (`:66-70`; go-fabric-ui HR-7).
- **Add-on.** `host_network: true`, `ingress_port: 8486`,
  `homeassistant_api: true` (`addon/config.yaml:29-45`); `run.sh` sets
  `WEB_LISTEN=0.0.0.0:8486` and `WEB_ALLOW=172.30.32.2`
  (`script/run.sh:70-71`); the daemon talks to Core through the
  Supervisor's proxy as an administrator with `SUPERVISOR_TOKEN`.
- **Web server.** `ReadHeaderTimeout` 10 s, `ReadTimeout` 30 s,
  `IdleTimeout` 120 s, no `WriteTimeout` (the event stream)
  (`internal/web/web.go:103-109`); plain HTTP.
- **Home Assistant client.** One WebSocket (`internal/ha/client.go:197`),
  REST for camera stills (`internal/ha/camera.go:95-160`).
- **Files written.** `selection` and `nodes` write through a temp file and
  `os.Rename` (`internal/selection/selection.go:181-194`,
  `internal/nodes/nodes.go:467-480`) and move a corrupt file aside
  (`selection.go:96`, `nodes.go:131`).
- **Fuzz.** One target, `FuzzDecode` (`internal/ha/message_test.go:68`);
  `make fuzz-smoke` and a CI job exist (`Makefile:78`,
  `.github/workflows/ci.yml:109`).
- **Coverage floors.** `script/coverfloor/floors.txt` (13 packages, 75–98).
- **Carried in from go-fabric-ui's review** (`threat-model.md` there,
  section 4): HR-1 (the Host check, since fixed by #66 — confirm), HR-2
  (container image listens everywhere without a filter), HR-4 (the log ring
  is the daemon's whole logger), HR-5 (plain HTTP stand-alone), HR-7 (an
  unparsable `Allow` entry turns the filter off for another caller).
  Open issue #64 (PR #68).

## Phases

| Phase | Content |
| --- | --- |
| 0 Basis | Pin the baseline (above). Mechanical evidence, each with HEAD and a summary line: `make ci`, `make race`, `make cover-check` plus an error-path coverage reading, `make fuzz-smoke`, `make vuln licenses secrets`, `GOARCH=386 make build` and `GOARCH=386 go test ./...`, `make version-check`, `make release`, `make docker addon-image` where the machine allows (else stated as skipped). Fuzzing: new standing targets for the parse sites (decision 10). The threat model, new (`threat-model.md`): the web listener and its gate in the three modes (add-on, stand-alone, `WEB_ADMIN`), the Home Assistant token and connection, untrusted entity data into the node and the pages, Matter controllers driving `call_service`, the camera relay, the files under the store, the add-on and container images, secrets in logs (the log ring) |
| 1 Package pass | Every hand-written Go package, unit by unit (table below); one workflow, one branch, one pull request (decision 7) |
| 2 Flows and adversarial | End to end: Home Assistant snapshot → plan → bridge → go-fabric node; controller command → adapt → `call_service`; camera session; web write → selection/nodes → resync; reconnect and shutdown. Attacker roles, exploits written as failing tests |
| 3 Architecture, tests, docs, supply chain | The package cut and `internal/arch`; test quality and the fake; every behavioural claim in `docs/`, `addon/DOCS.md`, `CONCEPT.md`, `README.md`, `changelog.md`; every citation of go-fabric, go-fabric-ui, Home Assistant core, Supervisor and the helper reference at its pin; workflows, images, release, Dependabot, licences and notices |
| 4 Convergence | A full-cut run over the HEAD of the day, repeated until one yields no new high or medium, with the delta of what landed |

### Phase 1 units

Four reviewers and one verifier. The security paths are cut out of the
packages they sit in, so that Fable reads them whole.

| Unit | Files | Lines (about) | Model |
| --- | --- | --- | --- |
| S, the security paths | `internal/web/*`, `internal/config/*`, `internal/ha/client.go`, `message.go`, `camera.go`, `internal/camera/camera.go`, `internal/selection`, `internal/nodes`, of `cmd/ha2fabric/main.go` the web and admin wiring, `script/run.sh`, `addon/config.yaml`, `Dockerfile`, `addon/Dockerfile` | 3 300 | **Fable** |
| G-A, the plan side | `internal/plan`, `internal/bridge`, `internal/adapt`, `internal/expose`, `internal/exposure` | 2 700 | Opus |
| G-B, the daemon side | `cmd/ha2fabric` (the rest), `internal/ha` (the rest: `subscription.go`, `fanout.go`, `snapshot.go`), the go-hamapping boundary (decision 9) | 1 700 | Opus |
| verifier | every high and medium; a sample of the lows (about one in five per unit, at least two) | — | **Fable** |

### Phase 2 units

| Unit | Content | Model |
| --- | --- | --- |
| X1 the network | An unauthenticated client on the LAN against the web listener in each mode (add-on on the host network, container image, stand-alone, `WEB_ADMIN`), DNS rebinding and a hostile origin, the Ingress source spoofed from the LAN, the Matter port's reach into `call_service` | Fable |
| X2 hostile entity data | Names, labels, attributes, areas, camera offers and candidates, stills from Home Assistant's integrations into go-hamapping's plan, the node's attributes (lengths, encodings), the log and the pages | Fable |
| X3 life cycle and flows | Reconnect, `Fanout`, resync under load, registry create/delete/rename, the camera session's end, shutdown; goroutine and subscription leaks; the write flows end to end | Opus |

### Phase 3 units

| Unit | Content | Model |
| --- | --- | --- |
| arch and tests | `internal/arch`, the package cut against `CLAUDE.md`; tests that assert nothing, missing negative cases, the fake against core's behaviour, floors, flakes | Opus |
| docs and supply | Every behavioural claim in the docs against the code; workflows (permissions, pins, `pull_request_target`), the two images, release, Dependabot auto-merge, branch protection, licences, `THIRD-PARTY-NOTICES.md` | Opus |
| cite | Every citation (go-fabric, go-fabric-ui, Home Assistant core `2026.10.3`, Supervisor, developers docs, helper reference `664575a`); the coordinator's script lists them and checks each range exists; Fable reads each that is the reason for behaviour, each doubtful one and a sample | Fable |

## Aspects

Authentication and the web gate · the Home Assistant token and connection
· untrusted entity data · what a Matter controller can make Home Assistant
do · secrets in logs, responses and files · concurrency and life cycle ·
resource bounds · error paths (fail open, swallowed errors) · correctness
against go-fabric's node, go-fabric-ui's `Options` and go-hamapping's plans
· configuration and the add-on · API and architecture · tests and guards ·
supply chain, images and release · docs and citations against code.

## Method per phase

As go-fabric-ui's plan, "Method per phase": map (script or Sonnet, a map
never a citation) → review (findings into `phase-<N>-<unit>.md`: ID,
severity, `file:line`, claim, reference line, proposed fix; measured and
inferred apart) → verify (one Fable verifier; highs and mediums re-read and
refuted or confirmed; a low or note verified by its fix's red regression
test, plus a sample of about one in five lows per unit) → fix (branch
`review/p<N>`, red test first, at most two implementers on disjoint paths,
one standing guard per defect class, the local guard set of `CLAUDE.md`
before the pull request, one pull request per phase; a facade gap becomes a
go-fabric, go-fabric-ui or go-hamapping issue, not a workaround). Decisions
go to the owner as a wizard, the recommendation first. Implementer briefs
forbid sub-agents (go-fabric-ui lesson). Push, pull request and the
owner's merge are the session's own calls; no tag (decision 8).

## Agents per phase (estimates)

| Phase | Review and verification | Cut | Fix wave |
| --- | --- | --- | --- |
| 0 | 4 | 1 Sonnet inventory, 1 Opus mechanical evidence and fuzz targets, 1 Fable threat model, 1 Fable verifier | ≤ 2 |
| 1 | 5 | 1 Fable (S), 2 Opus (G-A, G-B), 1 Fable verifier, inventory by script | ≤ 2 |
| 2 | 4 | 2 Fable (X1, X2), 1 Opus (X3), 1 Fable verifier | ≤ 2 |
| 3 | 4 | 2 Opus (arch and tests, docs and supply), 1 Fable (cite), 1 Fable verifier | ≤ 2 |
| 4 | 4 per run | 1 Fable (security paths), 2 Opus (plan side, daemon side, each with the delta), 1 Fable verifier | ≤ 2 |

## Finding IDs

`HR-P<N>-<theme>-<nn>`. Themes:

| Theme | Covers |
| --- | --- |
| `auth` | the web gate, token mode, the Ingress source, the client filter |
| `web` | browser-side: Host check, origin, framing as this host configures it |
| `leak` | a secret (HA token, passcode, onboarding codes) in a log, response, file or image |
| `ha` | the Home Assistant client: protocol, reconnect, subscriptions, `call_service` |
| `data` | untrusted entity data: lengths, encodings, hostile values |
| `conc` | races, locks, goroutine and subscription life cycle |
| `bound` | resource bounds: messages, lists, sessions, timeouts |
| `err` | error paths: fail open, swallowed error, wrong exit |
| `adpt` | against go-fabric's node, go-fabric-ui's `Options`, go-hamapping's plans: wrong call, a facade gap |
| `cfg` | configuration, defaults, the add-on manifest and `run.sh` |
| `api` | package cut, exported surface |
| `test` | tests and guards |
| `sup` | workflows, dependencies, images, release |
| `doc` | docs against code |
| `cite` | a citation that does not hold at its pin |

Status: open / fixed (#PR) / struck (reason) / accepted (decision number,
and where it is documented) / upstream (issue in go-fabric, go-fabric-ui or
go-hamapping).

## Done

Every finding down to note has one of the closing statuses; every defect
class found has one standing guard in `make ci` or a workflow; a
convergence run over the then-current HEAD yields no new high or medium;
the held-back rows are published with the pull request that carries their
fix.

## Open questions for the owner

None open. Decided on 2026-11-23 as decisions 7–12 (`decisions.md`).
