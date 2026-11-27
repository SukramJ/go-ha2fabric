# go-ha2fabric #73 — persist the served keys? Plan for the owner's decision

Read-only, at `origin/main` = `04c205a` (go-ha2fabric). go-fabric and
go-hamapping lines are from the module cache at the versions `go.mod:10`
and `go.mod:8` pin (go-fabric **v0.27.0**, go-hamapping **v0.9.0**); nothing
newer was read. Paths without a repo prefix are go-ha2fabric's.

## 1. What happens today (measured)

- **Who assigns the number.** go-fabric, per stable key, in its own store:
  `endpoint.Store.UpsertEndpointAssigning` allocates, the record stays
  until removed (go-fabric `endpoint/identity.go:98-115`). `Remove` is the
  close and keeps the record, `Forget` is the erase (go-fabric
  `node/reserve.go:17-30`). The bridge only hands in the key.
- **The key** is `<device UID>/<entity registry entry id>`
  (`internal/plan/plan.go:318-336`; go-hamapping `plan.go:291-301`; the
  e2e test spells one as `"ha:dev-shelly-1/" + regID`,
  `internal/bridge/bridge_f2_e2e_test.go:23`). It is not the registry id
  alone and cannot be turned back into one without assuming its format.
- **Removed vs forgotten.** `plan.Diff` forgets a dropped key only when
  the `forget` map holds it, else removes it (`internal/plan/diff.go:66-76`,
  `:88-99`). The map is filled in exactly one place: the
  `entity_registry_updated` event with `action: remove`
  (`internal/bridge/bridge.go:400-411`, `noteRemoved` `:421-440`), in
  memory only (`:228-237`). A removal the daemon got no event for is
  therefore a plain `Remove`, and the record keeps the number.
- **After a restart** the bridge has no groups, so the first sync is adds
  only (`bridge.go:664-688`: `served` is built from `b.groups`); a key that
  is not planned again is simply not added, its record stays.
- **Docs** state this: `docs/operations.md:571-579` (decision 45),
  `:314` (`bridge.forget`).
- **What ha2fabric persists itself:** nothing about endpoints. Its own
  files are `selection.json` (atomic: temp file, fsync, rename, plus a
  `.bak` copy — `internal/selection/selection.go:348-390`) and
  `nodes.json`; the SQLite file is go-fabric's, opened by path
  (`internal/config/node.go:71`).
- **Architecture rule:** "A sync reads everything … so a reconnect needs
  no path of its own. An incremental mirror of the registries was rejected
  on 2026-11-05" (`CLAUDE.md:68-70`).

Consequence of the status quo, both directions:

- An entity deleted while unobserved and **re-created** comes back under
  its old number. Home Assistant itself gives it its old registry id
  (cited at `bridge.go:426-428`), so controllers see the same device again.
- One deleted while unobserved and **never re-created** leaves one endpoint
  record in the store for good. Numbers are not reused either way
  ("A removal never rewinds it", go-fabric `endpoint/identity.go:97`).

## 2. Options

### A. Do nothing, close #73

- Behaviour: as section 1; already documented.
- Write cost: none. Upgrade path: none. Files/tests: none.

### B. Diff the registries in memory at the sync (no persistence)

The sync already holds the previous snapshot (`b.last`, set at
`bridge.go:804`) and reads a new one (`:611-616`). An entry id present in
the last snapshot's registry and absent from the new one is a removal,
whether or not an event said so: compute its key from the last snapshot
(`plan.RemovedKey`, `plan.go:369-394`) and put it into the `forget` map
before `plan.Diff`. `Diff` already drops a forget whose key is planned and
already forgets a key that was never served (`diff.go:72-75`, `:135-139`).

- Behaviour change: a deletion during a **disconnect** is forgotten, like
  one seen as an event. A deletion while the **daemon is stopped** is still
  only removed (no last snapshot). The docs sentence shrinks to the
  stopped case.
- Write cost: none.
- Upgrade path: none; no stored state.
- Guard needed: no absence-derived forget in a sync whose snapshot has
  `Skipped.Entities` (`internal/ha/snapshot.go:36-39`) — an undecodable
  entry is not a deleted one. Which id `Skipped.Entities` carries (entity
  id or registry id) is **unverified**.
- Files: `internal/plan` (a pure `RemovedSince(last, snap)`),
  `internal/bridge/bridge.go` (`syncConnected`, a few lines),
  `docs/operations.md:575-579`, `changelog.md`.
- Tests: `internal/plan` table test; one e2e beside
  `TestRemovedEntityIsForgotten` (`bridge_f2_e2e_test.go:17`): disconnect,
  delete, reconnect, re-create, fresh number; one with a skipped entry.
- Note: it stays within "a sync reads everything" — it is a comparison of
  two full reads, not a mirror. It would make the event path
  (`noteRemoved`, `removedAt`, `keepStaleForgets`, `bridge.go:421-458`)
  redundant in principle; removing that is a separate decision, not part
  of this option.

### C. Persist the keys in a file of ha2fabric's own (the issue's proposal)

A `served.json` beside the store (`served-<key>.json` per registry node,
as `nodes.go:87-90` names the others): `{version, keys: {<key>: {registry
id, entity id}}}`, loaded before the first sync; every key whose registry
id is absent from the first snapshot is seeded into the `forget` map.
Together with B's comparison this covers stopped and disconnected.

- Behaviour change: a deletion while stopped is forgotten too.
- **The set must not be "served keys".** An entity that is unexposed (its
  key removed, record kept) and then deleted while stopped is missed by a
  served-only set; the event path covers that case today
  (`RemovedKey` works for any registry entry, `diff.go:74-75`). The exact
  set is "every key added and not yet forgotten" — which is precisely
  go-fabric's endpoint records, kept a second time.
- Write cost: one atomic write (temp file, fsync, rename, as
  `selection.go:368-390`) per sync whose batch added or forgot a key; none
  for state changes or a sync without such a change. Size on the order of
  100 bytes per key (estimate, not measured).
- Upgrade/failure path: no file at the first start after the upgrade →
  empty set → that first sync behaves as today, the file is written after
  it; deletions that happened before the upgrade stay unforgotten. A
  damaged file is moved aside and read as empty, as `selection.go:165`
  does, never a start failure. A file that names keys the store does not
  have (store restored from an older backup) is harmless: a `Forget` of a
  key nothing is stored for is no error (go-fabric `node/reserve.go:418`).
  Erase must empty the file (`cmd/ha2fabric/nodes.go:304`), Delete must
  remove it (`internal/nodes/nodes.go:508-510`).
- Files: a new small package or `internal/bridge` port for the set,
  `internal/config` (path), `cmd/ha2fabric/main.go`, `cmd/ha2fabric/nodes.go`,
  `internal/nodes/nodes.go`, `internal/arch`, `docs/operations.md` (the
  store section `:26-40`, backup advice, `:575-579`), `changelog.md`.
- Tests: the set's own (load, damaged, atomic), e2e restart-delete-recreate,
  unexposed-then-deleted, erase and delete file handling
  (`internal/nodes/nodes_test.go:278`), restore-mismatch.

### D. Let go-fabric list its stored keys (no second copy)

go-fabric already holds the exact set (`ListEndpoints`,
`endpoint/identity.go:104-106`), but only uses it internally
(`node/reserve.go:369`, `:389`); no exported `Aggregator`/`Partition`/`Node`
method enumerates stored keys at v0.27.0 (grep of the exported methods in
`node/*.go`; later versions **unverified**).

- Behaviour: as C, with zero extra writes and nothing that can drift from
  the store or from a backup.
- Open: the host must decide "this stored key's entity is gone" from the
  key alone. That means computing the key of every registry entry in the
  snapshot and forgetting stored keys outside that set — which also
  forgets the old key of an entity moved to another device (the device UID
  is part of the key) and needs a rule for part keys (`<key>#n`,
  `bridge.go:852-853`) and for entities without a registry entry
  (`plan.go:322`). Each is a new decision.
- Cost: a facade request to go-fabric, a release there, then a wave here.

## 3. Risk common to B, C, D: inferring a deletion from absence

Today a `Forget` needs positive evidence, Home Assistant's remove event.
B–D add inference from "not in this read". A `Forget` cannot be undone:
controllers get a new device and lose what they attached to the old one.
Absence has other causes than a deletion:

- an entry that did not decode (guardable, see B);
- the daemon pointed at another Home Assistant, or Home Assistant restored
  from an older backup: every stored key is absent. Today that removes the
  endpoints and keeps the numbers, so going back restores them; with C or
  D everything is forgotten at the first sync;
- whether Home Assistant's `config/entity_registry/list` can answer with
  an incomplete list early in its start-up is **unverified** (home-assistant
  core at the pinned tag was not read for this).

The error of the status quo runs the harmless way (a number kept too
long); the error of B–D runs the destructive way.

## 4. Recommendation

**A — close #73 as "by design", keep the docs sentence.**

1. The gap costs nothing a user notices: a re-created entity is, for Home
   Assistant itself, the same entity (same registry id), and it reappears
   as the same device. The other case is one orphan row per entity.
2. Every fix trades that for a possible wrong `Forget`, which is
   irreversible on the controllers (section 3).
3. C, as the issue words it, is not exact (served vs reserved keys); the
   exact version duplicates go-fabric's records in a second file with its
   own backup, erase and delete paths — the kind of mirrored state
   `CLAUDE.md:68-70` rejected.

If the owner wants the two cases to behave alike after all:

- **B first** — no stored state, a few lines, covers the disconnect, stays
  inside "a sync reads everything". Before it is built, the start-up
  question in section 3 needs a read of home-assistant core at the pin.
- **D rather than C** for the stopped daemon — the set belongs to the
  store that already has it; as a question to go-fabric, not a workaround.

## 5. Not read

- home-assistant core (registry list at start-up, lifetime of
  `deleted_entities`).
- go-fabric after v0.27.0; go-hamapping's `Device.UID`.
- `cmd/ha2fabric/nodes.go` erase beyond its signature line (`:304`);
  `internal/nodes/nodes.go` beyond `:87-90`, `:505-510`.
- `notes/review/2026-11-23/decisions.md:104` (decision 45) was read as a
  grep line only.
