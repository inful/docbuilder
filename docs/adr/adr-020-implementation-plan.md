---
aliases:
  - /_uid/7b0d5b9e-0bcb-44b5-b1b6-00cf4f01a76f/
categories:
  - architecture-decisions
date: 2026-01-26T00:00:00Z
fingerprint: cd04b3ed916d932e76073a81537d2103d07a1d716b5b975367845ff07d80d834
lastmod: "2026-01-26"
tags:
  - daemon
  - scheduling
  - cron
  - gocron
  - refactor
  - implementation-plan
uid: 7b0d5b9e-0bcb-44b5-b1b6-00cf4f01a76f
---

# ADR-020 Implementation Plan: Replace cron-like daemon scheduling with gocron

**Status**: Draft / Tracking  
**Date**: 2026-01-26  
**Decision Makers**: DocBuilder Core Team

This plan implements the decision in [docs/adr/adr-020-replace-cron-like-scheduling-with-gocron.md](adr-020-replace-cron-like-scheduling-with-gocron.md).

## Working Rules (non-negotiable)

- Do not write code before stating assumptions.
  - If implementation reveals an assumption is wrong, update “Assumptions” and record the decision in “Ambiguities / Decisions Log” before continuing.
- Do not claim correctness you haven’t verified.
  - Any statement like “works”, “fixed”, “correct”, or “done” requires at least `go test ./...` to have been run for the change, and results recorded in this plan.
- Do not handle only the happy path.
  - Any new scheduling behavior must be covered with tests for invalid schedule values, overlapping execution, and shutdown semantics.

## Assumptions (must be stated before coding)

- The daemon uses `daemon.sync.schedule` as the operator-facing schedule for periodic sync/discovery.
- The desired schedule format is standard cron as used in existing configs/docs (5-field minute-resolution), unless we explicitly decide to support seconds.
- Cron timezone semantics must be explicit (UTC vs local time vs configurable location).
- We will remove `@every <duration>` support entirely (no deprecation window).
- `github.com/go-co-op/gocron/v2` is already a dependency and is acceptable as the single scheduling mechanism.
- Daemon scheduling should be “best effort” at runtime, but configuration validation should be strict:
  - Invalid cron expression => startup fails with a validation error.

**Known current-state note (must be resolved during implementation)**

- ✅ Resolved: ticker-based cron approximation has been removed; daemon scheduling is executed via `gocron`.

If any of these assumptions are wrong, document the correction in “Ambiguities / Decisions Log” before proceeding.

## Under What Conditions Does This Work?

- Daemon mode is enabled and the scheduler is started.
- `daemon.sync.schedule` is a valid cron expression according to the chosen cron format.
- A single scheduler instance owns all periodic tasks; the daemon loop does not create tickers for scheduling.

### When This Does NOT Work (by design)

- Configs using `@every <duration>`.
- Configs relying on the current “approximate interval” mapping from cron to `time.Duration`.

## Validation Commands (run after EVERY step)

- Tests: `go test ./...`
- Lint: `golangci-lint run --fix` then `golangci-lint run`
- Docs: `go run ./cmd/docbuilder lint --fix ./docs -y` then `go run ./cmd/docbuilder lint ./docs -y`

Record results (pass/fail + any notable output) in the step notes.

## Ambiguities / Decisions Log

Track any decision not explicitly covered by ADR-020.

- 2026-01-26: Cron format is 5-field (minute-resolution). Seconds are not supported (`gocron.CronJob(..., false)`).
- 2026-01-26: Timezone is the daemon process local time (influenced by `TZ`).
- 2026-01-26: Empty `daemon.sync.schedule` is invalid and fails config validation.
- 2026-01-26: Overlap policy is “no overlap” via `gocron.WithSingletonMode(gocron.LimitModeReschedule)`.
- 2026-01-26: Startup behavior keeps an initial discovery run shortly after start (for forges), plus cron-driven periodic sync. Explicit-repo mode keeps an initial build when no forges are configured.
- 2026-01-26: Periodic “status update / state save” is owned by the daemon scheduler (`daemon-status`, 30s).
- 2026-01-26: Prometheus metrics sync loop is owned by the daemon scheduler (`daemon-prom-sync`, 5s).
- _TBD_: LiveReload ping ticker remains out-of-scope unless explicitly included.

## Work Items (ordered, strict TDD)

### 0) Decide and document schedule semantics (docs + tests)

**Goal**: Make operator expectations explicit before refactoring.

- Decide and record:
  - cron format (default: 5-field minute cron; this matches existing docs/examples and current defaults)
  - timezone semantics (UTC vs local time vs configurable)
  - startup behavior (keep an initial one-shot run shortly after start vs cron-only)
  - overlap policy (skip/queue/allow; default should be “no overlap”)

- Update docs/reference/configuration.md:
  - state accepted cron format
  - state timezone semantics
  - state that `@every <duration>` is not supported
  - align the documented default schedule (docs currently show `*/5 * * * *`; config defaults currently set `0 */4 * * *`).

- Add tests for the chosen cron format (and reject invalid formats).

**Validation**

- `go test ./...`
- `golangci-lint run --fix` then `golangci-lint run`

**Commit**

- `docs(config): define daemon sync cron semantics`

**Status**: Completed

**Progress**

- Updated [docs/reference/configuration.md](../reference/configuration.md) to align default schedule and document cron-only semantics.
- Ran docs lint/fix to regenerate frontmatter fingerprints.

**Validation Results (2026-01-26)**

- Docs:
  - `go run ./cmd/docbuilder lint --fix ./docs -y` (fingerprints updated)
  - `go run ./cmd/docbuilder lint ./docs -y` (pass)
- Tests: `go test ./...` (pass)
- Lint:
  - `golangci-lint run --fix` (0 issues)
  - `golangci-lint run` (0 issues)

---

### 1) Baseline: characterize current behavior (tests only)

**Goal**: Lock in today’s behavior enough to refactor safely.

- Add/extend unit tests that reproduce:
  - `parseDiscoverySchedule()` accepting a few cron strings and `@every`.
  - Daemon loop uses a ticker-based interval (smoke test / structure-level test).

**Notes**

- These are characterization tests for behavior that ADR-020 removes; expect to delete/replace them in later steps.

**Validation**

- `go test ./...`
- `golangci-lint run --fix` and `golangci-lint run`

**Commit**

- `test(daemon): characterize current scheduling behavior`

**Status**: Skipped (superseded)

**Notes (2026-01-26)**

- The legacy behavior (`parseDiscoverySchedule()` + ticker-driven interval approximation) has been removed and replaced with strict cron validation + direct scheduler-based tests.
- Instead of characterizing removed behavior, tests now validate the new cron-only semantics and daemon scheduling wiring.

---

### 2) Inventory all periodic daemon tasks (tests + notes)

**Goal**: Ensure all periodic work is accounted for before consolidation.

- Identify all periodic tasks currently driven by tickers/timers in daemon mode (at least: discovery/sync tick, explicit repo scheduled builds, state persistence, metrics sync).
- Include periodic loops/tickers outside the main loop if they impact architecture and shutdown semantics (e.g., metrics sync, LiveReload ping).
- Add a small structural test (or targeted unit tests) proving where each periodic task is currently triggered.
- Record findings in this plan (short bullet list) to prevent accidental omission.

**Validation**

- `go test ./...`
- `golangci-lint run --fix` and `golangci-lint run`

**Commit**

- `test(daemon): inventory periodic daemon tasks`

**Status**: Completed

**Findings (2026-01-26)**

- Daemon periodic sync/discovery/build tick is scheduled via `gocron` (`daemon-sync`, cron expression `daemon.sync.schedule`).
- Daemon periodic status/state persistence is scheduled via `gocron` (`daemon-status`, 30s).
- Prometheus counter bridge sync is scheduled via `gocron` (`daemon-prom-sync`, 5s).
- Main loop retains a one-shot startup timer (`initialDiscoveryTimer`, 3s) to kick initial forge discovery shortly after start.
- LiveReload SSE uses a per-connection heartbeat ticker (`time.NewTicker(30s)`) inside the HTTP handler; this is connection-scoped and not owned by daemon scheduling.
- Other timers exist outside daemon scheduling (e.g., preview debounce) and are out-of-scope for ADR-020.

**Validation Results (2026-01-26)**

- Tests: `go test ./...` (pass)
- Lint:
  - `golangci-lint run --fix` (0 issues)
  - `golangci-lint run` (0 issues)

---

### 3) Add strict config validation for daemon sync schedule (tests first)

**Goal**: Invalid schedules fail fast.

- Add unit tests in `internal/config` covering:
  - valid cron schedule => config load succeeds
  - invalid cron schedule => config load fails with a validation error containing the schedule value
  - empty schedule behavior (decide: disallow empty, or treat as “disabled”; record decision)

- Implement validation in the config load/validation path.

**Notes**

- Validation should live in the central config validation flow (not in daemon runtime), using `internal/foundation/errors` with category `validation` and structured context (`daemon.sync.schedule`, value).
- Add a daemon-focused validator (e.g., `validateDaemon()` / `validateDaemonSchedule()`) and call it from the existing config validator sequence.

**Validation**

- `go test ./...`
- `golangci-lint run --fix` and `golangci-lint run`

**Commit**

- `feat(config): validate daemon sync schedule as cron`

**Status**: Completed

**Progress**

- Config validation rejects empty schedules and validates cron parsing via `gocron`.
- Tests exist under `internal/config` for valid/invalid schedules.

**Validation Results (2026-01-26)**

- Tests: `go test ./...` (pass)
- Lint:
  - `golangci-lint run --fix` (0 issues)
  - `golangci-lint run` (0 issues)

---

### 4) Extend daemon Scheduler to support cron jobs (tests first)

**Goal**: Centralize periodic scheduling behind `internal/daemon/Scheduler`.

- Add unit tests for a new API (shape may vary) that schedules a cron job successfully.
- Add tests for singleton/overlap behavior (ensure a long-running job does not overlap with itself).
- Implement scheduler cron job support using `gocron`.

**Non-flaky test guidance**

- Prefer tests that trigger task execution directly (e.g., invoking the scheduled task function) instead of waiting for wall-clock cron firing.
- Do not assume gocron provides a stable `RunNow` API; there is no existing in-repo usage of it today.
- Avoid sleeps where possible; coordinate with channels to prove overlap behavior.

**Validation**

- `go test ./...`
- `golangci-lint run --fix` and `golangci-lint run`

**Commit**

- `feat(daemon): add gocron cron scheduling support`

**Status**: Completed

**Progress**

- Added `ScheduleCron(...)` to the daemon scheduler wrapper with singleton mode.
- Added unit tests for valid/invalid cron scheduling.

**Validation Results (2026-01-26)**

- Tests: `go test ./...` (pass)
- Lint:
  - `golangci-lint run --fix` (0 issues)
  - `golangci-lint run` (0 issues)

---

### 5) Wire daemon sync scheduling through Scheduler (tests first)

**Goal**: Replace ticker-driven scheduling with gocron-driven cron scheduling.

- Add/adjust daemon tests that assert:
  - sync/discovery is scheduled via Scheduler (not a `time.Ticker` in the main loop)
  - schedule executes the correct task(s) (discovery runner for forges; scheduled build trigger for explicit repos)

- Implement wiring:
  - Remove `parseDiscoverySchedule()` and `discoveryTicker` from `internal/daemon/daemon_loop.go`.
  - Create a scheduler job for `daemon.sync.schedule`.
  - Ensure shutdown stops scheduler cleanly.

**Notes**

- If we keep “run shortly after start”, implement it as a one-shot timer/job owned by the scheduler (not inside the main loop), or document why it remains outside.

**Validation**

- `go test ./...`
- `golangci-lint run --fix` and `golangci-lint run`

**Commit**

- `refactor(daemon): run sync schedule via gocron`

**Status**: Completed

**Progress**

- Daemon startup schedules `daemon.sync.schedule` via the scheduler.
- Removed `parseDiscoverySchedule()` and ticker-driven sync scheduling from the main loop.
- Added a unit test covering `schedulePeriodicJobs(...)` error cases and happy path.

**Validation Results (2026-01-26)**

- Tests: `go test ./...` (pass)
- Lint:
  - `golangci-lint run --fix` (0 issues)
  - `golangci-lint run` (0 issues)

---

### 6) Consolidate remaining periodic tasks behind Scheduler (tests first)

**Goal**: Complete the architecture cleanup by removing ticker-based periodic work.

- Move any remaining periodic tasks identified in Step 2 (e.g., state persistence, metrics sync) out of the daemon loop tick path and into scheduler-managed jobs.
- Add tests that verify these tasks are scheduled and that shutdown prevents new runs.

**Validation**

- `go test ./...`
- `golangci-lint run --fix` then `golangci-lint run`

**Commit**

- `refactor(daemon): consolidate periodic tasks under scheduler`

**Status**: Completed

**Progress**

- Moved periodic status update + state save out of the daemon loop ticker and into scheduler-managed jobs.
- Replaced Prometheus metrics sync goroutine (`time.Sleep` loop) with a scheduler-managed job.

**Validation Results (2026-01-26)**

- Tests: `go test ./...` (pass)
- Lint:
  - `golangci-lint run --fix` (0 issues)
  - `golangci-lint run` (0 issues)

---

### 7) Remove `@every` support and cron-to-duration approximation (tests first)

**Goal**: Complete the cleanup described in ADR-020.

- Update/replace characterization tests:
  - Remove acceptance of `@every`.
  - Remove acceptance of “approximate interval” cron mappings.

- Delete dead code paths:
  - Remove `parseDiscoverySchedule()`.
  - Remove any remaining duration-based handling for `daemon.sync.schedule`.

**Validation**

- `go test ./...`
- `golangci-lint run --fix` and `golangci-lint run`

**Commit**

- `refactor(daemon): drop @every and cron-to-duration parsing`

**Status**: Completed

**Progress**

- Removed `parseDiscoverySchedule()` and all cron-to-duration approximation from the daemon scheduling path.
- Dropped `@every <duration>` support for `daemon.sync.schedule` (cron-only, strict validation).

**Validation Results (2026-01-26)**

- Tests: `go test ./...` (pass)
- Lint:
  - `golangci-lint run --fix` (0 issues)
  - `golangci-lint run` (0 issues)

---

### 8) End-to-end daemon scheduling verification (integration-style, non-flaky)

**Goal**: Prevent regressions across the daemon runtime lifecycle.

- Add an integration-style test that avoids wall-clock cron waits:
  - Starts daemon components with a scheduler configured.
  - Triggers the scheduled task execution directly (by calling the scheduled task function), rather than waiting for cron.
  - Verifies that the scheduled task enqueues the expected job(s) without overlap.
  - Verifies clean shutdown.

**Validation**

- `go test ./...`
- `golangci-lint run --fix` and `golangci-lint run`

**Commit**

- `test(daemon): add integration coverage for cron scheduling`

**Status**: Completed

**Progress**

- Refactored the scheduled sync tick into a callable method (`runScheduledSyncTick`) so tests can execute it directly (no wall-clock cron waits).
- Added a non-flaky integration-style test that verifies a scheduled tick triggers discovery and enqueues a discovery build job.
- Added a lifecycle smoke test that schedules daemon jobs and verifies scheduler start/stop completes cleanly.

**Validation Results (2026-01-26)**

- Tests: `go test ./...` (pass)
- Lint:
  - `golangci-lint run --fix` (0 issues)
  - `golangci-lint run` (0 issues)

---

## Completion Checklist

- All work items marked Completed with recorded validation output.
- `go test ./...` passes.
- `golangci-lint run --fix` and `golangci-lint run` pass.
- Docs updated and `docbuilder lint` passes for modified markdown.
