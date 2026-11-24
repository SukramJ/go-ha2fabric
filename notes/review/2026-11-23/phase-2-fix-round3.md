# Phase 2 fix wave — round 3 (after the pre-merge read)

Implementer round 3 (Opus). Brief `notes/briefs/2026-11-24-review-p2-fix.md`,
section "Round 3 — after the pre-merge read". Worktree
`go-ha2fabric-review`, branch `review/p2`, base `9b274b3`. Findings
`phase-2-premerge.md` (IDs 90–95), decision 52.

## Commits

| Commit | Rows |
| --- | --- |
| `07bd1bb` fix(plan): an oversized state's entity is unmappable, not re-typed | bound-90 |
| `cce7717` fix(bridge): an entity go-fabric refused is not served | data-91 |
| `7cbdb99` fix(exposure): sync_failed and its docs say what a failed batch leaves | doc-92, doc-93 |
| `4a21330` fix(adapt,nodes): an unsafe-only registry name falls through; labels checked at load | data-94 |
| `d9622dc` test(bridge): a dropped command's attribute is read back at Home Assistant's state | test-95 |
| `df008ac` test(nodes): the bidi control as an escape | data-94 (ST1018 in the new test) |
| this report | — |

## Guards

At `df008ac` (HEAD before and after the run): `make ci` 0 (lint with
staticcheck, so SA1019: no deprecated go-fabric identifier), `make race`
0, `make cover-check` 0, `make fuzz-smoke` 0. A first `make ci` at
`d9622dc` failed on ST1018 (a literal U+202E in the new nodes test),
fixed in `df008ac`. Logs in the session's scratchpad `R3/`. This report's
commit is notes only: no second run. Not run (the coordinator's):
`make vuln licenses secrets`, `make release`, `make docker addon-image`.

## Per row

| Row | Status | Test | Red seen |
| --- | --- | --- | --- |
| bound-90 (medium) | done, decision 52: `plan.Assess` gives an exposed entity whose state is `Oversized` the verdict unmappable with `OversizedReason` ("state over 256 KiB (N bytes): its attributes were dropped, so it is not mapped until a state under the cap") and does not group it, so the endpoint is removed, not re-typed. `MergeVerdicts` no longer appends the reason (no grouped entity is oversized now) and `Verdict.Oversized` is gone. The bridge: a served entity's `state_changed` over the cap re-syncs and is not applied; an entity the last sync left unmappable for its size (`hollow`, worker-owned) re-syncs on a state under the cap, so the endpoint comes back on its stored number | `TestOversizedUnmappable` (plan: `light.kitchen` and `climate.bedroom_ac` over the cap, Assess and Dry; replaces `TestOversizedReason`), `TestOversizedStateTakesTheEndpoint` (bridge e2e: over the cap, endpoint gone; under it, the same number back) | yes: the plan test red (both planned, dt `0x301`/`0x100`); the e2e red on "endpoint is gone"; the recovery half seen red with the `hollow` re-sync disabled |
| data-91 | done: `serving` is built from `g.plan.DeviceFor(key) != nil`, so an entity go-fabric refused (data-35) is not served and a command queued for it is dropped by the HR-P1-conc-33 guard (`bridge.command_unserved`) | `TestRefusedEntityIsNotServed` (the probe's setup; `Serving` false for `climate.hall`, true for `switch.coffee`; a queued command makes no call and counts one command error) | yes |
| doc-92 | done, the smaller true change (docs, no new log line): the `sync_failed` warning reads "the endpoints stay as the last sync left them, and what failed is tried again"; `docs/operations.md`'s `bridge.sync_failed` row names only the read, says a refused batch (`bridge.batch`) counts too; the `bridge.batch` row says the batch's other operations are applied and the retry uses `sync_failed`'s back-off; the status list names `bridge.batch` beside `bridge.sync_failed` | `TestProviderSyncFailed` asserts the message has no "succeeded" | yes (against the old `exposure.go`) |
| doc-93 | done: the reads table says "one read shared by every node on the connection" | none (docs) | — |
| data-94 | done: `adapt.entityName` cleans the registry name first and falls through on "" to `original_name` / `friendly_name` (a has-entity-name entity without an original name stays the device's main feature); `nodes.Open` checks each label with `checkLabel` and on refusal uses it `text.Clean`ed and logs `nodes.label_refused` at warn (not fatal, the file is not rewritten) | `TestUnsafeOnlyRegistryName`, `TestLabelCheckedAtLoad` | yes, both |
| test-95 | done: an e2e test with the command loop held on its first call (an upstream wrapper that ignores the call's context, so no failed call restores meanwhile) and the queue filled through a test export; a controller's Off then On are dropped and the plug's OnOff reads Home Assistant's off again. The internal err-63 tests stay | `TestDroppedCommandAttributeBack` | test-only row: seen red with `restore` removed from `onCommand` (OnOff stays on) |

## Changelog and docs lines

- `changelog.md` 0.7.0 `### Fixed`: a new line for HR-P2-bound-90; the
  HR-P2-data-31 line extended with HR-P2-data-94 (the fallback and
  `nodes.label_refused`). No line for data-91 (a narrow race behind the
  data-35 line) or doc-92/93 (docs).
- `docs/operations.md`: the oversized paragraph and the
  `ha.state_oversized` row (unmappable, back under the cap); the
  `nodes.json` paragraph and a `nodes.label_refused` log row; the
  `bridge.sync_failed` and `bridge.batch` rows and the status list
  (doc-92); the reads table (doc-93).

## Differences from the brief

- bound-90: the reason carries the size beside "state over 256 KiB"; the
  bridge half (re-sync when the state crosses the cap either way) is
  added, since otherwise the endpoint would not come back until the next
  registry event.
- test-95: covers the queue-full path end to end; the unmapped path keeps
  its internal test (the same `restore`). Three test exports added
  (`Serving`, `RunCommand`, `FillCommands` in `export_test.go`).
- One extra commit (`df008ac`) for the lint finding in the new test.

## Questions

None.
