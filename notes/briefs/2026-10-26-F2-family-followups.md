# Brief F2 — the F1 follow-ups: go-fabric-ui v0.1.3 (list-valued fields), go-fabric-server v0.3.2 (`forget`), go-ha2fabric v0.2.3 (Forget on registry remove, list fields)

Date 2026-10-26. Owner decisions at the F1 reports (2026-10-26): the
comma-in-label problem is solved in go-fabric-ui now; the /bridge
protocol gets its own `forget` command and `remove` keeps the number;
go-ha2fabric forgets the number of an entity removed from the registry;
the add-on Bluetooth option is a later bundle (S4). Issues: go-fabric-ui
#10, go-fabric-server #8, go-ha2fabric #10 — each is the specification of
its part; this brief only orders them. **Model:** Opus. Conventional
commits, English, no attribution lines, no auto-merge, no label; each
repository's `make ci` plus race, cover-check and whatever its Makefile
offers (version-check, release dry run, image builds for the daemons);
`df -h /` before guard runs, `go clean -cache -testcache` only when disk
is tight; go-fabric's `CLAUDE.md` token rules (slices, background guards
with a summary log, short report). One hand-back per part: PR number,
HEAD, guards with results, differences from the issue, Questions
verbatim. Copy this brief to `notes/briefs/` of each repository in its
first commit.

## 1 go-fabric-ui `v0.1.3` — issue #10

Branch `f2/list-fields` from main (v0.1.2). `Item.Lists map[string][]string`
(JSON `lists`) as the issue proposes; `Fields[key]` comma form stays
accepted for a `Multi` column until v0.2.0 (deprecated); `Lists` wins when
both are present. Both sides: the Go provider contract (`fabricui/
provider.go`, `memory_provider`, handlers) and the Svelte page
(`ui/src/lib/pages/node/fields.ts` and its callers); tests on both sides;
`docs/embedding.md`; changelog `## [0.1.3] - 2026-10-26`; the npm package
version if the Makefile's version check reads it. Independent of §2.

## 2 go-fabric-server `v0.3.2` — issue #8

Branch `f2/forget` from main **after** `v0.3.1` is tagged (fetch and
check `git tag -l v0.3.1` first; poll once a minute up to 30 minutes if it
is not there yet). The `forget` command as the issue specifies; `declare`
unchanged; `docs/protocol.md`; changelog `## [0.3.2] - 2026-10-26`;
`internal/version`, `addon/config.yaml`, `addon/CHANGELOG.md`;
`make version-check`; the compat job green. Independent of §1 and §3.

## 3 go-ha2fabric `v0.2.3` — issue #10

Branch `f2/forget-and-lists` from main (v0.2.2) **after** go-fabric-ui
`v0.1.3` is tagged. Pin go-fabric-ui v0.1.3; fill `Item.Lists["labels"]`;
`Forget` on `entity_registry_updated` `remove`; tests, docs, changelog
`## [0.2.3] - 2026-10-26`; `make version-check`; image builds and the
release dry run; a short `docs/reports/F2.md`.
