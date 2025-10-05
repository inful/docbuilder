# ADR-000: Uniform Error Handling Across DocBuilder

Date: 2025-10-03

## Status

Proposed

## Context

DocBuilder currently mixes error patterns: direct `fmt.Errorf`/`errors.New`, partial use of `foundation.Result`, and ad-hoc wrapping. This causes inconsistent user messages, logging, and exit/HTTP codes.

## Decision

Adopt a single error model based on `internal/errors` with:

- Category, severity, code, operation (op), cause, context fields, retry-eligible flag
- Helper constructors/wrappers: `New`, `Wrap`, and option setters: `WithCode`, `WithOp`, `WithField`, `WithRetryable`, `WithHTTPStatus`, `WithExitCode`
- Boundary adapters for CLI/HTTP and standardized logging fields
- CI guard to prevent raw error creation in non-test code

## Taxonomy

- Categories: Config, Auth, Git, Docs, Hugo, Pipeline, State, Daemon, Network, IO, Validation, NotFound, Conflict, Timeout, Canceled, RateLimit, Unknown
- Severity: Info, Warning, Error, Fatal
- Codes (examples): `ConfigNotFound`, `ConfigInvalidYAML`, `ConfigValidationFailed`, `GitAuthFailed`, `GitFetchFailed`, `DocsWalkFailed`, `FrontMatterInvalid`, `PlanCircularDependency`, `StateReadFailed`, `StateWriteFailed`, `ScheduleInvalid`

## Layer Behavior

- Libraries (config/git/docs/Hugo/pipeline/state): return typed errors, include `op`, `code`, retry-eligible, attach context fields; no logging
- Services: aggregate/wrap with higher-level `op` and identifiers (repo/path/url)
- CLI: map errors → exit codes with `ExitCodeFor(err)` and format user-facing messages with `FormatForUser(err)`
- HTTP: map errors → HTTP status with `HTTPStatusFor(err)`, return JSON `{ error: { code, category, message, correlationId }, details? }`
- Logging (boundary only): structured fields from error (category, code, op, retry-eligible, identifiers), level from severity

## Mapping Rules

- CLI exit codes: 0 OK; 2 Validation/Config; 10 Auth; 11 Git; 12 Docs; 13 Hugo; 20 Network (retry-eligible); 1 default
- HTTP status: 400 Validation; 401/403 Auth; 404 NotFound; 409 Conflict; 429 RateLimit; 504 Timeout; 500 Unknown/Internal; 503 service unavailable

## Migration Plan

1. Harden `internal/errors` API (options/extractors for Code/Op/`Retryable`/HTTP/Exit)
2. Add adapters:
   - `internal/cli/error_adapter.go` (ExitCodeFor, FormatForUser)
   - `internal/daemon/http_error_adapter.go` (HTTPStatusFor, JSON response builder)
   - `internal/logx/log_err.go` (structured logging helper)
3. Refactor batch 1: `internal/config`, `internal/git`, `internal/docs`, `internal/hugo`, `internal/pipeline`
4. Refactor batch 2: `internal/daemon`, `internal/state`
5. Align `foundation.Result[T]` usages to carry typed `internal/errors` errors
6. Tests: mapping tests, adapter tests, update assertions to check category/code
7. CI enforcement: script to fail on raw `fmt.Errorf`/`errors.New` outside tests and `internal/errors`
8. Documentation: this ADR + CONTRIBUTING note

## Edge Cases

- context.Canceled → Category Canceled, Info, not retry-eligible; HTTP 499/408; CLI non-zero depending on command
- context.DeadlineExceeded → Timeout, retry-eligible; HTTP 504; CLI 20
- Multi-errors: wrap `errors.Join` once, keep causes for Is/As
- Preserve `errors.Is/As` by retaining root cause

## Examples

Before: `return fmt.Errorf("failed to read config file: %w", err)`

After: `return errors.Wrap(err, errors.CategoryConfig, errors.SeverityError, "read config file", errors.WithOp("config.Load"), errors.WithCode(errors.ConfigReadFailed))`

CLI: `os.Exit(clierrors.ExitCodeFor(err))`

HTTP: `status := httperrors.HTTPStatusFor(err)` and return JSON problem response

## Consequences

- Pros: consistent UX/telemetry, easier support, better retry and policy decisions
- Cons: initial refactor effort, small learning curve

## Rollout

- Day 1: error API + adapters + tests
- Day 2–3: batch 1 refactor + tests
- Day 4–5: batch 2 refactor + tests
- Enable CI guard; iterate on any stragglers

## Progress Checklist

- [ ] Finalize `internal/errors` API (constructors, options, extractors)
- [ ] Implement CLI adapter (`internal/cli/error_adapter.go`) and wire in `cmd/docbuilder/main.go`
- [ ] Implement HTTP adapter (`internal/daemon/http_error_adapter.go`) and update handlers
- [ ] Add logging helper (`internal/logx/log_err.go`) and apply at boundaries
- [ ] Refactor batch 1: config, git, docs, Hugo, pipeline
- [ ] Refactor batch 2: daemon, state
- [ ] Update tests to assert category/code and add mapping tests
- [ ] Add CI guard for raw error creation
- [ ] Update CONTRIBUTING with guidelines and link to this ADR
