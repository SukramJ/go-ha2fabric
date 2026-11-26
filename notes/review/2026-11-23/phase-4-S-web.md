# Phase 4, run 1 — unit S-web (the web gate, configuration, add-on, images, workflows)

Reviewer S-web, Opus in Fable's role (owner rule 2026-11-24). Code at
`c3a9770` (go-fabric v0.26.0, go-fabric-ui v0.14.1); probes in a
`git archive c3a9770` copy under the session scratchpad (`p4/S-web`, and
`p4/S-web27` for the v0.27.0 lift). Brief
`notes/briefs/2026-11-26-review-p4.md`. Prefixes: `ui:` go-fabric-ui
v0.14.1, `gf:` / `gf27:` go-fabric v0.26.0 / v0.27.0, `sup:` Supervisor
`fe91b8db`, `s6:` just-containers/s6-overlay `v3.2.3.2` (the version the
pinned Home Assistant base image installs), `base:` home-assistant/docker-base
`2026.11.1` `alpine/Dockerfile`. **M** measured, **I** inferred.

**Held back: none.** No new exploitable security item.

## Delta read (d16f3ab..c3a9770)

Every changed line of the unit's files read against what it replaced:

- `cmd/ha2fabric/supervisor.go` (+119/−21): the request moved into
  `supervisorGet` (shared by the gate and the address read; same bounds:
  10 s, no redirect, no proxy, 1 MiB, `result == "ok"`), new
  `addonAddress`, `addonWebListen`, `addonInfoURL`. The gate's behaviour is
  unchanged (`supervisor.go:193-205` against the removed body). Decision
  54's bind is read below.
- `cmd/ha2fabric/main.go`: the bind call (`main.go:170-175`), import paths,
  citation pins. No other behaviour moved.
- `internal/config/*`, `internal/web/*`: import paths and citation pins
  only; `DefaultVendorID`/`DefaultProductID` now feed the flag defaults
  (`config.go:240-241`, same values).
- `script/run.sh`: comment only (`run.sh:76-79`).
- `addon/config.yaml`: armv7 dropped (decision 64), `label: str(1,32)`
  (HR-P3-cfg-68), comments, repository address.
- `addon/Dockerfile`, `Dockerfile`, `.github/*`, `script/licenses/generate.sh`
  (new), `script/extract-release-notes.sh` (repository default): phase 3's
  supply-chain wave; read whole below.

## Full cut — what holds (no finding)

- **Gate, add-on.** Filter `WEB_ALLOW = 172.30.32.2` forced by `config`
  (`config.go:298-302`, `:385-387`) and set again by `run.sh:81`;
  Admin = `gate.open() && IngressAdmin` (`main.go:320-327`,
  `web.go:173-188`); Host = `From(proxy)` (`main.go:342-347`). The groups
  header cannot be smuggled: the Supervisor drops it from the client's
  headers (`sup:supervisor/api/ingress.py:47-63`) and sets it after the
  copy (`:411-419`); a non-admin is refused before the add-on
  (`:190-197`, `panel_admin: true`). M.
- **Gate, stand-alone / container / `WEB_ADMIN`.** `ReadOnly` unless
  `WEB_ADMIN`; `WEB_ADMIN` off loopback needs `WEB_ALLOW`
  (`config.go:393-395`), and a text-loopback `WEB_LISTEN` must bind
  loopback (`web.go:125-128`, `main.go:310`). An unparsable `Allow` entry
  fails `web.New` (`web.go:72-79`). M.
- **Decision 54's bind** (`supervisor.go:331-384`, `main.go:170-175`).
  *Lies:* the address is the one the Ingress proxy dials by construction —
  both are `app.ip_address` (`sup:supervisor/api/ingress.py:124-126`,
  `sup:supervisor/api/apps.py:308`, `sup:supervisor/docker/app.py:187-191`),
  and the Supervisor is trusted (decision 11). *Times out:* three tries of
  at most 10 s each plus 2 × 1 s, then today's `0.0.0.0` with one warning;
  the filter still serves the proxy only — not fail open. *Not on the
  host:* the bind fails, exit 1 (HR-P3-cfg-94, accepted). *Unspecified /
  not an IP / error result / non-200 / undecodable:* refused
  (`TestAddonAddress`). A loopback or link-local answer would bind there
  and leave Ingress dark (fails closed); out of the trust model, no
  finding. M (code), I (Supervisor behaviour beyond the cited lines).
- **Host, origin, framing** as this host sets go-fabric-ui's `Options`:
  `AllowedHosts` validated at start (`config.go:474-488`, but see cfg-05),
  `AllowHost` only in the add-on, no `FrameAncestors` (Ingress is the same
  origin; HR-P2-frame-02 closed by the live test), `MaxStreams` default
  (HR-P2-bound-41 accepted), `Done` wired (`main.go:182`). M.
- **Secrets.** `HAToken`/`SupervisorToken` reach only `ha.New`, the gate
  and the address read (`main.go:91`, `:109`, `:173`); no `%v` of `Config`
  in non-test code; `nodeConfig` carries no passcode (`main.go:286-298`);
  the Supervisor's errors name the path, never the token
  (`supervisor.go:217-228`). M.
- **Config bounds.** Passcode `ValidateSetupPIN`, discriminator 12 bit, ids
  non-zero, label ≤ 32 bytes and `text.Safe`, `NODE_LIMIT` 0..32, URL
  without userinfo; `FuzzLoad` holds them. M.
- **Manifest and `run.sh`** against `sup:supervisor/apps/options.py`
  (`RE_SCHEMA_ELEMENT`, `str(1,32)` by `vol.Length`, `int(…)?` optional,
  `port` by `network_port`, `:153-182`): every option maps; `run.sh` quotes
  every value and evaluates none. M.
- **Workflows.** Tags only on `main` (`git merge-base --is-ancestor` in all
  three tag workflows), no `workflow_dispatch` left, every action by full
  SHA (lint job's grep, `ci.yml:61-68`), workflow-level `contents: read`,
  job-level writes only where needed, `ghcr` environment on both image
  jobs (settings read: `v*` tag policy), attest + SBOM steps present.
  Untrusted input in `run:`: none — event data reaches shell only through
  `env:` (`dependabot-auto-merge.yml:149-152`). Decision 57's gate:
  `update-type` is the group's maximum (fetch-metadata at the pinned SHA,
  `src/dependabot/output.ts:21`, `:79-81`), family names refused by
  pattern; required checks include `coverage`, `fuzz-smoke`, `docker`
  (branch protection read with `gh api`, app 15368). M.
- **Images.** Both base stages and the build stage by digest; distroless
  `nonroot`, `/data` owned by it; `licenses/` in both (CI diffs
  `MODULES.txt`); `.dockerignore` keeps `.git` (and the checkout's
  persisted token) out of the build context. The add-on runs as root —
  issue #72, not re-reported. M.
- `script/licenses/generate.sh`, `script/extract-release-notes.sh`: read
  whole; `set -euo pipefail`, no unquoted expansion of input, literal
  `index()` match of the version. No finding.

## Findings

### HR-P4-cfg-01 — low (M) — the add-on's stop budget is s6-overlay's 3 s, not 25 s

- **Where:** `addon/Dockerfile:92-108` (no `S6_*` setting; `CMD ["/run.sh"]`
  under the base image's `ENTRYPOINT ["/init"]`), `script/run.sh:7`
  ("exec's the daemon, which becomes PID 1 and receives the signals"),
  `addon/config.yaml:129-132`, `cmd/ha2fabric/main.go:62-65`,
  `docs/operations.md:717-728` ("The add-on's manifest gives the stop 30
  seconds before the Supervisor kills it").
- **Claim:** In the add-on `/init` (s6-overlay) is PID 1, not the daemon.
  On `docker stop` (the Supervisor stops with the manifest's `timeout`,
  `sup:supervisor/docker/interface.py:570-574`) s6 runs its stage 3 and
  TERMs the CMD last, then KILLs every process after `S6_KILL_GRACETIME`,
  default 3000 ms (`s6:README.md:167-173`, `:858-860`, `:903-907`;
  `s6:…/libexec/stage0:51-52`). The base image sets neither
  `S6_KILL_GRACETIME` nor `S6_CMD_RECEIVE_SIGNALS`
  (`base:alpine/Dockerfile:139-146`). So decision 29's one 25 s budget
  (HR-P1-err-64's fix) does not hold in the add-on: the daemon is killed
  about 3 s after its TERM, whatever `timeout: 30` says. What that cuts:
  the 20 s an erase in progress is given (the docs then say it "must be
  repeated"), the camera sessions' 10 s end, the nodes' orderly stop. The
  stand-alone binary and the container image are not affected by s6 (see
  cfg-02 for the container's own limit).
- **Reference / measurement:** the pinned base image
  `ghcr.io/home-assistant/amd64-base:3.24-2026.11.1@sha256:d4d8bc2b…`
  with a CMD that traps TERM and needs 20 s: `docker stop -t 30` returned
  after 3.7 s, the CMD's last line was `alive 2` (killed). With
  `-e S6_CMD_RECEIVE_SIGNALS=1` the same CMD got the TERM directly and
  ended cleanly (`clean exit`, 23 s); with `-e S6_KILL_GRACETIME=28000`
  also clean (28 s). Probe P1 below. The live add-on itself was not run
  (decision 12); its log would show `ha2fabric.stopping` without the
  node-stop lines that follow.
- **Fix:** `addon/Dockerfile` final stage: `ENV S6_CMD_RECEIVE_SIGNALS=1`
  (the container's SIGTERM goes to the CMD, which `run.sh` `exec`s into
  the daemon; s6 shuts down when it exits), optionally with
  `S6_KILL_GRACETIME=` a value below `timeout` as a backstop. Correct
  `run.sh:7` and the docs sentence. `S6_CMD_RECEIVE_SIGNALS=1` is the
  better of the two: the stop takes as long as the daemon needs, not a
  fixed grace.
- **Red test / guard:** P1's `TestProbeAddonStopSignal` (red at
  `c3a9770`) beside `TestAddonStopTimeout` (`cmd/ha2fabric/image_test.go:141`),
  which today checks the manifest only — the guard holds the image half.
- **Severity:** low, as HR-P1-err-64 was; it is that row's fix incomplete
  in the add-on (the brief's rule). The verifier may weigh it higher: it
  falsifies the premise of decision 29 and a sentence of the operator docs.

### HR-P4-cfg-02 — note (M for Docker's default, I for the effect) — the container image's stop has Docker's 10 s

- **Where:** `Dockerfile:1-18` (usage comment), `docs/operations.md:717-728`.
- **Claim:** The distroless image's daemon is PID 1 and gets the TERM
  (`Dockerfile:51`), but `docker stop` and Compose kill after 10 s by
  default, under the 25 s budget; nothing in the image or the docs sets or
  names `--stop-timeout` / `stop_grace_period`. Same cut as cfg-01, for
  image users.
- **Fix:** the usage comment and `docs/operations.md` name
  `--stop-timeout 30` (Compose `stop_grace_period: 30s`); `STOPSIGNAL` is
  already SIGTERM by default.
- **Guard:** a docs sentence; `TestImageDefaults` could require the flag in
  the usage comment.

### HR-P4-sup-03 — low (M for the line, I for the pull) — the daemon image's BuildKit frontend is unpinned

- **Where:** `Dockerfile:1` `# syntax=docker/dockerfile:1`;
  `ci.yml:66-68` checks `FROM` lines only.
- **Claim:** The directive makes BuildKit pull and run
  `docker.io/docker/dockerfile:1` — a moving tag — as the build's frontend
  in `docker-build-push.yml` and the CI `docker` job. Whoever controls
  that tag controls the build's output, which the release then attests
  (`docker-build-push.yml:126-139`). Decision 62 pins every base image by
  digest; this image escaped the pin and its guard. The Dockerfile uses no
  syntax the built-in frontend lacks (`addon/Dockerfile` has no directive
  and builds the same way).
- **Fix:** drop the directive (preferred: nothing needs it), or pin
  `docker/dockerfile:1@sha256:<digest>`; extend the lint job's grep to
  `# syntax=` lines.
- **Red test / guard:** P1's `TestProbeSyntaxPinned` (red at `c3a9770`),
  or the lint grep.

### HR-P4-sup-04 — note (M) — the Go version has four homes and no guard keeps them together

- **Where:** `go.mod:3`, `Dockerfile:20`, `addon/Dockerfile:75`,
  `GO_VERSION` in five workflows (`ci.yml:28`, `release-on-tag.yml:28`,
  `codeql.yml:27`, `addon-image.yml:37`, `docker-build-push.yml:30`).
- **Claim:** Dependabot's `docker` ecosystem bumps the `golang:` tag (a
  patch, outside the family — auto-merged by decision 57,
  `dependabot-auto-merge.yml:140`), but not `GO_VERSION` or `go.mod`. After
  such a merge the images are built with one Go, the release archives,
  `govulncheck` and the tests with another; the CycloneDX SBOM (built with
  the workflow's Go) then names a standard library the image does not
  carry. I for the drift happening, M for the absent guard.
- **Fix:** a lint-job check (or `TestImageDefaults` row) that the two
  `golang:` tags, every `GO_VERSION` and `go.mod`'s `go` line agree; or
  exclude `golang` from the auto-merge so the bump is one hand-made change.

### HR-P4-cfg-05 — note (M) — `WEB_ALLOWED_HOSTS` accepts white space go-fabric-ui refuses

- **Where:** `internal/config/config.go:303`, `:474-488`;
  `internal/expose/expose.go:257-266` (splits on `,`, space, `\n`, `\t`
  only); `ui:fabricui/host.go:116` (`unicode.IsSpace` refused).
- **Claim:** An entry with `\v`, `\r`, U+00A0 or U+2003 inside passes
  `config.Load` and fails `web.New` — after the node opened its store,
  exit 1 — which `allowedHost`'s comment says the start check exists to
  prevent. `FuzzLoad` checks against `allowedHost` itself, so it cannot see
  the gap.
- **Fix:** `allowedHost` refuses `strings.ContainsFunc(h, unicode.IsSpace)`
  as go-fabric-ui does; `FuzzLoad` additionally calls go-fabric-ui's
  validation (e.g. `fabricui.NewHandler(fabricui.Options{AllowedHosts: …})`
  in the fuzz body) so the two cannot drift again.
- **Red test:** P2 `TestProbeAllowedHostsSpace` (red at `c3a9770`).

## Counts

0 high · 0 medium · 2 low (cfg-01, sup-03) · 3 note (cfg-02, sup-04,
cfg-05). Held: 0.

## Not read

Nothing of the unit skipped. Read whole: `internal/web/*` (4 files),
`internal/config/*` (4 files), `cmd/ha2fabric/main.go`,
`cmd/ha2fabric/supervisor.go`, `script/run.sh`, `addon/config.yaml`,
`addon/Dockerfile`, `Dockerfile`, `.github/workflows/*.yml` (6),
`.github/dependabot.yml`, `.github/CODEOWNERS`,
`script/licenses/generate.sh`, `script/extract-release-notes.sh`. Read
in part, outside the unit, for context: `cmd/ha2fabric/supervisor_test.go:194-268`,
`cmd/ha2fabric/image_test.go:141-158`, `docs/operations.md:712-735`,
`.dockerignore`, `Makefile` (release, cite). `cmd/ha2fabric/warnings.go`
(`startWarnings`) is unit D's and was not read.

## Probes run

| Probe | Where | Result at `c3a9770` |
| --- | --- | --- |
| P1a s6 stop, base image as pinned | `docker run` of `amd64-base:3.24-2026.11.1@sha256:d4d8bc2b…`, `docker stop -t 30` | **fail** (killed after ~3 s; stop 3.7 s) |
| P1b same, `S6_CMD_RECEIVE_SIGNALS=1` | as P1a | pass (clean exit, 23 s) |
| P1c same, `S6_KILL_GRACETIME=28000` | as P1a | pass (clean exit, 28 s) |
| P1 `TestProbeAddonStopSignal` | `cmd/ha2fabric` | **fail** (red as expected) |
| P1 `TestProbeSyntaxPinned` | `cmd/ha2fabric` | **fail** (`Dockerfile` red) |
| P2 `TestProbeAllowedHostsSpace` | `internal/web` | **fail** (4 values) |
| v0.27.0 lift: `go test ./internal/config ./internal/web` | `p4/S-web27`, go-fabric v0.27.0 | pass |
| v0.27.0 lift: `make cite` | `p4/S-web27` | fails on version pins only (expected; list below) |

## v0.27.0

`gh release view v0.27.0 -R thingfabric/go-fabric`, *Changed* and *Fixed*,
against the go-fabric calls in this unit (`internal/config/node.go:46-75`
`node.Config`; `config.go:323`, `:328` `node.ValidateSetupPIN`,
`node.MaxDiscriminator`; `node.go:63` `device.AggregatorType`;
`internal/web/operation.go:19-87` `logring`):

- **No *Changed* or *Fixed* row touches a call in these files.** The
  nearest, `node: Config.TLS is a configuration error at Validate and New…`,
  concerns a field `Config.Node` never sets (`node.go:47-74`): no code or
  test change. `node: OpenOwnWindow refuses a negative timeout` and the
  `bridge` window-refusal order are go-fabric-ui's calls, not this
  unit's. M: `internal/config` and `internal/web` pass against v0.27.0
  (probe above).
- **Citations the lift re-pins** (`make cite` red on the version only; the
  lift is a text change, I): `cmd/ha2fabric/main.go:144` and
  `config.go:40` (`docs/guides/embed-in-a-daemon.md`, file unchanged),
  `config.go:63` (`fabrictest/device.go:36`, line unchanged),
  `config.go:339` (`cluster/core/basic_information.go:649-651`,
  unchanged), `node.go:44` (`bridge/tcp_listen.go:18-31`, unchanged) —
  pin only. **Ranges that moved** and need new lines (M by diff):
  `node.go:28` `node/config.go:956-965` → `checkPBKDF` now at
  `gf27:node/config.go:960-969`; `config.go:493`
  `node/config.go:1086-1092` → `gf27:node/config.go:1091-1097`;
  `config.go:143` `docs/guides/camera.md:577-599` → "The ecosystem gate"
  now from `gf27:docs/guides/camera.md:581`; `node.go:44`
  `docs/guides/camera.md:133-140` → shifted by about 4 lines (re-read at
  v0.27.0 when re-pinning).

## Questions for the owner

1. **cfg-01, how the add-on hands over the stop.** Recommendation:
   `S6_CMD_RECEIVE_SIGNALS=1` in `addon/Dockerfile` (the daemon gets the
   TERM itself and the stop lasts as long as it needs, within
   `timeout: 30`), with the guard test; rather than a fixed
   `S6_KILL_GRACETIME`, which makes every stop wait for stray processes. It
   is the best design because it puts the one budget of decision 29 back
   in the daemon, where the code and docs already place it. The owner's
   next live add-on update can confirm it: the log shows the node-stop
   lines after `ha2fabric.stopping`.
2. **sup-03.** Recommendation: drop `# syntax=docker/dockerfile:1` rather
   than pin it — nothing in the file needs it, and a pinned frontend is
   one more digest to keep current.
3. **sup-04.** Recommendation: the lint-job agreement check, and keep the
   `golang` patch auto-merge (a toolchain security patch should not wait);
   the check then turns the Dependabot pull request red until
   `GO_VERSION` and `go.mod` follow, in the same pull request.

## Appendix: probe texts

P1a–c (shell; the scratch script, run with the pinned base image):

```sh
cat > cmd.sh <<'EOF'
#!/bin/bash
echo "start $(date +%s)"
on_term() { echo "TERM $(date +%s)"; for i in $(seq 1 20); do sleep 1; echo "alive $i $(date +%s)"; done; echo "clean exit $(date +%s)"; exit 0; }
trap on_term TERM
sleep 1000 & wait
EOF
IMG=ghcr.io/home-assistant/amd64-base:3.24-2026.11.1@sha256:d4d8bc2b0b4b6158a2c8c863f2a4d20e84d98f829724bf34c3b165f8e3cee33f
for e in "" "-e S6_CMD_RECEIVE_SIGNALS=1" "-e S6_KILL_GRACETIME=28000"; do
  id=$(docker run -d $e $IMG /bin/bash -c "$(cat cmd.sh)"); sleep 4
  t0=$(date +%s); docker stop -t 30 $id >/dev/null; t1=$(date +%s)
  echo "$e: $((t1-t0)) s; $(docker logs $id 2>&1 | grep -E 'alive|clean' | tail -1)"
  docker rm -f $id >/dev/null
done
# c3a9770 base: 3.7 s, last "alive 2"; RECEIVE_SIGNALS=1: 23 s "clean exit"; KILL_GRACETIME=28000: 28 s "clean exit"
```

P1 (`cmd/ha2fabric/probe_p4_test.go`):

```go
package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestProbeAddonStopSignal (HR-P4-cfg-01): the add-on image hands the
// container's SIGTERM to the CMD (the daemon), so the stop budget is the
// daemon's and not s6-overlay's 3 s S6_KILL_GRACETIME.
func TestProbeAddonStopSignal(t *testing.T) {
	b, err := os.ReadFile("../../addon/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^ENV .*S6_CMD_RECEIVE_SIGNALS=1`).Match(b) {
		t.Fatal("addon/Dockerfile does not set S6_CMD_RECEIVE_SIGNALS=1: s6-overlay TERMs the daemon last and KILLs it 3 s later")
	}
}

// TestProbeSyntaxPinned (HR-P4-sup-03): a BuildKit frontend named by a
// `# syntax=` directive is an image the build runs; pinned by digest as the
// base images are (decision 62).
func TestProbeSyntaxPinned(t *testing.T) {
	for _, f := range []string{"../../Dockerfile", "../../addon/Dockerfile"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(string(b)) {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "# syntax="); ok && !regexp.MustCompile(`@sha256:[0-9a-f]{64}$`).MatchString(v) {
				t.Errorf("%s: frontend %q not pinned by digest", f, v)
			}
		}
	}
}
```

P2 (`internal/web/probe_p4_test.go`):

```go
package web

import (
	"io"
	"testing"

	fabricui "thingfabric.dev/go-fabric-ui/fabricui"

	"thingfabric.dev/go-ha2fabric/internal/config"
)

// TestProbeAllowedHostsSpace (HR-P4-cfg-05): a WEB_ALLOWED_HOSTS value that
// config.Load accepts must be one go-fabric-ui's New accepts, so it fails at
// start and not after the node opened its store.
func TestProbeAllowedHostsSpace(t *testing.T) {
	for _, v := range []string{"a.lan\vb.lan", "a.lan\rb.lan", "a.lan b.lan", "a.lan b.lan"} {
		c, err := config.Load([]string{"--store-path", t.TempDir() + "/s.db"}, config.MapEnv(map[string]string{
			"HA_URL": "ws://127.0.0.1/api/websocket", "HA_TOKEN": "t", "WEB_ALLOWED_HOSTS": v,
		}), io.Discard)
		if err != nil {
			continue
		}
		if _, err := New(Options{UI: fabricui.Options{Admin: ReadOnly, AllowedHosts: c.WebAllowedHosts}}); err != nil {
			t.Errorf("WEB_ALLOWED_HOSTS %q: config.Load accepted %q, web.New refused: %v", v, c.WebAllowedHosts, err)
		}
	}
}
```

Output at `c3a9770`: P1 both red, P2 red for all four values
(`fabricui: Options.AllowedHosts entry "a.lan\vb.lan" has white space`, …).
