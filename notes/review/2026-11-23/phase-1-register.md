# Phase 1 — the dispositions

Branch `review/p1`; the pull request is squashed, its single commits stay
retrievable through the pull request's reference. Sources: `phase-1-S.md`
(IDs 01–22), `phase-1-GA.md` (30–39), `phase-1-GB.md` (60–72),
`phase-1-verify.md`, the pre-merge reads (`phase-1-premerge.md`, IDs
90–102), the fix reports `phase-1-fix-F1.md`, `phase-1-fix-F2.md`,
`phase-1-fix-round2.md`, `decisions.md` rows 23–34. Severity is the
verifier's for every medium, every merge and the sampled lows (0 struck,
1 medium → low, 2 low → note, 3 merged), and the pre-merge reader's for
90–102; the rest is the reviewer's.

**1 row held back** (decisions 5 and 24): one medium, without a fix in
this repository. Its reports live in the review worktree's git-excluded
`held/` directory and are published once it is measured (X1 in phase 2 or
the owner's live Ingress test) and tagged.

Totals after the merges: 0 high · 4 medium (one held; two of them found
by the pre-merge read in the fix wave and fixed before the merge) ·
23 low · 27 note.

| ID | sev | disposition | where |
| --- | --- | --- | --- |
| (held) HR-P1-auth-01 | medium (I) | open, held | decision 24; measured by X1 or the live test |
| HR-P1-adpt-30 | medium | fixed `108fe3e` (stop-gap); upstream go-hamapping #39 | decisions 26, 32 |
| HR-P1-data-90 | medium | fixed `a0a1970`, `0a54177` | pre-merge read: a skipped registry entry was exposed; now not exposed, a skipped label entry fails the sync while labels are configured |
| HR-P1-err-91 | medium | fixed `7d12be9` | pre-merge read: the restore from `selection.json.bak` is written back |
| HR-P1-bound-02 | low | fixed `3dc88b0` | decision 23: 8 sessions per camera, the idlest ≥ 1 min evicted, else RESOURCE_EXHAUSTED |
| HR-P1-conc-03 | low | fixed `3dc88b0` | decision 31: idle end after 1 h; go-fabric signal → follow-up issue (decision 34) |
| HR-P1-bound-17 | low | fixed `3dc88b0` | 64 candidates per call, 256 per session; one still request per camera, reused 1 s |
| HR-P1-err-05 | low | fixed `3dc88b0` | a failed re-offer sends `End` |
| HR-P1-err-06 | low | fixed `080c027`, `122a8ad`, `7d12be9` | decision 25 (with HR-P1-cfg-35) |
| HR-P1-ha-07 | low | fixed `6c5ae3c` | back-off resets only after a connection up longer than `MaxBackoff` |
| HR-P1-ha-08 | low | fixed `6c5ae3c`, `5f8474b` | decision 28: 5 min doubling to 1 h, `ha_token_refused`, no exit |
| HR-P1-data-09 | low | fixed `6c5ae3c`, `a0a1970` | lists decode entry by entry; see data-90 |
| HR-P1-cfg-10 | low | fixed `259490a` | secret files tightened to 0600 when read wider |
| HR-P1-cfg-12 | low | fixed `9928fba` | decision 27, breaking for the container image |
| HR-P1-test-16 | low | fixed `3783e6e` | every `x-admin` route × client kind at the daemon's wiring |
| HR-P1-sup-18 | low | open → follow-up issue after the live Ingress test | decision 34 |
| HR-P1-err-31 | low | fixed `6d1d798`, `c929f2b` | the next sync retries; `sync_error` |
| HR-P1-conc-32 | low | fixed `1a21e81`, `4acd709`, `f4e79bb` | with HR-P1-conc-60; an owed sync instead of dropped work |
| HR-P1-cfg-34 | low | fixed `296fa92` | decision 33: label ids in memory |
| HR-P1-test-38 | low | fixed `7c2ddeb` | the plan side's negative cases; `TestAssessIgnoresInputOrder` standing |
| HR-P1-ha-62 | low | fixed `6c5ae3c` | with HR-P1-ha-04: a late result is unsubscribed |
| HR-P1-err-64 | low | fixed `c82697d`, `7497f9f` | decision 29: `timeout: 30`, one 25 s budget, nodes in parallel |
| HR-P1-test-70 | low | fixed `c82697d`, `0cfd567` | gaps, no red step |
| HR-P1-ha-92 | low | fixed `5f8474b` | the refusal clears after `auth_ok` |
| HR-P1-conc-93 | low (I) | fixed `3b3c64d` | a forget the sync still serves is kept one more sync |
| HR-P1-err-94 | low (I) | fixed `7497f9f`, `a2054f6` | an erase may finish within 20 s of the stop budget; docs: an interrupted erase must be repeated |
| HR-P1-err-99 | low | fixed `0df0fd1` (docs) | deleting `selection.json` alone restores the last-good copy; the docs say to delete both or clear in the web UI |
| HR-P1-cfg-11 | note | accepted | refusing `nodes.json` for one bad entry would drop every registry node; the entry is logged and listed not enabled |
| HR-P1-ha-13 | note | fixed `75b059f` | |
| HR-P1-ha-14 | note | fixed `75b059f` | docs: the Home Assistant connection honours `HTTP_PROXY`/`NO_PROXY` |
| HR-P1-cfg-15 | note | fixed `04c6175` | |
| HR-P1-sup-19 | note | fixed `9928fba` (`.dockerignore`, `EXPOSE 5540/tcp`); health check → follow-up issue (decision 34); base images by digest → phase 3 | |
| HR-P1-data-20 | note | fixed `3dc88b0`, `080c027`, `04c6175` | |
| HR-P1-cfg-21 | note | (a) documented `04c6175`; (b) accepted: the Supervisor schema's `match()` is not read at a pin, the start check stays the bound → phase 3 cite | |
| HR-P1-conc-22 | note | fixed `6c5ae3c` | |
| HR-P1-conc-33 | note (verifier: low → note) | fixed `cf835b3` | `bridge.command_unserved` |
| HR-P1-data-36 | note | open → phase 2 X2 | control and bidirectional characters into NodeLabel; a go-fabric `ComposeNodeLabel` candidate |
| HR-P1-data-37 | note | accepted | Home Assistant validates and refuses out-of-range values |
| HR-P1-bound-39 | note | accepted | bounded by device churn; freed with go-hamapping #40 |
| HR-P1-err-61 | note (verifier: low → note) | fixed `c82697d` | |
| HR-P1-conc-63 | note | fixed `c82697d` | |
| HR-P1-cfg-66 | note | fixed `259490a` | |
| HR-P1-data-67 | note | fixed `1c889a3` | |
| HR-P1-adpt-68 | note | upstream go-hamapping #41 | decision 30 |
| HR-P1-adpt-69 | note | upstream go-hamapping #40 | decision 30 |
| HR-P1-conc-71 | note | fixed `6c5ae3c` | |
| HR-P1-err-72 | note | fixed `c82697d` | |
| HR-P1-cfg-95 | note | fixed `7d12be9` | |
| HR-P1-conc-96 | note | fixed `f4e79bb` | |
| HR-P1-adpt-97 | note | accepted | decision 32 |
| HR-P1-bound-98 | note | accepted | decision 31 |
| HR-P1-data-100 | note | accepted | an entity removed while its entry was unread gets no forget; no exposure (pre-merge read 2) |
| HR-P1-err-101 | note (I) | fixed `703d0bd` | an erase after the stop began is refused |
| HR-P1-err-102 | note (I) | accepted | a registry node's hanging reset delays the other registry nodes' stop; within decision 29 |
| HR-P1-ha-04 | low | merged into HR-P1-ha-62 | verifier |
| HR-P1-cfg-35 | low | merged into HR-P1-err-06 | verifier |
| HR-P1-conc-60 | medium → low | merged into HR-P1-conc-32 | verifier |

Upstream: go-hamapping #39 (the range command), #40 (the stable-key
helper), #41 (the primary endpoint's order, a question). go-fabric's
camera kit: a follow-up issue after this pull request (decision 34).

Standing guards added: `TestAssessIgnoresInputOrder` (plan determinism),
the admin table at the daemon's wiring (`cmd/ha2fabric/admin_test.go`),
`TestImageDefaults` (the image's defaults), and every fix's regression
test (red seen, or stated otherwise in the fix reports).

Guards at the pull request's head: see the pull request body.
