# Phase 0 — the dispositions

Branch `review/p0`; the pull request is squashed, its single commits stay
retrievable through the pull request's reference. Sources:
`phase-0-evidence.md` (IDs 01–06), `threat-model.md` (IDs 20–32),
`phase-0-verify.md`, the pre-merge read (IDs 50–56), `decisions.md` rows
13–22. Severity is the verifier's for every medium and for the sampled lows
(0 of 3 struck, 1 of 2 non-sampled changed: data-02 low → note); the rest
is the reviewer's.

**4 rows held back** (decisions 5 and 21): one medium and three lows of the
pre-merge read on its fix. Their reports live in the review worktree's
git-excluded `held/` directory and are published with a notes pull request
after the `0.7.0` tag. All four are fixed in this pull request.

| ID | sev | disposition | where |
| --- | --- | --- | --- |
| (held) HR-P0-auth-20 | medium | fixed `a5d1e6a`, `1c54f81` | decisions 13, 21 |
| HR-P0-cfg-21 | medium | fixed `d1a4c01`, `91d5800`, `66f713f` | decision 14: a random passcode kept in `passcode.json`; `20202021` warned |
| HR-P0-cfg-22 | medium | fixed `fff0d70`, `f63f400` | decision 15: refused at start, marked breaking in `changelog.md` |
| HR-P0-ha-23 | low (verifier: medium → low) | fixed `0c48215` | decision 17: log line and status warning `token_in_clear` |
| HR-P0-data-01 | low | fixed `c25de05` | clamp in float; `color_temp_kelvin` 15…1 000 000 (go-hamapping v0.6.0 range); red on amd64 and 386 |
| HR-P0-err-03 | low | fixed `21df7f6`, `0390575` | `HTTPBase` and `config.Load` require a host; the fuzz seed is the regression |
| HR-P0-leak-24 | low | fixed `3afcb0f` | decision 16 |
| HR-P0-err-26 | low | fixed `fff0d70` | an unparsable `WEB_ALLOW` entry fails `web.New` (go-fabric-ui HR-7) |
| HR-P0-cfg-27 | low | open → after the owner's live Ingress test | decision 22 |
| HR-P0-bound-28 | low | fixed `2bf228e` | 64 queued candidates per session before Home Assistant's session; the session count per camera → phase 1 S |
| HR-P0-test-06 | low | fixed `667bffc`, `3c78010` | tests over existing branches; no red step (gaps, not defects) |
| (held) HR-P0-auth-50 | low | fixed `fcadb9a` | decision 21 |
| (held) HR-P0-auth-51 | low | fixed `fcadb9a` | decision 21 |
| (held) HR-P0-word-52 | low | fixed `10aee99` | decision 21 |
| HR-P0-doc-53 | low | fixed `66f713f` | the passcode after the update: saved options keep `20202021` with the warning, unsaved ones draw a random passcode |
| HR-P0-data-02 | note (verifier: low → note) | fixed `abc79b2` | `nodes.Slug` holds 56; the fuzz seeds are the regression |
| HR-P0-doc-05 | note | fixed `fe7438c`, `95abe84` | `README.md`, `ci.yml` header |
| HR-P0-leak-25 | note | fixed `0390575` | userinfo refused, the URL logged without query (`ha.LogURL`) |
| HR-P0-leak-29 | note | fixed `6b8f9b5`, `0f922f3` | `-h` prints neither `PASSCODE` nor `HA_URL` |
| HR-P0-cfg-30 | note | fixed `1c54f81` | in the add-on `WEB_ALLOW` is the Ingress proxy and takes no other value |
| HR-P0-ha-31 | note | fixed `3614fed`, `0f922f3` | neither the WebSocket nor the camera proxy's client follows a redirect |
| HR-P0-cite-32 | note | fixed `0f5765a` | the plan's line ranges at the baseline |
| HR-P0-cfg-54 | note | fixed `91d5800` | a passcode file that cannot be kept: status warning `passcode_not_kept`, no exit |
| HR-P0-cfg-55 | note | fixed `f63f400` | breaking mark; `WEB_ALLOW` behind a container gateway in `docs/operations.md` |
| HR-P0-ha-56 | note | fixed `0f922f3` | the camera proxy's redirects; `scrubURL` on an empty URL |
| HR-P0-test-04 | medium | struck | the verifier: the red `fuzz-smoke` was the new targets finding data-02 and err-03 on an unpushed commit; resolved by landing targets, fixes and seeds together (decision 20) |

Carried in from go-fabric-ui's review (its `threat-model.md`, section 4):
HR-1 closed by #66; HR-2 → HR-P0-cfg-22; HR-4 → HR-P0-leak-24; HR-5 →
HR-P0-cfg-22 (docs: plain HTTP); HR-7 → HR-P0-err-26.

Standing guards added: twelve fuzz targets in `make fuzz-smoke`
(`dfb949e`); every fix above has its regression test (red seen, or stated
otherwise in the row).

Guards at the pull request's head: see the pull request body.
