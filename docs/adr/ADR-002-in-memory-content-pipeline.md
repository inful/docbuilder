# ADR-001: In-Memory Content Pipeline Architecture

## Status

Proposed

## Context

The current Hugo content generation pipeline has a design issue where content is processed through multiple stages that involve unnecessary disk I/O operations:

1. **Discovery Stage**: Reads files from source repositories into memory (`DocFile.Content`)
2. **Transform Stage** (`copyContentFiles`): 
   - Loads content into memory
   - Runs transform pipeline (front matter parsing, link rewriting, etc.)
   - **Writes transformed content to disk** in the Hugo staging directory
3. **Index Stage** (`stageIndexes`):
   - **Re-reads files from disk** (either source or staging directory)
   - Manually processes front matter (bypassing the transform pipeline)
   - Writes final index files

### Problems with Current Architecture

1. **Duplicate Disk I/O**: Content is written to disk after transformation, then immediately re-read for index routing
2. **Pipeline Bypass**: The index stage re-reads source files and manually manipulates content, bypassing transforms
3. **Bug Potential**: The bypass caused the README link rewriting bug (fixed in commit fixing README.md → _index.md conversion)
4. **Duplicate Logic**: Front matter handling exists in both the transform pipeline and index stage
5. **Unclear Data Flow**: Content flows through pipeline → disk → re-read → process, making it hard to reason about
6. **Testing Complexity**: Must verify both pipeline transforms AND index stage manual processing

### Specific Example of the Problem

When README.md becomes a section index (_index.md):

```go
// Transform stage: README.md processed through pipeline
file.LoadContent()                    // Disk read #1
runTransformPipeline(file)           // Links rewritten, front matter added
writeFile(hugPath, transformed)      // Disk write #1

// Index stage: Re-processes the same content
rawContent := readFile(source)       // Disk read #2 (bypasses pipeline!)
// Manually manipulate front matter (duplicate logic)
writeFile(indexPath, reprocessed)    // Disk write #2 (overwrites transformed content!)
```

This caused transformed links to be lost when the index stage overwrote the pipeline output.

## Decision

Refactor the content pipeline to maintain transformed content **in memory** until final write, eliminating intermediate disk operations and pipeline bypasses.

### New Architecture

```
┌──────────────┐
│  Discovery   │ Load source files once
└──────┬───────┘
       │ (SourceContent in memory)
       │
┌──────▼───────┐
│  Transform   │ Process through pipeline
│  Pipeline    │ (front matter, links, etc.)
└──────┬───────┘
       │ (TransformedContent in memory)
       │
┌──────▼───────┐
│   Routing    │ Decide output paths
│   Stage      │ (index promotion, etc.)
└──────┬───────┘
       │ (OutputPath determined)
       │
┌──────▼───────┐
│   Write      │ Single write to staging
│   Stage      │ (from TransformedContent)
└──────────────┘
```

### Data Model Changes

Enhance `DocFile` to carry both source and transformed content:

```go
type DocFile struct {
    // Existing fields
    Path         string
    RelativePath string
    Repository   string
    Section      string
    Name         string
    Extension    string
    
    // Content evolution
    SourceContent      []byte   // Original from disk (immutable)
    TransformedContent []byte   // After pipeline (mutable through pipeline)
    
    // Computed/cached fields
    HugoPath           string   // Cached from GetHugoPath()
    OutputPath         string   // Final output path (after routing)
    IsIndexCandidate   bool     // Flagged during discovery
    
    // Existing fields
    Metadata    map[string]string
    IsAsset     bool
}
```

### Pipeline Stages

**Stage 1: Discovery & Load** (already exists, minimal change)
```go
func (d *Discovery) DiscoverDocs(...) ([]DocFile, error) {
    // ... existing discovery ...
    file.SourceContent = readFile(path)  // Single read
    file.IsIndexCandidate = (file.Name == "index" || file.Name == "README")
    return files
}
```

**Stage 2: Transform** (refactored to not write)
```go
func (g *Generator) transformContent(files []DocFile) error {
    for i := range files {
        if files[i].IsAsset {
            files[i].TransformedContent = files[i].SourceContent
            continue
        }
        
        // Run transform pipeline (existing logic)
        transformed := g.runPipeline(&files[i])
        files[i].TransformedContent = transformed  // Store in memory
        // NO DISK WRITE HERE
    }
    return nil
}
```

**Stage 3: Routing** (new, replaces index stage logic)
```go
func (g *Generator) routeContent(files []DocFile) error {
    for i := range files {
        // Default output path
        files[i].OutputPath = files[i].GetHugoPath()
        
        // Index routing logic (uses in-memory transformed content)
        if files[i].IsIndexCandidate && shouldBeIndex(&files[i], files) {
            files[i].OutputPath = getIndexPath(&files[i])
        }
    }
    return nil
}
```

**Stage 4: Write** (new, single write pass)
```go
func (g *Generator) writeContent(files []DocFile) error {
    for _, file := range files {
        outputPath := filepath.Join(g.buildRoot(), file.OutputPath)
        os.MkdirAll(filepath.Dir(outputPath), 0750)
        os.WriteFile(outputPath, file.TransformedContent, 0644)
    }
    return nil
}
```

### Pipeline Execution Order

```go
func (g *Generator) GenerateSite(files []DocFile) error {
    // ... config, layout stages ...
    
    if err := g.transformContent(files); err != nil {
        return err
    }
    
    if err := g.routeContent(files); err != nil {
        return err
    }
    
    if err := g.writeContent(files); err != nil {
        return err
    }
    
    // ... Hugo render stage ...
}
```

## Consequences

### Positive

1. **Single Disk Read**: Each source file read exactly once during discovery
2. **Single Disk Write**: Each output file written exactly once after all processing
3. **No Pipeline Bypass**: All content flows through the transform pipeline, no exceptions
4. **Clearer Data Flow**: Linear flow from source → transform → route → write
5. **Better Testability**: Can inspect `TransformedContent` at any stage
6. **Elimination of Duplicate Logic**: Front matter handling only in transform pipeline
7. **Memory-Efficient**: Content already loaded for transforms, no additional memory cost
8. **Bug Prevention**: Impossible to overwrite transformed content by re-reading source

### Negative

1. **Memory Usage**: Must hold both source and transformed content in memory
   - Mitigation: Most doc sites have <1000 files, <10MB total
   - Already loading content for transforms anyway
2. **Refactoring Effort**: Requires changes to multiple stages
   - Affected files: `content_copy.go`, `indexes.go`, stage pipeline
   - Existing tests need updates
3. **Breaking Changes**: Internal API changes to stage interfaces
   - Mitigation: Internal implementation only, no user-facing changes

### Risk Mitigation

1. **Gradual Migration**: Implement new stages alongside old, feature-flag the switch
2. **Comprehensive Testing**: Validate with existing test suite before removing old code
3. **Memory Monitoring**: Add metrics to track memory usage in large doc sets
4. **Rollback Plan**: Keep old code path available via feature flag initially

## Implementation Plan

### Phase 1: Data Model (Week 1)
- [ ] Add `TransformedContent` and `OutputPath` fields to `DocFile`
- [ ] Add `IsIndexCandidate` flag computed during discovery
- [ ] Ensure backward compatibility (empty fields for now)

### Phase 2: Transform Stage (Week 2)
- [ ] Refactor `copyContentFiles` to store transformed content in memory
- [ ] Add feature flag `DOCBUILDER_IN_MEMORY_PIPELINE=true`
- [ ] Write to disk only if flag disabled (old behavior)
- [ ] Update tests to verify in-memory content

### Phase 3: Routing Stage (Week 3)
- [ ] Create new `routeContent` stage
- [ ] Migrate index routing logic from `stageIndexes`
- [ ] Use `TransformedContent` instead of re-reading
- [ ] Update tests for routing logic

### Phase 4: Write Stage (Week 4)
- [ ] Create new `writeContent` stage
- [ ] Write from `TransformedContent` to `OutputPath`
- [ ] Validate output matches old behavior
- [ ] Performance testing

### Phase 5: Cleanup (Week 5)
- [ ] Remove old `useReadmeAsIndex` and manual processing code
- [ ] Remove feature flag, make new pipeline default
- [ ] Update documentation
- [ ] Archive this ADR as Accepted

## References

- [Current content_copy.go implementation](../../internal/hugo/content_copy.go)
- [Current indexes.go implementation](../../internal/hugo/indexes.go)
- [Transform pipeline design](../../CONTENT_TRANSFORMS.md)
- Related bug fix: README.md link rewriting bypass issue

## Notes

This ADR was created after discovering that the index stage was bypassing the transform pipeline, causing transformed content (specifically link rewrites) to be lost when README.md files were promoted to _index.md. The proposed architecture eliminates the bypass entirely by making pipeline processing mandatory for all content.

---

**Created**: 2025-12-12  
**Author**: Development Team  
**Decision**: Pending Implementation
