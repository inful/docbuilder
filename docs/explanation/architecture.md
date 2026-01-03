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
| Theme System | Relearn theme with specific parameter defaults. | `internal/hugo/` |
| Daemon Service | Long-running HTTP service for incremental builds and monitoring. | `internal/daemon/` |
| Forge Integration | GitHub/GitLab/Forgejo API clients. | `internal/forge/` |
| Error Foundation | Classified error system with retry strategies. | `internal/foundation/errors/` |
| Report | Aggregate metrics & fingerprints for external tooling. | `internal/hugo/` |

## Namespacing Logic

Forge namespacing (conditional `content/<forge>/<repo>/...`) prevents collisions and yields scalable URL design. Auto mode activates only when more than one forge type exists.

## Idempotence & Change Detection

- Repository update strategy (`clone_strategy`) avoids unnecessary reclones.
- **Delta Detection**: QuickHash comparison tracks repository changes between builds
  - `quick_hash_diff`: Git commit hash changed (most common)
  - `assumed_changed`: Unable to verify, assumes changed for safety
  - `unknown`: Change detection failed or unavailable
- Combined check: unchanged repo heads + identical doc file set ⇒ logged and optionally triggers early exit (when output already valid).
- **Skip Evaluation**: Daemon mode intelligently decides between `full_rebuild`, `incremental`, or `skip`
- `doc_files_hash` (SHA-256 of sorted content paths) offers external determinism for CI/CD.
- `config_hash` enables detection of configuration changes requiring full rebuilds.

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
- Theme-specific configuration for Relearn
- Index template override search order ensures safe customization
- Front matter includes forge, repository, section, editURL for theme logic

## Pruning Strategy

Optional top-level pruning removes non-doc directories to shrink workspace footprint—controlled with allow/deny precedence rules to avoid accidental removal of required assets.

## Daemon Mode

DocBuilder can run as a long-running HTTP service for incremental builds and continuous deployment:

**Core Features:**
- **Incremental Builds**: Detects repository changes and rebuilds only affected content
- **HTTP API**: Endpoints for triggering builds, health checks, metrics
- **Live Reload**: Automatic browser refresh during development
- **Build Queue**: Manages concurrent build requests with retry logic
- **State Persistence**: Tracks repository state across restarts (`daemon-state.json`)
- **Event Stream**: Real-time build progress notifications

**Delta Detection Strategy** (`internal/daemon/delta_manager.go`):
1. Compare current repository commit hashes with last known state
2. Classify changes: `quick_hash_diff`, `assumed_changed`, `unknown`
3. Decide build strategy: `full_rebuild`, `incremental`, or `skip`
4. Update state file after successful build

**Scheduler** (`internal/daemon/scheduler.go`):
- Periodic rebuild scheduling (cron-like intervals)
- Debouncing to prevent excessive builds
- Graceful shutdown with build completion

**Observability:**
- Prometheus metrics (build duration, success rate, queue depth)
- Health endpoints (liveness, readiness)
- Structured logging with build metadata

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
- **Theme customization**: Relearn is the primary theme; customize via `params` in config or override templates in `layouts/` (see [use-relearn-theme.md](../how-to/use-relearn-theme.md))
- **Additional issue codes**: Augment taxonomy without breaking consumers
- **Future caching**: Leverage `doc_files_hash` for selective downstream regeneration
- **Daemon endpoints**: Add new HTTP handlers in `internal/daemon/` for custom workflows

## Non-Goals

- Rendering arbitrary SSGs other than Hugo.
- Full-text search indexing logic (delegated to Hugo theme or external indexing). 
