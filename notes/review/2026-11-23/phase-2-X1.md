# Phase 2, unit X1 — the network

Baseline `2324fbf` (origin/main after #70). Flows read as an adversary:
every claim is a path from an untrusted input to a consequence, each hop
`file:line` at the baseline, references at their pin (`gf:` go-fabric
v0.25.0, `ui:` go-fabric-ui v0.13.0, `sup:` Supervisor `fe91b8db`, `core:`
Home Assistant core 2026.10.3). **M** measured (line read or probe run),
**I** inferred. Probes ran in the reviewer's `git archive` copy; nothing
ran against a live Home Assistant (decision 12).

Unit: `internal/web`, `cmd/ha2fabric` (web and admin wiring, the Supervisor
gate), `internal/config`, `script/run.sh`, `addon/config.yaml`, both
Dockerfiles, and go-fabric-ui's handler at its pin (what this host passes
to it and whether this host's wiring can be bypassed).

## Scope note on one held row

(held) HR-P1-auth-01: **measured, see the held report.** Nothing more is
said about it here — by ID and severity (medium) only, per decisions 21
and 24.

## Totals

0 high · 0 medium · 0 low · 4 note. Plus the held row above, measured.
No phase 0 or phase 1 fix on this path was found wrong or incomplete; the
adversarial reads that confirm them are in "Fixes under an adversary".

---

## The web listener, each mode — measured route/mode matrix

Method: the server the daemon actually builds (`webOptions`:
`cmd/ha2fabric/main.go:296-338`) driven by `httptest` for each mode, one
admin write route (`PUT /api/v1/operation/log/level`) and the status
route, each kind of client. Measured (probe `TestProbeX1WriteRoute`, and
the standing `TestAdminTable`/`TestHost`/`TestAllow`, all green):

| Mode | Admin write by the intended client | Every other client measured |
| --- | --- | --- |
| add-on, gate open | 200 only for `RemoteAddr 172.30.32.2` + `X-Remote-User-Groups: system-admin` + `X-Fabric-UI: 1` + same-origin | 403: a user-group-only request, a foreign Origin, a peer at another address with a forged group, a peer with a forged `X-Forwarded-For: 172.30.32.2` (the filter reads `RemoteAddr`, `web.go:99-107`) |
| add-on, gate unknown/closed (Supervisor older than 2026.10.4, or version unread) | none — `web.ReadOnly` | 403 for every request, admin group or not (`main.go:311-318`, `supervisor.go:110-167`) |
| stand-alone, image default (`127.0.0.1:8099`, decision 27) | none — `web.ReadOnly`; the listener is loopback so the LAN cannot reach it at all | 403 |
| stand-alone `WEB_ADMIN` + `WEB_ALLOW` | token mode: only a request carrying go-fabric-ui's token | 403 for an allowed address without the token, 403 for a non-allowed address |

The admin check, stated as it is (decision 21): in the add-on a request is
the administrator when its source address is the Ingress proxy
`172.30.32.2`, its `X-Remote-User-Groups` (set by the Supervisor) lists
`system-admin`, and the Supervisor the daemon read at start is 2026.10.4 or
later; otherwise the web UI serves the status page only. Stand-alone the
status page only, unless `WEB_ADMIN`, which is go-fabric-ui's token mode.

---

## HR-P2-net-01 — note — T2: a LAN peer forging source `172.30.32.2`

**Flow.** R1 (LAN) → a TCP SYN to the HAOS host's LAN address `:8486` with
source spoofed to `172.30.32.2` → the daemon's filter
(`internal/web/web.go:99-107`) would admit it by `RemoteAddr`.

**Claim.** The write never completes: the SYN's source is on the
Supervisor's internal bridge subnet (172.30.32.0/23), so the host routes
the SYN-ACK onto that bridge, not back to the LAN peer; the peer never sees
it and cannot finish the handshake. A host with reverse-path filtering may
also drop the SYN on ingress.

**M/I.** I (the routing argument is not run; no LAN segment was built).
`net.ipv4.conf.*.rp_filter = 2` on the review host (loose) — HAOS's value
is unread.

**Proposed fix.** None in code. `docs/operations.md` ("The web UI") should
state the argument the threat model row T2 already names: the add-on's
listener is on the host network, admits only `172.30.32.2`, and a LAN peer
cannot usefully forge that source because the return path is the internal
bridge. **Regression test.** None (documentation); the live test logs
HAOS's `rp_filter` (see questions).

---

## HR-P2-frame-02 — note — T9: framing, as this host configures it

**Flow.** R2 (a hostile page) → an `<iframe>` of the daemon's page → a
clickjack of an admin control.

**Claim.** go-fabric-ui's served page carries
`Content-Security-Policy: … frame-ancestors 'self' <Options.FrameAncestors>`
(`ui:fabricui/spa.go:129-153`). This host passes **no** `FrameAncestors`
(none set in `cmd/ha2fabric` or `internal/web`; grep: none), so the policy
is `frame-ancestors 'self'`: only the page's own origin may frame it. A
hostile page on another origin cannot. **M** (host code read; the policy
string read at the pin).

Under Ingress the page is framed by Home Assistant's frontend, which is the
**same** origin (same host:port, the Ingress path prefix differs only in
the path), so `'self'` admits it — but this rests on the frontend serving
the panel from the same origin, which needs the live frontend to confirm
(it is a question, not a finding, where it needs Home Assistant's
frontend). **M/I:** M for the policy this host sets; I for the framing
parent under a live Ingress.

**Proposed fix.** None; the policy is correct. See the live-test question.

---

## HR-P2-cfg-03 — note — the Supervisor gate does not re-read once open

**Flow.** The gate reads `GET http://supervisor/info` at start and opens
when the version is ≥ 2026.10.4 (`cmd/ha2fabric/supervisor.go:116-167`).
On `gateOpen` `run` **returns** (`:125-127`) — it never reads the version
again. A Supervisor later downgraded below the floor would leave the gate
open although the gate's own floor is no longer met.

**Claim.** Informational only: the Supervisor is trusted (decision 11), a
downgrade mid-run is not an untrusted-attacker action, and a restart
re-reads. Worth a line so the behaviour is deliberate, not assumed.
**M** (the `return` read at the pin).

**Proposed fix (optional).** Keep `run` looping at `recheck` even after
`gateOpen`, so a downgrade re-gates to the status page; cheap and makes the
gate symmetric with its open path. **Regression test.** A gate whose
successive reads go `open` then below-floor ends `gateClosed`.

---

## HR-P2-net-04 — note — the Matter port's reach before commissioning (T11)

**Flow.** R1/R4 on the LAN → the Matter UDP/TCP port (host network) → a
write in Home Assistant.

**Claim.** An uncommissioned LAN peer reaches only the node's PASE
commissioning surface; a write needs a CASE session, which needs a fabric,
which needs commissioning with the setup passcode. The configured node's
passcode is random since phase 0 (decision 14, kept beside the store) or
operator-set; a trivial value is refused
(`internal/config/config.go:319-327`, `node.ValidateSetupPIN`), the public
test value is honoured only when set explicitly, with a warning
(`:181-184`). So no LAN peer reaches `call_service` before the operator
commissions it; after commissioning it is a fabric administrator bounded to
the exposed entities' services (T12, a closed table — X2's path). **M**
for the passcode gate; the commissioning-window lifecycle (auto-open,
duration, re-open bounds) is go-fabric's node, gold standard, not re-read
here. **M/I:** M (config), I (window lifecycle deferred to go-fabric).

**Proposed fix.** None; confirms T11. **Regression test.** Covered by the
config validation tests and go-fabric's node tests.

---

## Fixes under an adversary (brief item 7)

Each phase 0/1 fix on this path was re-read as an attacker, not only under
its test; all hold.

- **HR-P0-auth-20 / the groups + version gate.** The add-on admits the
  administrator only with `X-Remote-User-Groups: system-admin` on a
  Supervisor ≥ 2026.10.4 (`main.go:311-324`, `supervisor.go:34-37`,
  `:116-167`). Measured: an old/unread Supervisor → `ReadOnly` for every
  request (matrix above). Holds.
- **HR-P0-cfg-22 / `WEB_ADMIN` off loopback without `WEB_ALLOW`.** Refused
  at `config.Load` (`config.go:393-395`, exit 2); and `web.Run` refuses a
  non-loopback bind under that mode (`internal/web/web.go:125-128`). A
  second, independent guard at bind time. Holds.
- **HR-P0-ha-23 / -ha-31 / the token over `ws://` and redirects.** The
  Supervisor gate's client takes no proxy and follows no redirect, so
  `SUPERVISOR_TOKEN` goes to the Supervisor and nowhere else
  (`supervisor.go:85-99`); `GET /info`'s body is limited to 1 MiB
  (`:190`). Holds for the admin-gate call too, not only the HA dial.
- **HR-P1-cfg-12 / the image's listener (decision 27).** The image drops
  `WEB_LISTEN=0.0.0.0:8099` and takes the binary default `127.0.0.1:8099`
  (`Dockerfile:10-14`, `config.go:51`); measured: the default is loopback,
  so a LAN peer cannot reach the listener at all. Holds.
- **HR-P1-test-16 / the admin table.** `TestAdminTable` enumerates every
  `x-admin` route of go-fabric-ui's contract × each client; re-run green,
  and the adversarial additions (forged XFF, foreign Origin, another
  peer address) are all 403. Holds.

---

## What I did not read, and why

- go-fabric-ui's own request handling internals (CORS absence, the Origin
  parser, the token's constant-time compare) beyond what this host passes
  to it: go-fabric-ui's own review owns its handler (brief). I read the
  seams this host wires (`Admin`, `AllowHost`, `AllowedHosts`,
  `FrameAncestors`) at the pin.
- go-fabric's commissioning-window lifecycle and PASE internals: gold
  standard, not re-implemented or re-judged here (HR-P2-net-04).
- The camera relay and entity-data flows: X2/X3's units.
- Live Home Assistant / live Ingress: decision 12; the owner's live test
  (the questions below).

## Probes run (pass/fail)

- `TestProbeX1WriteRoute` (reviewer's copy, `cmd/ha2fabric/probe_x1_test.go`) —
  the adversarial route/mode matrix — **pass** (green; the matrix above).
- `TestAdminTable`, `TestHost`, `TestAllow` (standing) re-run — **pass**.
- The measurement for the held row — ran; result in the held report.

## Questions for the owner (with recommendation)

1. **Live Ingress framing (T9, HR-P2-frame-02).** The live Ingress test
   should confirm Home Assistant's frontend frames the add-on panel from
   the same origin as the panel's own (so `frame-ancestors 'self'`
   suffices) and that no admin control is reachable from a cross-origin
   frame. **Recommendation:** add this check to the owed live Ingress test;
   no code change expected.
2. **Source spoofing from the LAN and `rp_filter` (T2, HR-P2-net-01).** The
   live test should log HAOS's `net.ipv4.conf.*.rp_filter` and confirm a
   LAN peer cannot complete a handshake with source
   `172.30.32.2`. **Recommendation:** add to the live test; document the
   argument in `docs/operations.md` regardless.
3. **The Supervisor gate's downgrade symmetry (HR-P2-cfg-03).** Keep the
   gate rechecking after it opens, so a Supervisor downgrade re-gates to
   the status page? **Recommendation:** yes — a few lines, makes the gate
   symmetric; low priority (Supervisor trusted, a restart re-reads).

## Probe appendix

The probe source (`cmd/ha2fabric/probe_x1_test.go` with a one-line
`nowZero()` helper) is in the reviewer's `git archive` copy only — it was
never written into the shared worktree. The fix wave can lift it as the
regression for the adversarial matrix. Its output is the matrix table
above. The held row's measurement artefacts are in the held report.

Probe source (`cmd/ha2fabric/probe_x1_test.go`; one row that only exercised the held row's code side is omitted here):

```go
// SPDX-License-Identifier: Apache-2.0
// Probe X1 (phase 2): an adversary against the web listener in each mode.
// Not for the tracked tree.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	fabricui "github.com/SukramJ/go-fabric-ui/fabricui"

	"github.com/SukramJ/go-ha2fabric/internal/config"
	"github.com/SukramJ/go-ha2fabric/internal/web"
)

// TestProbeX1WriteRoute drives one admin write route (PUT the log level)
// as several attackers, through the server the daemon builds (webOptions),
// in each mode, and prints who is admitted (200/4xx) vs refused.
func TestProbeX1WriteRoute(t *testing.T) {
	quiet := slog.New(slog.DiscardHandler)
	load := func(env map[string]string) *config.Config {
		env["STORE_PATH"] = filepath.Join(t.TempDir(), "ha2fabric.db")
		cfg, err := config.Load(nil, config.MapEnv(env), io.Discard)
		if err != nil {
			t.Fatalf("load %v: %v", env, err)
		}
		return cfg
	}
	build := func(cfg *config.Config, gate *supervisorGate) http.Handler {
		_, ring := web.NewLogger(io.Discard, new(slog.LevelVar))
		s, err := web.New(webOptions(cfg, gate, quiet, fabricui.Options{Logger: quiet, Operations: web.NewOperations(ring, "1", nowZero())}))
		if err != nil {
			t.Fatal(err)
		}
		return s.Handler()
	}
	type atk struct{ name, remote, host, origin, groups, xff string }
	route := "PUT /api/v1/operation/log/level"
	send := func(h http.Handler, a atk) int {
		m, p, _ := strings.Cut(route, " ")
		req := httptest.NewRequest(m, p, strings.NewReader(`{"level":"warn"}`))
		req.Host, req.RemoteAddr = a.host, a.remote
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(fabricui.RequestHeader, "1") // X-Fabric-UI: 1
		if a.origin != "" {
			req.Header.Set("Origin", a.origin)
		}
		if a.groups != "" {
			req.Header.Set(web.UserGroupsHeader, a.groups) // X-Remote-User-Groups
		}
		if a.xff != "" {
			req.Header.Set("X-Forwarded-For", a.xff)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	type mode struct {
		name string
		h    http.Handler
		atks []atk
	}
	addonGate := &supervisorGate{}
	addonGate.state.Store(int32(gateOpen))
	addonOld := &supervisorGate{} // gateUnknown
	modes := []mode{
		{"add-on (gate open)", build(load(map[string]string{config.SupervisorTokenEnv: "s"}), addonGate), []atk{
			{"proxy .2, admin group, same-origin", "172.30.32.2:4000", "ha.local:8123", "https://ha.local:8123", "system-admin", ""},
			{"proxy .2, user group only", "172.30.32.2:4000", "ha.local:8123", "https://ha.local:8123", "system-users", ""},
			{"proxy .2, admin group, foreign origin", "172.30.32.2:4000", "ha.local:8123", "https://evil.example", "system-admin", ""},
			{"co-installed add-on .33.5, forged admin group", "172.30.33.5:4000", "ha.local:8123", "https://ha.local:8123", "system-admin", ""},
			{"LAN peer, forged admin group + forged XFF=.2", "192.168.1.20:4000", "ha.local:8123", "https://ha.local:8123", "system-admin", "172.30.32.2"},
		}},
		{"add-on (gate unknown / old Supervisor)", build(load(map[string]string{config.SupervisorTokenEnv: "s"}), addonOld), []atk{
			{"proxy .2, admin group", "172.30.32.2:4000", "ha.local:8123", "https://ha.local:8123", "system-admin", ""},
		}},
		{"stand-alone (image default: loopback, no WEB_ADMIN)", build(load(map[string]string{"HA_URL": "ws://127.0.0.1:8123/api/websocket", "HA_TOKEN": "t"}), nil), []atk{
			{"loopback, admin group forged", "127.0.0.1:4000", "127.0.0.1:8099", "http://127.0.0.1:8099", "system-admin", ""},
		}},
		{"stand-alone WEB_ADMIN + WEB_ALLOW, no UI token", build(load(map[string]string{"HA_URL": "ws://127.0.0.1:8123/api/websocket", "HA_TOKEN": "t", "WEB_ADMIN": "true", "WEB_LISTEN": "0.0.0.0:8099", "WEB_ALLOW": "192.0.2.10"}), nil), []atk{
			{"allowed addr .10, no token", "192.0.2.10:4000", "192.0.2.10:8099", "http://192.0.2.10:8099", "", ""},
			{"non-allowed LAN peer", "192.168.1.20:4000", "192.0.2.10:8099", "http://192.0.2.10:8099", "", ""},
		}},
	}
	fmt.Printf("PROBE X1: %s  (200=write accepted; 403=refused)\n", route)
	for _, md := range modes {
		fmt.Printf("  mode: %s\n", md.name)
		for _, a := range md.atks {
			fmt.Printf("    %-55s -> %d\n", a.name, send(md.h, a))
		}
	}
}
```

`probe_x1_helper_test.go`:

```go
package main
import "time"
func nowZero() time.Time { return time.Unix(0,0) }
```
