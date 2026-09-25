package main

// docTemplateStructureSchema is the compact, structured description of
// what a valid docbuilder template looks like. It is exposed as an MCP
// resource ('structure://template') so LLMs that are authoring templates
// (or invoking them via create_from_template) can fetch the canonical
// shape.
//
// The full human-readable counterpart lives in
// 'docs/reference/doc-template-structure.md'. Test
// 'TestDocTemplateStructure_StaysInSyncWithDoc' ensures this const stays
// in step with the file.
//
// Note: Go raw strings cannot contain backticks. Single quotes are used
// for inline code references — the LLM parses this as plain text, not
// Markdown.
const docTemplateStructureSchema = `# DocBuilder Documentation Template Structure

A template is a regular documentation page (file ending in .template.md,
under docs/) that, when invoked, prompts the user and writes a new doc
based on a Go-template body.

## Three Parts of a Template Document

1. Frontmatter (with template config under params.docbuilder.template.*)
2. Optional human-readable description
3. Exactly one fenced markdown code block (language: markdown or md)
   holding the Go-template body

## Required Template Frontmatter

Under params.docbuilder.template.*:

| Field | Type | Notes |
|---|---|---|
| type | string | Canonical identifier (e.g. adr, guide) |
| name | string | Human-friendly display name |
| output_path | string | Go-template path relative to docs/ |

## Optional Template Frontmatter

| Field | Type | Notes |
|---|---|---|
| description | string | Short blurb for template pickers |
| schema | JSON string | Input field definitions |
| defaults | JSON string | Default field values |
| sequence | JSON string | Sequential numbering config |

## Schema Field Types (schema.fields[].type)

| Type | Notes |
|---|---|
| string | Free text input |
| string_enum | Pick from options[] (required) |
| string_list | Comma-separated list |
| bool | One of true/false/t/f/1/0/TRUE/FALSE/True/False/T/F |

## Schema Field Properties

| Property | Required when | Notes |
|---|---|---|
| key | always | Variable name (used in {{ .Key }}) |
| type | always | One of the four types above |
| required | optional, default false | Must be provided |
| options | type=string_enum | Allowed values |
| glob-suggestion | optional | Patterns: dir/*.md (file stems) or dir/*/ (dir names) |

## Output Path Template Variables

| Token | Expands to |
|---|---|
| {{ .FieldName }} | Schema field value |
| {{ index .categories 0 }} | First categories entry |
| {{ printf "%03d" N }} | Zero-padded number |
| {{ nextInSequence "name" }} | Next integer for a sequence |

Paths must be relative to docs/; absolute paths and .. are rejected.

## Sequence Configuration (sequence.*)

| Field | Notes |
|---|---|
| name | Identifier passed to nextInSequence "name" |
| dir | Directory to scan, relative to docs/ |
| glob | File pattern to count |
| regex | Must have exactly one capture group, extracts the number |
| width | Display padding width (optional) |
| start | Starting number if no matches (optional, default 1) |

The 'adr' type gets a default sequence automatically: Glob adr-*.md,
Regex ^adr-(\d{3})-, Width 3, Start 1.

## Body Block Rules

- Exactly ONE fenced markdown code block per template document
- Language must be 'markdown' or 'md'
- Content uses Go text/template syntax
- Body usually contains the generated doc's frontmatter + content

## Glob Suggestion Patterns

| Pattern | Suggests |
|---|---|
| dir/*.md | File stems under dir/ |
| dir/*/ | Direct child directory names |

Patterns are docs-relative; cannot escape docs/.

## Worked Example (frontmatter + body excerpt)

Schema:
  type: guide
  name: Quick Guide
  output_path: guides/{{ .Slug }}.md
  schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'

Body:
---
title: "{{ .Title }}"
date: 2026-09-25T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

## Overview
## Steps

## Verification Workflow

1. Author template at docs/templates/<type>.template.md
2. Build the docs site (docbuilder build)
3. Verify discovery: docbuilder template list --base-url <site>
4. Dry-run: docbuilder template new --base-url <site> --set Title=Test --set Slug=test --defaults --yes
5. Lint the generated doc: docbuilder lint <generated-path>

## Common Pitfalls

- Multiple or zero fenced markdown blocks in the body — parser rejects.
- output_path absolute or escapes docs/ — WriteGeneratedFile rejects.
- sequence.regex has 0 or >1 capture groups — next-number fails.
- Schema key not referenced in body — wasted prompt.
- Template not categorized as 'Templates' — discovery skips it.
- Template URL with .. or absolute path — discovery HTTP client rejects.
`
