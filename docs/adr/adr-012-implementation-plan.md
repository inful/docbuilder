---
aliases:
  - /_uid/f967d658-528f-4f12-a1d8-62c203356882/
categories:
  - architecture-decisions
date: 2026-01-23T00:00:00Z
fingerprint: 96840c5836e1074e3ec0b5506aeccc0ba24b75e1fc9ed68330e3253a6dd77875
lastmod: "2026-01-23"
tags:
  - linting
  - links
  - file-system
  - implementation-plan
  - git
uid: f967d658-528f-4f12-a1d8-62c203356882
---
# ADR-012 Implementation Plan: Autoheal links to files moved

**Status**: Draft / Tracking  
**Date**: 2026-01-23  
**Decision Makers**: DocBuilder Core Team

This document is the execution plan for [ADR-012: Autoheal links to files moved](adr-012-autoheal-links-to-moved-files.md).

## Goal

Extend `docbuilder lint --fix` to heal broken relative links caused by user-performed renames/moves (e.g., `git mv`) by detecting rename mappings from Git state/history and reusing the existing fixer link update machinery.

## Non-goals

- Proactively renaming files beyond existing filename normalization fixes.
- Rewriting links outside configured documentation roots.
- Reformatting Markdown or re-rendering content; edits must remain minimal-diff destination replacements.

## Guardrails

- Strict TDD: failing test first, then minimal implementation.
- Prefer reuse of existing components:
  - broken link detection (`detectBrokenLinks*`)
  - link discovery (`findLinksInFile` / `findLinksToFile`) and edit application (`applyLinkUpdates`)
  - fingerprint regeneration ordering (must remain last)
- Keep changes scoped to `internal/lint` (plus `internal/git` reuse if/when needed).
- Performance: healing should be proportional to broken links found (avoid scanning the whole repo for every rename).

## Target API (concrete shape)

This plan assumes the API sketch in the ADR is implemented in `internal/lint`:

- `RenameSource`, `RenameMapping`
- `GitRenameDetector` (uncommitted + history)
- `BrokenLinkHealer` (or equivalent orchestrator)
- `computeUpdatedLinkTarget(...)` for correct path rewriting for moved files

If the final implementation deviates, update this plan and ADR-012 accordingly.

## Work Items (ordered)

### 0) Baseline characterization (no behavior change)

- [x] Add tests that characterize existing rename + link update behavior:
  - [x] filename normalization rename (case/spaces) updates links correctly
  - [x] link updates preserve fragments (`#...`) and relative prefixes (`./`, `../`)
  - [x] link updates do not touch code blocks / inline code

**Definition of Done**

- Tests pass and clearly document current behavior and limitations.

**Completion**: 2026-01-23 — commit: `41ba5d7`

### 1) Introduce rename mapping type + plumbing hooks

- [x] Add a small internal type (or reuse existing) that represents `oldAbs -> newAbs` mappings and can be fed into the link update path.
- [x] Add unit tests for:
  - [x] mapping normalization (absolute paths, docs-root scoping)
  - [x] de-duplication and deterministic ordering

**Definition of Done**

- There is a single representation of renames used by both fixer-driven renames and Git-derived renames.

**Completion**: 2026-01-23 — commit: `c664cd1`

### 2) Git rename detection (uncommitted)

**Intent**: catch the common “pre-commit rename broke links” workflow.

- [x] Implement/introduce `GitRenameDetector` for uncommitted renames:
  - [x] staged renames
  - [x] unstaged renames
- [x] Ensure it is safe when not in a git repo: returns `(nil, nil)`.
- [x] Tests:
  - [x] returns mappings for a repo with a `git mv` rename
  - [x] ignores non-doc-root renames

**Definition of Done**

- We can produce a reliable set of `(oldAbs, newAbs)` mappings for working tree/index.

**Completion**: 2026-01-23 — commit: `ac7a996`

### 3) Correct link target rewriting for moved files

This is the key functional delta versus current link updates.

- [x] Implement `computeUpdatedLinkTarget(sourceFile, originalTarget, oldAbs, newAbs)`.
- [x] Unit tests must cover:
  - [x] relative link targets (`../a/b.md`) moved across directories
  - [x] same-dir links remain minimal
  - [x] site-absolute links (`/docs/foo`) stay site-absolute and update correctly
  - [x] extension style preserved (`foo` stays `foo` if originally extensionless; `foo.md` stays `.md`)
  - [x] fragments preserved (`#section`)

**Definition of Done**

- For moved targets, the updated destination resolves to `newAbs` when interpreted from `sourceFile`.

**Completion**: 2026-01-23 — commit: `8c76205`

### 4) Healing strategy: focus on broken links

- [x] Use existing broken-link detection output as the primary worklist.
- [x] For each broken link, resolve the absolute target (existing `resolveRelativePath` behavior) and match against rename mappings.
- [x] Apply link updates via existing edit machinery (minimal diffs; no Markdown reformatting).
- [x] Ensure fingerprint refresh is triggered for updated files (consistent with current fixer behavior).

**Definition of Done**

- A new healing phase runs during `lint --fix` and produces `LinksUpdated` entries, without requiring the fixer to have performed the rename itself.

**Completion**: 2026-01-23 — branch: `shaman-healer`

### 5) Git history detection (since last push)

- [x] Detect upstream tracking branch if present.
- [x] Extract rename mappings for commits “since last push” (HEAD vs upstream).
- [x] Provide bounded fallback when upstream is absent.
- [ ] Tests:
  - [x] uses upstream range when available
  - [x] bounded fallback works without upstream

**Definition of Done**

- Broken links can be healed even when the rename was already committed locally.

**Completion**: 2026-01-23 — branch: `shaman-healer`

### 6) Ambiguity + safety

- [ ] Multiple-candidate handling:
  - [ ] if a broken target maps to multiple plausible new targets, do not rewrite; emit a warning/result entry.
- [ ] Scope enforcement:
  - [ ] only heal within configured docs roots
  - [ ] do not rewrite external links, UID alias links, or Hugo shortcodes

**Definition of Done**

- Healer never rewrites links to out-of-scope targets.

### 7) Verification gate

- [ ] `go test ./... -count=1`
- [ ] `golangci-lint run --fix` then `golangci-lint run`

## Notes / Risks

- Current `applyLinkUpdates` is filename-focused (basename replace). For moved files, the rewrite must compute a correct new relative path; this should be implemented as a separate function and covered by tests.
- Avoid O(N renames × M markdown files) behavior; the broken-link list is the natural work queue.