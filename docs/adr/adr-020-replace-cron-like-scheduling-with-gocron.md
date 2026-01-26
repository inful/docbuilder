---
aliases:
  - /_uid/2ea73a9b-2ab9-49db-b879-4fabb1f54a8e/
categories:
  - architecture-decisions
date: 2026-01-26T00:00:00Z
fingerprint: a706710b48100ba30bb22c693754cfe7f4c5c4f7b7ce799573f143b239754afe
lastmod: "2026-01-26"
tags:
  - daemon
  - scheduling
  - cron
  - gocron
  - refactor
uid: 2ea73a9b-2ab9-49db-b879-4fabb1f54a8e
---

# ADR-020: Replace cron-like daemon scheduling with gocron

**Status**: Proposed  
**Date**: 2026-01-26  
**Decision Makers**: DocBuilder Core Team

## Context and Problem Statement

DocBuilder daemon mode performs periodic work:

- Forge discovery (discover repos, enqueue builds)
- Scheduled builds for explicitly configured repositories
- Periodic state persistence and metrics sync

Today, daemon scheduling is split across two approaches:

1. A **hand-rolled “cron-like” loop** in `internal/daemon/daemon_loop.go` that:
   - Parses `daemon.sync.schedule` using `parseDiscoverySchedule()` into an *approximate* `time.Duration`
   - Runs a `time.Ticker` at that interval
   - Supports only a small subset of cron patterns

2. A `gocron`-backed scheduler wrapper in `internal/daemon/scheduler.go`, currently used for duration-based periodic build jobs.

This creates ongoing maintenance and correctness costs:

- **Cron semantics are not cron semantics**: mapping a cron expression to a single fixed interval loses “calendar” meaning (e.g., “0 0 * * *” is not “every 24h” in local time around DST shifts).
- **Partial syntax support**: only a handful of expressions work; others fall back silently.
- **Two scheduling systems**: increased complexity, harder testing, inconsistent observability.

We do not want to maintain a homegrown cron parser/executor when a well-maintained library (`github.com/go-co-op/gocron/v2`) is already in use.

## Goals

- Remove the bespoke cron-like scheduling logic from daemon mode.
- Use `gocron` for all periodic daemon tasks (cron expressions and interval/duration jobs).
- Make schedule validation explicit and deterministic (invalid schedules should be surfaced clearly).
- Preserve operator UX: a single `daemon.sync.schedule` field that controls periodic sync.

## Non-Goals

- Introduce a full “job registry” UI or persistent scheduler state.
- Implement distributed scheduling / leader election (daemon instances are independent).
- Change the build queue semantics or retry policy.

## Decision

We will excise the custom cron-like scheduling code in the daemon loop and standardize on `gocron`:

- Replace `parseDiscoverySchedule()` + `discoveryTicker` with a `gocron` job.
- Interpret `daemon.sync.schedule` as a cron expression executed by `gocron`.
- Do not support `@every <duration>` in configuration.
- Consolidate periodic daemon tasks (discovery tick, explicit repo scheduled builds, state save, metrics sync) behind the existing `internal/daemon/Scheduler` wrapper.
- Remove ticker-driven scheduling from `daemon_loop.go` entirely; the daemon loop should not be responsible for “time math”.

### Schedule semantics

- `daemon.sync.schedule` is **cron-only** and is scheduled with `gocron.CronJob(...)`.

If the schedule cannot be parsed by `gocron`, daemon startup should fail fast with a configuration validation error. There is no fallback and no “approximate interval” behavior.

## Decision Drivers

- **Correctness**: real cron execution semantics (calendar-aware), rather than “approximate intervals”.
- **Maintainability**: delete in-house parsing logic and reduce the number of scheduling mechanisms.
- **Consistency**: a single scheduler abstraction and unified logging/metrics for all periodic tasks.
- **Operator safety**: invalid schedule values should be actionable errors, not silent fallbacks.

## Design Outline (High Level)

1. Extend `internal/daemon/scheduler.go` to support cron-based jobs:
   - `ScheduleCron(name, expression, task)` (shape TBD)
   - Optional singleton/overlap policy (prevent concurrent runs of the same job).

2. Use a single “daemon scheduler” instance for all periodic work:
   - Discovery / sync tick
   - Explicit repo scheduled builds
   - Periodic state save
   - Periodic metrics sync

   This keeps “what runs periodically” out of the daemon loop and in a single place.

3. Replace daemon loop tick scheduling:
   - `daemon_loop.go` should no longer parse/approximate cron.
   - The main loop remains responsible for status updates and stop/shutdown sequencing, but not for time-based scheduling.

4. Configuration validation:
   - Validate `daemon.sync.schedule` during config load/validation.
   - Emit a structured error (category `validation`) with the invalid schedule string.

## Migration Plan

1. Implement cron scheduling via `gocron` for `daemon.sync.schedule`.
2. Remove `parseDiscoverySchedule()` and `discoveryTicker` from the daemon loop.
3. Update documentation:
   - Clearly define accepted cron format.
   - Explicitly document that `@every <duration>` is not supported.
4. Add tests:
   - Unit tests for schedule validation behavior (valid/invalid cron).
   - A daemon-mode test that ensures scheduling is wired through `Scheduler` (no ticker-based scheduling code paths).

## Consequences

### Pros

- Deletes custom cron parsing logic.
- Full cron semantics via a maintained library.
- More predictable behavior around timezone/DST.
- Easier to instrument and reason about one scheduler.

### Cons / Tradeoffs

- Schedule parsing becomes stricter; configurations that previously “worked by fallback” may fail fast.
- Cron format expectations must be documented precisely (5-field vs 6-field, seconds support, timezone).
- Users that relied on `@every <duration>` must migrate to cron expressions.

## Acceptance Criteria

- No cron parsing or cron-to-duration approximation exists in the daemon loop.
- `daemon.sync.schedule` is executed via `gocron`.
- Invalid schedules cause a clear validation error.
- Periodic discovery/build behavior matches configured schedule and does not run concurrently unless explicitly allowed.

## Alternatives Considered

1. **Keep the custom schedule parser + ticker**
   - Rejected: ongoing maintenance, partial semantics, and correctness issues.

2. **Use `robfig/cron` directly**
   - Not chosen: we already depend on `gocron`, which provides a higher-level scheduler API.

3. **External scheduling (system cron, systemd timers, Kubernetes CronJobs)**
   - Not chosen: daemon mode is designed to be a self-contained long-running service.

## Related Documents

- `internal/daemon/daemon_loop.go`
- `internal/daemon/scheduler.go`
- `docs/explanation/architecture.md`
- `docs/reference/configuration.md`
- ADR-017: Split daemon responsibilities (package boundaries)
