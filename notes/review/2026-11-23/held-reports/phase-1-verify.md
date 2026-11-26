# Phase 1 — verification, held details

Untracked (`held/` is in `.git/info/exclude`). Companion of
`notes/review/2026-11-23/phase-1-verify.md`; marks and prefixes as there.
Baseline `5eefc0a`. Probes in the verifier's `git archive` copy only;
nothing ran against a live system.

## HR-P1-auth-01 — medium (I) — confirmed, holding justified

### Measured (read by the verifier at the pin)

| Fact | Line |
| --- | --- |
| The add-on's administrator is: the gate is open, the peer address is `172.30.32.2`, `X-Remote-User-Groups` lists `system-admin` | `cmd/ha2fabric/main.go:246-260`; `internal/web/web.go:141-148`, `:165-180` |
| The client filter before go-fabric-ui admits that one address and reads only `r.RemoteAddr` | `internal/web/web.go:91-104` |
| The listener is `0.0.0.0:8486` on the host's network | `script/run.sh:77-78`; `addon/config.yaml:37`, `:43-44` |
| The Supervisor keeps `NET_RAW` in an add-on's capabilities unless the feature flag `APP_DROP_NET_RAW` is set, default `False`; its own comment gives the reason: "NET_RAW allows ARP and MAC spoofing on the shared internal network." | `sup:supervisor/docker/app.py:402-407` (comment `:405`, flag `:406`) |
| The Supervisor's request filter drops the user headers a client sent (that it then sets its own, `:412-419`, is S's citation and was not re-read); the Ingress cookie is not in the filter | `sup:supervisor/api/ingress.py:47-64` |
| Validating an Ingress session is a call only Core may make | `sup:supervisor/api/ingress.py:164-173` (`require_home_assistant`) |

So it is measured that nothing in a forwarded request is a secret the
daemon could check, that the address is the only thing separating the
proxy from another peer of that network, and that the Supervisor's own
source names address spoofing on that network as the reason for a flag
it leaves off.

### Inferred (not read at a pin, not run)

- That a container on the Supervisor's bridge with `NET_RAW` can make
  the host send the answers for `172.30.32.2` to it (an ARP answer for
  the address, or frames with the proxy's MAC so the bridge learns it on
  the attacker's port), complete a TCP handshake to the host's
  `172.30.32.1:8486` from that address and send a request with the
  header. This is the well-known behaviour of a Linux bridge without
  ARP inspection, and the Supervisor's comment assumes it; nobody in
  this review ran it, and no line of the kernel, Docker or the
  Supervisor's network setup was read for it (ebtables or `bridge-nf`
  rules, the neighbour table's handling of a competing answer, whether
  the real proxy's RST for the unexpected SYN-ACK arrives first).
- That an add-on on the host's network can do the same from the host's
  namespace: not examined, not claimed.
- S's probe S8 (not re-run) measures the opposite direction only:
  every other address is refused, whatever headers it sends.

### Does medium hold?

Yes, as **medium (I)**. The consequence is every admin route (pairing
codes, selection, erase, registry, log), which is the highest this
daemon has; the role is the one decision 19 names as untrusted; the
measured half already shows there is no second factor against that
role. It is not high because it needs a hostile add-on the operator
installed, the decisive step is inferred, and such an add-on is one
manifest key (`homeassistant_api`, `hassio_api`) away from wider reach
that the operator would have had to accept as well. It is not low
because the code comments and the docs read the groups header as a
second factor, which it is against a Home Assistant user and is not
against this role.

The daemon's code is correct for the input it can see; the row is a
property of Ingress for every add-on on the host's network. The fix here
is the documentation sentence and, later, the tighter bind of decision
22 (which removes the LAN, not a container on the bridge); the rest is
an upstream proposal after a measurement.

### Holding

Justified: an untrusted role (decision 19), administrator reach, no fix
in this repository that closes it. But the condition it rests on is
public (the Supervisor's comment and flag; Ingress's documented rule
that the proxy's address is the check), so the hold is short by nature:
the documentation sentence S proposes is the publication. Wording per
decision 21 for every tracked file until then: "the add-on's
administrator is a request from the Ingress proxy's address whose user
groups, set by the Supervisor, list `system-admin`, on a Supervisor of
2026.10.4 or later" — the check as it is, no sentence about who else
could hold the address. Question 2 of the tracked report asks until
what the row is held, because decision 21's "until the tag that carries
the fix" has no fix to wait for.

## The four held lows — holding is not needed

| ID | Who can trigger it | Verifier |
| --- | --- | --- |
| HR-P1-bound-02 | a commissioned controller with rights on the camera's endpoint | low confirmed (sampled); public |
| HR-P1-conc-03 | the same, or a controller that simply went away | not sampled; public |
| HR-P1-ha-04 | the same and only if go-fabric cancels an invoke's context (I), or a Home Assistant connection stalled for 30 s | low confirmed; public; one defect with HR-P1-ha-62 |
| HR-P1-bound-17 | the same | not sampled (the candidate half read: `internal/camera/camera.go:410-415`); public |

Reasons:

1. **The role.** Decision 11: a commissioned controller is a fabric
   administrator the operator paired; it is bounded to the exposed
   entities' services, and a camera's session, still and candidates are
   that camera's services. Neither the LAN nor a co-installed add-on
   (decision 19) reaches any of the four: the commands need a CASE
   session. "Exploitable by an untrusted role" is not met.
2. **The class.** Each is a missing bound whose cost is at most one
   Home Assistant request or session per controller request. That is
   HR-P0-bound-28's class and role exactly (the candidate queue of one
   session), and that row was published and fixed in the open
   (`phase-0-register.md`). bound-02 is literally its other half.
3. **Decision 21** governs the wording about the admin check; none of
   the four touches it.
4. **Cost of holding.** Four rows, two go-fabric facade questions and
   G-B's duplicate would have to stay out of every tracked file, issue
   and commit message until a fix merges, and the go-fabric issues
   cannot be written without saying what they are for.

The counter-argument, for the owner: a paired controller of a large
ecosystem is software the operator does not control, and bound-02's
number (65 535 sessions per camera, each a provider session in Home
Assistant) is a cheap way to exhaust Home Assistant from a fabric. It
remains a commissioned administrator doing so, and the same controller
can call the exposed services in a loop without any defect.

### HR-P1-bound-02 — low — confirmed

M (read): the host's map has no cap and every offer is accepted
(`internal/camera/camera.go:183`, `:328-357`); the kit bounds only the
id space (`gf:cluster/camera/provider.go:210-224`) and its table is an
uncapped slice (`:97`, `:158-168`); the port's result is stored as it
comes (`:557-563`). M (probe V3, `internal/camera/probe_v_test.go`
`TestProbeVSessions`):
`PROBE V3: offers accepted=2000 of 2000, upstream subscriptions=2000, still open=2000, goroutines +2000`.
The cost on Home Assistant's side per session stays I. Low holds: one
session per request, a commissioned controller.

## The duplicate HR-P1-ha-04 (S, held) / HR-P1-ha-62 (G-B, tracked)

**They are the same defect**: the same lines
(`internal/ha/subscription.go:122-125`, `internal/ha/client.go:452-457`),
the same Home Assistant reference
(`core:homeassistant/components/camera/webrtc.py:275-280`), the same
measurement (S7, G-B's probe, the verifier's V4), the same fix
(`unsubscribe_events` for the id on a detached context). S adds who can
reach it and that the leaked session is counted by no table, so a cap
(bound-02) would not see it; G-B adds
`core:…/websocket_api/connection.py:258-268`.

G-B's report therefore describes, in a file meant to be tracked, a row
S held. Nothing is published yet: all three reports are untracked at
this read (`git status`: `??`).

**Does the item need holding at all?** No (the table above): low, and
only a commissioned controller or a stalled Home Assistant connection
reaches it.

- **If the owner publishes the four lows (recommended):** the survivor
  is **HR-P1-ha-62**, low, in G-B's report as written; G-B's text needs
  no change. S's report replaces the `HR-P1-ha-04` block by "merged
  into HR-P1-ha-62 (G-B)" and S's two additions (who; counted by no
  table, see HR-P1-bound-02) move into the fix brief. `HR-P1-test-70`
  keeps its reference to 62.
- **If the owner holds them:** the survivor is **HR-P1-ha-04** (held),
  and before anything is committed G-B's report must lose the
  description. Its block becomes exactly:

  > ### HR-P1-ha-62 — low — held
  >
  > Merged into HR-P1-ha-04 (unit S, held); named by ID and severity
  > only.

  and the other mentions go with it: in `HR-P1-test-70` the second
  bullet (`internal/ha/camera_test.go:54-192 … a ctx that ends before
  the result`) and "62" in its title and fix line; in item 3 "A late
  result: HR-P1-ha-62" becomes "One row held: HR-P1-ha-62"; in item 5
  nothing; in "Probes" the `TestProbeGBSubscribeTimeoutLeaves` line
  moves to a held file; the sentence "No finding is an open exploitable
  security weakness: no held file" is corrected. The verifier's tracked
  report then also reduces its `HR-P1-ha-62` paragraph and the V4 line
  to ID and verdict.

## What the tracked verification report leaves out

Only the above: the auth-01 analysis, the four lows' subjects and
triggers, probe V3's output, and the statement that ha-04 and ha-62 are
one defect. Its `HR-P1-ha-62` paragraph describes G-B's row with G-B's
own facts and is the one passage to cut if question 1 is answered
"hold".
