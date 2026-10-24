# syntax=docker/dockerfile:1
# Distroless, non-root, static: the form of the sibling repositories.
# Builds the placeholder main until the first release.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/habridge ./cmd/habridge

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/habridge /habridge
USER nonroot:nonroot
ENTRYPOINT ["/habridge"]
