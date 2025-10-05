# Post-refactor cleanup opportunities

Date: 2025-10-04

This document tracks code that can be simplified or removed now that the refactor is complete and tests are green.

## High-value cleanup targets

1. internal/config/defaults/applier.go (duplicate)

   - Current: duplicate defaults implementation in a separate `defaults` package accidentally committed; currently ignored via build tag.
   - Action: Remove the file from the repository to avoid confusion. The canonical defaults live in `internal/config/defaults.go` + `composite_defaults.go`.
   - Status: Done (file deleted)

2. Enum normalizers default behavior

   - Current: Normalizers now return empty on unknown values to allow explicit fallback+validation in normalize/defaults steps.
   - Action: Ensure comments and README reflect this; remove any remaining assumptions of implicit defaults in normalizers.

3. Legacy conversion helpers in config (forge_typed.go)

   - Current: Functions `ToLegacyForgeType` and `FromLegacyForgeType` exist to bridge old/typed APIs.
   - Action: Search for production usages outside tests; if none, deprecate and remove in a follow-up PR. Keep tests if they validate conversion logic for external callers; otherwise, prune.
   - Status: **Done** - Legacy converters removed; tests updated to use typed APIs directly; suite green

4. LegacyCompatibilityAdapter (internal/hugo/models/migration.go)

   - Current: Helper exposed map[string]any compatibility for front matter (ConvertToLegacyFormat, ConvertPatchToLegacyFormat, WrapLegacyFunction), originally for incremental migration.
   - Action: Remove adapter and update tests to use typed FrontMatter/FrontMatterPatch directly.
   - Status: Done (adapter deleted; tests updated; suite green)

5. Migration bridge deprecation (internal/hugo/models/migration_bridge.go)

   - Current: TransformMigrationBridge, LegacyTransformerAdapter, and PageShimAdapter provide transitional shims between legacy and typed models.
   - Action: Mark as Deprecated across constructors and methods; plan staged removal after confirming no production consumers. Update tests in phases to use typed pipeline directly.
   - Status: **Done** - Migration bridge and all adapters removed; no production usage found; tests green

6. State migration adapter (internal/state/migration.go)

   - Current: Provides compatibility to old state manager interface.
   - Action: Identify live consumers; if none, remove adapter types and inline examples. Convert any remaining call sites to the new service methods.
   - Status: **Done** - StateManagerAdapter removed; tests updated to use StateService directly; suite green

7. Normalize sub-package (internal/config/normalize)

   - Current: Parallel normalizer exists; main code uses `internal/config.NormalizeConfig`.
   - Action: Confirm there are no production imports of `internal/config/normalize`. If unused, consider removing to avoid confusion, or move any unique examples/tests into main package.
   - Status: Done (package removed; there were no production imports)

8. Deprecated comments and TODOs

   - Current: Some TODOs in daemon health/logging are placeholders.
   - Action: Convert TODOs into issues or implement minimal checks; otherwise reword to clarifying comments.
   - Status: **In progress** - Version TODOs resolved with internal/version package; build info centralized

9. **Version Management**
   - ✅ **Version centralization** - Created internal/version package with centralized version constants
   - ✅ **Build info TODO cleanup** - Replaced hardcoded version strings in daemon handlers/status with version.Version
   - ✅ **Build-time integration ready** - Version package supports ldflags injection for production builds

10. **Legacy Code Removal**

    - ✅ **Migration bridge cleanup** - Removed TransformMigrationBridge, LegacyTransformerAdapter, PageShimAdapter
    - ✅ **State adapter cleanup** - Removed StateManagerAdapter; updated tests to use StateService directly
    - ✅ **Legacy type converters** - Removed ToLegacyForgeType/FromLegacyForgeType; updated tests to use typed APIs
    - ✅ **Obsolete fixtures** - Removed unused daemon-data directory and stub test files

11. ✅ **CLI and HTTP Error Adapters** (See `internal/foundation/errors/`) - **COMPLETED**

    - ✅ CLI error adapter with exit code mappings per ADR specifications
    - ✅ HTTP error adapter with status code mappings and JSON responses
    - ✅ Structured logging integration with severity-based log levels
    - ✅ CLI integration in main.go replacing generic kong error handling
    - ✅ HTTP integration in daemon handlers replacing manual error handling
    - ✅ Panic recovery middleware with structured error responses
    - ✅ Comprehensive test coverage validating adapter behavior

12. Documentation Markdown Linting

    - Current: Recent documentation edits introduced markdown formatting issues (missing blank lines around lists/fences, heading spacing).
    - Action: Fix markdown lint errors in cleanup tracker and implementation docs to maintain documentation quality.
    - Status: In progress (cleanup tracker fixed; implementation doc needs formatting fixes)

## Minor polish

- Replace leftover `%v` in user-facing errors with structured fields via the new error adapters.
- Standardize log field keys with `internal/logfields` across boundaries.
- Remove obsolete fixtures under internal/daemon/daemon-data if no longer referenced by tests.

## Validation steps

- Run `go test ./...` after each removal.
- Grep for symbols before deleting:
  - `grep -R "StateManagerAdapter" -n` and `grep -R "ToLegacyForgeType\|FromLegacyForgeType" -n`
- If external integrations require legacy helpers, mark them with clear deprecation notices and keep them tested.

Validation status: Full test suite passed after adapter deletion and deprecation comments.

