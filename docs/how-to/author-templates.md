---
aliases:
  - /_uid/template-authoring-guide/
categories:
  - how-to
date: 2026-02-02T00:00:00Z
fingerprint: template-authoring-guide-fingerprint
lastmod: "2026-02-02"
tags:
  - templates
  - authoring
  - markdown
  - metadata
uid: template-authoring-guide-uid
---

# Authoring Documentation Templates

This guide explains how to create and publish templates for use with `docbuilder template new`.

## Overview

Templates are regular documentation pages that:
- Are categorized as "Templates"
- Include template metadata in frontmatter
- Contain a markdown code block with the template body
- Are published in your documentation site

## Template Structure

A template document has three parts:

1. **Frontmatter** - YAML metadata including template configuration
2. **Description** - Human-readable explanation (optional)
3. **Template Body** - A fenced markdown code block containing the template

## Basic Template Example

Create a file `docs/templates/adr.template.md`:

```yaml
---
title: "ADR Template"
categories:
  - Templates
params:
  docbuilder:
    template:
      type: "adr"
      name: "Architecture Decision Record"
      output_path: "adr/adr-{{ printf \"%03d\" (nextInSequence \"adr\") }}-{{ .Slug }}.md"
      description: "Create a new Architecture Decision Record"
      schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'
      defaults: '{"categories":["architecture-decisions"]}'
      sequence:
        name: "adr"
        dir: "adr"
        glob: "adr-*.md"
        regex: "^adr-(\\d{3})-"
        width: 3
        start: 1
---

# Architecture Decision Record Template

Use this template to create new ADRs following our standard format.

```markdown
---
title: "{{ .Title }}"
categories:
  - {{ index .categories 0 }}
date: 2026-01-01T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

**Status**: Proposed  
**Date**: {{ .Date }}  
**Decision Makers**: {{ .DecisionMakers }}

## Context and Problem Statement

## Decision

## Consequences
```
```

## Required Frontmatter Fields

### `params.docbuilder.template.type`

**Required.** Canonical template identifier (e.g., `"adr"`, `"guide"`).

```yaml
params:
  docbuilder:
    template:
      type: "adr"
```

### `params.docbuilder.template.name`

**Required.** Human-friendly display name shown in template lists.

```yaml
params:
  docbuilder:
    template:
      name: "Architecture Decision Record"
```

### `params.docbuilder.template.output_path`

**Required.** Go template string defining where generated files are written (relative to `docs/`).

```yaml
params:
  docbuilder:
    template:
      output_path: "adr/adr-{{ printf \"%03d\" (nextInSequence \"adr\") }}-{{ .Slug }}.md"
```

**Available template variables:**
- `{{ .Title }}` - User-provided title
- `{{ .Slug }}` - User-provided slug
- `{{ .FieldName }}` - Any field from schema
- `{{ nextInSequence "name" }}` - Next number in sequence

## Optional Frontmatter Fields

### `params.docbuilder.template.description`

Brief description shown to users.

```yaml
params:
  docbuilder:
    template:
      description: "Create a new Architecture Decision Record following our standard format"
```

### `params.docbuilder.template.schema`

JSON schema defining input fields and prompts.

```yaml
params:
  docbuilder:
    template:
      schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'
```

**Schema Field Types:**

- `string` - Text input
- `string_enum` - Select from options (requires `options` array)
- `string_list` - Comma-separated values
- `bool` - Boolean value (accepts `true`/`false`, `t`/`f`, `1`/`0`, `TRUE`/`FALSE`, `True`/`False`, `T`/`F`)

**Example Schema:**

```json
{
  "fields": [
    {
      "key": "Title",
      "type": "string",
      "required": true
    },
    {
      "key": "Category",
      "type": "string_enum",
      "required": true,
      "options": ["getting-started", "advanced", "reference"]
    },
    {
      "key": "Tags",
      "type": "string_list",
      "required": false
    },
    {
      "key": "Published",
      "type": "bool",
      "required": false
    }
  ]
}
```

### `params.docbuilder.template.defaults`

JSON object providing default values for fields.

```yaml
params:
  docbuilder:
    template:
      defaults: '{"categories":["architecture-decisions"],"tags":["adr"]}'
```

Defaults are used when:
- `--defaults` flag is set (skip all prompts)
- Field is not required and user doesn't provide value

### `params.docbuilder.template.sequence`

Configuration for sequential numbering.

```yaml
params:
  docbuilder:
    template:
      sequence:
        name: "adr"              # Identifier for nextInSequence()
        dir: "adr"               # Directory to scan (relative to docs/)
        glob: "adr-*.md"         # Filename pattern
        regex: "^adr-(\\d{3})-" # Extract sequence number (must have 1 capture group)
        width: 3                 # Display width for padding (optional)
        start: 1                 # Starting number if no matches (optional, default: 1)
```

**Sequence Example:**

Given existing files:
- `docs/adr/adr-001-first.md`
- `docs/adr/adr-003-third.md`
- `docs/adr/adr-010-tenth.md`

Next sequence number: `011` (max + 1)

## Template Body

The template body is a **single fenced markdown code block** in the template document.

**Important:**
- Must be exactly one markdown code block
- Code block should use `language-markdown` or `language-md`
- Content uses Go `text/template` syntax

**Example:**

````markdown
```markdown
---
title: "{{ .Title }}"
categories:
  - {{ index .categories 0 }}
date: {{ .Date }}
slug: "{{ .Slug }}"
---

# {{ .Title }}

## Overview

## Details
```
````

## Template Variables

Variables come from:
1. User input (via prompts or `--set` flags)
2. Template defaults
3. Sequence helpers

**Accessing variables:**

```go
{{ .Title }}              // Direct field access
{{ index .categories 0 }} // Array access
{{ .Date | default "2026-01-01" }} // With default
```

**Sequence helper:**

```go
{{ printf "%03d" (nextInSequence "adr") }} // Padded number: 001, 002, etc.
```

## Complete Example: Guide Template

```yaml
---
title: "Guide Template"
categories:
  - Templates
params:
  docbuilder:
    template:
      type: "guide"
      name: "User Guide"
      output_path: "guides/{{ .Slug }}.md"
      description: "Create a new user guide"
      schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true},{"key":"Category","type":"string_enum","required":true,"options":["getting-started","advanced","reference"]}]}'
      defaults: '{"tags":["guide"]}'
---

# Guide Template

Use this template to create new user guides.

```markdown
---
title: "{{ .Title }}"
categories:
  - {{ .Category }}
tags:
  - {{ index .tags 0 }}
date: 2026-01-01T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

## Overview

## Steps

## Next Steps
```
```

## Publishing Templates

1. **Create template file** in your docs repository:
   ```
   docs/templates/adr.template.md
   ```

2. **Build and publish** your documentation site:
   ```bash
   docbuilder build
   # Deploy to your documentation site
   ```

3. **Verify discovery**:
   ```bash
   docbuilder template list --base-url https://your-docs-site.com
   ```

## Template Discovery

Templates are discovered via:

1. **Taxonomy page**: `GET <baseURL>/categories/templates/`
2. **Link pattern**: Anchors matching `a[href*=".template/"]`
3. **Template type**: Extracted from link text or path (strips `.template` suffix)

**Example discovery page HTML:**

```html
<ul>
  <li><a href="/templates/adr.template/index.html">adr.template</a></li>
  <li><a href="/templates/guide.template/index.html">guide.template</a></li>
</ul>
```

## Metadata Injection

DocBuilder automatically injects template metadata as HTML meta tags when building sites:

```html
<meta property="docbuilder:template.type" content="adr">
<meta property="docbuilder:template.name" content="Architecture Decision Record">
<meta property="docbuilder:template.output_path" content="adr/adr-{{ printf \"%03d\" (nextInSequence \"adr\") }}-{{ .Slug }}.md">
```

This happens via `layouts/partials/custom-header.html` (auto-generated by DocBuilder).

## Best Practices

1. **Use descriptive names** - Template names should clearly indicate purpose
2. **Provide defaults** - Reduce user input with sensible defaults
3. **Document fields** - Use schema descriptions or template page content
4. **Test templates** - Verify templates work before publishing
5. **Version control** - Templates are versioned with your docs
6. **Keep templates simple** - Focus on structure, not complex logic

## Template Functions

Available in output paths and template bodies:

- `printf` - Format strings: `{{ printf "%03d" 42 }}` → `"042"`
- `nextInSequence` - Get next sequence number: `{{ nextInSequence "adr" }}`
- Standard Go template functions (limited set for security)

**Security:** Templates run in a sandboxed environment with no file I/O or network access.

## Troubleshooting

### Template Not Discovered

**Check:**
- File is in `categories: [Templates]`
- Filename ends with `.template.md`
- Site is built and published
- Discovery page exists at `/categories/templates/`

### Template Body Not Found

**Check:**
- Exactly one markdown code block exists
- Code block uses `language-markdown` or `language-md` class
- Code block is properly fenced

### Sequence Not Working

**Check:**
- `sequence.dir` is relative to `docs/` (no `..` or absolute paths)
- `sequence.regex` has exactly one capture group
- Existing files match the glob pattern
- Regex correctly extracts numbers from filenames

### Output Path Errors

**Check:**
- Template syntax is valid Go template
- All referenced variables are provided
- Path is relative to `docs/` directory

## Example Templates

Reference implementations are available in the docs repository:

- [ADR Template](../examples/adr.template.md) - Architecture Decision Record with sequence numbering
- [Guide Template](../examples/guide.template.md) - User guide with category selection

These examples demonstrate:
- Complete frontmatter configuration
- Schema definitions
- Sequence configuration
- Template body structure

## Next Steps

- [Using Templates](./use-templates.md) - Learn how to use templates
- [Example Templates](../examples/) - Reference template implementations
- [ADR-022](../adr/adr-022-cli-template-based-markdown-generation.md) - Technical specification
- [CLI Reference](../reference/cli.md) - Full command reference
