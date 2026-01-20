---
uid: 93bcd5b0-7d17-48c0-ac61-e41e2ae93baf
aliases:
  - /_uid/93bcd5b0-7d17-48c0-ac61-e41e2ae93baf/
title: "ADR-012: Link-Safe File Normalization"
date: 2026-01-20
categories:
  - architecture-decisions
tags:
  - linting
  - refactor
  - file-system
  - links
fingerprint: 77b435d1d6a32e5d38ef388679752e8e8308d6fd8640e950ba3cde8bad676713
lastmod: 2026-01-20
---

# ADR-012: Link-Safe File Normalization

**Status**: Accepted  
**Date**: 2026-01-20  
**Decision Makers**: DocBuilder Core Team  

## Context and Problem Statement

DocBuilder's linting system ([ADR-005](adr-005-documentation-linting.md)) identifies violations of filename conventions (e.g., spaces, uppercase characters, non-kebab-case names). While these violations are reported as errors, fixing them manually is a complex task because renaming a Markdown file breaks all internal links pointing to it from other files within the same repository.

To maintain a healthy documentation set, we need an automated way to rename files to match conventions while ensuring that no links are broken in the process.

## Decision

We will implement a link-aware self-healing system integrated into the existing `docbuilder lint --fix` command. This system will utilize Git for advanced features like history-based link healing and reliable rollbacks when available.

To maintain consistency with the rest of the DocBuilder codebase, the implementation will:
- Use the `internal/foundation/errors` package for uniform error reporting ([ADR-000](adr-000-uniform-error-handling.md)).
- Leverage the `github.com/go-git/go-git/v5` library (already utilized in `internal/git`) for repository state inspection and history-based rename detection.
- Gracefully skip structural operations if not running within a Git repository.

### 1. Unified Healing Workflow

The `docbuilder lint --fix` command will be the single entry point for maintaining structural integrity:

1.  **Automated Normalization**: If the linter identifies a filename violation (e.g., uppercase, spaces), the fixer will attempt to perform a `git mv` to a valid name and simultaneously update all inbound links.
2.  **External Rename Recovery**: If the linter finds a broken relative link and is running in a Git repo, it will consult the Git index/history to identify if the target was renamed, then heal the link.

### 2. Git Integration and Graceful Degradation

While Git is the preferred mechanism for structural changes, the system is designed to degrade gracefully:

- **Git-Aware Renaming**: All renames are performed using `git mv` when available to preserve file history and metadata.
- **Git-Based Rollback**:
    - If a Git repository is detected, operations (renames and content edits) are staged. If an error occurs, the fixer automatically rolls back all changes using `git checkout` or `git reset`, returning the workspace to its exact state before the transaction began.
    - If no Git repository is found, the system skips structural normalization and link healing phases. Other linter fixes that do not depend on file moving (such as frontmatter updates or formatting) will still proceed. This ensures the tool remains useful for local development and non-Git documentation sets.
- **Commit Boundary**: The fixer does not perform Git commits; it leaves changes staged for the user to review and commit.

### 3. Repository-Scoped Link Discovery

The fixer needs a view of links within the local documentation repository.
- **Scan Phase**: Before performing renames, the fixer scans Markdown files in the local path to build a map of link references.
- **Inbound Tracking**: For every file slated for renaming, the fixer identifies all "inbound" links pointing to it from other files in the same repository.

### 4. Safe Content Updates

When updating links within a Markdown file:
- **Reverse Order**: Updates are applied from the bottom of the file to the top (descending line numbers). This ensures that modifying a line does not invalidate the line numbers for subsequent updates in the same file.
- **Atomic Write**: Updated content is written to a temporary file which is then used to replace the original file, ensuring that the file is always in a valid state on disk.

### 5. Name Mapping and Validation

The system relies on the core linting standards to drive automated renames.

- **Standard Verification**: Every rename performed by the fixer must adhere to the naming conventions defined in [ADR-005](adr-005-documentation-linting.md) (lowercase, kebab-case, no special characters).
- **Collision Prevention**: The system must verify that the target destination does not already exist and is not part of another pending rename operation in the same transaction.

### 6. Git-Aware Link Recovery

The `docbuilder lint --fix` command will be extended to automatically repair broken relative links by consulting the Git history. 

- **Heuristic Recovery**: If a relative link points to a non-existent file, the fixer will inspect the latest Git commit(s) to check if the target file was recently renamed.
- **Git Rename Detection**: The system will use `git log --summary` or `git diff --name-status -M` to identify files that were moved or renamed.
- **Link Healing**: If a rename match is found (e.g., `OldName.md` moved to `old-name.md`), the fixer will automatically rewrite the broken link in the source file to point to the new location.
- **Scope**: This recovery is primarily focused on the most recent changes to ensure that link breaking is caught immediately after a structural change that might have happened outside of the `docbuilder mv` command (e.g., manual `git mv` or IDE-based renames).

## Consequences

### Pros

- **Consistency**: Ensures all documentation follows the same naming conventions without manual toil.
- **Reliability**: Guarantees that automated fixes do not introduce broken links.
- **Git-Powered Recovery**: Utilizes native Git operations for reliable rollbacks when available.
- **Self-Healing**: Automatically repairs broken links by detecting external renames via Git history.
- **Graceful Degradation**: Safely skips complex structural fixes when running in non-Git environments without failing the tool.
- **Preservation**: Maintains Git history, which is crucial for long-lived documentation projects.
- **Developer Experience**: Allows developers to focus on content while the tooling handles structural normalization.

### Cons

- **Conditional Functionality**: Link healing and automated renames are only available in Git-managed repositories.
- **Complexity**: The `lint` package becomes significantly more complex to support transactional multi-file updates and Git integration.
- **Performance**: Scanning all files for links and querying Git history can be slow on very large documentation sets.
- **Edge Cases**: Complex relative paths (e.g., those involving symlinks or deep nesting) require careful handling.

### Implementation and Reuse Strategy

DocBuilder already possesses significant infrastructure for file operations and link detection. The implementation will heavily reuse and refactor existing components rather than building from scratch.

- **`internal/lint/fixer.go`**: Reused as the central orchestration point. The existing `gitAware` logic will be enhanced to use `internal/git` instead of direct shell execution.
- **`internal/lint/fixer_broken_links.go`**: The `detectBrokenLinks` function will be the foundation for the "External Rename Recovery" phase. It will be extended to pass broken links to a new history inspector.
- **`internal/lint/fixer_file_ops.go`**: The existing `renameFile`, `shouldUseGitMv`, and `gitMv` functions will be refactored to support the atomic transaction requirement, likely replacing `os/exec` with `internal/git` calls for consistency.
- **`internal/lint/fixer_link_updates.go`**: Existing logic for rewriting links will be leveraged to handle the actual content updates during both healing and normalization.

## Implementation References

- `internal/lint/fixer.go`: Core orchestration logic using `internal/foundation/errors`.
- `internal/lint/fixer_file_ops.go`: `git mv` and Git-based rollback operations.
- `internal/lint/fixer_link_updates.go`: Atomic multi-file link rewriting.
- `internal/lint/fixer_link_detection.go`: Repository-scoped link discovery and `go-git` history tracking.
- `internal/git/errors.go`: Shared error classification for Git-related failures.
