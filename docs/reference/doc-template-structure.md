---
aliases:
  - /_uid/09e59ad9-520f-4fcf-bdf4-de00f35115f1/
categories:
  - reference
fingerprint: 6368f7a14bf0a7a831d6df2275694ff806d3514d8be2eb56c6f08e7d07a44aee
lastmod: "2026-09-25"
tags:
  - templates
  - authoring
  - schema
title: Documentation Template Structure
uid: 09e59ad9-520f-4fcf-bdf4-de00f35115f1
---

# Documentation Template Structure

A **template** is a regular documentation page that, when invoked,
prompts the user for input and writes a new doc under `docs/` based on
a Go-template body. Templates are authored and published like any other
doc; the doctemplate discovery step picks them up from the
`/categories/templates/` taxonomy page.

This page is the canonical schema for what a template looks like. The
narrative how-to guide lives in
[`docs/how-to/author-templates.md`](../how-to/author-templates.md).

## Overview

A template document has three parts:

````text
---                                    ← frontmatter (with template meta)
title: "ADR Template"
categories:
  - Templates
params:
  docbuilder:
    template:
      type: "adr"
      ...
---

# Architecture Decision Record Template   ← human-readable description

A short note about when to use this template.

--- (excerpt of inner markdown body) ---
````

The body is the **only** fenced markdown code block in the document; the
parser fails the template if there are zero or multiple blocks. The block
must use `language-markdown` (or `language-md`) on the fence.

## Template Frontmatter

The template configuration lives under `params.docbuilder.template.*`
in the page frontmatter.

### Required fields

| Field | Type | Notes |
|---|---|---|
| `params.docbuilder.template.type` | string | Canonical identifier (e.g. `adr`, `guide`). Used in default output paths and sequence lookups. |
| `params.docbuilder.template.name` | string | Human-friendly display name shown in template lists. |
| `params.docbuilder.template.output_path` | string | Go-template path relative to `docs/`. See *Output path templating* below. |

### Optional fields

| Field | Type | Notes |
|---|---|---|
| `params.docbuilder.template.description` | string | Short description shown in template pickers. |
| `params.docbuilder.template.schema` | JSON string | Input fields and types. See *Schema fields* below. |
| `params.docbuilder.template.defaults` | JSON string | Default values for fields. Used when `--defaults` is set or a non-required field is left empty. |
| `params.docbuilder.template.sequence` | JSON string | Sequential-numbering configuration. See *Sequences* below. |

## Schema fields

`params.docbuilder.template.schema` is a JSON object with a `fields`
array. Each field has the following shape:

```json
{
  "key": "FieldName",
  "type": "string | string_enum | string_list | bool",
  "required": true,
  "options": ["a", "b", "c"],
  "glob-suggestion": "guides/*/"
}
```

| Property | Type | Required when | Notes |
|---|---|---|---|
| `key` | string | always | Variable name used in `{{ .Key }}` references in the body. |
| `type` | string | always | One of the four field types below. |
| `required` | bool | optional, default false | If true, the field must be provided. |
| `options` | list[string] | when `type` is `string_enum` | Allowed values. |
| `glob-suggestion` | string | optional | Used by `doctemplate` TUI to pre-fill from the local filesystem. See below. |

### Field types

| Type | Input | Example output |
|---|---|---|
| `string` | free text | `My new ADR` |
| `string_enum` | pick from `options` | `getting-started` |
| `string_list` | comma-separated | `api, reference, v2` |
| `bool` | one of `true`/`false`/`t`/`f`/`1`/`0`/`TRUE`/`FALSE`/`True`/`False`/`T`/`F` | `true` |

### Glob suggestions

When `doctemplate` runs in interactive mode it can suggest values
based on the existing docs tree. The patterns:

| Pattern | Suggests | Example inputs → suggestions |
|---|---|---|
| `dir/*.md` | file stems under `dir/` | `dir/foo.md`, `dir/bar.md` → `foo`, `bar` |
| `dir/*/` | direct child directory names | `dir/one/x.md`, `dir/two/y.md`, `dir/file.md` → `one`, `two` |

Patterns are docs-relative and cannot escape `docs/`.

## Output path templating

`params.docbuilder.template.output_path` is a Go `text/template` string
evaluated against the resolved field values to produce the target path
relative to `docs/`. Available variables and helpers:

| Token | Expands to |
|---|---|
| `{{ .FieldName }}` | The value of schema field `FieldName`. |
| `{{ index .categories 0 }}` | First element of the `categories` list (commonly used to inject the user-picked category into the generated doc's frontmatter). |
| `{{ printf "%03d" N }}` | Zero-padded number, e.g. `001`. |
| `{{ nextInSequence "name" }}` | Next integer for the named sequence (see below). |

A typical ADR template's output path:

```text
adr/adr-{{ printf "%03d" (nextInSequence "adr") }}-{{ .Slug }}.md
```

Generates paths like `adr/adr-001-first-decision.md`. Paths must be
relative to `docs/` — `WriteGeneratedFile` rejects absolute paths and
`..` traversal.

## Sequences

Sequences give a template auto-incrementing numbers (ADR-001, ADR-002,
…). Configuration lives under `params.docbuilder.template.sequence`:

```yaml
params:
  docbuilder:
    template:
      sequence:
        name: "adr"           # identifier used by nextInSequence "adr"
        dir: "adr"            # directory to scan, relative to docs/
        glob: "adr-*.md"      # file pattern to count
        regex: "^adr-(\\d{3})-" # extract number; exactly one capture group required
        width: 3              # display width (optional, for printf padding)
        start: 1              # starting number if no matches (optional, default 1)
```

Given existing files `adr-001-first.md`, `adr-003-third.md`,
`adr-010-tenth.md`, the next number is `011` (max + 1).

The template for `type: adr` automatically gets an `adr` sequence
defined (`Glob: adr-*.md`, `Regex: ^adr-(\d{3})-`, `Width: 3`,
`Start: 1`) if no explicit sequence is configured. Other types must
define one explicitly if they want sequential numbering.

## Body block

The template body is a single fenced markdown code block. Rules:

- Exactly **one** code block per template document.
- Fence language: `markdown` (alias `md` also accepted).
- Content is Go `text/template` syntax, evaluated with the resolved
  field values plus sequence helpers.
- The body usually contains a YAML frontmatter block (with `title`,
  `date`, etc.) and the markdown content of the generated doc.

Example body:

````markdown
---
title: "{{ .Title }}"
categories:
  - {{ index .categories 0 }}
date: 2026-09-25T00:00:00Z
---

# {{ .Title }}

## Context and Problem Statement

{{ .Context }}
````

## Discovery

Templates are discovered by parsing the published documentation site's
`/categories/templates/` taxonomy page. The discovery code:

1. Fetches `<baseURL>/categories/templates/`.
2. Finds anchor tags whose `href` contains `.template/`.
3. Extracts the template type from the link text or the path
   (strips the `.template` suffix).
4. For each match, parses the template page's
   `docbuilder:template.*` meta tags to populate `TemplateMeta`.

Make sure your docs site is built and published with templates
categorized under `Templates` for discovery to find them.

## Worked Example

A complete minimal template:

````markdown
---
title: "Quick Guide Template"
categories:
  - Templates
params:
  docbuilder:
    template:
      type: "guide"
      name: "Quick Guide"
      description: "Create a quick how-to guide"
      output_path: "guides/{{ .Slug }}.md"
      schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'
      defaults: '{"tags":["guide"]}'
---

# Quick Guide Template

Use this template to create a new how-to guide.
````

The body of this template is the fenced block above; it contains
placeholders for `{{ .Title }}` and `{{ .Slug }}` that the user fills
in at prompt time.

After saving, `docbuilder build` makes the page available at
`/categories/templates/`. From then on, `docbuilder template list
--base-url <site>` will return `guide`, and
`docbuilder template new --base-url <site>` will offer it.

## Verification Workflow

1. **Author the template** in `docs/templates/<type>.template.md`.
2. **Build** the docs site (`docbuilder build`).
3. **Verify discovery**: `docbuilder template list --base-url <site>`
   shows your template.
4. **Dry-run the schema**: `docbuilder template new --base-url <site>
   --set Title="Test" --set Slug="test" --defaults --yes` writes a
   real file you can inspect.
5. **Lint the generated doc**: `docbuilder lint <generated-path>` to
   confirm the template produces valid output.

## Common Pitfalls

- **Multiple or zero fenced markdown blocks** in the template body —
  the parser requires exactly one.
- **`output_path` is absolute or escapes `docs/`** — `WriteGeneratedFile`
  refuses; you'll see a path-traversal error.
- **`sequence.regex` has zero or more than one capture group** — the
  next-number computation fails.
- **`sequence.dir` is not under `docs/`** — silently ignored or
  produces 1.
- **Schema `key` doesn't match a `{{ .Key }}` reference in the body** —
  field appears unused; the prompt is wasted.
- **Template not categorized as `Templates`** — discovery won't pick
  it up; check `/categories/templates/` in the rendered site.
- **Template URL contains `..` or absolute paths** — discovery's
  HTTP client rejects the request for safety.

## See Also

- [Authoring Templates](../how-to/author-templates.md) — narrative
  how-to with full examples.
- [Using Templates](../how-to/use-templates.md) — invoking templates
  from the CLI and from MCP.
- [Doc Page Structure](doc-page-structure.md) — the structure of the
  pages that templates generate.
- [CLI Reference](cli.md) — `docbuilder template` subcommands.
- [ADR-022](../adr/adr-022-cli-template-based-markdown-generation.md) —
  technical specification of the template system.
