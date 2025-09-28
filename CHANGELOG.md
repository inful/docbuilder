# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Removed

- Legacy `Page.FrontMatter` field replaced by patch-based system (`OriginalFrontMatter`, `Patches`, `MergedFrontMatter`).
- Deprecated V2 aliases: `V2Config`, `InitV2`, `LoadV2`, `IsV2Config` removed; use unified `Config` API (`config.Load`, `config.Init`).
- Outcome duplication eliminated (`OutcomeT`); single typed `BuildOutcome` retained on `BuildReport`.
- Legacy theme registry & prometheus resolver stubs marked for deletion (currently removed from runtime usage).

### Added

- `BuildReport.CloneStageSkipped` to distinguish pipelines without a clone stage.
- Index template reporting: `IndexTemplates` with source (embedded|file) and path.
- Structured issue taxonomy via `ReportIssue` (`Issues` slice in `BuildReport`).

### Changed

- Serialization logic derives `cloned_repositories` heuristically when clone stage skipped.
- Front matter merge logic now requires explicit patch injection (no implicit legacy mirroring).

### Migration Notes

1. Remove any direct references to `Page.FrontMatter`; use `OriginalFrontMatter` (read) or add patches producing `MergedFrontMatter`.
2. Replace calls to `config.InitV2` with `config.Init`.
3. Update any code/tests expecting `OutcomeT` to use `BuildReport.Outcome` (string value set from `BuildOutcome`).
4. If relying on deprecated theme/Prometheus placeholders, migrate to the current metrics and theme module logic.

---

Past versions used transitional compatibility layers that have now been fully removed to simplify maintenance.
