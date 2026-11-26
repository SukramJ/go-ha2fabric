# Phase 2 — verification, held details

Untracked (`held/` is in `.git/info/exclude`). Companion of
`notes/review/2026-11-23/phase-2-verify.md`; marks and prefixes as there.
Baseline `2324fbf`. Probes in the verifier's `git archive` copy only; no
container was run (by the brief) and nothing ran against a live system.

## HR-P1-auth-01 — medium — the measurement judged

X1's held report (`held/phase-2-X1.md`) is judged on its method and its
evidence; the measurement was not repeated.

### What the measurement supports

| Step of X1 | What it is evidence for | Verifier |
| --- | --- | --- |
| Code side (table) | the admin check as it is; nothing forwarded is a secret | M already in phase 1; the line numbers moved, the facts did not. Not re-read line by line here |
| 1 (capabilities of a default container) | the precondition "default capability set" on Docker 29.8.0 | sound; it is Docker's default, and the Supervisor's choice to leave it is the pinned line `sup:supervisor/docker/app.py:402-407` (read in phase 1) |
| 2 (the simple way is refused) | narrows the path to the one the Supervisor's comment names | sound, and a useful negative |
| 3 (the host's neighbour entry changed while the real holder was up) | the step phase 1 had as I | sound as observed: before and after values are given |
| 4 (a packet for the address arrived at the other container) | delivery follows the entry | sound, but it is one ICMP packet under a re-send loop, not a TCP exchange |
| 5 (`TestProbeX1WriteRoute`, the row labelled as the spoofed source) | the code side only | **over-labelled**: `httptest` sets `RemoteAddr` by construction; the row shows what phase 1's S8 complement already implied, not that such a request can be produced. It ties nothing to steps 3–4 beyond the argument |

### What it does not support

1. **End to end.** No request from the second container reached the
   listener and no admin route answered it. X1 says so ("What stays
   inferred") and calls the rest mechanics; that is a fair reading, but
   the header "MEASURED, confirmed" is stronger than the evidence. The
   honest label is: **confirmed, medium; the decisive network step M on
   a replica; the end-to-end request I.**
2. **The replica is not the target.** A user-defined Docker bridge on
   the review host stands in for the Supervisor's network on HAOS.
   Unread and unmeasured: how the Supervisor creates its network
   (options, isolation), HAOS's kernel settings and any filter rules on
   that bridge, and whether a current Supervisor release ships the flag
   on. X1 lists these for the live test; they are the reason the row
   stays "(I)" for the installed system.
3. **Stability.** The entry was held "for the capture window" against a
   real holder that was idle. The Supervisor's proxy is not idle while
   an operator uses Ingress; how the entry behaves under the real
   holder's own traffic was not measured. This affects reliability, not
   feasibility.
4. **One precondition is missing from X1's list.** The Supervisor gate
   must be open (2026.10.4 or later); on an older Supervisor the admin
   routes refuse everyone, this role included
   (`cmd/ha2fabric/main.go:311-318`: `gate.open() && ingress(r)`, M). The other stated preconditions (a
   co-installed add-on the operator installed, the default capability
   set, the add-on mode) are right.

### Verdict

Medium holds, for phase 1's reasons, now with the formerly inferred step
observed on a replica. Not high: an add-on the operator installed, the
end-to-end step and the target system still I, and the same role is one
manifest key from wider reach. Not low: the highest consequence this
daemon has, and no second factor against this role. X1's "the daemon's
code is correct for the input it can see; no fix here closes it" is
right.

Corrections for X1's held report before it is ever published: the
header's wording (point 1), the label of the probe's sixth row (step 5),
the gate precondition (point 4), and the probe's source pasted into the
appendix (it is only in X1's scratch copy, which a reboot clears).

### Publishing with the `0.7.0` tag — agree, with two conditions

X1 recommends publishing with the tag, together with the documentation
sentence, decision 22's follow-up and an upstream proposal. Agreed:

- there is no fix in this repository to wait for, so decision 21's
  "until the tag that carries the fix" can only mean the tag that
  carries the documentation sentence;
- the condition is public in the Supervisor's own source, and it holds
  for every add-on on the host's network, so a longer hold protects
  nobody;
- decision 24 already names the tag.

Conditions: (1) the published text uses the corrected label (replica M,
end to end and HAOS I) unless the owner's live test has by then run the
whole path — the live test is desirable before the tag, not a gate for
it; (2) the upstream proposal is opened the same day, written as a
proposal about the forwarded request's authenticity and the flag's
default, without this repository's routes as the example.

Options for the owner (question 3 of the tracked file):
(a) publish with the `0.7.0` tag, with the documentation sentence and
the upstream proposal — **recommended**; (b) publish only after the live
test ran the whole path; (c) hold until the Supervisor answers the
proposal.

## HR-P2-data-30 — medium — confirmed; the hold is not needed

### The read and the probe

- M (read): the three registry events and two cases of a state event
  call `resyncSoon` (`internal/bridge/bridge.go:338-349`, `:841-843`,
  `:849-851`); `resyncSoon` stops the pending timer and arms a new one
  on every call (`:394-407`), 1 s by default (`:197-198`); no other
  caller of a sync exists beside the connection's handlers, the owed
  mark and `Resync` (`:242`, `:246`, `:283`, `:295`, `:767`); a command
  is checked against `b.serving`, which only a sync writes
  (`internal/bridge/command.go:72-79`, `bridge.go:605`).
- M (probe `TestX2ProbeResyncStarved`, verifier's run): `switch.coffee,
  disabled, kept its endpoint for 5 s (25 debounces) while sensor.x2_new
  reported every 50 ms; last sync 5.562s before the flood ended`.
- One narrowing of X2's "who can make the stream": every entity of the
  last snapshot is known, exposed or not — `Assess` writes a verdict for
  each id (`internal/plan/plan.go:158-198`) and `known` is the verdicts'
  ids (`bridge.go:487-490`). So an ordinary chatty sensor that existed
  at the last sync does not do it; only entities that appeared since
  (they stay unknown because the sync they ask for is the one they put
  off), or a served entity whose description keeps differing. The rates
  of real integrations stay I.

### Severity

Medium holds on the functional half: a newly added integration whose
entities together report without a one-second gap is an ordinary event,
and from then on nothing changes on the bridge — not its own entities,
not any other registry change — without a line in the log, until a
reconnect, a restart or the web UI's reassemble. The security half
alone would be low: nothing becomes exposed that the rule did not
expose; what is delayed is a withdrawal from controllers the operator
paired (HR-P1-conc-33's reasoning, a note there), and an entity
disabled in Home Assistant no longer acts on a call anyway (I).

### Overlap with X3

Not a duplicate of any X3 row, but the same timer:

- X3 `err-61` proposes its retry "through `resyncSoon`'s timer" — under
  this row's condition that retry would be put off with everything
  else. The two fixes are one design: a pending sync has a latest time.
- X3 `bound-62` names the same triggers as a cost (one read per node
  per event); with this row fixed by a latest time, a continuous stream
  becomes a full read per node every `maxWait`, which makes `bound-62`'s
  shared reader matter more. Decide them together (question 5 of the
  tracked file, to which this row belongs once it is public).
- X2 `bound-37` and X3 `err-61` state the same visible consequence (a
  registry change not applied) from other causes; that is why their
  tracked sentences are not leaks.

### Should it be held?

No. Recommendation: **publish it now, with the reports' commit**, and
fix it in the phase 2 pull request.

1. The hold rule is for "an exploitable weakness" open to an untrusted
   role. Here the untrusted input gains no reach: it freezes the
   bridge's set as it is. It is a robustness defect with a security
   edge, the class of X3's `err-61` and X2's own `bound-37`, both
   public in the same reports.
2. Decision 23 published phase 1's camera lows for a weaker reason
   (their role was a commissioned controller); decision 18 published
   mediums that rest on public conditions. X2 itself leans this way
   (its question 6).
3. The cost of holding is real here: the fix must be designed with
   `err-61` and `bound-62` (above), whose briefs, tests and commit
   messages would have to avoid naming why the timer gets a latest
   time; the red test is the probe, which cannot be committed while the
   row is held.
4. Counter-argument, for the owner: until the fix merges, the row tells
   a reader how to keep an operator's exposure change from applying,
   with nothing but state reports. It needs a foothold in Home
   Assistant's entity data, is undone by a reassemble or a restart, and
   the fix is days away on an untagged main.

Options (question 2 of the tracked file): (a) public now, fixed in the
phase 2 pull request — **recommended**; (b) held until its fix merges,
then published with that pull request (X2's recommendation); (c) held
to the tag (decision 21's rule, made for a different kind of row).

If (b) or (c): the tracked verify file already says nothing; question 5
there must not list this row, and `err-61`'s brief must not explain the
latest time by it.

## Leak check — what each flagged line gives away

| File | Lines | What |
| --- | --- | --- |
| `phase-2-X1.md` | 64-65 | "a different, weaker position than a peer **on** that bridge (the held row)": names the held row's position outright. Remove the sentence; HR-P2-net-01 stands without it |
| `phase-2-X1.md` | 40 | among the refused clients, "a peer at another bridge address with a forged group": with 64-65 it points at the one address that is not refused. Reword to "another source address" |
| `phase-2-X1.md` | 54, 207 | "off-bridge" in the title and the question implies its counterpart. "A LAN peer" is enough; T2 itself is public in the threat model |
| `phase-2-X1.md` | 154-158 | HR-P0-auth-20's ID beside the description of the check as it is. Decision 21 allows the description; the pairing of ID and gate is already in `phase-0-register.md`. No change needed |
| `phase-2-X2.md` | 518-520 | "as decision 23 did for phase 1's camera rows": tells a reader the held row is of that class (a bound, not an authentication matter). Harmless in content, but it is a hint by the brief's rule; moot if the row is published |
| `phase-2-X3.md` | 119-122 | no leak: `bound-62` names the event classes and the one-second debounce as a cost. Read beside a held data-30 it supplies the path's first half; another reason to publish data-30 with it |

HR-P0-auth-20: no tracked sentence of the three reports describes the
condition before its fix; X1's lines 45-50 are the neutral wording of
decision 21.

## For the live test (added to X1's list)

- Run the whole path once on the installed system and record the
  answer's status, so the row's label can lose its "(I)" either way.
- Record the Supervisor's version and whether the gate was open.
- Record the state of the capability flag in that release.
