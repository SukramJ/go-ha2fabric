# go-ha2fabric — make targets. `make help` lists them.
GO ?= go
GOFUMPT_VERSION ?= v0.12.0
GOLANGCI_LINT_VERSION ?= v2.14.0

.DEFAULT_GOAL := help

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

setup: ## install the pinned gofumpt and golangci-lint
	$(GO) install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

build: ## go build ./...
	CGO_ENABLED=0 $(GO) build ./...

vet: ## go vet ./...
	$(GO) vet ./...

test: ## go test -count=1 ./...
	CGO_ENABLED=0 $(GO) test -count=1 ./...

race: ## go test -race (needs CGO_ENABLED=1)
	CGO_ENABLED=1 $(GO) test -race -count=1 ./...

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

vuln: ## govulncheck ./...
	govulncheck ./...

licenses: ## fail on a forbidden or restricted dependency licence (go-licenses)
	go-licenses check ./... --disallowed_types=forbidden,restricted

ci: build vet test lint ## everything the pipeline runs

.PHONY: help setup build vet test race fmt fmt-check lint tidy-check vuln licenses ci
