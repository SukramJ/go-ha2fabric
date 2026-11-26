# Phase 3 — verification

Verifier: Opus in Fable's role (owner rule 2026-11-24). Baseline `0a47095`;
reports `phase-3-A.md`, `phase-3-B.md`, `phase-3-C.md` as committed in
`93fb100`. Probes in two `git archive 0a47095` copies under the session's
scratchpad (`p3/V/tree`, `p3/V/probe`), `PATH=$HOME/go/bin:$PATH`.
References at their pins: Supervisor `fe91b8db` (`gh api …?ref=`), go-fabric
v0.26.0 (module cache), matter-hub `664575a` (`git show`). Repository
settings read with `gh api`, read-only, 2026-11-26. M = measured (read or
run here), I = inferred.

Scope: every high and medium (none high; five medium), a sample of the lows
(A: test-02, api-04; B: doc-38, doc-41; C: cite-63, cite-64), the carried
cfg-68, and a wording check of the three reports against the held rows of
phases 0–2.

## Verdicts

| ID | Reported | Verdict | M/I |
| --- | --- | --- | --- |
| HR-P3-test-01 | medium | **confirmed** medium; fix corrected (floors from the lowest of several runs) | M |
| HR-P3-test-02 | low | **confirmed** low; the red test's comment corrected | M |
| HR-P3-api-04 | low | **confirmed** low | M |
| HR-P3-doc-30 | medium | **confirmed** medium, claim strengthened (the instruction cannot be followed without deleting `/data`) | M (slug, refusal, deletion); I (update in place) |
| HR-P3-sup-31 | medium | **confirmed** medium | M |
| HR-P3-sup-32 | medium | **confirmed** medium (the medium rests on the dispatch overwrite, not on the missing ruleset alone) | M |
| HR-P3-sup-33 | low | **re-graded note** (family consistency: go-fabric-ui's UR-P3-sup-64, the same setting, is a note; needs write-level access like sup-32) | M |
| HR-P3-sup-37 | medium | **confirmed** medium; fix extended (Go's own licence, go-fabric's `NOTICE`, the UI bundle's npm licences) | M |
| HR-P3-doc-38 | low | **confirmed** low, one sub-claim **struck** (humanize, bigfft) | M |
| HR-P3-doc-41 | low | **confirmed** low | M |
| HR-P3-cite-63 | low | **confirmed** low | M |
| HR-P3-cite-64 | low | **confirmed** low; ranges disagree with A's api-05 aside — use symbol names | M |
| HR-P3-cfg-68 | note | **confirmed** note | M |

Line-reference correction for unit B (no verdict change): `addon/Dockerfile`
has 44 lines at `0a47095`. B's `addon/Dockerfile:58` (sup-34) is `:34`
(`FROM ${BUILD_FROM}`), `:74-86` (sup-37) and `:75-86` (sup-48) are the
runtime stage `:33-44`, `:81` (sup-48, `ARG BUILD_VERSION=0.1.0`) is `:39`.

## Per finding

### HR-P3-test-01 — confirmed medium

- Re-read: `script/coverfloor/main.go:62-76` fails only `pct < floor` and a
  floor without statements; a package with statements and no floor prints
  `floor 0.0%` and passes. The header and `floors.txt:1-2`, `CLAUDE.md:49`
  call it a ratchet. Reference holds: gf:`script/coverfloor/main.go:73-81`
  (`staleFloorSlack = 3.0`), `:270-286` (the stale check, zero floor
  without slack) — M.
- Re-measured (`make cover` equivalent + `coverfloor`, exit 0): cmd 88.1,
  bridge **93.7**, ha **94.7**, version 90.9, web 96.2, text 100.0 (no
  floor), hatest 95.4, `script/coverfloor` 0.0. A's run had bridge 94.0 and
  ha 95.2: a run-to-run spread of 0.3–0.5 points — M.
- **Fix correction:** set each raised floor at the whole percent below the
  *lowest* of at least two runs (bridge 93, ha 94 by these two), as gf's
  comment at `:73-81` says; with a floor at the whole percent below one run
  the stale check is fine but the floor check itself can flake. The port is
  of the check logic only: go-fabric parses `go test -cover` summary lines,
  this tool a `-coverpkg` profile — keep the profile parser. The exemption
  list (`internal/ha/hatest`, `script/coverfloor`) is right; `script/coverfloor`
  at 0.0 with a `main_test.go` then needs a real floor.

### HR-P3-test-02 — confirmed low

- Re-read `internal/ha/client.go:226-262` (`Run`; reset at `:238-240`) and
  `client_p1_test.go:188-201`: with `MaxBackoff` 50 ms and a 1 s bound the
  test cannot tell a reset from no reset — M.
- Probe: A's E.1 test copied verbatim into `p3/V/probe/internal/ha`:
  `-count=10` green, delay after the long connection 11.6–13.8 ms; with
  `if false && up > c.opts.MaxBackoff` red ×3 (321.7–323.9 ms > 150 ms),
  while the existing `TestClientBackoffResetsAfterStableConnection` stays
  green ×3 — M.
- **Correction to the red test:** its comment "after five short ones the
  delay is at MaxBackoff" is wrong: five doublings from 10 ms leave the
  delay at 320 ms, not 400 ms (measured 322 ms). The bite is unaffected;
  say "near MaxBackoff (320 ms)" or let six short connections run first.

### HR-P3-api-04 — confirmed low

- `internal/arch/arch_test.go:63-65` (`nodeFree`) and `:127-129` are the
  whole purity rule. Mutations in `p3/V/probe`: `import _ "sync"`,
  `"os"`, `"time"` in `internal/plan`, and `func init() { go func() {}() }`,
  each leaves `go test -run TestImports ./internal/arch` green — M.
  `go list` of `internal/plan`: standard library `encoding/json fmt maps
  slices sort strings`, no `go` statement — M. Fix as proposed.

### HR-P3-doc-30 — confirmed medium, claim strengthened

Read at `sup:fe91b8db`:

- **Slug from the URL (M).** `supervisor/store/utils.py:12-15`
  `get_hash_from_repository` = `sha1(url.lower())[:8]`;
  `store/repository.py:64-68` takes it as the repository slug and the clone
  directory; `store/data.py:241` `app_slug = f"{repository}_{slug}"`.
  Computed: `https://github.com/SukramJ/go-ha2fabric` → `99ce00f9` (equals
  the owner's installed `99ce00f9_go-ha2fabric`, 2026-11-25),
  `https://github.com/thingfabric/go-ha2fabric` → `d1d9d6e4`. The add-on
  from the new address is `d1d9d6e4_go-ha2fabric`, a different add-on.
- **`/data` (M).** `apps/app.py:823-825` `path_data` =
  `path_apps_data / slug`; `install` creates it empty (`:1044-1050`);
  `uninstall` → `unload` removes it (`:1016-1019`, called at `:1110`).
- **The instruction cannot be followed as written (M, new).**
  `store/__init__.py:262-266`: removing a repository that an installed
  add-on comes from is refused ("It's used by installed apps"). The user
  who follows `addon/CHANGELOG.md:13-18` must uninstall the add-on first,
  which deletes its `/data` (store, `passcode.json`, `selection*.json`,
  `nodes.json`): every controller loses the bridge.
- **Update in place under the old URL (M for the store, I for the
  update).** `store/git.py:223-259` fetches the configured URL; git follows
  GitHub's redirect. The owner's box (2026-11-25) measured the store showing
  `version_latest` 0.7.0 for `99ce00f9_go-ha2fabric` through the redirect;
  the 0.7.0 manifest names `ghcr.io/thingfabric/go-ha2fabric-addon-{arch}`
  (`addon/config.yaml:23`), so an update pulls the new image under the old
  slug with its `/data` — I (the update itself not run). The redirect lasts
  as long as nothing new is created at `SukramJ/go-ha2fabric` (GitHub's
  rule; I).
- **Reach (M/I).** The store's changelog is `CHANGELOG.md` in the add-on's
  directory of the repository clone (`apps/model.py:718-720`,
  `store/data.py:264`), i.e. what the default branch holds after the next
  store reload — so every 0.6.3 installation reads this instruction right
  now, and a fix on `main` reaches them without a release (I).
- **Fix (as B, sharpened):** "Keep the repository you added; its old
  address redirects and the add-on updates in place with its pairing.
  Removing it means uninstalling the add-on, which deletes its data:
  controllers must pair again." Same in `changelog.md:39-46`. Guard: the
  owner's live update 0.6.3 → 0.7.0 under the old address, recorded.

### HR-P3-sup-31 — confirmed medium

- Read: `dependabot-auto-merge.yml:4-23` (any non-major, `--auto --squash`);
  `dependabot.yml:18-22` (one `go-minor-patch` group), `:8-9` cooldown 7
  days. `gh api …/branches/main/protection`: `required_pull_request_reviews`
  null, `strict: true`, 7 contexts without `coverage`, `fuzz-smoke`,
  `docker (both images, amd64)` (`ci.yml:98-122` and on) — M.
- `gh pr view 65`: author `app/dependabot`, merged by `app/github-actions`
  2026-11-21, review decision empty, files `go.mod`, `go.sum` — M. Also
  auto-merged: #75, #62, #25 (actions groups) — M.
- Severity stands: unreviewed code into `main`, which releases cut from,
  with three guards of `CLAUDE.md`'s set not gating it. The vanity-domain
  sentence is I and secondary; the 7-day cooldown narrows it (add to the
  row). Fix as proposed.

### HR-P3-sup-32 — confirmed medium

- `gh api …/rulesets` `[]`, environments 0, tags up to `v0.7.0` — M.
- `release-on-tag.yml:11-14, 45-49`: `v*` tags, version checks, no ancestry
  check — M. `addon-image.yml:13-22` `workflow_dispatch` with a free `tag`;
  `:62-74` the input sets `VERSION`; `:82-88` compares it with the
  *checked-out ref's* `addon/config.yaml`; `:120-121` pushes `:latest` and
  `:<VERSION>`. `docker-build-push.yml:12-15, 83-85` the same with no
  version check — M. A dispatch from any branch carrying `version: "0.7.0"`
  (every branch off `main` today) replaces the image the Supervisor pulls
  for 0.7.0.
- Grade: go-fabric-ui graded the ruleset and environment rows low/note
  (UR-P3-sup-61, -63); what makes this one medium is the overwrite of a
  released, Supervisor-pulled tag past every check — keep medium, and say
  so in the row. Fix as proposed; add "refuse an existing tag" (a registry
  read before the push) for the dispatch path if dispatch is kept.

### HR-P3-sup-33 — re-graded low → note

- M: all 7 contexts `app_id: null`; go-fabric-ui binds all to `15368`;
  `enforce_admins: false`; Actions `allowed_actions: all`,
  `sha_pinning_required: false`.
- Reason: the same setting is UR-P3-sup-64, a note, in go-fabric-ui; a
  forged status needs a token with status write on this repository, the
  same write-level party as sup-32. Fix unchanged (decision 56 shape).

### HR-P3-sup-37 — confirmed medium, fix extended

- M: `Makefile:19` payload `README.md LICENSE changelog.md
  THIRD-PARTY-NOTICES.md`; `licenses/` holds only
  `LICENSE-coder-websocket.txt`; `Dockerfile:33-42` and
  `addon/Dockerfile:33-44` copy only the binary (and `run.sh`).
- What the licences require for a binary distribution: BSD-3-Clause clause
  2 — reproduce the notice, the conditions and the disclaimer "in the
  documentation and/or other materials provided with the distribution";
  MIT and ISC — the notice and permission notice in all copies; Apache-2.0
  §4(a) a copy of the licence, §4(d) a readable copy of the attribution
  notices of a `NOTICE` the work carries. `THIRD-PARTY-NOTICES.md` names
  licences but reproduces none, so the archive fails BSD/MIT/ISC for every
  third-party module, the images ship nothing.
- **Extensions (M):** (1) the Go standard library and runtime (BSD-3-Clause,
  The Go Authors) are linked into every Go binary and are not a module
  `go version -m` or `go-licenses save` lists; (2) go-fabric has a `NOTICE`
  (gf:`NOTICE`) pointing at `licenses/NOTICE-matter.js.txt` — Apache §4(d)
  applies, B's remark is now M; (3) go-fabric-ui embeds a compiled bundle
  whose npm licences (`ui:licenses/npm/`, 8 files: svelte, tailwindcss,
  lucide-svelte, qrcode, …) travel in the binary and are not Go modules.
- **Fix:** the generated tree = `go-licenses save` output + `$(go env
  GOROOT)/LICENSE` + gf:`NOTICE` and `licenses/NOTICE-matter.js.txt` +
  ui:`licenses/npm/*`, into `RELEASE_PAYLOAD` and
  `/usr/share/doc/go-ha2fabric/` in both images; this repository's own
  `NOTICE` (or the notices file) carries go-fabric's attribution text. The
  guard compares the tree against `go version -m` plus the three fixed
  extras.

### HR-P3-doc-38 — confirmed low, one sub-claim struck

- M: `go.mod:5-12` has six direct requires (notices `:4` "five");
  go-hamapping `:32` `github.com/SukramJ/…` `v0.7.1` vs `sukramj.dev/…
  v0.8.0`; go-hamqtt `:53` `v0.40.0` vs `v0.41.0`; go-ha-catalog `:61-63`
  pseudo-version vs `v0.4.0`.
- **Struck:** "names neither dustin/go-humanize nor remyoudompheng/bigfft".
  `:80-81` delegates "the others go-fabric's notices list", and
  gf:`THIRD-PARTY-NOTICES.md:127, :131` list both. The delegation itself
  is sup-37's problem (the file is not shipped), not a stale entry.

### HR-P3-doc-41 — confirmed low

`changelog.md:276-280` ("keeps a session from its idle end") against
`:184-189` ("A session has no idle end of its own") in the same `0.7.0`
section, and `internal/camera/camera.go:251-255` — M. Fix as proposed.

### HR-P3-cite-63 — confirmed low

Read hub:`packages/backend/src/services/bridges/bridge-registry.ts` at
`664575a`: `:307-312` battery behind `autoBatteryMapping` or
`autoComposedDevices`; `:345-350` humidity and `:446-451` pressure on
unless set `false`; `:1070-1095` power and energy of `switch`/`light`
marked with no flag; `:314-325` `autoComposedDevices` builds real composed
devices — M. `docs/by_design.md:97-100` says all five are "behind
`autoComposedDevices`": wrong. The proposed sentence holds; its range
`:1040-1106` covers the three passes.

### HR-P3-cite-64 — confirmed low

`CONCEPT.md:104-107` cites `command.go:15-16, :208-209, :276-284`; at
`0a47095` these are the catalog/hamapping imports, a vacuum feature switch
and an on/off branch — M. The go-fabric knowledge is `:18-19` (imports),
`:236-237` (`levelcontroldef.LevelMax`), `:304-312` (the
`keypadinputdef` codes). Unit A's aside in api-05 gives `:234-237` and
`:293-309` (comment lines included); use the symbol names, as C proposes,
so the two reports' ranges do not have to agree.

### HR-P3-cfg-68 — confirmed note

`sup:supervisor/apps/options.py:38` (`str(min,max)` in the schema regex),
`:167-170` (`vol.All(str, vol.Length(**range_args))` — characters),
`:95-102` (unknown keys dropped with a warning); `apps/app.py:512-516`
(defaults merged with the user's options) — M. `addon/config.yaml:72-91`
has `label: str` — M. `str(1,32)` is valid schema and refuses an ASCII
label > 32 in the form; the daemon's byte and `text.Safe` check stays the
bound (a label of spaces passes `str(1,32)` and is refused by the daemon's
trim). Fix as proposed.

## Wording check against the held rows of phases 0–2

Held at this baseline: HR-P1-auth-01 (open), and the fixed held rows of
phase 0 (auth-20, auth-50, auth-51, word-52), published by this phase's
pull request (decision 53). Searched all three reports for the held row's
condition, party and path (co-installed add-on, the proxy's address,
spoofing, interception, the groups header, the peer address) and for the
held IDs:

- **No hint at HR-P1-auth-01** in A, B or C.
- **Hits naming fixed held phase-0 IDs with their area** (they are
  published in this pull request anyway, so no change is needed before it
  merges): `phase-3-A.md:153` ("web gate (P0 auth-20/50/51, …)") and
  `phase-3-A.md:168` ("Supervisor gate (P2 cfg-03; P0 auth-50/51)").
- `phase-3-B.md:397` names decision 54's gateway bind (HR-P0-cfg-27): a
  public low row with a tracked decision, not held — no hit.

## Totals after verification

| Unit | high | medium | low | note | total |
| --- | --- | --- | --- | --- | --- |
| A | 0 | 1 | 5 | 9 | 15 |
| B | 0 | 4 | 7 | 10 | 21 |
| C | 0 | 0 | 5 | 4 | 9 |
| **Phase 3** | **0** | **5** | **17** | **23** | **45** |

No new held item.

## Questions for the owner

1. **doc-30 now or with the phase pull request?** The store shows the
   default branch's `addon/CHANGELOG.md`, so every 0.6.3 installation reads
   "remove the repository" today, and doing so deletes the pairing.
   Recommendation: a small docs pull request now (the corrected 0.7.0
   entry and the `changelog.md` bullet), outside the bundled phase-3 pull
   request; then the owner's live update 0.6.3 → 0.7.0 under the old
   address, recorded.
2. **sup-33 as a note** (family consistency with go-fabric-ui's
   UR-P3-sup-64)? Recommendation: yes; it is fixed with sup-32's settings
   anyway.
3. **sup-37's scope**: the licence tree including Go's own licence,
   go-fabric's `NOTICE` (with matter.js's) and go-fabric-ui's npm licences?
   Recommendation: yes — without the three the archive and images stay
   short of what BSD-3-Clause, MIT and Apache-2.0 §4(d) require.

## Appendix — probes

```sh
V=<scratchpad>/p3/V
git -C /home/markus/Dokumente/GitHub/go-ha2fabric-review archive 0a47095 | tar -x -C $V/tree
git -C /home/markus/Dokumente/GitHub/go-ha2fabric-review archive 0a47095 | tar -x -C $V/probe
export PATH=$HOME/go/bin:$PATH

# test-01
cd $V/tree && CGO_ENABLED=0 go test -count=1 -timeout 300s -coverpkg=./... -coverprofile=coverage.out ./... \
  && go run ./script/coverfloor coverage.out          # exit 0; table above

# test-02: phase-3-A.md appendix E.1 as internal/ha/zz_verify_test.go (with a t.Logf of the delay)
cd $V/probe && go test -count=10 -run TestClientBackoffResetsAfterLongConnection -v ./internal/ha   # PASS ×10, 11.6–13.8 ms
sed -i 's/if up > c.opts.MaxBackoff {/if false \&\& up > c.opts.MaxBackoff {/' internal/ha/client.go
go test -count=3 -run 'TestClientBackoffResetsAfter(LongConnection|StableConnection)' -v ./internal/ha
# Long: FAIL ×3 (321.7–323.9 ms); Stable: PASS ×3

# api-04
for imp in sync os time; do printf 'package plan\n\nimport _ "%s"\n' $imp > internal/plan/zz_mut.go
  go test -count=1 -run TestImports ./internal/arch; done             # all green
printf 'package plan\n\nfunc init() { go func() {}() }\n' > internal/plan/zz_mut.go
go test -count=1 -run TestImports ./internal/arch                     # green

# doc-30
for p in supervisor/store/{utils,repository,data,__init__,git}.py supervisor/apps/{app,manager,model,options}.py supervisor/config.py; do
  gh api "repos/home-assistant/supervisor/contents/$p?ref=fe91b8db" -H 'Accept: application/vnd.github.raw' > "$(echo $p | tr / _)"; done
python3 -I -c "import hashlib;[print(u,hashlib.sha1(u.lower().encode()).hexdigest()[:8]) for u in
  ['https://github.com/SukramJ/go-ha2fabric','https://github.com/thingfabric/go-ha2fabric']]"
# 99ce00f9, d1d9d6e4

# sup-31/32/33 (read-only)
R=thingfabric/go-ha2fabric
gh api repos/$R/branches/main/protection; gh api repos/$R/rulesets; gh api repos/$R/environments
gh api repos/$R/actions/permissions; gh api repos/$R/tags
gh api repos/thingfabric/go-fabric-ui/branches/main/protection; gh api repos/thingfabric/go-fabric-ui/rulesets
gh pr view 65 -R $R --json mergedBy,reviewDecision,files,author,mergedAt
gh pr list -R $R --state merged --author app/dependabot

# cite-63
git -C /home/markus/Dokumente/GitHub/home-assistant-matter-hub show \
  664575a:packages/backend/src/services/bridges/bridge-registry.ts | sed -n '300,330p;340,352p;444,453p;1028,1110p'
```
