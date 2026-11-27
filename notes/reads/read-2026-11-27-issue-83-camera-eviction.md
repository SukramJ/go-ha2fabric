# go-ha2fabric #83 — camera session bounds and eviction through the kit (plan, read-only)

Read on 2026-11-27. Pins: go-ha2fabric `origin/main` = `04c205a`; go-fabric
`v0.28.0` and `v0.27.0` (module cache); go-hamapping `v0.9.0` (module cache,
`sukramj.dev/go-hamapping@v0.9.0`). Short names: `gf:` = go-fabric at v0.28.0,
`hm:` = go-hamapping at v0.9.0, `h2f:` = go-ha2fabric at `origin/main`.
Everything not marked **unverified** was read at the line given.

## 0. Verdict

- **Not yet.** go-fabric v0.28.0 has what #83 waited for, but go-hamapping
  v0.9.0 (the latest release, and `main` = the v0.9.0 commit) passes only
  `MaxSessions` / `MaxSessionsPerFabric`. A go-hamapping change comes first
  (section 2). go-ha2fabric's `CLAUDE.md:132-133` says go-hamapping changes
  are "made directly, each in its own pull request" — the text in section 2
  is written so it serves as the issue or as that pull request's brief.
- **After it: yes**, the host can pass the bounds and keep an eviction that is
  equivalent within a fabric (same victim rule, same threshold), with four
  visible differences (section 1.3). Decision 76's third-fabric lock-out is
  solved by `EvictAcrossFabrics`, at the price of reversing one sentence of
  decision 72 — an owner decision (Q1).

## 1. Facts

### 1.1 go-fabric v0.28.0

| Fact | Where |
| --- | --- |
| `Declare.MaxSessions`, `MaxSessionsPerFabric`, `EvictIdle time.Duration`, `EvictAcrossFabrics bool` | `gf:cluster/spec/webrtctransportprovider/server_gen.go:316-333, 334-341, 342-355, 356-366` |
| The device copies all four into `camera.ProviderConfig`, plus `Clock: b.clock`; negative bound → `camera.ErrNegativeMaxSessions`, negative `EvictIdle` → `camera.ErrNegativeEvictIdle`, both judged before the port | `gf:device/defaults_camera.go:149-163` |
| A non-zero plain field of a Declare counts as a kit setting ("port"): `EvictIdle`/`EvictAcrossFabrics` without a `ProviderPort` fail the build like a bound does | `gf:device/options.go:288-315` (rule: `:310`), refusal `gf:device/device.go:492-496` (both files byte-identical to v0.27.0) |
| Eviction is on only with `EvictIdle > 0` beside a bound | `gf:cluster/camera/provider_cap.go:53-55` |
| Admission: below both bounds → admitted; at a bound without eviction → refused; with eviction, first (only at `MaxSessions`, with `EvictAcrossFabrics`, claim not spent, fabric under its share) the longest-idle entry of a fabric **over** its share, else the accessing fabric's own longest-idle entry; an entry is taken only when its activity is `EvictIdle` old and its going makes room | `gf:cluster/camera/provider_cap.go:97-139`, `idlestLocked` `:193-210` |
| Share = `max(1, MaxSessions / n)`, capped by `MaxSessionsPerFabric`; n = fabrics holding an entry, plus the accessing one when it holds none | `gf:cluster/camera/provider_cap.go:178-188` |
| Spent claim: a fabric whose offer failed after it took another fabric's entry takes only its own for `EvictIdle` or until an offer of it is stored | `gf:cluster/camera/provider_cap.go:145-172` |
| Evict: entry leaves CurrentSessions, then `port.EndSession(ctx, id, OutOfResources)` (its error logged, not returned), then the requestor's `End` with OutOfResources through the End queue | `gf:cluster/camera/provider_cap.go:243-258` (the End's send below `:259`, **unverified** past `:260`; stated in the comment `:234-242`) |
| Activity ("signalling"): commit, accepted re-offer, `ProvideAnswer`, `ProvideICECandidates` after the port accepted, and the camera's `Offer`/`Answer`/`IceCandidates` | `gf:cluster/camera/provider.go:92-122` (doc), touches at `:753, :788, :811`, `gf:cluster/camera/provider_peer.go:195-197`, new entries `gf:cluster/camera/provider_cap.go:60-76` |
| ProvideOffer: admission (`:723-728`) runs before the session id and before `port.ProvideOffer` (`:734`); the session is stored after the port returned (`:741`) | `gf:cluster/camera/provider.go:723-754` |
| **SolicitOffer admits (and so evicts) before the port is asked** | `gf:cluster/camera/provider.go:612-618` (admission), `:628-631` (port, failure → `settleClaim`) |
| Fabric removal and a lost peer end sessions through `port.EndSession` | `gf:cluster/camera/fabric_state.go:60`, `gf:cluster/camera/provider_gone.go:74` |
| A racing `Provider.End` of an evicted session sends nothing (`ErrUnknownSession`) | `gf:cluster/camera/provider_peer.go:164-175` |
| Guide: "With 8 sessions and 4 per fabric, two fabrics never cross (the share is 4); a third fabric's offers bring the table to 3, 3 and 2." | `gf:docs/guides/camera.md:462-464`; eviction section `:412-464` |
| Changelog entry | `gf:changelog.md:19` |
| v0.28.0: "No source change required: `make apidiff` reports no incompatible change against `v0.27.0`" | `gf:changelog.md:15` |

### 1.2 go-hamapping v0.9.0

| Fact | Where |
| --- | --- |
| `WebRTCSessionBounds() (total, perFabric int)`, optional interface of the `Options.Ports` object | `hm:ports.go:56-71` |
| `portsWebRTCProvider` sets `ProviderPort` and, when implemented, only `d.MaxSessions, d.MaxSessionsPerFabric`; nothing else of the Declare | `hm:ports.go:73-88` |
| No `EvictIdle` / `EvictAcrossFabrics` anywhere in the module (grep over the module: no hit) | `hm:` whole tree |
| Requires go-fabric v0.27.0 | `hm:go.mod:9` |
| Latest version is v0.9.0 (`go list -m -versions`: … v0.8.0 v0.9.0; GitHub tags v0.9.0 first; `main` head `009dc216` is the v0.9.0 commit; no open issue) | command output, 2026-11-27 |
| `Options.Ports func(e model.Entity) any` is the only way a host reaches the camera Declare; no declaration hook was found in `options.go` | `hm:plan.go:171-179`; "no hook" is from a grep of `options.go`, **not a full read** |
| Tests and docs of the bounds: `TestCameraSessionBounds`, `rows/camera.yaml:5, :25`, `docs/rows.md:216-218`, `changelog.md:51-57` | `hm:rows_camera_test.go:195-224` |

### 1.3 go-ha2fabric today, against the kit's eviction

Host today (`h2f:internal/camera/camera.go`): constants `MaxSessions = 8`
(`:154`), `MaxSessionsPerFabric = MaxSessions / 2` (`:167`),
`DefaultEvictIdle = time.Minute` (`:173`); check and eviction `:485-508`;
`held` `:546-555`, `idlest` `:557-570`, `touch` `:572-577` called at
`:747, :769, :798`; `session.fabric` `:268-269`, `session.last` `:281-287`;
`ErrTooManySessions` `:97-100`; `Options.EvictIdle` `:237-240`, default
`:332-334`. The bridge passes no `EvictIdle` (`h2f:internal/bridge/camera.go:35`),
so production is always one minute.

| | Host today | Kit, `EvictIdle` set, `EvictAcrossFabrics` false |
| --- | --- | --- |
| When | new session, total ≥ 8 or fabric ≥ 4 (`:486`) | same condition (`provider_cap.go:108-111`) |
| Victim | the offering fabric's oldest `last`, if ≥ `EvictIdle` old (`:559-570`) | the accessing fabric's oldest activity, if ≥ `EvictIdle` old (`:134-137, :193-210`) |
| Another fabric | never (`:487`) | never (`:126` needs the flag) |
| Else | RESOURCE_EXHAUSTED before Home Assistant (`:488-493`) | refused before the port (`provider.go:726-727`) |
| Home Assistant's session | `h.unsubscribe` before the new offer (`:501`) | `port.EndSession` → `Host.EndSession` unsubscribes (`camera.go:651-661`) before `port.ProvideOffer` |
| **End reason** | `ReasonUnknown` 12 (`:506`) | **OutOfResources** (decision 703, `provider_cap.go:50, :256`) |
| **Log** | `camera.session_evicted` Info (`:499`) | host: `camera.session_ended reason=…` Info (`:657`); kit: `camera.provider.evicted` at Debug (`provider.go:151-156`) |
| **Activity** | session creation, Home Assistant's session/answer/candidate events it acts on | commit, the camera's Answer/IceCandidates signals, **plus** the controller's accepted `ProvideICECandidates` |
| **SolicitOffer at a bound** | no eviction; the port answers INVALID_IN_STATE (`:453-455`) | **evicts first, then the port refuses**; without an evictable entry the kit refuses itself (`provider.go:612-618`) |

The relay of an evicted session does not send a second End: after
`Host.EndSession` removed the session, `drop` returns false (`:893-904`) and
`end` returns (`:867-872`) — the path a controller's own EndSession takes today.

**Residual to state, not to fix here:** a SolicitOffer — a command this host
never serves — costs its fabric's idlest view when that fabric is at a bound.
With `EvictAcrossFabrics` it can cost another fabric's borrowed view; that is
self-limiting, since the failed solicit leaves the table one under
`MaxSessions`, so the next one is not at the bound, and the spent claim
(`provider_cap.go:145-172`) holds for `EvictIdle`. Whether any ecosystem sends
SolicitOffer to this camera is **unverified**.

## 2. go-hamapping change first — issue / pull-request text

> **Camera row: pass the kit's `EvictIdle` and `EvictAcrossFabrics` from the host (go-fabric v0.28.0)**
>
> Follow-up of #50. go-fabric v0.28.0 lets the kit's WebRtcTransportProvider
> evict at its session bounds (thingfabric/go-fabric#634):
> `webrtctransportproviderdef.Declare.EvictIdle` and `EvictAcrossFabrics`
> (`cluster/spec/webrtctransportprovider/server_gen.go:342-366` at v0.28.0),
> copied into `camera.ProviderConfig` by the device
> (`device/defaults_camera.go:149-163` at v0.28.0). The camera row sets only
> `ProviderPort`, `MaxSessions` and `MaxSessionsPerFabric`
> (`ports.go:73-88` at v0.9.0), so a host that passes its bounds through
> `WebRTCSessionBounds` gets the kit's refusal and loses its own eviction —
> the kit refuses before the port is asked
> (`cluster/camera/provider.go:723-728` at v0.28.0). thingfabric/go-ha2fabric#83
> waits for this.
>
> **Shape (additive).** A second optional interface of the `Options.Ports`
> object, beside `WebRTCSessionBounds`:
>
> ```go
> // WebRTCSessionEviction is the eviction at the session bounds:
> // idle is Declare.EvictIdle, acrossFabrics Declare.EvictAcrossFabrics.
> type WebRTCSessionEviction interface {
> 	WebRTCSessionEviction() (idle time.Duration, acrossFabrics bool)
> }
> ```
>
> `portsWebRTCProvider` copies both unchanged into the declaration, only
> together with the host's `ProviderPort` (it already returns before the
> bounds when the port is missing). No relation is checked, as go-fabric
> checks none: `EvictIdle` without a bound and `EvictAcrossFabrics` without
> `EvictIdle` and `MaxSessions` are inert in the kit
> (`cluster/camera/provider_cap.go:53-55`, `cluster/camera/provider.go:142`
> at v0.28.0). A negative `idle` fails `Plan.Build` with a `*BuildError`
> wrapping `camera.ErrNegativeEvictIdle` (`device/defaults_camera.go:154-156`
> at v0.28.0).
>
> **Scope.** go-fabric v0.27.0 → v0.28.0 (`go.mod:9`); `ports.go`; the row's
> comment and cite (`rows/camera.yaml:5, :25` → `server_gen.go:316-366` at
> v0.28.0) and `make` of `rows_gen.go`; `docs/rows.md:216-218`; changelog;
> the `ports.go` comment's v0.27.0 cites re-pinned (`device/options.go:288-315`
> and `device/device.go:492-496` are byte-identical at v0.28.0;
> `defaults_camera.go:147-151` → `:149-156`, `:147-160` → from `:149`, read
> to `:166`, the old end maps to about `:168` — to be read when written;
> `server_gen.go:314-339` → `:316-341`).
>
> **Tests** (beside `TestCameraSessionBounds`, `rows_camera_test.go:195-224`):
> a host with both interfaces → the Declare carries the four values; eviction
> only, no bounds → the two values are copied, the bounds stay 0; eviction
> without a `ProviderPort` → the camera is unserved, no build error (as for
> bounds only); `idle < 0` → `errors.Is(err, camera.ErrNegativeEvictIdle)` in
> a `*BuildError`.
>
> **Acceptance.** Additive, a minor release (v0.10.0); a host implementing
> neither interface builds the same declaration as at v0.9.0.
>
> **Not in scope.** No default and no policy in go-hamapping: which bounds,
> which idle time and whether fabrics cross is the host's.

Alternative shape, not recommended: widen `WebRTCSessionBounds` to return a
struct — a breaking change of an interface one day old. Whether any consumer
other than go-ha2fabric implements `WebRTCSessionBounds` is **unverified**
(go-ha2fabric does not: the name occurs only in comments, `camera.go:145, :165`).

## 3. The change in go-ha2fabric (after go-hamapping v0.10.0)

### 3.1 Dependencies

`go.mod`: `thingfabric.dev/go-fabric v0.27.0 → v0.28.0`,
`sukramj.dev/go-hamapping v0.9.0 → v0.10.0`; `licenses/` regenerated
(`h2f:licenses/modules/sukramj.dev/go-hamapping/…` exists). go-fabric-ui
stays or moves to a release on v0.28.0 (v0.14.2 or later) — not needed for
#83. The repository cites `v0.27.0` in about 20 files outside `notes/`
(`git grep -c`); #83 re-pins the camera ones only (3.3), the rest is the
bump's own sweep — which pull request carries it is the coordinator's call.

### 3.2 `internal/camera/camera.go`

1. **Add** on `*Host`:
   `WebRTCSessionBounds() (total, perFabric int)` returning
   `MaxSessions, MaxSessionsPerFabric`, and
   `WebRTCSessionEviction() (time.Duration, bool)` returning
   `h.opts.EvictIdle` and the Q1 value; compile-time assertions against both
   go-hamapping interfaces. `dryHost` (`:303`) is a `*Host` and so declares
   the same in previews (`:305-316`) — intended, the preview is the plan.
2. **Remove** the host's share: the check and eviction `:485-508` (keep
   `old`'s handling `:483-484, :509-511`), `held` `:546-555`, `idlest`
   `:557-570`, `touch` `:572-577` and its calls `:747, :769, :798`,
   `session.fabric` `:268-269` and `session.last` `:281-287` (and
   `fabric:`/`last:` at `:522`), `ErrTooManySessions` `:97-100`.
   `Options.EvictIdle` (`:237-240`, default `:332-334`) and
   `DefaultEvictIdle` (`:171-173`) stay as the value handed to the kit.
   The comment at `:741-743` ("Only an event it acts on is activity",
   HR-P2-bound-40) changes its reason: the host signals only for events it
   acts on, and the kit counts the signal (`provider_peer.go:195-197`).
3. **Keep** `EndSession` `:651-661` as is; it is now also the eviction's
   path. Optional (Q3): log `camera.session_evicted` there when the reason is
   OutOfResources — not recommended, a controller may send that reason itself.
4. **Rewrite the comments** (Fable note → Opus text, per the model table):
   - `MaxSessions` `:140-153` and `MaxSessionsPerFabric` `:156-166`: the
     bounds and the eviction are the kit's, passed through go-hamapping's
     `WebRTCSessionBounds` and `WebRTCSessionEviction` (lines at v0.10.0,
     to be read then); cite `gf:cluster/camera/provider.go:70-91` (bounds),
     `:92-143` (eviction), `gf:cluster/camera/provider_cap.go:97-139`
     (admission); state OutOfResources, signalling-idle, and the SolicitOffer
     residual of 1.3.
   - `ProvideOffer` `:460-476`: the cap sentence goes; what remains is that a
     slow Home Assistant holds the camera's other commands — the kit's
     serialisation, now `gf:cluster/camera/provider.go:253-259`,
     `cluster/camera/state.go:13-28` (file unchanged).

### 3.3 Camera cites re-pinned to v0.28.0

Mapped through `diff v0.27.0 v0.28.0` and each target read, unless noted.

| `h2f:` line | Cite at v0.27.0 | At v0.28.0 |
| --- | --- | --- |
| `camera.go:22-23` | `docs/guides/camera.md:137-144` | same lines (the file differs from `:410` on only) — target text **not re-read** |
| `camera.go:52` | `cluster/camera/provider.go:712-727` (end reason, highest value) | `:828-843` |
| `camera.go:71` | `im/status.go:94` | same (file identical) |
| `camera.go:106` | `…/webrtctransportprovider/definition_gen.go:721` | same (file identical) |
| `camera.go:107` | `provider.go:680-697` (ProvideICECandidates unbounded) | `:795-813` |
| `camera.go:144` | `provider.go:65-86` | `:70-91` |
| `camera.go:146` | go-hamapping `ports.go:56-88` at v0.9.0 | at v0.10.0, **to be read** |
| `camera.go:148` | `provider.go:518, :621-623` | `:612-618, :723-728` (rewritten anyway, 3.2) |
| `camera.go:214` | `provider_peer.go:102-173` | `:102-186` (start read, `End` read `:164-186`; `:105-163` **not re-read**) |
| `camera.go:229` | `provider.go:629-633` | `:734-741` |
| `camera.go:308` | go-hamapping `plan.go:171-179` at v0.9.0 | at v0.10.0, **to be read** |
| `camera.go:378` | `node/webrtc_provider_peers.go:30-51` | same (file identical) |
| `camera.go:452` | `cluster/camera/chime.go:302-311` | same (file identical) |
| `camera.go:470-471` | `provider.go:172-178`, `state.go:13-28` | `:253-259`; same |
| `internal/ha/camera.go:83, :85` | `secure/sessionparams/sessionparams.go:44`, `cluster/camera/avstream.go:79-85` | same (files identical) |
| `docs/by_design.md:142` | `docs/guides/camera.md:581-603` | `:635-657` |
| `bridge_r6_e2e_test.go:56` | `node/webrtc_provider_peers_test.go` at v0.27.0 | version only — file **not compared** |

### 3.4 Tests

- `internal/camera/bounds_test.go` — the host no longer bounds or evicts:
  `TestSessionsPerCameraAreBounded` (`:67`), `TestIdleSessionEvictedAtTheBound`
  (`:98`), `TestQuietSessionStays` (`:133`), `TestIgnoredEventsDoNotKeepTheSession`
  (`:360`), `TestEvictionStaysInTheFabric` (`:425`),
  `TestEvictionTakesTheFabricsOwnIdlest` (`:455`), `TestFabricShareRefused`
  (`:488`) leave the host's package. Their policy is go-fabric's
  (`cluster/camera/provider_evict_test.go`, `provider_cap_test.go` at v0.28.0
  — existence read, contents **unverified**). `close_test.go:37-45` fills a
  host to `MaxSessions` directly and stays valid.
- New in `internal/camera`: the host returns `(8, 4)` and
  `(DefaultEvictIdle, <Q1>)`, and `Options.EvictIdle` reaches the second;
  `EndSession` with OutOfResources unsubscribes Home Assistant's session and
  the ended relay sends no End of its own.
- New in `internal/bridge` (the wiring, through the kit; pattern
  `TestCameraOfferRefusedEndToEnd`, `bridge_r6_e2e_test.go:183-212`): one
  controller fills its fabric's 4, the fifth offer is answered
  RESOURCE_EXHAUSTED, `ha.Offers()` holds 4, CurrentSessions 4. An eviction
  end-to-end needs either a short `EvictIdle` through the bridge (today it
  passes only the logger, `internal/bridge/camera.go:35`) or the device's
  clock moved; whether `fabrictest.Options.Clock` reaches the device's clock
  is **unverified**, as is a second fabric in `fabrictest`. Recommended: a
  test-only bridge option for `EvictIdle`, the own-fabric eviction end-to-end,
  the cross-fabric rule left to go-fabric's tests.
- A kit-level test inside `internal/camera` (a `camera.NewProvider` over the
  host) is not recommended: the accessing fabric comes from `im`'s context
  (`gf:cluster/camera/provider.go:389-395`), below the facade.

### 3.5 Docs

- `docs/operations.md:448-460` (Cameras): the bounds are go-fabric's; the
  evicted controller gets `End` with OutOfResources (was "reason 12"); the log
  key (`camera.session_ended`, or Q3); "no command and no Home Assistant event
  for a minute" becomes signalling in either direction; with Q1 = yes, the
  sentence "another fabric's session is never ended for it" (`:453-454`)
  becomes the share rule (a fabric keeps `8 / fabrics holding sessions`, at
  most 4; beyond that a view idle for a minute can go to a fabric under its
  share); `:459-460` likewise.
- `docs/by_design.md:146-156`: same facts; `:154-156` ("nor end another
  ecosystem's live view") changes with Q1 = yes. The matter-hub cites there
  are untouched and were **not re-read**.
- `changelog.md` `[Unreleased]`: Changed — bounds and eviction moved to
  go-fabric's kit; End reason OutOfResources; Q1's outcome; SolicitOffer at a
  bound; go-fabric v0.28.0, go-hamapping v0.10.0.
- `notes/review/2026-11-23/decisions.md`: decision 72's "after which the
  host's share goes" is fulfilled (`:178`); decision 76 (`:187`) closed by
  Q1's answer. Register `phase-4-register.md:54` gets the closing commit.

## 4. Owner decisions

**Q1 — `EvictAcrossFabrics`: on or off? (decision 76 against decision 72)**
Off keeps today's behaviour exactly: no fabric ever ends another's session,
and two ecosystems at 4 each lock a third out until one of them offers again
or its controller is proven lost. On removes the lock-out: with 8 and 4, two
fabrics never cross (share 4); a third takes the longest-idle entries beyond
the others' share until 3, 3, 2 (`gf:docs/guides/camera.md:462-464`); an entry
within a share is never taken. The price: idle is signalling-idle, so the
entry taken may be a view someone is watching
(`gf:cluster/camera/provider.go:131-134`) — the thing HR-P4-bound-20 was
raised against, now limited to entries beyond a share.
**Recommendation: on.** The lock-out is unbounded in time and hits a whole
ecosystem; the cost is bounded (at most the entries beyond a share, oldest
signalling first, one per offer, the controller is told OutOfResources and can
offer again). It is the "fair-share eviction" decision 76 named, and go-fabric's
owner decision 705 took the same side. It needs the docs sentence of 3.5
reversed in the open.

**Q2 — go-hamapping shape: a second optional interface or a widened
`WebRTCSessionBounds`?** **Recommendation: the second interface**
(`WebRTCSessionEviction`, section 2): additive, v0.10.0, and a host can bound
without evicting.

**Q3 — the evicted session's reason and log.** The kit sends OutOfResources
where the host sent 12, and the host's own line becomes
`camera.session_ended reason=<OutOfResources>` instead of
`camera.session_evicted`. **Recommendation: accept both**, document them in
`operations.md`; no extra log line in `Host.EndSession`.

**Q4 — SolicitOffer at a bound evicts before this host refuses it** (1.3).
**Recommendation: accept and document** in the `MaxSessionsPerFabric` comment
and `operations.md`; ask go-fabric only if a live test shows an ecosystem
soliciting. (A kit option "this port serves no SolicitOffer" would be a new
go-fabric issue.)

**Q5 — `EvictIdle` stays one minute?** go-fabric warns below 30 s a session
still being set up can be taken (`gf:cluster/camera/provider.go:115-120`).
**Recommendation: keep `DefaultEvictIdle = time.Minute`.**

**Q6 — order of work.** **Recommendation:** go-hamapping pull request and
v0.10.0 first; then one go-ha2fabric pull request carrying both bumps and #83
(the go-fabric v0.28.0 bump alone needs no source change, `gf:changelog.md:15`).

## 5. Not read

- go-hamapping `options.go` in full (only a grep for a declaration hook).
- go-fabric `provider_cap.go:260-345` (the End queue, `refuseSolicitOffer`
  past `:304`), `provider_evict_test.go`, `provider_cap_test.go`.
- go-ha2fabric: `bounds_test.go` bodies (function list and doc comments
  only), `close_test.go` past `:45`, how the node's clock reaches the device.
- Whether Apple Home or any ecosystem sends SolicitOffer, or abandons views
  without EndSession.
