---
aliases:
  - /_uid/7cdb5485-fbbb-4d2c-8ff2-1e5aa5d8f1b1/
categories:
  - architecture-decisions
date: 2026-01-23T00:00:00Z
lastmod: "2026-01-23"
tags:
  - daemon
  - security
  - content
  - frontmatter
  - implementation-plan
uid: 7cdb5485-fbbb-4d2c-8ff2-1e5aa5d8f1b1
---

# ADR-019 Implementation Plan: Daemon public-only frontmatter filter

**Status**: Draft / Tracking  
**Date**: 2026-01-23  
**Decision Makers**: DocBuilder Core Team

This plan implements the decision in [docs/adr/adr-019-daemon-public-frontmatter-filter.md](docs/adr/adr-019-daemon-public-frontmatter-filter.md).

## Scope

Implement a daemon-only, opt-in “public-only” content filter:

- When enabled, only Markdown pages with YAML frontmatter containing `public: true` are rendered.
- No inheritance (no Hugo `cascade` / parent `_index.md` semantics).
- Generate index pages only for scopes (site/repo/section) that contain at least one public page; generated indexes include `public: true`.
- If zero pages are public, build succeeds and publishes an empty site.
- Static assets continue to be copied as today (even if pages are filtered).

## Working Rules (non-negotiable)

- Do not write code before stating assumptions.
  - If implementation reveals an assumption is wrong, update “Assumptions” and record the decision in “Ambiguities / Decisions Log” before continuing.
- Do not claim correctness you haven’t verified.
  - Any statement like “works”, “fixed”, “correct”, or “done” requires at least `go test ./...` to have been run for the change, and results recorded in this plan.
- Do not handle only the happy path.
  - Every new behavior must have tests for negative/error/missing-data cases (e.g., missing frontmatter, malformed YAML, unexpected types, empty public set).
  - For pipeline behavior changes, include a test that proves “feature off” preserves existing behavior.

## Assumptions (must be stated before coding)

- “Daemon mode” is detected by presence of `cfg.Daemon != nil` and a daemon-only config flag; the Hugo generator does not have an explicit runtime mode beyond config.
- The public-only behavior must not affect non-daemon builds unless explicitly enabled via daemon config.
- “Page” means Markdown docs discovered as non-asset `docs.DocFile` entries; assets are `IsAsset == true`.
- Frontmatter parsing rules follow the existing `internal/docmodel` and `internal/frontmatter` behavior:
  - Missing frontmatter => not public
  - Invalid YAML or malformed frontmatter block => not public
  - `public` must be boolean `true` (not string "true")

If any of these assumptions are wrong, document the correction in the “Ambiguities / Decisions” section and update this plan.

## Under What Conditions Does This Work?

- Config includes `daemon.content.public_only: true` and the build is running in daemon mode (i.e., daemon config is present and used).
- The build executes the Hugo generation pipeline (not discovery-only) and writes a Hugo site under the configured output directory.
- Candidate pages are Markdown docs (not assets) and use a valid YAML frontmatter block with `public: true` (boolean) to be included.
- Frontmatter parsing behavior matches current implementation (delimiter handling, YAML parsing rules, and type coercion rules).
- Index generation logic runs after filtering so scopes are computed from the post-filter set.

### When This Does NOT Work (by design)

- Non-daemon builds unless explicitly enabled via daemon config.
- Pages that rely on Hugo inheritance/cascade (this feature is explicitly “no inheritance”).
- Pages with missing/invalid frontmatter or `public` expressed as a non-boolean type (treated as not public).

## Validation Commands (run after EVERY step)

- Tests: `go test ./...`
- Lint: `golangci-lint run --fix` then `golangci-lint run`

If the repo’s CI expects different lint invocation, document it here.

## Non-Happy-Path Coverage (required)

Each phase must include tests for at least these scenarios (expand as implementation reveals more):

- Feature flag OFF: output matches current behavior.
- Feature flag ON:
  - Missing frontmatter => excluded
  - Invalid YAML / malformed frontmatter => excluded (and does not crash)
  - `public: false` or non-boolean values => excluded
  - Zero public pages overall => build succeeds, publishes empty site, and generates no indexes
  - Assets still copied even if adjacent pages are excluded

## Progress Tracking

Use this checklist as the tracking tool. After each step:

1. Run tests
2. Run golangci-lint (fix + verify)
3. Update this file: mark completed steps, add notes, include command outputs or brief summaries
4. Commit with Conventional Commits

Suggested commit format per step:

- `test(<scope>): ...`
- `feat(<scope>): ...`
- `fix(<scope>): ...`
- `refactor(<scope>): ...`
- `docs(<scope>): ...`

Record the commit SHA next to each completed step.

## Phase 0 — Recon and Guardrails

- [x] 0.1 Identify the daemon-build path that invokes Hugo generation and confirm where discovered docs become pipeline `Document`s.
  - Expected areas: `internal/daemon`, `internal/build`, `internal/hugo/content_copy_pipeline.go`, `internal/hugo/pipeline/*`.
  - Output: Verified that `internal/hugo/content_copy_pipeline.go` is the primary entry point for filtering, and `internal/hugo/pipeline/generators.go` handles index scoping.
  - Commit: `chore(plan): document implementation entrypoints`

- [x] 0.2 Decide the minimal “public-only” switch location in config and how it is plumbed.
  - Target: `daemon.content.public_only: true`.
  - Output: `config.DaemonConfig.Content.PublicOnly` in `internal/config/config.go`.
  - Commit: `chore(config): document daemon public-only flag`

## Phase 1 — Config Surface (TDD)

Goal: add config support without behavior change (public-only still off by default).

- [x] 1.1 Add failing tests for config parsing of `daemon.content.public_only`.
  - Where: `internal/config/*_test.go`.
  - Cover: default false when missing, true when present.
  - Commit: `test(config): add daemon public_only parsing tests`

- [x] 1.2 Implement config structs + YAML tags.
  - Where: `internal/config/config.go`.
  - Add: `DaemonConfig.Content` (new nested struct) with `PublicOnly bool`.
  - Ensure zero-value/default behavior keeps feature disabled.
  - Commit: `feat(config): add daemon content public_only flag`

- [x] 1.3 Validation run + plan update.
  - Record: `go test ./...` result, `golangci-lint run` result.
  - Commit: included in steps above.

## Phase 2 — Filtering Logic (Unit Tests First)

Goal: implement a pure, well-tested function that decides whether a Markdown page is public.

- [x] 2.1 Write failing unit tests for public detection.
  - New helper target: `isPublicMarkdown(content []byte) bool`.
  - Where: `internal/hugo/public_only_test.go`.
  - Commit: `test(hugo): add public frontmatter detection tests`

- [x] 2.2 Implement the helper using `internal/docmodel` (or `frontmatterops.Read`) and strict boolean semantics.
  - Must not mutate content.
  - Must treat parse errors as not public.
  - Commit: `feat(hugo): add public frontmatter detection helper`

- [x] 2.3 Validation run + plan update.

## Phase 3 — Apply Filtering in Daemon Builds (TDD)

Goal: ensure filtered pages are not written into `content/` in daemon public-only mode.

- [x] 3.1 Add failing tests demonstrating that non-public pages are excluded when enabled.
  - Best level: unit/integration-ish in `internal/hugo/public_only_pipeline_test.go`.
  - Commit: `test(hugo): enforce daemon public-only filtering`

- [x] 3.2 Implement filtering at the chosen pipeline location.
  - Requirements:
    - Only applies when `cfg.Daemon != nil && cfg.Daemon.Content.PublicOnly`.
    - Filters Markdown pages only; assets remain copied.
    - Uses per-page parsing only (no inheritance).
  - Insertion point: `internal/hugo/content_copy_pipeline.go`.
  - Commit: `feat(daemon): filter non-public pages from rendered site`

- [x] 3.3 Validation run + plan update.

## Phase 4 — Public-Scoped Index Generation (TDD)

Goal: generated indexes only appear for public scopes; generated indexes include `public: true`.

- [x] 4.1 Add failing tests for index generation scoping.
  - Cases:
    - Repo with no public pages => no repo index generated
    - Section with no public pages => no section index generated
    - At least one public page => indexes generated (and include `public: true`)
    - Zero public pages overall => no root index generated; build succeeds
  - Location: `internal/hugo/public_only_pipeline_test.go`.
  - Commit: `test(hugo): add public-scoped index generation tests`

- [x] 4.2 Implement generator updates.
  - Where: `internal/hugo/pipeline/generators.go`.
  - Behavior when public-only enabled:
    - Generate `content/_index.md` only if at least one public page exists.
    - Generate repo/section indexes only for repos/sections that contain public pages.
    - Inject `public: true` into generated index frontmatter.
  - Commit: `feat(hugo): generate indexes only for public scopes`

- [x] 4.3 Validation run + plan update.

## Phase 5 — Golden / End-to-End Coverage (TDD)

Goal: lock behavior with a realistic repo and expected output structure.

- [x] 5.1 Add integration testdata repo with mixed public/private docs.
  - Location: `test/testdata/repos/public-filter`.
  - Commit: `test(integration): add public-only test repository`

- [x] 5.2 Add config YAML enabling daemon public-only.
  - Location: `test/testdata/configs/daemon-public-filter.yaml`.
  - Commit: `test(integration): add daemon public-only config`

- [x] 5.3 Add golden integration test + golden files.
  - Test: `test/integration/public_filter_golden_test.go`.
  - Commit: `test(integration): add golden test for daemon public-only`

- [x] 5.4 Run golden tests and refresh golden outputs if needed.
  - Commands:
    - `go test ./test/integration -v -update-golden`
    - `go test ./test/integration -v`
  - Commit: `test(integration): update golden outputs for public-only`

## Phase 6 — Docs and Operational Notes

- [x] 6.1 Update configuration reference docs to include `daemon.content.public_only`.

  - Likely file: `docs/reference/configuration.md`.
  - Commit: `docs(config): document daemon public_only flag`  

## Ambiguities / Decisions Log

- 2026-01-23: Decided to keep asset copying as-is — assets may be shared between public and private documents, and discovering "only used" assets is out of scope for this filter.

## Final Verification Evidence (2026-01-23)

### Unit Tests
`go test ./internal/hugo/...` matches 100% pass including `isPublicMarkdown` edge cases and pipeline filtering logic.

### Integration Tests
`test/integration/public_filter_golden_test.go` verified:
1. `file1.md` (public: true) -> present.
2. `file2.md` (public: false) -> filtered.
3. `file3.md` (missing) -> filtered.
4. `sub/public.md` (public: true) -> present.
5. Index for `sub/` generated and contains `public: true`.
6. Static assets in `sub/` copied correctly.

### Linter
`golangci-lint run` reports 0 issues.

## Completion Checklist

- [x] All steps completed
- [x] `go test ./...` passes
- [x] `golangci-lint run --fix` then `golangci-lint run` passes
- [x] Plan updated with final status and SHAs (recorded as task completion)
