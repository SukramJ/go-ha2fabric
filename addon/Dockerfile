# SPDX-License-Identifier: Apache-2.0
# Home Assistant add-on Dockerfile (the local-build fallback; the preferred path
# is the pre-built image `image:` in addon/config.yaml names, built by
# .github/workflows/addon-image.yml).
#
# The Go sources are at the repository root: build with the root as context,
#
#   docker build --build-arg BUILD_ARCH=amd64 -f addon/Dockerfile -t go-ha2fabric-addon .
#
# BUILD_ARCH (amd64, aarch64; the Supervisor passes it) picks the runtime
# base below. Every base is pinned by tag and digest, one FROM line each so
# that Dependabot (docker ecosystem) keeps them current (decision 62,
# HR-P3-sup-34); a BUILD_FROM the Supervisor passes is not used. No armv7:
# Home Assistant publishes no armv7 base after 3.22-2025.11.1 (decision 64).

# An ARG used in a FROM is declared before the first FROM.
ARG BUILD_ARCH=amd64

# ---------- The per-arch Home Assistant base images ----------
FROM ghcr.io/home-assistant/amd64-base:3.24-2026.11.1@sha256:d4d8bc2b0b4b6158a2c8c863f2a4d20e84d98f829724bf34c3b165f8e3cee33f AS base-amd64
FROM ghcr.io/home-assistant/aarch64-base:3.24-2026.11.1@sha256:c1c5c5d83c7b5e75489c0751da127b07314fcf1984c99e24d0dbbe5f1184b4e5 AS base-aarch64

# ---------- Stage 1: build the Go binary ----------
FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=addon
ARG COMMIT=none
ARG BUILD_DATE=unknown
ENV CGO_ENABLED=0
RUN go build -trimpath \
      -ldflags="-s -w \
        -X thingfabric.dev/go-ha2fabric/internal/version.Version=${VERSION} \
        -X thingfabric.dev/go-ha2fabric/internal/version.Commit=${COMMIT} \
        -X thingfabric.dev/go-ha2fabric/internal/version.BuildDate=${BUILD_DATE}" \
      -o /out/ha2fabric ./cmd/ha2fabric

# ---------- Stage 2: Home Assistant add-on runtime ----------
FROM base-${BUILD_ARCH}
COPY --from=builder /out/ha2fabric /usr/bin/ha2fabric
COPY script/run.sh /run.sh
RUN chmod +x /run.sh
# The licence texts of everything linked (decision 59, HR-P3-sup-37).
COPY LICENSE THIRD-PARTY-NOTICES.md /usr/share/doc/go-ha2fabric/
COPY licenses /usr/share/doc/go-ha2fabric/licenses
# The container's SIGTERM goes to the CMD, which run.sh exec's into the
# daemon, and s6-overlay ends when the daemon exits: the stop takes the
# daemon's one budget of 25 s within the manifest's timeout: 30. Without it
# s6-overlay TERMs the CMD last and KILLs it after S6_KILL_GRACETIME, 3 s
# (HR-P4-cfg-01, decision 75 of the 2026-11-23 review).
ENV S6_CMD_RECEIVE_SIGNALS=1
ARG BUILD_ARCH
ARG VERSION=addon
ARG BUILD_VERSION=${VERSION}
LABEL \
  io.hass.version="${BUILD_VERSION}" \
  io.hass.type="addon" \
  io.hass.arch="${BUILD_ARCH}" \
  org.opencontainers.image.source="https://github.com/thingfabric/go-ha2fabric" \
  org.opencontainers.image.licenses="Apache-2.0"
CMD ["/run.sh"]
