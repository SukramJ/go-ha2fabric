# go-ha2fabric — Home Assistant add-on

This add-on runs the [go-ha2fabric](https://github.com/SukramJ/go-ha2fabric)
daemon inside Home Assistant: your entities, served to Matter controllers
(Apple Home, Google Home, Amazon Alexa) as one bridge. The bridge is
uncertified; a controller may warn when pairing it.

## Installation

1. In Home Assistant go to **Settings → Add-ons → Add-on Store**.
2. Open the **⋮** menu → **Repositories** and add
   `https://github.com/SukramJ/go-ha2fabric`.
3. Install **go-ha2fabric**, start it and open its **Web UI** for the
   pairing code. [`DOCS.md`](DOCS.md) has the options.

## Images

The add-on pulls the pre-built image `ghcr.io/sukramj/go-ha2fabric-addon-{arch}`
(`.github/workflows/addon-image.yml`): a Home Assistant base image with the
daemon and `script/run.sh`, which maps the options onto the daemon's
environment. The distroless image `ghcr.io/sukramj/go-ha2fabric` runs the
daemon directly and is not an add-on image. To build the add-on on the host
instead, remove `image:` from `config.yaml`; the Supervisor then builds
`Dockerfile` from `build.yaml`'s base images.
