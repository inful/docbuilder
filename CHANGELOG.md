# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once a stable v1.0.0 is tagged. Pre-1.0 the API surface may change between
commits; pin to a SHA for reproducibility per the README.

## [Unreleased]

### Added
- **Tests**: direct unit tests for `nodeFromAny` (`internal/frontmatter/serialize.go`)
  covering all scalar, sequence, and map branches.
- **Tests**: edge-case coverage for `FromMap` (`internal/hugo/models/frontmatter.go`)
  including wrong-type coercion, invalid date strings, and mixed-type taxonomy arrays.
- **Tests**: focused unit tests for `inferRenameMappingFromGitHead`
  (`internal/lint/broken_link_healer.go`) covering all 17 branches without requiring
  a git repository.
- **Tests**: direct unit tests for `repoAbsPath`
  (`internal/lint/git_uncommitted_rename_detector.go`) including traversal
  rejection.

### Changed
- **Refactor (hugo)**: extracted the duplicated `normalizeTaxonomyValues` /
  `normalizeSnippetTaxonomyValues` into a single shared `docs.NormalizeTaxonomyValues`.
  Both `internal/doctemplate/app/taxonomy_client.go` and
  `internal/docs/taxonomy_snippets.go` now call the shared helper.

### Fixed
- **Refactor (forge)**: broke a call-graph cycle between
  `internal/forge/enhanced_mock.go` and `internal/forge/enhanced_mock_factory.go`
  by moving the `EnhancedMockBuilder` type and the `CreateRealistic*Mock`
  constructors into `enhanced_mock.go` (where `EnhancedMockForgeClient` lives).
- **Refactor (stages)**: broke a call-graph cycle between
  `internal/hugo/stages/repo_fetcher.go` and `internal/hugo/stages/stage_clone.go`
  by removing the deprecated `readRepoHead` wrapper. Callers now use
  `gitpkg.ReadRepoHead` directly.

### Removed
- Six stale coverage artefacts at the repo root (`coverage.out`,
  `coverage_cmd.out`, `coverage_final.out`, `coverage_new.out`,
  `coverage_review.out`, `coverage.html`). They were never tracked by git;
  `.gitignore` already excluded them. This file (`CHANGELOG.md`) is now the
  source of truth for release notes.
- Dead code surfaced by the `analyze kind=dead_code` graph pass (each item
  verified to have zero in-tree callers, including tests):
  - `internal/hugo/models/typed_transformers.go` (entire file) — four unused
    `V2`/`V3` transformers (`FrontMatterParserV2`, `FrontMatterBuilderV3`,
    `EditLinkInjectorV3`, `ContentProcessorV2`) and their tests.
  - `internal/hugo/stages/stage_execution.go` (entire file) —
    `StageExecution` type and outcome helpers (`ExecutionSuccess*` /
    `ExecutionFailure*`).
  - `internal/hugo/models/early_skip.go` (entire file) — `EarlySkipDecision`
    type and `EvaluateEarlySkip`, `NoSkip`, `SkipAfter` (the runner already
    inlines the early-skip decision at line 59 of
    `internal/hugo/stages/runner.go`).
  - Per-forge webhook handlers `HandleGitHubWebhook`, `HandleGitLabWebhook`,
    `HandleForgejoWebhook` in `internal/server/handlers/webhook.go` — the
    dispatcher `HandleForgeWebhook` (wired by config) and the generic
    `HandleWebhook` (wired at `/webhook`) remain the single entry points.
  - `foundation.Option.Match` / `UnwrapOrElse`,
    `foundation.Result.ToTuple`, `ContentPage.GetOriginalFrontMatter` /
    `SetOriginalFrontMatter` / `AddTransformationRecord` /
    `GetTransformationHistory` / `HasBeenTransformed`,
    `TransformationPipeline.SetContext`, `TransformationResult.SetSource`,
    `Pipeline.AddIf`, `Statistics.UpdateDiscoveryStats`,
    `Manager.WithAutoSave`, `Service.GetScheduleStore` /
    `GetDaemonInfoStore`, `Resolve.WithObserver`,
    `Generator.WithObserver`, `GitState.SetCommitDate`, `Report.GetDocBuilderVersion`,
    `MigrationHelper.ConvertLegacyFrontMatter`,
    `EditorLinkResolver.NewResolverWithChain`, `CommitDetector.Clear`, and
    the testutil fluent helpers `WithTitle` / `WithTheme` / `WithOutputDir` /
    `WithEnv` / `AssertFailure`.

## [0.x] — Recent themes

The last 90 days focused on four themes:

### Hugo sidebar: categories become the new default
- `feat(hugo)`: make categories sidebar the new default with `_uncategorized` triage bucket.
- `feat(hugo)`: add categories-mode sidebar wiring.
- `feat(hugo)`: per-category sub-grouping via `hugo.sidebar.group_by`.
- `feat(hugo)`: support N-level max-depth nesting in `hugo.sidebar.group_by`.
- `feat(hugo)`: extend `sidebar.group_by` with sibling-category and multi-value axes.
- `fix(hugo)`: render category titles via top-level wrapper entry.
- `fix(hugo)`: align sidebar identifier with Hugo menu key for Relearn lookup.
- `fix(hugo)`: load doc content on demand in the categories-menu stage.
- `fix(hugo)`: filter categories sidebar by `daemon.content.public_only`.
- `fix(hugo)`: group categories case-insensitively, keep first-seen spelling.
- `fix(hugo)`: make `group_by` case-insensitive on config keys and field names.
- `fix(hugo)`: rename project taxonomy to plural `projects` (Hugo convention).
- `fix(hugo)`: preserve `$...$` inline math through Goldmark passthrough.

### Doctemplate TUI (template generation)
- `feat(doctemplate)`: add Bubble Tea TUI for template generation.
- `feat(doctemplate)`: use preview taxonomy API for tag/category suggestions.
- `feat(doctemplate)`: left/right arrows cycle suggestions for `string_list` fields.
- `feat(templates)`: add `GlobSuggestion` field to `SchemaField`.
- `fix(tui)`: only suggest directories for glob patterns ending with `*/`.
- `fix(templates)`: support metadata headers without transitions.

### Daemon & webhooks
- `fix(daemon)`: recheck filtered repos on webhook (#61).
- `feat`: implement asset size limits and forge namespace stability.

### Build pipeline hygiene
- `refactor(hugo)`: centralize content-tree path construction in `docs.HugoContentPath`.
- `refactor(concurrency)`: use `WaitGroup.Go` in async paths.
- `feat`: add doctemplate build configuration.
- `fix(hugo)`: lowercase repository names in index paths.
- `fix(docs)`: only include GitLab group in content path when needed.

[Unreleased]: https://github.com/inful/docbuilder/compare/main...HEAD
