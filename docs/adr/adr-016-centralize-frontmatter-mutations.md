---
aliases:
  - /_uid/a7382480-b52e-4dcf-a0df-64129dbe4604/
categories:
  - architecture-decisions
date: 2026-01-22T00:00:00Z
fingerprint: 5cde158f7caf42b9552017165bfe1c33acbecd2380fbac81db779b19cc0ee20e
lastmod: "2026-01-22"
tags:
  - frontmatter
  - yaml
  - refactor
  - hugo
  - pipeline
  - linting
  - fingerprint
  - uid
uid: a7382480-b52e-4dcf-a0df-64129dbe4604
---

# ADR-016: Centralize frontmatter mutations (map-based ops)

**Status**: Implemented  
**Date**: 2026-01-22  
**Decision Makers**: DocBuilder Core Team

**Implementation Plan**: [adr-016-implementation-plan.md](adr-016-implementation-plan.md)

**Implementation Completed**: 2026-01-22 (plan completion commit: `e95b442`)

## Context and Problem Statement

DocBuilder has already centralized the *parsing primitives* for YAML frontmatter:

- `internal/frontmatter` provides `Split`, `ParseYAML`, `SerializeYAML`, `Join` ([ADR-014](adr-014-centralize-frontmatter-parsing-and-writing.md))

However, multiple subsystems still implement their own *mutation logic* on top of those primitives:

- Hugo build pipeline mutates frontmatter (`title`, `type`, `date`, repo metadata, `editURL`, `fingerprint`) in multiple transforms.
- Index generation mutates frontmatter in template expansion paths.
- Lint/fix mutates source files to add/update `uid`, `aliases`, `fingerprint`, and `lastmod`.

This leads to duplicated and potentially divergent behavior:

- Multiple places re-implement “split → parse → mutate → serialize → join”.
- Fingerprinting requires canonicalization (which keys are excluded from hashing) and exists in both build and lint/fix code paths.
- Field naming conventions can drift (e.g., `editURL` vs `edit_url`), especially as the typed frontmatter model evolves.

### Why this matters

Frontmatter values are used for:

- Hugo layout selection (`type`, `layout`)
- Navigation and page metadata (`title`, `date`, `weight`)
- DocBuilder stability and invariants (`uid`, `aliases`, `fingerprint`, `lastmod`)

If the same inputs yield different outputs depending on which subsystem touches them, we risk:

- Unstable diffs and hard-to-debug “it changed again” behavior
- Lint and build disagreeing about fingerprint semantics
- Edge cases being handled inconsistently (no frontmatter, empty frontmatter, malformed frontmatter)

## Decision

Introduce a shared, map-based frontmatter mutation layer that centralizes *addition/modification/removal* of frontmatter fields while keeping the existing YAML-only parsing/writing primitives.

Concretely:

- Keep `internal/frontmatter` as the source of truth for splitting/parsing/serializing/joining YAML frontmatter.
- Add a new internal package (tentative name: `internal/frontmatterops`) that:
  - Provides a single set of helpers to mutate `map[string]any` frontmatter.
  - Defines canonical behavior for DocBuilder-specific fields.
  - Provides a single canonical fingerprint computation helper.

This is an incremental step intended to reduce duplication immediately without requiring a full migration to the typed `internal/hugo/models` frontmatter system.

## Non-Goals

- Introducing TOML (`+++`) or JSON frontmatter support.
- Re-rendering Markdown bodies from an AST.
- Fully migrating all frontmatter handling to typed `FrontMatter` + `FrontMatterPatch`.
- Standardizing arbitrary user custom fields beyond providing safe set/merge utilities.

## Proposed API Shape (internal)

The new `internal/frontmatterops` package should be intentionally small and policy-focused.

### 1) Document split/merge convenience

Helpers that wrap the low-level split/parse/join so call sites do not repeat it:

- `Read(content []byte) (fields map[string]any, body []byte, had bool, style frontmatter.Style, err error)`
- `Write(fields map[string]any, body []byte, had bool, style frontmatter.Style) ([]byte, error)`

These should delegate to `internal/frontmatter` for the mechanics.

### 2) Canonical mutation helpers

Policy functions for DocBuilder invariants (examples):

- `EnsureTypeDocs(fields map[string]any)`
- `EnsureTitle(fields map[string]any, fallback string)`
- `EnsureDate(fields map[string]any, commitDate time.Time, now time.Time)`
- `EnsureUID(fields map[string]any) (uid string, changed bool)`
- `EnsureUIDAlias(fields map[string]any, uid string) (changed bool)`
- `SetIfMissing(fields map[string]any, key string, value any) (changed bool)`
- `DeleteKey(fields map[string]any, key string) (changed bool)`

### 3) Canonical fingerprinting

A single helper that both build and lint/fix can use:

- `ComputeFingerprint(fields map[string]any, body []byte) (fingerprint string, err error)`

This helper defines:

- Which fields are excluded from hashing (at minimum: `fingerprint`, `lastmod`, `uid`, `aliases`).
- Serialization style for hashing (LF, single trailing newline trimmed) so hashing is stable.

Updating `fingerprint` and applying [ADR-011](adr-011-lastmod-on-fingerprint-change.md) (“update `lastmod` when fingerprint changes”) should also be centralized:

- `UpsertFingerprintAndMaybeLastmod(fields map[string]any, body []byte, now time.Time) (changed bool, err error)`

### 4) Key naming normalization (pragmatic)

To reduce drift while preserving existing output expectations:

- Treat `editURL` as the canonical map key emitted by the map-based pipeline.
- When reading, allow both `editURL` and `edit_url` and normalize internally.

(Full key schema unification is deferred to a future typed-frontmatter migration ADR.)

## Options Considered

### Option A: Centralize map-based frontmatter ops (this ADR)

- Pros:
  - Immediate reduction in duplication across pipeline/index/lint.
  - Low migration risk (no schema changes required).
  - Central place to define canonical fingerprint semantics.
- Cons:
  - Still map-based (less type safety than the typed model).
  - Requires discipline to route new mutations through the ops layer.

### Option B: Centralize via `internal/docmodel` only

- Pros:
  - Fewer split/parse calls; encourages “parse once” workflow reuse.
- Cons:
  - Does not by itself prevent policy drift (call sites can still mutate maps inconsistently).
  - Not as explicit about frontmatter semantics and invariants.

### Option C: Migrate everything to typed `FrontMatter` + `FrontMatterPatch`

- Pros:
  - Best long-term type safety.
- Cons:
  - Larger refactor and more risk of behavior changes.
  - Requires resolving key naming differences (`editURL` vs `edit_url`) across all outputs.

## Consequences

### Positive

- Consistent semantics for:
  - UID/aliases insertion
  - fingerprint + lastmod behavior
  - required Hugo fields (`title`, `date`, `type`)
- Reduced code duplication across:
  - Hugo pipeline transforms
  - index generation
  - lint/fix

### Negative

- Adds one more internal package boundary.
- Requires ongoing maintenance: new frontmatter behavior should be added to ops, not re-implemented in call sites.

## Migration Plan

1. Create `internal/frontmatterops` with a minimal surface:
   - read/write helpers
   - canonical fingerprint helpers
2. Migrate fingerprint logic in:
   - build pipeline (`fingerprintContent`)
   - lint fixer/rules (frontmatter fingerprint checks and fixes)
3. Migrate UID/alias insertion in lint fixer to use ops helpers.
4. Migrate index generation helpers (`ensureRequiredIndexFields`, `reconstructContentWithFrontMatter`) to use ops helpers.
5. Add focused unit tests for ops (hash canonicalization, key normalization, UID alias behavior).

## Acceptance Criteria

- Build pipeline and lint/fix produce the same fingerprint semantics for the same (frontmatter, body) inputs.
- UID alias logic is identical across all code paths.
- No regression in current frontmatter output expectations (tests/goldens remain stable).
- New frontmatter mutations are implemented via `internal/frontmatterops`.
