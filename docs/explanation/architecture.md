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

**Transform Content Stage** executes the transform pipeline:
```
Parse → Build → Enrich → Merge → Transform → Finalize → Serialize
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
| Transform Registry | Dependency-ordered content processing pipeline. | `internal/hugo/transforms/` |
| Front Matter Core | Patch-based front matter merging with conflict tracking. | `internal/hugo/fmcore/` |
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

**Transform Pipeline** (`internal/hugo/transforms/`):

Each markdown file passes through a dependency-ordered transform pipeline:

1. **Parse** - Extract YAML front matter from markdown
2. **Build** - Generate default front matter fields (title from filename, date, etc.)
3. **Enrich** - Add repository/forge/section metadata
4. **Merge** - Apply front matter patches with conflict tracking
5. **Transform** - Content modifications (link rewriting, etc.)
6. **Finalize** - Post-processing (heading stripping, shortcode escaping)
7. **Serialize** - Output final YAML + markdown

**Transform Features:**
- Registry-based with topological dependency sorting
- Configurable enable/disable filtering
- PageShim facade for uniform access
- Patch-based front matter with merge modes (Deep, Replace, SetIfMissing)

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

- **Add new transform**: Register in `internal/hugo/transforms/` with stage and dependencies
- **Add new theme**: Implement `Theme` interface in `internal/hugo/theme/themes/`
- **Additional issue codes**: Augment taxonomy without breaking consumers
- **Future caching**: Leverage `doc_files_hash` for selective downstream regeneration
- **Custom merge modes**: Extend `FrontMatterPatch` merge strategies in `fmcore/`

## Non-Goals

- Rendering arbitrary SSGs other than Hugo.
- Full-text search indexing logic (delegated to Hugo theme or external indexing). 
