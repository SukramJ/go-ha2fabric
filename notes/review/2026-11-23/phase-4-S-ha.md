# Phase 4, run 1 — unit S-ha (Home Assistant client, camera, selection and nodes, controller → `call_service`)

Reviewer S-ha (Opus in Fable's role, owner rule 2026-11-24). Code at
`c3a9770` (go-fabric v0.26.0). Brief `notes/briefs/2026-11-26-review-p4.md`.
Prefixes: `gf:` go-fabric v0.26.0, `gf27:` go-fabric v0.27.0, `hm:`
go-hamapping v0.8.0, `core:` Home Assistant core 2026.10.3. M = measured
(line read, or a probe run), I = inferred.

## Scope read

Delta first (`git diff d16f3ab c3a9770` of every unit file): the import
paths (#76, #80) everywhere; citation re-pins in `internal/ha/camera.go`,
`internal/ha/message.go`, `internal/adapt/command.go`,
`internal/nodes/nodes.go`, `internal/camera/camera.go`; HR-P3-ha-09's reply
check in `internal/ha/client.go:565-587`; R7's removal of the camera
session's idle end (decision 31) in `internal/camera/camera.go` (option,
timer, `expire`, `stopIdle`, every call site). Each changed line was read
against what it replaced: no call site of the removed timer is left
(`grep IdleTimeout|session_idle|idle end`: only `internal/web/web.go:118`,
the HTTP server's, and the explanatory comment `camera.go:253`), and
HR-P3-ha-09's switch keeps a ping's `pong` and every other command's
`result` with `success` (core `websocket_api/messages.py:63-65`,
`commands.py:730-736`, as cited).

Full cut, every line: `internal/ha/client.go` (882), `message.go` (394),
`camera.go` (194); `internal/camera/camera.go` (971);
`internal/selection/*` (`selection.go`, `selection_test.go`,
`recover_test.go`, `fuzz_test.go`); `internal/nodes/*` (`nodes.go`,
`passcode.go`, `nodes_test.go`, `passcode_test.go`, `modes_test.go`,
`fuzz_test.go`, `export_test.go`, `testdata/fuzz/FuzzSlug/*`);
`internal/text/*` (`text.go`, `text_test.go`); `internal/adapt/command.go`
(434); `internal/bridge/command.go` (197); `internal/bridge/camera.go` (88).

References read at their pins for the judgements below:
`gf:cluster/camera/provider.go` (`NewProvider`, `change`, `provideOffer`,
`provideIceCandidates`, `abandonSession`, `endSession`, the
`ProviderConfig` bounds `:60-80`), `gf:cluster/camera/provider_peer.go`
(`End`, `signal`), `gf:cluster/camera/state.go:20-26` (`serial`),
`gf:node/webrtc_provider_peers.go` (the peer-gone queue),
`gf:cluster/spec/webrtctransportprovider/server_gen.go:254`, `:310-339`,
`gf:cluster/spec/webrtctransportrequestor/definition_gen.go:252`, `:269`,
`hm:ports.go:55-75`, `core:homeassistant/components/camera/webrtc.py:255-305`,
and, for context only, `internal/ha/subscription.go:40-160` (unit D's).

Verdicts that hold (no finding):

- **Decision 31's premise holds at v0.26.0** (M): the node queues a
  requestor proven lost (`gf:node/webrtc_provider_peers.go:81-91`, only
  `PeerGoneLost`) and ends its sessions on every provider of the topology
  through `Provider.PeerGone` → the port's `EndSession`
  (`gf:cluster/camera/provider_gone.go:41-60`), which is
  `Host.EndSession` (`camera.go:601-611`): the Home Assistant session is
  unsubscribed. A session the kit's table refuses is ended at the port too
  (`gf:provider.go:673-679` `abandonSession` → `Host.EndSession`).
- **No deadlock on the re-offer failure path** (M): `tell` → `Provider.End`
  runs inside `provideOffer`, which holds `p.op`
  (`gf:provider.go:159-165`); `End` takes only `p.mu` and `p.pub`
  (`gf:provider_peer.go:139-175`: `End` and `signal`; `gf:provider.go:222-243`: `change`), never `p.op`.
- **Ownership of a session id** is the kit's (M): ProvideOffer's re-offer,
  ProvideICECandidates and EndSession check the invoker's fabric and node
  (`matchingSession`, `gf:provider.go:551`, `:658`, `:696`) before the
  host is asked, so the host's id-keyed map cannot be driven across
  fabrics by those commands.
- **The controller → `call_service` path** (M): the entity id is
  `cmd.Entity.Key()`, the plan's (`command.go:96`), the domain its prefix
  (`:97`), services are constants per domain (`:37-86`, `:114-242`),
  numbers parsed (`number` refuses NaN and ±Inf, `:394-397`) and clamped
  in float (`clampInt`, `:424-434`); the four free texts (`hvac_mode`,
  `preset_mode`, `fan_mode`, `option`) are go-hamapping's rendering
  (T12, decision 9). A command for an entity that left the bridge is
  refused (`bridge/command.go:89-94`); a refused or failed one restores
  Home Assistant's state (`:45`, `:104`, `:126`).
- **The token** (M): sent only in `auth` (`client.go:375-379`) and as the
  stills' Bearer header (`ha/camera.go:165-167`); neither the dial
  (`client.go:192`, `:288`) nor the REST client (`:175`, `:184`) follows a
  redirect; dial errors are scrubbed (`:291`, `:859-872`); `auth_invalid`
  logs Home Assistant's text cut (`:390`).
- **The 256 KiB state cap** (decisions 42, 52) is per state object as it
  decodes (`message.go:167-192`), so it holds for `get_states` and, through
  `StateChanged`'s two `*State`, for each state of a `state_changed`
  (M for the decoder; the event path is unit D's).
- **Core's offer order** (M): `core:camera/webrtc.py:274-292` registers the
  teardown, sends the result, then the `session` event; the client registers
  the stream before the write (`client.go:539-542`), so no event is lost and
  `unsubscribeLate` (`:600-614`) covers a caller that gave up.
- **Files** (M): `nodes.json` and `passcode.json` are written 0600 by
  `os.CreateTemp`, synced and renamed (`nodes.go:532-557`), tightened when
  read wider (`:188-202`), keys validated against the contract's pattern
  and kept inside the directory (`:164-181`, `:318-329`; `FuzzOpen` holds
  it); `selection.json` likewise (`selection.go:366-389`), with the
  last-good copy and lenient read of decision 25.

## Findings

Counts: 0 high · 0 medium · 1 low · 4 note. Held back: none.

### HR-P4-bound-20 — low — the eviction at `MaxSessions` takes a standing live view, of any fabric

- **Where:** `internal/camera/camera.go:452-476` (ProvideOffer's
  eviction), `:514-527` (`idlest`), `:130-140` (`MaxSessions`' comment),
  `:237-258` (the session has no fabric).
- **Claim:** at 8 sessions a new offer evicts the session idle longest once
  it has been idle for `EvictIdle` (1 min). Since R7 (decision 31) a
  session's `last` moves only while it signals (`:529-534`, `:579`,
  `:688`, `:710`, `:739`), and a live view whose peer connection stands
  signals nothing — the comment says so itself (`:253-256`). So every
  watched stream older than a minute is "idle", and the eviction does not
  distinguish the abandoned view it was meant for from one a person is
  watching. The session carries no fabric, so an offer of fabric B evicts
  fabric A's live view: one commissioned controller (R4, decision 11) can
  end other ecosystems' viewers by opening offers, and the ninth household
  viewer silently ends another. The two premises of decision 23
  (HR-P1-bound-02) moved: the controller that went away is now ended by
  go-fabric's peer-gone path (verdict above), and the kit at v0.26.0 has
  its own fabric-scoped bound, `MaxSessionsPerFabric`, "so one fabric
  cannot take every session of the camera"
  (`gf:cluster/spec/webrtctransportprovider/server_gen.go:332-339`,
  `gf:cluster/camera/provider.go:62-80`) — which go-hamapping's camera row
  does not set (`hm:ports.go:59-65` sets `ProviderPort` only).
- **Reference:** decision 23 (eviction ≥ 1 min idle), decision 31 (no idle
  end; the eviction stays), `gf:node/webrtc_provider_peers.go:81-91`.
- **M/I:** M (probe `TestProbeEvictionCrossesFabrics`: eight answered,
  quiet sessions of fabric 1; fabric 2's offer ends session 1 with reason
  12). The harm to a real viewer is I (a standing view signals nothing:
  the comment at `:253-256` and decision 31's reasoning).
- **Proposed fix (owner question 1):** keep `args.FabricIndex` in
  `session`; at the bound evict only a session of the offering fabric
  (its own abandoned view), else refuse with RESOURCE_EXHAUSTED as today.
  Optionally a per-fabric share (e.g. 4 of 8) in the host, or a
  go-hamapping issue to pass the kit's `MaxSessionsPerFabric` through the
  camera row (decision 9) — the host's cap stays the total.
- **Red test:** the probe in the appendix, inverted: fabric 2's offer at
  the bound is refused with RESOURCE_EXHAUSTED and no `end` is sent;
  a second case where fabric 1's own ninth offer still evicts fabric 1's
  idlest.

### HR-P4-bound-21 — note — `MaxCandidateBytes` bounds the candidate line only; `sdpMid` passes at any length

- **Where:** `internal/camera/camera.go:122-128`, `:730-733`, `:626-641`.
- **Claim:** HR-P2-bound-40 bounds "one of Home Assistant's ICE candidates
  on its way to the controller" by `len(ev.Candidate.Candidate)` only; the
  candidate's `sdpMid` is copied to the controller unchecked (`:629-632`).
  Neither the requestor's schema (`gf:…/webrtctransportrequestor/
  definition_gen.go:252`, `:269`: "min 1", decoded up to 2^31-1) nor the
  host bounds it. The transport bounds the signal in the end (UDP:
  `ErrPayloadTooLarge`; TCP: the large-message bound, I), and the session
  then ends — what a hostile integration (R5) could do with an `error`
  event anyway, so no new reach.
- **M/I:** M (probe `TestProbeSdpMidUnbounded`: a 64 KiB `sdpMid` reaches
  the signaller).
- **Proposed fix:** count `len(Candidate) + len(*SdpMid)` against
  `MaxCandidateBytes` (a media id is a short token, RFC 8843 §5.2,
  I), refused and logged as today; the same for the controller's
  candidates in `ProvideIceCandidates` for symmetry.
- **Red test:** the probe, as `TestHACandidatesAreBounded`'s third case.

### HR-P4-data-22 — note — `text.Unsafe` lets U+2028 and U+2029 through

- **Where:** `internal/text/text.go:27-37`.
- **Claim:** the rule (decision 41) drops C0, DEL, C1 and the bidi
  controls; LINE SEPARATOR U+2028 and PARAGRAPH SEPARATOR U+2029 are
  `Zl`/`Zp`, not `Cc`, so `Safe("Kitchen Light")` is true and `Clean`
  keeps it. Both break a line as LF does, which `Clean` turns into a space
  (`:55-56`), in a controller's label and in the pages. The log is safe
  (slog's text handler quotes).
- **M/I:** M (probe `TestProbeLineSeparators`); the rendering in a
  controller's app is I.
- **Proposed fix:** `Unsafe` also for `unicode.In(r, unicode.Zl,
  unicode.Zp)`; `Clean` already maps an unsafe space to `' '`
  (`unicode.IsSpace` holds for both). A `TestClean` and `TestSafe` row
  each; `FuzzClean` keeps holding.
- **Red test:** the probe.

### HR-P4-cite-23 — note — "go-fabric's kit bounds only the id space" is false at v0.26.0

- **Where:** `internal/camera/camera.go:132`.
- **Claim:** the comment says the kit bounds only the session id space
  (65 535). At v0.26.0 the kit has `ProviderConfig.MaxSessions` and
  `MaxSessionsPerFabric` (`gf:cluster/camera/provider.go:62-80`,
  `provider_cap.go:16-67`), reachable through the Declare
  (`gf:…/webrtctransportprovider/server_gen.go:314-339`); go-hamapping
  does not set them (`hm:ports.go:59-65`). Decision 34's follow-up (a
  session cap option in go-fabric's camera kit) has landed.
- **M/I:** M.
- **Proposed fix:** "the kit's own bounds (MaxSessions,
  MaxSessionsPerFabric) are not set by go-hamapping's camera row, so the
  host's is the one in force", with the citation at `go.mod`'s version;
  rewritten together with HR-P4-bound-20's fix.
- **Standing guard:** none beyond `make cite` (which checks existence, not
  content).

### HR-P4-doc-24 — note — `wss://` with a private certificate authority is not documented

- **Where:** `docs/operations.md:202-208`, `README.md:147`, `:170`;
  `internal/ha/client.go:192`, `:175` (the default transport).
- **Claim:** the docs send the operator to `wss://` ("with the certificate
  verified") and warn on `ws://` (`token_in_clear`), but say nothing for a
  Home Assistant behind a self-signed certificate or a home CA, the common
  stand-alone case; such an operator falls back to `ws://`. Both clients
  use the default transport, so Go's system roots apply, which on Linux
  honour `SSL_CERT_FILE` and `SSL_CERT_DIR` (standard library, I).
- **M/I:** M for the code and the docs; the environment variables are I.
- **Proposed fix:** one docs sentence: a private CA through
  `SSL_CERT_FILE`/`SSL_CERT_DIR` (the image: a mounted bundle and the
  variable); no `InsecureSkipVerify` option.
- **Standing guard:** none (docs).

## Earlier rows re-checked, no change

HR-P1-bound-02 / decision 23 — see HR-P4-bound-20 (premise moved, the
decision is the owner's). HR-P1-conc-03 / decision 31 — premise holds
(verdict above). HR-P2-bound-40 — completed by HR-P4-bound-21 (a gap, not
a wrong fix). HR-P2-data-31 / decision 41 — HR-P4-data-22 (two separators
outside the stated class). HR-P3-ha-09 — the fix is right; a reply of the
wrong type to a subscription that core took would leave core's teardown
until the connection closes (the stream is dropped without
`unsubscribe_events`, `subscription.go:124-127`), but core sends `result`
(`core:camera/webrtc.py:280`) and is trusted (decision 11): no finding.

## Not read

Nothing of the unit skipped. Outside the unit and not read:
`internal/ha/hatest/*` (unit D), `internal/ha/subscription.go` beyond
`:40-160` (unit D), `internal/camera/*_test.go` beyond the helpers and the
bound tests named in the appendix (not in the unit's file list), go-fabric's
`provider_gone.go` beyond `:14-60`.

## Probes

Copy `git archive c3a9770` in
`/tmp/claude-1000/-home-markus-Dokumente-GitHub/87ad4fd5-a123-4d9e-9e1b-3f61a436a6f1/scratchpad/p4/S-ha`.

| Probe | Package | Result |
| --- | --- | --- |
| `TestProbeEvictionCrossesFabrics` | `internal/camera` | FAIL (finding shown): `["end 1 12"]` |
| `TestProbeSdpMidUnbounded` | `internal/camera` | FAIL (finding shown): a 65 536-byte `sdpMid` reached the controller |
| `TestProbeLineSeparators` | `internal/text` | FAIL (finding shown): both `Safe` |
| existing tests, `-skip TestProbe` | `internal/camera`, `internal/text`, `internal/selection`, `internal/nodes` | pass |

## v0.27.0

`gh release view v0.27.0 -R thingfabric/go-fabric`, rows that touch a
call in this unit's files. None makes today's code wrong.

| Row | Call here | Lift |
| --- | --- | --- |
| *Changed* `cluster/camera`: `Provider.End` removes the session from CurrentSessions before it sends the requestor's End, also when the requestor cannot be reached or there is no `PeerConnector` (then `ErrNoPeerConnector`) (`gf27:cluster/camera/provider_peer.go:157-173`) | `internal/camera/camera.go:394` (`Close`), `:827` (`tell`, which serves the eviction `:474`, `end` `:812` and the failed re-offer `:486`) | **I** — no code or test change: both calls log the error at Debug and depend neither on the order nor on the error; the tests use a fake `Signaller` (`camera_test.go:181`, `close_test.go:32`). The comment `:815-817` ("the kit takes the session out of CurrentSessions whether or not the command arrives") becomes true also for the unreachable case; no edit needed |
| *Added*/*Deprecated* `Provider.EndLostPeer` for `PeerGone`; *Changed* `node`: `matter.camera.peer_gone` logged only when a session ended | none (the node calls it; the host is reached through `Host.EndSession`) | **I** — none; the comments `camera.go:136-138`, `:254-256` name no identifier |
| *Changed* `device`: a Declare's `MaxSessions`/`MaxSessionsPerFabric` beside a `HostServer`, or without a `ProviderPort`, is refused | none here; go-hamapping's camera row sets only `ProviderPort` (`hm:ports.go:59-65`) | **I** — none; relevant only if HR-P4-bound-20 is fixed through the kit's bound |
| *Fixed* `cluster/camera`: `Sessions()` shows a change before the listeners | none | — |
| Citation drift (not a release row): `docs/guides/camera.md`, `cluster/camera/provider.go`, `provider_peer.go` and `node/webrtc_provider_peers.go` changed; the ranges cited "at v0.26.0" move: `camera.go:52` `provider.go:680-695` → `:712-727` (maxEndReason/endSession), `:103` `:648-665` → `:680-697`, `:202` `:597-601` → `:629-633`, `:439` `:159-165` → `:172-178`, `:187` `provider_peer.go:104-148` (moved, `Offer` now `:106`, `End` `:157`), `:22` `camera.md:133-140` → `:137-…`, `:15` `camera.md:93-110` (start unchanged, end to re-read), `:348` `webrtc_provider_peers.go:30-51` (end moved). Unchanged files: `sessionparams.go`, `avstream.go`, `levelcontrol/values.go`, `keypadinput/definition_gen.go`, `im/status.go`, `webrtctransportprovider/definition_gen.go`, `chime.go`, `state.go` | the comments listed | **M** — comment re-pins in the lift (decision 61). `make cite` will not catch them: it checks that the file and lines exist, and every shifted range still exists (unit D owns the guard) |

## Questions for the owner

1. **HR-P4-bound-20, the eviction at the camera's bound.** Recommendation:
   **evict only within the offering fabric, else refuse** — one fabric's
   controller can then neither end another ecosystem's live view nor be
   locked out by an abandoned view of its own (it evicts its own). It
   is the better design than today's: since go-fabric's peer-gone path
   ends lost controllers, the eviction's only remaining target is an
   abandoned view, and that is always the offering fabric's to give up.
   Alternatives: (b) keep decision 23 as is (accepted, a docs sentence
   that a ninth viewer may end another's view); (c) a go-hamapping issue
   for the kit's `MaxSessionsPerFabric` in the camera row (decision 9),
   later, beside (a).

## Appendix — probe texts

`internal/camera/p4_probe_test.go`:

```go
// Phase 4 S-ha probes (not part of the tree).

package camera_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"thingfabric.dev/go-ha2fabric/internal/camera"
)

// TestProbeSdpMidUnbounded: MaxCandidateBytes bounds the candidate line
// only; Home Assistant's sdpMid reaches the controller at any length.
func TestProbeSdpMidUnbounded(t *testing.T) {
	up := &upstream{}
	h := boundedHost(t, up, camera.Options{})
	sig := &signaller{}
	h.Attach(sig)
	if _, err := h.ProvideOffer(context.Background(), offer(5)); err != nil {
		t.Fatal(err)
	}
	st := up.stream(0)
	st.push(session("ha-1"))
	mid := strings.Repeat("M", 64*camera.MaxCandidateBytes)
	st.push(candidate("candidate:1", mid, 0))
	eventually(t, "the candidate", func() bool {
		return slices.ContainsFunc(sig.seen(), func(s string) bool { return strings.HasPrefix(s, "candidate 5 candidate:1") })
	})
	for _, s := range sig.seen() {
		if strings.Contains(s, mid) {
			t.Fatalf("a candidate with a %d-byte sdpMid reached the controller (MaxCandidateBytes %d)", len(mid), camera.MaxCandidateBytes)
		}
	}
}

// TestProbeEvictionCrossesFabrics: at MaxSessions a new offer of fabric 2
// evicts a standing (answered, quiet) live view of fabric 1.
func TestProbeEvictionCrossesFabrics(t *testing.T) {
	up := &upstream{script: func(s *stream) {
		s.push(session("ha"))
		s.push(map[string]any{"type": "answer", "answer": "v=0 answer"})
	}}
	h := boundedHost(t, up, camera.Options{EvictIdle: 20 * time.Millisecond})
	sig := &signaller{}
	h.Attach(sig)
	ctx := context.Background()
	for i := range camera.MaxSessions {
		if _, err := h.ProvideOffer(ctx, offer(uint16(i+1))); err != nil { // FabricIndex 1
			t.Fatal(err)
		}
	}
	eventually(t, "the answers", func() bool {
		n := 0
		for _, s := range sig.seen() {
			if strings.HasPrefix(s, "answer ") {
				n++
			}
		}
		return n == camera.MaxSessions
	})
	time.Sleep(50 * time.Millisecond) // standing live views signal nothing
	other := offer(100)
	other.FabricIndex, other.PeerNodeID = 2, 0x99
	if _, err := h.ProvideOffer(ctx, other); err != nil {
		t.Fatalf("fabric 2's offer: %v", err)
	}
	eventually(t, "an end", func() bool {
		return slices.ContainsFunc(sig.seen(), func(s string) bool { return strings.HasPrefix(s, "end ") })
	})
	t.Fatalf("fabric 2's offer evicted a standing live view of fabric 1: %q", slices.DeleteFunc(sig.seen(), func(s string) bool { return !strings.HasPrefix(s, "end ") }))
}
```

Output: `fabric 2's offer evicted a standing live view of fabric 1:
["end 1 12"]`; `a candidate with a 65536-byte sdpMid reached the
controller (MaxCandidateBytes 1024)`.

`internal/text/p4_probe_test.go`:

```go
package text_test

import (
	"testing"

	"thingfabric.dev/go-ha2fabric/internal/text"
)

// TestProbeLineSeparators: U+2028 and U+2029 break a line as LF does.
func TestProbeLineSeparators(t *testing.T) {
	for _, s := range []string{"Kitchen Light", "Kitchen Light"} {
		if text.Safe(s) {
			t.Errorf("%q is Safe; Clean gives %q", s, text.Clean(s))
		}
	}
}
```

Output: `"Kitchen Light" is Safe; Clean gives "Kitchen Light"`
(and the same for U+2029).
