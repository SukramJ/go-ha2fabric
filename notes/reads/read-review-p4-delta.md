# Phase 4 delta map: d16f3ab..c3a9770

By script (git diff --numstat), a map never a citation. Phase 2's merge d16f3ab to phase 3's merge c3a9770: #76 move to thingfabric.dev, #78 R7 (go-fabric v0.26.0, batch settle, camera idle end dropped), #79 release 0.7.0, #80 sukramj.dev paths, #81 doc-30, #82 phase 3.

## Commits

- `c3a9770` Review phase 3: architecture, tests, docs, supply chain (#82)
- `4f5f6e3` docs(changelog): keep the old add-on repository address and update in place (#81)
- `0a47095` chore(deps): move Go libraries to sukramj.dev import paths (#80)
- `e78707a` chore(release): 0.7.0 (#79)
- `a2b00bb` R7: go-fabric v0.26.0, go-fabric-ui v0.14.1, go-hamapping v0.7.1; batch settle from the node's set; camera idle end dropped (0.7.0) (#78)
- `f7885e6` chore: move to thingfabric.dev/go-ha2fabric, images under ghcr.io/thingfabric, go-fabric-ui v0.14.0 (0.7.0) (#76)
- `0da8848` chore(ci): bump actions/setup-go (#75)

## Hand-written code and build files (added/removed lines)

| + | - | file |
| ---: | ---: | --- |
| 3 | 0 | `.github/CODEOWNERS` |
| 40 | 8 | `.github/dependabot.yml` |
| 95 | 54 | `.github/workflows/addon-image.yml` |
| 41 | 15 | `.github/workflows/ci.yml` |
| 23 | 12 | `.github/workflows/codeql.yml` |
| 24 | 5 | `.github/workflows/dependabot-auto-merge.yml` |
| 89 | 37 | `.github/workflows/docker-build-push.yml` |
| 48 | 9 | `.github/workflows/release-on-tag.yml` |
| 1 | 1 | `.golangci.yaml` |
| 4 | 2 | `CLAUDE.md` |
| 6 | 4 | `CONCEPT.md` |
| 15 | 6 | `Dockerfile` |
| 22 | 8 | `Makefile` |
| 38 | 20 | `README.md` |
| 33 | 0 | `SECURITY.md` |
| 33 | 19 | `THIRD-PARTY-NOTICES.md` |
| 17 | 5 | `addon/CHANGELOG.md` |
| 21 | 9 | `addon/DOCS.md` |
| 27 | 14 | `addon/Dockerfile` |
| 4 | 4 | `addon/README.md` |
| 11 | 8 | `addon/config.yaml` |
| 98 | 15 | `changelog.md` |
| 20 | 13 | `cmd/ha2fabric/main.go` |
| 20 | 19 | `cmd/ha2fabric/nodes.go` |
| 119 | 21 | `cmd/ha2fabric/supervisor.go` |
| 2 | 2 | `cmd/ha2fabric/warnings.go` |
| 14 | 9 | `docs/by_design.md` |
| 56 | 15 | `docs/operations.md` |
| 34 | 28 | `docs/roadmap.md` |
| 6 | 6 | `go.mod` |
| 5 | 5 | `internal/adapt/adapt.go` |
| 12 | 12 | `internal/adapt/command.go` |
| 84 | 20 | `internal/bridge/bridge.go` |
| 3 | 3 | `internal/bridge/camera.go` |
| 4 | 4 | `internal/bridge/command.go` |
| 29 | 63 | `internal/camera/camera.go` |
| 14 | 12 | `internal/config/config.go` |
| 4 | 4 | `internal/config/node.go` |
| 7 | 7 | `internal/exposure/exposure.go` |
| 2 | 2 | `internal/ha/camera.go` |
| 15 | 2 | `internal/ha/client.go` |
| 51 | 0 | `internal/ha/hatest/camera.go` |
| 22 | 1 | `internal/ha/hatest/hatest.go` |
| 2 | 2 | `internal/ha/message.go` |
| 5 | 4 | `internal/nodes/nodes.go` |
| 4 | 4 | `internal/plan/input.go` |
| 5 | 5 | `internal/plan/plan.go` |
| 1 | 1 | `internal/selection/selection.go` |
| 1 | 1 | `internal/version/version.go` |
| 1 | 1 | `internal/web/operation.go` |
| 2 | 2 | `internal/web/web.go` |
| 2 | 2 | `repository.yaml` |
| 583 | 0 | `script/cite/main.go` |
| 21 | 14 | `script/coverfloor/floors.txt` |
| 58 | 13 | `script/coverfloor/main.go` |
| 1 | 1 | `script/extract-release-notes.sh` |
| 116 | 0 | `script/licenses/generate.sh` |
| 4 | 1 | `script/run.sh` |
| 1 | 1 | `testdata/ha/get_states.json` |

## Tests and testdata

| + | - | file |
| ---: | ---: | --- |
| 6 | 6 | `cmd/ha2fabric/admin_test.go` |
| 2 | 2 | `cmd/ha2fabric/exits_test.go` |
| 88 | 18 | `cmd/ha2fabric/image_test.go` |
| 4 | 4 | `cmd/ha2fabric/main_test.go` |
| 5 | 5 | `cmd/ha2fabric/nodes_test.go` |
| 24 | 6 | `cmd/ha2fabric/stop_test.go` |
| 104 | 0 | `cmd/ha2fabric/supervisor_test.go` |
| 3 | 3 | `cmd/ha2fabric/warnings_test.go` |
| 2 | 2 | `cmd/ha2fabric/web_test.go` |
| 5 | 5 | `internal/adapt/adapt_test.go` |
| 5 | 5 | `internal/adapt/fuzz_test.go` |
| 49 | 18 | `internal/arch/arch_test.go` |
| 298 | 0 | `internal/arch/ast_test.go` |
| 5 | 4 | `internal/bridge/bridge_c1_e2e_test.go` |
| 1 | 1 | `internal/bridge/bridge_c2_e2e_test.go` |
| 6 | 6 | `internal/bridge/bridge_e1_e2e_test.go` |
| 12 | 12 | `internal/bridge/bridge_e2e_test.go` |
| 1 | 1 | `internal/bridge/bridge_f2_e2e_test.go` |
| 4 | 4 | `internal/bridge/bridge_h3_e2e_test.go` |
| 3 | 3 | `internal/bridge/bridge_p1_e2e_test.go` |
| 2 | 2 | `internal/bridge/bridge_p2_e2e_test.go` |
| 5 | 5 | `internal/bridge/bridge_p2r2_e2e_test.go` |
| 6 | 6 | `internal/bridge/bridge_p2r3_e2e_test.go` |
| 3 | 3 | `internal/bridge/bridge_r5_e2e_test.go` |
| 41 | 5 | `internal/bridge/bridge_r6_e2e_test.go` |
| 145 | 0 | `internal/bridge/bridge_r7_e2e_test.go` |
| 3 | 3 | `internal/bridge/bridge_test.go` |
| 7 | 7 | `internal/bridge/bridge_u6_e2e_test.go` |
| 10 | 1 | `internal/bridge/export_test.go` |
| 6 | 6 | `internal/bridge/fuzz_test.go` |
| 5 | 5 | `internal/bridge/queue_internal_test.go` |
| 3 | 3 | `internal/bridge/sync_internal_test.go` |
| 34 | 41 | `internal/camera/bounds_test.go` |
| 3 | 3 | `internal/camera/camera_test.go` |
| 1 | 1 | `internal/camera/close_test.go` |
| 1 | 1 | `internal/camera/fuzz_test.go` |
| 3 | 3 | `internal/camera/ha_test.go` |
| 7 | 7 | `internal/exposure/exposure_test.go` |
| 67 | 2 | `internal/ha/camera_test.go` |
| 52 | 1 | `internal/ha/client_hostile_test.go` |
| 42 | 2 | `internal/ha/client_p1_test.go` |
| 2 | 2 | `internal/ha/client_test.go` |
| 2 | 2 | `internal/ha/client_url_test.go` |
| 1 | 1 | `internal/ha/excerpt_test.go` |
| 1 | 1 | `internal/ha/fanout_test.go` |
| 1 | 1 | `internal/ha/snapshot_test.go` |
| 1 | 1 | `internal/ha/state_cap_test.go` |
| 1 | 1 | `internal/nodes/fuzz_test.go` |
| 1 | 1 | `internal/nodes/modes_test.go` |
| 2 | 2 | `internal/nodes/nodes_test.go` |
| 1 | 1 | `internal/nodes/passcode_test.go` |
| 1 | 1 | `internal/plan/diff_test.go` |
| 7 | 7 | `internal/plan/plan_test.go` |
| 4 | 4 | `internal/plan/skipped_test.go` |
| 1 | 1 | `internal/selection/fuzz_test.go` |
| 1 | 1 | `internal/text/text_test.go` |
| 1 | 1 | `internal/web/operation_test.go` |
| 1 | 1 | `internal/web/web_test.go` |
| 210 | 0 | `script/cite/main_test.go` |
| 99 | 0 | `script/coverfloor/main_test.go` |
