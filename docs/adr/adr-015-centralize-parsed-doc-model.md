---
aliases:
  - /_uid/4b11a5c2-8bcb-4fd0-9b0e-1c5e9a7c2d1b/
categories:
  - architecture-decisions
date: 2026-01-21T00:00:00Z
fingerprint: 7319e55ba9f5655f635dd228e8af463efad796e554ed246620b4580509b1b6b5
lastmod: "2026-01-22"
tags:
  - markdown
  - frontmatter
  - parsing
  - linting
  - performance
  - refactor
uid: 4b11a5c2-8bcb-4fd0-9b0e-1c5e9a7c2d1b
---

# ADR-015: Centralize parsed document model (frontmatter + Markdown body)

**Status**: Proposed  
**Date**: 2026-01-21  
**Decision Makers**: DocBuilder Core Team

## Context and Problem Statement

DocBuilder now has centralized *parsing primitives*:

- YAML frontmatter split/parse/write via `internal/frontmatter` ([ADR-014](adr-014-centralize-frontmatter-parsing-and-writing.md))
- Markdown body parsing and link extraction via `internal/markdown` (Goldmark) ([ADR-013](adr-013-goldmark-for-internal-markdown-parsing.md))

However, multiple subsystems still independently repeat the same “parse the document” workflow:

- read file bytes
- split frontmatter/body
- parse frontmatter to a map
- parse Markdown body to an AST (sometimes)
- extract links / compute skippable ranges / line mapping
- apply edits and re-join frontmatter/body

This duplication exists across linting, fixing, link verification, and (potentially) future transforms.

### Why this is a problem

Even with centralized *helpers*, duplicating the workflow at call sites has costs:

- **Inconsistent derived metadata**: line-number attribution, code-block skipping rules, and “what counts as a link” may drift.
- **Repeated work**: the same document can be split and parsed multiple times within one run (especially in fix flows).
- **Harder feature work**: future analyzers (e.g., “extract all headings”, “extract code fences”, “extract internal anchors”) risk re-implementing parsing and bookkeeping.
- **Unclear ownership**: it’s easy to add “just one more” ad-hoc scan in a consumer instead of extending a shared model.

The project is already pursuing “minimal surprise” and “minimal-diff” updates (byte-range edits over re-rendering Markdown). A shared parsed document model provides a consistent foundation for that approach.

## Decision

Introduce a shared internal *parsed document model* that represents a Markdown file as:

- original bytes
- frontmatter bytes + structured frontmatter fields (YAML)
- body bytes
- optional Markdown AST
- optional extracted link/index metadata

Consumers (lint, fixer, linkverify, future transforms) will use this model instead of re-running split/parse/extract logic ad-hoc.

This ADR intentionally distinguishes:

- **Centralized implementation** (already done: `internal/frontmatter`, `internal/markdown`), from
- **Centralized workflow ownership** (this ADR: “parse once, reuse everywhere”).

## Non-Goals

- Introducing a new “universal document IR” that replaces the pipeline model.
- Re-rendering Markdown from an AST (we continue to prefer minimal-diff byte edits).
- Global caching across multiple DocBuilder runs (cache is per-run).
- Adding multi-theme behavior (DocBuilder is Relearn-only).

## Proposed API Shape (internal)

A small, composable API focused on correctness and reuse:

- `Parse(content []byte, opts Options) (*ParsedDoc, error)`
- `ParseFile(path string, opts Options) (*ParsedDoc, error)`

Where `ParsedDoc` exposes:

- `Original() []byte`
- `Frontmatter() (raw []byte, fields map[string]any, had bool, style frontmatter.Style)`
- `Body() []byte`
- `AST() (*ast.Node, bool)` (lazily built)
- `Links() ([]markdown.Link, error)` (lazily extracted; uses existing `internal/markdown.ExtractLinks`)
- `ApplyEdits(edits []markdown.Edit) ([]byte, error)` or `ApplyBodyEdits(...)` + re-join

Options allow consumers to pay only for what they need:

- `WithFrontmatterFields bool`
- `WithAST bool`
- `WithLinks bool`

### Location

Preferred: a new package such as `internal/docmodel` or `internal/document`.

- Avoids turning `internal/markdown` into a “god package”.
- Keeps `internal/frontmatter` and `internal/markdown` as focused building blocks.

(Exact naming is an implementation detail; acceptance criteria focuses on behavior and dependency boundaries.)

### Caching

Optionally introduce a per-run cache keyed by absolute path + content hash (or mtime+size where safe):

- speeds up workflows that parse the same files multiple times
- prevents subtle drift in derived metadata

Cache invalidation must be explicit: if a fixer rewrites a file, it must either bypass cache or update cache entries.

## Options Considered

### Option A: Keep workflow duplication (status quo)

- Continue to call `frontmatter.Split` + `markdown.ExtractLinks` from each consumer.
- Allow each subsystem to manage line mapping and “skip code” logic.

### Option B: Centralize parsed document model (this ADR)

- Add a `ParsedDoc` model built from existing primitives.
- Provide lazy AST/link extraction and shared line mapping.

### Option C: Push everything into `internal/markdown`

- Provide `markdown.ParseDocument` that includes frontmatter splitting and caching.

Rejected as the primary direction: `internal/markdown` already holds AST + edits; mixing frontmatter handling and caching there risks broadening that package’s responsibility.

## Benefits

- **Consistency**: one “document boundary” and one set of derived metadata (links, line mapping, skippable regions).
- **Performance**: avoid repeated parse work during fix flows.
- **Extensibility**: new analyzers can be implemented by extending the model (or adding indexed views) rather than re-parsing.
- **Safer edits**: a single join path reduces the risk of frontmatter/body boundary mistakes.

## Costs and Risks

- **API surface area**: a doc model must avoid becoming overly generic.
- **Caching correctness**: stale-cache bugs are costly; cache must be per-run and invalidation must be well-defined.
- **Dependency coupling**: placing the doc model in the wrong package can create unwanted dependencies (e.g., `internal/docs` shouldn’t necessarily depend on Goldmark).

## Migration Plan

1. Introduce `ParsedDoc` + parsing helpers as a new package.
2. Migrate one consumer first (recommended: lint/fixer path, where we already depend on frontmatter split + link extraction).
3. Add regression tests around:
   - correct frontmatter/body splitting
   - stable join behavior
   - link extraction parity, including permissive whitespace destinations
   - correct line-number attribution (including fenced/inline code skipping)
4. Migrate remaining consumers (linkverify, future transforms).
5. (Optional) Introduce per-run caching once the model is stable and well-covered by tests.

## Acceptance Criteria

- Consumers that need the same metadata (links/line mapping) get identical results for identical inputs.
- Existing link update behavior remains “minimal diff” (byte-range edits), and no Markdown re-rendering is introduced.
- No new parsing libraries are added (Goldmark remains the Markdown engine).
- All tests pass, and new tests cover at least one multi-consumer scenario to prevent workflow drift.

## Consequences

### Pros

- Simplifies future Markdown-aware features.
- Reduces duplicated parsing workflow code.
- Makes caching possible without each consumer reinventing it.

### Cons

- Adds a new internal package that must be maintained.
- Requires careful dependency management to avoid import cycles and inappropriate coupling.
