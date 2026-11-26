# Phase 0 — pre-merge read of the fix wave (Fable)

Untracked (`held/`). Diff read: `git diff 13b4f53..HEAD -- . ':!notes'`
(20 commits, HEAD `1a4fe63`). Nothing was executed; no test was run.
M = read at the line named, I = inferred. Supervisor files via `gh api`
at `fe91b8db` (and `api/ingress.py` at `2026.10.3`, `2026.10.4`);
go-fabric-ui v0.13.0 and go-fabric v0.25.0 from the module cache.

Verdict: **ready to merge.** No high, no medium. Seven low/note rows;
50 and 53 are cheap and worth doing before the merge, 52 is an owner call.

## 1. HR-P0-auth-20 — the fix closes the hole as the held shape intends

| Check | Result | M/I |
| --- | --- | --- |
| Header check | `internal/web/web.go:167-182`: peer is the proxy **and** some `X-Remote-User-Groups` value, split on `,`, equals `system-admin`; absent → false. Duplicated header lines are all read (`Header.Values`); Go canonicalises the name, so letter case is no bypass; an underscore variant is a different header to Go | M |
| The Supervisor's side of the header at the gate's floor | `supervisor/api/ingress.py:47-64` filter compared lower-case (`:407`), then set once from `user.effective_group_ids` (`:412-419`), on both the request and the WebSocket path (`:242`, `:286`); identical at `2026.10.4` (diff against the pin: two lines, an exception type); absent at `2026.10.3` (grep) | M |
| Version gate | `cmd/ha2fabric/supervisor.go:86-123`: unknown at start, one-way to open or closed; unreadable → retry every 30 s, stays unknown; unparsable or older → closed. `admin` is `gate.open() && ingress(r)` per request (`main.go:236-243`); the state is an atomic that never goes back from closed/open, so there is no window between gate and handler in which an older Supervisor is admitted | M |
| Every admin path | One predicate: `fabricui` `handler.admin` (`handler.go:313`), `isAdmin` (`:777-779`), copied to every registry node's sibling handler (`:607`); registry routes and `/operation/log`, `/log/stream`, `/log/level` all pass `adminOnlyBecause` (`:396-400`, `:535-559`). `internal/web` adds no route of its own. `allowHost` still uses `web.From` — the Host check, not an admin decision | M |
| `panel_admin: true` | `addon/config.yaml:47` | M |
| The API call is allowed with this manifest | `supervisor/api/middleware/security.py:107-116` (`api_bypass` lists `/info`) and `:448-451` (an `App` principal on an `api_bypass` path returns before `access_hassio_api` is looked at) at `fe91b8db`; the token is taken from `Authorization` (`api/utils.py:109-110`); the answer's `supervisor` field `api/root.py:40-44`; route `api/__init__.py:498`. `homeassistant_api` plays no part. The code comment's ranges (`supervisor.go:22-25`) hold | M; that it works from the running add-on is I (owner's live test) |

No bypass found. Residuals are rows 50 and 51.

## 2. Findings

### HR-P0-auth-50 — low — the gate admits a pre-release of its floor

`cmd/ha2fabric/supervisor.go:187-211`; the test fixes it as intended
(`supervisor_test.go:21`, `"2026.10.4b1": true`). A suffix after the third
number is ignored, so `2026.10.4.dev…`/`2026.10.4b1` opens the gate. A
development build numbered for the next release can predate commit
`3fbb083b` and then forwards the browser's header (I: the Supervisor's dev
numbering was not read; dev channel only). **Fix:** a version with any
suffix is admitted only when its numbers are strictly above the floor;
equal with a suffix → closed. Change the test row.

### HR-P0-auth-51 — low — a closed gate never re-reads

`supervisor.go:91-103`: `run` returns after the first version read. The
population the gate closes for is exactly the one whose Supervisor then
updates; the add-on stays status-only until someone restarts it (the
warning says so, M). An unparsable version is also reported as "too old"
with the raw string. **Fix:** while closed, read again at a slow interval
(10 min) and open when the floor is reached; a version that does not parse
gets its own message. Not a security defect.

### HR-P0-word-52 — low, owner call — public text states the pre-fix condition

None of it is a recipe, but two facts are stated in tracked text:

- that the proxy address was no user check ("the proxy forwards every
  [Home Assistant] user's request, so its address alone decides nothing"):
  `cmd/ha2fabric/main.go:231-235`, `internal/web/web.go:139-141`,
  `docs/operations.md:347-348`;
- that before 2026.10.4 the header is the browser's to set ("drops a value
  the browser sent", "strips the browser's own value"):
  `internal/web/web.go:151-153`, `:163-166`,
  `cmd/ha2fabric/supervisor.go:28-32`, `docs/operations.md:345-346`, and
  the body of commit `2d71e5c` ("strips the browser's own").

`changelog.md:31-38`, `addon/CHANGELOG.md` and `addon/DOCS.md:131-139` are
neutral. The second fact harms nobody once the gate is in. The first
describes every released add-on on a Supervisor before 2026.10.2, and
decision 8 says no tag during the review: after the merge the description
is public while no released add-on carries the fix. **Proposal:** either
the owner accepts that (decision 13 reads that way) or the three
"forwards every user's request" sentences become "the proxy's address says
who sent the request, not who the user is" and the squash message drops
"strips the browser's own"; `held/*` is published with the 0.7.0 tag, not
with the merge.

### HR-P0-doc-53 — low — "an installation from before 0.7.0 kept 20202021" is true only if the options were ever saved

`supervisor/apps/app.py:512-516` at `fe91b8db`: an add-on's options are
the manifest's `options` merged with what the user stored (M). With the
default removed from the manifest, an installation whose options were
never saved has no `passcode` after the update: it draws a random one,
without the `passcode_test_value` warning. Whether the frontend's save
stores the full set is I. The claim is in `changelog.md:25-27`,
`addon/CHANGELOG.md` (the passcode bullet), `addon/DOCS.md` (step 5),
`docs/operations.md` (Pairing) and `script/run.sh:17-19`. Effect: a paired
bridge stays paired (go-fabric holds the passcode in memory only, for the
node's own window, `node/node.go:441-442` — I by grep, no stored verifier
compared); an unpaired bridge's codes change on update. **Fix:** wording —
"options saved before 0.7.0 keep 20202021 and get the warning; an
installation that never saved its options gets a random passcode with the
update; a paired bridge stays paired either way".

### HR-P0-cfg-54 — note — a passcode file that cannot be written ends the daemon, also for a paired node

`cmd/ha2fabric/main.go:101-108`, `internal/nodes/passcode.go:42-67`: a read
error other than not-exist, a failed move-aside or a failed write is exit
1 before any node starts. A paired node does not need the passcode to
serve. In practice the store sits in the same directory and would fail
too (I), so this is a note. **Option:** on a write failure run with the
drawn passcode for this start, log at error and show a warning.

### HR-P0-cfg-55 — note — cfg-22's refusal stops the whole bridge on update; the container case of `WEB_ALLOW`

`internal/config/config.go:383-385`: a stand-alone installation with
`WEB_ADMIN=true` on the image's `0.0.0.0:8099` and no `WEB_ALLOW` exits 2
after the update — the bridge, not only the UI. Decided (15) and in the
changelog; it deserves the changelog's breaking mark or first position.
With a published container port the peer the daemon sees can be the
bridge network's gateway, so a `WEB_ALLOW` naming it admits everyone (I,
Docker's behaviour not read): one sentence in `docs/operations.md`.

### HR-P0-ha-56 — note — the REST client still follows redirects; two leftovers of leak-25

- `internal/ha/client.go:140` with `internal/ha/camera.go:166-168`: the
  camera proxy's client keeps `net/http`'s redirect policy, which keeps
  `Authorization` for the same host or a subdomain on any port or scheme
  (I, Go's rule). Trusted endpoint by decision 11; the README's `HA_URL`
  row ("a redirect is not followed") is wider than the fix — say "the
  WebSocket handshake".
- `internal/ha/client.go:578-585`: `strings.ReplaceAll(msg, raw, …)` with
  an empty `url.Error.URL` would interleave the replacement between every
  character; guard `raw != ""`. Not reachable from `http.Client.Do` (I).
- `-h` still prints an environment `HA_URL` as the flag's default, before
  `checkHAURL` refuses userinfo (`config.go:227`, I). Same shape as
  leak-29; give `ha-url` no default in the usage text or leave it as a
  recorded note.

## 3. Read and found correct

- cfg-21: `passcode.json` is 0600 through `os.CreateTemp` and a rename
  (`nodes.go:474-501`); corrupt or invalid → moved aside, logged, redrawn
  (`passcode.go:47-60`); `ConfiguredPasscode` holds `r.mu` and calls only
  helpers that expect it held; `PASSCODE=0` is refused, not read as unset
  (`config.go:311-316`); `run.sh:20-22` exports only a set option, so
  bashio's `null` never reaches the daemon; an explicit 20202021 warns in
  the log and on the status page (`warnings.go:30-38`); `--dump-plan`
  returns before the passcode is needed (`main.go:84-86`).
- cfg-22/err-26: `config.go:383-385`, `web.go:68-74`; no remaining path
  where a dropped entry empties the filter.
- cfg-30: `config.go:290-294`, `:375-377`; the admin predicate and the
  filter now rest on the same constant.
- ha-23/31/leak-25: `config.go:165-176`, `:401-422`; `client.go:143-149`,
  `:206-210`; a refused redirect is a dial error inside the reconnect
  loop, not an exit.
- leak-24: `nodes.go:315-320`; unreadable fabrics (−1) log no codes; the
  ring's log routes are admin-only in go-fabric-ui (`handler.go:396-400`).
- leak-29: `config.go:233`, `:259-261`.
- No new exit path beyond rows 54 and 55.
