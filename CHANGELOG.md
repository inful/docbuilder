# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once a stable v1.0.0 is tagged. Pre-1.0 the API surface may change between
commits; pin to a SHA for reproducibility per the README.

## [Unreleased]

## [0.19.2] - 2026-09-30

### Changed
- **Build (gitignore)**: excluded local goreleaser build artifacts.
  Added `/-/` (the literal root-level scratch dir local goreleaser runs
  write to when the output dir is overridden — this operator's setup)
  and `*.tar.gz` (goreleaser archive pattern — verified no `.tar.gz`
  files exist anywhere else in the repo, so the broad pattern is
  safe). Without these, every local release build regenerated ~100 MB
  of `.tar.gz` files in `git status` as untracked-but-real entries.

## [0.19.1] - 2026-09-30

### Added
- **Ragabast ingester (config)**: new `ragabast_base_url` field for the
  ragabast host. The dispatcher derives both endpoints from this single
  value: `{base}/api/ingest/file` for the upload and
  `{base}/api/documents/<uid>/fingerprint` for preflight. Path
  components on the field are stripped, so the "POST to / because the
  operator forgot the path" foot-gun is eliminated by construction.
  The endpoint paths are now constants in the dispatcher (matching
  ragabast's stable API contract) rather than something the operator
  has to spell out.

### Deprecated
- **Ragabast ingester (config)**: `ingest_url` (the v0.19.0 form) is
  deprecated. When set without `ragabast_base_url`, the dispatcher
  derives scheme+host from it (path is discarded), logs a deprecation
  warning, and proceeds — so existing configs keep working. Migrate by
  replacing `ingest_url: https://ragabast.example.com/api/ingest/file`
  with `ragabast_base_url: https://ragabast.example.com`.

### Fixed
- **Ragabast ingester (config validation)**: an unparseable
  `ingest_url` (or unparseable `ragabast_base_url`) now fails the
  dispatcher construction with an explicit error rather than
  silently disabling preflight. The previous "garbage URL" path was
  the only test case for that, but the new behavior makes
  misconfigurations visible at startup instead of after the dispatcher
  has been silently running with no preflight base.

## [0.19.0] - 2026-09-30

### Fixed
- **Hugo (auto-generated section `_index.md`)**: stopped emitting today's
  date. The `generateSectionIndex` pipeline stage and the
  `buildBaseFrontMatter` transform were both falling back to `time.Now()`
  when an auto-generated section `_index.md` had no `date` in its
  FrontMatter. For local builds (no git history, so
  `doc.CommitDate.IsZero()`), that meant the build time leaked into
  the rendered site — sitemap `<lastmod>`, RSS `<pubDate>`, OG meta
  tags (`datePublished`, `dateModified`, `article:published_time`,
  `article:modified_time`), and the visible `dateModified` all
  showed today. Replaced both `time.Now()` fallbacks with the
  deterministic epoch `2024-01-01T00:00:00Z` (date) /
  `2024-01-01` (lastmod) — matching what `generateMainIndex` and the
  legacy `internal/hugo/indexes.go` were already doing. Authored
  docs are untouched (their `date` / `lastmod` are preserved
  verbatim). For docs that do have a `lastmod`, Hugo's `.Lastmod`
  template variable continues to drive `dateModified`, sitemap
  `lastmod`, and `article:modified_time` automatically — authors
  who want latest-change-date semantics in the rendered output can
  switch their template from `.Date` to `.Lastmod` without any
  docbuilder changes.

### Added
- **Ragabast ingester (preflight fingerprint check)**: when
  `daemon.outbound.ragabast` is configured, the dispatcher consults
  `GET <base>/api/documents/<uid>/fingerprint` before POSTing each
  document and skips the upload when the stored fingerprint matches
  the document's frontmatter. Saves the upload on rebuilds where
  content hasn't changed. Failure policy is fail-open: 5xx, network
  errors, and parse failures fall through to the upload
  (`preflight_errors_total++`); 401/403 are counted as `fails_total`
  without uploading; 404 (new doc) falls through. The fingerprint
  and uid are parsed from the document's YAML frontmatter via
  `internal/frontmatter` (never recomputed). Preflight is on by
  default when a base URL can be derived from `ingest_url`; set
  `preflight_enabled: false` to opt out, or `preflight_base_url:
  <host>` to use a different host for preflight than for uploads.

### Changed
- **Ragabast ingester (multipart upload migration)**: switched the
  upload path from `POST /api/ingest/async` (JSON `{"content":"..."}`)
  to `POST /api/ingest/file` (multipart/form-data with a `file`
  field). The old endpoint returns 405 to POSTs and is no longer
  supported upstream; ragabast's example script uses the new one.
  The upload path now uses `mime/multipart.Writer` with
  `CreateFormFile("file", path)`. `Content-Type` is set from
  `FormDataContentType()` so the boundary is correct; `application/json`
  is no longer set. Go's multipart parser strips directory components
  from the filename, so the receiver sees just the basename
  (`index.md` rather than `docs/index.md`) — same as `curl -F`
  behaves. Auth via `Authorization: Bearer <token>` is unchanged.
  **Operators must update `daemon.outbound.ragabast.ingest_url` from
  `…/api/ingest/async` to `…/api/ingest/file`** — `config.example.yaml`
  has been updated.

## [0.18.0] - 2026-09-28

### Changed
- **Lint (internal-link-style rule, scoped down)**: dropped the
  missing-`.md` half of `internal-link-style` and its auto-fix. The
  rule used to flag any relative link without `.md` extension as
  WARNING and offered "Append `.md`" as the fix, but that contradicted
  the README's documented `./name/` directory-link pattern (Hugo
  resolves `[text](./api/)` to `api/_index.md`) and the auto-fix
  actively rewrote `[API](./api)` → `[API](./api.md)`, breaking valid
  links to `api/_index.md`. Non-existence is now caught by the
  `broken-links` rule, which already does the right `.md` fallback
  (and also handles directory targets via `os.Stat`). The rule still
  fires for site-rooted links (starting with `/`), which is the only
  remaining cross-renderer portability concern. Removed the
  `applyInternalLinkStyleFixes` function, `appendMarkdownExtension`,
  `LinkStyleUpdate` type, `FixResult.LinkStyleUpdates` field, and the
  wiring in `fixer.go`. The `internal-link-style` entry was removed
  from the MCP `autoFixableRules` map and the closed-set test in
  `lint_fix_manual_test.go`. The rule description in
  `docbuilder-best-practices.md` was updated. Closes #76.

### Removed
- **Lint (missing-index-page rule)**: dropped the WARNING that flagged
  directories with ≥3 `.md` children and no `_index.md`, plus its
  auto-fixer that generated a placeholder landing page. The rule
  produced more lint noise than value: the auto-generated placeholder
  itself failed `frontmatter-required-fields` and surfaced as an ERROR
  in the `Detect Lint Rule Drift` workflow, requiring a follow-up fix
  in 0.17.1. The placeholder was also opinion-driven (curated landing
  page vs. Hugo's auto-generated listing). `DocBuilder` continues to
  generate Hugo `_index.md` files at the site, repository, and section
  levels via the build pipeline; authors who want curated section
  indexes can add them by hand. Removed the `DirectoryRule` interface
  and `runDirectoryRules` helper from the linter, the
  `MissingIndexPages` field from `FixResult`, the `itoa` helper was
  moved into `rule_tag_count.go` (its remaining caller), and the rule
  entries in `tools.go`, `lint_fix_manual_test.go`, and four docs
  (`lint-rules.md`, `lint-rules-changelog.md`, `migrate-to-linting.md`,
  `docbuilder-best-practices.md`) were updated. Closes #70.

## [0.17.1] - 2026-09-27

### Fixed
- **Hugo (auto-generated _index.md)**: the `generateMainIndex` function in
  the pipeline now pre-populates the root `_index.md` Document with all
  `frontmatter-required-fields` rule fields (`date`, `lastmod`,
  `categories`, `tags`, `uid`). Previously the downstream
  `transformFrontmatter` only filled `date`; the generated page failed
  the new lint rule and the `Detect Lint Rule Drift` workflow flagged it
  as ERROR. `date` and `lastmod` are pinned to a deterministic epoch
  (matching the manual index path) so generated index frontmatter is
  stable across rebuilds.

### Changed
- **Lint (idiomatic cleanups)**: replaced manual loops with
  `slices.Contains` and `strings.Cut`, dropped `+=` string concatenation
  for `strings.Builder` in three test helpers, removed unused fields
  (`FilenameRule.cfg`, `FixResult.uidTargetsAdded`) and the unused
  `issueCounts` parameter on `applyMissingIndexPageFixes`, extracted
  `runDirectoryRules` from `LintPath` to keep branch complexity under
  the `nestif` threshold, and renamed the local `min` shadowing the
  builtin in `MissingIndexPageRule` to `threshold`. No rule semantics
  changed.

### Tests
- **Lint (golden fixtures + fixture swap)**: `TestIntegration_RenameWithLinkUpdates`
  was vacuous against the default excludes — its `docs/README.md`
  fixture was being skipped from the fix pass. Renamed it to
  `docs/introduction.md` so the integration actually exercises the
  rename-update path. `TestFixer_RenameFile` assertion updated
  (`ErrorsFixed: 3 → 4`) to count the new `frontmatter-required-fields`
  fixer pass. Golden files (`fix-dry-run.golden.txt`,
  `fix-with-links.golden.json`) updated to reflect the new error count
  and shifted link-update line numbers from the auto-generated
  `_index.md` frontmatter block.

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
- **Lint (issue iteration)**: the fixer's issue-collection loop now keeps
  both `SeverityError` and `SeverityWarning` so the new warning-level
  rules participate in target building.
- **Lint (filename matcher)**: the cross-forge `excludes` list supports
  recursive globs (`**`) for nested template directories
  (`.github/ISSUE_TEMPLATE/**/*.md`, `.gitlab/issue_templates/*.md`,
  `.gitea/ISSUE_TEMPLATE/**/*.md`).
- **Refactor (hugo/docs)**: extracted the duplicated `normalizeTaxonomyValues` /
  `normalizeSnippetTaxonomyValues` into a single shared `docs.NormalizeTaxonomyValues`.
  Both `internal/doctemplate/app/taxonomy_client.go` and
  `internal/docs/taxonomy_snippets.go` now call the shared helper.
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

> **Note (retroactive)**: this section was amended after the v0.17.0 tag
> was published. The ragabast dispatcher, taxonomy/forge/stages refactors,
> dead-code removal, coverage-artefact cleanup, and additional test
> coverage all shipped in v0.17.0 but were originally listed under
> `[Unreleased]`. The release-notes commit (`965105c`) that closed out
> v0.17.0 didn't roll those entries forward. This entry corrects the
> record; the binaries are unchanged.

[0.18.0]: https://github.com/inful/docbuilder/compare/v0.17.2...v0.18.0
[0.17.1]: https://github.com/inful/docbuilder/compare/v0.17.0...v0.17.1
[0.17.0]: https://github.com/inful/docbuilder/compare/v0.16.1...v0.17.0
[Unreleased]: https://github.com/inful/docbuilder/compare/v0.18.0...HEAD
