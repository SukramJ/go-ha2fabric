# Phase 2 fix wave — F2 (camera, adapt, client, daemon edges)

Implementer F2 (Opus). Brief `notes/briefs/2026-11-24-review-p2-fix.md`,
section F2. Worktree `go-ha2fabric-review-F2`, branch `review/p2-F2`, base
`d6003b4`.

## Commits

| Commit | Row |
| --- | --- |
| `cfa014b` | fix(camera): end a host's sessions in parallel within the caller's deadline (HR-P2-conc-60) |
| `fef16e7` | fix(camera): bound Home Assistant's candidates per session and per candidate; ignored events are no activity (HR-P2-bound-40) |
| `37b0949` | fix(adapt): send a select's option, a fan's preset and a climate's fan mode untrimmed (HR-P2-data-32) |
| `ecc5a5e` | fix(adapt): drop control and bidirectional control characters from Home Assistant's text, refuse them in names and labels, quote --dump-plan (HR-P2-data-31) |
| `05cd908` | fix(ha): a state over 256 KiB decodes without its attributes and is logged (HR-P2-bound-38) |
| `dae6247` | fix(ha): name a message over the read limit in the error the sync reports (HR-P2-bound-37) |
| `09953b8` | fix(ha): cut Home Assistant's error text to 512 bytes before it reaches the log ring and the status (HR-P2-bound-39) |
| `1148829` | fix(cmd): the Supervisor gate keeps re-reading the version after it opened; a downgrade closes it (HR-P2-cfg-03) |
| `fb8789b` | test(adapt): write the format characters of HR-P2-data-31's tests as escapes (ST1018) |
| (this file) | review: phase 2 fix report F2 |

## Guards

| Guard | HEAD | Result |
| --- | --- | --- |
| `make race` | `1148829` | exit 0 |
| `make cover-check` | `1148829` | exit 0 |
| `make ci` | `1148829` | exit 2 — lint ST1018 only (format characters written literally in four test files); fixed by `fb8789b`, `golangci-lint run ./...` 0 issues there |
| `make ci` | `fb8789b` | exit 0 |
| `make fuzz-smoke` | `fb8789b` | exit 0 |

`fb8789b` changes test literals only (the same runes as escapes), so
race and cover-check were not run again on it. Skipped: `make docker
addon-image`, `make release` (no manifest, version or changelog change),
`make vuln licenses secrets` (no dependency change).

## Per row

### HR-P2-conc-60 (low) — fixed, the camera half

`camera.Host.Close(ctx)`: every session's unsubscribe and `End` go out at
once, all bounded by one deadline — `SignalTimeout` from now, or ctx's
deadline when earlier (`WithoutCancel`, so an ended ctx still sends);
Close returns by that deadline, logging `camera.close_overrun` at warn
when work was abandoned (a relay's last signal, a still). The signature
is unchanged (`Close(ctx)`), so F1's `stopRun` and moved closes need
nothing from here. Tests `TestCloseHonoursItsDeadline` (8 sessions of a
gone controller, `SignalTimeout` 2 s, ctx 100 ms: red at 16.0 s) and
`TestCloseEndsSessionsInParallel` (ended ctx, 300 ms: red at 2.4 s); red
seen.

### HR-P2-bound-40 (low) — fixed

Home Assistant's candidates count per session against
`MaxSessionCandidates` (256, refused or relayed); the next one ends the
session with ICE failed (`camera.candidates_exceeded`). A candidate over
`MaxCandidateBytes` (1024, new) is refused and logged
(`camera.candidate_refused` with `bytes`, `max`); the session stays. Only
an event the relay acts on (session, answer, a relayed candidate) touches
the session; an ignored, refused or end-of-candidates event does not put
off the idle end. Tests `TestHACandidatesAreBounded`,
`TestIgnoredEventsDoNotKeepTheSession`; red seen (no end; ignored events
kept the session).

### HR-P2-data-31 (low, closes HR-P1-data-36) — fixed

- New leaf package `internal/text` (`Unsafe`, `Safe`, `Clean`): control
  characters (C0, DEL, C1) and the bidi controls U+061C, U+200E, U+200F,
  U+202A–U+202E, U+2066–U+2069; U+200C/U+200D kept. `internal/arch`
  registers it (allowed for adapt, selection, nodes, config, cmd) — the
  three users cannot import each other, so one rule in one place instead
  of three copies.
- `internal/adapt`: `Device` (name, manufacturer, model, model id,
  versions, serial, area), `OwnDevice`'s name, `entityName` (registry
  name, original name, friendly_name) and the web UI name are
  `text.Clean`ed. A select's options are not touched (they go back to
  Home Assistant byte for byte; the label half stays the go-hamapping
  candidate).
- Refused: `selection.Set` (a web UI name that is not `text.Safe`);
  `nodes.Registry.Allocate` and `Rename` (wrapping
  `fabricui.ErrInvalidArgument`); `LABEL` now also refuses bidi controls.
  A name read from `selection.json` is cleaned after the cut (the fuzz
  round trip holds).
- `--dump-plan`: the table is `writePlan`; a cell that is not
  `text.Safe` is printed quoted (`%q`), the others as they are.
- Standing guards: `FuzzEntityState` (in `fuzz-smoke`) now asserts the
  entity's and the device's names are `text.Safe`; `selection`'s
  `FuzzOpen` asserts the names are; `internal/text` has `FuzzClean` (not
  in `FUZZ_TARGETS` — `Makefile` is shared with F1's test-42 edit; see
  questions).
- Tests `TestHostileText` (adapt), `TestNameText` (selection),
  `TestLabelText` (nodes; config's extended), `TestPlanTableQuotes`
  (cmd; `writePlan` was first extracted unchanged, then the test ran
  red on U+001B), `TestClean`, `TestSafe`; red seen for all but the
  `internal/text` tests (new package).
- Not changed: `plan.go:186` (`Verdict.Name` from `friendly_name`, read
  raw) is F1's path; it reaches no controller, and `--dump-plan` does
  not print it. A registry node's label already in `nodes.json` is not
  checked at load (`cmd/ha2fabric/nodes.go` `nodeLabel`, F1's path, cuts
  only); a file written by this version cannot hold one.

### HR-P2-data-32 (low) — fixed

`adapt.Service` sends the payload untrimmed for `select_option`, and for
the same class — a value of the entity's own list — `set_preset_mode`
(fan) and `set_fan_mode` (climate); a blank payload is still no option.
Test `TestOptionNotTrimmed`; red seen (9 failures).

### HR-P2-bound-38 (low) — fixed, the decode half

`ha.State.UnmarshalJSON`: a state object over `MaxStateBytes` (256 KiB
raw) keeps `entity_id`, `state` and its times, drops its attributes, and
sets `State.Oversized` (its size in bytes; `json:"-"`). It holds for
every decode — `get_states` and `state_changed`'s old and new state.
`GetStates` logs `ha.state_oversized` (`entity_id`, `bytes`, `max`), at
warn the first time per entity, then debug. Tests
`TestStateOverTheCap`, `TestGetStatesLogsTheOversized`; red seen.

For the bridge (F1 or round 2): **the verdict reason** — `plan.Assess`
can set a reason when `st.Oversized > 0` (for example "Home Assistant's
state is N bytes, over 256 KiB: its attributes were dropped"); and a
`state_changed` over the cap is not logged by the client (the bridge
decodes the event), so the bridge's state path should log it once per
entity as the client does. The field is `ha.State.Oversized`.

### HR-P2-bound-37 (low) — fixed, the client half

`ha.ErrReadLimit`: a read that ends on `websocket.ErrMessageTooBig` fails
the session with `ha: read: ha: a message from Home Assistant exceeds the
read limit of N bytes: …`, and a command whose connection ended returns
`ErrDisconnected` wrapping the session's cause (`dropped`). The sync's
error now reads `get_states: ha: connection dropped: ha: read: ha: a
message from Home Assistant exceeds the read limit of 67108864 bytes: …`
(`errors.Is` both). Test `TestReadLimitIsNamed`; red seen (`ha:
connection dropped`). The `sync_failed` warning is F1's.

### HR-P2-bound-39 (low) — fixed

`ha.Excerpt` (512 bytes, character boundary, `… (n more bytes)`);
`ha.Error.Error()` cuts code and message, so `Status.LastCommand`,
`bridge.call_service_failed`, `bridge.sync_failed` and
`camera.offer_failed` carry the cut text; `auth_invalid`'s reason and
`camera.ha_error`'s `code` and `message` are cut too. Tests
`TestErrorTextIsCut` (ha), `TestHAErrorTextIsCut` (camera); red seen
for both (the camera one by reverting its line once).

### HR-P2-cfg-03 (note) — fixed

`supervisorGate.run` keeps reading every recheck after the gate opened:
a version below 2026.10.4 closes it again (with `supervisor_too_old`), a
read that fails while open leaves it open (Debug after the first);
`ha2fabric.supervisor` is logged only on a change. Test
`TestSupervisorGateRecloses`; red seen. `TestSupervisorGate`'s call
count is now "at least 3" (the gate no longer stops reading).

### HR-P2-net-01 (note) — none in code (F1's docs).

### HR-P2-bound-41 (note) — accepted

A connection cap on the listener (`LimitListener`) counts connections
before the client filter, which runs per request: a peer `WEB_ALLOW` does
not admit could then hold every slot for `IdleTimeout` and shut the
Ingress proxy out — trading descriptor use for a web UI denial; in the
add-on the admitted peer is the Ingress proxy alone. The better design
is a cap per peer address; register it as accepted for now.

### HR-P2-data-70 (note) — none.

## Changelog lines (for F1, `[Unreleased]`, `### Fixed`)

- A camera's WebRTC sessions end together, within the stop's budget: a
  controller that went away no longer holds the stop for ten seconds per
  session (HR-P2-conc-60).
- Camera: Home Assistant's ICE candidates are bounded as the
  controller's are — 256 per session, 1 KiB each — and an event the
  relay ignores no longer keeps a session from its idle end
  (HR-P2-bound-40).
- Home Assistant's names, manufacturers, models, versions and serials
  reach the controllers without control and bidirectional control
  characters; a web UI name, a node's label and `LABEL` carrying one are
  refused; `--dump-plan` prints such a cell quoted (HR-P2-data-31, closes
  HR-P1-data-36).
- A select's option, a fan's preset and a climate's fan mode go back to
  Home Assistant untrimmed (HR-P2-data-32).
- A state over 256 KiB is read without its attributes and logged
  (`ha.state_oversized`) (HR-P2-bound-38).
- A message from Home Assistant over the read limit is named in the
  sync's error, not only as a dropped connection (HR-P2-bound-37).
- Home Assistant's error text is cut to 512 bytes in errors, the log and
  the status (HR-P2-bound-39).
- The add-on's Supervisor check reads the version every 10 minutes also
  after it passed; a Supervisor downgraded below 2026.10.4 takes the web
  UI's administrator away again (HR-P2-cfg-03).

## Docs lines (for F1, `docs/operations.md`)

- Log table, new rows: `camera.close_overrun` | warn | A camera's
  sessions did not all end within the stop's deadline or `SignalTimeout`
  (`sessions`); the rest was abandoned. — `camera.candidates_exceeded` |
  warn | Home Assistant signalled more than 256 candidates in one session
  (`session`, `max`); the session ends. — `ha.state_oversized` | warn,
  then debug | A state over 256 KiB (`entity_id`, `bytes`, `max`); its
  attributes were dropped; at warn the first time per entity.
- Log table, new row `camera.candidate_refused` (not listed today) | warn | A
  candidate of Home Assistant's that cannot be carried: over 1 KiB
  (`bytes`, `max`) or an `sdpMLineIndex` out of range; the session stays.
- Log table, `ha2fabric.supervisor`: "…at warn when it is older; read
  again every 10 minutes (`recheck_in`), also once it passed: a
  Supervisor downgraded below 2026.10.4 takes the administrator away
  again." `ha2fabric.supervisor_unreadable`: "every 10 minutes once a
  version was read; a gate that passed stays open."
- Administrator paragraph (`:428-434`): "…while it is below, it reads the
  version again every 10 minutes and gets its administrator once the
  Supervisor is updated; it keeps reading after that, and a downgrade
  below 2026.10.4 takes the administrator away again."
- Cameras list (after `:409`): "Home Assistant's side is bounded the
  same way: at most 256 candidates a session (the session ends beyond)
  and 1 KiB a candidate (refused, logged). Only a signal the daemon
  relays counts as activity for the hour."
- The stop paragraph (`:635`): "A camera's sessions end together within
  the budget (each at most 10 seconds)." (with F1's `stopRun` sentence).
- The read-limit symptom (with F1's `sync_failed` warning): the sync's
  error reads `… exceeds the read limit of 67108864 bytes`.
- Web UI names and node labels: "A name or label with a control or
  bidirectional control character is refused; Home Assistant's own names
  are shown without them."

## Differences from the brief

- `internal/text` is a new package and `internal/arch/arch_test.go`
  changed (neither path is in F1's or F2's list; no overlap with F1).
- `text.Clean` turns a white-space control (tab, line break, U+0085)
  into a space instead of dropping it, so "Living\nroom" reads "Living
  room"; every other one is dropped as decided.
- data-32 also covers the fan preset and the climate fan mode (the same
  class: a value of the entity's own list).
- `--dump-plan` quotes a cell only when it carries such a character, not
  every name cell.

## Questions

1. `FuzzClean` (`./internal/text/:FuzzClean`) to `FUZZ_TARGETS`? Left
   out to keep off the `Makefile` line F1 edits for test-42.
2. The verdict reason and the `state_changed` log line for an oversized
   state (above) — F1 now or round 2?
3. A `READ_LIMIT` option (X2's bound-37 fix names it) was not added: the
   brief does not list it, and it is a configuration surface (config,
   add-on schema, docs). Owner's call.
