---
aliases:
  - /_uid/a8168637-9de1-47a2-9d96-76d1bbf2deb5/
categories:
  - explanation
date: 2025-12-15T00:00:00Z
fingerprint: 535ec36f5ab5f525794d73c89492c7abb30489ca7d9c819c1f7b677028c94b74
lastmod: "2026-11-24"
tags:
  - optimization
  - performance
title: Skip Evaluation Logic
uid: a8168637-9de1-47a2-9d96-76d1bbf2deb5
---

## Overview

The skip evaluation system prevents unnecessary full rebuilds when nothing has changed. It uses a rule-based validation approach to determine if a build can be safely skipped, comparing current state against previous build artifacts and persisted metadata.

## When Builds Are Skipped

A build will be **skipped** (reusing previous `public/` output) when **all** of the following are true:

1. **No repository changes**: All repository commits match the previous build
2. **No configuration changes**: Hugo config hash is identical to previous build
3. **No version changes**: DocBuilder and Hugo versions match previous build
4. **Previous build exists**: Valid `build-report.json` and `public/` directory exist
5. **The repositories are the same**: The repositories of the build are the ones of the last build, and the published commits are the ones of the workspace

## When Builds Are Forced

A **full rebuild** will occur when **any** of these conditions are detected:

| Condition | Why Rebuild? |
|-----------|-------------|
| Repository added/removed | Content structure changed |
| Repository updated (new commits) | Documentation content changed |
| DocBuilder version changed | New features, bug fixes, compatibility |
| Hugo version changed | Rendering engine updates |
| Configuration changed | Parameters, theme settings, URLs, etc. |
| Previous build missing/corrupt | Cannot validate skip safety |
| Content file modified outside git | Integrity violation |

## Architecture

### Where the decision is made

The decision is made by the build pipeline itself, right after the
repositories have been cloned. The pipeline knows the commit each
repository was at before the clone and the commit it is at now, so it
can tell whether anything moved. It does not need a second, separate
evaluation pass in front of the build: a check that runs before the
clone cannot know what the repositories look like now, and a check that
cannot know that must never say "nothing changed".

```
┌────────────────────────────────────────────────────────┐
│                    Build pipeline                      │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Stage: clone                                     │  │
│  │  Reads the commit of each clone before the update  │  │
│  │  and after it. Both are in the build state.        │  │
│  └──────────────────────────────────────────────────┘  │
│                           │                            │
│                           ▼                            │
│  ┌──────────────────────────────────────────────────┐  │
│  │  All repositories unchanged?                      │  │
│  │  → the previous output is still valid?            │  │
│  │  → the configuration is unchanged?                │  │
│  │  → the versions are unchanged?                    │  │
│  │  → skip the rest of the pipeline                  │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

### The checks

The pipeline skips the remaining stages when all of these hold:

1. **The repositories did not move.** Every repository's commit after the
   clone is the commit it had before.
2. **The configuration is unchanged.** The configuration hash of the
   current configuration matches the one the last build stored.
3. **The versions are unchanged.** The build report of the last build
   was written by the same DocBuilder and Hugo versions.
4. **The previous output is still usable.** The last build was not a
   failure, it found documentation files, and the output directory holds
   a rendered `public/index.html` and markdown content.
5. **The repositories are the same.** The repositories of the build are
   the ones of the last build - a repository that was removed from the
   configuration is not in the list of the commits any more - and the
   commits of the published site are the ones of the workspace.

Anything else - a moved repository, a changed configuration, a new
version, a broken output directory - means the build runs.

### What the state holds

The daemon's state file records the commit each repository was built at,
the configuration hash of that build, the number of documents of each
repository and the hashes of the document files. The pipeline writes
them after a build and reads the commits, the configuration hash and
the repositories back to make the decision.

## Configuration

### Enabling Skip Evaluation

**Daemon Mode** (enabled by default):
```yaml
build:
  skip_if_unchanged: true  # Default for daemon
```

**CLI Mode** (opt-in):
```yaml
build:
  skip_if_unchanged: false  # Default for CLI
```

### Configuration Hash

The config hash is computed from:
- Hugo configuration (title, base_url, theme, params, etc.)
- Build configuration (render mode, skip settings, etc.)
- Repository list (URLs, branches, paths, auth)

Changes to **any** of these trigger a rebuild.

## State Persistence

The daemon's state manager keeps what the pipeline needs to decide:

```go
type DaemonStateManager interface {
    // Configuration tracking
    GetLastConfigHash() string
    SetLastConfigHash(hash string)

    // Repository tracking
    GetRepoLastCommit(repoURL string) string
    SetRepoLastCommit(repoURL, name, branch, commit string)

    // Document hashing
    GetRepoDocFilesHash(repoURL string) string
    SetRepoDocFilesHash(repoURL, hash string)
    GetLastGlobalDocFilesHash() string
    SetLastGlobalDocFilesHash(hash string)
}
```

State is persisted in the daemon's state directory and survives daemon
restarts.

## Build Report

When a build is skipped, the evaluator returns the **previous build report** unmodified:

```json
{
  "status": "success",
  "timestamp": "2024-01-15T10:30:00Z",
  "repositories": [
    {
      "name": "myrepo",
      "commit": "abc123def456",
      "docs_found": 42,
      "errors": []
    }
  ],
  "checksum": "sha256:..."
}
```

The caller cannot distinguish a skipped build from a successful build - this is intentional for idempotency.

## Integration with Daemon

### Wiring

The daemon passes its state manager directly via `BuildRequest.StateManager`:

```go
req := build.BuildRequest{
    Config:       cfg,
    OutputDir:    outDir,
    StateManager: daemon.stateManager,  // where the commits are recorded
    Options:      build.BuildOptions{SkipIfUnchanged: cfg.Build.SkipIfUnchanged},
}
```

The pipeline writes the commit each repository was built at into that
state manager, and reads it back on the next build. The configuration
decides whether a build may be skipped at all: with
`skip_if_unchanged` off, every build runs.

There is no factory, no wrapper, and no adapter layer between the daemon and
`BuildService` — the state manager is the only seam.

## Testing

### Unit Tests

The pipeline's decision is tested where it is made:

```go
func TestBuild_SkipsUnchangedRepository(t *testing.T) {
    // First build - runs fully
    first, err := svc.Run(ctx, req)
    require.NoError(t, err)
    require.False(t, first.Skipped)

    // Second build - same repositories, nothing moved
    second, err := svc.Run(ctx, req)
    require.NoError(t, err)
    require.True(t, second.Skipped)
}
```

### The other half

A build after a change must **not** be skipped. This is the test that
keeps the site honest:

```go
func TestBuild_AfterChangeIsNotSkipped(t *testing.T) {
    // First build, then the repository gets a new document
    addDocToRepo(t, repoDir, "b.md", "# B\n")

    second, err := svc.Run(ctx, req)
    require.NoError(t, err)
    require.False(t, second.Skipped)
}
```

## Performance Impact

The decision costs a clone of the repositories (which the daemon already
does) and a few reads of the state file. Everything the pipeline would
do afterwards - discovery, transformation, Hugo - is skipped.

**Total overhead**: a few milliseconds against a **full rebuild** of
5-30 seconds.

## Logging

Skip decisions are logged at INFO level:

```
# Skip successful
INFO Build skipped - no changes detected
  repositories=3 config_hash=abc123 version=1.2.3

# Skip failed - version changed
INFO Build required - version mismatch
  previous_version=1.2.2 current_version=1.2.3

# Skip failed - repo updated
INFO Build required - repository changes detected
  repository=myrepo previous_commit=abc123 current_commit=def456

# Skip failed - config changed
INFO Build required - configuration changed
  previous_hash=abc123 current_hash=def456
```

## Troubleshooting

### Skip Not Working

**Symptom**: Builds always run fully even when nothing changed

**Diagnosis**:
1. Check `skip_if_unchanged` is enabled in config
2. Verify the state manager is initialized (the daemon's state file exists)
3. Check the logs for `No repository head changes detected`
4. Ensure `public/index.html` and `build-report.json` exist from the previous build

**Common Causes**:
- Configuration changes not reflected in config hash
- Timestamps in config (use static values)
- File system changes outside git (edited files directly)
- State file corruption or deletion

### False Skips

**Symptom**: Build skipped but content appears outdated

**Diagnosis**:
1. Check repository commits match: `git log -1 --format=%H`
2. Verify content file integrity (no manual edits)
3. Compare config hashes (previous vs. current)

**Common Causes**:
- Files edited outside git (breaks content integrity)
- Force-pushed branches (commit SHA same but content differs)
- Symlinked content (changes not tracked)

## Future Enhancements

Potential improvements to the skip system:

1. **Partial rebuilds**: Skip unchanged repos, rebuild only changed ones
2. **Content diffing**: Detect file-level changes without full tree scan
3. **Incremental Hugo**: Use Hugo's `--gc` and caching for faster builds
4. **Parallel validation**: Run rules concurrently for large repositories
5. **Skip hints**: Allow repos to declare "always rebuild" vs. "safe to skip"

## Related Documentation

- [ADR-002: In-Memory Content Pipeline](../adr/ADR-002-in-memory-content-pipeline.md)
- [Incremental Builds](../how-to/run-incremental-builds.md)
- [Build Service Architecture](architecture.md)
