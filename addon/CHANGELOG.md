# Changelog

Changes to the **go-ha2fabric** Home Assistant add-on. The add-on version
tracks the project release; the project's
[changelog.md](https://github.com/SukramJ/go-ha2fabric/blob/main/changelog.md)
has the details.

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
