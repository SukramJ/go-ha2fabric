# go-ha2fabric add-on

Serves your Home Assistant entities to Matter controllers — Apple Home,
Google Home, Amazon Alexa — as one bridge. Which entity becomes which
Matter device is decided by [go-hamapping](https://github.com/SukramJ/go-hamapping);
the add-on shows its verdict for every entity.

The bridge is **uncertified**: it carries the CSA test vendor and a test
attestation chain, and a controller may warn when you pair it.

## Quickstart

1. Install the add-on and start it.
2. Choose what to expose. By default the add-on follows Home Assistant's
   **Settings → Voice assistants → Expose** list (exposed to Assist). Add
   entities with `include`, remove them with `exclude`.
3. Open the add-on's **Web UI** (the side panel). It shows the pairing QR
   code and the manual code, and every entity with the reason it is or is
   not served.
4. In your controller's app, add an accessory and scan the QR code (or type
   the manual code). The code works while no controller has paired yet.
5. **Change the passcode** before you pair for real: the default is a
   public test value.

## Which entities are served

An entity is served when it is

- exposed to Assist in Home Assistant (`expose_assist: true`, the
  default), **or** named in `include`, **or** carries a label of
  `include_labels`,
- **and** not named in `exclude` and carries no label of `exclude_labels`,
- **and** not disabled or hidden in Home Assistant,
- **and** go-hamapping maps it to a Matter device.

A label is named by its name (upper or lower case does not matter) or its
id; a label on a device counts for every entity of the device. Example:
give the device "Living room climate sensor" the label **Matter** and set
`include_labels: [Matter]` — its temperature and battery are served. The
web UI can also turn single entities on or off (and rename them); those
choices win over the lists, never over disabled or hidden, and live in
`/data/selection.json`.

`include` and `exclude` take entity ids or globs: `light.*`,
`sensor.*_temperature`. The add-on never changes Home Assistant's exposure
settings. The web UI's selection page lists every candidate with its
labels, what decided and go-hamapping's verdict.

Mapped today: lights (on/off, dimmable, colour temperature, colour),
switches, sensors (temperature, humidity, pressure, illuminance, power,
voltage, current, energy; a battery shows on its device's main endpoint),
binary sensors (door, window, garage door, motion, occupancy, moisture,
smoke, carbon monoxide), covers, climate (heating, cooling, heat/cool),
locks and fans. Scenes, media players, areas and a climate's fan or preset
modes are not mapped.

## Options reference

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `port` | port | `5540` | UDP port of the Matter node. Change it if another Matter node on this host uses 5540. |
| `passcode` | int | `20202021` | Setup passcode (1 to 99999998; trivial ones such as 11111111 or 12345678 are refused). **Change it.** |
| `discriminator` | int | `3840` | 12-bit discriminator (0 to 4095). |
| `vendor_id` | int | `65521` | Vendor id (0xFFF1, the CSA test vendor). |
| `product_id` | int | `32769` | Product id (0x8001). |
| `label` | str | `Matter Bridge` | The bridge's name as controllers show it (at most 32 bytes). |
| `expose_assist` | bool | `true` | Serve the entities exposed to Assist. |
| `include` | list | `[]` | Entity ids or globs always served. |
| `exclude` | list | `[]` | Entity ids or globs never served. |
| `include_labels` | list | `[]` | Home Assistant labels (name or id) whose entities are served. |
| `exclude_labels` | list | `[]` | Home Assistant labels (name or id) whose entities are never served. |
| `log_level` | list | `info` | `debug`, `info`, `warn` or `error`. |

## Network

Matter needs the host's network: controllers find the bridge over mDNS and
reach it over UDP (`port`) and IPv6 on your LAN, so the add-on runs with
host networking. The web UI is served only through Home Assistant's
Ingress.

## Data and backups

The add-on keeps two files: `/data/ha2fabric.db`, the paired controllers,
their keys and the endpoint numbers, and `/data/selection.json`, the web
UI's per-entity choices. Losing it unpairs the bridge
everywhere. A Home Assistant backup stops the add-on briefly to copy it
consistently. The add-on stores no token and no copy of your
configuration; it reads Home Assistant again on every start.

## Access

The add-on talks to Home Assistant through the Supervisor's proxy as an
administrator (its token is the Supervisor's, never logged or stored). It
reads the entity, device, area and label registries, the states and the
exposure list, and calls the services of the entities it serves, nothing else.

## More

The daemon's [operations page](https://github.com/SukramJ/go-ha2fabric/blob/main/docs/operations.md)
covers ports, logs, health and a factory reset.
