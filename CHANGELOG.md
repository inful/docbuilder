# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once a stable v1.0.0 is tagged. Pre-1.0 the API surface may change between
commits; pin to a SHA for reproducibility per the README.

## [Unreleased]

### Added
- **Daemon (ragabast integration)**: opt-in outbound dispatcher that pushes
  each generated document to ragabast's `/api/ingest/async` endpoint
  immediately after it has been written to disk. Daemon-mode-only and
  off by default; operators enable it via `daemon.outbound.ragabast` in
  config. The bearer token is read from `RAGABAST_INGEST_TOKEN` at
  daemon startup; the dispatcher is bounded, drops on overflow, and
  honours ragabast's persistent job queue + `uid`/`fingerprint` dedup
  for self-healing delivery.
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

### MCP server for LLM integration (v0.14.0–v0.14.3)
- **New `docbuilder-mcp` binary** in `cmd/mcp-server` speaking the
  [Model Context Protocol](https://modelcontextprotocol.io/) over stdio.
  Surfaces 10 tools — 6 read-only (`get_config`, `list_templates`,
  `describe_template`, `resolve_template_inputs`, `lint_docs`,
  `read_doc`) and 4 mutating (`create_from_template`, `lint_fix`,
  `create_doc`, `update_doc`) — plus a `config://current` resource.
  Mutating tools advertise `destructiveHint` and require `confirm=true`
  so hosts gate them behind user prompts. Auth tokens, passwords, and
  key paths are redacted as `***` in every response. Write tools are
  path-contained to the configured `--docs-dir` (read & lint tools
  accept any path the caller provides, since lint is a general tool).
  Built on `github.com/mark3labs/mcp-go`.
- **Patch releases in this series:**
  - v0.14.0 — initial MCP server release.
  - v0.14.1 — added `docbuilder-mcp` to GoReleaser config so the
    release tarball ships it.
  - v0.14.2 — added a workflow + safety + pitfalls instructions
    string sent in the `initialize` response so LLMs learn the
    recommended tool order at session start.
  - v0.14.3 — security bump: `go-git` v5.19.1 → v5.19.2, clearing two
    Dependabot advisories (malicious reference names; worktree
    symlink follow).

### Daemon & webhooks
- `fix(daemon)`: recheck filtered repos on webhook (#61).
- `feat`: implement asset size limits and forge namespace stability.

### Build pipeline hygiene
- `refactor(hugo)`: centralize content-tree path construction in `docs.HugoContentPath`.
- `refactor(concurrency)`: use `WaitGroup.Go` in async paths.
- `feat`: add doctemplate build configuration.
- `fix(hugo)`: lowercase repository names in index paths.
- `fix(docs)`: only include GitLab group in content path when needed.

## [0.17.0] - 2026-09-27

### Added
- **Lint (cross-forge excludes)**: new `lint.excludes` config block with a
  cross-forge union of git-forge-conventional filenames (GitHub / GitLab /
  Forgejo — `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`,
  `LICENSE`, etc.). The walk skips these files from schema checks and the
  `filename-conventions` rename suggestion. Closes #77.
- **Lint (new rules)**:
  - `body-h1` (WARNING) — flags `# Title` at the start of a body, which
    duplicates the rendered H1 from frontmatter `title`. Closes #68.
  - `missing-index-page` (WARNING) — flags directories with ≥3 `.md`
    children and no `_index.md`. Closes #70.
  - `sequence-prefix-filename` (ERROR) — flags files in sequence-numbered
    directories (`adr/` by default) that don't match `prefix-NNN-slug.md`.
    Closes #73.
  - `tag-count` (WARNING) — flags `tags:` lists with more than 10 entries.
    Closes #75.
  - `internal-link-style` (WARNING) — flags `[text](relative/path)` without
    `.md` extension or site-rooted (`/foo`) targets. Closes #76.
  - `category-naming` (ERROR) — flags categories that don't match
    `^[a-z][a-z0-9-]*$` (kebab-case). Closes #71.
  - `cross-mode-category` (ERROR) — flags docs whose `categories:` lists
    two distinct doc modes without a configured parent/child relation.
    Closes #74.
  - `directory-category-consistency` (ERROR) — flags docs in mode-specific
    directories (`how-to/`, `reference/`, etc.) whose `categories:` is
    missing the expected mode value. Closes #69.
  - `frontmatter-required-fields` (ERROR for missing/empty, WARNING for
    malformed dates) — verifies `title`, `date`, `lastmod`,
    `categories`, `tags` per the doc-builder contract. Closes #72.
- **Lint (auto-fixers)**: new `lint_fix` phases for the auto-fixable
  rules (`frontmatter-required-fields`, `category-naming`,
  `directory-category-consistency`, `internal-link-style`,
  `sequence-prefix-filename`, `missing-index-page`). The fixer's
  audit-test path (`lint_docs` → `lint_fix` → `lint_docs`) now reaches
  zero ERRORs without manual intervention for the auto-fixable subset.
- **MCP (lint-fix hints)**: server `instructions` and the `lint_docs`
  tool description now mention that each issue carries a `fix` field
  with a concrete remediation hint. `lint_fix`'s response includes a
  `manual_required` array listing issues that couldn't be auto-fixed
  (typically `body-h1`, `tag-count`, `cross-mode-category`) so the LLM
  can surface them with their fix hints verbatim.

### Changed
- **Lint (issue iteration)**: the fixer's issue-collection loop now keeps
  both `SeverityError` and `SeverityWarning` so the new warning-level
  rules participate in target building.
- **Lint (filename matcher)**: the cross-forge `excludes` list supports
  recursive globs (`**`) for nested template directories
  (`.github/ISSUE_TEMPLATE/**/*.md`, `.gitlab/issue_templates/*.md`,
  `.gitea/ISSUE_TEMPLATE/**/*.md`).

[0.17.0]: https://github.com/inful/docbuilder/compare/v0.16.1...v0.17.0
[Unreleased]: https://github.com/inful/docbuilder/compare/v0.17.0...HEAD
