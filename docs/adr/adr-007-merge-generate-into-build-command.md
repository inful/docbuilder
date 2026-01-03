# ADR-007: Merge Generate Command into Build Command

**Status**: Accepted  
**Date**: 2026-01-03  
**Implementation Date**: 2026-01-03  
**Decision Makers**: DocBuilder Core Team  
**Technical Story**: Simplify command-line interface by consolidating redundant commands

## Context and Problem Statement

DocBuilder currently has two commands for building documentation sites:

1. **`build`** - Full-featured build with Git integration, multi-repo support, config files
2. **`generate`** - Simplified CI/CD generator for single local directories

Analysis reveals that these commands have **significant functional overlap**:

**Shared functionality:**
- Both discover documentation files from local directories
- Both generate Hugo sites with themes
- Both support custom output directories, titles, and base URLs
- Both create minimal in-memory configs when needed

**Current differences:**
- `generate` has `--render` boolean flag (skip Hugo execution)
- `generate` uses temporary directory workflow with public/ copy
- `build` supports `--incremental`, `--render-mode`, `--relocatable`, `--keep-workspace`
- `build` has local fallback mode that mirrors `generate` functionality

**Key insight**: When `build` runs without a config file, it already implements the same workflow as `generate`:
```go
// build.go - runLocalBuild()
// This is functionally identical to generate command
discovery := docs.NewDiscovery(repos, &cfg.Build)
docFiles, err := discovery.DiscoverDocs(repoPaths)
generator := hugo.NewGenerator(cfg, outputDir)
err := generator.GenerateSite(docFiles)
```

The existence of two commands creates:
1. **Confusion**: Users must choose between `build` and `generate` for simple use cases
2. **Maintenance burden**: ~170 lines of duplicate code to maintain
3. **API inconsistency**: Similar operations with different flag names (`--render` vs `--render-mode`)
4. **Split documentation**: Two commands to document and support

## Decision

We will **merge the `generate` command into `build`** by:

1. **Removing** the `generate` command entirely (project is greenfield, no backward compatibility needed)
2. **Using** existing `--render-mode=never` flag for `--render=false` use case (already supported)
3. **Simplifying** mental model: `build` handles all build scenarios

## Rationale

### Why Merge into `build` (not the reverse)?

1. **`build` is more feature-complete**: Supports incremental mode, versioning, multi-repo
2. **`build` is the primary command**: Used in production, has richer config support
3. **Natural hierarchy**: `build` encompasses all building scenarios (local, remote, multi-repo)
4. **Less breaking**: Most users likely use `build`; `generate` was designed for CI/CD niche

### Why Not Keep Both?

1. **Semantic distinction is weak**: Both "build" and "generate" mean the same thing (create documentation site)
2. **Implementation proves redundancy**: Local mode in `build` already duplicates `generate`
3. **User confusion**: No clear rule for when to use which command
4. **Maintenance cost**: Every feature must be implemented twice or users get inconsistent experience

## Implementation Plan

Since the project is still greenfield, we can remove `generate` immediately:

1. **Delete** `cmd/docbuilder/commands/generate.go`
2. **Remove** `generate` from command registration in `main.go`
3. **Update** all documentation and examples
4. **Update** README with clear build command usage
5. **Verify** CI/CD examples use `build` command

### Command Usage Guide

Users who might have used `generate` should use `build` instead:

```bash
# Old: generate command
docbuilder generate -d ./docs -o ./public --render=true

# New: build command (identical functionality)
docbuilder build -d ./docs -o ./public

# Old: generate without rendering
docbuilder generate -d ./docs -o ./hugo-project --render=false

# New: build with render-mode
docbuilder build -d ./docs -o ./hugo-project --render-mode=never
```

**Flag mapping:**
| `generate` flag | `build` equivalent |
|-----------------|-------------------|
| `-d, --docs-dir` | `-d, --docs-dir` (same) |
| `-o, --output` | `-o, --output` (same) |
| `--title` | `--title` (same) |
| `--base-url` | `--base-url` (same) |
| `--render=false` | `--render-mode=never` |
| `--render=true` | (default behavior) |

## Consequences

### Benefits

1. **Simpler CLI**: One command for all build scenarios
2. **Reduced code**: Remove ~170 lines of duplicate code
3. **Consistent UX**: Unified flag names and behavior across all build modes
4. **Easier maintenance**: Single code path to test, document, and enhance
5. **Clearer mental model**: "Build from local? Use `build`. Build from remote? Use `build`."
6. **Better discoverability**: New users find one obvious command instead of choosing between two

### Risks and Mitigation

1. **Breaking change for early adopters**
   - **Risk**: Minimal - project is greenfield with limited users
   - **Mitigation**: Update all documentation and examples immediately
   - **Mitigation**: Clear release notes explaining the consolidation

2. **Lost semantic clarity**
   - **Risk**: Some users might prefer "generate" name for CI/CD
   - **Mitigation**: Documentation shows clear "build for CI/CD" examples
   - **Mitigation**: `build` command is intuitive and self-explanatory

## Testing Strategy

1. **Add integration tests** verifying `build` local mode produces expected output
2. **Verify** all CI/CD examples in docs work with `build` command
3. **Update** any existing tests that referenced `generate` command
4. **Ensure** help text and documentation reflect removal

## Alternatives Considered

### Alternative 1: Merge `build` into `generate`

**Rejected** because:
- `generate` is less feature-complete (no incremental, no versioning)
- `build` is the primary, more widely-used command
- Would require renaming "build" to "generate" in all documentation
- Name "build" is more intuitive than "generate" for the main command

### Alternative 2: Keep both, but make `generate` call `build` internally

**Rejected** because:
- Still maintains two commands in user-facing API
- Doesn't reduce maintenance burden (two sets of docs, two help texts)
- Users still confused about which to use
- Unnecessary complexity for a greenfield project

### Alternative 3: Make `generate` a subcommand of `build`

Example: `build generate` or `build local`

**Rejected** because:
- Adds unnecessary nesting (no other subcommands exist)
- `build -d ./docs` is simpler than `build local -d ./docs`
- Flags already disambiguate behavior (presence of `-c` vs `-d`)

### Alternative 4: Keep both, clearly document use cases

**Rejected** because:
- Doesn't solve code duplication
- Relies on users reading documentation (many won't)
- Semantic distinction is too subtle to enforce
- Creates unnecessary decision point for users

## Related Decisions

- **ADR-006**: Drop local namespace for single-repo builds - This ADR's single-repo detection aligns with unified build command
- **ADR-001**: Golden Testing Strategy - Consolidation reduces test surface area

## References

- [Kong CLI framework](https://github.com/alecthomas/kong) - Used for command-line parsing
