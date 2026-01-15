---
uid: 7d804d6f-42df-436f-8b7c-cadc4c6b88c4
aliases:
  - /_uid/7d804d6f-42df-436f-8b7c-cadc4c6b88c4/
fingerprint: 26e67b00465e2dff8ea87c8cdcc80d08f42b7cbd6d521baa673428e3342b5f97
---

# Index File Handling

DocBuilder automatically generates index pages for repositories and sections, but also respects user-provided index files. This document explains how index files are discovered, processed, and what takes precedence when multiple options exist.

## Overview

DocBuilder generates Hugo `_index.md` files at three levels:

1. **Site Level**: Main landing page (`content/_index.md`)
2. **Repository Level**: Repository overview pages (`content/{repository}/_index.md`)
3. **Section Level**: Section overview pages (`content/{repository}/{section}/_index.md`)

Users can provide their own index content to replace auto-generated indexes.

## Transform Pipeline Processing

All index files are processed through DocBuilder's fixed transform pipeline, which applies transformations in a specific order:

1. **normalizeIndexFiles** - Automatically converts `README.md` → `_index.md` for Hugo compatibility
2. **buildBaseFrontMatter** - Adds default metadata (title, type, date, repository)
3. **extractIndexTitle** - Extracts H1 heading as title for index pages
4. **rewriteRelativeLinks** - Fixes markdown links to work in Hugo (`.md` → `/`, directory-style)
5. **rewriteImageLinks** - Corrects image paths relative to content root
6. **addRepositoryMetadata** - Injects repository/forge/commit information
7. **addEditLink** - Generates edit URLs for source links

This ensures all index files (whether user-provided or auto-generated) receive consistent processing and link rewriting.

For more details on the transform pipeline, see [Content Transform Pipeline](content-transforms.md) and [ADR-003: Fixed Transform Pipeline](../adr/ADR-003-fixed-transform-pipeline.md).

## Repository-Level Indexes

At the repository level (directly under the configured `paths`), DocBuilder supports three scenarios:

### Case 1: README.md Only

When a repository contains only a `README.md` file at the root of the docs path:

```
your-repo/
  docs/
    README.md          ← Automatically converted to _index.md by pipeline
    guide.md
    api/
      reference.md
```

**Behavior**: `README.md` is automatically normalized to `_index.md` by the `normalizeIndexFiles` transform. The transform pipeline then processes it like any other document (link rewriting, metadata injection, etc.).

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
    index.md           ← Automatically converted to _index.md by pipeline
    guide.md
    api/
      reference.md
```

**Behavior**: `index.md` is automatically normalized to `_index.md` by the `normalizeIndexFiles` transform, just like README.md files.

### Case 4: Both index.md and README.md

When both files exist at the repository root:

```
your-repo/
  docs/
    README.md          ← Processed as regular document, accessible at /repo/readme/
    index.md           ← Normalized to _index.md (takes precedence)
    guide.md
```

**Behavior**: 
- Both files are discovered and processed through the transform pipeline
- `index.md` is normalized to `_index.md` first (takes precedence for the repository index)
- `README.md` is **also** normalized to `_index.md` but is renamed to prevent collision
- The original `README.md` position in the pipeline ensures it's accessible at `/repo/readme/`

**Precedence Order**: `index.md` > `README.md` > auto-generated

**Implementation Note**: This precedence is handled during the generation phase, before transforms run. The pipeline only receives files that should exist in the final site.

## Section-Level Indexes

Within sections (subdirectories), `README.md` or `index.md` files are recognized as section indexes:

```
your-repo/
  docs/
    api/
      index.md         ← Normalized to _index.md by pipeline
      endpoint-a.md
      endpoint-b.md
    guides/
      README.md        ← Also normalized to _index.md
      tutorial.md
```

**Behavior**: Both `README.md` and `index.md` in a section directory are normalized to `_index.md` by the transform pipeline. If both exist in the same section, `index.md` takes precedence (same as repository-level).

If no user-provided index exists, the generation phase creates a section index listing all documents in that section. Generated indexes are then processed through the same transform pipeline.

## File Naming Conventions

Important notes about file naming:

1. **Case-Insensitive**: `README.md`, `readme.md`, `Readme.md` are all treated the same
2. **Lowercase URLs**: Files are converted to lowercase for URLs (`README.md` → `/repo/readme/`)
3. **Automatic Normalization**: Both `README.md` and `index.md` are automatically converted to `_index.md` by the `normalizeIndexFiles` transform early in the pipeline
4. **Hugo Compatibility**: The `_index.md` naming is required by Hugo for section/repository index pages

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

If no front matter is present, the `buildBaseFrontMatter` transform adds it automatically:

```yaml
title: Repository Name        # Extracted from filename or H1
repository: repository-name   # Source repository
type: docs                    # Hugo content type
date: 2025-12-12T15:30:00Z   # Commit date or fixed epoch
```

Additional metadata is injected by later transforms:
- `editURL` - Added by `addEditLink` transform if repository forge is configured
- Repository/commit info - Added by `addRepositoryMetadata` transform

User-provided front matter takes precedence and is merged with auto-generated fields.

## Ignored Files

The following files are **filtered during discovery** and never enter the processing pipeline:

- `CONTRIBUTING.md`
- `CHANGELOG.md`  
- `LICENSE.md`
- `CODE_OF_CONDUCT.md`
- `.github/` directory contents

These files are typically repository-level documentation not relevant to the generated docs site and are excluded at discovery time regardless of location (root or subdirectories).

**Important**: `README.md` is **not** ignored. It can be used as a repository/section index (automatically normalized to `_index.md`) or as a regular document depending on the presence of `index.md` files.

## Best Practices

1. **Use index.md for Docs Sites**: If you're building a dedicated documentation site, use `index.md` for repository and section indexes. Reserve `README.md` for GitHub/GitLab repository overview.

2. **Keep README.md for Dual Purpose**: If your repository README is also suitable as docs landing page, use `README.md` and skip `index.md`. Both are normalized to `_index.md` by the pipeline.

3. **Section Organization**: Provide `README.md` or `index.md` files in section directories to give users context about what's in that section. Missing indexes are auto-generated but lack custom content.

4. **Front Matter**: Add proper front matter to control titles, descriptions, and ordering in navigation. The pipeline merges user front matter with auto-generated fields.

5. **Relative Links Work**: Both user-provided and auto-generated indexes receive link rewriting, so you can use relative markdown links (`[Guide](./guide.md)`) and they'll be converted to Hugo-compatible URLs automatically.

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
2. File is named exactly `index.md` or `README.md` (case-insensitive)
3. File has `.md` or `.markdown` extension
4. Repository has been rebuilt (daemon mode requires rebuild trigger)
5. Check verbose logs for "normalizeIndexFiles" transform output

**Q: Links in my index files aren't working**

A: Index files go through the same link rewriting transforms as regular documents. Use standard markdown links:
- Relative links: `[Guide](./guide.md)` → `/repo/guide/`
- Section links: `[API](./api/overview.md)` → `/repo/api/overview/`
- The `rewriteRelativeLinks` transform handles conversion automatically

**Q: README.md is missing from my site when I have both README.md and index.md**

A: This is expected behavior. When `index.md` exists at the same level, it takes precedence during the generation phase. `README.md` may still be accessible as a regular document at `/repository/readme/` depending on how the precedence was resolved.

**Q: My section index.md isn't showing up**

A: Check that:
1. File is in a subdirectory (section), not at repository root
2. Section is not empty (contains other .md files)
3. File is being discovered (check verbose logs for "Discovered file")
4. Transform pipeline logs show normalization: "normalizeIndexFiles: README → _index"

**Q: Front matter from my index file is being overwritten**

A: User-provided front matter is merged with auto-generated fields, not replaced. If you see unexpected values:
1. Check that your front matter is valid YAML
2. Ensure the front matter fence starts at line 1 (no content before `---`)
3. User values take precedence over pipeline defaults
4. Some fields (like `editURL`) are added by transforms after user front matter is parsed
