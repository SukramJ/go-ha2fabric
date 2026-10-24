# syntax=docker/dockerfile:1
# Distroless, non-root, static: the form of the sibling repositories.
# Builds the placeholder main until the first release.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ha2fabric ./cmd/ha2fabric

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ha2fabric /ha2fabric
USER nonroot:nonroot
ENTRYPOINT ["/ha2fabric"]
