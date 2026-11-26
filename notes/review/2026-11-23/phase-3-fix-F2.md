# Phase 3 fix wave — F2 (supply chain and docs)

Implementer F2 (Opus). Brief `notes/briefs/2026-11-26-review-p3-fix.md`,
section F2. Worktree `go-ha2fabric-review-F2`, branch `review/p3-F2`, base
`2f11edd` (`review/p3`).

## Commits

| Commit | Rows |
| --- | --- |
| `913d8a2` | licenses: generated licence tree and current third-party notices (sup-37, doc-38) |
| `03e8b1f` | images: base images by digest, licence texts and OCI labels in both images (sup-34, sup-37, sup-48) |
| `8461530` | ci: Dependabot auto-merge for patch updates outside the family only, gated on the author (sup-31, sup-47) |
| `76d2b59` | ci: actions by full SHA, pin and licence guards, CodeQL for actions (sup-35, sup-34, sup-37, sup-45) |
| `3640556` | ci: publish only tags on main, dispatch from main for a release tag, attested provenance and SBOM (sup-32, sup-36, sup-46) |
| `2e908b7` | docs: security policy and the maintainer address (sup-44, doc-50) |
| `b93fb2e` | docs: mapped entities from go-hamapping v0.8.0, README leftovers, the 0.7.0 camera eviction line (doc-39, doc-40, doc-41) |
| `06ebd36` | docs: roadmap markers, the web UI node's rule, CONCEPT's status (doc-42, doc-43) |
| `b2bcc99` | docs: citations at go.mod's versions, matter-hub's composition, the adapter's Matter knowledge by symbol, CLAUDE.md's state (cite-60, -63..-67) |
| `1957bd3` | docs: the last two citations at go.mod's versions (cite-60, -62) |
| `bcf1ad8` | docs: co-installed add-ons and the Supervisor-address bind, the old image names, attestations (decisions 53, 54; sup-49, sup-36) |
| `7fda407` | changelog: the wave's supply chain and docs lines |
| (this file) | review: phase 3 fix report F2 |

## Guards

| Guard | HEAD | Result |
| --- | --- | --- |
| `make ci` | `7fda407` | exit 0 |
| `make version-check` | `7fda407` | exit 0 |
| `licenses-check` (`script/licenses/generate.sh --check`) | `7fda407` | exit 0 |
| `make release` | `7fda407` | exit 0 |
| every archive holds `licenses/MODULES.txt` | `7fda407` | exit 0 |
| `actionlint` v1.7.12 | `7fda407` | exit 0 |
| `make docker` | `7fda407` | exit 0 |
| `make addon-image` (`BUILD_ARCH=amd64`) | `7fda407` | exit 0 |
| both images: `MODULES.txt` equals the tree; OCI labels present | `7fda407` | exit 0 |
| F1's `script/cite` over the docs (not roadmap) | `1957bd3` | 0 findings |

`make release`, `docker`, `addon-image` and `licenses-check` ran with a
scratch copy of the Makefile carrying exactly the lines of "Makefile lines
for the coordinator" below (the branch's Makefile is F1's); `make ci` and
`version-check` ran with the branch's Makefile. `actionlint` v1.7.12 via
`go run` (no local install). Not run: `make race cover-check fuzz-smoke`
(no Go code changed), `make vuln secrets` (no dependency change; the new
`licenses/` texts are third-party licence files).

**Not run anywhere: the SBOM step** (`cyclonedx-gomod` v1.12.0). A local
`go run` of it was refused by the session's auto-mode classifier, so its
flags (`app -json -licenses -std -main cmd/ha2fabric -output … .`) are
from its README at v1.12.0, not from a run. The attest steps, the
`ghcr` environment and the dispatch path can only run on GitHub. The
first tag after the merge is their test; a `workflow_dispatch` of
`docker-build-push` for an existing tag from `main` is a cheaper first
test (it rebuilds that tag's own source; `:latest` moves only when it is
the newest tag).

## Per row

### HR-P3-sup-31, -sup-47 (decision 57) — fixed

- `dependabot.yml`: groups `family` (`thingfabric.dev/*`, `sukramj.dev/*`,
  minor and patch: the five family modules and any later one under those
  domains), `go-patch`, `go-minor`; the first matching group wins, so a
  family module never lands in `go-patch`. `reviewers:` removed;
  `.github/CODEOWNERS` (`* @SukramJ`) requests the owner's review instead.
- `dependabot-auto-merge.yml`: `if` on `github.event.pull_request.user.login`
  and `github.actor`; merge only for `update-type ==
  version-update:semver-patch` (fetch-metadata reports a group's highest
  change) and no `dependency-names` under the two domains. Action comment
  `# v3.1.0`.
- The required checks are a setting (calls below).
- Difference: B's "CI step that fails a go.mod change without a
  changelog line" is not built (not in the brief's item; a candidate).

### HR-P3-sup-32, -sup-46 (decision 58) — fixed in the workflows; settings listed

- `release-on-tag.yml`: `git merge-base --is-ancestor "$GITHUB_SHA"
  origin/main` before anything else; top-level `contents: read`, the job's
  own `contents: write` plus the attestation permissions; `cache: false`
  (B's table note: no module cache from another ref in a release build).
- `docker-build-push.yml`, `addon-image.yml`: one resolve step (input via
  `env:`, sup-46; a dispatch from any ref but `refs/heads/main` fails;
  the version must match `^[0-9]+\.[0-9]+\.[0-9]+$`), then the checkout of
  `refs/tags/v<version>` — a dispatch rebuilds an existing release tag
  from its own source, never the dispatched branch; then the ancestry
  check against `origin/main` and the version check against
  `addon/config.yaml` and `internal/version` (now in both workflows);
  `:latest` is pushed only when the tag is the newest `v*` tag; the
  `tag` input is required. `environment: ghcr` on both jobs (the call
  that creates it is below; unprotected until it runs, GitHub creates a
  referenced environment on first use). `BUILD_DATE` is the commit date
  (a rebuild is reproducible); the `type=gha` build cache is dropped
  (a tag build no longer reads a cache other refs wrote).
- Difference from verify's "refuse an existing tag" for dispatch:
  dispatch now exists to rebuild a released tag from that tag's source,
  so it overwrites `:<version>` with the same source on a newer pinned
  base by design; the overwrite with other code, the medium of the row,
  is gone. Say if dispatch should be dropped instead.

### HR-P3-sup-33 (note) — settings only

The binding of the required checks to app 15368 is in the calls below.

### HR-P3-sup-34, -35, -36 (decision 62) — fixed

- **Digests (sup-34).** `Dockerfile`: `golang:1.27.4-alpine@sha256:70bd7d…`,
  `gcr.io/distroless/static-debian12:nonroot@sha256:afa5c8…`.
  `addon/Dockerfile`: three stages `base-amd64`, `base-aarch64`,
  `base-armv7`, each `FROM ghcr.io/home-assistant/<arch>-base:<tag>@sha256:…`,
  and `FROM base-${BUILD_ARCH}`; `BUILD_FROM` is gone (the Supervisor's
  local build passes `BUILD_ARCH` too). Reason: Dependabot's `docker`
  ecosystem updates `FROM` lines, not an `ARG` default or a workflow
  matrix value, so one `FROM` per arch is what keeps all three current.
  Tags: `3.24-2026.11.1` (= `latest` today, same digests) for amd64 and
  aarch64; **armv7 `3.22-2025.11.1`**: Home Assistant publishes no armv7
  base after it (`ghcr.io/home-assistant/armv7-base` tag list: newest
  `3.22-2025.11.1` = `latest`; M, registry API). `dependabot.yml`:
  `docker` for `/` and `/addon`. Guard: the lint job's step "actions by
  full SHA, base images by digest" (greps both Dockerfiles; `FROM
  base-${BUILD_ARCH}` the one exception).
- **SHAs (sup-35).** Every `uses:` is `@<40 hex> # vX.Y.Z`: checkout
  v7.0.1 (what `@v7` resolves to today; `codeql.yml` moves from v7.0.0 to
  it), setup-go v7.1.0, the docker actions with their exact versions
  (same SHAs as before), action-gh-release v3.0.4, fetch-metadata
  v3.1.0, attest v4.2.2. Same lint step greps it.
- **Provenance and SBOM (sup-36).** Choice: `actions/attest` v4.2.2 for
  all three artefacts (provenance by default, SBOM with `sbom-path`);
  `actions/attest-build-provenance` is a wrapper of it and
  `actions/attest-sbom` is deprecated in favour of it (its v4.1.0
  `action.yml` prints that warning). Not buildx's `provenance`/`sbom`:
  those would put an attestation manifest into the add-on images' index,
  which the Supervisor does not expect (the existing comment), and would
  be a second mechanism beside the archives'. The SBOM is CycloneDX from
  `cyclonedx-gomod app` at v1.12.0 (`go install` by version, verified by
  the Go checksum database, like `make secrets`' `go run`; no new
  third-party action); it lists the Go modules as linked, not the base
  image's packages (those are fixed by the digest in the Dockerfile the
  provenance's commit names). Daemon image: attestations pushed to the
  registry (`push-to-registry: true`); add-on images: kept in GitHub's
  attestation store only, so the registry holds exactly the plain image
  the Supervisor pulls (I: whether a referrer would disturb the
  Supervisor is not measured; this avoids the question). Permissions on
  those jobs only: `id-token: write`, `attestations: write`,
  `artifact-metadata: write` (the storage record; organisation-owned
  repository), `packages: write` where pushing. The archives' SBOM is
  also a release asset. Guard: `gh attestation verify` (README).

### HR-P3-sup-37 (decision 59), HR-P3-doc-38 — fixed

- `script/licenses/generate.sh`: the module set is `go list -deps` of
  `./cmd/ha2fabric` for linux amd64, arm64 and arm (`CGO_ENABLED=0`);
  per module the root's licence, notice, authors, contributors, patents
  and third-party-notices files plus its `licenses/` directory (go-fabric:
  `NOTICE`, `licenses/NOTICE-matter.js.txt` and its other texts;
  go-fabric-ui: `licenses/npm/*`, the embedded bundle's eight); Go's
  `LICENSE` and `PATENTS` from `GOROOT`; `licenses/MODULES.txt` with the
  versions. Not `go-licenses save`: it exits 1 on `modernc.org/mathutil`
  (unrecognised licence wording, the Makefile's known case) and keeps one
  file per module (it dropped `modernc.org/libc`'s `LICENSE` for
  `LICENSE-3RD-PARTY.md`); the script needs no tool beyond `go`.
  19 modules today.
- `--check`: regenerates into a temporary directory and `diff -r`; and
  checks that `THIRD-PARTY-NOTICES.md` names every direct requirement as
  "module `<path>`, pinned `<version>`" (doc-38's guard). Bites: a stale
  pin in the notices → exit 1 naming it; a changed file in `licenses/` →
  exit 1 with the diff.
- `THIRD-PARTY-NOTICES.md`: six direct modules at `go.mod`'s paths and
  versions; go-fabric's NOTICE content summarised and pointed at in the
  tree (verify's "this repository's notices file carries go-fabric's
  attribution"); the indirect list names go-humanize and bigfft and
  points at `licenses/MODULES.txt`.
- Both images: `COPY LICENSE THIRD-PARTY-NOTICES.md` and `licenses` to
  `/usr/share/doc/go-ha2fabric/`. Release archive: the Makefile lines
  below. CI: the security job runs `script/licenses/generate.sh --check`
  directly (no Makefile dependency); the build job and `release-on-tag`
  check every archive for `licenses/MODULES.txt`; the docker job compares
  `MODULES.txt` in both images with the tree (red until the Makefile
  lines land — intended, it forces the integration).
- The old `licenses/LICENSE-coder-websocket.txt` is replaced by
  `licenses/modules/github.com/coder/websocket/LICENSE.txt`.

### HR-P3-sup-48 (note) — partly; #72 left to the coordinator

Fixed: `addon/Dockerfile`'s `BUILD_VERSION` defaults to `VERSION` (a local
build no longer labels `0.1.0`); both images carry
`org.opencontainers.image.source` and `.licenses`. The undecided points
of #72 go to the issue, not to `docs/operations.md` (they are open
design, not operator facts). Comment for #72 (coordinator):

> Phase 3 of the review (HR-P3-sup-48) records what is still undecided
> here: (a) the user and UID and the tool the add-on drops to it with —
> `run.sh` is the `CMD` under `with-contenv bashio` with `init: false`,
> so the drop needs `su-exec` or `s6-setuidgid` from the HA base image;
> (b) the one-time `chown` of `/data` must cover every file the daemon
> keeps there (`ha2fabric*.db` and `-journal`, `selection*.json` with
> `.bak` and `.corrupt-*`, `nodes.json`, `passcode.json`) and keep their
> 0600 modes; (c) the AppArmor profile's content (UDP, mDNS multicast on
> the host network, `/data` only); (d) the watchdog waits for #71's
> `--health`; (e) the web UI now binds to the Supervisor's address of the
> add-on (decision 54), which the profile has to allow.

### HR-P3-sup-44, -sup-45, -doc-50 — fixed (settings listed)

`SECURITY.md` (private vulnerability reporting, the advisory URL, what to
include and never include, supported version, scope; no response-time
promise — the owner did not give one). `codeql.yml`: a matrix `go`
(autobuild) and `actions` (`build-mode: none`); the context name `codeql
(go)` is unchanged, `codeql (actions)` is new. `repository.yaml`:
`SukramJ <me@sukramj.dev>`.

### HR-P3-sup-49 (note) — docs line

README, Stand-alone: the `ghcr.io/sukramj/` images stay at 0.6.3 and get
no update; also in `SECURITY.md`. A description on the old packages
remains the owner's (no API call for a container package's description
that I could verify).

### HR-P3-doc-39, -40, -41, -42, -43 — fixed

- doc-39: `addon/DOCS.md` "Mapped today" and the README Features bullet
  from go-hamapping v0.8.0's rows (`rows_gen.go` row names: scene, script,
  automation, button as plugs; media players; vacuum; valve; water
  heater; select; event; camera; air-quality and electrical sensors; an
  air conditioner's fan modes), with a link to its `docs/rows.md` at
  v0.8.0; preset modes and areas stay "not mapped" (`rows/climate.yaml:23-24`).
- doc-40: status line (released; Apple Home pairing is the condition for
  cameras and the announcement), `make build` comment, the import rule
  (`logring`, `cluster/spec/`, `cluster/camera` for the camera host,
  `internal/plan` without `node`; from `internal/arch/arch_test.go:4-15`),
  a `NODE_LIMIT` row (default 8, 0 to 32: `internal/config/config.go:55-56`).
- doc-41: "no longer counts as activity for the eviction".
- doc-42: the six `[~]` items are `[x]`, their "Left:" lines kept; the
  legend says a "Left:" line is open without a brief.
- doc-43: `by_design.md` "for the configured node; a web UI node's rule is
  its selection alone"; `CONCEPT.md` status names 0.7.0.

### Citations (cite-60, -62..-67) — docs fixed; roadmap open

Checked with F1's `script/cite` (its commit `e258c5b`, run from F1's
worktree over this branch's files): `docs/operations.md`,
`docs/by_design.md`, `CONCEPT.md`, `README.md`, `addon/DOCS.md`,
`CLAUDE.md`, `THIRD-PARTY-NOTICES.md`, `SECURITY.md` — **0 findings**.

- cite-60: `operations.md` (`guides/operations.md:72, 76`, `camera.md:577-599`,
  `:133-140` at v0.26.0), `by_design.md` (`camera.md:577-599`,
  `chime.go:302-311` `portError`, `values.go:14-44` pinned v0.26.0),
  roadmap (`chime.go:302-311`, `provider_peer.go:51-63`, `camera_gen.go:34-45`
  pinned v0.26.0; `avstream.go:708-731` → `:871-888`, `videoStreamAllocate`,
  read at v0.26.0); README's matter.js NOTICE link at `blob/v0.26.0`.
- cite-62: `by_design.md` camera row at v0.8.0 (its caveat holds,
  `rows/camera.yaml:14-16`).
- cite-63: C's sentence with verify's ranges (`bridge-registry.ts:307-312,
  345-350, 446-451, 1040-1106`, `:314-325`; read at `664575a`).
- cite-64: symbol names (`keypadinputdef`/`levelcontroldef` imports,
  `levelcontroldef.LevelMax`, the `keypadinputdef` key-code switch;
  `command.go:18-19, 236-237, 304-312` at HEAD).
- cite-65: `operations.md` `exposed_entities.py:417-423`; roadmap
  climate `:511` added. The `docs/reports/H2.md` sites are left (history;
  F1's tool skips `docs/reports/` too).
- cite-66: survey citations of finished items pinned at go-hamapping
  v0.2.1 (and the camera blocker at go-fabric v0.15.4, the version of
  `0ee0fd2`'s go.mod, stated as past); open items at v0.8.0
  (`climate.yaml:159-180`, `binary_sensor.yaml:142`, `cover.yaml:9-143`);
  this repository's own drifting lines in the roadmap by symbol.
- cite-67: `CLAUDE.md` state line (#80, go-hamapping v0.8.0, go-hamqtt
  v0.41.0, go-ha-catalog v0.4.0); notices' catalog entry (doc-38). The
  `config.go:70` pin is F1's.
- **Open: `docs/roadmap.md` and F1's tool disagree.** `script/cite` over
  the roadmap reports 61 findings: decision 61's tool demands go.mod's
  version on every citation (also paths without lines), while the brief's
  cite-66 line keeps finished survey items at the survey's v0.2.1, and
  many "Done in go-hamapping v0.3.0" phrases are read as pins. I kept
  cite-66 and did not re-pin the history. Recommendation: the docs join
  `CITE_PATHS` as the six files above, and `docs/roadmap.md` joins the
  tool's history list (beside `docs/reports/`, the changelogs); the
  alternative is an explicit historical marker the tool honours.

### Decision 53 — docs

`docs/operations.md`, The web UI, a paragraph "Co-installed add-ons": the
check rests on `172.30.32.2` alone; nothing forwarded is a verifiable
secret (`supervisor/api/ingress.py:47-64` at `fe91b8db`); every add-on
shares the `hassio` network's layer 2 and keeps `NET_RAW` unless a
feature flag, off by default, drops it (`supervisor/docker/app.py:402-407`
at `fe91b8db`, its comment quoted; read with `gh api`); "the interception
of that address was observed on a real Home Assistant OS; a complete
forged exchange with the web UI was not run on a live system" (decision
53's label); the reach (pairing codes → a fabric, selection, erase,
nodes, log); decision 54's bind takes the listener off the LAN's
addresses and does not stop an add-on on the Supervisor network; what the
operator can do (trust co-installed add-ons as administrators; check the
Fabrics page and remove an unknown fabric). `addon/DOCS.md`, Access: two
sentences and a link. The held files were read only.

### Decision 54 — docs lines written here

F1's commit `4d2da0a` read for the shape: `docs/operations.md` Ports and
addresses (the bind to `GET /addons/self/info`'s `ip_address`, the
fallback with a warning) and a Logs row `ha2fabric.web_listen` (info,
warn). F1: check the wording against the code; I did not see F1's
report.

## Makefile lines for the coordinator

```make
RELEASE_PAYLOAD := README.md LICENSE SECURITY.md changelog.md THIRD-PARTY-NOTICES.md licenses
ADDON_ARCH   ?= amd64                      # replaces ADDON_BASE
# release: cp -R instead of cp (licenses is a directory)
	  cp -R $(RELEASE_PAYLOAD) "$$stage/"; \
addon-image: ## build the add-on image for ADDON_ARCH (addon/Dockerfile, its pinned base-<arch>)
	$(DOCKER) build --build-arg BUILD_ARCH=$(ADDON_ARCH) --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) -f addon/Dockerfile -t $(DOCKER_IMAGE)-addon:$(VERSION) .

licenses-check: ## fail when licenses/ or THIRD-PARTY-NOTICES.md is stale against go.mod (script/licenses)
	script/licenses/generate.sh --check

licenses-tree: ## regenerate licenses/ after a dependency change
	script/licenses/generate.sh

ci: build vet test lint cite licenses-check ## (F1's line plus licenses-check)
# .PHONY: add licenses-check licenses-tree
CITE_PATHS ?= cmd internal script testdata docs/operations.md docs/by_design.md CONCEPT.md README.md addon/DOCS.md CLAUDE.md
```

## Repository settings for the coordinator (not run)

`R=thingfabric/go-ha2fabric`. Endpoints from the REST API docs
(rulesets, environments, deployment branch policies, branch protection,
repository update, Actions permissions); go-fabric-ui's `d56-calls.sh` was
not consulted.

```sh
# 1. v* tags: no update, deletion or force-move for anyone (decision 58)
gh api -X POST repos/$R/rulesets --input - <<'EOF'
{"name":"release tags","target":"tag","enforcement":"active",
 "conditions":{"ref_name":{"include":["refs/tags/v*"],"exclude":[]}},
 "rules":[{"type":"update"},{"type":"deletion"},{"type":"non_fast_forward"}],
 "bypass_actors":[]}
EOF
# 2. v* tag creation: repository admins only (RepositoryRole 5 = admin)
gh api -X POST repos/$R/rulesets --input - <<'EOF'
{"name":"release tag creation","target":"tag","enforcement":"active",
 "conditions":{"ref_name":{"include":["refs/tags/v*"],"exclude":[]}},
 "rules":[{"type":"creation"}],
 "bypass_actors":[{"actor_id":5,"actor_type":"RepositoryRole","bypass_mode":"always"}]}
EOF
# 3. the ghcr environment: v* tags only
gh api -X PUT repos/$R/environments/ghcr --input - <<'EOF'
{"deployment_branch_policy":{"protected_branches":false,"custom_branch_policies":true}}
EOF
gh api -X POST repos/$R/environments/ghcr/deployment-branch-policies -f name='v*' -f type=tag
#    no reviewer (decision 66); no main policy (no dispatch, decision 65)
# 4. required checks bound to GitHub Actions (15368), plus coverage,
#    fuzz-smoke, docker (decision 57) and codeql (actions) (sup-45)
gh api -X PATCH repos/$R/branches/main/protection/required_status_checks --input - <<'EOF'
{"strict":true,"checks":[
 {"context":"build","app_id":15368},{"context":"lint","app_id":15368},
 {"context":"test (ubuntu-latest)","app_id":15368},
 {"context":"test (macos-latest)","app_id":15368},
 {"context":"test (windows-latest)","app_id":15368},
 {"context":"security (vuln + licenses + secrets)","app_id":15368},
 {"context":"coverage","app_id":15368},
 {"context":"fuzz-smoke","app_id":15368},
 {"context":"docker (both images, amd64)","app_id":15368},
 {"context":"codeql (go)","app_id":15368},
 {"context":"codeql (actions)","app_id":15368}]}
EOF
# 5. secret scanning and push protection (sup-44)
gh api -X PATCH repos/$R --input - <<'EOF'
{"security_and_analysis":{"secret_scanning":{"status":"enabled"},
 "secret_scanning_push_protection":{"status":"enabled"}}}
EOF
# 6. (sup-45, "consider") CodeQL alerts block a merge: high or higher
gh api -X POST repos/$R/rulesets --input - <<'EOF'
{"name":"code scanning","target":"branch","enforcement":"active",
 "conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"],"exclude":[]}},
 "rules":[{"type":"code_scanning","parameters":{"code_scanning_tools":[
   {"tool":"CodeQL","security_alerts_threshold":"high_or_higher","alerts_threshold":"errors"}]}}]}
EOF
# 7. (sup-33/-35, "consider") Actions must be pinned by SHA
gh api -X PUT repos/$R/actions/permissions -F enabled=true -f allowed_actions=all -F sha_pinning_required=true
```

Order: 4 after the pull request's checks report `codeql (actions)` once
(a required context that never reports blocks every merge). 3 before the
first tag after the merge (otherwise GitHub creates `ghcr` unprotected on
first use).

## Changelog lines

`changelog.md` `[Unreleased]`: Added (licence tree, attestations,
`SECURITY.md`), Changed (the add-on's bind — written from F1's commit
`4d2da0a`, F1's own line replaces it if different; digests and
`BUILD_ARCH`; the publishing gate), Fixed (docs). Not added: F1's other
rows (cfg-68 `label: str(1,32)`, test and guard rows are not
user-visible except cfg-68) — the coordinator merges F1's lines.
`addon/CHANGELOG.md` not touched (per-release file; doc-30 lines left).

## Follow-up after decisions 64–67 (on `review/p3`)

- **Decision 64, armv7 dropped from the add-on** (the decision names the
  add-on; the release archives and the plain image keep linux/arm/v7, they
  need no Home Assistant base): `addon/config.yaml` `arch`, the
  `base-armv7` stage of `addon/Dockerfile`, the `addon-image.yml` matrix;
  docs (`addon/DOCS.md` intro, `docs/operations.md` "Platforms",
  README Features); `changelog.md` `### Removed` and the digest line
  without armv7. Dependabot needed nothing (directories only).
- **Decision 65, no manual rebuild**: `workflow_dispatch` removed from
  `docker-build-push.yml` and `addon-image.yml` (no dispatch path is left;
  none is needed); the resolve step reads the tag only; the checkout is
  the pushed tag; the ancestry and version checks stay. Settings call 3:
  the `ghcr` environment's branch policy is `v*` tags only (the `main`
  policy is dropped with the dispatch).
- **Decision 66**: no reviewer; call 3's optional reviewer line removed.
- **Decision 67**: `docs/roadmap.md` is not in `CITE_PATHS` (ecf166e) and
  nothing else references it for the check.

Supersedes above: the sup-32 dispatch paragraph and its "Difference",
call 3's `main` policy and reviewer line, question 2 to 4.

## Questions (answered by decisions 64–67; kept as asked)

1. `docs/roadmap.md` against F1's `script/cite`: history list (my
   recommendation) or re-pin the history to go.mod's versions (against
   the brief's cite-66 line)?
2. armv7: Home Assistant publishes no armv7 base after `3.22-2025.11.1`;
   the add-on's armv7 image stays on that base. Keep armv7, or drop it
   from `addon/config.yaml` and the matrix (owner)?
3. Dispatch of the image workflows: keep (rebuild of a release tag from
   its own source, as built) or drop?
4. Required reviewer on the `ghcr` environment: yes or no (call 3)?
