---
aliases:
  - /_uid/93bcd5b0-7d17-48c0-ac61-e41e2ae93baf/
categories:
  - architecture-decisions
date: 2026-01-20T00:00:00Z
fingerprint: b268ba1564258800f28d157f1e8949412aa6d3d8fd6269bd3a0ead1d062ceb94
lastmod: "2026-01-22"
tags:
  - linting
  - refactor
  - file-system
  - links
uid: 93bcd5b0-7d17-48c0-ac61-e41e2ae93baf
---

# ADR-012: Autoheal links to files moved

**Status**: Accepted  
**Date**: 2026-01-20  
**Decision Makers**: DocBuilder Core Team  

## Context and Problem Statement

DocBuilder's linting system ([ADR-005](adr-005-documentation-linting.md)) identifies violations of filename conventions (e.g., spaces, uppercase characters, non-kebab-case names). Users often rename files manually or via other tools to fix these violations, which frequently breaks internal relatives links pointing to those files.

To maintain a healthy documentation set, we need a system that detects these structural changes and automatically heals the broken links, rather than forcing the user to manually hunt down every reference.

## Decision

We will implement a link-aware self-healing system integrated into the existing `docbuilder lint --fix` command. This system will utilize Git history to detect file renames and heal broken links.

To maintain consistency with the rest of the DocBuilder codebase, the implementation will:
- Use the `internal/foundation/errors` package for uniform error reporting ([ADR-000](adr-000-uniform-error-handling.md)).
- Leverage the `github.com/go-git/go-git/v5` library for repository state inspection and history-based rename detection.
- Gracefully skip healing operations if not running within a Git repository.

### 1. Unified Healing Workflow

The `docbuilder lint --fix` command will focus on maintaining referential integrity:

1.  **Reactive Link Healing**: If the linter finds a broken relative link and is running in a Git repo, it will consult Git state (including uncommitted changes) and recent history to identify if the target was renamed.
2.  **Update References**: If a rename is detected (e.g. `OldFile.md` -> `new-file.md`), the fixer will update the broken link to point to the new location.

### 2. Git Integration and Graceful Degradation

- **Git-Based Detection**: The system relies on Git state and history to determine if a missing file was actually moved.
- **Uncommitted Renames**: Healing should work for renames that have not been committed yet (e.g., `git mv` in the working tree/index), which is the common case when running `docbuilder lint --fix` in a pre-commit workflow.
- **No Git Access**: If no Git repository is found, the link healing phase is skipped. Other fixes (like frontmatter updates) proceed as normal.
- **No Automated Renaming**: The system does NOT proactively rename files that violate naming conventions. It only reacts to renames that have already occurred.
- **No Rollback**: The system does not attempt to automatically rollback changes on failure. It relies on the user to manage their git state.

#### History Horizon (Pre-Commit Oriented)

- **Since last push**: When an upstream tracking branch is available, history-based detection should prefer scanning commits since the last push (i.e., changes between the current `HEAD` and the upstream branch).
- **Fallback**: If an upstream tracking branch is not available, the tool should fall back to a bounded recent history window.

### 3. Repository-Scoped Link Discovery

The fixer needs a view of links within the local documentation repository.

- **Scope**: Healing is strictly limited to files within the repository, and more strictly to content within the configured documentation root(s) (by default `docs`).
- **Scan Phase**: The fixer scans Markdown files under the configured documentation root(s) to identify broken relative links.
- **In-Scope Links**: Any relative Markdown link is in scope (including image links and reference-style links), excluding links that appear inside code blocks (fenced or indented).
- **History Lookup**: For each broken link, it queries Git state/history to see if the target path was moved within the configured documentation root(s).

#### Ambiguity Handling

- **Multiple Candidates**: If more than one plausible rename target is found, the fixer warns the user and lists all possible targets, without applying an automatic rewrite.
- **Multiple Moves**: If a file has been moved multiple times, the fixer chooses the most recent filename (provided the candidate target path exists) when applying an automatic rewrite.

### 4. Safe Content Updates

When updating links within a Markdown file:
- **Reverse Order**: Updates are applied from the bottom of the file to the top (descending line numbers). This ensures that modifying a line does not invalidate the line numbers for subsequent updates in the same file.
- **Atomic Write**: Updated content is written to a temporary file which is then used to replace the original file, ensuring that the file is always in a valid state on disk.

### 5. Implementation Details: Git-Aware Recovery

The healing logic operates by consulting Git history when a dead relative link is encountered:

- **Heuristic Recovery**: If a relative link points to a non-existent file, the fixer inspects Git changes (including uncommitted changes) and recent commits.
- **Git Rename Detection**: The system uses diffs/rename information available via `github.com/go-git/go-git/v5` to identify if the target file was moved or renamed (e.g. `OldName.md` moved to `old-name.md`).
- **Link Healing**: If a match is found, the fixer rewrites the broken link in the source file to point to the new location.
- **Scope**: Recovery is focused on changes since the last push (preferred) to catch breaks immediately after a structural change in typical pre-commit workflows.

## Acceptance Criteria

- Heals broken relative links for targets moved/renamed but not yet committed.
- Scans only within the configured documentation root(s) (default `docs`) and only rewrites links whose resolved targets remain within those roots.
- Processes any relative Markdown links outside code blocks (fenced or indented), including inline, image, and reference-style links.
- Prefers history scanning since last push when an upstream is configured; otherwise uses a bounded recent history fallback.
- If multiple rename targets are plausible, emits a warning and lists candidates without rewriting.
- If a target moved multiple times, rewrites to the most recent existing path.

## Consequences

### Pros

- **Reliability**: Guarantees that links are maintained even when files are renamed manually.
- **Self-Healing**: Automatically repairs broken links by detecting external renames via Git history.
- **Graceful Degradation**: Safely skips healing when running in non-Git environments without failing the tool.
- **Developer Experience**: Allows developers to rename files using standard tools (like `git mv`) without worrying about manual link updates.

### Cons

- **Conditional Functionality**: Link healing is only available in Git-managed repositories.
- **Performance**: Scanning all files for links and querying Git history can be slow on very large documentation sets.
- **Edge Cases**: Complex relative paths (e.g., those involving symlinks or deep nesting) require careful handling.

### Implementation and Reuse Strategy

DocBuilder already possesses significant infrastructure for file operations and link detection. The implementation will heavily reuse and refactor existing components rather than building from scratch.

- **`internal/lint/fixer.go`**: Reused as the central orchestration point. Existing `gitAware` logic will be enhanced to use `internal/git`.
- **`internal/lint/fixer_healing.go`**: New (or refactored) component dedicated to the healing logic and history inspection.
- **`internal/lint/fixer_link_updates.go`**: Existing logic for rewriting links will be leveraged to handle content updates.

## Implementation References

- `internal/lint/fixer.go`: Core orchestration logic.
- `internal/lint/fixer_healing.go`: Link healing and history lookup.
- `internal/lint/fixer_link_updates.go`: Link rewriting.
- `internal/lint/fixer_link_detection.go`: Repository-scoped link discovery.
