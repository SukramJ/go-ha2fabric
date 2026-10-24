# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Home Assistant entities as a bridge for Matter controllers, in pure Go on go-fabric. Twin of `go-hafabric`; both are built on
[go-fabric](https://github.com/SukramJ/go-fabric) and share its project
setup. **State: concept, not started.** `CONCEPT.md` is the design; the
go-fabric roadmap names the milestone in which work starts. Nothing runs
yet: `go build ./...` compiles a `doc.go` and a placeholder main.

## Commands

```sh
make ci        # build vet test lint — what the pipeline runs
make race      # go test -race (CGO_ENABLED=1 for this run only)
make fmt       # gofumpt -w .
```

Go 1.27.2, `CGO_ENABLED=0`, no dependencies yet. Every further dependency
is a decision: Apache-2.0 / MIT / BSD only, and it updates the notices.

## Rules

- **Naming.** "Matter" is a trademark of the Connectivity Standards
  Alliance and is not part of this project's name, its packages, its
  binaries or its images; it appears only descriptively. See the README.
- **go-fabric is the gold standard** for everything on the Matter side:
  device types, clusters, the facade (`node`, `device`, `contract`, the
  `<cluster>def` packages). This repository never reimplements a
  cluster or reaches below go-fabric's facade.
- **Which model does which work** (as go-fabric's CLAUDE.md): analysis,
  planning, anything that becomes a fact about Matter or Home Assistant
  in the repository is the strongest model's; implementation of a
  finished plan with guards behind it may use a cheaper one; a
  generated or formatted artefact a test checks may use the cheapest.
- **Official documents in English**: issues, pull requests, docs, notes.
- **One pull request per wave**, squash-merged; CI is the gate.
- **No attribution lines** in commits or pull request bodies.
- A user-visible change gets a `changelog.md` entry under `[Unreleased]`.
