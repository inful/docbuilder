# Index File Handling

DocBuilder automatically generates index pages for repositories and sections, but also respects user-provided index files. This document explains how index files are discovered, processed, and what takes precedence when multiple options exist.

## Overview

DocBuilder generates Hugo `_index.md` files at three levels:

1. **Site Level**: Main landing page (`content/_index.md`)
2. **Repository Level**: Repository overview pages (`content/{repository}/_index.md`)
3. **Section Level**: Section overview pages (`content/{repository}/{section}/_index.md`)

Users can provide their own index content to replace auto-generated indexes.

## Repository-Level Indexes

At the repository level (directly under the configured `paths`), DocBuilder supports three scenarios:

### Case 1: README.md Only

When a repository contains only a `README.md` file at the root of the docs path:

```
your-repo/
  docs/
    README.md          ← Used as repository _index.md
    guide.md
    api/
      reference.md
```

**Behavior**: `README.md` is used as the repository's main index page.

### Case 2: No User-Provided Index

When neither `README.md` nor `index.md` exists:

```
your-repo/
  docs/
    guide.md
    api/
      reference.md
```

**Behavior**: DocBuilder auto-generates a repository index listing sections and documents.

### Case 3: index.md Only

When a repository contains an `index.md` file:

```
your-repo/
  docs/
    index.md           ← Used as repository _index.md
    guide.md
    api/
      reference.md
```

**Behavior**: `index.md` is used as the repository's main index page.

### Case 4: Both index.md and README.md

When both files exist at the repository root:

```
your-repo/
  docs/
    README.md          ← Copied as regular document (readme.md)
    index.md           ← Used as repository _index.md
    guide.md
```

**Behavior**: 
- `index.md` takes precedence and becomes the repository's `_index.md`
- `README.md` is copied as a regular document and accessible at `/repo/readme/`

**Precedence Order**: `index.md` > `README.md` > auto-generated

## Section-Level Indexes

Within sections (subdirectories), only `index.md` files are recognized as section indexes:

```
your-repo/
  docs/
    api/
      index.md         ← Used as section _index.md
      endpoint-a.md
      endpoint-b.md
```

**Behavior**: `index.md` in a section directory becomes that section's `_index.md`.

If no `index.md` exists, DocBuilder generates a section index listing all documents in that section.

## File Naming Conventions

Important notes about file naming:

1. **Case-Insensitive**: `README.md`, `readme.md`, `Readme.md` are all treated the same
2. **Lowercase URLs**: Files are converted to lowercase for URLs (`README.md` → `/repo/readme/`)
3. **index.md Conversion**: Files named `index.md` are automatically converted to `_index.md` in Hugo's content directory

## Configuration Impact

### Forge-Discovered Repositories

Repositories discovered via forge auto-discovery (GitLab, GitHub, Forgejo) default to using `["docs"]` as their documentation paths:

```yaml
forges:
  - name: "gitlab"
    type: "gitlab"
    api_url: "https://gitlab.example.com/api/v4"
    auto_discover: true

filtering:
  required_paths: ["docs"]  # Only repos with "docs" folder are included
```

For these repositories, place your `index.md` or `README.md` at:
- `docs/index.md` (repository index)
- `docs/section-name/index.md` (section index)

### Explicitly Configured Repositories

For explicitly configured repositories, you control the documentation paths:

```yaml
repositories:
  - url: "https://github.com/example/repo.git"
    name: "my-repo"
    paths: ["documentation", "guides"]
```

Index files should be placed at the root of each configured path:
- `documentation/index.md`
- `guides/index.md`

## Front Matter in User-Provided Indexes

User-provided index files can include Hugo front matter:

```markdown
---
title: "Custom Repository Title"
description: "A detailed description"
weight: 10
---

# Welcome to My Repository

Custom content here...
```

If no front matter is present, DocBuilder adds minimal front matter automatically:

```yaml
title: Repository Name
repository: repository-name
type: docs
date: 2025-12-12T15:30:00Z
```

## Ignored Files

The following files are ignored at the repository root level (but can exist in subdirectories):

- `CONTRIBUTING.md`
- `CHANGELOG.md`
- `LICENSE.md`

These are typically repository-level documentation not relevant to the generated docs site.

Exception: As shown above, `README.md` is **not** ignored and can be used as the repository index or a regular document depending on whether `index.md` exists.

## Best Practices

1. **Use index.md for Docs Sites**: If you're building a dedicated documentation site, use `index.md` for repository and section indexes. Reserve `README.md` for GitHub/GitLab repository overview.

2. **Keep README.md for Dual Purpose**: If your repository README is also suitable as docs landing page, use `README.md` and skip `index.md`.

3. **Section Organization**: Always provide `index.md` files in section directories to give users context about what's in that section.

4. **Front Matter**: Add proper front matter to control titles, descriptions, and ordering in navigation.

## Examples

### Example 1: Technical Documentation with Separate README

```
my-project/
  README.md              ← GitHub repository overview
  docs/
    index.md             ← Docs landing page
    getting-started.md
    api/
      index.md           ← API section overview
      rest.md
      graphql.md
```

Result: 
- Site uses `docs/index.md` as repository index
- GitHub shows root `README.md`
- Root `README.md` not included in docs site

### Example 2: Simple Project with README as Docs

```
my-tool/
  docs/
    README.md            ← Repository and docs landing page
    installation.md
    usage.md
```

Result:
- `docs/README.md` becomes repository index
- Simple structure for small projects

### Example 3: Multi-Language Documentation

```
my-app/
  docs/
    index.md             ← Main docs index
    en/
      index.md           ← English section index
      guide.md
    fr/
      index.md           ← French section index
      guide.md
```

Result:
- Clear index pages at each level
- Language sections well-organized

## Troubleshooting

**Q: My index.md isn't being used as the repository index**

A: Ensure:
1. File is at the root of your configured `paths` directory
2. File is named exactly `index.md` (case-insensitive)
3. File has `.md` extension
4. Repository has been rebuilt (daemon mode requires rebuild trigger)

**Q: README.md is missing from my site when I have both README.md and index.md**

A: This is expected behavior. When `index.md` exists, it takes precedence for the repository index, and `README.md` becomes a regular document accessible at `/repository/readme/` (lowercase).

**Q: My section index.md isn't showing up**

A: Check that:
1. File is in a subdirectory (section), not at repository root
2. Section is not empty (contains other .md files)
3. File is being discovered (check verbose logs for "Discovered file")
