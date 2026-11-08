# Brief C2 (go-ha2fabric) — script, scene and automation as plug-in units: the adapter's services, fixtures, e2e (0.5.6)

Owner decision (2026-11-08): the host side of go-hamapping brief C2 (rows
`script/plug`, `scene/plug`, `automation/plug`, OnOffPlugInUnit that always
reads off, release v0.4.5). Starts after the v0.4.5 tag. One pull request on
`feat/action-plugs`, squash-merged, auto-merge; release 0.5.6 folded in
(user-visible: scripts become endpoints).

Sources, verified 2026-11-08 (cite, do not re-derive):
- Home Assistant 2026.10.3: `script/services.yaml:4-8` (`script.turn_on`,
  no fields), `scene/services.yaml:3-14` (`scene.turn_on`, optional
  `transition` — not sent), `automation/services.yaml:22-30`
  (`automation.trigger`, `skip_condition` default true — not sent). Read
  from raw files, never run.
- go-hamapping v0.4.5 `rows/script.yaml`, `rows/scene.yaml`,
  `rows/automation.yaml` (methods `turn_on`, `turn_on`, `trigger`; payload a
  constant the service does not need), `docs/by_design.md` "A script, scene
  or automation is a plug-in unit that always reads off".
- This repo (main after 0.5.5): `internal/adapt/command.go:50-70` — the
  `services` map by domain: the method go-hamapping's row delivers and the
  service, "the payload is the row's constant and carries nothing the
  service needs" (the `button` entry `:55` is the pattern);
  `internal/adapt/adapt.go:147` `EntityPlatform: hacatalog.Platform(domain)`
  (any domain passes). Fixtures `testdata/ha/*.json` (cover entries of C1 as
  the form; every file cites the core lines it mirrors). e2e pattern
  `internal/bridge/bridge_c1_e2e_test.go` (`coverInvoke`, `nextCall`).
  `docs/roadmap.md:334-339` "Not planned" names `scene` and `script`.

## §1 Bump and adapter

`go get github.com/SukramJ/go-hamapping@v0.4.5 && go mod tidy`; notices pin.
`internal/adapt/command.go` `services`: `"script": {hamapping.MethodTurnOn:
"turn_on"}`, `"scene": {hamapping.MethodTurnOn: "turn_on"}`, `"automation":
{hamapping.MethodTrigger: "trigger"}`, each with its core citation as the
existing entries. Nothing else in `adapt` (the state is not read by the
row). `internal/adapt` is Fable's row in the model table — these three
entries are facts about Home Assistant: the implementing agent takes them
from this brief verbatim and cites the lines given; it writes no other.

## §2 Fixtures and e2e

Add to `testdata/ha/` one script (`script.markise_einfahren`, state `off`),
one scene (`scene.abend`, state a timestamp string or `unknown`), one
automation (`automation.nachtlicht`, state `on`), registered and exposed,
with the core lines cited in each file's `_mirrors` as C1 did; check the
count-based assertions (`grep -n 'candidates\|endpoints' internal/bridge/*_test.go cmd/ha2fabric/*_test.go`)
and adjust with the reason. `internal/bridge/bridge_c2_e2e_test.go`: each
has an endpoint of type OnOffPlugInUnit (`fabrictest.ReadCluster`
Descriptor DeviceTypeList, or the pattern the c1 test uses), `OnOff` reads
false; `On` → `WaitCall` `script.turn_on` / `scene.turn_on` /
`automation.trigger` on the entity; after the call `OnOff` reads false
again; a `state_changed` to `on` (the script running) leaves `OnOff` false.

## §3 Docs, roadmap, changelog, release

`docs/roadmap.md:334-339`: `scene` and `script` leave "Not planned" with a
line under a new "C — Actions as plug-in units (2026-11-08)": rows in
go-hamapping v0.4.5, automation with them, matter-hub parity (its
`legacy/script|scene|automation/index.ts` @664575a). `docs/operations.md`:
one sentence where exposure is explained — a script, scene or automation
appears as a plug that reads off; switching it on runs it. `changelog.md`
`### Added` (#? none — cite the web UI finding of 2026-11-08) and `###
Changed` for the bump; `addon/CHANGELOG.md`; release 0.5.6 is the owner's
commit.

## §4 Guards, commits

Commits, each green under `make ci`: (1) this brief; (2) bump + adapter;
(3) fixtures + e2e; (4) docs + changelog. Guards in the background with one
summary log: `make ci`, `make race`, `make cover-check`, `make secrets`.
Skip `make fuzz-smoke` (decoder untouched) and the image build (CI) — say
so. Local work only; the owner's session pushes, opens the pull request,
folds in the release and tags.

## §5 Report (hand back, fixed shape)

Per section the commits; guards with HEAD and exit; differences from this
brief; decisions; left out; questions; `by_design` candidates. Under 200
words.
