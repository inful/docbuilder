---
uid: 5b920f1e-30f3-40ab-9c34-86eb5f8f8db4
aliases:
  - /_uid/5b920f1e-30f3-40ab-9c34-86eb5f8f8db4/
date: 2026-01-20
categories:
  - architecture-decisions
tags:
  - frontmatter
  - yaml
  - refactor
  - parsing
  - hugo
  - linting
---

# ADR-014: Centralize frontmatter parsing and writing

**Status**: Accepted 
**Date**: 2026-01-20  
**Decision Makers**: DocBuilder Core Team

## Context and Problem Statement

DocBuilder reads and writes Markdown frontmatter in multiple subsystems:

- Linting/fixing (UID insertion, fingerprint/lastmod updates)
- Build pipeline transforms (parse, normalize, serialize)
- Link verification/event reporting (parse extracted frontmatter)

Today, frontmatter handling is implemented in several places with slightly different behaviors and parsing strategies:

- Different delimiter detection strategies (`---\n` vs split-on-"---")
- Different handling of malformed or empty frontmatter
- Duplicate helper functions (e.g., extracting `uid:` for preservation logic exists in more than one package)

This duplication increases the risk that:

- the same Markdown file is interpreted differently depending on which subsystem touches it
- small changes to frontmatter rules require edits in multiple packages
- future Markdown-aware refactors (e.g., AST-based link updates) become harder because we can’t rely on a single “frontmatter boundary” definition

## Decision

Introduce a single, shared internal frontmatter component responsible for:

- Detecting YAML frontmatter blocks
- Parsing frontmatter into `map[string]any` (or typed models when appropriate)
- Writing frontmatter back to Markdown deterministically
- Providing helper utilities for common DocBuilder operations (read/set scalar fields, preserve selected keys)

This component will be used by linting, build pipeline transforms, and link verification.

## Non-Goals

- Replacing Hugo’s frontmatter semantics or implementing full Hugo compatibility beyond YAML parsing.
- Supporting TOML (`+++`) or JSON frontmatter blocks.
- Inferring frontmatter format heuristically.
- Re-rendering Markdown content bodies (this ADR is frontmatter-only).

## Proposed API Shape (internal)

A small API that clearly separates “frontmatter” from “body”, enabling safe round-trip edits:

- `Split(content []byte) (frontmatter []byte, body []byte, had bool, style Style, err error)`
- `ParseYAML(frontmatter []byte) (map[string]any, error)`
- `SerializeYAML(fields map[string]any, style Style) ([]byte, error)`
- `Join(frontmatter []byte, body []byte, had bool, style Style) []byte`

Where `Style` captures details required for stable rewriting (newline style, delimiter form, trailing newline).

This package is **YAML-only** and only recognizes YAML frontmatter using `---` delimiters.

## Benefits

- **Consistency**: one authoritative definition of “what is frontmatter” across the tool.
- **Determinism**: a single serialization strategy reduces diffs and makes builds easier to reason about.
- **Simpler refactors**: future Markdown AST work can operate on the body only, with frontmatter handled orthogonally.
- **Reduced duplication**: de-duplicates UID/fingerprint/lastmod helpers and parsing strategies.

## Costs and Risks

- **Migration work**: moving existing logic into a shared component requires careful testing.
- **Behavior changes**: unifying parsing rules may change edge-case handling (especially malformed frontmatter). This must be covered by tests.

## YAML-Only Policy

- If a document begins with `---`, it is treated as YAML frontmatter.
- TOML-style frontmatter (`+++`) and JSON frontmatter are **not** parsed by this component.
- If we encounter non-YAML frontmatter in inputs, the default behavior should be to treat it as “no frontmatter” for parsing purposes (and allow linting to report it if we want to enforce YAML-only in the docs corpus).

## Interaction with `mdfp` Fingerprinting

DocBuilder already uses `github.com/inful/mdfp` to verify and (optionally) rewrite documents to include an updated `fingerprint:` field.

Centralizing YAML frontmatter handling should make `mdfp` integration more reliable:

- **Preferred role split (Option 2)**: treat `mdfp` as the source of truth for computing the fingerprint value, and treat the frontmatter component as the source of truth for parsing/merging/writing YAML deterministically.
- **Avoid full-document rewrites**: prefer `mdfp.CalculateFingerprintFromParts(frontmatter, body)` (available in `mdfp v1.2.0`) over `mdfp.ProcessContent(...)`, then update only the YAML `fingerprint` field (and apply [ADR-011](adr-011-lastmod-on-fingerprint-change.md) logic for `lastmod`). This keeps diffs minimal and avoids unintended reformatting.
- **Compatibility fallback**: if we must use `mdfp.ProcessContent()` in some paths (for parity or speed of rollout), re-parse and re-merge via the centralized frontmatter component to preserve stable fields (e.g., `uid`, `aliases`, custom metadata).

This keeps the fingerprint algorithm centralized in `mdfp` while reducing duplicated “preserve UID” logic across packages.

### `mdfp` Support for Parts-Based Fingerprinting

As of `mdfp v1.2.0`, callers that already have `(frontmatter, body)` can compute the canonical fingerprint via:

- `mdfp.CalculateFingerprintFromParts(frontmatter, body)`

This dovetails directly with this ADR: DocBuilder can own YAML parsing/serialization (and minimal-diff edits), while `mdfp` owns the hashing semantics.

## Work Estimate (Order-of-Magnitude)

- **Small (1–3 days)**: Create the shared package and migrate one consumer (e.g., `internal/linkverify`).
- **Medium (3–7 days)**: Migrate linting/fixing frontmatter helpers and the build pipeline frontmatter transform.
- **Large (1–2+ weeks)**: Remove legacy helpers, standardize behaviors across all call sites, and add golden tests where output formats matter.

## Migration Plan

1. Implement `internal/frontmatter` (or similar) with split/parse/join + newline-style handling.
2. Migrate `internal/linkverify` parsing to the shared component.
3. Migrate build pipeline frontmatter parsing/serialization.
4. Migrate lint fixer helpers (UID insertion, lastmod updates) to build on the shared component.
5. Delete duplicated helpers and add regression tests.

## Acceptance Criteria

- All existing tests pass and new tests cover:
  - LF vs CRLF frontmatter
  - empty frontmatter
  - malformed frontmatter (no closing delimiter)
  - files without frontmatter
- Consumers agree on the same `had frontmatter` semantics.
- Output Markdown remains stable and deterministic across runs.

## Consequences

### Pros

- One place to evolve frontmatter policy.
- Lower risk when introducing AST-based Markdown parsing elsewhere.

### Cons

- Up-front refactor cost before other Markdown improvements.
