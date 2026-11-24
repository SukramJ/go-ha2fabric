# syntax=docker/dockerfile:1
# SPDX-License-Identifier: Apache-2.0
# The daemon's plain image: distroless, non-root, static (CGO_ENABLED=0). Run it
# on the host's network, the store on a volume:
#
#   docker run --network host -v ha2fabric:/data \
#     -e HA_URL=ws://homeassistant.local:8123/api/websocket -e HA_TOKEN=... \
#     ghcr.io/sukramj/go-ha2fabric
#
# The web UI listens on the binary's default, 127.0.0.1:8099 (decision 27 of
# the 2026-11-23 review); to reach it from the network set WEB_LISTEN, with
# WEB_ALLOW naming the clients for WEB_ADMIN (docs/operations.md, The web UI):
#
#   -e WEB_LISTEN=0.0.0.0:8099 -e WEB_ALLOW=192.0.2.10

FROM golang:1.27.4-alpine AS build
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

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ha2fabric /ha2fabric
COPY --from=build --chown=nonroot:nonroot /out/data /data
VOLUME ["/data"]
ENV STORE_PATH=/data/ha2fabric.db
# 5540/tcp is Matter over TCP with CAMERA; 8099/tcp the web UI once
# WEB_LISTEN leaves loopback.
EXPOSE 5540/udp 5540/tcp 8099/tcp
USER nonroot:nonroot
ENTRYPOINT ["/ha2fabric"]
