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
- [x] Refine stage naming & separation: added `clone_repos` & `discover_docs` as first-class stages (full pipeline supported via `GenerateFullSite`)

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
- [x] Populate: clonedRepos, failedRepos (accurate counts via `clone_repos` stage; default prefill removed)
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
Completed: Phase 1 items 1 (optional subtask deferred) & 2 (filter include/exclude), item 5, Phase 2 items 6, 7 (except golden test), 8 (clone & discovery stages), structured stage errors, domain sentinel errors, context cancellation (partial), stage timings, build report enrichment (rendered pages, outcome, stage counts, staticRendered, precise clone/fail/skip counts), basic build outcome metrics, status endpoint surfaces enriched fields including skipped_repositories.
Pending: V2→legacy conversion helper, transient/permanent error classification, additional cancellation checks, docsCount alias, report persistence endpoint, front matter typing, golden tests, per-stage metrics export (stage histograms + counters), retry logic scaffolding, parallel cloning workers, lint integration, ToLegacy tests, repository attribute-based filtering (.docignore / archived) centralization.
Risk Level: Low – Tests green; filter & skip metrics integrated. Technical debt: missing histograms & retry semantics; config conversion still ad hoc.

### Proposed Next Step
Add per-stage histogram metrics & V2→Legacy conversion helper (`ToLegacy()`), then classify transient errors.

Rationale:
- Histograms (build_duration_seconds, stage_duration_seconds{stage=...}) unlock SLO/alerting and trend analysis; we already collect raw durations.
- Formal `ToLegacy()` removes duplicated config translation logic in build queue and centralizes conversion validation.
- Establishing transient vs permanent classification early prevents ad hoc retry logic later.

Execution Outline:
1. Implement `(*V2Config) ToLegacy() (*config.Config, error)` producing legacy structure; add parity test comparing fields (theme, base URL, params, menu, output dir, repositories length/names).
2. Add metrics recording in `runStages`: record per-stage duration histogram via metrics collector interface (guard if collector nil). Introduce metrics names: `docbuilder_stage_duration_seconds` (labels: stage), `docbuilder_build_duration_seconds`.
3. Emit per-stage success/warning/fatal counters via metrics: `docbuilder_stage_result_total{stage, result}`.
4. Add `StageError` helper `IsTransient()` stub (initially hard-coded mapping: clone errors transient, discovery maybe transient, hugo runtime transient) to be refined later.
5. Tests: (a) invoke a minimal build and assert metrics collector received expected counter increments (use a fake collector); (b) parity test for `ToLegacy()`.
6. Update `refactoring.md` and architecture docs to reference new metrics and conversion helper.

Acceptance Criteria:
- Metrics collector (when provided) receives stage duration observations and counters for each stage executed.
- `ToLegacy()` parity test passes; build queue uses it (single conversion site).
- No regression in existing tests; added tests cover metrics emission and config conversion.

Follow-on After This Step:
- Implement retry scaffolding using transient classification.
- Add parallel cloning worker pool (bounded, metrics for concurrency).
- Persist last successful `BuildReport` endpoint.

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
