# SPDX-License-Identifier: Apache-2.0
# The daemon's plain image: distroless, non-root, static (CGO_ENABLED=0). Run it
# on the host's network, the store on a volume, with a stop timeout above the
# daemon's stop budget of 25 s (Docker's default kills after 10 s; Compose:
# stop_grace_period: 30s):
#
#   docker run --network host --stop-timeout 30 -v ha2fabric:/data \
#     -e HA_URL=ws://homeassistant.local:8123/api/websocket -e HA_TOKEN=... \
#     ghcr.io/thingfabric/go-ha2fabric
#
# The web UI listens on the binary's default, 127.0.0.1:8099 (decision 27 of
# the 2026-11-23 review); to reach it from the network set WEB_LISTEN, with
# WEB_ALLOW naming the clients for WEB_ADMIN (docs/operations.md, The web UI):
#
#   -e WEB_LISTEN=0.0.0.0:8099 -e WEB_ALLOW=192.0.2.10
#
# Base images by tag and digest; Dependabot (docker ecosystem) keeps both
# current (decision 62, HR-P3-sup-34). The licence texts of everything linked
# are in /usr/share/doc/go-ha2fabric/ (decision 59, HR-P3-sup-37).

FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
ENV CGO_ENABLED=0
RUN go build -trimpath \
      -ldflags="-s -w \
        -X thingfabric.dev/go-ha2fabric/internal/version.Version=${VERSION} \
        -X thingfabric.dev/go-ha2fabric/internal/version.Commit=${COMMIT} \
        -X thingfabric.dev/go-ha2fabric/internal/version.BuildDate=${BUILD_DATE}" \
      -o /out/ha2fabric ./cmd/ha2fabric && \
    mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/ha2fabric /ha2fabric
COPY LICENSE THIRD-PARTY-NOTICES.md /usr/share/doc/go-ha2fabric/
COPY licenses /usr/share/doc/go-ha2fabric/licenses
COPY --from=build --chown=nonroot:nonroot /out/data /data
VOLUME ["/data"]
ENV STORE_PATH=/data/ha2fabric.db
# 5540/tcp is Matter over TCP with CAMERA; 8099/tcp the web UI once
# WEB_LISTEN leaves loopback.
EXPOSE 5540/udp 5540/tcp 8099/tcp
LABEL \
  org.opencontainers.image.source="https://github.com/thingfabric/go-ha2fabric" \
  org.opencontainers.image.licenses="Apache-2.0"
USER nonroot:nonroot
ENTRYPOINT ["/ha2fabric"]
