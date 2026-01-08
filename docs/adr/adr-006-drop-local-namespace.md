---
tags: []
categories: []
id: 9710e166-77be-47f6-9c25-5ff6fa0d0825
---

# ADR-006: Drop "local" Namespace for Single-Project Preview and Build

**Status**: Proposed  
**Date**: 2026-01-02  
**Decision Makers**: DocBuilder Core Team  
**Technical Story**: Simplify content structure for preview and build commands

## Context and Problem Statement

The `preview` and `build` commands are designed for single-project documentation scenarios. Currently, both commands create content with a "local" namespace, resulting in a path structure like:

```
content/local/api/guide.md
content/local/getting-started.md
content/local/adr/index.html
```

This namespace adds unnecessary complexity for local, single-project use cases where there's no need for repository organization. In contrast, multi-repository builds properly use repository names for organization:

```
content/repo-a/api/guide.md
content/repo-b/getting-started.md
```

The "local" namespace creates:
1. **Cognitive overhead**: Users see "local" paths that serve no organizational purpose
2. **Menu navigation clutter**: Navigation generated from directory structure includes unnecessary nesting
3. **Navigation links**: Links in generated HTML reference `/local/path` creating the appearance of namespace pollution
4. **Inconsistency**: Single-project builds shouldn't behave differently from how multi-repo builds would without namespace

## Decision

We will **drop repository namespacing for single-repository builds**, producing cleaner content structures:

```
content/api/guide.md
content/getting-started.md
content/adr/index.html
```

This simplification will be achieved by detecting single-repository builds and skipping namespace generation, rather than relying on magic strings.

## Implementation Details

### Solution: Detect Single-Repository Builds

Instead of using "local" as a magic string, detect when there's only one repository in the entire build:

```go
// GetHugoPath returns the Hugo-compatible path for this documentation file.
func (df *DocFile) GetHugoPath(isSingleRepo bool) string {
    // Path shapes:
    //   Single repository:                content/{section}/{name}.md
    //   Multiple repos, single forge:     content/{repository}/{section}/{name}.md
    //   Multiple forges:                  content/{forge}/{repository}/{section}/{name}.md
    
    parts := []string{"content"}
    
    // Only add forge namespace if multiple forges exist
    if df.Forge != "" {
        parts = append(parts, strings.ToLower(df.Forge))
    }
    
    // Skip repository namespace for single-repository builds
    if !isSingleRepo {
        parts = append(parts, strings.ToLower(df.Repository))
    }
    
    if df.Section != "" {
        parts = append(parts, strings.ToLower(df.Section))
    }
    
    parts = append(parts, strings.ToLower(df.FileName))
    return strings.Join(parts, "/")
}
```

The `isSingleRepo` flag is computed at discovery time:
```go
isSingleRepo := len(uniqueRepositories) == 1
```

### Why This Approach

**Pros:**
- ✅ No magic strings or special cases
- ✅ Works for ANY single-repository scenario (not just "local")
- ✅ Semantically correct (namespace exists only when needed for disambiguation)
- ✅ User can have a config with one repository and get clean paths
- ✅ No conflict with "local" forge type
- ✅ Future-proof for single-repo configs

**Cons:**
- ⚠️ Requires passing context (`isSingleRepo`) through discovery pipeline
- ⚠️ Slightly more complex than string comparison

### Previous "local" Magic String Approach (Rejected)

Initial proposal was to check `if df.Repository != "local"`, but this is problematic because:

1. **"local" is a forge type** (`ForgeLocal`), not a repository naming convention
   - Defined in `internal/config/forge.go` as `ForgeLocal ForgeType = "local"`
   - Has dedicated client: `internal/forge/local.go` (`LocalClient`)
   - Used for development environments where docs are in current working directory
2. **Conflates concepts**: Forge type vs. repository name are different architectural layers
3. **Fragile**: What if user names their actual repository "local"?
4. **Arbitrary**: Preview/build commands hardcode `Name: "local"` with no semantic reason
   - See `cmd/docbuilder/commands/preview.go:88` and `build.go:306`
5. **Doesn't generalize**: Won't help users with single-repo configs using other names

Using single-repository detection is architecturally sound and works for all cases.

### Impact Analysis

**Files affected:**
- `internal/docs/discovery.go`: GetHugoPath() signature change (add `isSingleRepo bool` parameter)
- `internal/docs/discoverer.go`: Compute `isSingleRepo` flag during discovery
- Integration golden tests: Content structure paths change (auto-regenerated)
- No changes to command-line interfaces or configuration format

**Repositories affected:**
- Preview command: Single repo → no namespace
- Build command (local mode): Single repo → no namespace
- Single-repo configs: No namespace
- Multi-repository builds: Namespaced (unchanged)

**Tests updated:**
- Golden tests regenerated to expect new path structure
- Unit test added to verify single-repo path generation

## Consequences

### Benefits
1. **Cleaner navigation**: Users see flatter, more logical menu structures
2. **Simplified URLs**: Links and hrefs no longer include unnecessary "/local/" segments
3. **Better UX**: Single-project scenarios feel native, not namespace-polluted
4. **Future-proof**: Establishes pattern for other single-project features

### Risks and Mitigation
1. **Breaking change for users with custom Hugo templates**: Low risk since preview/build are primarily for development
   - *Mitigation*: Document in release notes
2. **API signature change**: GetHugoPath() gains parameter
   - *Mitigation*: Internal API only, no external consumers
3. **Test maintenance**: Golden files need regeneration
   - *Mitigation*: Automated via `go test -update-golden`

## Testing Strategy

1. **Unit Test**: Verify GetHugoPath() behavior with single/multi repo flag
   ```go
   func TestGetHugoPath_SingleRepo_SkipsNamespace(t *testing.T) {
       df := &DocFile{
           Repository: "my-docs",
           Section:    "api",
           FileName:   "guide.md",
       }
       
       // Single repo: no namespace
       got := df.GetHugoPath(true)
       assert.Equal(t, "content/api/guide.md", got)
       
       // Multi repo: include namespace
       got = df.GetHugoPath(false)
       assert.Equal(t, "content/my-docs/api/guide.md", got)
   }
   ```

2. **Integration Tests**: Golden tests auto-regenerated
   ```bash
   go test ./test/integration -run TestGolden -update-golden
   ```

3. **Manual Verification**:
   ```bash
   # Test preview
   docbuilder preview -d ./docs
   # Verify: site/content/{section}/{file}.md (no /local/)
   
   # Test build
   docbuilder build -d ./docs -o ./output
   # Verify: output/content/{section}/{file}.md (no /local/)
   ```

## Related Decisions

- **ADR-001**: Forge Integration and Daemon Mode — establishes multi-repo support where namespacing is essential
- This ADR reinforces that single-project and multi-project scenarios have different organizational needs

## References

- GetHugoPath Implementation: `internal/docs/discovery.go`
- Preview Command: `cmd/docbuilder/commands/preview.go`
- Build Command: `cmd/docbuilder/commands/build.go`
- Golden Test Framework: `test/integration/helpers.go`
