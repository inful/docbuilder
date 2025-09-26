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
- [ ] Extract link rewrite logic into `internal/hugo/links.go`
- [ ] Provide function `RewriteInternalLinks(content []byte) []byte`
- [ ] Add unit tests (cases: anchor links, already extensionless, code fence ignoring, image refs)
- [ ] Optional: guard code blocks (avoid rewriting inside fenced blocks)

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
- [ ] Introduce `BuildReport` (counts, duration, staticRendered)
- [ ] Populate in current build path (even pre-pipeline) and attach to job metadata
- [ ] Expose via admin/status endpoint

---
## Phase 2: Structural Decomposition (Generator & Build Pipeline) (★)

### 6. Split `generator.go`
- [ ] Create files: `generator.go`, `config_writer.go`, `modules.go`, `content_copy.go`, `indexes.go`, `links.go`
- [ ] Move related functions without behavior change
- [ ] Add package doc comment summarizing responsibilities

### 7. Content Transformation Pipeline
- [ ] Define `Page` struct (front matter, content, path, metadata)
- [ ] Interface `ContentTransformer` with `Name()` + `Transform(ctx, *Page) error`
- [ ] Implement: `FrontMatterInjector`, `RelativeLinkRewriter`, `EditLinkInjector`
- [ ] Integrate pipeline into `copyContentFiles`
- [ ] Tests: pipeline ordering & idempotency

### 8. Build Pipeline Stages
- [ ] `BuildState` struct (config, repos, paths, doc files, timings, report)
- [ ] Stages: `PrepareOutput`, `CloneRepos`, `DiscoverDocs`, `GenerateScaffold`, `RunHugo`, `PostProcess`
- [ ] Orchestrator `Run(ctx, stages...)`
- [ ] Ensure timings recorded per stage
- [ ] Replace body of `performSiteBuild` with stage runner

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
- [ ] Add timing + success/failure counters per stage
- [ ] Export to metrics collector (existing `MetricsCollector` integration)
- [ ] Include metrics snapshot in admin API

### 12. Structured Errors
- [ ] Define sentinel errors (e.g., `ErrClone`, `ErrDiscovery`, `ErrHugo`)
- [ ] Wrap with `fmt.Errorf("clone repo %s: %w", name, err)`
- [ ] Distinguish transient vs permanent for retry logic (future)

### 13. Context Cancellation Checks
- [ ] Add cancellation checks in long loops: clone, copy content, discovery
- [ ] Add test using context cancellation to ensure early stop

### 14. Timeouts for Forge Operations
- [ ] Wrap forge calls with `context.WithTimeout`
- [ ] Timeout configurable (default 30s)
- [ ] Log slow operations exceeding threshold

### 15. Build Report Enrichment
- [ ] Populate: clonedRepos, failedRepos, docsCount, staticRendered, stageDurations
- [ ] Persist last successful report for admin retrieval

---
## Phase 4: Quality & Extensibility

### 16. Front Matter Typed Struct
- [ ] Replace map usage with struct(s)
- [ ] Use yaml tags for stability
- [ ] Add tests verifying YAML output unchanged (golden)

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
