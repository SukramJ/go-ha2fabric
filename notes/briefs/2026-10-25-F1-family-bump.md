# Brief F1 — the family bump: go-hamapping v0.2.1 (`Options.Name`), go-fabric-server v0.3.1, go-ha2fabric v0.2.2 onto go-fabric v0.13.3 and go-fabric-ui v0.1.2

Date 2026-10-25, amended 2026-10-26: milestone 5 closed with go-fabric `v0.13.3`, so every go-fabric pin below is `v0.13.3` (not `v0.12.2`); read go-fabric `CHANGELOG.md` 0.13.0–0.13.3 and the migration guide `docs/guides/migrating-to-the-facade.md` ("Changed in v0.13.x") before bumping — `MarkComplete` is gone (0.13.0), `Sink.SetTemperatureSetpointHold` is a break only for a host that implements `thermo.Sink` (none of the three repositories does). go-ha2fabric is already at `v0.2.1` (its own fixes #6–#8), so §3 releases `v0.2.2`. `ble/bluez` stays `v0.12.1` (unchanged since). Three small pull requests in sequence, one agent. The
owner merges and tags each (the launcher says when a tag exists); the
next repository pins the tag. Sources: go-fabric-ui `docs/reports/U1a.md`
and `U1b.md` (`Column.Multi`, `Options.Warnings`, `Events.Observers`
deprecated, the v0.12.2 routes), go-fabric-server `docs/reports/S3.md`
Q22 (ble/bluez), go-ha2fabric `docs/reports/H3.md` Q2/Q3 and the "side
effect" note (unresolved-label warning only in the log), go-fabric
`CHANGELOG.md` 0.12.2 and `ble/bluez/v0.12.1`. **Model:** Opus.
Conventional commits, no auto-merge, no label, no attribution lines,
English; each repository's `make ci` plus race, cover-check, and for the
daemons the image builds, release dry run and `make version-check`.
`df -h /` before guard runs, `go clean -cache -testcache` after. go-fabric's
`CLAUDE.md` token rules. Hand back after each PR (number, HEAD, Questions
verbatim) and wait for the tag before opening the next.

## 1 go-hamapping `v0.2.1` — `Options.Name` (H3 Q3)

Repository `/home/markus/Dokumente/GitHub/go-hamapping` (main = v0.2.0),
branch `f1/options-name`. Today an entity's node label is
`device.ComposeNodeLabel(deviceName, entityName)` (`plan.go:247-256`).
Add `Options.Name func(model.Entity) (name string, verbatim bool)`:
when set and `verbatim` is true, the returned name is the endpoint's
NodeLabel as given (go-fabric's 32-byte rule applies, truncated the way
`ComposeNodeLabel` truncates), without the device name; when false or
unset, today's composition. Semantics decided by the owner 2026-10-25:
a name a user typed in a UI is final. Test: a renamed entity carries the
verbatim label; the default is unchanged. Pin go-fabric `v0.13.3`.
Changelog `## [0.2.1]`.

## 2 go-fabric-server `v0.3.1`

Repository `/home/markus/Dokumente/GitHub/go-fabric-server` (main = v0.3.0),
branch `f1/bump`. Pins: go-fabric `v0.13.3`, go-fabric-ui `v0.1.2`,
`github.com/SukramJ/go-fabric/ble/bluez v0.12.1` — `--bluetooth-adapter`
now passes `bluez.NewCentral` to `controllerapi.Options.BLE` (S3 said the
code path is there; make the flag work and add a unit test that the
option is wired; no BLE in CI). Drop the `Events.Observers` line
(`internal/daemon/run.go:315`; events now come from `Node.Observe` inside
the handler). Version 0.3.1 (`internal/version`, `changelog.md`,
`addon/config.yaml`; `make version-check`). The compat job stays green.

## 3 go-ha2fabric `v0.2.2`

Repository `/home/markus/Dokumente/GitHub/go-ha2fabric` (main = v0.2.0),
branch `f1/bump` (main = v0.2.1). Pins: go-fabric `v0.13.3`, go-fabric-ui `v0.1.2`,
go-hamapping `v0.2.1`. Changes: the labels column is `Column.Multi` and
its filter `Filter.Multi` (H3 Q2); `fabricui.Options.Warnings` reports
unresolved label values (the `bridge.label_unresolved` case) and any
other half-valid option so the UI shows them (H3 side effect); the UI's
rename override goes through `hamapping.Options.Name` verbatim (§1);
drop the `Events.Observers` line (`cmd/ha2fabric/main.go:83`); the
e2e tests on `fabrictest.Site.AddNode` with the daemon's own `node.Config`
if go-fabric 0.12.1's route reads cleaner than `Device.Node()` (say
which). Version 0.2.2 everywhere `make version-check` reads; changelog;
`docs/operations.md` label-warning paragraph updated; the report
`docs/reports/F1.md` short (per §, guards with HEAD, differences,
questions).

Copy this brief to `notes/briefs/` of each repository in its first
commit. If a permission is denied, say which and stop that step; never
ask the launcher to perform it.
