---
uid: f94cf6fb-b200-44e5-9177-0daf24be4367
aliases:
  - /_uid/f94cf6fb-b200-44e5-9177-0daf24be4367/
title: "ADR-011: Set lastmod When Fingerprint Changes"
date: 2026-01-15
categories:
  - architecture-decisions
tags:
  - frontmatter
  - fingerprint
  - lastmod
  - hugo
fingerprint: c3211462fd46798739faccd46a630ae9768537286b99646380e4821464d3f701
---

# ADR-011: Set lastmod When Fingerprint Changes

**Status**: Accepted  
**Date**: 2026-01-15  
**Decision Makers**: DocBuilder Core Team  
**Technical Story**: Track content modification time based on fingerprint updates

## Context and Problem Statement

DocBuilder uses a `fingerprint` field in YAML frontmatter as a deterministic, content-derived change indicator (used by linting, build reporting, and downstream ingestion/integration workflows).

Hugo supports `lastmod` in frontmatter to represent “last modified” time. Without a stable `lastmod` policy, consumers can end up with confusing or noisy “updated” signals:

- File modification timestamps in the generated Hugo project change for reasons unrelated to content (e.g., re-runs, staging directories, fresh clones).
- Source repositories may not consistently maintain `lastmod`.
- Downstream systems (e.g., search indexers or external ingestion) want a clear and reliable “content changed” signal that matches the fingerprint.

We want `lastmod` to reflect meaningful content changes (as represented by the fingerprint), not incidental filesystem or pipeline behavior.

## Decision

When DocBuilder regenerates a frontmatter `fingerprint` for a markdown file and the fingerprint value changes, DocBuilder MUST set (or update) a `lastmod` field in the document frontmatter to the current date, formatted as:

```yaml
lastmod: YYYY-MM-DD
```

### Update rule

- If the newly computed fingerprint differs from the existing fingerprint in the file’s frontmatter, set `lastmod` to “today” (UTC date) and write the updated frontmatter.
- If the fingerprint does not change, do not modify `lastmod`.

This keeps `lastmod` aligned with DocBuilder’s canonical “content changed” signal.

### Scope

This policy applies to the **fingerprinting step that updates source markdown** (i.e., the same place where DocBuilder already updates `fingerprint` in frontmatter).

Build-time content generation SHOULD NOT introduce non-deterministic `lastmod` values for documents that are missing fingerprints in source content, since this would create daily, content-independent deltas in the generated site and build report hashes.

## Rationale

- The fingerprint is the most reliable representation of “the content changed”.
- `lastmod` is broadly useful for Hugo features (page metadata, sitemaps, RSS) and for external consumers.
- Updating `lastmod` only when the fingerprint changes avoids “touch noise” and preserves determinism for unchanged content.

## Consequences

### Benefits

- Clear, consistent “last modified” metadata tied to actual content changes.
- Reduced noise compared to relying on filesystem mtimes.
- Better alignment between DocBuilder linting/build reporting and Hugo’s content metadata.

### Trade-offs

- Any workflow that runs the fingerprint fixer against source files will update `lastmod` on the same change that updates `fingerprint`, creating additional diffs to commit.
- Date-level precision (`YYYY-MM-DD`) intentionally loses time-of-day detail.

## Alternatives Considered

1. **Always set `lastmod` during fingerprinting**
   - Rejected: would cause non-content changes (e.g., runs on different days) to appear as content updates.

2. **Derive `lastmod` from Git history (last commit timestamp)**
   - Rejected: requires reliable VCS metadata and would be incorrect for generated/transformed content changes not represented as commits.

3. **Derive `lastmod` from filesystem mtime**
   - Rejected: unstable in DocBuilder’s staged pipeline and across clones/build directories.

4. **Do nothing and rely on Hugo defaults**
   - Rejected: does not provide a consistent policy across sources and pipelines.

## Implementation Notes

- The date should be computed in UTC and formatted using Go’s `2006-01-02` layout.
- The update should be conditional on an actual fingerprint value change (not merely a frontmatter rewrite).

## Related Documents

- ADR-005: Documentation Linting
- ADR-009: External Ingester Stage
- ADR-010: Stable Document Identity via UID Aliases
