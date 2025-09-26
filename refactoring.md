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
- [x] Introduce `RepositoryFilter` struct holding compiled include/exclude patterns
- [x] Precompile wildcard patterns to regex once
- [x] Provide `Include(repo *Repository) (bool, reason string)` (method signature adapted to value param)
- [x] Unit tests: include-all default, exclude pattern, precedence (archived/.docignore/required_paths filtering deferred – handled elsewhere)

### 3. Legacy Config Removal (Completed)
- [x] Remove legacy v1 config structs & loader
- [x] Introduce unified `Config` (formerly `V2Config`) with backward-compatible aliases
- [x] Update generator/daemon/CLI/tests to use unified config
- [-] `ToLegacy()` no longer required (strategy changed: v1 removed instead of dual-conversion)

### 4. Theme Constants & Date Formats
- [ ] Add constants `ThemeDocsy`, `ThemeHextra`
- [ ] Replace magic strings in generator (partial: still string literals)
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
- [x] Refine stage naming & separation: added `clone_repos` & `discover_docs` as first-class stages (full pipeline supported via `GenerateFullSite`)

### 9. Builder Interface
- [x] Define `type Builder interface { Build(ctx context.Context, job *BuildJob) (*hugo.BuildReport, error) }` (2025-09-27)
- [x] Implement `SiteBuilder` using pipeline (2025-09-27)
- [x] Refactor `BuildQueue.executeBuild` to delegate to injected `Builder` (2025-09-27)
- [x] Added initial retry scaffold (linear backoff, transient detection) (2025-09-27)
- [ ] Expose configurable retries in config (max_retries, backoff strategy)
- [ ] Emit retry metrics (`build_retries_total`, `build_retry_exhausted_total`)
- [ ] Tests for retry behavior (transient vs permanent, exhaustion)

### 10. Logging Context Standardization
- [ ] Add helper `logger := slog.With("job_id", job.ID)`
- [ ] Pass logger through stages / transformations (or embed in BuildState)
- [ ] Ensure all stage logs include `stage` key

---
## Phase 3: Operability & Observability

### 11. Metrics per Stage
- [x] Add success/failure counters per stage (captured in `BuildReport.StageCounts`)
- [x] Export basic build outcome counters (success/failed/warning/canceled) via metrics collector hook (placeholder; full exporter TBD)
- [x] Expose per-stage counts & rendered pages in status endpoint
- [x] Introduce `metrics.Recorder` interface + integration (stage duration + result emission) (2025-09-27)
 - [x] Add Prometheus histograms: total build duration & per-stage duration (2025-09-27)
 - [x] Implement Prometheus exporter (labeled counters + histograms) & HTTP exposure (build tag `prometheus`) (2025-09-27)
 - [x] Bridge daemon in‑memory counters to Prometheus (`daemon_builds_total`, `daemon_builds_failed_total`) (2025-09-27)
 - [x] Add runtime gauges: `daemon_active_jobs`, `daemon_queue_length` (2025-09-27)
 - [x] Add snapshot gauges: `daemon_last_build_rendered_pages`, `daemon_last_build_repositories` (2025-09-27)
 - [ ] Expose transient/permanent failure counters (`stage_failures_total{transient=}`) (planned)
 - [ ] Queue wait time histogram (`build_queue_wait_seconds`) (planned)

### 12. Structured Errors
- [x] Stage-level structured errors (`StageError` with kinds fatal|warning|canceled)
- [x] Domain sentinel errors (`ErrClone`, `ErrDiscovery`, `ErrHugo`)
 - [x] Transient vs permanent classification helper (`StageError.Transient()`) + tests (2025-09-27)
 - [ ] Emit transient/permanent labeled metrics (see Phase 3 item 11 planned counters)

### 13. Context Cancellation Checks
- [x] Context-aware site generation (`GenerateSiteWithReportContext`)
- [x] Cancellation test ensures early abort
- [ ] Add cancellation checks in long loops: clone (partial), copy content, discovery

### 14. Timeouts for Forge Operations
- [ ] Wrap forge calls with `context.WithTimeout`
- [ ] Timeout configurable (default 30s)
- [ ] Log slow operations exceeding threshold

### 15. Build Report Enrichment
- [x] Stage durations populated
- [x] Report stored in job metadata & surfaced (stage timings)
- [x] renderedPages incremented via instrumentation hook
- [x] cloned/failed/skipped repository counts accurate
- [ ] Add `DocsCount` field (alias to Files) for clarity
- [x] `StaticRendered` flag recorded and exposed
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
- [ ] Metrics: clone duration histogram & concurrency gauge

### 20. Enhanced Link Rewriter (AST Based)
- [ ] Optional goldmark-based parser to avoid false positives in code blocks
- [ ] Fallback to regex if parser fails

### 21. Search Index Stage (Future Feature)
- [ ] Extend `post_process` stage or add new `index_search` stage
- [ ] Reserve extension point for indexing backend adapters

---
## Phase 5: Tooling, Docs & Governance

### 22. golangci-lint Integration
- [ ] Add `.golangci.yml` (errcheck, staticcheck, revive, gocyclo threshold)
- [ ] Add `make lint` target
- [ ] Fix violations / justify suppressions
- [ ] Add CI job for lint

### 23. Architecture Documentation
- [ ] Add `docs/architecture.md` summarizing pipeline, components, data flow
- [ ] Include diagrams (PlantUML / Mermaid) for build pipeline & daemon

### 24. CLI Reference Automation
- [ ] Generate `docs/cli.md` from Kong help output
- [ ] Add CI step to detect drift
- [ ] Optionally embed version/commit info (`docbuilder version` command)

### 25. Golden Tests
- [ ] Create `test/golden/` fixtures (index pages, sample transformed page)
- [ ] Add helper to update golden with `UPDATE_GOLDEN=1`

### 26. Release Hygiene
- [ ] Add semantic version tagging script
- [ ] Auto-create changelog entries from commit messages (future)

---
## Optional / Deferred Ideas
- [-] Multi-theme simultaneous generation (one content, multiple themes)
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

### Current Status Summary (2025-09-27)
Completed: Phase 1 items 1,2,5; Phase 2 items 6,7 (golden test pending), 8, 9 (core implementation); unified config & legacy removal; structured errors & sentinel domains; transient classification with tests; stage timings & counts; build report enrichment (rendered pages, clone/fail/skip, outcome, staticRendered); status endpoint enrichment; metrics recorder abstraction; Prometheus exporter (histograms, counters), daemon counter bridge, runtime & snapshot gauges; initial retry scaffold.
Pending: Theme constants, logging context standardization, transient/permanent labeled metrics, additional cancellation points, DocsCount alias, report persistence, front matter typing, golden fixtures, configurable retry policy & metrics, parallel cloning, queue wait histogram, lint + CI, search indexing extension, filtering enhancements (.docignore / archived centralization), version command, CLI reference automation.
Risk Level: Low – Observability foundation in place; next structural abstractions isolated.

### Proposed Next Step (Updated 2025-09-27 After Builder Integration)
Implement Repository Parallel Cloning (Phase 4 item 19) with instrumentation.

Why This Order:
- Now that the Builder abstraction isolates pipeline execution, adding concurrency to the clone step won’t tangle with queue concerns.
- Parallel cloning provides immediate wall-clock improvement for multi-repo builds.
- Instrumentation (clone duration histogram, concurrency gauge) leverages existing metrics foundation.

Scope:
1. Introduce a parallel clone stage variant: detect `clone_repos` stage and dispatch repository clone tasks to a worker pool (configurable `ConcurrentClones`, default 4 or min(#repos,4)).
2. Maintain ordered results; collect errors per repo (aggregate into StageError if any fail). Partial success allowed.
3. Add metrics:
   - Histogram `docbuilder_clone_repo_duration_seconds` (per repo) with labels: `result` (success|failed).
   - Gauge `docbuilder_clone_concurrency` (current workers active) via GaugeFunc.
4. Integrate transient handling: network/auth failures remain transient for retry policy (already covered by StageError.Transient()).
5. Config: add `build.concurrent_clones` (int) under existing config (fallback to 1 = current behavior if not set).
6. Tests:
   - Unit test with a mock Git client simulating varied latency & one failure.
   - Ensure StageDurations for `clone_repos` still recorded (duration of full parallel stage, not sum of repos).
   - Verify metrics emission using a test recorder.
7. Update documentation (README + roadmap) and mark roadmap item 19 partially / fully done.

Acceptance Criteria:
- All existing tests pass; new clone concurrency tests added.
- No regression in build output (content & hugo.yaml identical for sequential vs parallel).
- Metrics for clone stage & concurrency visible when Prometheus enabled.
- Configuration gracefully falls back when `concurrent_clones` omitted or set to 1.

Follow-On After This Step:
- Add retry metrics & configurable retry policy.
- Implement queue wait time histogram and stage failure transient labeling metrics.
- Persist last successful BuildReport to disk for post-restart visibility.

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
