# go-ha2fabric add-on

Serves your Home Assistant entities to Matter controllers — Apple Home,
Google Home, Amazon Alexa — as one bridge, or one per ecosystem. Which entity becomes which
Matter device is decided by [go-hamapping](https://github.com/SukramJ/go-hamapping);
the add-on shows its verdict for every entity.

The bridge is **uncertified**: it carries the CSA test vendor and a test
attestation chain, and a controller may warn when you pair it.

It runs on amd64 and aarch64 (64-bit ARM). There is no armv7 build after
0.7.0: Home Assistant publishes no armv7 base image any more, so an
armv7 installation stays at 0.7.0.

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
5. The passcode is random unless you set the `passcode` option: the add-on
   draws one on its first start and keeps it. Options saved before 0.7.0
   keep the old default 20202021, a public test value; the status page
   warns about it (`passcode_test_value`) — remove the option for a random
   one, or set your own. An installation whose options were never saved
   gets a random passcode with the update. A paired bridge stays paired
   either way.

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

Mapped today (the rows of [go-hamapping v0.8.0](https://github.com/SukramJ/go-hamapping/blob/v0.8.0/docs/rows.md)): lights (on/off, dimmable, colour
temperature, colour), switches, and scenes, scripts, automations and
buttons as on/off plugs; sensors (temperature, humidity, pressure,
illuminance, power, voltage, current, energy, frequency, flow, air quality;
a battery shows on its device's main endpoint), binary sensors (door,
window, garage door, motion, occupancy, moisture, freeze, smoke, carbon
monoxide), covers, climate (heating, cooling, heat/cool, an air
conditioner's fan modes), locks, fans, media players (speakers, TVs),
vacuums, valves, water heaters, selects, events (as a generic switch) and,
with the `camera` option, cameras. Areas and a climate's preset modes are
not mapped; the selection page shows each entity's verdict and reason.

## Options reference

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `port` | port | `5540` | UDP port of the Matter node. Change it if another Matter node on this host uses 5540. |
| `passcode` | int | unset (random) | Setup passcode (1 to 99999998; trivial ones such as 11111111 or 12345678 are refused). Unset, the add-on draws a random one on its first start and keeps it in `/data/passcode.json`; the codes are on the web UI's pairing page. The public test value 20202021 is honoured with a warning. |
| `discriminator` | int | `3840` | 12-bit discriminator (0 to 4095). |
| `vendor_id` | int | `65521` | Vendor id (0xFFF1, the CSA test vendor). |
| `product_id` | int | `32769` | Product id (0x8001). |
| `label` | str | `Matter Bridge` | The bridge's name as controllers show it (at most 32 bytes). |
| `expose_assist` | bool | `true` | Serve the entities exposed to Assist. |
| `camera` | bool | `false` | Serve cameras with a live stream (opt-in, not yet tested with Apple Home). The bridge relays the WebRTC signalling to Home Assistant (go2rtc) and also accepts Matter over TCP on its port. |
| `include` | list | `[]` | Entity ids or globs always served. |
| `exclude` | list | `[]` | Entity ids or globs never served. |
| `include_labels` | list | `[]` | Home Assistant labels (name or id) whose entities are served. A label whose name holds a comma is named by its id. |
| `exclude_labels` | list | `[]` | Home Assistant labels (name or id) whose entities are never served. |
| `log_level` | list | `info` | `debug`, `info`, `warn` or `error`. |
| `node_limit` | int | `8` | How many nodes the web UI may create beside this one (0 to 32; see Nodes). |
| `mdns_interfaces` | list | `[]` | Host interfaces the bridge announces itself on over mDNS, by name (`enp0s18`, `vlan30`); empty is every interface. For a host on two networks whose router relays mDNS between them: name the one the controllers are on (the project's `docs/operations.md`, mDNS). Applies to every node. |

## Nodes

The options above configure the add-on's **first node**. The web UI's
node selector (in its header) creates further nodes — one per ecosystem,
when Apple Home and Alexa should see different devices: **Create node…**,
give it a name, pick its entities on its **Selection** page, and pair it
from its **Pair** page with its own code. A new node serves nothing until
you pick its entities; the `include`/`exclude` lists and
`expose_assist` belong to the first node.

The add-on chooses each new node's port (the next free one above `port`),
its discriminator and its passcode; only its name can be changed later.
Every node starts with the add-on. **Deleting a node is final**: the
controllers that paired it lose it, and its store and selection are
removed. The first node cannot be deleted; its values stay the options.
**Erase** on a node's **Fabrics** page, the first node included, starts
it over for the controllers — every one of them loses it and pairs it
again with the same code — and keeps its entities.

## Network

Matter needs the host's network: controllers find the bridge over mDNS and
reach it over UDP (`port`, and the ports of the nodes created in the web
UI) and IPv6 on your LAN, so the add-on runs with host networking; a new
node's port needs no mapping. The web UI is served only through Home Assistant's
Ingress.

## Data and backups

The add-on keeps two files: `/data/ha2fabric.db`, the paired controllers,
their keys and the endpoint numbers, and `/data/selection.json`, the web
UI's per-entity choices. Each node created in the web UI adds its own
pair, `/data/ha2fabric-<key>.db` and `/data/selection-<key>.json`, and
`/data/nodes.json` lists those nodes with their ports and passcodes;
`/data/passcode.json` holds the bridge's own random passcode when the
`passcode` option is unset. Losing the store unpairs the bridge
everywhere. A Home Assistant backup stops the add-on briefly to copy it
consistently. The add-on stores no token and no copy of your
configuration; it reads Home Assistant again on every start.

**Updating to 0.7.0 is one-way:** the add-on upgrades its store at the first
start, and an older version is not expected to open it. Take a backup
before the update; to go back, restore the add-on with its data from that
backup.

## Access

The add-on talks to Home Assistant through the Supervisor's proxy as an
administrator (its token is the Supervisor's, never logged or stored). It
reads the entity, device, area and label registries, the states and the
exposure list, and calls the services of the entities it serves, nothing else.

The web UI's administrator pages — the selection, pairing with its code,
fabrics, the node registry, erase and the log — are for Home Assistant
administrators only: the add-on reads the user's groups the Supervisor
passes on with every Ingress request. That needs **Supervisor 2026.10.4 or
later**, which the add-on reads at start; on an older Supervisor, or until
it has read the version, the web UI shows the status page only, with a
warning (`supervisor_too_old`, `supervisor_unknown`). Any other Home
Assistant user sees the status page only. The check rests on the address
of the Supervisor's Ingress proxy, which another add-on on the same Home
Assistant can claim on the Supervisor's internal network: install only
add-ons you trust as an administrator, and remove a fabric on the Fabrics
page that nobody paired ([operations page, "Co-installed add-ons"](https://github.com/thingfabric/go-ha2fabric/blob/main/docs/operations.md#the-web-ui)).

## More

The daemon's [operations page](https://github.com/thingfabric/go-ha2fabric/blob/main/docs/operations.md)
covers ports, logs, health and a factory reset.
