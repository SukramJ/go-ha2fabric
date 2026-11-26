# Third-party notices

This repository is Apache-2.0 (see `LICENSE`). Third-party code it ships is
listed here with its licence.

## npm packages in the built bundle

The committed bundle `fabricui/dist/` (and so every binary that embeds the
Go module) carries these packages, compiled into its `vendor` chunk — except
`jsqr`, the QR decoder, which is compiled into the `scan` chunk the
commissioning page loads on its first camera scan — and `tailwindcss`,
whose generated layers (theme, the preflight base styles derived from
modern-normalize, MIT, and the utilities) are in the bundle's stylesheet.
The JavaScript packages are checked in
CI against the chunks themselves (`make notices-check`:
the build writes the chunk's package list and `ui/scripts/check-notices.mjs`
fails on a package missing here or without its licence text). The full
licence texts are under `licenses/npm/`.

| Package | Version | Licence | Text |
| --- | --- | --- | --- |
| `@lucide/svelte` | 1.74.0 | ISC | [`licenses/npm/lucide-svelte.txt`](./licenses/npm/lucide-svelte.txt) |
| `clsx` | 2.1.1 | MIT | [`licenses/npm/clsx.txt`](./licenses/npm/clsx.txt) |
| `dijkstrajs` | 1.0.3 | MIT | [`licenses/npm/dijkstrajs.txt`](./licenses/npm/dijkstrajs.txt) |
| `jsqr` | 1.4.0 | Apache-2.0 | [`licenses/npm/jsqr.txt`](./licenses/npm/jsqr.txt) |
| `qrcode` | 1.5.4 | MIT | [`licenses/npm/qrcode.txt`](./licenses/npm/qrcode.txt) |
| `svelte` | 5.57.3 | MIT | [`licenses/npm/svelte.txt`](./licenses/npm/svelte.txt) |
| `tailwind-merge` | 3.7.0 | MIT | [`licenses/npm/tailwind-merge.txt`](./licenses/npm/tailwind-merge.txt) |
| `tailwindcss` | 4.3.5 | MIT | [`licenses/npm/tailwindcss.txt`](./licenses/npm/tailwindcss.txt) |

The npm package `@thingfabric/fabric-ui` itself ships sources, not this bundle;
its runtime dependencies (`@lucide/svelte`, `clsx`, `jsqr`, `qrcode`,
`tailwind-merge`; `svelte` is a peer dependency) are installed by the
consumer's package manager with their own licence files. The other
build-time tools (Vite, TypeScript, the test stack) are not shipped.

## Go modules

The Go module depends on go-fabric and, through it, on the modules below,
and on oapi-codegen's runtime, which the generated controller handlers
(`internal/api`) bind parameters with (brief U2, decision 0.1).
They are fetched by the Go toolchain from their own repositories with
their own licence files; none is vendored here. `make licenses` (go-licenses)
fails on a forbidden, restricted, reciprocal or unclassified licence.

| Module | Licence |
| --- | --- |
| `thingfabric.dev/go-fabric` | Apache-2.0 |
| `github.com/oapi-codegen/runtime` | Apache-2.0 |
| `github.com/apapsch/go-jsonmerge/v2` (through the runtime) | MIT |
| `filippo.io/nistec` | BSD-3-Clause |
| `github.com/dustin/go-humanize` | MIT |
| `github.com/google/uuid` | BSD-3-Clause |
| `github.com/mattn/go-isatty` | MIT |
| `github.com/miekg/dns` | BSD-3-Clause |
| `github.com/ncruces/go-strftime` | MIT |
| `github.com/remyoudompheng/bigfft` | BSD-3-Clause |
| `golang.org/x/net` | BSD-3-Clause |
| `golang.org/x/sys` | BSD-3-Clause |
| `modernc.org/libc` | BSD-3-Clause |
| `modernc.org/mathutil` | BSD-3-Clause |
| `modernc.org/memory` | BSD-3-Clause |
| `modernc.org/sqlite` | BSD-3-Clause |

The generator `github.com/oapi-codegen/oapi-codegen/v2` (Apache-2.0) is a
pinned `tool` of `tools/go.mod`, a module of its own (`make generate`); it
writes `internal/api`, is not linked into the module's packages, and its
requirements are not in the root `go.mod`, so they do not reach a
consumer's module graph.

## Ported source

The node pages, their handlers, the UI kit, the design tokens and the
theme bridge are ported from [openccu-loom](https://github.com/SukramJ/openccu-loom)
at commit `b40b289a` (Apache-2.0, same owner); every ported file names its
origin with the line range at that commit.
