# Brief H3 — label-based exposure in go-ha2fabric, the go-hamapping follow-ups of H2 (Q3–Q5), the matter-hub comparison

Date 2026-10-25. Sources: go-ha2fabric's report `docs/reports/H2.md` §Questions
(Q3–Q5 go-hamapping, Q7), the read map `read-h3-labels-matterhub.md` (copy to
`notes/reads/` of each repository you touch, in its first commit; a map with
file:line — verify every line before acting), concept v8
(`go-fabric/docs/concept-v8-fabric-server.md` B.1, B.3), the owner decisions
of 2026-10-25 below. **Reference for go-ha2fabric's behaviour** (owner
2026-10-25): [home-assistant-matter-hub](https://github.com/RiDDiX/home-assistant-matter-hub),
local clone `/home/markus/Dokumente/GitHub/home-assistant-matter-hub` at
`664575a` — a helper reference for "how does a real HA→Matter bridge
behave" (cite `file:line` at that commit), never a gold standard for Matter
behaviour (that stays matter.js, reached only through go-fabric's facade).
Home Assistant facts come from `home-assistant/core` at tag `2026.10.3`,
read as raw files (the read map's §2 names the files and lines; the
downloads are under the scratchpad's `hacore/`, never executed).

**Model:** Opus. **Two pull requests in sequence, one agent.** English in
everything that lands in a repository. Never add attribution lines. go-fabric's
`CLAUDE.md` token rules apply. No auto-merge; the owner merges and tags.
No Matter knowledge of your own in go-ha2fabric: which entity becomes which
device type is go-hamapping's `Map`.

**Owner decisions (2026-10-25):** (D1) a label value in the options matches
a Home Assistant label by **name, case-insensitively, else as `label_id`**;
**entity labels and device labels both count** (a device label exposes
every entity of the device). (D2) H3 carries the go-hamapping follow-ups
Q4 and Q5 (and documents Q3) in a go-hamapping PR first, then go-ha2fabric.
(D3, standing) exposure stays Assist ∪ include − exclude, disabled and hidden
never; labels extend include and exclude.

## Part 1 — go-hamapping (branch `h3/identity-and-methods`, version `v0.2.0`)

Repository `/home/markus/Dokumente/GitHub/go-hamapping`, main at `v0.1.0`.
Read its `CLAUDE.md`, `CONCEPT.md`, the row file format (`internal/rowfile`),
`plan.go:236-259`, `binding.go:232-239`, `value.go`, and how its CI and
release work (whether a version constant or changelog section gates a tag;
do what the repository's own pipeline needs).

- **1.1 Stable identity (H2 Q4).** Today `StableKey` is `device uid + "/" + e.Key()`
  (`plan.go:242-257`) and `Key()` is the entity id, so a renamed entity
  renumbers its endpoint. Add an explicit identity the source supplies:
  read go-hamqtt `model` (`~/go/pkg/mod/github.com/!sukram!j/go-hamqtt@v0.38.0/model/entity.go`)
  for a field or method that already carries a stable identity (unique id,
  registry id); if one exists, `Map` prefers it for `StableKey` and keeps
  `Key()` for addresses and verdicts; if none exists, `Map` takes it as an
  option (`Options.StableKey func(model.Entity) string`, default `Key()`)
  and the report says which route was taken and why. Changing the key of an
  existing consumer renumbers once — say so in the changelog as a note for
  go-ha2fabric (it moves in Part 2, before its first tagged release is in
  anyone's hands long).
- **1.2 Method names (H2 Q5).** Exported constants for every `method` the
  rows use (`rows_gen.go`: direction, mode, oscillation, percentage,
  preset_mode, set_position, temperature, temperature_high, temperature_low,
  tilt), generated from the rows by `internal/gen` so a new row's method
  appears without a hand edit, with a test that every row method has a
  constant. go-ha2fabric's `internal/adapt/command.go:59-125` uses them in
  Part 2.
- **1.3 The Extra/State contract for a non-MQTT source (H2 Q3).** Document,
  in `docs/rows.md` (generated) or a page beside it, which `state.<key>` and
  `extra.<key>` names the rows read per platform (generated from the rows:
  one table platform → keys → whether the key has a catalog default), so a
  source that is not MQTT knows what to fill. No row changes.
- Tests: the existing fabrictest round trips still pass; a test that
  renaming the entity id keeps the `StableKey` when the stable identity is
  set; the generator staleness test covers the constants and the table.
- Changelog `## [0.2.0]`, version constant if the repository has one; PR
  body = report (shape below). After the owner tags `v0.2.0`, Part 2 pins it
  (until then, a `replace` or pseudo-version in Part 2's worktree only —
  the Part 2 PR must pin the tag; wait for the launcher's word that the tag
  exists before opening the Part 2 PR).

## Part 2 — go-ha2fabric (branch `h3/labels`, version `v0.2.0`)

Repository `/home/markus/Dokumente/GitHub/go-ha2fabric`, main at `v0.1.0`.
Read its `CLAUDE.md`, `CONCEPT.md`, `docs/reports/H2.md`, `internal/expose`,
`internal/bridge/bridge.go:227-239, 289-311, 329-370, 444-466`,
`internal/ha/client.go:405-500`, `internal/ha/message.go`,
`internal/config/config.go`, `addon/config.yaml`, `script/run.sh`.

- **2.1 Label registry in the HA client.** `config/label_registry/list`
  (`components/config/label_registry.py:53-67`; payload `_entry_dict`
  `:155-166`: label_id, name, icon, color, description, created_at,
  modified_at; not admin) as `ListLabels`; `labels []string` on
  `EntityEntry` (`helpers/entity_registry.py:264`, `:393` in the list
  payload) and `DeviceEntry` (`helpers/device_registry.py:412`, `:616`);
  subscribe `label_registry_updated` (`helpers/label_registry.py:23-26`,
  data `{action: create|remove|update, label_id}` `:50-54`) beside the two
  registry events, routed to `resyncSoon`. A label rename is an `update`
  event: the resync re-resolves names. Fixtures under `testdata/ha/` with
  the file:line they mirror.
- **2.2 Exposure with labels.** `expose.Rule` gains `IncludeLabels`,
  `ExcludeLabels []string`; `expose.Entity` gains `Labels` (entity labels ∪
  its device's labels, as label ids) and the rule takes a resolver
  (name → label_id, case-insensitive, from the label registry; an unresolved
  value is used as a label_id as given — D1). Decide order stays: disabled,
  hidden, exclude (ids, globs, labels), include (ids, globs, labels), Assist.
  `Decision.Reason` names the label ("included by label 'Matter'"). matter-hub
  comparison: it slugifies an unresolved name (`matches-entity-filter.ts:265-270`)
  — we do not; say so in the report's by_design list. Validate: a label
  value that resolves to nothing is a warning in the log and in the plan
  view, not an error (labels are created later).
- **2.3 Bug: disabled and hidden entities.** `bridge.go:363` builds
  `expose.Entity` without `DisabledBy`/`HiddenBy`, so the rule's first two
  checks never fire. Set them from the registry entry; a test that a
  disabled entity gets no endpoint and the verdict says why. Check whether
  H2's end-to-end "registry disable removes the endpoint" passed for another
  reason and say which.
- **2.4 Options and config.** Add-on options `include_labels: []`,
  `exclude_labels: []` (`addon/config.yaml` schema `[str]`), env/flags
  `INCLUDE_LABELS`/`EXCLUDE_LABELS`, `run.sh` mapping, `addon/DOCS.md` and
  README exposure section (one worked example: label "Matter" on a device).
  The web UI's plan view shows the entity's labels and the reason.
- **2.5 go-hamapping v0.2.0 and go-fabric v0.12.0.** Pin both. Use the
  method constants in `internal/adapt/command.go`; set the stable identity
  (1.1) from the registry entry id (`EntityEntry.ID`, `helpers/entity_registry.py:239-264`
  — confirm the field that is stable across renames) so a renamed entity
  keeps its endpoint number; a test. Note in the changelog that existing
  endpoint numbers change once at this upgrade (a controller re-learns the
  bridge; say what Apple Home does on a renumbering as far as the H2 report
  and matter-hub's issues #468/#404 at `bridge-endpoint-manager.ts:949-965`
  tell — cite, do not guess).
- **2.6 The matter-hub comparison** (the H2 report's missing section).
  `docs/by_design.md` (create if absent) with one paragraph per difference
  against matter-hub at `664575a`, cited: exposure model (its matcher types,
  include/exclude, `includeMode`; ours Assist ∪ include − exclude with
  labels), label refresh (its 60 s poll `start-options-builder.ts:84-89`,
  `home-assistant-registry.ts:100-131`; our subscription), hidden entities
  (its flag `bridge-registry.ts:1009-1012`; ours never), unknown state
  (its `basic-information-server.ts:93-95` reachable on "unknown"; ours per
  H2 — state which), removal grace (its 300 s `bridge-endpoint-manager.ts:59`;
  ours immediate — say whether a grace is wanted as a question), naming
  precedence (`basic-information-server.ts:44-48`; ours via
  `device.ComposeNodeLabel`), composition (its auto-composed devices
  `bridge-registry.ts:1032-1105`; ours go-hamapping's rows). `CONCEPT.md:12`
  and `CLAUDE.md` name matter-hub as the helper reference (and the archived
  fork `SukramJ/home-assistant-matter-bridge` is not mentioned).
- **2.7 Docs.** `docs/operations.md` label section; changelog `## [0.2.0]`;
  `internal/version` 0.2.0; `addon/config.yaml` version 0.2.0
  (`make version-check`).
- Tests: `internal/expose` table (labels, resolver, precedence, unresolved
  name), `internal/ha` fake with the label list and event, end-to-end:
  a device label exposes two entities, removing the label (event) removes
  the endpoints, renaming the entity keeps the endpoint number, a disabled
  entity never appears. Guards as H2 (ci, race, cover-check, fuzz-smoke,
  arch, docker build of both images, release dry run, `make version-check`).

- **2.8 The web interface from go-fabric-ui (concept v9, owner 2026-10-25).**
  Pin `github.com/SukramJ/go-fabric-ui` `v0.1.0` (read its `README.md`,
  `docs/embedding.md`, `docs/provider.md`, `fabricui/options.go`,
  `fabricui/provider.go`). Replace `internal/web` by `fabricui.Handler(fabricui.Options{...})`
  on the Ingress listener: `Node` = the daemon's `*node.Node`, `Admin` =
  the Ingress source check (`172.30.32.2`; stand-alone: a `WEB_ADMIN`
  bool, default false, documented), `Reassemble` = a full resync,
  `Events` = the daemon's exposure changes, `Logger`. Keep `GET /api/health`
  and `GET /api/plan` as thin aliases for one release (H2 documented them)
  or drop them with a changelog line — decide and say. Delete the H2 QR
  encoder and its golden test only if nothing else uses them.
  - **HA `ExposureProvider`** (`internal/exposeui` or inside `internal/bridge`):
    `Schema` columns entity_id, name, domain, device, area, labels,
    assist, platform; filters domain, area, label, verdict state, source;
    `Groups` by device. `List` = every registry entity with its
    go-hamapping verdict (from the last dry run), `Enabled` = exposed,
    `Source` = one of `rule:assist`, `rule:include`, `rule:label`,
    `rule:exclude`, `override:on`, `override:off`, `disabled`, `hidden`.
    `Preview(updates)` = the rule plus the proposed overrides through
    `hamapping.Map` without `Build` (verdicts only). `Apply(updates)`
    writes the overrides and triggers a resync.
  - **Overrides under `/data/selection.json`** (owner decision 0.4 of
    concept v9): `{version: 1, entities: {<entity_id>: {enabled: bool,
    name?: string}}}`, written atomically (temp + rename), read at start;
    layered above the options: disabled/hidden → override → exclude →
    include/labels → Assist. `--dump-plan` and the plan view show the
    source. The add-on's `backup: cold` already covers the file; document
    it in `docs/operations.md`.
  - Tests: provider table tests (sources, preview equals apply's result,
    atomic write, a corrupt file is renamed aside and logged, not fatal);
    e2e: open the window through the UI's API and commission a site
    controller; an override through PUT removes an endpoint and the SSE
    stream reports it.
  - by_design: how matter-hub stores its filter (`bridge-storage`,
    `bridge-service.ts:318-330`) versus the daemon-owned file; one paragraph.

## Report (each PR body; also `docs/reports/H3-mapping.md` and `docs/reports/H3.md`)

Per §: what landed with commit hashes; guards with HEAD; differences from
the brief; decisions; left out; questions (gaps with the symbol needed —
go-fabric gaps go to the owner for CJ/CK); by_design candidates with the
matter-hub citations. Hand back after Part 1's PR with its number (the
owner tags v0.2.0), continue Part 2 in the meantime on a pseudo-version,
and hand back finally with both PR numbers. If a permission is denied, say
which and stop that step; never ask the launcher to perform it. Copy this
brief to `notes/briefs/2026-10-25-H3-labels-and-mapping-followups.md` in
both repositories' first commits.
