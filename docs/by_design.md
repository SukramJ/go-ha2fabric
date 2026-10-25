# By design

Where go-ha2fabric deliberately behaves differently from
[home-assistant-matter-hub](https://github.com/RiDDiX/home-assistant-matter-hub),
the helper reference for how a Home Assistant bridge behaves. Citations are
`file:line` at its commit `664575a`, paths under `packages/` (and
`packages/backend/src/` for `services/` and `matter/`). matter-hub is never a
reference for Matter behaviour; that is go-fabric's.

## Exposure model

matter-hub filters with matchers: sixteen types (pattern, regex, domain,
platform, entity and device labels and their regexes, area, entity
category, device and product name, manufacturer, device class;
`common/src/home-assistant-filter.ts:1-19`), an include and an exclude list
and an `includeMode` of `any` or `all` (`:26-33`); an empty include list
matches everything (`services/bridges/bridge-registry.ts:1158-1169`), the
exclude list always combines with `any` (`:1170-1174`). go-ha2fabric
starts from Home Assistant's own Assist exposure and adds include and
exclude by entity id, glob and label: Assist ∪ include − exclude, with
labels in both lists (`internal/expose`). The installation's existing
"Expose" setting is the default answer, and the few lists cover what it
does not; matcher combinations stay out until an owner asks for one.

## Unresolved label names

matter-hub resolves a label value by name, case-insensitively, else
slugifies it (NFD, diacritics stripped, lower case, other runs to `_`;
`services/bridges/matcher/matches-entity-filter.ts:255-271`) and compares
the slug with the label ids. go-ha2fabric uses an unresolved value as a
label id exactly as given (`expose.Labels.Resolve`, owner decision D1 of
2026-10-25) and warns: a slug of a name is Home Assistant's id generation
(`helpers/label_registry.py:130-132` at 2026.10.3) only for a label created
under that name, so guessing it can match a different label.

## Label refresh

matter-hub polls the registries every 60 s by default
(`backend/src/commands/start/start-options-builder.ts:84-89`,
`services/home-assistant/home-assistant-registry.ts:100-131`); a label
rename that keeps the `label_id` does not enter its fingerprint (`:222-236`).
go-ha2fabric subscribes to `label_registry_updated` beside the entity and
device registry events (`helpers/label_registry.py:23-26`, in the
subscribe allow list `auth/permissions/events.py:46`) and re-plans after a
one-second debounce, renames included.

## Hidden entities

matter-hub excludes a hidden entity unless the `includeHiddenEntities`
feature flag is set (`services/bridges/bridge-registry.ts:1008-1012`).
go-ha2fabric never serves a hidden or disabled entity; not even a web UI
override does (`expose.Rule.Decide`). Hidden is the user's "not in the
UI", and a flag that undoes it is a second switch for the same choice.

## Unknown state

matter-hub keeps an entity reachable while its state is `unknown`
(`matter/behaviors/basic-information-server.ts:93-95`, reachable unless
`unavailable`), though its entity behaviour treats `unknown` as no data
(`matter/behaviors/home-assistant-entity-behavior.ts:44-49`). go-ha2fabric
makes `unknown` unreachable like `unavailable` (`internal/adapt` `State`,
`homeassistant/const.py:303,313`; H2's decision): a controller shows a
device that has no value as not responding rather than with a stale one.

## Removal grace

matter-hub keeps an endpoint 300 s after its entity leaves the registry
and until a newer registry snapshot (`ENDPOINT_REMOVAL_GRACE_MS`,
`services/bridges/bridge-endpoint-manager.ts:55-59`, `:968-999`), because
deleting erases the persisted number and controllers lose groups; an
entity its filter drops is closed at once and keeps its number for a later
re-inclusion (`:949-965`, issues #468, #404). go-ha2fabric removes the
endpoint on the next sync, and go-fabric numbers a device added again anew
(H3 test `TestWebUI`). Whether a grace or a number reservation is wanted is
an open question of brief H3.

## Naming

matter-hub names a bridged device by a custom name, then the registry
name (behind `preferEntityRegistryName`), then `friendly_name`, then the
entity id, each cut to 32 bytes (`matter/behaviors/basic-information-server.ts:41-48`).
go-ha2fabric takes the registry name, the integration's name for an
entity with `has_entity_name`, else `friendly_name` (`internal/adapt`
`entityName`), the web UI's rename over all of them, and go-hamapping
composes it with the device name (`device.ComposeNodeLabel`).

## Composition

matter-hub composes devices: user-defined composed entities
(`services/storage/entity-mapping-storage.ts:227-229`) and, behind
`autoComposedDevices`, battery, humidity, pressure, power and energy pulled
onto a device's main entity (`services/bridges/bridge-registry.ts:1032-1105`).
go-ha2fabric composes nothing: go-hamapping's rows decide, one endpoint per
mapped entity, with riders (a battery) on the device's primary endpoint.
The table is the one place a device shape is decided.

## Where the selection lives

matter-hub stores a bridge's filter with the bridge in its own storage and
re-mounts the endpoints on a change (`services/bridges/bridge-service.ts:318-330`,
`services/bridges/bridge.ts:399-403`). go-ha2fabric keeps its rule in the
add-on options and only the web UI's per-entity overrides in a file it
owns, `/data/selection.json` (go-fabric concept v9 decision 0.4), written
atomically beside the node's store, so the add-on's cold backup holds both
and the options stay the single place for the rule.
