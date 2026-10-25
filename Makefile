# go-ha2fabric — make targets. `make help` lists them.
GO ?= go

MODULE   := github.com/SukramJ/go-ha2fabric
PKG_VER  := $(MODULE)/internal/version
BIN_DIR  := bin
DIST_DIR := dist

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w -X $(PKG_VER).Version=$(VERSION) -X $(PKG_VER).Commit=$(COMMIT) -X $(PKG_VER).BuildDate=$(BUILD_DATE)

# The release matrix: linux/amd64, linux/arm64, linux/arm/v7 (CONCEPT.md,
# Packaging). RELEASE_VERSION is internal/version's default — bumping it, the
# changelog section and addon/config.yaml's version happen in one commit.
RELEASE_TARGETS ?= linux/amd64 linux/arm64 linux/arm/v7
RELEASE_VERSION ?= $(shell awk -F'"' '/^[[:space:]]*Version = /{print $$2; exit}' internal/version/version.go)
RELEASE_PAYLOAD := README.md LICENSE changelog.md THIRD-PARTY-NOTICES.md

DOCKER       ?= docker
DOCKER_IMAGE ?= go-ha2fabric
ADDON_BASE   ?= ghcr.io/home-assistant/amd64-base:latest
FUZZTIME     ?= 10000x

GOFUMPT_VERSION       ?= v0.12.0
GOLANGCI_LINT_VERSION ?= v2.14.0
GOVULNCHECK_VERSION   ?= v1.8.0
GOLICENSES_VERSION    ?= v1.6.0
GITLEAKS_VERSION      ?= v8.30.1

.DEFAULT_GOAL := help

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

setup: ## install the pinned gofumpt, golangci-lint, govulncheck and go-licenses
	$(GO) install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$(GO) install github.com/google/go-licenses@$(GOLICENSES_VERSION)

build: ## go build ./... and the daemon into bin/
	CGO_ENABLED=0 $(GO) build ./...
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/ha2fabric ./cmd/ha2fabric

vet: ## go vet ./...
	$(GO) vet ./...

test: ## go test -count=1 ./...
	CGO_ENABLED=0 $(GO) test -count=1 -timeout 300s ./...

race: ## go test -race (CGO_ENABLED=1 for this run only)
	CGO_ENABLED=1 $(GO) test -race -count=1 -timeout 600s ./...

fmt: ## gofumpt -w .
	gofumpt -w .

fmt-check: ## fail when a file is not gofumpt-clean
	@diff=$$(gofumpt -l .); if [ -n "$$diff" ]; then echo "gofumpt would rewrite:"; echo "$$diff"; exit 1; fi

lint: fmt-check ## gofumpt check + golangci-lint + go mod tidy -diff + go mod verify
	golangci-lint config verify
	golangci-lint run --timeout=5m ./...
	$(GO) mod tidy -diff
	$(GO) mod verify

tidy-check: ## verify go.mod/go.sum are tidy and the module checksums verify
	$(GO) mod tidy -diff
	$(GO) mod verify

cover: ## coverage profile of every package, as the floors count it
	CGO_ENABLED=0 $(GO) test -count=1 -timeout 300s -coverpkg=./... -coverprofile=coverage.out ./...

cover-check: cover ## fail when a package is below its floor (script/coverfloor/floors.txt)
	$(GO) run ./script/coverfloor coverage.out

fuzz-smoke: ## every fuzz target, $(FUZZTIME) each
	$(GO) test -run '^$$' -fuzz '^FuzzDecode$$' -fuzztime $(FUZZTIME) ./internal/ha/

vuln: ## govulncheck ./...
	govulncheck ./...

# GOROOT explicit: go-licenses tells the standard library by its GOROOT.
# modernc.org/mathutil's LICENSE is BSD-3-Clause; the classifier does not
# recognise its wording and logs it without failing the check (go-fabric's
# cookbook §1).
licenses: ## fail on a forbidden, restricted or reciprocal dependency licence (go-licenses)
	GOROOT=$$($(GO) env GOROOT) go-licenses check ./... --disallowed_types=forbidden,restricted,reciprocal

# The daemon reads a Home Assistant token; the fixtures carry none.
secrets: ## scan the tracked git history for committed secrets (gitleaks)
	$(GO) run github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION) git --no-banner --redact .

docker: ## build the distroless daemon image (Dockerfile)
	$(DOCKER) build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) -t $(DOCKER_IMAGE):$(VERSION) .

addon-image: ## build the add-on image for amd64 (addon/Dockerfile on ADDON_BASE)
	$(DOCKER) build --build-arg BUILD_FROM=$(ADDON_BASE) --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) -f addon/Dockerfile -t $(DOCKER_IMAGE)-addon:$(VERSION) .

version-check: ## fail when internal/version, addon/config.yaml and changelog.md disagree
	@v=$(RELEASE_VERSION); a=$$(awk -F'"' '/^version:/{print $$2; exit}' addon/config.yaml); \
	test "$$v" = "$$a" || { echo "internal/version $$v, addon/config.yaml $$a"; exit 1; }; \
	grep -q "^## \[$$v\]" changelog.md || { echo "changelog.md has no ## [$$v] section"; exit 1; }; \
	echo "version $$v: internal/version, addon/config.yaml and changelog.md agree"

release: version-check ## stage the cross-compiled release archives and notes into dist/ (no upload)
	@rm -rf $(DIST_DIR) && mkdir -p $(DIST_DIR)
	@echo "release version: $(RELEASE_VERSION)"
	@version="$(RELEASE_VERSION)"; \
	ldflags="-s -w -X $(PKG_VER).Version=$$version -X $(PKG_VER).Commit=$(COMMIT) -X $(PKG_VER).BuildDate=$(BUILD_DATE)"; \
	for tgt in $(RELEASE_TARGETS); do \
	  goos=$$(echo $$tgt | cut -d/ -f1); goarch=$$(echo $$tgt | cut -d/ -f2); goarm=$$(echo $$tgt | cut -d/ -f3 | tr -d v); \
	  name="go-ha2fabric-$$version-$$goos-$$goarch$${goarm:+v$$goarm}"; stage="$(DIST_DIR)/$$name"; \
	  mkdir -p "$$stage"; echo "==> $$tgt -> $$stage"; \
	  CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch GOARM=$$goarm $(GO) build -trimpath -ldflags="$$ldflags" -o "$$stage/ha2fabric" ./cmd/ha2fabric || exit 1; \
	  cp $(RELEASE_PAYLOAD) "$$stage/"; \
	  ( cd $(DIST_DIR) && tar -czf "$$name.tar.gz" "$$name" ); rm -rf "$$stage"; \
	done
	@cd $(DIST_DIR) && sha256sum *.tar.gz > SHA256SUMS
	@script/extract-release-notes.sh $(RELEASE_VERSION) > $(DIST_DIR)/RELEASE_NOTES.md
	@ls -lh $(DIST_DIR)

ci: build vet test lint ## everything the pipeline's main jobs run

clean: ## remove build artefacts
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out

version: ## print the resolved build metadata
	@echo "VERSION    = $(VERSION)"
	@echo "COMMIT     = $(COMMIT)"
	@echo "BUILD_DATE = $(BUILD_DATE)"

.PHONY: help setup build vet test race fmt fmt-check lint tidy-check cover cover-check fuzz-smoke vuln licenses secrets docker addon-image version-check release ci clean version
