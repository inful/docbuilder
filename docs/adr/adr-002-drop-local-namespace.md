# ADR-002: Drop "local" Namespace for Single-Project Preview and Build

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

We will **drop the "local" repository namespace** for single-project preview and build commands, producing cleaner content structures:

```
content/api/guide.md
content/getting-started.md
content/adr/index.html
```

This simplification will be achieved by treating "local" as a special case in the path generation logic rather than modifying command-level code.

## Implementation Details

### Solution: Conditional Namespace in GetHugoPath()

Modify the `GetHugoPath()` function in `internal/docs/discovery.go` to skip the repository name when it equals "local":

```go
// GetHugoPath returns the Hugo-compatible path for this documentation file.
func (df *DocFile) GetHugoPath() string {
    // Path shape:
    //   Single forge (no namespace):     content/{repository}/{section}/{name}.md
    //   Multiple forges:                  content/{forge}/{repository}/{section}/{name}.md
    //   Local (single-project):           content/{section}/{name}.md  (skips "local")
    parts := []string{"content"}
    
    if df.Forge != "" {
        parts = append(parts, strings.ToLower(df.Forge))
    }
    
    // Skip "local" repository namespace (used for preview/build single projects)
    if df.Repository != "local" {
        parts = append(parts, strings.ToLower(df.Repository))
    }
    
    if df.Section != "" {
        parts = append(parts, strings.ToLower(df.Section))
    }
    
    parts = append(parts, strings.ToLower(df.FileName))
    return strings.Join(parts, "/")
}
```

### Why This Approach

**Pros:**
- ✅ Minimal code change (single location, single conditional)
- ✅ Backward compatible (existing multi-repo builds unaffected)
- ✅ Semantically clear ("local" has explicit special meaning)
- ✅ No public API changes
- ✅ No command-level changes required
- ✅ Scales to future single-project scenarios

**Cons:**
- ⚠️ "local" becomes a reserved repository name (acceptable trade-off given its use is internal-only)

### Impact Analysis

**Files affected:**
- `internal/docs/discovery.go`: GetHugoPath() modification
- Integration golden tests: Content structure paths change (auto-regenerated)
- No changes to command-line interfaces or configuration format

**Repositories affected:**
- Preview command (local mode): Uses "local" repo name
- Build command (local mode): Uses "local" repo name
- Multi-repository builds: Unaffected (use actual repo names)

**Tests updated:**
- Golden tests regenerated to expect new path structure
- Unit test added to verify "local" namespace skipping

## Consequences

### Benefits
1. **Cleaner navigation**: Users see flatter, more logical menu structures
2. **Simplified URLs**: Links and hrefs no longer include unnecessary "/local/" segments
3. **Better UX**: Single-project scenarios feel native, not namespace-polluted
4. **Future-proof**: Establishes pattern for other single-project features

### Risks and Mitigation
1. **Breaking change for users with custom Hugo templates**: Low risk since preview/build are primarily for development
   - *Mitigation*: Document in release notes
2. **Confusion about "local" keyword**: "local" is never exposed in config or CLI
   - *Mitigation*: Internal implementation detail only
3. **Test maintenance**: Golden files need regeneration
   - *Mitigation*: Automated via `go test -update-golden`

## Testing Strategy

1. **Unit Test**: Verify GetHugoPath() behavior
   ```go
   func TestGetHugoPath_Local_SkipsNamespace(t *testing.T) {
       df := &DocFile{
           Repository: "local",
           Section:    "api",
           FileName:   "guide.md",
       }
       expected := "content/api/guide.md"
       assert.Equal(t, expected, df.GetHugoPath())
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

- [GetHugoPath Implementation](internal/docs/discovery.go)
- [Preview Command](cmd/docbuilder/commands/preview.go)
- [Build Command](cmd/docbuilder/commands/build.go)
- [Golden Test Framework](test/integration/helpers.go)
