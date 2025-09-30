# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Removed

- Legacy `Page.FrontMatter` field replaced by patch-based system (`OriginalFrontMatter`, `Patches`, `MergedFrontMatter`).
- Legacy front matter builder & edit link injector closures (`BuildFrontMatter`, `InjectEditLink`) fully removed; replaced by V2 transform pair (`front_matter_builder_v2`, `edit_link_injector_v2`).
- Deprecated V2 aliases: `V2Config`, `InitV2`, `LoadV2`, `IsV2Config` removed; use unified `Config` API (`config.Load`, `config.Init`).
- Outcome duplication eliminated (`OutcomeT`); single typed `BuildOutcome` retained on `BuildReport`.
- Legacy theme registry & prometheus resolver stubs fully removed.
- `computeBackoffDelay` helper and its unit test (use `retry.Policy` directly).

### Added

- Automatic multi-forge content namespacing: when documentation is built from repositories spanning more than one forge type, Markdown is written under `content/<forge>/<repository>/...`; single-forge builds retain the previous `content/<repository>/...` layout. (`DocFile.Forge` field added.)
- `BuildReport.CloneStageSkipped` to distinguish pipelines without a clone stage.
- Index template reporting: `IndexTemplates` with source (embedded|file) and path.
- Structured issue taxonomy via `ReportIssue` (`Issues` slice in `BuildReport`).
- Stable hash of discovered documentation file set: `BuildReport.DocFilesHash` (SHA-256 hex of sorted Hugo paths) for consumer-side cache invalidation and change detection.

### Changed

- `cloned_repositories` is no longer heuristically derived when the clone stage is skipped; the value now reflects only actually cloned repositories (zero or omitted when no clone stage ran). If you previously relied on the fallback count, update any dashboards/scripts to tolerate zero.
- Front matter merge logic now requires explicit patch injection (no implicit legacy mirroring).

### Migration Notes

1. Remove any direct references to `Page.FrontMatter`; use `OriginalFrontMatter` (read) or add patches producing `MergedFrontMatter`.
2. Replace calls to `config.InitV2` with `config.Init`.
3. Update any code/tests expecting `OutcomeT` to use `BuildReport.Outcome` (string value set from `BuildOutcome`).
4. If relying on deprecated theme/Prometheus placeholders, migrate to the current metrics and theme module logic.
5. Replace any direct usage of the removed `computeBackoffDelay` with `retry.NewPolicy(...).Delay(n)`.
6. Adjust any tooling expecting a synthesized `cloned_repositories` count when skipping the clone stage; the heuristic has been removed for accuracy.

---

Past versions used transitional compatibility layers that have now been fully removed to simplify maintenance.
