# Brief C3 (go-ha2fabric) — the action plugs' unbound commands, over go-hamapping v0.4.6 (0.5.7)

Issue #51 (2026-11-08): a scene's plug answers Off, Toggle, OffWithEffect,
OnWithRecallGlobalScene and OnWithTimedOff with Success; go-fabric's own OnOff
server runs them (`cluster/onoff/server.go:748-752` at v0.18.0), so the daemon
never hears of them — no `bridge.command_unmapped`, no call. The mapping is
go-hamapping's: brief C3 there
(`go-hamapping notes/briefs/2026-11-08-C3-action-plug-commands.md`, PR #22,
v0.4.6) binds the on family to the action, discards the off family through the
new `Plan.OnDiscard`, binds Toggle on climate and media players to `toggle`,
discards Toggle on a mute plug and binds the lock's UnlockWithTimeout to the
unlock (relock: go-hamapping #23). Verified sources are in that brief; the
host adds `climate/services.yaml:145-151` and `media_player/services.yaml:17-24`
at 2026.10.3 (toggle; TURN_ON and TURN_OFF).

## §1 Host

- `internal/adapt/command.go`: `toggle` for `climate` and `media_player` in the
  service table, with the citations.
- `internal/bridge`: `hp.OnDiscard(b.onDiscard)` beside `OnChange`;
  `onDiscard` logs `bridge.command_discarded` (`entity`, `cluster`, `command`)
  at info and counts `Status.Discards` (`discards` on the status page).
- `docs/operations.md`: the log line's row. `changelog.md` 0.5.7: Fixed (#51),
  Changed (go-hamapping v0.4.6). `internal/version`, `addon/config.yaml`,
  `README.md`, `CLAUDE.md` state lines.

## §2 Tests

`internal/adapt/adapt_test.go` `TestService`: climate and media_player
`toggle`. `internal/bridge/bridge_c2_e2e_test.go` `TestActionPlugs`: Toggle
calls the action's service, Off calls nothing and counts one discard, OnOff
stays false throughout.

## §3 Guards, release

`make ci`, `make race`, `make cover-check`, `make secrets` in the background
with one summary log; `make docker addon-image` and `make release` (the binary
and the add-on move). One pull request on `fix/action-plug-commands`,
squash-merged, auto-merge; tag 0.5.7. A short answer in #51.
