# Brief: go-ha2fabric moves to thingfabric.dev

Owner, 2026-11-24: the repository was transferred to
`github.com/thingfabric/go-ha2fabric` (old URLs redirect); the module path becomes
`thingfabric.dev/go-ha2fabric` (go-import served by `thingfabric/thingfabric.github.io`).
Owner decisions (wizard 2026-11-24): images move to `ghcr.io/thingfabric/`;
go-fabric-ui moved to `thingfabric.dev/go-fabric-ui` v0.14.0 (npm
`@thingfabric/fabric-ui`, problem `type` URIs on
`https://thingfabric.dev/go-fabric-ui/problems#problem-<code>`, contract
0.14.0); go-hamapping, go-hamqtt, go-ha-catalog and openccu-loom stay under
`github.com/SukramJ`. **No tag, no release** (owner's release lock). Model:
Opus for every role (owner rule 2026-11-24).

Branch `chore/thingfabric-move` from `origin/main`. First commit: this brief.

## Work

| # | What |
| --- | --- |
| 1 | `go.mod` `module thingfabric.dev/go-ha2fabric`; every import; the import guard and any test naming the path; the linter's local prefix; coverage-floor script; `gofumpt` |
| 2 | `thingfabric.dev/go-fabric-ui` **v0.14.0** instead of `github.com/SukramJ/go-fabric-ui` (`go get`, `go mod tidy`, imports, `THIRD-PARTY-NOTICES.md`); its changelog "What a consumer must do"; a test or code that compares a problem `type` string follows the new base; no deprecated go-fabric identifier (staticcheck SA1019) |
| 3 | Images and add-on: `ghcr.io/sukramj/…` → `ghcr.io/thingfabric/…` (workflows, `addon/config.yaml` `image`, Dockerfiles' labels, docs); `repository.yaml` and `addon/config.yaml` `url` → `https://github.com/thingfabric/go-ha2fabric`; the add-on repository URL users add in Home Assistant, in README/docs |
| 4 | Workflows: every `github.repository_owner == 'SukramJ'` → `'thingfabric'` (without it no image or release job runs after the transfer); any other owner or repository literal |
| 5 | Links: `github.com/SukramJ/go-ha2fabric`, `…/go-fabric`, `…/go-fabric-ui`, `…/go-fabric-server`, `…/go-ha2fabric` → `github.com/thingfabric/…` (URLs) or `thingfabric.dev/…` (import paths); `SukramJ/go-hamapping`, `go-hamqtt`, `go-ha-catalog`, `openccu-loom` stay; copyright headers, `CLA.md`, authors and Dependabot reviewers stay (they name the person); `notes/` history and past reports stay |
| 6 | `changelog.md` `[Unreleased]` (or the unreleased section the project uses): Changed — module path, images, add-on repository URL (breaking for anyone who pulls the image or added the old add-on URL), go-fabric-ui v0.14.0; `CLAUDE.md` / `README.md` state lines |

## Rules

No sub-agents; no push, pull request, tag or release; never `git stash`;
`PATH=$HOME/go/bin:$PATH`; edit by script with asserted anchors;
generated files regenerated. The local guard set of `CLAUDE.md` (what
`.github/workflows/ci.yml` runs, including the image builds) before the
report, output to log files under
`/tmp/claude-1000/-home-markus-Dokumente-GitHub/8fe7f689-a76f-424d-88a1-c6d98442f53c/scratchpad/M-go-ha2fabric/`
(slow machine; read summaries). A `git grep -n -i sukramj` outside
`notes/` lists only what row 5 keeps. Report
`notes/briefs/2026-11-24-M-report.md` (commits, guards with HEAD, per row,
what stays under SukramJ and why, questions), committed last. Stop when it
is written; leave no background work running.
