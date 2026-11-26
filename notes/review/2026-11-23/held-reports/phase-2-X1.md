# Phase 2, unit X1 — held details

Untracked (`held/` is in `.git/info/exclude`). Companion of the tracked
`notes/review/2026-11-23/phase-2-X1.md`. Baseline `2324fbf`. Marks: **M**
measured (a line read at the pin, a probe run, or a container measurement
run on this host), **I** inferred. Prefixes: `gf:` go-fabric v0.25.0,
`ui:` go-fabric-ui v0.13.0, `sup:` Supervisor `fe91b8db`, `core:` Home
Assistant core 2026.10.3. Probes ran in the reviewer's `git archive` copy
(`…/scratchpad/X1/src`); the container measurement ran against
reviewer-built Docker resources (all prefixed `p2x1-`, all removed);
nothing ran against a live Home Assistant.

This file holds one thing the tracked report may not: the measurement of
phase 1's held row (decision 24). Every HR-P2-* finding of X1 is public
and stands in the tracked report; none is an open exploitable weakness by
an untrusted role.

---

## (held) HR-P1-auth-01 — medium (I → the decisive step now M) — MEASURED, confirmed

Decision 24 ordered X1 to measure it. The claim (from `held/phase-1-S.md`,
`held/phase-1-verify.md`): the add-on's administrator check is
`RemoteAddr == 172.30.32.2` **and** `X-Remote-User-Groups` contains
`system-admin` **and** the Supervisor gate is open; the second and third
hold only for a request the Supervisor really forwarded, so a peer that
takes the address `172.30.32.2` on the Supervisor's Docker network writes
the groups header itself and is the administrator. S measured every code
line; the one inferred, un-run step was that a co-installed add-on with
`NET_RAW` can make the host deliver `172.30.32.2`'s traffic to it and
complete a TCP handshake from that source. **That step is now measured on
this host.**

### The code side (M, re-read at `2324fbf`)

| Fact | Line |
| --- | --- |
| The client filter admits a request only when `net.ParseIP(RemoteAddr)` equals an `Allow` IP; it reads `r.RemoteAddr` and nothing else (no `X-Forwarded-*`) | `internal/web/web.go:99-107` |
| In the add-on `Allow` is forced to the Ingress proxy `172.30.32.2` only | `internal/config/config.go:298-302`, `:385-387`; `script/run.sh:77-78` |
| The admin check: `gate.open() && IngressAdmin(172.30.32.2)` — `From(proxy)` (RemoteAddr match) then `X-Remote-User-Groups` split on `,`, exact `system-admin` | `cmd/ha2fabric/main.go:311-324`; `internal/web/web.go:149-156`, `:167-188` |
| `isAdmin` also needs `X-Fabric-UI: 1` and the Origin to be admitted | `ui:fabricui/handler.go:775-782`, `:759-772` |
| The listener is `0.0.0.0:8486` on the host network; the Supervisor dials the add-on at the bridge gateway `172.30.32.1:8486` with source `172.30.32.2` | `script/run.sh:77`, `addon/config.yaml:35-37`; `notes/reads/read-2026-11-23-ingress-host-facts.md` hop 2 |
| Nothing forwarded is a secret the daemon can verify (the groups header is a plain value; the Ingress cookie is Core's to validate) | `sup:supervisor/api/ingress.py:47-64`, `:164-173` |

So in the add-on the **only** thing separating the proxy from any other
peer on that network is the source address `172.30.32.2`.

### The network step (M — container measurement on this host)

Topology built to mirror the hassio network: a user-defined Docker bridge
`p2x1-net` 172.30.32.0/23, gateway `172.30.32.1` (the host side of the
bridge, standing in for the HAOS host that runs the host-networked
daemon); `p2x1-supervisor` the legitimate holder of `172.30.32.2`;
`p2x1-attacker` a co-installed add-on at `172.30.32.9` with **Docker's
default capability set** (the set the Supervisor leaves — it drops
`NET_RAW` only behind `APP_DROP_NET_RAW`, default `False`,
`sup:supervisor/docker/app.py:402-407`).

Measurements (every container removed afterwards):

1. **Default caps = NET_RAW present, NET_ADMIN absent.** `CapEff` of both
   containers `00000000a80425fb` → `NET_RAW` set (bit 13), `NET_ADMIN`
   clear (bit 12). M.
2. **The simple takeover is blocked.** `ip addr add 172.30.32.2/23 dev
   eth0` in the attacker → `RTNETLINK answers: Operation not permitted`
   (needs `NET_ADMIN`). M. So the attacker cannot just configure the
   address and let its kernel do TCP; it must drive the takeover from
   raw sockets.
3. **The ARP takeover works with NET_RAW alone.** The attacker sent five
   crafted ARP replies "`172.30.32.2` is-at `<attacker MAC>`" to broadcast
   (a 42-byte `AF_PACKET`/`SOCK_RAW` frame, `arpspoof.c` in the appendix —
   `NET_RAW` only, no `ip`/`ifconfig`). The host's neighbour entry for
   `172.30.32.2` flipped from the real holder's MAC
   (`7a:a1:21:0f:83:66`) to the attacker's (`c2:90:64:dd:02:d2`) **while
   the real `.2` container was up**. M.
4. **The host then delivers `.2`'s traffic to the attacker.** With the
   attacker re-sending the ARP reply in a loop, a host → `172.30.32.2`
   packet was captured on the attacker's NIC:
   `IP 172.30.32.1 > 172.30.32.2: ICMP echo request` (tcpdump on the
   attacker, host pinging `.2`). M. The SYN-ACK of a handshake the
   attacker opens with source `.2` to `172.30.32.1:8486` would likewise
   land on the attacker.
5. **Code tie-in (probe `TestProbeX1WriteRoute`,
   `cmd/ha2fabric/probe_x1_test.go`).** A request with `RemoteAddr`
   `172.30.32.2`, `X-Remote-User-Groups: system-admin`, `X-Fabric-UI: 1`
   and the browser's Host/Origin drives the admin write `PUT
   /api/v1/operation/log/level` to **200** through the server the daemon
   builds (`webOptions`, gate open). Every other attacker in the table is
   refused (see below). So once the attacker owns `RemoteAddr == .2`
   (step 3–4), the forged groups header is accepted and every admin route
   answers.

### What stays inferred (I, not run)

The mechanical finish only: driving a full userspace TCP 3-way handshake
and the HTTP request from source `172.30.32.2` over raw sockets
(`NET_RAW`), since the address is not configured on the interface (step 2)
and no in-kernel socket handles it. This is standard spoofed-source
raw-socket work (e.g. a scapy/`AF_PACKET` TCP shim); it was not built
here — no scapy on the host and a bespoke TCP stack is out of the probe's
budget — and it is a matter of attacker effort, not feasibility. The
real `.2` never sees the SYN-ACK (it is delivered to the attacker), so the
competing-RST concern of `held/phase-1-verify.md` is on the ARP race, not
the TCP flow: the attacker keeps the host's cache poisoned by re-sending
the reply (measured to hold for the capture window).

### Host-specific caveat for the live test

This host: `net.ipv4.conf.all.rp_filter = 2` (loose), `net.ipv4.conf.
default.rp_filter = 2`, `net.bridge.bridge-nf-call-iptables = 1`; Docker
29.8.0. The ARP delivery was unaffected by these. HAOS may differ. The
owner's **live test** must, on the installed HAOS/Supervisor:

1. confirm the add-on's daemon sees `RemoteAddr == 172.30.32.2` with
   `host_network: true` (log it once — already owed by
   `read-2026-11-23-ingress-host-facts.md`);
2. from a second, minimal co-installed add-on (default caps), attempt the
   ARP takeover of `172.30.32.2` and a spoofed-source request to the
   gateway `:8486` carrying `X-Remote-User-Groups: system-admin` and
   `X-Fabric-UI: 1`; record whether an admin route answers 2xx;
3. record HAOS's `rp_filter`, any `ebtables`/`bridge-nf` rules and whether
   the Supervisor ships `APP_DROP_NET_RAW` on by that release.

### Verdict

**Confirmed, medium (I) holds.** The consequence is every admin route
(pairing codes → a standing fabric and `call_service` on the exposed
entities, selection, erase, the node registry, the log). Not high: it
needs a hostile co-installed add-on the operator installed, and that
add-on is one manifest key (`homeassistant_api`, `hassio_api`) from wider
reach anyway. The previously decisive inferred step (ARP takeover +
traffic delivery) is now **measured**; only the userspace-TCP mechanics
stay inferred. The daemon's code is correct for the input it can see — the
row is a property of Ingress and the Supervisor's shared network, not a
defect in this repository. No fix here closes it; the remedies are those
already on record (decision 22's tighter bind removes the LAN but not a
container on the bridge; the documentation sentence; an upstream
Supervisor proposal — a per-add-on secret on forwarded requests, or
`APP_DROP_NET_RAW` on by default).

### Owner question specific to this row (held — carries the hint, so not in the tracked file)

Until what is HR-P1-auth-01 published? Decision 21's "until the tag that
carries the fix" has no fix to wait for (there is none in this repo).
**Recommendation:** publish it with the `0.7.0` tag together with the
documentation sentence (the publication is the sentence) and the decision
22 follow-up, and open the upstream Supervisor proposal then; the
condition is already public in the Supervisor's own source comment, so a
long hold buys nothing.

---

## Probe appendix

### `cmd/ha2fabric/probe_x1_test.go` — the adversarial route/mode table (ran, all green)

Drives one admin write route as several attackers through the server the
daemon builds (`webOptions`), in each mode. Output:

```
PROBE X1: PUT /api/v1/operation/log/level  (200=write accepted; 403=refused)
  mode: add-on (gate open)
    proxy .2, admin group, same-origin                      -> 200
    proxy .2, user group only                               -> 403
    proxy .2, admin group, foreign origin                   -> 403
    co-installed add-on .33.5, forged admin group           -> 403
    LAN peer, forged admin group + forged XFF=.2            -> 403
    SPOOFED source .2 (ARP-takeover), forged admin group    -> 200
  mode: add-on (gate unknown / old Supervisor)
    proxy .2, admin group                                   -> 403
  mode: stand-alone (image default: loopback, no WEB_ADMIN)
    loopback, admin group forged                            -> 403
  mode: stand-alone WEB_ADMIN + WEB_ALLOW, no UI token
    allowed addr .10, no token                              -> 403
    non-allowed LAN peer                                    -> 403
```

Reading: the only 200s are the genuine proxy-forwarded admin and the
"SPOOFED source .2" row, which is the httptest stand-in for the attacker
of step 3–4 above once it owns `RemoteAddr == .2`. A forged
`X-Forwarded-For: 172.30.32.2` is ignored (the filter reads `RemoteAddr`).
The source of the probe test is in the reviewer's copy
(`…/scratchpad/X1/src/cmd/ha2fabric/probe_x1_test.go`, with a one-line
`nowZero()` helper); it is the fix wave's candidate regression if the
owner wants the attacker row asserted. It is not an open-weakness probe
(no red line), so it need not be a held artefact; it is here because it is
the measurement tying the network step to the code.

### `arpspoof.c` — the NET_RAW ARP-reply tool (ran in `p2x1-attacker`)

A 42-byte Ethernet/ARP reply over `AF_PACKET`/`SOCK_RAW`, sender-IP
spoofed to the claimed address, our MAC, to broadcast; sent five times.
Needs `NET_RAW` only (no `NET_ADMIN`, no configured address). Full source
in `…/scratchpad/X1/arpspoof.c`. Used only against reviewer-built
`p2x1-*` containers, all removed.

### Docker resources (all created with the `p2x1-` prefix, all removed)

`p2x1-net` (network), `p2x1-supervisor`, `p2x1-attacker`, `p2x1-captest`
(containers). `docker ps -a --filter name=p2x1-`, the network and image
filters: none remain. The public `alpine:3` base image was pulled (not a
`p2x1-` image, shared, left in place).
