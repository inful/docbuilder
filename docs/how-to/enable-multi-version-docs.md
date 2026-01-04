---
title: "How To: Enable Multi-Version Documentation"
date: 2025-12-15
categories:
  - how-to
tags:
  - versioning
  - documentation
---

# How to Enable Multi-Version Documentation

Build documentation from multiple branches and tags to provide version-specific documentation for your users.

## Overview

Multi-version documentation allows you to:
- Build docs from multiple branches (e.g., `main`, `develop`, `release-1.x`)
- Build docs from version tags (e.g., `v1.0.0`, `v2.0.0`)
- Provide version switchers in your documentation site
- Maintain historical documentation alongside current docs

## Basic Setup

### Enable Versioning

Add to your configuration:

```yaml
versioning:
  enabled: true
  strategy: branches_and_tags
  max_versions_per_repo: 10
```

### Configure Version Selection

#### Branches and Tags

```yaml
versioning:
  enabled: true
  strategy: branches_and_tags
  branch_patterns:
    - "main"
    - "develop"
    - "release/*"
  tag_patterns:
    - "v*"           # v1.0.0, v2.0.0, etc.
    - "[0-9]*"       # 1.0.0, 2.0.0, etc.
  max_versions_per_repo: 5
```

#### Branches Only

```yaml
versioning:
  enabled: true
  strategy: branches_only
  branch_patterns:
    - "main"
    - "develop"
    - "feature/*"
  max_versions_per_repo: 3
```

#### Tags Only

```yaml
versioning:
  enabled: true
  strategy: tags_only
  tag_patterns:
    - "v[0-9]*"      # Semantic versions only
  max_versions_per_repo: 10
```

## How It Works

### Version Discovery

1. **Remote Query**: DocBuilder queries Git remote for available refs
2. **Pattern Matching**: Filters branches/tags by configured patterns
3. **Sorting**: Orders by creation time (newest first)
4. **Limiting**: Applies `max_versions_per_repo` limit
5. **Expansion**: Creates separate repository entry for each version

### Repository Expansion

Given this configuration:

```yaml
repositories:
  - url: https://github.com/org/project.git
    name: project
    branch: main

versioning:
  enabled: true
  strategy: branches_and_tags
  max_versions_per_repo: 3
```

DocBuilder expands it to:

```
project-main (branch)
project-v2.0.0 (tag)
project-v1.0.0 (tag)
```

### Content Organization

Each version is cloned and processed separately:

```
content/
  project-main/
    _index.md
    getting-started.md
  project-v2.0.0/
    _index.md
    getting-started.md
  project-v1.0.0/
    _index.md
    getting-started.md
```

### Hugo Configuration

DocBuilder generates Hugo config with version metadata:

```yaml
params:
  versions:
    - name: main
      url: /project-main/
      is_default: true
    - name: v2.0.0
      url: /project-v2.0.0/
    - name: v1.0.0
      url: /project-v1.0.0/
```

## Complete Example

```yaml
version: "2.0"

repositories:
  - url: https://github.com/myorg/api-server.git
    name: api
    paths: ["docs"]
    auth:
      type: token
      token: ${GITHUB_TOKEN}

versioning:
  enabled: true
  strategy: branches_and_tags
  max_versions_per_repo: 5
  
  # Include main development branches
  branch_patterns:
    - "main"
    - "develop"
  
  # Include semantic version tags
  tag_patterns:
    - "v[0-9]*.[0-9]*.[0-9]*"
  
hugo:
  title: "API Documentation"
  theme: relearn
  params:
    version_menu: true

output:
  directory: ./site
  clean: true
```

## Version Patterns

### Semantic Versioning

```yaml
tag_patterns:
  - "v[0-9]*.[0-9]*.[0-9]*"         # v1.0.0, v2.1.3
  - "[0-9]*.[0-9]*.[0-9]*"           # 1.0.0, 2.1.3
```

### Release Branches

```yaml
branch_patterns:
  - "main"
  - "release-[0-9]*"                 # release-1, release-2
  - "release/v[0-9]*.[0-9]*"         # release/v1.0, release/v2.1
```

### Development Branches

```yaml
branch_patterns:
  - "main"
  - "develop"
  - "next"
  - "feature/*"                      # All feature branches
```

## Authentication

Versioning works with all authentication methods:

```yaml
repositories:
  - url: https://gitlab.example.com/org/project.git
    name: project
    auth:
      type: token
      token: ${GITLAB_TOKEN}

versioning:
  enabled: true
```

## Performance Considerations

### Clone Time

Each version requires a separate clone:
- 1 repository × 5 versions = 5 clones
- 10 repositories × 3 versions = 30 clones

Use shallow clones to reduce time:

```yaml
build:
  shallow_depth: 1  # Only fetch latest commit
  
versioning:
  enabled: true
  max_versions_per_repo: 3  # Limit versions
```

### Build Time

More versions = longer builds:
- Discovery: O(1) per repository (remote query)
- Clone: O(n) per version
- Hugo: O(n) content files

### Incremental Builds

Combine with incremental builds for best performance:

```yaml
build:
  enable_incremental: true
  shallow_depth: 1

versioning:
  enabled: true
  max_versions_per_repo: 5
```

## Theme Integration

### Relearn Theme

The Relearn theme automatically detects version metadata and displays it in the navigation:

```yaml
hugo:
  theme: relearn
  params:
    version_menu: true
```

## Default Version

The default version is determined by:
1. Default branch from Git (usually `main` or `master`)
2. First branch alphabetically if default not found
3. Marked as `is_default: true` in Hugo config

## Troubleshooting

### Tags Not Cloning

Ensure patterns match your tag names:

```yaml
versioning:
  tag_patterns:
    - "*"  # Temporarily match all tags for testing
```

Check logs for:
```
DEBUG msg="Evaluating reference for inclusion" name=v1.0.0 type=tag include=true
DEBUG msg="Cloning tag reference" name=project-v1.0.0 tag=v1.0.0 ref=refs/tags/v1.0.0
```

### Too Many Versions

Reduce the limit:

```yaml
versioning:
  max_versions_per_repo: 3  # Keep only 3 most recent
```

### No Versions Discovered

Check:
1. Repository has branches/tags matching patterns
2. Authentication is working
3. Patterns are correct (test with `*` wildcard)

Enable verbose logging:

```bash
docbuilder build -c config.yaml -v
```

Look for:
```
INFO msg="Discovering versions for repository" repo_url=...
INFO msg="Repository expansion complete" original=1 expanded=5
```

### Wrong Versions Selected

Adjust patterns to be more specific:

```yaml
# Too broad
tag_patterns:
  - "*"

# More specific
tag_patterns:
  - "v[0-9]*.[0-9]*.[0-9]*"    # Only semantic versions
  - "!v*-rc*"                   # Exclude release candidates
```

## Disabling Versioning

To disable multi-version builds:

```yaml
versioning:
  enabled: false  # Or remove versioning section entirely
```

Or build only default branch:

```yaml
versioning:
  enabled: true
  default_branch_only: true
```
