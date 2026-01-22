---
aliases:
  - /_uid/9a3b1d41-7504-4c45-9a93-f18b4d6ccf1b/
categories:
  - architecture-decisions
date: 2026-01-22T00:00:00Z
fingerprint: c9937c835e27979ba5dfdcd89eb195bae44a32e54709ded4d5f14af5171c2874
lastmod: "2026-01-22"
tags:
  - daemon
  - refactor
  - implementation-plan
  - preview
  - http
  - discovery
  - build
uid: 9a3b1d41-7504-4c45-9a93-f18b4d6ccf1b
---

# ADR-017 Implementation Plan: Split daemon responsibilities

**Status**: In Progress  
**Date**: 2026-01-22  
**Decision Makers**: DocBuilder Core Team

This document is the execution plan for [ADR-017: Split daemon responsibilities](adr-017-split-daemon-responsibilities.md).

## Goal

Reduce the scope of `internal/daemon` to a lifecycle + wiring composition root by extracting preview, HTTP server wiring, build queue, discovery runner, and status view model into focused packages with clear dependency direction.

## Guardrails

- No CLI behavior changes (flags/subcommands) unless explicitly justified.
- Preserve runtime behavior (start/stop ordering, ports, routes, build triggers).
- Avoid `internal/daemon` imports outside the daemon package.
- Prefer small interfaces over passing `*Daemon`.

## Execution Rules

- Use a strict TDD approach (add/adjust tests first, watch them fail, then implement).
- Update this plan after each completed step (mark the step as completed and note any deviations).
- Create a conventional commit after each completed step as a checkpoint.
- Unless user input is required, continue step-by-step until the plan is fully completed.

## Work Items (ordered)

### 1) Extract preview mode

**Target**: move local preview logic out of daemon.

**Status**: Completed (2026-01-22)

- Create `internal/preview` package.
- Move preview watcher/debounce/rebuild loop from daemon preview code.
- Update [cmd/docbuilder/commands/preview.go](../../cmd/docbuilder/commands/preview.go) to use `internal/preview`.
- Keep preview dependent on build pipeline + HTTP server wiring only.

**Definition of Done**

- Preview command compiles and runs.
- `internal/daemon` no longer contains preview-only concerns.
- Tests referencing preview behavior are updated and still pass.

**Notes / Deviations**

- Preview entrypoint moved to `internal/preview` and CLI now calls it.
- Preview initially reused daemon HTTP server wiring (`daemon.NewHTTPServer`) until Step 2.
- Introduced `daemon.NewPreviewDaemon(...)` to construct the minimal daemon required by the HTTP server.
- Exported the build status method (`GetStatus`) so preview build status can be implemented outside `internal/daemon`.

### 2) Extract HTTP server wiring

**Target**: separate HTTP runtime wiring from daemon lifecycle.

**Status**: Completed (2026-01-22)

- Create `internal/server/httpserver` (name can change) to own:
  - `HTTPServer` start/stop
  - port prebinding
  - route wiring for docs/admin/webhook/prom/livereload
- Keep request handlers in `internal/server/handlers`.
- Define small adapter interfaces for handler dependencies (status/build triggers/metrics access).
- Make daemon implement adapters (or create a thin adapter type).

**Definition of Done**

- Daemon uses `httpserver.New(...)` instead of owning HTTP server internals.
- Preview can reuse the HTTP server wiring with a different adapter.
- HTTP-related tests continue to pass.

**Notes / Deviations**

- Implemented as `internal/server/httpserver` with `httpserver.New(cfg, runtime, opts)` and a `Runtime` interface.
- Preview and daemon now both construct the server via `httpserver.New(...)`.
- Build-status and LiveReload behavior in the docs server is driven via injected options (`Options.BuildStatus`, `Options.LiveReloadHub`) instead of direct daemon references.
- Moved VS Code edit handler into the new httpserver package so preview/daemon wiring stays centralized.

### 3) Extract build queue + job model

**Target**: make build queue a reusable service with stable APIs.

- Create `internal/build/queue`.
- Move:
  - `BuildQueue`, `BuildJob`, type/priority/status enums
  - retry policy configuration + metrics recorder usage
  - event emission interface (`BuildEventEmitter`)
- Replace scheduler’s daemon back-reference with an `Enqueuer` interface.

**Definition of Done**

- Daemon depends on `internal/build/queue`.
- Queue package has unit tests for retry/backoff and worker behavior.
- No `internal/build/queue` code imports `internal/daemon`.

### 4) Extract discovery runner + cache

**Target**: discovery orchestration independent of daemon.

- Create `internal/forge/discoveryrunner` (or `internal/services/discovery`).
- Move:
  - discovery runner orchestration
  - discovery cache for status queries
- Make it enqueue builds via an interface (not direct queue type).

**Definition of Done**

- Daemon calls runner service via explicit methods.
- Status can use cache snapshots without deep daemon locks.

### 5) Relocate status view model

**Target**: make status rendering a server concern.

- Move status DTOs and HTML rendering helpers into server/admin handler package.
- Provide a minimal `StatusProvider` interface for daemon/preview.

**Definition of Done**

- Status handler composes data from interfaces/caches.
- Daemon no longer owns UI rendering code.

### 6) Move delta bookkeeping out of daemon

**Target**: delta/hash logic belongs to build.

- Move delta manager helpers to `internal/build/delta` (or the appropriate build-stage package).
- Keep state interactions behind `internal/state` interfaces.

**Definition of Done**

- No delta logic remains in daemon.
- Golden/integration tests for partial builds continue to pass.

## Validation Checklist

- `go test ./...`
- `go test ./test/integration -v`
- `golangci-lint run --fix` then `golangci-lint run`
- No imports of `internal/daemon` outside that package

## Rollout Notes

Do the extraction in separate commits/PRs if needed (one subsystem per PR) to keep reviews focused and reduce risk.
