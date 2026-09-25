---
aliases:
  - /_uid/c941e505-3989-41d2-85a2-e51318114c80/
categories:
  - reference
fingerprint: c5585911e3abaeb1cf4bf596356ff112357fd57b401c1d56d148ae06b8f0fe1a
lastmod: "2026-09-25"
tags:
  - frontmatter
  - structure
  - authoring
title: Documentation Page Structure
uid: c941e505-3989-41d2-85a2-e51318114c80
---

# Documentation Page Structure

This page describes what a valid docbuilder documentation page looks like:
the required and optional frontmatter fields, body rules, and filename
conventions. Templates are documented separately — see
[Authoring Templates](../how-to/author-templates.md).

## Overview

A docbuilder page is a Markdown file (`*.md` or `*.markdown`) inside the
configured docs directory (default `./docs`). Each file has two parts:

```text
---                          ← frontmatter delimiter (opens)
title: My Page
uid: ...
date: 2026-01-01T00:00:00Z
...                          ← YAML frontmatter
---
Markdown body here.          ← body (everything after the closing `---`)
```

The frontmatter is parsed as YAML; the body is Markdown (Goldmark by
default). The lint rules in [`lint_rules.md`](lint-rules.md) enforce most
of what's below — `docbuilder lint` is the canonical verifier.

## Required Frontmatter

These fields **must** be present on every page. The lint tool reports an
ERROR if any is missing or malformed.

| Field | Type | Format | Purpose |
|---|---|---|---|
| `title` | string | Free text | Page title. Shown in the rendered site, browser tab, and search results. |
| `uid` | string | UUID (RFC 4122) | Stable identifier for cross-references and aliases. Generate one with `uuidgen` or any tool that emits v4 UUIDs. |
| `date` | string | ISO 8601 (`YYYY-MM-DDTHH:MM:SSZ`) | Creation timestamp. Used for sorting, feeds, and Hugo's date taxonomy. |
| `lastmod` | string | `YYYY-MM-DD` | Last-modified date. Lint updates this automatically when the fingerprint changes. |
| `fingerprint` | string | SHA-256 hex (64 chars) | Content hash excluding `fingerprint`/`lastmod`/`uid`/`aliases`. Auto-managed; **never edit by hand** — use `docbuilder lint --fix`. |
| `categories` | list of strings | Single-word slugs | Hugo taxonomy. Drives the sidebar and the `/categories/<slug>/` index pages. Common values: `tutorial`, `how-to`, `reference`, `explanation`, `adr`. |
| `tags` | list of strings | Kebab-case slugs | Hugo taxonomy. Free-form; multiple values common. |
| `aliases` | list of strings | URL paths starting with `/_uid/` or `/` | Legacy or alternative URLs the page should also be reachable at. |

## Optional Frontmatter

These fields are read by Hugo or by docbuilder but are not enforced by
the lint rules.

| Field | Type | Purpose |
|---|---|---|
| `weight` | int | Ordering weight within a section. Lower numbers appear first. |
| `draft` | bool | If `true`, the page is excluded from production builds. |
| `description` | string | Short summary used by Hugo's meta tags and search. |
| `keywords` | list of strings | Additional taxonomy. Rarely needed alongside `tags`. |

## Build-Injected Frontmatter

These fields are added by docbuilder during the build — you should
**not** set them yourself; they will be overwritten.

| Field | Set to |
|---|---|
| `repository` | The repository slug the page came from. |
| `forge` | The forge name (GitHub, GitLab, Forgejo, local). |
| `section` | The Hugo content section the page was placed in. |
| `edit_url` | URL to edit the page in the source forge (when `edit_url_base` is configured). |

## Body Rules

The body is Markdown. The lint tool checks:

- **Frontmatter delimiter**: must be exactly `---\n` at the start and `\n---` to close. CRLF line endings are accepted but normalized to LF.
- **Internal links**: any `[text](relative/path.md)` must resolve to an existing file inside the docs tree. Broken links are reported as ERRORs.
- **Headings**: H1 is the page title (don't repeat it as `# Title` in the body — the `title` frontmatter field is used). Start with H2 (`##`) or deeper.
- **Code blocks**: fenced blocks with a language tag are rendered with syntax highlighting. For embedded templates, use `language-markdown` on the fenced block.
- **No raw HTML restrictions** beyond what Hugo's renderer applies.

## Filename Conventions

Enforced by the `filename-conventions` lint rule:

- File extension: `.md` or `.markdown`.
- Lowercase only. `My-Page.md` is an ERROR.
- ASCII letters, digits, hyphens, underscores, and dots only.
- **Sequence prefixes** are conventional (not enforced): `adr-NNN-slug.md`,
  `tutorial-NN-slug.md`. The `NNN` is zero-padded. The linter doesn't
  enforce sequence numbering — use a template's `sequence` definition
  for that (see Authoring Templates).

Example valid filenames:

```text
docs/
├── index.md
├── getting-started.md
├── how-to/
│   ├── use-templates.md
│   └── migrate-project-to-taxonomy.md
├── reference/
│   ├── cli.md
│   └── doc-page-structure.md      ← you are here
└── adr/
    ├── adr-001-initial-architecture.md
    └── adr-002-replace-cron-scheduling.md
```

## Worked Example

A minimal but complete page:

```markdown
---
title: My New Guide
uid: 7f1c2a3b-9d4e-4a5b-8c6f-2e1a0b3c4d5e
date: 2026-09-25T00:00:00Z
lastmod: "2026-09-25"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - how-to
tags:
  - example
  - onboarding
aliases:
  - /_uid/my-new-guide/
weight: 10
---

# H1 is auto-generated from `title`; do not repeat it in the body.

Start with an H2.

## A section

Body content goes here.

## Another section

You can use code blocks:

​```go
package main

func main() {
    println("hello")
}
​```

Links to other docs use relative paths: [CLI Reference](cli.md).
Links to external sites use absolute URLs: [Hugo](https://gohugo.io/).
```

Before saving, the **fingerprint** must be computed correctly. Run
`docbuilder lint --fix` (or use the MCP `lint_fix` tool) to populate it;
do not type it by hand.

## Verifying a Page

After authoring:

1. **Lint it**: `docbuilder lint path/to/page.md` (or `lint_docs` over MCP).
   The lint tool reports all violations of the rules above.
2. **Apply fixes**: `docbuilder lint --fix path/to/page.md` (or `lint_fix`
   over MCP) to update the fingerprint and lastmod automatically.
3. **Re-lint**: confirm the file passes with zero errors.
4. **Preview** (optional): `docbuilder preview` to render the page in a
   browser and verify layout.

## Common Mistakes

- **Missing `uid`**: causes the `frontmatter-uid` lint ERROR. Generate
  one with `uuidgen` and add it.
- **Stale `fingerprint`**: causes the `frontmatter-fingerprint` lint
  ERROR. Run `docbuilder lint --fix` to recompute.
- **Uppercase or spaces in filename**: causes the `filename-conventions`
  lint ERROR. Rename to lowercase-hyphenated form.
- **Broken internal link**: caused by `[text](relative/path.md)` where
  the file doesn't exist. Either create the target or fix the path.
- **Repeating the title as H1 in the body**: the `title` frontmatter
  field already drives the rendered H1; an H1 in the body creates a
  duplicate heading.

## See Also

- [Authoring Templates](../how-to/author-templates.md) — defining your
  own templates that produce pages matching this structure.
- [CLI Reference](cli.md) — full command reference, including
  `docbuilder lint` and `docbuilder lint --fix`.
- [Lint Rules](lint-rules.md) — every lint rule and what it enforces.
