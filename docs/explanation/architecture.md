---
title: "Architecture Overview"
date: 2025-12-15
categories:
  - explanation
tags:
  - architecture
  - design
---

# Architecture Overview

DocBuilder implements a staged pipeline to turn multiple Git repositories into a unified Hugo documentation site.

## Pipeline Flow

```
Config → Clone → Discover → Generate Hugo Config → Transform Content → Index Pages → (Optional) Run Hugo
```

**Transform Content Stage** executes the fixed transform pipeline:
```
Parse → Normalize → Build → Extract Title → Strip Heading → Rewrite Links → Rewrite Images → Keywords → Metadata → Edit Link → Serialize
```

Each stage records duration, outcome, and issues for observability.

## Key Components

| Component | Responsibility | Location |
|-----------|----------------|----------|
| Config Loader | Parse YAML, expand `${ENV}` variables, apply defaults. | `internal/config/` |
| Build Service | Orchestrate build pipeline execution. | `internal/build/` |
| Git Client | Clone/update repositories with auth strategies (token, ssh, basic). | `internal/git/` |
| Discovery | Walk configured doc paths, filter markdown, build `DocFile` list. | `internal/docs/` |
| Hugo Generator | Emit `hugo.yaml`, content tree, index pages, theme params. | `internal/hugo/` |
| Transform Pipeline | Fixed-order content processing pipeline with direct mutation. | `internal/hugo/pipeline/` |
| Theme System | Theme-specific parameter injection and configuration. | `internal/hugo/theme/` |
| Forge Integration | GitHub/GitLab/Forgejo API clients. | `internal/forge/` |
| Error Foundation | Classified error system with retry strategies. | `internal/foundation/errors/` |
| Report | Aggregate metrics & fingerprints for external tooling. | `internal/hugo/` |

## Namespacing Logic

Forge namespacing (conditional `content/<forge>/<repo>/...`) prevents collisions and yields scalable URL design. Auto mode activates only when more than one forge type exists.

## Idempotence & Change Detection

- Repository update strategy (`clone_strategy`) avoids unnecessary reclones.
- Combined check: unchanged repo heads + identical doc file set ⇒ logged and optionally triggers early exit (when output already valid).
- `doc_files_hash` offers external determinism for CI/CD.

## Error & Retry Model

**Error Classification** (`internal/foundation/errors`):

**Severity Levels:**
- `Fatal` - Stops execution completely
- `Error` - Fails the current operation
- `Warning` - Continues with degraded functionality
- `Info` - Informational, no impact

**Retry Strategies:**
- `RetryNever` - Permanent failure
- `RetryImmediate` - Retry immediately
- `RetryBackoff` - Exponential backoff
- `RetryRateLimit` - Wait for rate limit window
- `RetryUserAction` - Requires user intervention

**Error Categories:**
User-facing (Config, Validation, Auth, NotFound), External (Network, Git, Forge), Build (Build, Hugo, FileSystem), Runtime (Runtime, Daemon, Internal)

Transient classification guides retry policy (clone/update network issues; certain Hugo invocations).

## Content Generation Details

**Transform Pipeline** (`internal/hugo/pipeline/`):

Each markdown file passes through a fixed-order transform pipeline:

1. **parseFrontMatter** - Extract YAML front matter from markdown
2. **normalizeIndexFiles** - Rename README.md → _index.md for Hugo
3. **buildBaseFrontMatter** - Generate default fields (title, type, date)
4. **extractIndexTitle** - Extract H1 as title for index pages
5. **stripHeading** - Remove H1 from content when appropriate
6. **rewriteRelativeLinks** - Fix markdown links (.md → /, directory-style)
7. **rewriteImageLinks** - Fix image paths to content root
8. **generateFromKeywords** - Create new documents from keywords (@glossary, etc.)
9. **addRepositoryMetadata** - Inject repository/forge/commit metadata
10. **addEditLink** - Generate editURL for source links
11. **serializeDocument** - Output final YAML + markdown

**Pipeline Features:**
- Fixed execution order (explicit, no dependency resolution needed)
- Direct document mutation (no patch merge complexity)
- Document type with all fields accessible
- Generators create missing index files before transforms run

**Theme Integration:**
- Supported themes use Hugo Modules (no local theme directory needed)
- Theme-specific transforms (e.g., `hextra_type_enforcer` sets `type: docs`)
- Index template override search order ensures safe customization
- Front matter includes forge, repository, section, editURL for theme logic

## Pruning Strategy

Optional top-level pruning removes non-doc directories to shrink workspace footprint—controlled with allow/deny precedence rules to avoid accidental removal of required assets.

## Design Rationale Highlights

| Concern | Approach |
|---------|----------|
| Cross-repo collisions | Conditional forge prefix + repository segmentation. |
| Performance | Incremental fetch + pruning + shallow clones. |
| Theming | Module-based imports; param injection per theme. |
| Observability | Structured build report + issue taxonomy + stage timing. |
| Reproducibility | Environment expansion + explicit config + stable hashing. |

## Extensibility Points

- **Add new transform**: Create function in `internal/hugo/pipeline/transforms.go` and add to `defaultTransforms()` list
- **Add new generator**: Create function in `internal/hugo/pipeline/generators.go` and add to `defaultGenerators()` list  
- **Add new theme**: Implement `Theme` interface in `internal/hugo/theme/themes/`
- **Additional issue codes**: Augment taxonomy without breaking consumers
- **Future caching**: Leverage `doc_files_hash` for selective downstream regeneration

## Non-Goals

- Rendering arbitrary SSGs other than Hugo.
- Full-text search indexing logic (delegated to Hugo theme or external indexing). 
