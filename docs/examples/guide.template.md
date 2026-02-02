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
      description: "Create a new user guide with category selection"
      schema: '{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true},{"key":"Category","type":"string_enum","required":true,"options":["getting-started","advanced","reference"]}]}'
      defaults: '{"tags":["guide"]}'
---

# Guide Template

Use this template to create new user guides with consistent structure.

## Usage

When you use this template, you'll be prompted for:
- **Title**: The guide title (e.g., "API Authentication")
- **Slug**: URL-friendly identifier (e.g., "api-auth")
- **Category**: Select from getting-started, advanced, or reference

## Template Body

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
