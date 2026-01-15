---
uid: a8168637-9de1-47a2-9d96-76d1bbf2deb5
aliases:
  - /_uid/a8168637-9de1-47a2-9d96-76d1bbf2deb5/
title: "Skip Evaluation Logic"
date: 2025-12-15
categories:
  - explanation
tags:
  - optimization
  - performance
fingerprint: a7697302e7c84c616d6dccf59b916857231e0f70f248f182e0f3ef56a07f4591
---

# Skip Evaluation System

## Overview

The skip evaluation system prevents unnecessary full rebuilds when nothing has changed. It uses a rule-based validation approach to determine if a build can be safely skipped, comparing current state against previous build artifacts and persisted metadata.

## When Builds Are Skipped

A build will be **skipped** (reusing previous `public/` output) when **all** of the following are true:

1. **No repository changes**: All repository commits match the previous build
2. **No configuration changes**: Hugo config hash is identical to previous build
3. **No version changes**: DocBuilder and Hugo versions match previous build
4. **Previous build exists**: Valid `build-report.json` and `public/` directory exist
5. **Content integrity**: All content files and their hashes match previous build

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

### Components

```
┌────────────────────────────────────────────────────────┐
│                    BuildService                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │            SkipEvaluatorFactory                  │  │
│  │  Creates evaluator with:                         │  │
│  │  - Output directory                              │  │
│  │  - State manager (commit/config tracking)        │  │
│  │  - Hugo generator (config hash computation)      │  │
│  └──────────────────────────────────────────────────┘  │
│                           │                            │
│                           ▼                            │
│  ┌──────────────────────────────────────────────────┐  │
│  │         SkipEvaluator (daemon wrapper)           │  │
│  │  Delegates to validation-based evaluator         │  │
│  └──────────────────────────────────────────────────┘  │
│                           │                            │
│                           ▼                            │
│  ┌──────────────────────────────────────────────────┐  │
│  │    validation.SkipEvaluator (core logic)         │  │
│  │  Executes validation rule chain                  │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

### Validation Rules

Rules are executed in order, with early exit on first failure:

```go
// Phase 1: Basic prerequisites
BasicPrerequisitesRule    // State manager, generator, repos exist
ConfigHashRule           // Configuration unchanged
PublicDirectoryRule      // Previous build artifacts exist

// Phase 2: Previous build validation
PreviousReportRule       // build-report.json exists and valid

// Phase 3: Change detection
VersionMismatchRule      // DocBuilder + Hugo versions match
ContentIntegrityRule     // File tree structure unchanged
GlobalDocHashRule        // Overall content hash unchanged
PerRepoDocHashRule       // Per-repository content hashes match
CommitMetadataRule       // All repository commits match
```

### Rule Validation Pattern

Each rule implements this interface:

```go
type Rule interface {
    Name() string
    Validate(ctx Context) Result
}

type Result struct {
    Passed bool
    Reason string  // Why validation failed
}
```

Rules have access to:
- **Context.State**: Persisted commit/config metadata
- **Context.Generator**: Current Hugo configuration
- **Context.Repos**: Current repository list
- **Context.OutDir**: Output directory path
- **Context.PrevReport**: Previous build report (loaded by PreviousReportRule)

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

The skip evaluator relies on the **StateManager** to track:

```go
type DaemonStateManager interface {
    // Configuration tracking
    GetLastConfigHash() string
    SaveConfigHash(hash string) error
    
    // Repository tracking
    GetLastCommit(repoName string) string
    SaveCommit(repoName, commitSHA string) error
    
    // Document hashing
    GetLastDocHash(repoName string) string
    SaveDocHash(repoName, hash string) error
}
```

State is persisted in `/data/state/daemon-state.json` and survives daemon restarts.

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

### Factory Pattern

The daemon uses a factory to create the skip evaluator with late binding:

```go
WithSkipEvaluatorFactory(func(outputDir string) build.SkipEvaluator {
    if daemon.stateManager == nil {
        return nil  // Not initialized yet
    }
    gen := hugo.NewGenerator(daemon.config, outputDir)
    inner := NewSkipEvaluator(outputDir, daemon.stateManager, gen)
    return &skipEvaluatorAdapter{inner: inner}
})
```

This allows:
1. **Lazy creation**: Evaluator created only when needed during build
2. **Late binding**: State manager initialized after build service creation
3. **Type adaptation**: Bridge typed daemon.SkipEvaluator to generic build.SkipEvaluator

### Type Adapter

The `skipEvaluatorAdapter` bridges the type gap:

```go
// daemon.SkipEvaluator (typed)
Evaluate(repos []config.Repository) (*hugo.BuildReport, bool)

// build.SkipEvaluator (generic)
Evaluate(repos []any) (report any, canSkip bool)
```

The adapter performs runtime type checking and conversion.

## Testing

### Unit Tests

Validation rules are tested in isolation:

```go
func TestVersionMismatchRule(t *testing.T) {
    ctx := Context{
        State: &mockState{
            lastVersion: "1.0.0",
            lastHugoVersion: "0.120.0",
        },
    }
    
    // Current version differs
    version.SetVersion("1.0.1")
    
    result := VersionMismatchRule{}.Validate(ctx)
    assert.False(t, result.Passed)
    assert.Contains(t, result.Reason, "version mismatch")
}
```

### Integration Tests

End-to-end skip behavior is tested in daemon integration tests:

```go
func TestDaemon_SkipUnchangedBuilds(t *testing.T) {
    // First build - should run fully
    result1 := daemon.Build(ctx, req)
    assert.Equal(t, BuildStatusSuccess, result1.Status)
    
    // Second build - same repos, should skip
    result2 := daemon.Build(ctx, req)
    assert.Equal(t, BuildStatusSuccess, result2.Status)
    assert.True(t, result2.Skipped)  // Build was skipped
}
```

## Performance Impact

Skip evaluation adds minimal overhead:

1. **Config hash computation**: ~5ms (YAML marshaling + SHA256)
2. **File tree scan**: ~10-50ms (depends on content size)
3. **State manager lookups**: ~1ms (in-memory with disk cache)
4. **Rule validation**: ~20-100ms total

**Total overhead**: ~50-200ms vs. **full rebuild**: 5-30 seconds

The cost of skip validation is negligible compared to git operations and Hugo rendering.

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
2. Verify state manager is initialized (`/data/state/daemon-state.json` exists)
3. Check logs for skip validation failures
4. Ensure `public/` directory and `build-report.json` exist from previous build

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
