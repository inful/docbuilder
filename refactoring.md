# Refactoring Roadmap

Structured plan to improve maintainability, testability, and extensibility. Use the checkboxes to track progress. Phases are ordered to minimize risk (tests/extractions first, structural changes after coverage).

---
## Legend
- [ ] Not started
- [x] Completed
- [-] Skipped / Not Applicable
- (★) High impact / priority

---
## Phase 1: Safety & Test Foundations (High ROI, Low Risk)

### 1. Link Rewriting Isolation (★)
- [x] Extract link rewrite logic into `internal/hugo/links.go`
- [x] Provide function `RewriteRelativeMarkdownLinks(content string) string` (name differs from draft but equivalent)
- [x] Add unit tests (anchors, extension removal, images untouched, relative path variants)
- [ ] Optional: guard code blocks (avoid rewriting inside fenced blocks) (Deferred – regex adequate for now)

### 2. Filtering Logic Object
- [ ] Introduce `RepositoryFilter` struct holding compiled include/exclude patterns
- [ ] Precompile wildcard patterns to regex once
- [ ] Provide `Include(repo *Repository) (bool, reason string)`
- [ ] Unit tests: full include, exclude, archived, .docignore, required_paths disabled

### 3. V2 → Legacy Config Conversion
- [ ] Add `ToLegacy()` method on `*V2Config`
- [ ] Replace ad-hoc conversion in `performSiteBuild`
- [ ] Test: ensure field parity (theme, params, menu, output dir)

### 4. Theme Constants & Date Formats
- [ ] Add constants `ThemeDocsy`, `ThemeHextra`
- [ ] Replace magic strings in generator
- [ ] Add `const TimeFormatRFC3339 = time.RFC3339` (or reuse directly) for consistency

### 5. Basic Build Report Struct
- [x] Introduce `BuildReport` (counts, duration, errors; staticRendered pending)
- [x] Populate in current build path via `GenerateSiteWithReport`
- [x] Attach to job metadata / daemon
- [x] Expose via admin/status endpoint (stage timings now visible; warnings/errors exposure pending)

---
## Phase 2: Structural Decomposition (Generator & Build Pipeline) (★)

### 6. Split `generator.go`
- [x] Create files: `generator.go`, `config_writer.go`, `modules.go`, `content_copy.go`, `indexes.go`, `links.go`, `params.go`, `run_hugo.go`, `structure.go`, `utilities.go`
- [x] Move related functions without behavior change
- [x] Add package doc comment summarizing responsibilities

### 7. Content Transformation Pipeline
- [x] Define `Page` struct (front matter, content, path, metadata)
- [x] Interface `ContentTransformer` with `Name()` + `Transform(*Page) error`
- [x] Implement: `FrontMatterParser`, `FrontMatterBuilder`, `EditLinkInjector`, `RelativeLinkRewriter`, `FinalFrontMatterSerializer`
- [x] Integrate pipeline into content copy (initial integration; preserves original behavior)
- [x] Tests: ordering & idempotency (adjusted expectation for reserialized front matter)
- [ ] Add golden test for representative transformed page (planned with Phase 4 item 25)

### 8. Build Pipeline Stages
- [x] `BuildState` struct (config, repos, paths, doc files, timings, report)
- [x] Stages implemented (current set: prepare_output, generate_config, layouts, copy_content, indexes, run_hugo, post_process)
- [x] Orchestrator with timing & error handling (`runStages`)
- [x] Timings recorded & exposed via `BuildReport.StageDurations` and daemon status
- [x] Integrated into `GenerateSiteWithReport` (daemon uses context-aware variant)
- [ ] Refine stage naming to match original design (CloneRepos/DiscoverDocs separation) (optional)

### 9. Builder Interface
- [ ] Define `type Builder interface { Build(ctx context.Context, job *BuildJob) error }`
- [ ] Implement `SiteBuilder` using pipeline
- [ ] Refactor `BuildQueue.executeBuild` to delegate to injected `Builder`
- [ ] Add constructor wiring in daemon initialization

### 10. Logging Context Standardization
- [ ] Add helper `logger := slog.With("job_id", job.ID)`
- [ ] Pass logger through stages / transformations (or embed in BuildState)
- [ ] Ensure all stage logs include `stage` key

---
## Phase 3: Operability & Observability

### 11. Metrics per Stage
- [x] Add success/failure counters per stage (captured in `BuildReport.StageCounts`)
- [x] Export basic build outcome counters (success/failed/warning/canceled) via metrics collector hook
- [x] Expose per-stage counts & rendered pages in status endpoint (metrics export for stage counts still pending)

### 12. Structured Errors
- [x] Stage-level structured errors (`StageError` with kinds fatal|warning|canceled)
- [x] Define domain sentinel errors (e.g., `ErrClone`, `ErrDiscovery`, `ErrHugo`) for retry semantics (wrapping implemented in clone/discovery/hugo paths)
- [ ] Distinguish transient vs permanent for retry logic (future)

### 13. Context Cancellation Checks
- [x] Context-aware site generation (`GenerateSiteWithReportContext`)
- [x] Cancellation test to ensure early abort
- [ ] Add cancellation checks in long loops: clone, copy content, discovery (partial)

### 14. Timeouts for Forge Operations
- [ ] Wrap forge calls with `context.WithTimeout`
- [ ] Timeout configurable (default 30s)
- [ ] Log slow operations exceeding threshold

### 15. Build Report Enrichment
- [x] Stage durations populated
- [x] Report stored in job metadata & surfaced (stage timings)
- [x] Populate: renderedPages (during copy stage)
- [ ] Populate: clonedRepos, failedRepos (partial – counters incremented, initial default still set; formal stage timing pending)
- [ ] Add docsCount alias or clarify semantics with Files
- [x] Add staticRendered flag (set on successful Hugo execution and exposed via status)
- [ ] Persist last successful report for dedicated admin retrieval endpoint

---
## Phase 4: Quality & Extensibility

### 16. Front Matter Typed Struct
- [ ] Replace map usage with struct(s)
- [ ] Use yaml tags for stability
- [ ] Add tests verifying YAML output unchanged (golden)
- [ ] Consider builder pattern for incremental assembly

### 17. Index Page DRY Refactor
- [ ] Generalize main/repo/section generation with shared builder
- [ ] Potential template-driven approach
- [ ] Golden tests for output

### 18. Hugo Runner Abstraction
- [ ] Interface `HugoRunner` with `Run(ctx, dir string) error`
- [ ] Default binary runner; add `NoOpRunner`
- [ ] Wire selection via config/env

### 19. Repository Parallel Cloning
- [ ] Bounded worker pool (n = configurable)
- [ ] Preserve error collection; partial success continues
- [ ] Metrics: clone duration histogram

### 20. Enhanced Link Rewriter (AST Based)
- [ ] Optional goldmark-based parser to avoid false positives in code blocks
- [ ] Fallback to regex if parser fails

### 21. Search Index Stage (Future Feature)
- [ ] Add stub `PostProcess` stage example
- [ ] Reserve extension point for indexing

---
## Phase 5: Tooling, Docs & Governance

### 22. golangci-lint Integration
- [ ] Add `.golangci.yml` with selected linters (errcheck, staticcheck, revive, gocyclo threshold)
- [ ] Add `make lint` target
- [ ] Fix initial violations or suppress with justification

### 23. Architecture Documentation
- [ ] Add `docs/architecture.md` summarizing pipeline, components, data flow
- [ ] Include diagrams (PlantUML / Mermaid) for build pipeline & daemon

### 24. CLI Reference Automation
- [ ] Generate `docs/cli.md` from Kong help output
- [ ] Add CI step to detect drift

### 25. Golden Tests
- [ ] Create `test/golden/` fixtures (index pages, sample transformed page)
- [ ] Add helper to update golden with `UPDATE_GOLDEN=1`

### 26. Release Hygiene
- [ ] Add semantic version tagging script
- [ ] Auto-create changelog entries from commit messages (future)

---
## Optional / Deferred Ideas
- [ ] Multi-theme simultaneous generation (one content, multiple themes)
- [ ] Pluggable storage backend for repo cache (S3, local FS abstraction)
- [ ] Webhook signature validation & retries
- [ ] Diff-based incremental rebuild (only changed repos)

---
## Cross-Cutting Acceptance Criteria
- Each structural refactor retains identical output (content + hugo.yaml) unless marked as enhancement.
- All new exported APIs documented with GoDoc comments.
- Tests added/updated per phase before merging.
- Logging remains consistent; no loss of existing context keys.

---
## Progress Tracking Notes
Record decisions (e.g., skipping AST parser) inline with a short rationale.

### Recent Decisions
- Code block–aware link rewriting deferred (low immediate risk; revisit with AST in Phase 4 item 20).
- `RewriteRelativeMarkdownLinks` chosen over planned `RewriteInternalLinks` for clarity; doc updated.
- Build report currently excludes `staticRendered` until pipeline/runner abstraction (Phase 3 & 18) clarifies final render success semantics.

### Current Status Summary (2025-09-26)
Completed: Phase 1 item 1 (optional subtask deferred), item 5 (core), Phase 2 items 6, 7 (except golden test), 8 (initial stage runner), structured stage errors, domain sentinel errors (clone/discovery/hugo) integrated, context cancellation (Phase 3 item 13 partial), stage timings exposed, build report enrichment (rendered pages, outcome, stage counts, staticRendered), basic build metrics counters (outcome) emitted, status endpoint shows outcome/summary/errors/warnings/rendered pages/stage counts/static render flag/clone stats (partial clone accuracy).
Pending: Precise cloned/failed repo accounting (initial default override cleanup), RepositoryFilter object, V2→legacy conversion helper, domain retry semantics (transient vs permanent), cancellation checks in long loops, docsCount alias, report persistence endpoint, front matter typing, golden tests, per-stage metrics export & histograms, formal clone/discovery stages for timing, repository filtering, retry logic scaffolding, stage duration histograms.
Risk Level: Low – All tests green; recent additions purely additive. Minor technical debt: clone metrics initialization & lack of formal clone/discovery stages.

### Proposed Next Step
Formalize pre-content stages (CloneRepos, DiscoverDocs) and introduce `RepositoryFilter`.

Rationale:
- Current clone/discovery logic lives inside build queue without timing or standardized error recording; elevating them to first-class stages improves observability and paves the way for retry/backoff and parallel cloning.
- Formal stages let us remove the provisional default of `ClonedRepositories = total` and ensure accurate counters (success vs failure) plus duration metrics.
- RepositoryFilter (include/exclude patterns, archived skip, required paths) reduces unnecessary clone/discovery work and yields cleaner status output.
- Establishing these early stages now avoids rework before we add histogram metrics and retry semantics.

Execution Outline:
1. Add two new stages before `prepare_output`: `clone_repos`, `discover_docs` (implemented within generator or a thin wrapper orchestrator) capturing durations and StageCounts.
2. Move clone loop from `BuildQueue.executeBuild` into `stageCloneRepos`; populate `BuildState` with repo paths and update `BuildReport.ClonedRepositories` / `FailedRepositories` precisely (no default prefill).
3. Implement `RepositoryFilter` (Phase 1 item 2): precompiled patterns, `Include()` method returning (bool, reason). Apply filter prior to cloning to skip repos early and record skipped count (optional new field `SkippedRepositories`).
4. Adjust `BuildReport` initialization: start cloned/failed/skipped counters at 0; remove current default assignment.
5. Update status endpoint to include optional `skipped_repositories` and ensure clone metrics reflect new logic.
6. Add tests: (a) repository clone failure increments failed & outcome possibly still success if others succeed; (b) filter exclusion yields skip count; (c) discover_docs stage cancellation propagates with canceled outcome.
7. Update metrics collector (if available) to increment per-stage counters for clone/discovery and record duration histograms (stub if histogram infra not yet done).
8. Update this roadmap and package docs describing new stages and filtering.

Acceptance Criteria:
- `BuildReport` shows accurate clone/discovery durations and counts (cloned, failed, optional skipped).
- Status JSON includes these counters (and omitted when zero).
- Existing downstream Hugo stages unaffected; all tests pass; new tests cover failures & filtering.
- Removal of default cloned=total behavior (validated in test).

Follow-on After This Step:
- Add per-stage histogram metrics (duration_seconds) & counts export.
- Implement retry semantics for transient clone errors using sentinel domain errors.
- Introduce parallel cloning worker pool (Phase 4 item 19) once baseline metrics stable.

---
## Quick Start Sequence (Recommended)
1. Phase 1 items 1–3 (unlock safe structure changes)
2. Phase 2 item 6 (file split) once tests exist
3. Introduce pipeline (Phase 2 item 8) using existing logic incrementally
4. Bolt on metrics & reports (Phase 3) after stabilization

---
## Changelog Mapping
Maintain a short mapping when closing items to feed into release notes (future script can parse `[refactor]` conventional commits).

---

(End of roadmap)
