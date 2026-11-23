# Phase 0 — mechanical evidence

- **Code:** baseline `10ee7b4`; guards run on branch `review/p0` at HEAD
  `b75e9a3` (two notes-only commits on top), HEAD unchanged before and
  after every guard. The fuzz targets are commit `dfb949e`
  (`test(fuzz): …`) on top; `make ci` and `make fuzz-smoke` ran again
  with them before that commit (same HEAD `b75e9a3`, targets in the
  working tree).
- **Toolchain:** go1.27.4 linux/amd64 (`go.mod` go 1.27.4), gofumpt
  v0.12.0, golangci-lint 2.14.0, govulncheck, go-licenses, gitleaks
  v8.30.1 (via `go run`), Docker 29.8.0.
- **Date:** 2026-11-23. Guards 15:40:50–15:58:06 (sequential, one job at
  a time); fuzzing 15:58:56–16:51:04 (twelve targets one after another,
  5 min each, `-parallel 4`); re-run of `make ci` and `make fuzz-smoke`
  16:51:33–16:55:10.
- **Machine:** 5 cores, 19 GB; two other read-only review agents ran in
  parallel.

## Guards

| Guard | Exit | Duration | Summary |
| --- | --- | --- | --- |
| `make ci` | 0 | 96 s | build, vet; `go test`: 14 packages ok (`internal/bridge` 28.5 s, `cmd/ha2fabric` 13.8 s, the rest < 1 s); gofumpt clean; golangci-lint **0 issues**; `go mod tidy -diff` clean, `go mod verify` all verified |
| `make race` | 0 | 89 s | no `DATA RACE`; 14 packages ok |
| `make cover-check` | 0 | 63 s | every floor met: `cmd/ha2fabric` 83.5 (75), `adapt` 96.8 (95), `bridge` 87.9 (82), `camera` 94.0 (90), `config` 98.8 (92), `expose` 96.8 (95), `exposure` 96.7 (96), `ha` 90.8 (80), `ha/hatest` 92.6 (0), `nodes` 90.7 (88), `plan` 98.6 (98), `selection` 94.6 (90), `version` 90.9 (75), `web` 95.5 (85) |
| `make fuzz-smoke` | 0 | 15 s | baseline: `FuzzDecode` only, 10018 execs |
| `make vuln` | 0 | 25 s | govulncheck: **No vulnerabilities found** |
| `make licenses` | 0 | 33 s | go-licenses passed; one `E` line: no licence classified for `modernc.org/mathutil` v1.7.1 (known, `Makefile` comment above `licenses`) |
| `make secrets` | 0 | 7 s | gitleaks: 72 commits, ~1.56 MB, **no leaks found** |
| `GOARCH=386 make build` | 0 | 295 s | builds |
| `GOARCH=386 go test ./...` | 0 | 50 s | 14 packages ok, none cached |
| `make version-check` | 0 | 0 s | 0.7.0: `internal/version`, `addon/config.yaml`, `changelog.md` agree |
| `make release` | 0 | 35 s | `dist/`: linux/amd64, arm64, armv7 archives (14/13/13 MB), `SHA256SUMS`, `RELEASE_NOTES.md` |
| `make docker` | 0 | 178 s | image `go-ha2fabric:v0.6.3-7-gb75e9a3` (the tag is `git describe`: 0.7.0 is untagged, by the owner's rule) |
| `make addon-image` | 0 | 148 s | builds; one BuildKit warning `InvalidDefaultArgInFrom` for `ARG BUILD_FROM` without default (`addon/Dockerfile:33-34`; the Makefile and the builder pass it) |
| `make ci` (with the fuzz targets) | 0 | 61 s | as above, golangci-lint **0 issues** |
| `make fuzz-smoke` (with the fuzz targets) | **2** | 156 s | first seven targets ok; **`FuzzSlug` fails** at 10000x (HR-P0-data-02), `set -e` stops the loop; the five remaining targets (camera, adapt) run by hand at 10000x: all ok |

## Fuzz targets (commit `dfb949e`)

Each in a `fuzz_test.go` of its package with seeds and the SPDX header;
`make fuzz-smoke` runs every entry of `FUZZ_TARGETS` (`Makefile`, one
`go test -fuzz` per target). 5-minute local runs:

| Target | Package | Parse site | Execs | Result |
| --- | --- | --- | --- | --- |
| `FuzzEventData` | `internal/ha` | event envelope; `StateChanged`, `RegistryUpdated`, `LabelRegistryUpdated`, `WebRTCEvent` (`client.go:334`, `bridge/bridge.go:266,280`, `camera/camera.go:460`) | 3.20 M | ok |
| `FuzzSnapshotResults` | `internal/ha` | every `ReadSnapshot` result type, `Tuple` (`client.go:476`, `message.go:260-284`) | 2.78 M | ok |
| `FuzzHTTPBase` | `internal/ha` | `HTTPBase` of `HA_URL` (`camera.go:95-117`) | 13.6 k | **fail** after 2 s: HR-P0-err-03 |
| `FuzzLoad` | `internal/config` | list and number variables of `Load` (`config.go:171-330`) | 0.72 M | ok |
| `FuzzOpen` | `internal/selection` | the overrides file (`selection.go:74-108`), with a write-back round trip | 1.65 M | ok |
| `FuzzOpen` | `internal/nodes` | `nodes.json` (`nodes.go:113-162`) | 0.69 M | ok |
| `FuzzSlug` | `internal/nodes` | a web UI label as key (`nodes.go:243-265`) | — | **fail** after 10 s: HR-P0-data-02 |
| `FuzzCandidateFromHA` | `internal/camera` | HA candidate event → `fromHA` (`camera.go:420-435`), round trip through `toHA` | 1.22 M | ok |
| `FuzzCandidateToHA` | `internal/camera` | controller candidate → `toHA` (`camera.go:407-418`) → JSON | 0.97 M | ok |
| `FuzzCaptureSnapshot` | `internal/camera` | the still's check (`camera.go:653-673`) | 1.14 M | ok |
| `FuzzEntityState` | `internal/adapt` | hostile state and attributes → `Entity`, `State` | 1.14 M | ok |
| `FuzzService` | `internal/adapt` | a controller command → `call_service` (`command.go:88-…`) | 1.17 M | ok |

Crash seeds are kept, not committed (a committed failing seed turns
`make ci` red): `held/phase-0-fuzz-seeds/internal/ha/testdata/fuzz/FuzzHTTPBase/674851e0d764d4fa`
(`"ws://::/websocket"`) and
`held/phase-0-fuzz-seeds/internal/nodes/testdata/fuzz/FuzzSlug/{79ef6f3b48b21a5e,a373968b240a230a}`;
copied back under the package's `testdata/fuzz/`, `go test` replays them.

## Error-path coverage (from `coverage.out` of `make cover-check`)

Block-level reading of the merged profile (`-coverpkg=./...`, a block
counts as reached when any test binary reached it), baseline `b75e9a3`,
without the new fuzz seeds. Error branches no test reaches:

- **`internal/ha`:** the auth handshake's failures —
  `client.go:264-265` (read of auth_required), `:267-268` (not
  auth_required), `:271-272` (marshal), `:274-275` (write), `:278-279`
  (read of the reply), `:286` (neither auth_ok nor auth_invalid; auth_invalid
  itself is reached); `:293-294`, `:297-298` (`readOne`); hostile-data
  paths of the read loop — `:313-314` (`ha.frame_refused`), `:335-336`
  (`ha.event_refused`), `:477-478` (a result that does not decode); `:217-219`,
  `:250-252` (write and ping failure end the session); `:422-423`, `:431`
  (`ha: command failed` without an error object), `:449-450`
  (`subscribe_events` fails); `subscription.go:67-68`, `:82-83`,
  `:144-147`, `:150-151`; `camera.go:149-150`, `:170-171` (request build,
  body read). `message.go:271-272`, `:278-279` (`Tuple` element errors) are
  unreachable by construction: the outer `json.Unmarshal` has validated
  the element.
- **`internal/config`:** `config.go:349-350` (a port outside 1..65535 in
  `hostPort`); `:158` `OSEnv`.
- **`internal/web`:** `web.go:125-128` (the server's own error other than
  shutdown); `operation.go:65` (nil observer).
- **`internal/camera`:** `camera.go:258-259` (`ErrNoProvider`), `:286-287`,
  `:330-333` (offer after close), `:387-388`, `:443-444`, `:461-462`
  (`camera.event_refused`), `:490-491` (`camera.candidate_failed`),
  `:526`, `:546-547`, `:560`, `:576-577`, `:582-583`, `:667-668` (still
  size outside 1..65535).
- **`internal/nodes`:** `nodes.go:132-133` (corrupt file cannot be moved
  aside), `:202-212` (allocation errors), `:295`, `:337`, `:373`, `:385-391`
  (no free key, port, discriminator, passcode), `:345-353` (`bindable`
  false), `:420-421`, `:436-437`, `:452-453`, `:461-466`, `:483-485`
  (write failures).
- **`internal/selection`:** `selection.go:97-98` (corrupt file cannot be
  moved aside), `:179-180`, `:197-199` (temp file create / write failure).
- **`cmd/ha2fabric`:** `main.go:77-79`, `:90-92`, `:103-105` (selection,
  registry, node open fails → exit 1), `:121-134` (the web listener fails
  → exit 1), `:143-144`, `:257-258`, `:265-267` (dump-plan error),
  `:272-273`, `:285-286`; `nodes.go:86-87`, `:103-105` (node start fails),
  `:224-225`, `:245-246`, `:271-278`, `:309-311`, `:378-379`, `:390-392`
  (`nodeLabel` cut of a label over 32 bytes), `:406-407`, `:461-479`
  (create-node failure and its rollback of files), `:493-569` (rename and
  remove errors), `:583-593`, `:607`, `:635-664` (fabric removal and stop
  failures).

## Findings

| ID | Severity | Status | Finding |
| --- | --- | --- | --- |
| HR-P0-data-01 | low | open | `clampInt` (`internal/adapt/command.go:389-392`) converts before it clamps: `int(math.Round(f))` of a value beyond the int range is the minimum int on amd64 (and beyond 2^31 on 386 and the release target arm/v7), so it clamps to `lo`, not `hi` — measured with a probe: 1e300 → 0 on amd64, 3e9 → 0 on 386. Hits brightness (`:325`, 1e300 → 0), `rgb_color` (`:345`), cover position and tilt (`:116`, `:121`), `:152`; `color_temp_kelvin` (`:330`) is not clamped at all (1e300 → a negative int). `:218` clamps in float first, correctly. Reachability from a controller: inferred unlikely (go-hamapping renders bounded Matter arguments); phase 1 should confirm. |
| HR-P0-data-02 | low | open | `Slug` (`internal/nodes/nodes.go:243-265`) is documented "at most 56 characters (room for a suffix)" (`:241`) but returns 57 when the bound is crossed by a dash and a character (`:250`, `:257`): measured by `FuzzSlug` (`Slug("000…000 0")`, 55 zeros, gives 57). No key over 64 results (57 + `-999` = 61; `validKey` at `:269-279` holds the contract), so the effect is the bound's statement only. Makes `make fuzz-smoke` red (see HR-P0-test-04). |
| HR-P0-err-03 | low | open | `ha.HTTPBase` (`internal/ha/camera.go:95-117`) accepts `ws://::/websocket` and returns `http://::/api`, which `url.Parse` refuses (`invalid port "::" after host`): measured by `FuzzHTTPBase`. `config.Load` checks `HA_URL` by prefix only (`internal/config/config.go:251`), so such a URL fails at dial and at each still rather than at start. Not a security item. |
| HR-P0-test-04 | medium | open | With `dfb949e`, `make fuzz-smoke` exits 2 locally (FuzzSlug at 10000x) and CI's `fuzz-smoke` job (`.github/workflows/ci.yml:119`, `FUZZTIME=20000x`) is expected red until HR-P0-data-02 (and likely HR-P0-err-03, which fell at 13.6 k execs) is resolved or the target's bound is decided — a coordinator decision before any push. |
| HR-P0-doc-05 | info | open | `README.md:174` and `.github/workflows/ci.yml:6` still describe `fuzz-smoke` as the decoder's one target. |
| HR-P0-test-06 | low | open | Error branches on hostile input that no test reaches: `ha.frame_refused` (`client.go:313-314`), `ha.event_refused` (`:335-336`), a result that does not decode (`:477-478`), the auth handshake's failures (`:262-286` except auth_invalid), `camera.event_refused` (`camera.go:461-462`); and the daemon's exits on a failing web listener (`cmd/ha2fabric/main.go:121-134`) and the create-node rollback (`cmd/ha2fabric/nodes.go:461-479`). List above. |

Notes, not findings: the `modernc.org/mathutil` licence classifier line
(known); the add-on image's `InvalidDefaultArgInFrom` warning (the base is
always passed); the Docker image tag from `git describe` while 0.7.0 is
untagged.

No security weakness found by this unit; nothing in `held/` beyond the
fuzz seeds.
