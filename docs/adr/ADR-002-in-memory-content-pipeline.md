# ADR-002: Fix Index Stage Pipeline Bypass

## Status

Accepted - Implemented 2025-12-13

## Context

### Current Architecture (Mostly Correct)

DocBuilder **already implements** an in-memory content pipeline with proper separation of concerns:

1. **Discovery Stage** (`stageDiscoverDocs`): Reads files from source repositories into memory once
2. **Transform Stage** (`copyContentFiles`): 
   - Loads content into memory (`file.LoadContent()`)
   - Runs dependency-ordered transform pipeline (front matter, link rewriting, etc.)
   - Processes content **entirely in memory** via `Page` struct
   - Writes final transformed output to disk **once**
3. **Index Stage** (`stageIndexes`): Generates repository/section index pages
4. **Hugo Render** (`stageRunHugo`): Runs Hugo on the prepared content tree

**The transform pipeline is already in-memory and works correctly** with:
- ✅ Single source read during discovery
- ✅ In-memory transformation via `Page` struct
- ✅ Dependency-based transform ordering
- ✅ Single write after all transforms complete
- ✅ Front matter patching and merging
- ✅ Link rewriting through the pipeline

### The Actual Problem: Index Stage Bypass

**One specific issue** exists in `/internal/hugo/indexes.go` where README.md files are promoted to `_index.md`:

The `useReadmeAsIndex` function:
1. **Re-reads the source README.md file from disk** (bypassing transformed content)
2. **Manually parses and manipulates front matter** (duplicating transform logic)
3. **Overwrites the already-transformed file** at the index location

```go
// indexes.go (current problematic code)
func (g *Generator) useReadmeAsIndex(...) {
    // ❌ Bypasses pipeline: re-reads source instead of using transformed content
    readmeContent, _ := os.ReadFile(readmeSourcePath)
    
    // ❌ Duplicates logic: manually parses front matter
    fm := parseFrontMatter(readmeContent)
    
    // ❌ Loses transforms: overwrites pipeline output with source content
    os.WriteFile(indexPath, readmeContent, 0644)
}
```

**Impact**: When README.md is promoted to `_index.md`, transformations applied by the pipeline (especially link rewrites) are lost because the index stage writes the original untransformed content.

### Why This Happened

The index stage was written before the transform pipeline was fully established. It predates the current dependency-based transform system and operates on the assumption that it needs to read source files directly.

## Decision

**Fix the index stage to use already-transformed content** instead of re-reading source files. This is a targeted fix that eliminates the pipeline bypass without requiring a full architectural refactor.

### Core Insight

The existing architecture is **already correct** - we don't need to refactor the pipeline. We only need to:
1. Capture transformed content after the pipeline runs
2. Make it available to the index stage
3. Stop re-reading source files in index generation

### Minimal Changes Required

**Change 1: Add field to track transformed content**

```go
// internal/docs/discovery.go
type DocFile struct {
    // ... existing fields ...
    Content          []byte  // Original source (already exists)
    TransformedBytes []byte  // NEW: After transform pipeline
    // ... existing fields ...
}
```

**Change 2: Capture transformed content in copyContentFiles**

```go
// internal/hugo/content_copy.go
func (g *Generator) copyContentFiles(ctx context.Context, docFiles []docs.DocFile) error {
    for i := range docFiles {
        // ... existing transform pipeline code ...
        
        // NEW: Store transformed bytes for later use
        docFiles[i].TransformedBytes = p.Raw
        
        // Existing: Write to disk
        os.WriteFile(outputPath, p.Raw, 0644)
    }
}
```

**Change 3: Fix index generation to use transformed content**

```go
// internal/hugo/indexes.go
func (g *Generator) useReadmeAsIndex(file docs.DocFile, ...) error {
    // BEFORE: rawContent, _ := os.ReadFile(readmeSourcePath)
    // AFTER: Use already-transformed content
    if len(file.TransformedBytes) == 0 {
        return fmt.Errorf("README not yet transformed: %s", file.Path)
    }
    
    // No need to re-parse or manipulate - just copy transformed content
    indexPath := filepath.Join(g.buildRoot(), "content", ...)
    return os.WriteFile(indexPath, file.TransformedBytes, 0644)
}
```

### Architecture After Fix

```
┌──────────────┐
│  Discovery   │ Load source files once
│              │ DocFile.Content populated
└──────┬───────┘
       │
┌──────▼───────┐
│  Transform   │ Process through pipeline (in-memory)
│  Pipeline    │ DocFile.TransformedBytes populated ← NEW
└──────┬───────┘
       │
┌──────▼───────┐
│Content Write │ Write transformed content to disk
│              │ (copyContentFiles)
└──────┬───────┘
       │
┌──────▼───────┐
│Index Stage   │ Use DocFile.TransformedBytes ← FIXED
│              │ (no re-reading source)
└──────┬───────┘
       │
┌──────▼───────┐
│ Hugo Render  │ Build final static site
└──────────────┘
```

**Key principle**: Transform pipeline remains authoritative. Index stage becomes a pure consumer of transformed content.

## Consequences

### Positive

1. **Pipeline Integrity**: All content flows through transform pipeline with no bypasses
2. **Bug Fix**: README.md → _index.md conversion preserves link rewrites and other transforms
3. **Eliminates Duplicate Logic**: Front matter parsing happens only in transform pipeline
4. **Minimal Changes**: ~15 lines of code vs. full refactor
5. **Low Risk**: Doesn't change core architecture, just fixes data flow
6. **Better Testability**: Can verify transformed content is used consistently
7. **Future-Proof**: Makes it easier to add new transforms knowing they'll apply everywhere

### Negative

1. **Minimal Memory Overhead**: Adds `TransformedBytes` field to `DocFile`
   - Mitigation: Negligible impact (content already in memory during transform)
   - Only populated for markdown files, not assets
2. **Pass-by-value consideration**: `docFiles` slice must be passed by reference or returned
   - Current code already uses `[]docs.DocFile` slice which shares backing array
   - May need to ensure mutations are visible across function boundaries

### Trade-offs Avoided

By **not** doing a full refactor, we avoid:
- ❌ Rewriting working transform pipeline
- ❌ Changing stage interfaces
- ❌ Updating all transform implementations
- ❌ Extensive test updates
- ❌ Risk of introducing new bugs in working code

## Implementation Plan

### Phase 1: Foundation (Day 1-2)

**Files Modified**: 1 file, 2 lines
- [ ] Add `TransformedBytes []byte` field to `DocFile` struct in `internal/docs/discovery.go`
- [ ] Add godoc comment explaining field purpose
- [ ] Run tests to ensure no breakage from schema change

**Acceptance**: Field compiles, tests pass

---

### Phase 2: Capture Transformed Content (Day 2-3)

**Files Modified**: 1 file, ~3 lines
- [ ] In `internal/hugo/content_copy.go`, after transform pipeline completes:
  ```go
  // After: shim.SerializeFn()
  docFiles[i].TransformedBytes = p.Raw
  ```
- [ ] Ensure this happens inside the loop that processes each file
- [ ] Add debug logging to verify field is populated

**Acceptance**: `TransformedBytes` populated for markdown files after pipeline

**Testing**:
- [ ] Add test to verify `TransformedBytes` matches `p.Raw`
- [ ] Verify assets skip this (only markdown files)

---

### Phase 3: Fix Index Generation (Day 3-5)

**Files Modified**: 1 file, ~10-15 lines

**3a. Modify `useReadmeAsIndex` function**:
- [ ] Replace `os.ReadFile(readmeSourcePath)` with `file.TransformedBytes`
- [ ] Remove manual front matter parsing (already in transformed content)
- [ ] Add validation that `TransformedBytes` is populated
- [ ] Simplify logic - just copy transformed bytes to index location

**3b. Update calling code**:
- [ ] Ensure `useReadmeAsIndex` receives `DocFile` with `TransformedBytes`
- [ ] Pass full `DocFile` instead of just paths where needed

**Acceptance**: README.md promoted to _index.md preserves transforms

**Testing**:
- [ ] Test README.md with relative links becomes _index.md with rewritten links
- [ ] Test README.md with added front matter from pipeline is preserved
- [ ] Test multiple repositories with README files

---

### Phase 4: Integration Testing (Day 5-7)

**Files Modified**: Test files only

- [ ] Run existing `TestPipelineReadmeLinks` - should now pass
- [ ] Add test for front matter preservation in README → _index.md
- [ ] Test with multiple themes (Hextra, Docsy)
- [ ] Test edge cases:
  - README without front matter
  - README in subdirectories
  - Repositories without README files
- [ ] Run full integration test suite

**Acceptance**: All existing tests pass, README transforms preserved

---

### Phase 5: Documentation & Cleanup (Day 7-8)

- [ ] Update `CONTENT_TRANSFORMS.md` to document that transforms apply to all files including index promotions
- [ ] Add comments in `indexes.go` explaining why we use `TransformedBytes`
- [ ] Update this ADR status to "Accepted"
- [ ] Add CHANGELOG entry for bug fix

**Optional Cleanup** (can be separate PR):
- [ ] Remove now-unused manual front matter parsing in index stage
- [ ] Consolidate duplicate path resolution logic
- [ ] Add metrics for transform pipeline coverage

---

### Timeline Summary

- **Total Effort**: 1-2 weeks (with testing)
- **Code Changes**: ~20 lines across 2 files
- **Test Changes**: ~50-100 lines for comprehensive coverage
- **Risk Level**: Low (targeted fix, no architectural changes)

---

### Rollback Plan

If issues discovered:
1. **Immediate**: Revert `useReadmeAsIndex` to read from disk (restore 1 function)
2. **Short-term**: Add feature flag to toggle between old/new behavior
3. **Long-term**: Keep `TransformedBytes` field for future use, fix bugs incrementally

---

### Success Criteria

1. ✅ README.md files promoted to _index.md preserve all transforms
2. ✅ Links in README → _index.md are correctly rewritten
3. ✅ Front matter patches from pipeline are present in index files
4. ✅ No regression in existing functionality
5. ✅ All tests pass
6. ✅ No performance degradation

## References

- [Transform pipeline implementation](../../internal/hugo/content_copy.go)
- [Index generation](../../internal/hugo/indexes.go)
- [DocFile struct](../../internal/docs/discovery.go)
- [Transform pipeline design](../../CONTENT_TRANSFORMS.md)
- [Page struct with in-memory processing](../../internal/hugo/transform.go)
- [BuildState architecture](../../internal/hugo/build_state.go)

## Related Issues

- README.md link rewriting bypass when promoted to _index.md
- Front matter patches not applied to index files
- Duplicate front matter parsing logic in index stage

## Notes

### Discovery Process

This ADR was created after investigating why README.md files lost transform pipeline changes when promoted to `_index.md`. Initial analysis suggested the entire pipeline needed refactoring, but deeper investigation revealed:

1. **The transform pipeline already works correctly** - it processes content in-memory with proper dependency ordering
2. **The bug is isolated** - only the index generation stage bypasses the pipeline
3. **The fix is minimal** - capture and reuse transformed content instead of re-reading sources

### Key Learnings

- **Don't assume the architecture is broken** - investigate thoroughly before proposing large refactors
- **The codebase already implements best practices** - in-memory processing, dependency resolution, staged execution
- **Targeted fixes are often better** - 20 lines beats rewriting thousands

### Future Enhancements

This fix enables:
- Confidence that all transforms apply universally
- Easier debugging (single authoritative transformed content)
- Future optimization: avoid duplicate writes for README/index cases

---

**Created**: 2025-12-13  
**Updated**: 2025-12-13 (revised after codebase analysis)  
**Author**: Development Team  
**Decision**: Proposed → Implementation Ready
