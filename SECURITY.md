# Security policy

## Reporting a vulnerability

Please report a vulnerability privately through GitHub's private
vulnerability reporting:
<https://github.com/thingfabric/go-ha2fabric/security/advisories/new>
(the repository's **Security** tab, **Report a vulnerability**). Do not
open a public issue, pull request or discussion for it.

Include the version (`ha2fabric --version`, or the add-on's version), how it
runs (add-on, container image or binary), what an attacker needs (network
position, credentials, a commissioned controller) and the steps to reproduce.
Never include your Home Assistant token, a pairing code or a `/data` file
in a report; redact them from logs first.

A fix is released as a patch version; the advisory is published with it.

## Supported versions

Only the newest release receives fixes. The add-on and the container image
follow the release tags (`ghcr.io/thingfabric/go-ha2fabric`,
`ghcr.io/thingfabric/go-ha2fabric-addon-<arch>`); the images under
`ghcr.io/sukramj/` stay at 0.6.3 and receive nothing.

## Scope

go-ha2fabric's own code, its add-on manifest, images and release workflow.
A vulnerability in a dependency belongs to that project: go-fabric
(Matter), go-fabric-ui (the web UI), go-hamapping (the mapping) or Home
Assistant itself; report it there, and here when this project's use of it
makes it exploitable. What the daemon defends against, and what it does
not, is in [`docs/operations.md`](docs/operations.md).
