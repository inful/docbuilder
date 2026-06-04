---
aliases:
  - /_uid/6359eb3d-f704-412d-9f55-373f496a1959/
categories:
  - Templates
fingerprint: 542fa6d70553d3edd98365ceb61aa43b1cd4bcb65abbb2e0c3321decc1d30fe0
lastmod: "2026-06-04"
params:
  docbuilder:
    template:
      defaults: '{"tags":["guide"],"Category":"advanced"}'
      description: Create a new user guide with category selection
      name: User Guide
      output_path: guides/{{ .Slug }}.md
      schema: '{"fields":[{"key":"Tags","type":"string_list","required":true},{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true,"glob-suggestion":"*/"},{"key":"Selection","type":"string_enum","required":true,"options":["getting-started","advanced","reference"]},{"key":"Categories","type":"string_list","required":true},{"key":"false","type":"bool","required":true}]}'
      type: guide
title: Guide Template
uid: 6359eb3d-f704-412d-9f55-373f496a1959
---

# Guide Template

Use this template to create new user guides with consistent structure.

## Usage

When you use this template, you'll be prompted for:
- **Tags**: Select from existing tags, or create your own
- **Title**: The guide title (e.g., "API Authentication")
- **Slug**: URL-friendly identifier (e.g., "api-auth")
- **Selection**: Select from getting-started, advanced, or reference
- **Categories**: Select from existing categories or create your own

## Template Body

```markdown
---
title: "{{ .Title }}"

categories:
{{- range .Categories}}
  - {{ . }}
{{- end}}
tags:
{{- range .Tags}}
  - {{ . }}
{{- end}}
date: 2026-01-01T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

## Overview

Brief overview of what this guide covers.

## Prerequisites

- 

## Steps

### Step 1: 

### Step 2: 

## Next Steps

- 

## Related Documentation

- 
```
