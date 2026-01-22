---
aliases:
  - /_uid/96c8f654-7ff8-4022-b290-cbc2c2c5fbe7/
categories:
  - architecture-decisions
date: 2026-01-14T00:00:00Z
fingerprint: 3572a5b8963a9f86f9e2de63c22dedf1d902a9111426527962f9cbfb1d8403e2
lastmod: "2026-01-22"
tags:
  - document-identity
  - redirects
  - hugo-aliases
  - urls
title: 'ADR-010: Stable Document Identity via UID Aliases'
uid: 96c8f654-7ff8-4022-b290-cbc2c2c5fbe7
---

# ADR-010: Stable Document Identity via UID Aliases

**Status**: Accepted  
**Date**: 2026-01-14  
**Implemented**: 2026-01-14  
**Decision Makers**: DocBuilder Core Team  
**Technical Story**: Enable stable document URLs independent of repository/path changes

## Context and Problem Statement

DocBuilder aggregates documentation from multiple repositories, each with its own content structure. As documentation evolves:

- Documents move between repositories or sections
- File paths change (e.g., `guides/installation.md` → `getting-started/install.md`)
- Repository structures are reorganized

This creates a problem for downstream systems:

- **Search indexes**: document URLs change, links become stale
- **Ingestion pipelines**: external systems track by URL, but URLs are unstable
- **User bookmarks and citations**: links break when pages move
- **External references**: other sites linking to documentation become broken

Every document already has a stable `uid` (UUID) in frontmatter that never changes. This `uid` should be the **canonical document identity** independent of location.

## Decision

DocBuilder will automatically inject a Hugo `aliases` entry into each user-authored document's frontmatter, mapping `/_uid/<uid>/` to the document's canonical URL. Generated index files (`_index.md`) are excluded as they are ephemeral and don't require stable identifiers.

Hugo uses the `aliases` field to generate redirect pages at alternative URLs that point to the canonical page. When a document moves:

1. The old canonical path gets a `aliases` entry pointing to the new path (via Hugo's redirect mechanism)
2. The stable `/_uid/<uid>/` alias always points to the *current* canonical URL, regardless of moves
3. Downstream systems can reliably reference `base_url/_uid/<uid>/` and always reach the document

### Implementation

**Current Implementation**: The linter (`docbuilder lint --fix`) is the primary mechanism for adding uid-based aliases. This was implemented in January 2026 and handles both new UID generation and alias injection for existing UIDs.

**Linter Extension**: The `FrontmatterUIDRule` in `internal/lint/rule_frontmatter_uid.go` validates:
1. Presence and format of `uid` in frontmatter
2. Presence of the `/_uid/<uid>/` entry in the `aliases` field
3. Excludes generated `_index.md` files from validation (they don't need UIDs)

The linter's auto-fix mode (`docbuilder lint --fix`) handles two cases:
1. **Missing UID**: Generate a new UUID, add it as `uid`, and add the corresponding `aliases: ["/_uid/<uuid>/"]` entry
2. **UID exists but alias missing**: Add `/_uid/<uid>/` to the `aliases` list (appending to any existing user-defined aliases)

**Implementation Complete**:

1. **Linter Rule** (`internal/lint/rule_frontmatter_uid.go`):
   - ✅ Extended `FrontmatterUIDRule.Check()` to validate that `aliases` contains `/_uid/<uid>/`
   - ✅ Added exclusion for `_index.md` files (generated indexes don't need UIDs)
   - ✅ Reports missing alias issue when UID exists but alias is missing

2. **Linter Fixer** (`internal/lint/fixer_uid.go`):
   - ✅ Extended `addUIDAndAliasIfMissing()` to inject both uid and alias when generating new UIDs
   - ✅ Added `ensureFrontmatterUIDAlias()` to append uid-based alias to existing documents
   - ✅ Preserves user-defined aliases (appends rather than replaces)
   - ✅ Added dedicated fix phase for alias injection (Phase 2 in fixer pipeline)

3. **Content Pipeline** (`internal/hugo/content_copy_pipeline.go`):
   - ⚠️ Not implemented - deferred because linter-based approach is sufficient
   - Generated `_index.md` files intentionally excluded from UID requirement
   - User-authored source documents already have aliases before pipeline processes them

**Frontmatter Modification**:
```yaml
---
uid: 550e8400-e29b-41d4-a716-446655440000
title: "Installation Guide"
aliases:
  - /_uid/550e8400-e29b-41d4-a716-446655440000/
---
```

**Hugo Behavior**: Hugo will generate:
- Canonical page at: `/repo/section/installation/index.html`
- Redirect page at: `/_uid/550e8400-e29b-41d4-a716-446655440000/index.html` → `/repo/section/installation/`

**Document Move Handling**: When a document moves to a new path but retains its `uid`:
- The *old* canonical URL gets replaced by the new one
- The `/_uid/<uid>/` alias automatically points to the new canonical URL
- The `uid` never changes
- External indexers and linkers can always use `/_uid/<uid>/` as a stable entrypoint

### Interaction with External Ingestion (ADR-009)

The ingestion stage (ADR-009) sends the full markdown document (including frontmatter) to the external ingester. When a document has moved to a new location but retains its `uid`, the ingester can parse the frontmatter to extract:

- The stable `uid` for document identity
- The `aliases` field containing both `/_uid/<uid>/` and any previous canonical URLs
- The current canonical URL derived from the document's Hugo path

The ingester can then:
- Update its primary index entry to the new canonical URL
- Register the `/_uid/<uid>/` URL as an alias/redirect
- Optionally index previous URLs from the `aliases` field for search fallback

### Configuration

No configuration needed. The alias injection is **automatic and required** for all user-authored markdown documents that have a valid `uid`. Generated `_index.md` files are automatically excluded from this requirement.

### Usage

To add uid-based aliases to existing documentation:

```bash
# Lint and auto-fix all documentation
docbuilder lint path /path/to/docs --fix --yes

# Dry-run to preview changes
docbuilder lint path /path/to/docs --fix --dry-run
```

## Rationale

- **Stability**: `uid` never changes; URLs derived from paths inherently change
- **Simple mechanics**: Hugo's built-in `aliases` feature handles redirects; no custom routing needed
- **Static-site friendly**: Works with any static host (no server-side routing logic needed)
- **Downstream compatible**: Ingestion pipelines (search, archives, portals) get both canonical and stable URLs
- **User experience**: Bookmarks to `/_uid/<uid>/` never break, even if documentation is reorganized

## Consequences

### Benefits

- External systems have a stable, durable reference to each document
- Document moves are transparent to downstream consumers
- No server-side routing logic required; works with any static hosting
- Integrates cleanly with ingestion pipelines and external indexers

### Trade-offs

- Every rendered page will have at least one alias (the `/_uid/<uid>/` redirect)
- Larger Hugo content tree (one extra redirect page per document)
- The `/_uid/` URL structure is hardcoded; not configurable

### Limitations

- Requires that `uid` exists and is valid (enforced by linter, required before ingestion)
- Generated `_index.md` files are excluded from UID/alias requirements (intentional - they are ephemeral)
- Does not handle deletion; tombstones or reconciliation are delegated to the ingester (see ADR-009)
- Alias URLs are site-relative; absolute URL generation requires `base_url` to be set
- Content pipeline integration deferred; linter-based approach is currently sufficient

## Alternatives Considered

1. **Server-side routing/rewrite rules** (nginx, CloudFront, etc.)
   - Rejected: ties deployment to specific infrastructure; not portable across static hosts

2. **Central redirect registry** (separate service)
   - Rejected: adds operational complexity; easier to use Hugo's native aliases
3. **Accept URL instability; use search indexes only**
   - Rejected: breaks external links, citations, and integrations

## Implementation Details

### URL Pattern

The stable alias pattern is `/_uid/<uid>/` where `<uid>` is the document's UUID from frontmatter.

- Simple and predictable structure
- Easy to distinguish from content URLs
- No ambiguity with repository or section paths

### Hugo Alias Handling

Hugo processes the `aliases` field automatically. No special configuration in `hugo.yaml` is required. When Hugo builds the site, it generates redirect pages for each alias URL that point to the canonical page.

### Existing User Aliases

If a document already has user-defined `aliases` in its frontmatter, DocBuilder will **append** the `/_uid/<uid>/` alias to the existing list. This preserves any manual redirects while adding the stable UID-based redirect.

## Related Documents

- docs/explanation/architecture.md
- docs/reference/report.md
- ADR-008: Staged Pipeline Architecture
- ADR-009: External Ingester Stage
- ADR-005: Documentation Linting (discusses `uid` in frontmatter)
