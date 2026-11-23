# Phase 0 — verification

Verifier's re-read of the phase 0 findings at `dfb949e` (baseline `10ee7b4`
plus notes and the fuzz targets). Pins as in `threat-model.md`; `sup:` is
home-assistant/supervisor read with `gh api` at the ref named, `core:`
`git show 2026.10.3:`, `gf:` go-fabric `v0.25.0`, `ui:` go-fabric-ui
`v0.13.0` (module cache). **M** = measured (line read, or the command
run), **I** = inferred. Executed: the three crash seeds in a `git archive`
export of HEAD under the session's scratchpad, and a `clampInt` probe on
amd64 and 386; nothing else. The worktree was not changed (`git status`
before and after: the same three untracked files).

**Held back: 1 row**, `HR-P0-auth-20`, in `held/phase-0-verify.md`.

## Verdicts

| ID | Author | Verdict | Severity |
| --- | --- | --- | --- |
| HR-P0-auth-20 | medium | confirmed, with corrections (held) | medium |
| HR-P0-cfg-21 | medium | confirmed | medium |
| HR-P0-cfg-22 | medium | confirmed | medium (the image; low for the binary) |
| HR-P0-ha-23 | medium | confirmed, severity changed | **low** |
| HR-P0-leak-24 (sample) | low | confirmed | low |
| HR-P0-bound-28 (sample) | low | confirmed; the inferred link read | low |
| HR-P0-data-01 (sample) | low | confirmed (probe) | low |
| HR-P0-data-02 | low | confirmed (seeds replayed), severity changed | **note** |
| HR-P0-err-03 | low | confirmed (seed replayed) | low |
| HR-P0-test-04 | medium | **struck** as a finding; kept as a process condition | — |

## HR-P0-cfg-21 — confirmed, medium

- M: the default is `20202021` (`internal/config/config.go:43`, `:191`),
  and the add-on ships it as the option's value (`addon/config.yaml:50`),
  so every add-on installation starts with it unless the user edits it.
- M: it passes the check (`config.go:260-266`; `gf:node/facade.go:149-154`,
  `gf:node/config.go:939`, `gf:secure/setup/setup.go:364-374`: range and
  the trivial list only).
- M: the node gets it as `Commissioning.Passcode`
  (`internal/config/node.go:47-52`); `WindowOnDemand` is not set anywhere
  (grep over non-test Go: none), and its zero value opens the window at
  start without a fabric and again after the last fabric goes
  (`gf:node/config.go:523-535`).
- M: no refusal, no warning, no log line names the default.
- Refutation tried: the docs say "Change it" (`README.md:107`, `:148`,
  `addon/DOCS.md:22`, `:62`, `docs/operations.md` "The defaults are public
  test values"). That is a request to the user, not a control.
- I (why medium holds): whoever commissions first can, as a fabric
  administrator, open an enhanced window with a verifier of their own
  choosing — the same public value — so the owner's later pairing succeeds
  and nothing but the fabrics page shows the second fabric.

Fix shape (recommendation): **a random passcode on first start**, kept
beside the store, when `PASSCODE` is unset — the registry nodes already
work that way (`internal/nodes/nodes.go:376-391`). The add-on option
becomes optional (`passcode: int(1,99999998)?`, no default in
`addon/config.yaml`). An explicit `PASSCODE=20202021` is honoured with a
`Warning` on the status page and a log line (the value is then a
choice). Refusing the default outright breaks every existing add-on
installation that never changed the option at its next start; a warning
alone leaves the first-start window open. An already commissioned node is
unaffected by the changed passcode (it serves only the node's own window).
Contract: yes (the manifest option, the documented default).

## HR-P0-cfg-22 — confirmed, medium

- M: `WEB_ADMIN` selects token mode (`cmd/ha2fabric/main.go:194-203`), and
  in token mode the status answer hands the token to every caller
  (`ui:fabricui/handlers_node.go:151`, `CSRFToken: h.token`, outside the
  admin branch; the check is `ui:fabricui/handler.go:776-782`).
- M: without `WEB_ALLOW` there is no client filter
  (`internal/web/web.go:86-89`); `config.Load` checks `WEB_LISTEN` for
  form only (`config.go:312-316`) and never relates it to `WEB_ADMIN`
  (`:294-298`).
- M: the image's default listener is `0.0.0.0:8099` (`Dockerfile:31`) and
  its documented run is `--network host` (`Dockerfile:6`,
  `README.md:119-123`), so on the image the one switch `WEB_ADMIN=true`
  makes every LAN client the administrator.
- Refutation tried: it is documented (`docs/operations.md:316-319`,
  "anyone who can reach the port is the administrator — keep it on a
  trusted network"), and the Host check does not help (IP-address hosts
  pass, `docs/operations.md:322-323`). The documentation lowers it for
  the binary, whose default listener is loopback (`config.go:48`): there
  it takes two deliberate settings, low. For the image it is one setting
  whose reach the `WEB_ADMIN` row of the README (`README.md:161`) does not
  state: medium holds.

Fix shape: `config.Load` **refuses** `WEB_ADMIN=true` when `WEB_LISTEN` is
not a loopback address and `WEB_ALLOW` is empty (a validation issue, exit
2, naming both remedies). No separate override switch: `WEB_ALLOW` is the
explicit statement. The image keeps `0.0.0.0:8099` (status page only by
default); the README's `WEB_ADMIN` row and `docs/operations.md` say that
the page is plain HTTP and that an address filter is not authentication.
`web.New` failing on an unparsable `Allow` entry (HR-P0-err-26) belongs in
the same change. Contract: yes (a configuration that starts today is
refused).

## HR-P0-ha-23 — confirmed, severity medium → low

- M: the token goes in the WebSocket `auth` message
  (`internal/ha/client.go:269-273`) and as a Bearer header
  (`internal/ha/camera.go:151-153`) to whatever `HA_URL` names; the check
  is the scheme prefix (`config.go:251-253`); `ws` maps to `http`
  (`camera.go:100-103`); the examples are `ws://homeassistant.local:8123`
  (`config.go:188`, `Dockerfile:7`, `README.md:121`).
- I: a LAN peer reading or answering for that name (not run).
- Why low: the code does what it is configured to do and `wss://` works;
  Home Assistant serves plain HTTP on 8123 by default, so for most
  stand-alone installations there is no encrypted endpoint to prefer, and
  every browser and app on that LAN already sends its Home Assistant
  credentials over the same hop. What is missing is a statement and a
  warning, not a control the daemon could enforce. The asset (an
  administrator's token) is the reason it is a finding at all. In the
  add-on the hop is the Supervisor's internal name (`config.go:33`).

Fix shape: a start-up **warning** (log, and a status-page `Warning`) when
the scheme is `ws` and the host is neither loopback nor `supervisor`; a
paragraph on the token's hop in `docs/operations.md`; the README's
example with a note that `wss://` is to be used where Home Assistant has
TLS. No refusal. `HR-P0-ha-31` (redirects) is the part with a real
control to add and should be fixed with it.

## HR-P0-leak-24 — confirmed, low (sample)

M: `logPairing` runs at every node start, with or without fabrics
(`cmd/ha2fabric/nodes.go:106-107`), and logs the manual code and the QR
payload at info (`:306-315`); the logger is the ring in front of stderr
(`internal/web/operation.go:19-22`); the ring redacts nothing
(`gf:logring/logring.go:37-39`); the port's contract says nothing it
returns carries a passcode (`ui:fabricui/operations.go:25-26`,
`ui:docs/embedding.md:320-321`). The log routes are admin-only, who can
read the pairing page anyway; the exposure is the add-on log and pasted
logs. Low holds.

## HR-P0-bound-28 — confirmed, low (sample)

- M: `pending` grows by every `ProvideIceCandidates` call while `haID` is
  empty, without a cap (`internal/camera/camera.go:177`, `:378-382`).
- M (the author's inferred link, now read): go-fabric's kit caps sessions
  only by the id space — `MaxSessionID` 65534
  (`gf:cluster/camera/provider.go:38-40`), `generateSessionID`
  `RESOURCE_EXHAUSTED` when every id is taken (`:207-224`); it passes the
  candidates through uncounted (`:603-617`). So neither the kit nor this
  package bounds the sessions of one camera below 65535, each a Home
  Assistant subscription (`camera.go:322`).
- The caller is a commissioned controller (decision 11). Low holds; the
  session half deserves its own row in phase 1 S.

## HR-P0-data-01 — confirmed, low (sample)

M (probe, go1.27.4): `clampInt(1e300, 0, 255)` is 0 on amd64 and 386;
`clampInt(3e9, 0, 255)` is 255 on amd64 and 0 on 386
(`internal/adapt/command.go:389-392`). `integer` refuses NaN and
infinities first (`:376-386`), so only finite out-of-range values reach
it; `:330` is unclamped. Reachability from a controller stays I.

## HR-P0-data-02 — confirmed, low → note

M: both seeds replayed and fail (`Slug` returns 57 bytes;
`internal/nodes/nodes.go:249-258`: at 55 bytes a pending dash and a
character are both written before the bound is tested). M: `key` adds at
most `-999` (`:285-288`), 61 ≤ `maxKey` 64 (`:59`), and `validKey` checks
every key (`:290`). No behaviour follows from it; the comment's bound
(`:241`) and the fuzz target's 56 are what disagree with the code. Note.
Fix: test the bound before writing (or state 57) — one line, the seeds as
the regression.

## HR-P0-err-03 — confirmed, low

M: the seed replayed and fails (`HTTPBase("ws://::/websocket")` =
`http://::/api`, which `url.Parse` refuses;
`internal/ha/camera.go:95-117`). M: `config.Load` takes the URL by prefix
(`config.go:251-253`). An operator's own typing error, found at dial
instead of at start. Fix: `config.Load` calls `ha.HTTPBase` (or parses the
URL and requires a host) so that the URL is refused with exit 2; `HTTPBase`
re-parses its result.

## HR-P0-test-04 — struck as a finding

At the baseline `make fuzz-smoke` is green (evidence table, exit 0). The
red run is the new targets finding HR-P0-data-02 and HR-P0-err-03, on a
commit that is not pushed; decision 10 says a hit becomes a finding with
its seed as the regression, which those two rows are. Nothing remains
once they are fixed, so it is not a defect of the code or of the guard and
carries no severity. What stays is a **condition on the fix wave**: the
targets (`dfb949e`), the two fixes and the three seeds (moved from
`held/phase-0-fuzz-seeds/` into the packages' `testdata/fuzz/`) land in
one pull request, and `review/p0` is not pushed before. M: CI runs
`make fuzz-smoke FUZZTIME=20000x` (`.github/workflows/ci.yml:119`); that
it would be red is I (not run there).

One observation of its own, note: the loop is `set -e`
(`Makefile:86-90`), so the first failing target hides the state of those
after it. Acceptable for a smoke; no change proposed.
