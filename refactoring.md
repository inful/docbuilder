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
- [ ] Expose per-stage counts & rendered pages in status/metrics endpoints (partial – currently only outcome & timings surfaced)

### 12. Structured Errors
- [x] Stage-level structured errors (`StageError` with kinds fatal|warning|canceled)
- [ ] Define domain sentinel errors (e.g., `ErrClone`, `ErrDiscovery`, `ErrHugo`) for retry semantics
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
- [ ] Populate: clonedRepos, failedRepos (placeholder currently defaults cloned=total, failed=0)
- [ ] Add docsCount alias or clarify semantics with Files
- [ ] Add staticRendered flag (depends on Hugo runner abstraction)
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
Completed: Phase 1 item 1 (optional subtask deferred), items 5 (core), Phase 2 items 6, 7 (except golden test), 8 (initial stage runner), structured stage errors (Phase 3 item 12 partial), context cancellation (Phase 3 item 13 partial), stage timings exposed, build report enrichment (rendered pages, outcome, stage counts), basic build metrics counters (outcome) emitted, status endpoint now shows outcome/summary/errors/warnings.
Pending: Exposing per-stage counts via status, cloned/failed repo counts, staticRendered flag, golden tests, sentinel domain errors, repository filtering object, V2→legacy conversion helper, domain retry semantics, front matter typing, stage duration histograms.
Risk Level: Low – All tests green, structured error layer adds clarity without breaking external APIs.

### Proposed Next Step
Expose per-stage counts & repository clone stats, then introduce sentinel domain errors for retry semantics.

Rationale:
- We now internally capture stage counts; surfacing them (and rendered pages) closes the visibility gap.
- Adding cloned/failed repo metrics aligns operators’ mental model with early pipeline health (before Hugo stages).
- Sentinel errors next allow classification (clone vs discovery vs hugo) enabling targeted retries and clearer status reporting.

Execution Outline:
1. Enhance clone loop to increment cloned/failed repo counters (populate `BuildReport.ClonedRepositories` / `FailedRepositories`).
2. Add `staticRendered` boolean to report (true only if Hugo stage ran & succeeded) and expose in status.
3. Extend status JSON to include: rendered_pages, stage_counts (success/warning/fatal per stage), cloned_repos, failed_repos.
4. Define sentinel errors (`ErrClone`, `ErrDiscovery`, `ErrHugo`) and wrap existing errors at source sites; adjust StageError to carry underlying domain classification (optional field or via error unwrap).
5. Emit metrics: per-stage success/warning/fatal counters & build_duration histogram (reuse MetricsCollector histograms).
6. Add tests: (a) simulated clone failure increments failed counter; (b) warning in run_hugo reflects in StageCounts and status serialization.
7. Update docs (this file + package comment) to reflect new exported observability fields.

Acceptance Criteria:
- Status endpoint shows new fields (counts & clone stats) when a build has occurred.
- Tests green with new assertions for StageCounts exposure and clone failure accounting.
- No regressions in existing report summary string (only additive fields kept at end).

Follow-on After This Step:
- Implement RepositoryFilter (Phase 1 item 2) to prune discovery set early.
- Add golden page test (Phase 4 item 25) with stable front matter before typing refactor.
- Introduce HugoRunner abstraction (Phase 4 item 18) to finalize staticRendered semantics.

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
