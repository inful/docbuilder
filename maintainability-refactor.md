# Maintainability & Refactor Roadmap

A structured, actionable checklist to improve readability, reduce cognitive load, and enhance long‑term maintainability. Organized by phases so work can be delivered incrementally with low regression risk.

## Legend

- [ ] Not started
- [~] In progress
- [x] Complete
- [Δ] Follow-up / optional enhancement




## Phase 0: Baseline & Tracking

- [x] Capture current strategy document (this file)
- [ ] Add architecture diagram (current vs target) (`docs/architecture/diagram.png`)
- [ ] Add CONTRIBUTING section detailing refactor conventions




## Phase 1: Low-Risk Extraction

- [x] Extract theme logic into dedicated theme packages (`internal/hugo/theme` API + `internal/hugo/themes/{hextra,docsy}` implementers; legacy param helpers removed)
- [x] Move link rewriting into a dedicated content transform module `internal/hugo/content/links.go` (implemented as `content/links.go` with legacy wrapper `hugo/links.go`)
- [x] Split `stages.go` into separate files (kept in `internal/hugo/` package instead of introducing `pipeline/` yet)
  - [x] `classification.go` (StageError, StageOutcome, classifyStageResult)
  - [x] `build_state.go` (BuildState struct + constructor)
  - [x] `runner.go` (runStages + timing + early exit)
  - [x] `stage_prepare.go`
  - [x] `stage_clone.go` (includes `classifyGitFailure` & `readRepoHead` helpers)
  - [x] `stage_discover.go`
  - [x] `stage_generate_config.go`
  - [x] `stage_layouts.go`
  - [x] `stage_copy_content.go`
  - [x] `stage_indexes.go`
  - [x] `stage_run_hugo.go`
  - [x] `stage_post_process.go`
  - [ ] (Deferred) Introduce dedicated `pipeline/` directory structure — decided to postpone until Phase 5 when broader pipeline abstractions land
- [x] Introduce `internal/hugo/errors/` for Hugo/generation sentinel errors
- [x] Add unit tests ensuring no diff in build report for a simple fixture before/after extraction (golden subset stability test added)

## Phase 2: Abstractions & Interfaces

- [x] Theme interface (implemented as `internal/hugo/theme` with `Theme` + `ThemeFeatures`; legacy helpers removed)
- [x] Content transform pipeline
  - [x] Pipeline orchestrator via registry (`internal/hugo/transforms/registry.go`) with priority ordering
  - [x] Registered transforms: front matter parse/build, edit link injector, merge, relative link rewrite, serializer
  - [x] Parity tests against legacy inline pipeline (now decommissioned; stub retained)
  - [x] Formal interface for page object (`PageFacade`) with adapter + facade-oriented transformers (Serialize promoted)
  - [x] Remove legacy `TransformerPipeline` and inline transformers (completed; tests green)
  - [x] Config-driven enable/disable mechanism (`hugo.transforms.enable/disable`) with precedence (disable > enable)
  - [x] Conflict logging assertions (FrontMatterConflict semantics locked by `transform_conflicts_test.go`)
- [x] Renderer abstraction (`Renderer.Execute()`) – implemented via `Renderer` interface with Binary + Noop
  - [x] BinaryRenderer implementation
  - [x] NoopRenderer (tests)
- [x] Observer abstraction (BuildObserver) decoupling metrics recorder (adapter bridges existing metrics)
- [x] RepoFetcher abstraction to unify clone/update decision logic

### Isolation Hardening (Forges / Themes / Transforms) [NEW]

> Goal: Adding a forge, theme, or transform should be an isolated, <200 LOC change touching only its own package + tests. No unrelated file edits. The project is permanently self‑contained (no external/runtime plugins). Reflection is disallowed for core extension points; use generics where they clarify intent.

Planned tasks:

- [x] Consolidate edit link logic: remove `fmcore.ResolveEditLink` and route all generation through a single `EditLinkResolver` (canonical file path normalization; eliminate `docs/docs/` duplication risk).
- [x] Introduce `forge/capabilities.go` with `ForgeCapabilities{SupportsEditLinks, SupportsWebhooks, SupportsTopics,...}` map.
- [x] Introduce `themes/capabilities.go` with `ThemeCapabilities{WantsPerPageEditLinks, SupportsSearchJSON,...}` registered per theme.
- [x] Replace ad hoc protected key maps & transform filter slices with a generic `Set[T comparable]` helper (`internal/util/sets`).
- [x] Add deterministic transform registry order golden hash test (`transform_registry_golden_test.go`).
- [x] Add golden test for capability maps (sorted JSON snapshot) to flag unintentional changes.
- [x] Path normalization test ensuring edit links never duplicate docs base segment.
- [ ] Introduce optional `TransformMeta{Before,After}` (future) with validation (topological check) WITHOUT altering existing priorities yet.
- [x] CI guard test: forbid importing `reflect` outside explicit allowlist (`internal/policy/no_reflect_test.go`).
- [ ] Documentation updates: architecture + CONTENT_TRANSFORMS referencing single resolver & capability maps.
- [ ] Update acceptance criteria section (below) with isolation rules.

Non-goals (explicitly out of scope / will not be revisited):

- Dynamic plugin loading (binary/module discovery, RPC, ABI negotiation).
- Runtime reflection for duck-typing transformers or themes.
- External registry of forge/theme implementations.

Implementation sequencing recommendation:

1. Add capabilities structs & generic Set helper (no behavior change).
2. Swap edit link injector to canonical resolver; remove old fmcore function; fix tests.
3. Add path normalization test & update existing expectations.
4. Introduce registry & capability golden tests.
5. Add no-reflect guard & doc updates.
6. (Optional) Introduce `TransformMeta` + validator.

Risk Mitigation:

- Each step accompanied by focused tests; golden tests ensure no silent behavioral drift.
- Removal of `fmcore.ResolveEditLink` done only after new resolver is covered by permutation tests (GitHub / GitLab / Forgejo / Bitbucket fallback / site-level suppression / existing editURL override).

Exit Criteria for Isolation Hardening:

- All checkboxes above completed.
- Adding a new forge only edits `internal/forge/<forge>.go` + `forge/capabilities.go` + tests.
- Adding a new theme only edits `internal/hugo/themes/<theme>/` + `themes/capabilities.go` + config golden test.
- Adding a new transform only adds one file + a test file (no edits to registry or unrelated code).


## Phase 3: Configuration System Refinement

- [x] Split config loading into phases: load → normalize → apply defaults → validate
- [x] Create `internal/config/normalize/` (build, versioning, monitoring, output, filtering extracted) – monolith removed
- [x] Provide `ConfigSnapshot()` method for hashing build-affecting fields (`Config.Snapshot()`)
- [x] Table-driven normalization tests (render_mode, namespacing, clone strategy, retry modes + versioning/output/filtering)
- [x] Filtering normalization & inclusion in snapshot
- [x] Emit warnings for deprecated env variables once per process (deduplicated) *(render env vars)*

Status Delta (2025-09-30): RepoFetcher integrated; normalization modular; `config_hash` persisted; PageFacade migration complete with golden pipeline test guarding behavior; deprecation warnings active.




## Phase 4: Error & Issue Classification

- [x] Introduce typed git errors (AuthError, NotFoundError, UnsupportedProtocolError, RemoteDivergedError, RateLimitError, NetworkTimeoutError)
- [x] Return typed errors from git client instead of string parsing
- [x] Map typed errors to IssueCodes via lookup table (prioritized over legacy heuristics)
- [x] Replace discovery/generation generic errors with typed wrappers (discovery and generation stages now use sentinel errors)
- [x] Add tests asserting error → issue code matrix stability (typed + heuristic fallback coverage)




## Phase 5: State & Pipeline Evolution

- [ ] Decompose `BuildState` into sub-structs (GitState, DocsState, PipelineState)
- [ ] Replace implicit fields with accessor methods (`AllReposUnchanged()` computes on demand or cached)
- [ ] StageFunc signature returns structured result (`{Err error; Skip bool}`)
- [ ] Add decorator helpers (Timed, WithObserver)
- [ ] Early skip logic isolated in pure function
- [x] Add build report field `pipeline_version`

## Phase 6: Testing & Golden Artifacts

(*Re-list items after earlier sections are stabilized – placeholder heading retained for structure.*)

## Phase 7: Observability & Metrics Cleanup

- [x] Implement Prometheus BuildObserver (recorder adapter + issue & render mode metrics)
- [~] Remove direct recorder usage in stages (clone stage still uses recorder directly for fine-grained repo metrics)
- [x] Add metric: effective_render_mode *(reported via build report field; now emitted as gauge)*
- [x] Add metric: content_transform_failures_total

## Phase 8: Documentation & Developer Experience

- [ ] Update README with new architecture and extension points
- [ ] Add THEME_INTEGRATION.md
- [x] Add CONTENT_TRANSFORMS.md with examples
- [ ] Update migration notes (legacy env → render_mode) & planned deprecation schedule
- [ ] CONTRIBUTING: How to add a stage / transform / theme

## Phase 9: Deprecations & Cleanup

- [ ] Mark legacy env vars (DOCBUILDER_RUN_HUGO, DOCBUILDER_SKIP_HUGO) deprecated in logs
- [ ] Add feature flag guard removal plan (`DOCBUILDER_EXPERIMENTAL_PIPELINE` if introduced)
- [ ] Remove duplicate early-exit logic remnants
- [ ] Collapse any shim layers after adoption period

## Phase 10: Optional Enhancements (Δ)

- [ ] Remote rendering service adapter (future scaling)
- [ ] Partial rebuild detection via per-file hash graph
- [ ] Parallel content transform execution (bounded worker pool)
- [ ] Structured tracing (OpenTelemetry spans per stage)

## Cross-Cutting Quality Gates

- [ ] Ensure no file > 500 LOC (CI check)
- [ ] Lint rule: forbid direct theme branching in generator (must use Theme interface)
- [ ] Coverage threshold ≥ 70% for pipeline, config, transforms packages
- [ ] Static analysis: vet & staticcheck clean

## Work Tracking Fields (add as implemented)

- Pipeline version in report: `report.pipeline_version` (done)
- Effective render mode in report: `report.effective_render_mode` (done)
- Added test fixtures under `testdata/`

## Execution Order Recommendation (Summary)

1. Phase 1 (extractions) – safest, unlocks everything else
2. Phase 2 (interfaces) – creates stable contracts
3. Phase 3 (config) – reduces downstream branching
4. Phase 4 (errors) – simplifies classification before state split
5. Phase 5 (state & pipeline) – larger churn after contracts stable
6. Phases 6–8 – tests & docs consolidate new architecture
7. Phase 9 – deprecations once stabilized
8. Phase 10 – opportunistic enhancements

---

## Acceptance Criteria Snapshot

- Adding a new theme touches ≤ 2 files
- Adding a new content transform requires no stage changes
- Render mode precedence logic fully unit tested
- Early skip decision pure & deterministic
- Build report exposes pipeline & render metadata

---

## Notes

Keep PRs narrowly scoped (target < 400 line diff) and include: motivation paragraph, before/after file list, and confirmation of unchanged behavior via tests or golden output.

---
Happy refactoring! Iterate incrementally; delete dead code aggressively after stable transitions.
