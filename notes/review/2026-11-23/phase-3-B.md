# Phase 3, unit B — docs and supply chain

Reviewer: Opus (unit B of `notes/briefs/2026-11-26-review-p3.md`).
Baseline `0a47095`; probes in a `git archive 0a47095` copy under the
session's scratchpad (`p3/B`), the history scan in a `git clone --no-local`
of the review worktree checked out at `0a47095` (`p3/Bgit`). Repository and
registry reads with `gh` (read-only, 2026-11-26). Line references are at
`0a47095` unless prefixed (`sup:` Supervisor `fe91b8db`, `gf:` go-fabric
v0.26.0, `hm:` go-hamapping v0.8.0, `ui:` go-fabric-ui v0.14.1, `fm:`
`dependabot/fetch-metadata` at the pinned `25dd0e34`). M = measured,
I = inferred.

No finding of this unit is an exploitable item for a party without write
access to the repository; `held/phase-3-B.md` is not written.

## 1. Behavioural claims in the docs

Method: every option, default, flag, variable, file name, port, log line,
warning code and number in the named files was checked against the code.
Two mechanical cross-checks cover the long lists: every backticked
`<word>.<word>` in `README.md`, `CONCEPT.md`, `docs/operations.md`,
`docs/by_design.md`, `addon/DOCS.md`, `CLAUDE.md` against every such string
literal in `internal/` and `cmd/` (missing: only Home Assistant service
names and go-fabric's `commissioning.own_window.open`, which is
`gf:node/commissioning.go:206`) — M; every documented status warning code
against the `Code:`/`Warning…` constants (all 14 present; the three code
strings not in the docs are camera error codes and a fake's) — M.

| Claim (file:line) | Code (file:line) | Verdict |
| --- | --- | --- |
| Defaults `LISTEN :5540`, `DISCRIMINATOR 3840`, `VENDOR_ID 0xFFF1`, `PRODUCT_ID 0x8001`, `LABEL Matter Bridge`, `WEB_LISTEN 127.0.0.1:8099`, `NODE_LIMIT 8` 0..32 (README.md:157-169, operations.md:110-117, 638-640) | config/config.go:45-56, 232-254, 366-370 | holds |
| `HA_URL` default in the add-on, no user/password, IPv6 in brackets (README.md:155, changelog.md:87-89) | config.go:275-281, 410-431 | holds |
| `HA_TOKEN` defaults to `SUPERVISOR_TOKEN` (README.md:156) | config.go:268-274 | holds |
| `STORE_PATH` `ha2fabric.db`, add-on `/data/ha2fabric.db`; `selection.json` beside it (README.md:163, operations.md:14-25) | config.go:57-61, 285-294; script/run.sh:30; Dockerfile:37 | holds |
| `LABEL` ≤ 32 bytes, no control/bidi characters (README.md:162, changelog.md:224) | config.go:334-342 | holds |
| `INCLUDE`/`EXCLUDE` comma- or newline-separated (README.md:85) | expose/expose.go:257-266 (also space and tab) | holds |
| `INCLUDE_LABELS` comma or newline (README.md:167, operations.md:319-320) | config.go:515-523 | holds |
| `WEB_ALLOW` IP addresses only, add-on Ingress proxy only (README.md:170, operations.md:128-131, 488-489) | config.go:296-301, 379-386 | holds |
| `WEB_ADMIN` off loopback without `WEB_ALLOW` refused, exit 2 (README.md:172, operations.md:479-481, changelog.md:69-71) | config.go:392-394; cmd/ha2fabric/main.go:76-79 | holds |
| `localhost` admitted by name, refused if it bound a non-loopback address (operations.md:482-483, changelog.md:224-226) | cmd/ha2fabric/main.go:301-304; internal/web/web.go:125 | holds |
| `WEB_ALLOWED_HOSTS` rules (README.md:171, operations.md:495-499) | config.go:473-487 | holds |
| Every setting has an env variable and a flag of the same name; the token has no flag (README.md:149-151) | config.go:222-256 (`-h` output, appendix) | holds |
| `passcode.json`, `nodes.json`, `ha2fabric-<key>.db`, `selection-<key>.json` (operations.md:14-19, addon/DOCS.md:111-117) | nodes and selection packages (log names cross-check) | holds |
| Store schema 7 → 12 at go-fabric v0.26.0 (operations.md:37-41, changelog.md:96-100) | gf:CHANGELOG.md:311 ("`store.Migrations()` has 12 steps") | holds (the "from 7" end: I) |
| Retries: connection 1 s doubling to 1 min, reset after > 1 min up; `auth_invalid` 5 min doubling to 1 h; ping 30 s (operations.md:217, 221-228) | internal/ha/client.go:52-59 | holds |
| Sync retry 5 s doubling to 5 min; `sync_failed` from the 2nd failure (operations.md:215, 281) | internal/bridge/bridge.go:242-243, 443, 475-478 | holds |
| Debounce 1 s, at most 5 debounces (operations.md:334-336, by_design.md:45, changelog.md:247-250) | bridge.go:239-240, 451, 464-465 | holds |
| 256 KiB state cap, 64 MiB read limit `67108864`, error text 512 bytes (operations.md:239-247) | ha/message.go:77, 167; ha/client.go:62 | holds |
| `bridge.queue_full`/`command_dropped` every 10 s (operations.md:282-283) | bridge/drops.go:13 | holds |
| Supervisor gate 2026.10.4, re-read 10 min, retry 30 s (operations.md:266-267, 463-471) | cmd/ha2fabric/supervisor.go:37, 43, 106 | holds |
| Camera: 8 sessions, 1 min idle for eviction, no idle end, 64/256 candidates, 1 KiB, 64000-byte still, 1 s reuse, End reasons 12 and 0, 10 s per signal (operations.md:424-451, 688) | internal/camera/camera.go:58-63, 110-149, 253, 299-300; ha/camera.go:87 | holds |
| Stop budget 25 s, erase grace 20 s, `timeout: 30` (operations.md:679-692) | main.go:64; nodes.go:92; addon/config.yaml:148 | holds |
| A new node's port is the first free above `LISTEN`'s, TCP too with `CAMERA` (operations.md:113, 606-609; addon/DOCS.md:92) | internal/nodes/nodes.go:359-385 | holds |
| UI error codes `host_not_allowed`, `origin_not_allowed`, `node_is_default`, `node_limit` (operations.md:499, 506, 629, 640) | present in ui: (`grep`, 2 hits each) | holds |
| `-journal` beside the store (operations.md:694) | gf:store/sqlite.go:29 (no WAL pragma), gf:node/store_path.go:18 | holds |
| `unknown` is unreachable (by_design.md:55-63, README.md:65-66) | internal/adapt/adapt.go:240 | holds |
| Volume rounded to two places (by_design.md:116-127) | internal/adapt/command.go:230-238 | holds |
| `internal/camera` the one importer of `cluster/camera` (CLAUDE.md layout) | `go list` (appendix): only `internal/camera` | holds |
| Logs to standard error in slog text form (operations.md:251) | not traced beyond `main.go` wiring | not checked |
| "Scenes, media players, areas and a climate's fan or preset modes are not mapped" (addon/DOCS.md:54-60) | hm:rows_gen.go — rows `scene/plug`, `media_player/video_player`, `/speaker*`, climate fan modes (`rows_gen.go:714`, `:1165`) | **wrong** → HR-P3-doc-39 |
| Migration: "remove [the old repository] and add the new one, then install the add-on from it" (addon/CHANGELOG.md:13-18) | sup:supervisor/store/utils.py:12-15, data.py:241 | **wrong in effect** → HR-P3-doc-30 |
| "an event the relay ignores no longer keeps a session from its idle end" (changelog.md:277-280) | camera.go:253 "A session has no idle end"; changelog.md:186 | **stale** → HR-P3-doc-41 |
| THIRD-PARTY-NOTICES.md pins and paths (:4, :32, :53, :61-63) | go.mod:5-12 | **stale** → HR-P3-doc-38 |
| `make build # … the daemon's main is a placeholder until the first release` (README.md:180) | Makefile:43-45; cmd/ha2fabric/main.go | **stale** → HR-P3-doc-40 |
| Status "a real pairing with Apple Home is the owner's manual step before the release is announced" (README.md:23-26) | 0.7.0 released 2026-11-25 (`gh release list`) | stale → HR-P3-doc-40 |
| `internal/arch` allows "node, device, contract, eligibility, the `<cluster>def` packages and nothing below it" (README.md:191-192) | internal/arch/arch_test.go:5-7, 37, 53-58 (also `logring`, `cluster/spec/…`, `cluster/camera` for the camera host) | incomplete → HR-P3-doc-40 |
| README's configuration table (README.md:153-172) | `NODE_LIMIT` has no row (it is in Features and operations.md) | gap → HR-P3-doc-40 |
| Roadmap `[~] in progress` on items that shipped (docs/roadmap.md:39, 50, 100, 193, 200, 206; legend :16-17; "Landed" :203) | hm:rows_gen.go rows `event/generic_switch`, `vacuum/robotic_vacuum_cleaner`, `valve/water_valve`, `water_heater/thermostat` | stale → HR-P3-doc-42 |
| "the options stay the single place for the rule" (by_design.md:110-114) | a web UI node's rule is its selection alone (operations.md:68-69, 613-615) | incomplete → HR-P3-doc-43 |
| CONCEPT.md status line "implemented in 0.1.0 … 0.2.0" (CONCEPT.md:3-4) | 0.7.0 | stale → HR-P3-doc-43 |
| CLAUDE.md "State: 0.7.0 (… go-hamapping v0.7.1)" | go.mod:8 `sukramj.dev/go-hamapping v0.8.0` (#80) | stale — a pin, unit C item 5; not re-reported here |
| CLAUDE.md "make ci … what the pipeline runs"; the local guard set = ci.yml's jobs | ci.yml:35-148 | holds (CI runs the parts of `make lint`, the race test on three OSes, `make release`, both images) |

The rest of `docs/operations.md` (read in full, 698 lines) and
`docs/by_design.md` hold where they state this daemon's behaviour; their
reference citations are unit C's.

## 2. Findings

### HR-P3-doc-30 — medium — the 0.7.0 add-on migration instruction installs a new add-on with empty data

- **Where:** `addon/CHANGELOG.md:13-18` ("If you added the repository
  under its old address, remove it and add the new one …, then install the
  add-on from it"); `changelog.md:39-46` (says the old URLs redirect, and
  calls the move breaking for anyone who added the repository by its old
  address).
- **Claim:** the Supervisor names an installed add-on
  `<sha1(repository URL)[:8]>_<slug>` and keeps its `/data` under that
  name (sup:`supervisor/store/utils.py:12-15` `get_hash_from_repository`,
  sup:`supervisor/store/repository.py:66`, sup:`supervisor/store/data.py:241`
  `app_slug = f"{repository}_{app[ATTR_SLUG]}"`) — M. The add-on installed
  from `https://github.com/thingfabric/go-ha2fabric` is therefore a
  different add-on from the one installed from the old address, and
  starts with an empty `/data`: no store (every controller loses the
  bridge, endpoint numbers restart at 2), no `selection.json`, no
  `nodes.json` (web UI nodes gone), a new random passcode — I (from the
  slug rule; not run on a live Supervisor). A backup of the old add-on
  restores under the old slug, not into the new one — I. The instruction
  does not say any of this, while the old address keeps working: the old
  GitHub URL redirects (changelog.md:40) and the manifest it serves already
  names the new image (`addon/config.yaml:125`), so an installation that
  keeps its repository updates in place with its data — I (GitHub's
  redirect of a git clone, not measured on a Supervisor).
- **Fix:** rewrite the 0.7.0 entry of `addon/CHANGELOG.md` (and the
  `changelog.md` bullet): keep the installed repository, it updates in
  place; switching to the new address is a new add-on that pairs from
  nothing — or state how to carry `/data` over. Owner question 1.
- **Guard:** none mechanical; the owner's live test of an update from
  0.6.3 under the old address, recorded.

### HR-P3-sup-31 — medium — Dependabot pull requests merge without review and without three of the CI jobs

- **Where:** `.github/workflows/dependabot-auto-merge.yml:4-20`;
  `.github/dependabot.yml:18-22, 39-41`; branch protection of `main`
  (`gh api repos/thingfabric/go-ha2fabric/branches/main/protection`).
- **Claim (M):** branch protection has no `required_pull_request_reviews`
  and requires 7 contexts: `build`, `lint`, `test (ubuntu-latest)`,
  `test (macos-latest)`, `test (windows-latest)`, `codeql (go)`,
  `security (vuln + licenses + secrets)`. **`coverage`, `fuzz-smoke` and
  `docker (both images, amd64)` are not required.** The workflow enables
  `gh pr merge --auto --squash` for every Dependabot update that is not
  `semver-major`. So a grouped `go-minor-patch` update — which for the
  `v0.x` direct dependencies (go-fabric, go-fabric-ui, go-hamapping,
  go-hamqtt, go-ha-catalog) is the breaking step — merges into `main`
  with nobody reading it, while the coverage floor, the fuzz targets or
  an image build is red. Measured on #65 (go-hamqtt 0.39.0 → 0.40.0,
  merged 2026-11-21 by `app/github-actions`, review decision empty,
  files `go.mod`, `go.sum` only): a direct-dependency bump without the
  `changelog.md` `### Changed` line and the notices update `CLAUDE.md`
  requires. A go-fabric-ui minor bump carries a new compiled web bundle
  into the binary unreviewed. With the modules under vanity domains
  (`thingfabric.dev`, `sukramj.dev`), whoever controls a domain can
  publish a new minor that this rule merges once CI is green — I (the
  checksum database pins only versions already seen).
- **Fix:** auto-merge `patch` only, and not for the five family modules
  (an `ignore`/separate group without auto-merge, or `update-type ==
  'version-update:semver-patch'` plus a dependency-name exclusion); add
  `coverage`, `fuzz-smoke` and `docker (both images, amd64)` to the
  required checks. Owner question 2.
- **Guard:** the required-check list; a CI step that fails a pull request
  touching `go.mod`'s direct requires without a `changelog.md` change
  (the rule `CLAUDE.md` states).

### HR-P3-sup-32 — medium — publishing has no gate: any tag, any branch, the released add-on tag overwritable

- **Where:** `.github/workflows/release-on-tag.yml:9-12, 24-46`;
  `docker-build-push.yml:8-17, 40-56, 81-84`; `addon-image.yml:12-21,
  60-88, 119-121`; repository rulesets (`[]`), environments (none).
- **Claim (M):** no ruleset protects `v*` tags (go-fabric-ui has
  "release tags": `refs/tags/v*`, update, deletion, non-fast-forward; its
  decision 56). `release-on-tag` checks the tag against
  `internal/version` and `addon/config.yaml` but not that the tagged
  commit is on `main`. `docker-build-push` and `addon-image` also run on
  `workflow_dispatch` from **any ref** with a free `tag` input, push
  `:<tag>` and `:latest`, and `docker-build-push` compares the tag with
  nothing; `addon-image` compares it with the *dispatched branch's*
  `addon/config.yaml`. A dispatch from an unmerged branch with `tag:
  0.7.0` therefore replaces `ghcr.io/thingfabric/go-ha2fabric-addon-<arch>:0.7.0`
  — the exact reference the Supervisor pulls (`addon/config.yaml:105-107,
  125`) — with that branch's code, past every required check. No
  environment with a required reviewer holds `packages: write`. Every
  path needs write access to the repository (no outside party), which is
  why this row is not held back.
- **Fix:** a `v*` tag ruleset (creation restricted to the owner, no
  update or deletion); `release-on-tag` and both image workflows check
  `git merge-base --is-ancestor "$GITHUB_SHA" origin/main`; drop
  `workflow_dispatch` from the image workflows or restrict it to `main`
  plus an existing tag that equals the manifest's version; an environment
  (`ghcr`) for the push jobs. Settings, not code: the owner's session
  applies them (as go-fabric-ui's decision 56). Owner question 3.
- **Guard:** the ruleset; the ancestry step in the three workflows.

### HR-P3-sup-33 — low — required checks are not bound to GitHub Actions

- **Where:** branch protection, `required_status_checks.checks[].app_id`
  is `null` for all 7 contexts — M.
- **Claim:** any integration or a commit status of the same name satisfies
  them; go-fabric-ui binds every context to app 15368 (decision 56, M:
  its protection read). `enforce_admins` is false (note: the owner can
  merge past the checks). `allowed_actions: all`, `sha_pinning_required:
  false` (repository Actions settings, M).
- **Fix:** bind each context to GitHub Actions (15368); consider
  `sha_pinning_required` once HR-P3-sup-35 is fixed.
- **Guard:** the setting.

### HR-P3-sup-34 — low — base images by tag, not digest; no Dependabot for them (carried HR-P1-sup-19 (d))

- **Where:** `Dockerfile:16` (`golang:1.27.4-alpine`), `Dockerfile:33`
  (`gcr.io/distroless/static-debian12:nonroot`), `addon/Dockerfile:58`;
  `addon-image.yml:46, 49, 52` and `ci.yml:146`, `Makefile:23`
  (`ghcr.io/home-assistant/<arch>-base:latest`); `.github/dependabot.yml`
  has no `docker` ecosystem — M.
- **Claim:** every image build takes whatever the tag points at; the
  add-on's runtime base is `:latest`, so two builds of one tag can differ
  in their Alpine and s6 layer, and a rebuild of a release is not
  reproducible. HR-P1-sup-19 handed (d) to this phase; it is still open.
- **Fix:** `FROM image:tag@sha256:…` in both Dockerfiles; the add-on
  matrix with a versioned HA base (`…-base:3.x@sha256:…`) through build
  args, the same in `ci.yml` and `Makefile`; a `docker` ecosystem (and the
  workflow files' `base:` via a `# renovate`-style comment, or a scripted
  bump) in Dependabot.
- **Guard:** a CI grep that refuses a `FROM`/`base:` without `@sha256:`.

### HR-P3-sup-35 — low — first-party actions pinned by tag in four workflows

- **Where:** `ci.yml:39-40, 62, 65, 88-89, 102-103, 114-115, 126-127,
  142`; `release-on-tag.yml:32, 41`; `docker-build-push.yml:38`;
  `addon-image.yml:55` (`actions/checkout@v7`, `actions/setup-go@v7.1.0`)
  — M. `codeql.yml:32-33` pins the same actions by SHA, and
  `.github/dependabot.yml:24-25` says the actions are pinned by SHA.
- **Claim:** the jobs that hold `contents: write` (release) and
  `packages: write` (images) run a mutable reference. Low: GitHub-owned
  actions.
- **Fix:** SHA plus `# vX.Y.Z` comment everywhere, as `codeql.yml`.
- **Guard:** `sha_pinning_required` in the Actions settings, or a CI grep
  for `uses: .*@v[0-9]`.

### HR-P3-sup-36 — low — no provenance, SBOM or signature on any published artefact

- **Where:** `docker-build-push.yml:78` and `addon-image.yml:112`
  (`provenance: false`, no `sbom:`), `release-on-tag.yml:56-70` (archives
  and an unsigned `SHA256SUMS`), no `attest-build-provenance`, no cosign —
  M.
- **Claim:** the add-on's comment (`addon-image.yml:109-111`, the
  Supervisor wants a plain image) does not apply to the distroless image
  or the release archives; a user cannot tell a release built by this
  workflow from one pushed by hand (see HR-P3-sup-32).
- **Fix:** `actions/attest-build-provenance` for the archives and both
  images (attestations live beside the image, not in its index, so the
  Supervisor still pulls a plain image — I), `provenance: mode=max` and
  `sbom: true` for the distroless image; `id-token: write`,
  `attestations: write` on those jobs only.
- **Guard:** `gh attestation verify` in a post-release check.

### HR-P3-sup-37 — medium — no published artefact carries the licence texts its dependencies require

- **Where:** `Makefile:19` (`RELEASE_PAYLOAD := README.md LICENSE
  changelog.md THIRD-PARTY-NOTICES.md`), `Dockerfile:33-42`,
  `addon/Dockerfile:74-86`; `licenses/` holds only
  `LICENSE-coder-websocket.txt` — M.
- **Claim:** the binary links 19 modules (`go version -m`, appendix):
  BSD-3-Clause (filippo.io/nistec, google/uuid, miekg/dns,
  remyoudompheng/bigfft, golang.org/x/net, x/sys, modernc.org/memory,
  modernc.org/sqlite, modernc.org/mathutil), MIT (apapsch/go-jsonmerge,
  dustin/go-humanize, go-hamqtt, go-ha-catalog, modernc.org/libc), ISC
  (coder/websocket), Apache-2.0 (oapi-codegen/runtime, the family) — M
  (`go-licenses report`). BSD-3-Clause requires a binary redistribution to
  reproduce the copyright notice, conditions and disclaimer in its
  documentation; MIT and ISC require the notice in copies. The release
  archive ships `THIRD-PARTY-NOTICES.md`, which names licences but
  reproduces none, and points at `licenses/…` and at go-fabric's notices,
  neither of which is in the archive; the two images ship no notice at
  all. Apache-2.0 §4(d): go-fabric's `NOTICE` (gf:`NOTICE`) is not carried
  either — I (not read in full).
- **Fix:** `go-licenses save ./cmd/ha2fabric --save_path=dist/licenses`
  (or a checked-in generated `licenses/` tree) into `RELEASE_PAYLOAD`, and
  `COPY` it to `/usr/share/doc/go-ha2fabric/` in both images.
- **Guard:** a `make licenses-check` that compares the saved tree with
  `go version -m`'s module list; `TestImageDefaults`-style check that the
  directory is in the image.

### HR-P3-doc-38 — low — THIRD-PARTY-NOTICES.md is stale against go.mod

- **Where:** `THIRD-PARTY-NOTICES.md:4` ("five Go modules directly"; go.mod
  has six), `:32` (go-hamapping `github.com/SukramJ/…` `v0.7.1`; go.mod:8
  `sukramj.dev/go-hamapping v0.8.0`), `:53` (go-hamqtt `v0.40.0`; go.mod:9
  `v0.41.0`), `:61-63` (go-ha-catalog at a pseudo-version; go.mod:7
  `v0.4.0`); `:77-81` names neither dustin/go-humanize (MIT) nor
  remyoudompheng/bigfft (BSD-3-Clause) — M.
- **Claim:** `CLAUDE.md` ("Every further dependency … updates the
  notices") is not enforced; #80 and #65 moved pins without it.
- **Fix:** update the file (module paths, pins, the two indirect
  modules); fold the indirect list into HR-P3-sup-37's generated tree.
- **Guard:** a test that reads `go.mod`'s direct requires and asserts each
  `module@version` string appears in `THIRD-PARTY-NOTICES.md`.

### HR-P3-doc-39 — low — the add-on's "Mapped today" says scenes, media players and climate fan modes are not mapped

- **Where:** `addon/DOCS.md:54-60`; `README.md:48-54` (incomplete, not
  wrong).
- **Claim (M):** go-hamapping v0.8.0 maps `scene` (hm:`rows_gen.go`
  `scene/plug`, also `docs/operations.md:339-354`), `media_player`
  (`media_player/video_player`, `/speaker`, `/speaker_mute`,
  `/speaker_volume`, hm:`rows_gen.go:3087-3501`) and a climate's fan modes
  (RoomAirConditioner, hm:`rows_gen.go:714`, `:1165`; changelog.md:129-130);
  the list omits vacuum, valve, water heater, select, button, event,
  camera, script and automation, all with rows. The page an add-on user
  reads first tells them these entities will not appear.
- **Fix:** rewrite the paragraph from go-hamapping's row list (and the
  README's Features bullet), or link go-hamapping's table instead of
  restating it.
- **Guard:** none; the go-hamapping bump checklist names this paragraph.

### HR-P3-doc-41 — low — the 0.7.0 release notes state a camera idle end that R7 removed

- **Where:** `changelog.md:277-280` ("an event the relay ignores no longer
  keeps a session from its idle end (HR-P2-bound-40)") against
  `changelog.md:184-189` ("A session has no idle end of its own") and
  `internal/camera/camera.go:253` — M.
- **Fix:** "… no longer counts as activity for the eviction"
  (`docs/operations.md:444-445` already says so). The GitHub release
  notes of v0.7.0 carry the sentence too (`script/extract-release-notes.sh`)
  — editing them is the owner's call.
- **Guard:** none.

### HR-P3-sup-44 — low — secret scanning and push protection are off; no security policy file

- **Where:** `gh api repos/thingfabric/go-ha2fabric`
  `security_and_analysis`: `secret_scanning`, `…_push_protection`,
  `…_non_provider_patterns`, `…_validity_checks` all `disabled` — M; no
  `SECURITY.md` in the repository or in `thingfabric/.github` — M;
  private vulnerability reporting is enabled — M.
- **Claim:** the daemon's users paste Home Assistant tokens into issues
  and logs; `make secrets` scans only the repository's history in CI, not
  issue text, and does not stop a push. Free for public repositories.
- **Fix:** enable secret scanning and push protection; a `SECURITY.md`
  (or an org-level one) pointing to private reporting.
- **Guard:** the setting.

### HR-P3-doc-40 — note — README leftovers

- `README.md:180` `make build` "the daemon's main is a placeholder until
  the first release" — the daemon has been real since 0.1.0.
- `README.md:23-26` the status still waits for "the release" — 0.7.0 is
  out; the Apple Home pairing is now the condition for `CAMERA` and the
  announcement only.
- `README.md:191-192` the import rule omits `logring`, `cluster/spec/…`
  and the camera host's `cluster/camera` (`internal/arch/arch_test.go:5-7,
  53-58`).
- `README.md:153-172` the configuration table has no `NODE_LIMIT` row.
- Fix: four edits. Guard: none.

### HR-P3-doc-42 — note — roadmap status markers behind the code

`docs/roadmap.md:39, 50, 100, 193, 200, 206` are `[~] in progress` while
`:203` says R5 landed in go-hamapping v0.3.0 / go-ha2fabric 0.4.0 and the
rows exist in hm:`rows_gen.go` (`event/generic_switch`,
`vacuum/robotic_vacuum_cleaner`, `valve/water_valve`,
`water_heater/thermostat`); the legend (`:16-17`) makes `[x]` "done
(release named)". Whether the air-quality and electrical items (`:39`,
`:50`) are complete was not checked row by row. Fix: mark with the
release, or state what remains. Guard: none.

### HR-P3-doc-43 — note — `by_design.md` and `CONCEPT.md` describe one node

`docs/by_design.md:110-114` "the options stay the single place for the
rule": true for the configured node only; a web UI node's rule is its
selection alone (`selection-<key>.json`, `docs/operations.md:68-69,
613-615`). `CONCEPT.md:3-4` status names 0.1.0 and 0.2.0. Fix: one
sentence each. Guard: none.

### HR-P3-sup-45 — note — CodeQL scans Go only and never blocks

`codeql.yml:38-42` `languages: go`; the repository's code-scanning setup
offers `actions` too (`gh api …/code-scanning/default-setup`: languages
`actions`, `go`) — M. The `actions` query pack finds the pattern of
HR-P3-sup-46. The required `codeql (go)` context passes with open alerts
(`codeql.yml:5-8`), so an alert blocks nothing. Fix: add `actions`;
consider a code-scanning merge protection rule for high alerts.

### HR-P3-sup-46 — note — `workflow_dispatch` input expanded into `run:`

`docker-build-push.yml:45` and `addon-image.yml:62`:
`INPUT_TAG="${{ github.event.inputs.tag }}"` is substituted into the shell
text before it runs — M. Only writers can dispatch, so it is a
consistency note; with HR-P3-sup-32's fix it goes anyway. Fix: pass the
input through `env:` and validate it against `^[0-9]+\.[0-9]+\.[0-9]+$`.

### HR-P3-sup-47 — note — the auto-merge gate reads `github.actor`; `reviewers:` in `dependabot.yml`

`dependabot-auto-merge.yml:12` gates on `github.actor`, which is whoever
triggered the event, not the pull request's author. Not exploitable here:
the pinned `fetch-metadata` refuses a pull request whose author or first
commit author is not `dependabot[bot]` or whose first commit is unsigned
(fm:`src/dependabot/verified_commits.ts:26-52`) and fails the step
(fm:`src/main.ts:47`), so the merge step is skipped — M (source read, not
run). Fix: also `github.event.pull_request.user.login == 'dependabot[bot]'`
in the job's `if`. `.github/dependabot.yml:11, 32` use `reviewers:`, which
GitHub retired in favour of `CODEOWNERS` in 2025 — I (not verified at a
pin); there is no `CODEOWNERS`, so Dependabot pull requests request no
reviewer.

### HR-P3-sup-48 — note — T25 and issue #72: what is still undecided

T25 (`threat-model.md:160`) stands as measured: `addon/Dockerfile:75-86`
has no `USER`, `addon/config.yaml` no `apparmor`, `watchdog` or
privilege key, `host_network: true` (`:139`) — M. Issue #72 (open) carries
the change; undecided in it: (a) the user and UID and how the daemon drops
to it — `run.sh` is the `CMD` under `#!/usr/bin/with-contenv bashio` with
`init: false` (`script/run.sh:1`, `addon/config.yaml:128`), so the drop
needs a tool in the HA base image (`su-exec`, `s6-setuidgid`) — not
chosen; (b) the one-time `chown` of `/data` must cover every file the
daemon keeps there (`ha2fabric*.db` and its `-journal`, `selection*.json`
with `.bak` and `.corrupt-*`, `nodes.json`, `passcode.json`) and keep the
0600 modes; (c) the AppArmor profile's content (UDP, mDNS multicast on the
host network, `/data` only); (d) the watchdog waits for #71's `--health`;
(e) whether decision 54's gateway bind (phase 3 fix) changes what the
profile must allow. Also `addon/Dockerfile:81` `ARG BUILD_VERSION=0.1.0`:
a local `make addon-image` labels the image `io.hass.version=0.1.0` —
note. Neither image sets `org.opencontainers.image.source`/`.licenses` —
note.

### HR-P3-sup-49 — note — the old ghcr packages stay at 0.6.3 without a notice

`ghcr.io/sukramj/go-ha2fabric-addon-{amd64,aarch64,armv7}` and
`ghcr.io/sukramj/go-ha2fabric` are public, newest `0.6.3,latest`
(2026-11-12/13) — M (`gh api users/SukramJ/packages/…/versions`). A
stand-alone user on `ghcr.io/sukramj/go-ha2fabric:latest` stops receiving
updates silently; changelog.md:44-45 says so, the registry page does not.
`docker-build-push` for v0.7.0 ran and pushed
`ghcr.io/thingfabric/go-ha2fabric:0.7.0,latest` (run of 2026-11-25,
`e78707a`, success; package version `sha256:fbab14a0…`) — M; the add-on
images likewise. Fix: a package description on the old packages pointing
to the new name, or delete them after a grace period (owner).

### HR-P3-doc-50 — note — `repository.yaml` maintainer address

`repository.yaml:6` gives the maintainer an address other than the
project identity address the owner set on 2026-11-26 (the git identity
rule) — M. The Supervisor shows it in the store. Fix: the project address.

## 3. Workflows (aspect summary)

| Workflow | Permissions | Pins | Notes |
| --- | --- | --- | --- |
| `ci.yml` | `contents: read` (workflow) | tags (sup-35) | jobs: lint, security, test ×3, coverage, fuzz-smoke, build, docker; 3 not required (sup-31) |
| `codeql.yml` | `contents: read`, `security-events: write` | SHA | Go only (sup-45) |
| `dependabot-auto-merge.yml` | `contents: write`, `pull-requests: write`; `pull_request_target`, no checkout of the head — M | SHA (no version comment) | sup-31, sup-47 |
| `release-on-tag.yml` | `contents: write` | tags for checkout/setup-go | no on-main check (sup-32); `cache: true` restores main's module cache into the release build — I, low impact (go.sum verifies modules) |
| `docker-build-push.yml` | `contents: read`, `packages: write` | docker/* SHA, checkout tag | dispatch from any ref, no version check (sup-32), injection note (sup-46), `cache-from: type=gha` |
| `addon-image.yml` | same | same | dispatch from any ref (sup-32), base `:latest` (sup-34) |

No `pull_request_target` workflow checks out pull request code — M.
`default_workflow_permissions: read`, `can_approve_pull_request_reviews:
false`, fork approval for first-time contributors — M.

## 4. Repository settings against go-fabric-ui's decision 56

| Decision 56 item | go-fabric-ui | go-ha2fabric | Row |
| --- | --- | --- | --- |
| Required checks bound to GitHub Actions | 11 contexts, app 15368 | 7 contexts, `app_id: null` | sup-33 |
| `v*` tag ruleset; release checks tag on `main` | ruleset "release tags" (update, deletion, non-fast-forward) | none; no check | sup-32 |
| Environment for the publish job | `npm` | none (`packages: write` unguarded) | sup-32 |
| CodeQL of every language required | `codeql (go)`, `codeql (javascript-typescript)` | `codeql (go)`; `actions` not analysed | sup-45 |
| `coverage` required | yes | no | sup-31 |

## 5. Counts

| Severity | Count | IDs |
| --- | --- | --- |
| high | 0 | |
| medium | 4 | doc-30, sup-31, sup-32, sup-37 |
| low | 8 | sup-33, sup-34, sup-35, sup-36, doc-38, doc-39, doc-41, sup-44 |
| note | 9 | doc-40, doc-42, doc-43, sup-45, sup-46, sup-47, sup-48, sup-49, doc-50 |

0 high · 4 medium · 8 low · 9 note. Unused IDs in 30–59: 51–59.

## 6. Not read, and why

- `docs/reports/*.md`: the brief names them only for unit C.
- `changelog.md` sections before 0.7.0 (lines 307-1015): they state past
  behaviour; the 0.7.0 section and `[Unreleased]` read in full.
- `docs/roadmap.md` R1–R5 bodies (lines 32-227): read by `grep` for
  status markers and the claims cited in doc-42, not line by line; they
  carry reference facts (unit C) and plans, not current behaviour.
- `addon/CHANGELOG.md` beyond its 0.7.0 entry (lines 31-224) and
  `addon/README.md` beyond the check of its image section: older entries.
- The logging setup behind "standard error in slog's text form"
  (`docs/operations.md:251`): not traced.
- go-fabric's `NOTICE` and `THIRD-PARTY-NOTICES.md` (sup-37's Apache §4(d)
  remark is I).
- Docker images were not built (brief: slow machine); the published
  manifests were not inspected beyond `gh` package versions.

## 7. Probes run

All in the scratch copy, `PATH=$HOME/go/bin:$PATH`:

| Probe | Result |
| --- | --- |
| `CGO_ENABLED=0 go build -trimpath -o bin/ha2fabric ./cmd/ha2fabric` | pass |
| `go version -m bin/ha2fabric` | 19 linked modules (appendix) |
| `go-licenses check ./... --disallowed_types=forbidden,restricted,reciprocal` (`make licenses`) | pass (mathutil "Unknown", as `Makefile:97-99` says) |
| `go-licenses report ./cmd/ha2fabric` | licences per module (sup-37) |
| `govulncheck ./...` (`make vuln`) and `govulncheck -mode binary` | pass, no vulnerabilities (DB 2026-11-26) |
| gitleaks v8.30.1 `git --redact` over the clone (`make secrets`) | pass, 149 commits, no leaks |
| `go mod tidy -diff`, `go mod verify` | pass |
| `bin/ha2fabric -h` | flags and environment names as the docs state |
| `go list -f '{{.ImportPath}} {{.Imports}}'` | only `internal/camera` imports `cluster/camera`; `internal/adapt` imports `cluster/spec/{keypadinput,levelcontrol}` (A2, not built) |

No red test was written: every finding here is a document, a setting or a
workflow; the guards are named per row.

## 8. Questions for the owner

1. **doc-30, the add-on's repository move.** Recommendation: tell users to
   *keep* the old repository address (it redirects and updates in place,
   with the pairing), and describe switching as a new installation that
   pairs from nothing; confirm on a Supervisor that the redirect path
   updates 0.6.3 → 0.7.0 before rewriting. Alternative: a documented
   `/data` copy between the two add-ons.
2. **sup-31, Dependabot auto-merge.** Recommendation: patch only, never
   the five family modules, and `coverage`, `fuzz-smoke`, `docker`
   required. Alternative: drop auto-merge entirely (the repository has one
   maintainer, Dependabot pull requests are few).
3. **sup-32/sup-33, repository settings** (as go-fabric-ui's decision 56).
   Recommendation: all of it — tag ruleset, on-`main` ancestry check in
   the three publishing workflows, `workflow_dispatch` removed from the
   image workflows, a `ghcr` environment, checks bound to 15368. Settings
   applied by the owner's session with confirmation.
4. **sup-37, licence texts in the artefacts.** Recommendation: a
   generated `licenses/` tree in the archive and both images in the fix
   wave; it touches `Makefile` and both Dockerfiles only.

## Appendix — probe text

```sh
S=<scratchpad>/p3/B
git -C /home/markus/Dokumente/GitHub/go-ha2fabric-review archive 0a47095 | tar -x -C $S
cd $S && export PATH=$HOME/go/bin:$PATH
CGO_ENABLED=0 go build -trimpath -o bin/ha2fabric ./cmd/ha2fabric
go version -m bin/ha2fabric | grep -E '^\s+dep'
GOROOT=$(go env GOROOT) go-licenses check ./... --disallowed_types=forbidden,restricted,reciprocal
GOROOT=$(go env GOROOT) go-licenses report ./cmd/ha2fabric
govulncheck ./... ; govulncheck -mode binary bin/ha2fabric
go mod tidy -diff && go mod verify
git clone -q --no-local /home/markus/Dokumente/GitHub/go-ha2fabric-review <scratchpad>/p3/Bgit
cd <scratchpad>/p3/Bgit && git checkout -q 0a47095
go run github.com/zricethezav/gitleaks/v8@v8.30.1 git --no-banner --redact .
```

Linked modules (`go version -m`): filippo.io/nistec v0.0.4,
apapsch/go-jsonmerge/v2 v2.0.0, coder/websocket v1.8.15,
dustin/go-humanize v1.1.0, google/uuid v1.6.0, miekg/dns v1.1.73,
oapi-codegen/runtime v1.9.0, remyoudompheng/bigfft 24d4a6f8daec,
golang.org/x/net v0.62.0, golang.org/x/sys v0.49.0, modernc.org/libc
v1.79.0, modernc.org/mathutil v1.7.1, modernc.org/memory v1.12.1,
modernc.org/sqlite v1.61.1, sukramj.dev/go-ha-catalog v0.4.0,
sukramj.dev/go-hamapping v0.8.0, sukramj.dev/go-hamqtt v0.41.0,
thingfabric.dev/go-fabric v0.26.0, thingfabric.dev/go-fabric-ui v0.14.1.

Settings reads (read-only):

```sh
R=thingfabric/go-ha2fabric
gh api repos/$R/rulesets
gh api repos/$R/branches/main/protection
gh api repos/$R/environments
gh api repos/$R --jq '{allow_auto_merge,security_and_analysis}'
gh api repos/$R/actions/permissions; gh api repos/$R/actions/permissions/workflow
gh api repos/$R/code-scanning/default-setup
gh run list -R $R -w docker-build-push.yml
gh api orgs/thingfabric/packages/container/go-ha2fabric/versions
gh api users/SukramJ/packages/container/go-ha2fabric-addon-amd64/versions
gh pr list -R $R --state merged --author app/dependabot
gh api repos/thingfabric/go-fabric-ui/rulesets/<id>; …/branches/main/protection
gh api 'repos/home-assistant/supervisor/contents/supervisor/store/utils.py?ref=fe91b8db' -H 'Accept: application/vnd.github.raw'
gh api 'repos/dependabot/fetch-metadata/contents/src/dependabot/verified_commits.ts?ref=25dd0e34f4fe68f24cc83900b1fe3fe149efef98' -H 'Accept: application/vnd.github.raw'
```
