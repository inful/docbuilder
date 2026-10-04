---
aliases:
  - /_uid/ce53277a-bda7-4d1e-8fb7-8db4c3afdf5d/
categories:
  - architecture-decisions
date: 2026-10-04T00:00:00Z
fingerprint: 4ca8458453cb01663b574bb5f5e988e5aa514409b3a847e9e65556d7bc1374d9
lastmod: "2026-10-04"
tags:
  - daemon
  - refactor
  - architecture
  - testability
title: 'ADR-023: Decompose the Daemon god class'
uid: ce53277a-bda7-4d1e-8fb7-8db4c3afdf5d
---

# ADR-023: Decompose the Daemon god class

## Status

Accepted. Implemented on branch `refactor/daemon-decompose` in October 2026.

## Context

The 2026-10-01 `build-service-unification` review (follow-ups F1
through F8) closed a real silent regression in the daemon's
build wiring. The Phase 4 refactor of that branch had dropped
`SkipState` and `OnDocumentReady` from `jobToBuildRequest`
because the function lost access to the daemon's state manager
and outbound dispatcher when the previous adapter was deleted.
The wiring was restored by a follow-up commit and guarded by a
regression test.

The root cause that allowed the regression was structural: the
`Daemon` struct in `internal/daemon/daemon.go` carried 34 flat
fields across 5 distinct concern groups, and the 191-line
`NewDaemonWithConfigFile` (cyclomatic complexity 22, 14 returns)
initialised all 11 subsystems in a single procedure. Tests could
not use the constructor (it does real I/O) and had to reach into
the struct's unexported fields, which is why the regression slipped
through review: the test seam was no seal at all.

The project's code-graph analysis also identified the daemon as
the top god-class (34 fields, top of the gini distribution) and
the 191-line constructor as the worst single function in the
codebase.

## Decision

Refactor the `Daemon` struct into a thin façade over six
per-concern sub-structs:

| Sub-struct          | Owns                                                                                |
|---------------------|-------------------------------------------------------------------------------------|
| `lifecycleStack`    | status, run coordination, scheduled-job bookkeeping                                 |
| `stateStack`        | state manager, event store, projection, emitter                                     |
| `buildStack`        | `BuildService`, queue, debouncer, repo updater, mutex                               |
| `discoveryStack`    | forge manager, discovery service, cache, runner                                     |
| `serveStack`        | HTTP server, scheduler, orchestration bus, metrics                                  |
| `optionalStack`     | livereload, link verifier, outbound dispatcher (any may be nil)                     |

The `Daemon` struct itself holds only the configuration reference,
the two shared atomic counters, and pointers to the sub-structs.

The constructor is split into nine per-concern `init*` methods so
the recipe is short enough to read in one screen and each
subsystem can be reviewed in isolation.

The test seam is replaced with a single `daemonForTest(t, opts)`
helper that accepts the seams the test wants to wire up and
defaults the rest to non-nil empty sub-structs. Tests no longer
mutate private fields.

## Consequences

### Positive

- The constructor is now a 35-line recipe. Each line is one
  `init*` call. Reading top-to-bottom is the fastest way to see
  what a fully-initialized daemon contains.
- The struct's shape documents the concern groups. Adding a
  dependency now touches one sub-struct, not the top-level
  surface.
- A new regression-guard test
  (`TestNewDaemonWithConfigFile_FullWiring`) asserts every
  mandatory component is non-nil after construction. This is
  the test that would have caught the build-service-unification
  Phase 4 regression on review.
- The `daemonForTest` helper is a single point of change. Future
  shape changes are reflected there rather than silently
  broken across the test files.
- Code-graph gini and the god-class leaderboard both drop:
  the daemon no longer tops either list.

### Negative / costs

- Every `d.<field>` access in the package becomes
  `d.<group>.<field>`. The mechanical rename touched ~400
  sites across 30 files. The compiler caught every site.
- A handful of production methods are now defensive against
  nil sub-structs (e.g. `jobToBuildRequest` guards `d.state`
  and `d.optional`). The checks are zero-cost in production
  (the constructor always populates every sub-struct) and
  only matter for the test seam.
- One test helper file (`build_service_adapter_test.go`) was
  left with its existing `newDaemonForTest` name; the build
  service adapter was deleted in the previous refactor, so
  the file's purpose is now narrower. The rename can land in
  a follow-up.

## Alternatives considered

- **Sub-package extraction** (one package per concern). Rejected:
  increases churn, breaks the existing `internal/daemon/`
  internal layout, and the cross-cutting concern (e.g. the
  `Build` method that needs all five sub-structs) would still
  need a facade. Sub-structs deliver 80% of the value at 20%
  of the cost.
- **Option types / function builders** for the constructor.
  Rejected: the goal was readability of an existing structure,
  not a new one. Per-concern `init*` methods are simpler and
  match the codebase's existing style.
- **No refactor** (just add the regression test). Rejected:
  the regression test is necessary but not sufficient. The
  next refactor that touches the constructor will face the
  same god-class problem unless the shape changes.

## References

- Branch: `refactor/daemon-decompose`
- Plan: `plan/refactor-daemon-decompose.md`
- Commits: `d63d6cc` (safety net), `fe45526` (init methods),
  `2c4bd11` (sub-structs), `160c503` (test helper),
  `b6db63a` (test migrations pt 1), `cee58b6` (test
  migrations pt 2)
- The build-service-unification review that motivated the
  refactor is the project root `CODE_REVIEW_build-service-unification.md`
  document.
