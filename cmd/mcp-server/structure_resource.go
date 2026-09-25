package main

// docPageStructureSchema is the compact, structured description of what
// a valid docbuilder documentation page looks like. It is exposed as an
// MCP resource ('structure://doc-page') so LLMs can fetch the canonical
// shape of a page before calling create_doc / create_from_template.
//
// The full human-readable version lives in
// 'docs/reference/doc-page-structure.md'. The test
// 'TestDocPageStructure_StaysInSyncWithDoc' ensures this const stays in
// step with the file; if you intentionally change one, update the other.
//
// Note: this string contains no backticks because Go raw strings cannot.
// Single quotes are used for inline code references — the LLM parses
// this as plain text, not Markdown.
const docPageStructureSchema = `# DocBuilder Documentation Page Structure

Every page is a Markdown file (extension .md or .markdown) inside the
configured docs directory.

## Required Frontmatter

| Field | Type | Format | Notes |
|---|---|---|---|
| title | string | free text | Page title; rendered as H1 |
| uid | string | UUID v4 | Stable cross-reference id |
| date | string | RFC 3339 / ISO 8601 | Creation timestamp |
| lastmod | string | YYYY-MM-DD | Auto-managed by lint_fix |
| fingerprint | string | SHA-256 hex (64 chars) | Auto-managed; never edit by hand |
| categories | list[string] | single-word slugs | Hugo taxonomy; common: how-to, reference, explanation, tutorial, adr |
| tags | list[string] | kebab-case slugs | Free-form taxonomy |
| aliases | list[string] | URL paths starting with /_uid/ or / | Legacy or alternative URLs |

## Optional Frontmatter

| Field | Type | Notes |
|---|---|---|
| weight | int | Ordering within a section; lower numbers appear first |
| draft | bool | If true, excluded from production builds |
| description | string | Meta summary; used by search |
| keywords | list[string] | Additional taxonomy; rare |

## Build-Injected (do NOT set these)

repository, forge, section, edit_url — overwritten on every build.

## Body Rules

- Markdown only (Goldmark).
- Frontmatter delimiters: '---' to open, '---' to close.
- H1 in body: do NOT repeat the title; the 'title' frontmatter field
  drives the rendered H1. Start with H2.
- Internal links (relative .md paths) must resolve to existing files.
- Code blocks: fenced with a language tag (e.g. 'go', 'markdown'). For
  embedded templates use 'language-markdown'.
- No raw HTML restrictions beyond what Hugo's renderer applies.

## Filename Rules

- Lowercase only.
- Extension: .md or .markdown.
- Allowed characters: ASCII letters, digits, hyphens, underscores, dots.
- Conventional sequence prefixes (not enforced by lint): adr-NNN-slug.md,
  tutorial-NN-slug.md with zero-padded numbers.

## Minimal Worked Example

---
title: My Guide
uid: 7f1c2a3b-9d4e-4a5b-8c6f-2e1a0b3c4d5e
date: 2026-09-25T00:00:00Z
lastmod: "2026-09-25"
fingerprint: AUTO_GENERATED_BY_LINT_FIX
categories:
  - how-to
tags:
  - example
aliases:
  - /_uid/my-guide/
weight: 10
---

## Overview

Start with an H2. The title frontmatter field becomes the H1.

## A section

Body text. Use [relative links](other-page.md) for other docs and
absolute URLs for external sites.

## Verification Workflow

1. After writing, run 'docbuilder lint path/to/page.md' (or the MCP
   'lint_docs' tool) — it reports all frontmatter / filename / link
   violations.
2. Run 'docbuilder lint --fix' (or MCP 'lint_fix') — it populates
   'fingerprint' and updates 'lastmod'.
3. Re-run lint — must report zero errors.

## Common Pitfalls

- Missing 'uid' → ERROR. Generate with 'uuidgen'.
- Wrong 'fingerprint' → ERROR. Run 'lint --fix'; never type it by hand.
- Uppercase or spaces in filename → ERROR. Rename to lowercase-hyphenated.
- Broken internal link → ERROR. Fix path or create target.
- H1 in body duplicates the rendered title.
`
