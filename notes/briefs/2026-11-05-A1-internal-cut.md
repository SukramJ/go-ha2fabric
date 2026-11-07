# Brief A1 (go-ha2fabric) — the internal cut

Owner decisions (2026-11-05; `CLAUDE.md` "Architecture", `CONCEPT.md`
"Decided (owner, 2026-11-05)", `docs/roadmap.md` "A — Architecture"; the
brief's five questions decided 2026-11-07, last section): A1 is
the internal cut only, **no behaviour change** — no log line, no
`--dump-plan` column, no web UI item, no endpoint number moves.
`cmd/ha2fabric` (`main.go`, and `nodes.go` since U6b, #36) is the one
composition root; `internal/plan` is pure (no
node, no goroutines): assess, the effective rule with its overrides, the
diff as batch operations; `internal/bridge` is only the reconciler and
imports neither `fabricui` nor `internal/config`; `internal/exposure` is the
web UI's provider and warnings on the bridge's read model; `internal/arch`
holds the new layers in this wave. Out of scope (A2): go-hamapping's
vocabulary, structured commands, `internal/adapt`'s go-fabric imports. No
go-hamapping or go-fabric change. Rejected: an incremental registry mirror;
splitting files within one package only. One pull request on
`arch/a1-internal-cut`, squash-merged; no release (nothing user-visible).

Model: Opus implements; a question about Home Assistant or Matter is not
part of this brief and goes to the owner.

Files the implementing agent reads (HEAD `34075a6`, main after the 0.5.3
release; every `file:line` below is at that HEAD, re-verified 2026-11-07
after #34 and #36 — `internal/bridge`'s non-test code did not change since
`af376e1`): `internal/bridge/bridge.go`, `provider.go`, `node.go`,
`camera.go`, `command.go`; `internal/bridge/bridge_test.go`,
`bridge_e2e_test.go:1-120`, `bridge_h3_e2e_test.go:120-140, 360-414`,
`bridge_u6_e2e_test.go`; `cmd/ha2fabric/main.go`, `nodes.go`, `main_test.go`,
`nodes_test.go`; `internal/arch/arch_test.go`;
`internal/selection/selection.go:105-140`; `internal/expose/expose.go:30-45,
109-135`; `internal/config/config.go:74-129` (after the token fields,
`:67-72`); `internal/ha/fanout.go`; `script/coverfloor/main.go`,
`floors.txt`. Dependencies at their tags, as `go.mod:6-9` at `34075a6`
names them: go-hamapping v0.4.3 `plan.go:31-50,
97-130, 137-150, 309-367, 511-519` (no commit between v0.4.2 and v0.4.3
touches `plan.go`); go-fabric v0.18.0 `node/aggregator.go:30-32, 287, 343,
409, 443, 489`, `node/reserve.go:206-230`, `node/config.go:380-385,
598-602, 625`;
go-fabric-ui v0.6.1 `fabricui/provider.go:23-48, 100-139`,
`fabricui/options.go:80-110, 153-249`.

Facts the brief rests on (verified at HEAD):

- `sync` (`bridge.go:688-880`) does, in order: `Read` (:692), the effective
  rule and names (:699, `rule()`/`names()` :883-903), `assess` (:700),
  `candidates` (:701), warnings (:703), the keep-or-build decision by
  signature (:725-740), the removed set from `b.groups` in map order
  (:741-746), the forget rules (:747-770), the batch (:771-811), the forget
  log (:815-817), `plan.Close` on replaced and removed (:818-820), the camera
  hosts (:822), states (:829-836), endpoint numbers into the verdicts
  (:837-843), the exposure diff for `OnChange` (:715-720, :844-857), the read
  model under `b.mu` (:858-873), the log line (:874-875), `OnChange` (:876-879).
- The stable keys of a group's endpoints are known after go-hamapping's
  `Map`, before `Build`: `Plan.options` sets `device.Options.StableKey` while
  `Map` builds the endpoints (go-hamapping `plan.go:309-345`), and `Build`
  returns one `*device.Device` per `Plan.Endpoints[i]` in order (`:356-367`),
  so `g.keys[i]` pairs with `g.devices[i]` (`bridge.go:784-790,
  1016-1018`). `Build` needs no node (`device.New`, `:358`); only
  `Aggregator().Batch` does (`bridge.go:771`). A failed build leaves a
  group with neither devices nor keys (`:1003-1013`).
- `Batch.Add(ep node.Endpoint)`, `Replace(key, ep)`, `Remove(key)`,
  `Forget(key)` (`node/aggregator.go:409, 443, 489`; `node/reserve.go:218`);
  `*device.Device` is the `node.Endpoint` (`aggregator.go:30-32`).
- `fabricui.Verdict` is `= eligibility.Verdict` (`fabricui/provider.go:100`);
  `Item`, `Update`, `Warning`, `ExposureProvider`, `Options.Warnings`
  (`provider.go:23-48, 103-139`; `options.go:166-178`). fabricui itself
  imports go-fabric `device`, `eligibility`, `node`, `contract`.
- `camera.New` starts nothing (`internal/camera/camera.go:180-194`; the only
  goroutine is the relay on an offer, `:319`), so the dry host
  (`bridge/camera.go:37`) is inert. `internal/camera` is the one importer of
  go-fabric's `cluster/camera` (`camera.go:39`) and does not import
  go-hamqtt's `model` today; `bridge.ports` (`bridge/camera.go:20`) has the
  shape `func(model.Entity) any`.
- `selection.Store.Enabled()` and `Entries()` are in-memory reads under the
  store's mutex (`selection.go:122-140`); the file is touched by `Open` and
  `Set` only. The names derivation (`Entries` filtered on `Name != ""`) is
  written twice (`bridge.go:892-903`, `main.go:221-226`).
- `node.Config.Validate` requires the SPAKE2+ salt and iteration count
  (`node/config.go:598-602, 625`; the fields `:380-385`), which today only `bridge.NodeConfig`
  supplies (`node.go:25-29, 44-46`); the e2e harnesses build the daemon's
  identity through it (`bridge_e2e_test.go:76-78`; two nodes in
  `bridge_u6_e2e_test.go:36-41`), as does `TestNodeConfig`
  (`bridge_test.go:120-140`). Since U6b the daemon calls it from two
  places — the configured node (`main.go:97`) and each registry node
  (`nodes.go:163`) — both feeding `serve`'s `ncfg`, which calls `node.New`
  (`nodes.go:45-46`). The status page's `nodeConfig(cfg)` (`main.go:140-154`)
  and `fleet.entry` (`nodes.go:178-196`) are something else: a
  `*fabricui.NodeConfigView`, the displayed values without the passcode;
  they read `bridge.VendorName` and `ProductName` (`main.go:148-149`,
  `nodes.go:184-185`).
- Tests reach into bridge only through its exported API: `Options`, `New`,
  `Run`, `Synced`, `Status`, `Plan`, `Resync`, `Exposure`, `Warnings`,
  `Verdict`, `Snapshot`, `Dry`, `NodeConfig`, `WarningLabelUnresolved`,
  `WarningSelectionCorrupt` (all test files are `package bridge_test`;
  `bridge_u6_e2e_test.go` uses `New`, `Options{Rule, Selection, Debounce,
  Logger}`, `Run`, `Synced`, `Plan`, `NodeConfig` and `ha.Fanout`, nothing
  beyond the list; `cmd/ha2fabric/main_test.go` and `nodes_test.go` are
  `package main` and call `run` — `nodes_test.go` also reads the registry
  file through `nodes.Open`, `:255-257`).
- `script/coverfloor` fails only a package that has a floor and is below it
  (`main.go:64-79`); a package without a floor is printed and passes. There
  is no regeneration mode: floors are read from `floors.txt` and set by hand.
- `fabricui` is imported by `internal/bridge` (`bridge.go:38`,
  `provider.go:14`), `internal/web`, `cmd/ha2fabric`, and by the tests
  `bridge_f2_e2e_test.go` and `bridge_h3_e2e_test.go`. The guard's
  `thirdParty` list is global (`arch_test.go:86-92, 171-178`).

## §1 Where each symbol goes

| Today (`internal/bridge`) | After A1 | Why |
| --- | --- | --- |
| `Upstream` (`bridge.go:54-64`) | `ha.Source` (the seven reads, :56-62) in `internal/ha`; `bridge.Upstream` embeds `ha.Source` and adds `Connected`, `CallService` | the reads are the wire's; the reconciler adds what it alone needs |
| `Snapshot` (:394-402), `Read` (:416-443) | `ha.Snapshot`, `ha.ReadSnapshot(ctx, src Source, assist bool)`; error strings unchanged | `Read` does I/O, so it is not in `plan`; a one-shot read of the wire belongs to the wire's package, not to the reconciler, or `--dump-plan` would import the reconciler for one call |
| `Snapshot.LabelRegistry` (:406-412) | `plan.Labels(s *ha.Snapshot) expose.Labels` | keeps `internal/ha` free of `expose` |
| `Verdict` (:95-113) | `plan.Verdict`; the unexported `verdict` becomes `Full eligibility.Verdict` with `json:"-"` | `internal/exposure` reads it (`provider.go:125, 127`) |
| `planned` (:446-454) | `plan.Group` (exported fields `Key, Device, Entities, Regs, Descs, Names, Sig`) plus `func (g *Group) Describe(id string, st *ha.State, unit string) string` (the body of `descOf`, :1068-1070) | the bridge's `group` embeds `*plan.Group` |
| `assess` (:469-583) | `plan.Assess(s *ha.Snapshot, in Input) ([]Verdict, []*Group)` | pure |
| `describe` (:605-611), `mapVerdicts` (:614-627), `mapOptions` (:647-661) | `plan.Describe`, `plan.MergeVerdicts`, `plan.MapOptions` (exported: `build` and `sync` call them) | pure |
| `deviceName` (:631-636), `isCamera` (:461) | unexported in `plan` | only `Assess` uses them |
| `cameraOff` (:458) | `plan.CameraOff` (exported const) | `logCameraOff` (:587-601) stays in the bridge and compares reasons to it |
| `dryHost`, `dryPorts` (`camera.go:37-47`) | `camera.DryPorts(e model.Entity) any` in `internal/camera`, over an unexported inert host (`camera.New("", nil, camera.Options{})`); `plan.Input.Ports` carries it (§3), set by the composition root | owner, 2026-11-07, against the brief's recommendation (the import): the pure core stays off a package that imports `cluster/camera` (`internal/camera/camera.go:39`) and starts a relay goroutine (`:319`). go-hamapping still decides a camera row on the ports' interfaces (`plan.go:120-128`), so the stand-in stays a `*camera.Host`, not a second implementation of the port set. Cost on record: `internal/camera` imports go-hamqtt's `model` for the one signature — the shape `bridge.ports` (`camera.go:20`) already has, so A2 changes both together |
| `Dry` (:665-672), `candidates` (:677-684) | `plan.Dry(s, in) []Verdict`, `plan.Candidates(s, in) map[string]*Verdict` | pure |
| `removedKey` (:333-355) | `plan.RemovedKey(s, entityID) device.SourceKey` | pure; `noteRemoved` (:321-328) stays |
| `rule()`, `names()` (:883-903) and `main.go:219-226` | `plan.Effective(rule expose.Rule, o Overrides, cameras bool) Input`; `selection.Store` gains `Names() map[string]string` | one place (§3) |
| `warn` (:919-946) | the message (`:926`) becomes `plan.Warnings(in Input, labels expose.Labels) []string`; `since`/`warned` tracking and the log line stay in the bridge; the `fabricui.Warning` construction leaves | the finding is pure, its age is reconciler state, its UI code is the UI's |
| `Warnings(ctx) []fabricui.Warning` (:952-966), `WarningLabelUnresolved`, `WarningSelectionCorrupt` (:907-912) | `exposure.(*Provider).Warnings`, `exposure.WarningLabelUnresolved`, `exposure.WarningSelectionCorrupt` | the codes are the web UI's (`:905-906`) |
| `provider.go` entirely | `internal/exposure` | decided |
| `NodeConfig`, `VendorName`, `ProductName`, `spake2Salt`, `spake2Iterations` (`node.go:18-60`) | `config.(*Config).Node(logger *slog.Logger) node.Config`, `config.VendorName`, `config.ProductName` in `internal/config`; `serve` (`nodes.go:45-46`) already takes a `node.Config` and calls `node.New`, so its two callers pass `cfg.Node(logger)` (`main.go:97`) and `c.Node(log)` (`nodes.go:163`) — one function, called from both; `nodeConfig` (`main.go:148-149`) and `fleet.entry` (`nodes.go:184-185`) read the two constants from `config` (owner, 2026-11-07; U6b re-read) | the e2e harness and `TestNodeConfig` need it; `package main` cannot be imported, and a copy of the salt and iterations in test code would drift |
| `LogFabrics`, `LogPairing` (`node.go:68-95`) | `cmd/ha2fabric/nodes.go`, unexported, beside `serve`, their one caller (`nodes.go:54-55`); no new file | start-up logs are the composition root's |
| `Status`, `Plan()`, `Resync`, `Synced`, the `ha.Handler` methods, `build`, `apply`, `stateChanged`, `ports`, `attachCameras`, `closeCameras`, the command loop | stay | the reconciler |
| `sync`'s diff (:722-811) | `plan.Diff` (§2); `sync` keeps the keep-or-build loop, applies the ops in one `Batch`, logs, closes | the rules become testable without a node |
| the exposure diff for `OnChange` (:715-720, :844-857) | stays in `sync` | bookkeeping over the bridge's previous state; moving it buys nothing |

`bridge.Options` after A1: `Input func() plan.Input` (replaces `Rule`,
`Selection`, `Camera`; called at every sync, as the rule reads the overrides
at every sync today, :75-78, :699), `Debounce`, `Logger`, `OnChange`,
`CameraUpstream`. `sync` sets `in.Ports = b.ports` on the `Input` it draws:
the live hosts (`b.hosts`) are reconciler state, so the closure `cmd` hands
out is the same for the bridge and for `exposure`, and the bridge replaces
its dry `Ports` with its own. `ports` (`camera.go:20-33`) checks
`CameraUpstream != nil` and the platform; the option's gate is `Assess`'s
alone (`bridge.go:538-539`: a camera the option leaves out is not planned,
so `Map` never asks for its ports — today's `b.opts.Camera` check in `ports`
`:21` is reached by no camera entity). `Options.Camera`'s doc (:83-86)
moves to `Input.Cameras`.

`bridge.go`'s package doc (:4-21) is rewritten for the reconciler; `arch`'s
(:4-11) for the new layers.

## §2 The diff: `plan.Diff`

The bridge keeps deciding keep-or-build by signature (:726) and building
(:731, `build` :996-1022); what it then hands the pure function is the
result of that, not the node.

```go
// Built is one device group as the reconciler serves it: the signature its
// plan was built from and its endpoints' stable keys, in go-hamapping's
// endpoint order, each with the endpoint the batch adds or replaces.
type Built[E any] struct {
    Key, Sig  string
    Keys      []device.SourceKey
    Endpoints []E // len(Endpoints) == len(Keys); both empty after a failed build
}

type OpKind uint8 // Remove, Forget, Replace, Add

type Op[E any] struct {
    Kind     OpKind
    Key      device.SourceKey
    Endpoint E      // Replace, Add
    Entity   string // Forget: the entity id, for the log
}

type Changes[E any] struct {
    Ops                      []Op[E]
    Added, Replaced, Removed []string          // group keys, in Ops order
    Forgotten                map[string]string // key.String() → entity id
}

// Diff is the batch that brings old to next. next is in the order Assess
// returns the groups; a kept group is old's own entry. forget is the stable
// key of every entity removed from the registry since the last sync, by
// entity id.
func Diff[E any](old map[string]*Built[E], next []*Built[E], forget map[string]device.SourceKey) Changes[E]
```

`E` is a type parameter because the diff's contract is about keys and
order and nothing else; the bridge instantiates it with `*device.Device`,
the tests with `string`. The rules, each as `sync` holds them today:

1. A group in `next` whose `Sig` equals `old[key].Sig` is kept: no ops
   (:726-730). The bridge passes the old entry itself as the `next` entry.
2. `Removed`: every `old` key not in `next` (:741-746), in **sorted key
   order** (today map order, so no test can depend on it). Each of its keys
   is dropped (rule 6).
3. `Replaced`: every `next` group with an `old` entry and a different `Sig`,
   in `next`'s order (as `Assess` returns the groups: first-seen in sorted
   entity id order, :556-560, :725-740). Per group: for each new key in
   endpoint order, `Replace` if the old group had the key, else `Add`
   (:784-791); then each old key not in the new group is dropped (:792-796).
4. `Forgotten`: every `forget` entry whose key is **not** live, live being
   every key of every `next` group, kept ones included (:750-761). An entry
   whose key is live is dropped silently — re-added within the debounce, it
   keeps its endpoint (:747-749, F2 report Q1).
5. After the removed and replaced sections, a `Forget` for every forgotten
   key not yet forgotten by rule 6 — a removed entity the bridge did not
   serve frees its stored number (:798-804) — in sorted key order (today map
   order).
6. Dropping a key is `Forget` when it is in `Forgotten`, else `Remove`
   (:764-770).
7. `Added`: every `next` group without an `old` entry, in `next`'s order,
   each endpoint an `Add` (:805-809), last.
8. The ops' section order is removed, replaced, trailing forgets, added
   (:773-809); it is preserved exactly.

Not in `Diff`, stays in `sync`: `b.forget` is reset before the batch and
whatever its outcome (:762); `errors.Join` of every op's error, logged as
`bridge.batch` (:810-814); the `bridge.forget` lines (:815-817) from
`Forgotten` in sorted key order; `plan.Close` on the old plans of `Replaced`
and `Removed` (:818-820); `b.groups = next`; the log counts
(`added`/`replaced`/`removed`, :874-875) and `changed` for `OnChange` (:876)
from the three slices.

## §3 The effective rule and the read model

**`plan.Input`** is the value every assessment takes besides the snapshot:

```go
type Input struct {
    Rule    expose.Rule       // the configured rule, Overrides from the web UI
    Names   map[string]string // the web UI's name overrides
    Cameras bool              // the camera option; off, Assess gives a camera CameraOff
    // Ports is go-hamapping's Options.Ports (plan.go:120-128 at v0.4.3): a
    // camera entity's host object, asked once per entity during Map. The
    // composition root sets camera.DryPorts for --dump-plan and the web
    // UI's previews; the bridge sets its live hosts at every sync (§1).
    // MapOptions hands it through unchanged.
    Ports   func(model.Entity) any
}

// Overrides are the web UI's overrides as the rule reads them;
// *selection.Store implements it. nil is none.
type Overrides interface {
    Enabled() map[string]bool
    Names() map[string]string
}

func Effective(rule expose.Rule, o Overrides, cameras bool, ports func(model.Entity) any) Input
func (in Input) With(enabled map[string]bool, names map[string]string) Input // Preview's proposed (provider.go:196-213)
func (in Input) Without(keys ...string) Input                                 // Apply's base (provider.go:255-259)
```

`With` and `Without` clone before they write, as `proposed` does
(`:198-204`). `selection.Store.Names()` is the derivation of
`bridge.go:896-901`; `plan` takes the interface, not the store, so the pure
core depends on two maps and not on a file. `cmd/ha2fabric` composes once
per node: `serve` (`nodes.go:45`) takes `in func() plan.Input` in place of
`rule, sel, cam` and hands the same closure to the node's bridge and to its
`exposure.New`; `run` builds the configured node's — `func() plan.Input {
return plan.Effective(cfg.Exposure(), sel, cfg.Camera, camera.DryPorts) }`
(`main.go:97`) — and `fleet.start` a registry node's from `expose.Rule{}`,
the node's own selection and `c.Camera` (`nodes.go:159-163`); `--dump-plan`
calls the configured node's directly (`main.go:219-226` disappear). `plan.MapOptions(regs, names, in.Ports)` is today's
`mapOptions` (`bridge.go:647-649`) with the field in place of the argument
`dryPorts(cameras)` (`:667`) or `b.ports`.
Tests pass a literal `nil` as `Overrides` when there is no store (an
untyped nil; a nil `*selection.Store` through the interface would not be
nil — `Effective` is documented so).

**The bridge's read model** — what the last sync left for readers, under
`b.mu`, returned by value:

```go
// Warning is a configuration finding of the last sync and when an unbroken
// run of syncs first saw it (bridge.go:914-918).
type Warning struct { Message string; Since time.Time }

type View struct {
    Snapshot *ha.Snapshot            // nil before the first sync
    Verdicts []plan.Verdict          // Plan()'s content
    All      map[string]*plan.Verdict // Candidates of that sync
    Warnings []Warning
}

func (b *Bridge) View() View
```

`Plan()` and `Status()` stay (tests, health). `b.warnings
[]fabricui.Warning` (:186) becomes `[]Warning`; `Status.Warnings []string`
(:127) stays as today's messages.

**`internal/exposure`**:

```go
type Model interface { View() bridge.View }

type Options struct {
    Input     func() plan.Input // the same closure the bridge gets
    Selection *selection.Store  // Apply writes it; nil refuses Apply (provider.go:248-250)
    Logger    *slog.Logger
}

func New(m Model, opts Options) *Provider
// *Provider implements fabricui.ExposureProvider (Schema, List, Preview, Apply)
func (p *Provider) Warnings(ctx context.Context) []fabricui.Warning
```

`view()` (`provider.go:93-111`) reads `m.View()` and `opts.Input().Names`;
`proposed` and `Apply` take `opts.Input()` through `With`/`Without`. The
current, not the sync's, input is read on purpose: that is what `:109`,
`:197` and `:255` do today. `Warnings` maps `View.Warnings` to
`fabricui.Warning{Code: WarningLabelUnresolved, …}` and appends the
`selection_corrupt` one from `opts.Selection.Recovered()` (`:956-964`).
`served` (`nodes.go:30-38`) gains the node's `prov`; `cmd` wires the
configured node's `fabricui.Options{Exposure: def.prov, Warnings:
def.prov.Warnings, Reassemble: def.br.Resync, …}` (`main.go:108-112`) and a
registry node's `NodeEntry.Exposure = m.s.prov` (`nodes.go:189-194`;
`fabricui.NodeEntry` has no `Warnings`, `options.go:80-110` at v0.6.1, so a
registry node's findings reach the operator through the bridge's log line
only, as today). Nothing else in `exposure`
changes: the field constants, the schema, `item`, `check`, the error texts.

A `Model` interface over one method is kept so `exposure` has a direct test
with a fake view; the struct type stays the bridge's because the `Since`
times are reconciler state. The alternative — a snapshot-only read model
with `exposure` recomputing `Candidates` per request — was not taken: it
moves work from one sync to every page load, which is a behaviour change.

## §4 Imports and the layers

`internal/plan` imports `internal/ha` (the snapshot's types), `internal/adapt`
(`Entity`, `Device`, `OwnDevice`), `internal/expose` (`Rule`, `Labels`,
`Decide`, `Unresolved`), go-hamapping
(`Map`, `Options`, `Verdict`), go-fabric `device` (`SourceKey`,
`StringKey`) and `eligibility` (`Verdict`, `StateUnmappable`), go-hamqtt
`model`. Not `internal/selection` (an interface instead, §3), not `internal/camera`
(`Input.Ports` instead, §3; owner 2026-11-07), not `internal/config`, not
`fabricui`, not go-fabric `node`. Pure means: every
function is a function of its arguments, nothing is started, nothing is
stored — go-hamapping's `Map` qualifies (`plan.go:137-…`, no goroutine;
`Build` is never called in `plan`).

`layers` (`arch_test.go:59-74`; `internal/nodes: {}` and `internal/expose`
in `cmd`'s entry are U6b's) after A1:

```go
"internal/config":    {"internal/expose", "internal/version"},
"internal/plan":      {"internal/ha", "internal/adapt", "internal/expose"},
"internal/bridge":    {"internal/ha", "internal/adapt", "internal/camera", "internal/plan"},
"internal/exposure":  {"internal/bridge", "internal/plan", "internal/ha", "internal/expose", "internal/selection"},
"cmd/ha2fabric":      {"internal/bridge", "internal/camera", "internal/config", "internal/expose", "internal/exposure", "internal/ha", "internal/nodes", "internal/plan", "internal/selection", "internal/version", "internal/web"},
```

`testLayers`: `internal/plan: {hatest, internal/camera}` (the camera-on
case passes `camera.DryPorts`, §6); `internal/bridge: {hatest,
internal/config, internal/exposure}` (the e2e tests compose the stack as
`cmd` does, §6); `internal/exposure: {hatest}` if its direct test loads the
fixtures. Every other entry unchanged; `internal/camera`'s (`{internal/ha}`)
in particular, because go-hamqtt's `model` is in `thirdParty`
(`arch_test.go:86-92`), not a layer; `internal/nodes: {}` and `internal/web:
{}` too — `internal/web`'s go-fabric `logring` is a facade entry (`:31-38`),
not a layer. `internal/bridge` loses `expose`,
`config`, `selection`, `version`: the guard fails if a leftover import
remains, which is the point.

## §5 The guard

`thirdParty` stays the allow-list of modules. Add a per-package restriction
beside it:

```go
// restricted are third-party imports only some packages may make:
// go-fabric-ui is the web UI's, behind internal/exposure and internal/web,
// wired by the composition root; a package's tests may compose the stack.
var restricted = map[string][]string{
    "github.com/SukramJ/go-fabric-ui/fabricui": {"internal/web", "internal/exposure", "cmd/ha2fabric"},
}
```

`allowedThirdParty(pkg, p string, isTest bool) bool`: in the list, and
either unrestricted, or `pkg` is named, or `isTest`. `TestImports`
(`:157-160`) passes `pkg` and `isTest`. Extract the layer check of
`:140-156` into `layerAllows(pkg, dep string, isTest bool) bool` so it is
testable. `TestGuardRefuses` adds, refused: `fabricui` in `internal/bridge`,
`internal/plan`, `internal/adapt` (non-test); layer: `internal/bridge` →
`internal/config`, `internal/selection`, `internal/exposure` (non-test);
`internal/plan` → `internal/bridge`, `internal/selection`, `internal/config`,
`internal/camera` (non-test); `internal/exposure` → `internal/config`; `internal/config` →
`internal/bridge`. Allowed: `fabricui` in `internal/web`,
`internal/exposure`, `cmd/ha2fabric`, and in `internal/bridge` tests;
`internal/bridge` test → `internal/exposure`; `internal/plan` test →
`internal/camera`; `internal/plan` → go-fabric `device`, `eligibility`
(facade, unchanged). The pure core's "no node" is
made a guard too: `var nodeFree = map[string]bool{"internal/plan": true}`
and `facadeImport` refuses `node` for a package in it (the same form as
`cameraOnly`/`cameraPkg`, `:52-55, 99-101`); `TestGuardRefuses` adds
`facadeImport("internal/plan", "node", false)` refused and
`facadeImport("internal/plan", "device", false)` allowed.

## §6 Tests, floors, behaviour preservation

- **Move, assertions unchanged.** `bridge_test.go:23-45` (`snapshot`,
  `byID`), `TestDryPlan` (`:47-…`), the no-state case (`:101`), the camera
  cases (`:111-115`) → `internal/plan/plan_test.go` (`package plan_test`,
  `hatest.Fixture` as today): `bridge.Dry(s, rule, nil, false)` becomes
  `plan.Dry(s, plan.Input{Rule: rule})`; the camera-on case (`:115`)
  `plan.Dry(s, plan.Input{Rule: rule, Cameras: true, Ports: camera.DryPorts})`. `TestNodeConfig` (`:120-140`) →
  `internal/config/config_test.go` (`package config`), `c.Node(nil)`.
- **Stay end to end, in `internal/bridge`'s tests, assertions unchanged:**
  `bridge_e2e_test.go`, `_f2_`, `_h3_`, `_r5_`, `_r6_`, `_u6_` (#36: two
  nodes on one `ha.Fanout`), `TestResync`, `TestStatus`. Wiring edits only: `Options{Rule: rule}` → `Input: func()
  plan.Input { return plan.Input{Rule: rule, Cameras: …} }`; where a test
  sets `opts.Selection` (`_h3_`; `_u6_`'s second node, which reads no
  provider, needs only the `Input` closure), it builds `plan.Effective(rule, sel,
  cameras)` and an `exposure.New(e.br, exposure.Options{Input, Selection:
  sel, Logger})`; `e.br.Exposure()` → that provider; `e.br.Warnings` →
  `prov.Warnings`; `bridge.Warning*` → `exposure.Warning*`;
  `bridge.NodeConfig(&config.Config{…}, l)` → `(&config.Config{…}).Node(l)`.
  The one new assertion allowed: none. If an e2e test needs a changed
  assertion, the cut changed behaviour — stop and report.
- **New direct tests.** `internal/plan/diff_test.go`, a table over
  `Diff[string]`: added; kept; replaced with a shared, a new and a vanished
  key (`Replace`, `Add`, `Remove` in that order); removed group; a forget
  whose key a removed group holds (`Forget`, not `Remove`, and not twice);
  a forget of a key never served (trailing `Forget`); a forget of a live key
  (nothing, `Forgotten` empty); a failed build replacing a served group
  (old keys dropped, nothing added); two removed groups (sorted order);
  the section order across all four kinds in one call. `plan_test.go`
  adds: `Effective` with `nil` and with a store holding an enabled override
  and a name, and `Ports` handed through to `MapOptions`; `With`/`Without` do not alias their receiver's maps;
  `Warnings` message text equals `bridge.go:926`'s; `RemovedKey` with and
  without a device, and for an entity without a registry entry.
  `internal/exposure/exposure_test.go`: a fake `Model` whose `View` is the
  fixtures through `plan.Dry`/`plan.Candidates`; `List` yields one item per
  verdict with `Fields[fieldVerdict]` set; `Preview` on an unknown key is
  `fabricui.ErrUnknownItem`; `Apply` without a store is refused; `Warnings`
  carries the code and `Since`. The e2e tests in `internal/bridge` remain
  the behaviour proof for Preview and Apply through the handler.
- **Behaviour preservation** is shown by the e2e tests passing with their
  assertions untouched (`git diff --stat` of `*_e2e_test.go` shows wiring
  lines only; name them in the report), by `TestDumpPlan`
  (`cmd/ha2fabric/main_test.go:36-58`) unchanged, and by the same log
  event names (`grep -rho 'bridge\.[a-z_]*"\|ha2fabric\.[a-z_]*"'` before and
  after: the same set).
- **Floors** (`script/coverfloor/floors.txt`): after `make cover-check` at
  the last code commit, add `internal/plan` and `internal/exposure` with the
  measured percentage rounded **down** to the integer (the tool's line
  `%-28s %6.1f%%`); this initial-value rule is this brief's decision, the
  file's header only says "raise with the change that raises". Keep
  `internal/bridge 82` and every other floor as they are; if the bridge
  measures below 82 after the pure code left it, lower it to the measured
  integer with the reason in the commit message (the lost statements moved
  to `plan`, where they are covered directly; owner, 2026-11-07), and
  report the measured value and the lines that lost cover.

## §7 Commits, each building and passing `make ci`

1. `docs(brief): A1 internal cut` — this file under `notes/briefs/`.
2. `refactor(ha): Snapshot, Source, ReadSnapshot` — from `bridge.go:54-64,
   394-443`; `bridge` and `dumpPlan` call them; `LabelRegistry` stays as an
   unexported bridge function until 3; `snapshot(t)` in `bridge_test.go`
   builds `ha.Snapshot`.
3. `refactor(plan): the pure core` — `internal/plan` with everything in §1
   except `Diff`; `selection.Store.Names`; `bridge.Options.Input`;
   `provider.go` (still in `bridge`) on `plan` and `Input`; `camera.DryPorts`
   exported, `bridge/camera.go:37-47` deleted; `dumpPlan` on
   `plan.Effective`/`plan.Dry` with `camera.DryPorts`; `arch` layer for
   `plan`; `TestDryPlan` and
   friends move; `plan_test.go` direct tests.
4. `refactor(plan): Diff as batch operations` — §2 with `diff_test.go`;
   `sync` applies `Changes`.
5. `refactor(exposure): the web UI's provider` — §3's `View`, `Warning`,
   `internal/exposure`; `cmd` wires; e2e wiring edits; `arch`: `exposure`
   layer, `restricted`, `layerAllows`, the `TestGuardRefuses` cases of §5;
   `exposure_test.go`.
6. `refactor(cmd): the composition root` — `config.(*Config).Node`,
   `config.VendorName`/`ProductName`; `serve`'s callers pass `cfg.Node(logger)`
   (`main.go:97`) and `c.Node(log)` (`nodes.go:163`); `LogFabrics`/`LogPairing`
   unexported in `cmd/ha2fabric/nodes.go`; `bridge/node.go` deleted; `bridge`'s
   layer entry final; `TestNodeConfig` moves; package docs of `bridge` and
   `arch`.
7. `docs(A1): layout, floors, report` — `floors.txt` (§6); `CLAUDE.md`
   Layout paragraph (`:12-26`: `internal/ha` gains the snapshot,
   `internal/plan` and `internal/exposure` appear, `internal/bridge` becomes
   "the reconciler on go-fabric's node and go-hamapping's plans",
   `cmd/ha2fabric` (`main.go`, `nodes.go`) the composition root) and "Architecture" (the "Target,
   not built yet" block: A1's five bullets marked built with the wave, the
   `internal/adapt`/vocabulary bullet stays "not built yet", A2); `CONCEPT.md`
   "Decided (owner, 2026-11-05)" (first bullet: built in A1, report
   `docs/reports/A1.md`; "None of this is built yet" → the two remaining
   bullets) and "Better than before?" (the `bridge.go`/`main.go`/`node.go`
   citations become "before A1, at `34075a6`"); `docs/roadmap.md` A1 `[x]`
   with the PR number; `docs/reports/A1.md`.

No `changelog.md` entry: no flag, output, log line, endpoint or page
changes (the repo's rule gives an entry to a user-visible change; `make
version-check` does not read `[Unreleased]`). If the implementer finds a
user-visible difference, that is a brief deviation to report, not an entry
to write.

## §8 Guards

Run, in the background with a summary log, at the last code commit (6) and
again at 7 only for `make ci`: `make ci`, `make race` (the worker and the
command loop moved no goroutine, but `sync`'s read-model writes did move),
`make cover-check` (the new floors), `make docker addon-image` once (the
binary changed; the Dockerfile and add-on did not — the rule reads "touches
the binary", so it runs once). Skipped, with the reason in the report:
`make fuzz-smoke` (the one fuzz target, `internal/ha/message_test.go`,
parses messages; `ReadSnapshot` is not on its path), `make secrets` (no
credential path was touched: `internal/config`'s token fields and
`internal/ha`'s auth are not in this brief's file list), `make release` and
`make version-check` (`internal/version`, `addon/config.yaml`, `changelog.md`
unchanged). The agent does not touch `HAToken`, `SUPERVISOR_TOKEN` or the
client's auth (CLAUDE.md, first row).

## §9 Report (hand back, fixed shape)

`docs/reports/A1.md`: per section the commits; guards with HEAD per guard
and the skipped ones with the reason; differences from this brief;
decisions the agent took; left out; questions; `by_design` candidates
(expected: none — nothing here is about how a bridge behaves); and three
facts the owner reads first: the measured floors of `internal/plan`,
`internal/exposure` and `internal/bridge`; the `git diff --stat` of the e2e
tests; the before/after set of log event names. The report's decisions list
carries the owner's five of 2026-11-07 (last section) — `config.(*Config).Node`
handed to `serve` by both its callers, `Input.Ports` with
`camera.DryPorts`, `bridge.View`, the floors, the sorted order — as decided,
and names where the code landed each.

## Decided (owner, 2026-11-07)

- **Q1 — where `NodeConfig` lives:** `config.(*Config).Node(logger)` in
  `internal/config`; `cmd` composes `node.New` in `serve` (`nodes.go:46`,
  since U6b), and its two callers pass `cfg.Node(logger)` (`main.go:97`)
  and `c.Node(log)` (`nodes.go:163`) — one function, called from both; the
  status page's `nodeConfig` view (`main.go:140-154`) is not that function
  and stays in `cmd` (§1, §4, §7; `CLAUDE.md` "Architecture" says so).
- **Q2 — the dry run's camera ports:** not the brief's recommendation (the
  import of `internal/camera` into `plan`). `plan.Input.Ports
  func(model.Entity) any`, the shape of go-hamapping `Options.Ports`
  (`plan.go:120-128` at v0.4.3); the composition root sets `camera.DryPorts`
  for `--dump-plan` and the UI's previews, the bridge its live `ports`
  (§1, §3). Reason accepted: the pure core stays off a package that imports
  `cluster/camera` and starts a relay goroutine (`internal/camera/camera.go:39,
  319`). Cost on record (§1): `internal/camera` imports go-hamqtt's `model`
  for the one signature.
- **Q3 — `exposure` imports `bridge`:** `bridge.View` as written (§3).
- **Q4 — the bridge's floor:** below 82 after the move, lower it to the
  measured integer with the reason in the commit; new packages get floors
  from their measured coverage, rounded down (§6).
- **Q5 — sorted order** for `Removed` and the trailing forgets (§2 rules 2
  and 5): accepted.

Each design choice above is the best architecture I can state for A1 under
the standard, with its alternative and the reason named where there was one;
on Q2 the owner chose the alternative, and the reason is on record in §1.
None of the decided points is blocked by the code: the stable keys exist
before `Build` (facts, second bullet), so the pure diff needs no node, and
go-hamapping v0.4.3, go-fabric v0.18.0 and go-fabric-ui v0.6.1 (`go.mod:6-9`
at `34075a6`) carry everything A1 calls (verified 2026-11-07 at the tags).
