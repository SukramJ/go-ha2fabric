# Changelog

Changes to the **go-ha2fabric** Home Assistant add-on. The add-on version
tracks the project release; the project's
[changelog.md](https://github.com/SukramJ/go-ha2fabric/blob/main/changelog.md)
has the details.

## 0.4.0

- More Home Assistant entities reach your controller (go-hamapping 0.3.0):
  air quality, electrical, flow and soil sensors, buttons, event entities,
  selects, air conditioners with fan modes, robot vacuums, water valves,
  water heaters, speakers and TVs. Their controls call the matching Home
  Assistant actions.

## 0.3.7

- Library updates (go-fabric 0.15.4, go-fabric-ui 0.2.8). The Surface tab
  learns of every surface change the node makes, including reassemblies the
  host runs on its own, and shows the root endpoint.

## 0.3.6

- Library updates (go-fabric 0.15.3, go-fabric-ui 0.2.7). The node's surface
  report has its own page (Surface tab): effective surface per endpoint,
  decisions, withheld elements and announced changes.

## 0.3.5

- Library updates (go-fabric 0.15.2, go-fabric-ui 0.2.6). The status page shows
  the node's surface policies: specification level, controller profiles and
  enabled extensions.

## 0.3.4

- The log shows at start how many controllers are paired
  (`ha2fabric.fabrics count=…`). A start with `count=0` and an open
  commissioning window means no controller has paired or the store was
  lost; restoring the add-on from a backup taken while paired brings the
  pairing back.

## 0.3.3

- Library updates (go-fabric 0.15.1, go-fabric-ui 0.2.5). The selection
  page hides entities that cannot be mapped to an endpoint; a toggle above
  the list shows them again. A device whose exposed clusters or features
  change across a restart is announced to paired controllers through its
  ConfigurationVersion, so they reload it instead of showing stale data.

## 0.3.2

- Library updates (go-fabric 0.14.0, go-fabric-ui 0.2.4); nothing changes
  in use.

## 0.3.1

- Library updates (go-fabric 0.13.6, go-fabric-ui 0.2.3, go-hamqtt 0.39.0);
  nothing changes in use.

## 0.3.0

- The side panel is called **Matter Bridge**, and so is the default bridge
  label. If you never saved the add-on's configuration, controllers may
  show the new name after the update; the bridge stays the same and nothing
  has to be paired again. A saved label is kept.
- The web UI follows your Home Assistant theme from the first moment,
  including light and dark mode and custom themes.

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
