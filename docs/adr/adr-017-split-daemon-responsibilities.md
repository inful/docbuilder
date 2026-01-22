---
aliases:
  - /_uid/2d7f1a48-79a7-4dc2-8e59-13f97e3b0a79/
categories:
  - architecture-decisions
date: 2026-01-22T00:00:00Z
fingerprint: 256410563f3517d356c6330f413d409c5c39af556665ef19cc05ed970fb6731b
lastmod: "2026-01-22"
tags:
  - daemon
  - refactor
  - architecture
  - preview
  - http
  - discovery
  - build
uid: 2d7f1a48-79a7-4dc2-8e59-13f97e3b0a79
---

# ADR-017: Split daemon responsibilities (package boundaries)

**Status**: Proposed  
**Date**: 2026-01-22  
**Decision Makers**: DocBuilder Core Team

**Implementation Plan**: [adr-017-implementation-plan.md](adr-017-implementation-plan.md)

## Context and Problem Statement

The `internal/daemon` package has been split into smaller files, but it still acts as a “god package” that owns many unrelated responsibilities:

- Daemon lifecycle and orchestration (`Daemon`, main loop)
- Build queue, job definitions, retry policy, build service adaptation
- Forge discovery scheduling and cache
- HTTP server wiring and routing (docs/webhook/admin/prom/livereload)
- UI-ish concerns (status DTOs, HTML rendering)
- Preview mode (local filesystem watcher + rebuild debounce)
- VS Code edit link plumbing
- Metrics collection and exposition
- Event emission glue for event-sourced build history
- Delta/hash bookkeeping and state interactions

This broad scope has concrete costs:

- It is hard to test components in isolation (many tests need a daemon-shaped dependency graph).
- Minor changes cause wide rebuild/retest churn and increase merge conflicts.
- The daemon package becomes the default “place to put things”, diluting clear ownership.
- Running the daemon and running local preview are coupled, even though they are distinct products.

We want `internal/daemon` to be primarily a runtime composition root and lifecycle controller, not a dumping ground for unrelated subsystems.

### Symptoms we see today

- Changes in one daemon feature frequently require touching unrelated files (higher merge conflict rate).
- Tests are harder to target because many components are only accessible through a daemon-shaped object graph.
- Coupling between “daemon mode” and “preview mode” makes it hard to evolve them independently.
- Interfaces are sometimes inverted (e.g., scheduler needs `*Daemon` back-references) to avoid import cycles.

## Decision

We will refactor daemon-mode code into a set of focused packages with explicit interfaces between them.

### High-level rule

- `internal/daemon` stays responsible for:
  - Lifecycle and state transitions
  - Dependency wiring
  - The main loop and stop/shutdown sequencing
  - Minimal “glue” interfaces for adapters

Everything else moves behind clearer boundaries.

### Dependency direction rules

- `internal/daemon` may depend on “leaf” packages (queue, discovery runner, http server, preview).
- “Leaf” packages must not depend on `internal/daemon`.
- HTTP handlers depend on small interfaces, not concrete daemon structs.
- Anything that needs to enqueue builds depends on an `Enqueuer` interface (not a daemon back-reference).

### Proposed package boundaries

1. **Preview mode**
   - Move local preview behavior (watcher, debouncer, preview build loop) out of `internal/daemon`.
   - New package: `internal/preview`
   - CLI commands (`cmd/docbuilder/commands/preview.go`) call `internal/preview`.
   - Preview must not require daemon-only dependencies (event store, forge discovery).

2. **HTTP server**
   - Extract the HTTP server wiring (`HTTPServer`) into a server-oriented package.
   - New package: `internal/server/httpserver` (name TBD)
   - Daemon provides an adapter implementing narrow interfaces required by handlers.
   - Preview mode can also reuse the HTTP server wiring with a different adapter.

3. **Build queue + scheduler**
   - Extract `BuildQueue` and job model into a build-oriented package.
   - New package: `internal/build/queue` (name TBD)
   - Scheduler depends on an `Enqueuer` interface rather than a `*Daemon` back-reference.
   - The queue exposes lifecycle hooks/events via interfaces (e.g., `BuildEventEmitter`) so event sourcing stays optional.

4. **Discovery runner + cache**
   - Extract discovery orchestration (forge discovery → enqueue build) into a dedicated service.
   - New package: `internal/forge/discoveryrunner` or `internal/services/discovery` (name TBD)
   - The runner returns a structured result (repos found/filtered/errors + timing) for status display.

5. **Status and admin “view model”**
   - Move status DTOs/HTML rendering next to the admin/status HTTP handler.
   - New location: `internal/server/handlers` (or `internal/server/admin`) under an explicit interface to query daemon state.
   - The status view is fed by cached snapshots (queue length, last discovery result, last build report), not by deep daemon internals.

6. **Delta/hash bookkeeping**
   - Move delta-related helpers into build-stage code (where the report is created and hashes are computed).
   - New package: `internal/build/delta` (name TBD)

7. **Metrics**
   - If the daemon continues to own an in-process metrics collector, keep it behind a dedicated package.
   - New package: `internal/observability/daemonmetrics` (or reuse `internal/metrics` if appropriate)

### Target shape (conceptual)

```
cmd/docbuilder/commands
   daemon.go  -> internal/daemon
   preview.go -> internal/preview

internal/daemon
   daemon.go, daemon_loop.go   (composition root)

internal/server/httpserver
   wiring for docs/admin/webhook/prom/livereload

internal/server/handlers
   http handlers and view models (status/admin)

internal/build/queue
   job model + retry + worker pool + build adapter interface

internal/forge/discoveryrunner
   forge discovery orchestration + cache

internal/preview
   local watcher + debounce + rebuild loop (uses internal/build + internal/server/httpserver)
```

## Non-Goals

- Changing the external CLI surface area (flags, subcommands) as part of this refactor.
- Re-architecting the build pipeline stages (see ADR-008).
- Introducing multi-theme support (DocBuilder is Relearn-only today).
- Replacing event sourcing; this refactor only changes ownership and wiring.

## Decision Drivers

- Reduce coupling and prevent “daemon as dumping ground”.
- Improve testability by isolating queue/discovery/http/preview.
- Avoid import cycles without `*Daemon` back-references.
- Keep changes incremental and behavior-preserving.

## Migration Plan

We will implement this in small, reviewable steps to keep risk low.

1. Extract preview mode to `internal/preview` and update the preview command to use it.
2. Extract HTTP server wiring to `internal/server/httpserver`, leaving handlers in `internal/server/handlers`.
3. Extract build queue + job model to `internal/build/queue`.
4. Extract discovery runner + cache.
5. Relocate status DTOs/templates closer to the HTTP handler.
6. Move delta bookkeeping out of daemon.

Each step must:

- Preserve behavior (golden tests/integration tests remain green).
- Reduce daemon package surface area (fewer files and fewer imports from unrelated domains).
- Prefer dependency inversion via small interfaces over passing `*Daemon` around.

### Validation / acceptance criteria

- `internal/daemon` no longer contains preview mode code.
- No package outside `internal/daemon` imports `internal/daemon`.
- Build queue and discovery runner can be unit tested without spinning up HTTP servers.
- `go test ./...` remains green.

## Consequences

### Pros

- Clear ownership boundaries; easier to locate code.
- Smaller, more testable units (preview, queue, discovery, http server).
- Less coupling between “preview” and “daemon” products.
- Reduced risk of accidental import cycles.

### Cons / Risks

- Short-term churn: many moves/renames and updates to imports/tests.
- Some new interfaces/adapters will be needed, which may feel like “extra plumbing”.
- Risk of subtle behavior changes in shutdown ordering and shared state access.

## Open Questions

- Exact package names (`internal/server/httpserver` vs `internal/server/runtime`, `internal/build/queue` vs `internal/daemon/queue`).
- Whether the metrics collector should consolidate with `internal/metrics`.
- Whether live reload belongs with preview, server, or stays a shared component.

## Alternatives Considered

1. **Keep current package and only split files**
   - Rejected: improves readability but does not improve ownership boundaries.

2. **Split by feature but keep everything under `internal/daemon/*` subpackages**
   - Rejected: still treats “daemon” as the umbrella for unrelated concerns.

3. **Large rewrite into a new daemon architecture**
   - Rejected: too risky; we want incremental, behavior-preserving moves.

## Related Documents

- ADR-008: Staged Pipeline Architecture
- ADR-005: Documentation Linting
