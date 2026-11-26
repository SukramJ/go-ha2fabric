# Phase 4 run 1, fix wave — F2 (images, workflows, docs)

Brief `notes/briefs/2026-11-26-review-p4-fix.md`, section F2. Branch
`review/p4-F2` from `ccc4104`; commits `f53b4c0..26d019a` plus this report.
Opus, no sub-agents.

## Rows

| Row | Commit | Status |
| --- | --- | --- |
| HR-P4-cfg-01 (decision 75) | `f53b4c0` | fixed: `ENV S6_CMD_RECEIVE_SIGNALS=1` in `addon/Dockerfile`'s final stage; `script/run.sh:7`, `addon/config.yaml` (timeout comment), `docs/operations.md` (stop paragraph) corrected; changelog *Fixed*. Guard: a Go test, text below, for `cmd/ha2fabric/image_test.go` (F1's path), **not** in the lint job |
| HR-P4-cfg-02 | `f53b4c0` | fixed: `--stop-timeout 30` (Compose `stop_grace_period: 30s`) in `Dockerfile`'s usage, `README.md`'s run line, `docs/operations.md` |
| HR-P4-sup-03 | `712cdf1` | fixed: `# syntax=` dropped from `Dockerfile`; the lint step "actions by full SHA, base images by digest" refuses a syntax directive (any case, spaces) not pinned by `@sha256:` |
| HR-P4-sup-04 | `712cdf1` | fixed: lint step "one Go version in go.mod, both Dockerfiles and every workflow": `go.mod`'s `go` (and `toolchain`, if any), each Dockerfile's `FROM golang:<v>-`, every workflow's `GO_VERSION` that uses `env.GO_VERSION`; `dependabot.yml` comment |
| HR-P4-doc-24 | `f53b4c0` | fixed: `docs/operations.md`, The token: `SSL_CERT_FILE` / `SSL_CERT_DIR` (Linux, Go's roots; the image: mount and variable; no skip option). Add-on docs unchanged (no stand-alone there). Default transport confirmed (`internal/ha/client.go:175`, `:192`) |
| Decision 69 re-pins | `8f3355c` | each range re-read in the v0.27.0 module cache: `camera.md:577-599` → `:581-603` (by_design, operations), `camera.md:133-140` → `:137-144` (operations), roadmap `provider_peer.go:51-63` → `:56-67`; unchanged files re-pinned only (`levelcontrol/values.go`, `chime.go`, `docs/guides/operations.md`, `NOTICE-matter.js.txt`, `camera_gen.go`, `avstream.go`); `THIRD-PARTY-NOTICES.md` pinned `v0.27.0`; changelog *Changed* "go-fabric v0.26.0 → v0.27.0". `docs/operations.md:42` (0.7.0's upgrade note) is history and keeps v0.26.0 |
| Decision 72 docs | `1dc3f9a` | written from the decision: operations.md Cameras bullet (8 per camera, 4 per fabric, eviction of the offering fabric's own idle session, else RESOURCE_EXHAUSTED, never another fabric's) and the `camera.session_evicted` log row. `by_design.md` not touched (see questions) |
| Decision 74 docs | `1dc3f9a` | operations.md Nodes "Delete" (500, record and files kept, second Delete retries; names the existing `ha2fabric.node_stop` warn) and the erase paragraph's "its Delete works"; `addon/DOCS.md` one clause |
| F1's changelog lines | `26d019a` | drafted from the rows: *Changed* bound-20; *Fixed* err-40, err-64, cfg-05, bound-21 + data-22. Separate commit so the coordinator can adjust or drop it |

## Guards (HEAD)

- Combined check: a temporary detached worktree at F1's lift `fe9cc38` with
  F2's commits cherry-picked (clean, HEAD `59a18b3`, removed afterwards):
  `make ci` exit 0 (with `cite` and `licenses-check` at v0.27.0),
  `make version-check` 0, `make release` 0.
- `make docker addon-image` on `review/p4-F2` `26d019a` (the Docker
  daemon cannot read the scratch directory): both exit 0. Add-on image
  env has `S6_CMD_RECEIVE_SIGNALS=1`; stop check (S-web P1's TERM handler
  that needs 20 s, `docker stop -t 30`): **23 s, `clean exit`** (the base
  image without the variable: killed at about 4 s, verifier's re-run).
  Images removed afterwards.
- The two lint steps, extracted from `ci.yml` and run with
  `bash -eo pipefail`: red at `f53b4c0` on `Dockerfile:1 # syntax=…`
  (red first), green after. Mutations bite: an addon `golang:1.27.5`,
  `go 1.27.5` in go.mod, one workflow's `GO_VERSION`, a `toolchain` line,
  `golang:1.27.40`, `#Syntax = …` unpinned; a digest-pinned frontend
  passes.
- `actionlint`: not installed, not run (YAML parsed with Python).
- On `review/p4-F2` alone `make cite` / `licenses-check` are red by design
  (go.mod still v0.26.0); they pass with F1's lift (above).

## cfg-01 guard text for `cmd/ha2fabric/image_test.go`

Checked in the combined worktree: passes; red with the `ENV` line
removed. Needs `"slices"` in the imports (sorted, for gofumpt).

```go
// TestAddonStopSignal (HR-P4-cfg-01, decision 75): the add-on's final
// stage hands the container's SIGTERM to the CMD (S6_CMD_RECEIVE_SIGNALS=1)
// and run.sh exec's the daemon into that process, so the stop budget is
// the daemon's and not s6-overlay's 3 s S6_KILL_GRACETIME.
func TestAddonStopSignal(t *testing.T) {
	b, err := os.ReadFile("../../addon/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	set := false
	for _, in := range instructions(string(b)) {
		f := strings.Fields(in)
		switch {
		case len(f) == 0:
		case strings.EqualFold(f[0], "FROM"):
			set = false // an ENV holds in its own stage only
		case strings.EqualFold(f[0], "ENV") && slices.Contains(f[1:], "S6_CMD_RECEIVE_SIGNALS=1"):
			set = true
		}
	}
	if !set {
		t.Error("addon/Dockerfile's final stage does not set ENV S6_CMD_RECEIVE_SIGNALS=1: s6-overlay TERMs the daemon last and KILLs it 3 s later")
	}
	r, err := os.ReadFile("../../script/run.sh")
	if err != nil {
		t.Fatal(err)
	}
	last := ""
	for line := range strings.Lines(string(r)) {
		if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "#") {
			last = l
		}
	}
	if last != "exec /usr/bin/ha2fabric" {
		t.Errorf("script/run.sh ends with %q, not exec /usr/bin/ha2fabric: the signal would reach the shell, not the daemon", last)
	}
}
```

## Differences from the rows

- cfg-01's guard is a Go test beside `TestAddonStopTimeout` (it runs in
  `make ci` on every OS and also holds `run.sh`'s `exec`), not a lint step.
- cfg-02: the flag is in the usage line itself, not a separate sentence.
- sup-04 kept the `golang` patch auto-merge (S-web's recommendation); the
  check makes such a Dependabot pull request red until it is completed by hand.

## Left for the coordinator

1. Add the test above to `cmd/ha2fabric/image_test.go` (F1's path).
2. Check decision 72's and 74's docs (`1dc3f9a`) and F1's changelog lines
   (`26d019a`) against F1's code: the semantics at the per-fabric share
   (does an offer at the fabric's 4 evict its own idle session, or is it refused?),
   whether Delete still leaves the fabrics before a failed stop, the log
   event on a failed stop (`ha2fabric.node_stop` assumed), whether the
   refused node is listed as not enabled, and err-41 (web UI reason) if F1
   makes it user-visible.
3. go-fabric-ui v0.14.1 and go-hamapping v0.8.0 stayed with
   `go get go-fabric@v0.27.0` (scratch check); if F1's tidy moves either,
   `THIRD-PARTY-NOTICES.md` and citations follow.
4. `addon/CHANGELOG.md` has no unreleased section; its entry comes with
   the tag (decision 71).

## Questions

- `docs/by_design.md` for decision 72: does matter-hub bound sessions per
  fabric? That is a matter-hub fact and needs the Fable-role read; not
  written here.

## Tokens

About 150 k.
