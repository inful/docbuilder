---
aliases:
  - /_uid/1f1a9e2c-3a7e-4d8f-b35e-60c9d78d0a4c/
categories:
  - architecture-decisions
date: 2026-01-20T00:00:00Z
fingerprint: ecbafb24b55170dbaab1bc19a9d81c7668f369088f6490bfd9b4a2c6a969d0a3
lastmod: "2026-01-22"
tags:
  - markdown
  - parsing
  - linting
  - links
  - hugo
uid: 1f1a9e2c-3a7e-4d8f-b35e-60c9d78d0a4c
---

# ADR-013: Use Goldmark for internal Markdown parsing

**Status**: Proposed  
**Date**: 2026-01-20  
**Decision Makers**: DocBuilder Core Team

## Context and Problem Statement

DocBuilder performs a non-trivial amount of Markdown-aware work outside of Hugo itself, including (but not limited to):

- Broken-link detection during `docbuilder lint` (see the ad-hoc scanners in `internal/lint/*link*`)
- Link discovery and rewriting during fixes (e.g. link healing from [ADR-012](adr-012-autoheal-links-to-moved-files.md))
- Content transformations and feature-specific Markdown behaviors ([ADR-004](adr-004-forge-specific-markdown.md))

Today, most of this logic uses hand-rolled scanning (string search + heuristics like “skip fenced blocks” and “skip inline code”). This has several known risks:

- **Correctness drift**: our interpretation of “what is a link / code / text” may differ from what Hugo renders.
- **Edge cases**: nested parentheses, reference-style link nuances, autolinks, escaped characters, and fenced-code variants are easy to miss.
- **Duplication**: multiple internal components implement similar parsing rules.
- **Maintainability**: adding new Markdown-aware features tends to expand ad-hoc parsing.

Hugo itself uses Goldmark for Markdown rendering (and DocBuilder already configures Hugo’s `markup.goldmark` settings in the generated config), so adopting Goldmark internally may also reduce semantic mismatch.

Frontmatter parsing/writing is now centralized and treated as **YAML-only** using `---` delimiters (see [ADR-014](adr-014-centralize-frontmatter-parsing-and-writing.md)). This ADR focuses on Markdown **body** parsing; frontmatter remains a dedicated concern.

## Decision

Introduce a single, shared internal Markdown parsing layer based on **Goldmark** (CommonMark-oriented parser) for DocBuilder operations that require structural understanding of Markdown.

- Goldmark will be used for **analysis** (link discovery, code-block skipping, structured transforms), not for generating Hugo-rendered HTML.
- Migration will be **incremental**, starting with link discovery/broken-link detection where correctness benefits are highest.
- Any behavior that must align with Hugo should aim to mirror Hugo’s Goldmark configuration where relevant.
- Goldmark parsing should operate on the Markdown **body only** (split from YAML frontmatter via the centralized frontmatter component from ADR-014, implemented as `internal/frontmatter`).
- For link rewriting, prefer **minimal-diff edits** (byte-range patches targeted to link destinations/definitions) to avoid reformatting and minimize surprise.

## Options Considered

### Option A: Keep current ad-hoc parsing (status quo)

- Continue using string scanning / regex / heuristics.
- Incrementally patch edge cases as bugs arise.

### Option B: Adopt Goldmark for internal parsing (this ADR)

- Parse Markdown into an AST using Goldmark.
- Implement link discovery and transform logic by visiting AST nodes.

### Option C: Adopt a different Markdown parser

- Alternatives exist (e.g. Blackfriday, gomarkdown/markdown).
- Hugo’s default engine is Goldmark; choosing a different parser increases the risk of divergence.

## Benefits

### 1. Better correctness and coverage

Goldmark handles many Markdown details that are difficult to reproduce reliably with scanning:

- Fenced code blocks (including language info strings), inline code spans, and block structure
- Reference-style links vs inline links
- Escaping rules and nested constructs

This directly improves link detection and rewrite safety.

### 2. Alignment with Hugo semantics

DocBuilder already configures Hugo’s Goldmark settings (e.g., `renderer.unsafe`, attribute blocks, passthrough for math). Using the same parsing engine internally reduces “DocBuilder says it’s a link / Hugo renders it differently” mismatches.

### 3. Reduced duplication and simpler feature work

A shared AST-based approach can replace multiple bespoke scanners. New Markdown-aware linting and transforms can be implemented as AST visitors rather than additional regex rules.

### 4. Better safety and auditability

Centralizing Markdown parsing means:

- one place to reason about which constructs are in-scope for rewrites
- one set of tests for edge cases
- clearer boundaries between “content parsing” and “text rewriting”

## Costs and Risks

### 1. Round-trip rewriting is non-trivial

Goldmark is excellent for parsing to an AST and rendering to HTML, but **it does not ship a built-in “render Markdown back out while preserving the original formatting / minimizing diffs”**.

This does not prevent round-trip edits. It does mean we should avoid “AST → re-render Markdown” approaches unless we explicitly accept normalized output.

For operations like “rewrite only the link target but keep the original formatting”, we can instead:

- use the AST to locate the exact source ranges for link destinations (byte offsets / segments), and
- apply targeted byte-range replacements to the original source content.

This is doable (and keeps diffs small), but it is more complex than line-based string replacement.

### 2. Behavioral changes / mismatch risk still exists

Even with the same parser, Hugo’s configuration (enabled extensions, renderer settings) affects interpretation. For internal link discovery, most of this is irrelevant, but for any transform that depends on extension syntax, we must explicitly decide which Goldmark extensions to enable.

### 3. Dependency and learning curve

Adding Goldmark introduces:

- a new dependency and versioning surface
- some team ramp-up on Goldmark AST APIs

## Work Estimate (Order-of-Magnitude)

Because DocBuilder already has extensive unit tests around link detection and link update behavior, we can migrate safely in steps.

### Small (1–3 days): Goldmark-based link *discovery* only

- Create `internal/markdown` package that parses content and returns link nodes
- Wire `docbuilder lint` broken-link detection to use Goldmark parsing
- Keep the existing link rewrite approach unchanged for now
- Add edge-case tests (nested parens, code fences, escaped brackets)

### Medium (1–2 weeks): Goldmark-based link rewriting with source ranges

- Replace “line-based replace” with “range-based replace” using Goldmark node segments
- Support inline links, image links, and reference-style link definitions
- Preserve fragments (`#...`) and path style (`../`, `./`)
- Expand test suite to cover mixed content and multiple links per line

### Large (2–4+ weeks): Consolidate all Markdown-aware transforms

- Migrate any forge-specific Markdown transforms to AST visitors where appropriate
- Remove duplicated parsing utilities and unify behavior
- Add golden tests for transform outputs where needed

## Migration Plan

0. **(Done via ADR-014)** Use centralized frontmatter splitting/parsing/writing (via `internal/frontmatter`) so Markdown-body parsing operates on the body only ([ADR-014](adr-014-centralize-frontmatter-parsing-and-writing.md)).
1. **Introduce a new internal package** `internal/markdown`:
   - `Parse(source []byte) (*ast.Node, error)` wrapper
   - Visitors for “extract links” and “extract reference definitions”
   - Clear decisions about which extensions are enabled
2. **Swap broken-link detection** to use this package.
3. **Adopt AST-driven link discovery** for fixer operations (used by link healing).
4. **Implement minimal-diff link rewriting**:
  - Use Goldmark AST nodes/segments to locate link destinations and reference definitions.
  - Apply targeted byte-range patches to the original source to keep diffs small.
5. **Delete duplicated scanners** once parity is achieved.

## Acceptance Criteria

- Broken-link detection is at least as accurate as today and improves edge-case handling.
- No regression in performance beyond an acceptable bound for typical docs repositories.
- Link-healing and link-update features remain deterministic and test-covered.
- Hugo site generation behavior is unchanged (this ADR only targets DocBuilder’s internal parsing).

## Consequences

### Pros

- More correct Markdown interpretation for linting and transforms.
- Less duplicated parsing logic and fewer “regex fights”.
- Better long-term foundation for future Markdown-aware features.

### Cons

- Migration cost, especially for safe round-trip rewrites.
- Adds a new internal parsing subsystem (and Goldmark dependency) that must be maintained/versioned, even though it centralizes Markdown-aware behavior and makes it easier to reason about.

## Open Questions

(None at this time.)

## Resolved

- Internal parsing should intentionally match the DocBuilder-generated Hugo Goldmark configuration (the effective configuration Hugo renders with), including enabling the same Goldmark extensions/settings configured in `markup.goldmark`.
- For link rewriting, we prefer minimal-diff edits (byte-range patches) over re-rendering/normalizing Markdown output.
