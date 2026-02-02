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
      description: "Create a new Architecture Decision Record following the standard ADR format"
      schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true},{"key":"DecisionMakers","type":"string","required":false}]}'
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

This template helps you create new Architecture Decision Records (ADRs) that follow a consistent format.

## Usage

When you use this template, you'll be prompted for:
- **Title**: The decision title (e.g., "Use Redis for caching")
- **Slug**: URL-friendly identifier (e.g., "redis-caching")
- **Decision Makers**: Optional list of decision makers

The template will automatically:
- Number your ADR sequentially (e.g., ADR-042)
- Generate proper frontmatter
- Place the file in the correct directory

## Template Body

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
**Decision Makers**: {{ if .DecisionMakers }}{{ .DecisionMakers }}{{ else }}Engineering Team{{ end }}

## Context and Problem Statement

Describe the context and problem that requires a decision.

## Decision

Describe the decision that was made.

## Consequences

### Positive
- 

### Negative
- 

### Neutral
- 
```
