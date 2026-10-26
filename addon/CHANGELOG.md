# Changelog

Changes to the **go-ha2fabric** Home Assistant add-on. The add-on version
tracks the project release; the project's
[changelog.md](https://github.com/SukramJ/go-ha2fabric/blob/main/changelog.md)
has the details.

## 0.2.3

- An entity deleted from Home Assistant gets a fresh endpoint number when
  it is created again; a disabled or hidden one keeps its number.
- A label name with a comma is one label in the web UI.

## 0.2.2

- The web UI shows the configuration's warnings: a label of
  `include_labels`/`exclude_labels` that names no Home Assistant label, and
  an overrides file that was moved aside at start.
- The labels filter offers single labels and matches an entity that
  carries the chosen one.
- A name typed in the web UI is the device's name in the controller as
  typed, without the device name in front.
- An entity turned off and on again keeps its endpoint number.

## 0.2.1

- The add-on has its icon and logo in the store.
- A device whose integration registered a numeric identifier no longer
  stops the sync (`bridge.sync_failed`, "cannot unmarshal number").

## 0.2.0

- Labels: `include_labels` and `exclude_labels` serve or hide the entities
  that carry a Home Assistant label (by name or id); a device's label counts
  for its entities.
- The web UI is the family's interface: pairing page with the window,
  fabrics, endpoints, diagnostics, and an entity selection that turns single
  entities on or off and renames them (`/data/selection.json`).
- A renamed entity id keeps its endpoint. This upgrade renumbers every
  endpoint once: controllers re-learn the bridged devices.

## 0.1.0

The first release: your exposed Home Assistant entities as one bridge for
Matter controllers, with a web UI showing the pairing code and every
entity's verdict.
