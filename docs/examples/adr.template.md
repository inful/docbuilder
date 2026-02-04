---
aliases:
  - /_uid/0135c423-8777-4292-99e9-19ab7b82b852/
categories:
  - Templates
fingerprint: 4443d2f31d95095df4d1b1cb9debc596ada8f4805a073d5c288c8317fc58ab87
lastmod: "2026-02-04"
params:
  docbuilder:
    template:
      defaults: '{"categories":["architecture-decisions"]}'
      description: Create a new Architecture Decision Record following the standard ADR format
      name: Architecture Decision Record
      output_path: adr/adr-{{ printf "%03d" (nextInSequence "adr") }}-{{ .Slug }}.md
      schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true},{"key":"DecisionMakers","type":"string","required":false}]}'
      sequence:
        dir: adr
        glob: adr-*.md
        name: adr
        regex: ^adr-(\d{3})-
        start: 1
        width: 3
      type: adr
title: ADR Template
uid: 0135c423-8777-4292-99e9-19ab7b82b852
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
