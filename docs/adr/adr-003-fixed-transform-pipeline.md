---
title: "ADR-003: Fixed Transform Pipeline"
date: 2025-12-16
categories:
  - architecture-decisions
tags:
  - pipeline
  - transforms
  - architecture
  - simplification
weight: 4
---

# ADR-003: Fixed Transform Pipeline

Date: 2025-12-16

## Status

**Implemented** - December 16, 2025 (Now Default)

## Implementation Summary

The fixed transform pipeline is fully implemented and is now the **default and only** content processing system in DocBuilder.

**Deliverables:**
- - New pipeline package (`internal/hugo/pipeline/`)
- - Document type replacing Page/PageShim  
- - Generator and Transform function types
- - 3 generators (main/repo/section indexes)
- - 11 transforms (parse FM, normalize indexes, build FM, extract title, strip heading, rewrite links/images, keywords, metadata, edit link, serialize)
- - Comprehensive unit tests (71 passing in internal/hugo)
- - **Old system removed** - Transform registry, patch system, and all legacy code deleted

**Migration Complete (December 16, 2025):**
- Removed `internal/hugo/transforms/` directory (24 files, registry-based system)
- Removed `internal/hugo/fmcore/` directory (3 files, patch merge system)
- Removed visualize command and Page/PageShim abstractions
- Simplified integration code (content_copy.go: 216→13 lines)
- Net code reduction: -6,233 lines (-88%)

**Test Status:**
- Pipeline unit tests: PASS (6 test functions, 12 sub-tests)
- Hugo package tests: PASS (71 tests)
- Full short test suite: PASS (all packages)
- golangci-lint: 0 issues

## Context

### Current Architecture

DocBuilder's content transformation system uses a **registry-based, dependency-ordered pipeline** with a **front matter patching system**:

**Components:**
- `TransformRegistry`: Global registry where transforms register themselves via `init()`
- `Transform` interface: Requires `Name()`, `Priority()`, `DependsOn()`, and `Apply(page *Page) Patch`
- Dependency resolution: Topological sort based on `DependsOn()` declarations
- Patch system: Three merge modes (`MergeDeep`, `MergeReplace`, `MergeSetIfMissing`) with priority-based ordering
- Protected keys: Reserved front matter fields that block `MergeDeep` patches

**Current transforms:**
1. `front_matter_builder_v2` (priority 50): Initializes base front matter
2. `extract_index_title` (priority 55): Extracts H1 from README/index files  
3. `strip_heading`: Removes first H1 from content
4. `relative_link_rewriter`: Fixes relative markdown links
5. `image_link_rewriter`: Fixes image paths
6. Various metadata injectors (repo info, edit links, etc.)

**Example transform:**
```go
func init() {
    RegisterTransform(&ExtractIndexTitle{})
}

type ExtractIndexTitle struct{}

func (t *ExtractIndexTitle) Name() string { return "extract_index_title" }
func (t *ExtractIndexTitle) Priority() int { return 55 }
func (t *ExtractIndexTitle) DependsOn() []string { return []string{"front_matter_builder_v2"} }

func (t *ExtractIndexTitle) Apply(pg *Page) Patch {
    // Extract title from H1
    return Patch{
        Mode:     fmcore.MergeReplace,  // Required to override protected "title" key
        Priority: 55,
        FrontMatterUpdates: map[string]any{
            "title": extractedTitle,
        },
    }
}
```

### Problems with Current Architecture

1. **Hidden complexity**: Dependencies and execution order are not obvious from reading the code
2. **Non-local reasoning**: Understanding transform behavior requires checking:
   - Registration order in `init()`
   - Declared dependencies in `DependsOn()`
   - Priority values across multiple transforms
   - Protected key system in patching logic
   - Merge mode semantics (MergeDeep vs MergeReplace)

3. **Debugging difficulty**: 
   - Recent bug: `extract_index_title` extracted correct title but was silently blocked by protected keys
   - Required temporary debug logging to discover the issue
   - Solution was non-obvious: change `MergeDeep` to `MergeReplace`

4. **Indirection overhead**:
   - Registry pattern adds abstraction without benefit
   - Topological sort runs on every build
   - Patch merging adds cognitive overhead

5. **False flexibility**: 
   - Users **cannot** configure transforms dynamically
   - Registry/dependency system suggests extensibility we don't support
   - Added complexity without delivering value

6. **Maintenance burden**:
   - Adding transforms requires understanding registration, priorities, dependencies, and patch semantics
   - Easy to introduce subtle bugs (wrong merge mode, missing dependency, priority conflicts)

### Key Insight

**DocBuilder is greenfield and we control the pipeline.** We don't need dynamic transform registration or user-configurable pipelines. We need a **solid, predictable pipeline** for our specific use case.

## Decision

Replace the registry-based, patch-driven pipeline with a **fixed, explicit transform pipeline**.

### Core Principles

1. **Fixed execution order**: Transforms are called in explicit sequence defined in code
2. **Direct mutation**: Transforms modify `Document` directly (no patching)
3. **No dynamic registration**: No `init()` registry, no dependency declarations
4. **Simple interfaces**: Transform = function that modifies a document
5. **Transparent behavior**: Reading the pipeline code shows exact execution order

### New Architecture

**Core Interfaces:**
```go
// FileTransform modifies a document in the pipeline.
// Can optionally return new documents to inject into the pipeline.
// New documents will be queued and processed through ALL transforms from the beginning.
type FileTransform func(doc *Document) ([]*Document, error)

// FileGenerator creates new documents based on analysis of discovered documents.
// Generators run before transforms to create missing files (e.g., _index.md).
type FileGenerator func(ctx *GenerationContext) ([]*Document, error)

// GenerationContext provides access to discovered files for analysis.
type GenerationContext struct {
    Discovered []*Document  // All discovered files from repositories
    Config     *config.Config
}

// Document represents a file being processed through the pipeline.
type Document struct {
    // Content is the markdown body (transformed in-place)
    Content string
    
    // FrontMatter is the YAML front matter (modified directly)
    FrontMatter map[string]any
    
    // Metadata for transforms to use
    Path         string  // Hugo content path (e.g., "repo-name/section/file.md")
    IsIndex      bool    // True if this is _index.md or README.md
    Repository   string  // Source repository name
    SourceCommit string  // Git commit SHA
    SourceURL    string  // Repository URL for edit links
    Generated    bool    // True if this was generated (not discovered)
}
```

**Pipeline Execution:**
```go
// processContent runs the complete content processing pipeline.
func (g *Generator) processContent(discovered []*Document) ([]*Document, error) {
    // Phase 1: Generation - Create missing files
    generators := []FileGenerator{
        generateMainIndex,           // 1. Create site _index.md
        generateRepositoryIndexes,   // 2. Create repo _index.md files
        generateSectionIndexes,      // 3. Create section _index.md files
    }
    
    ctx := &GenerationContext{
        Discovered: discovered,
        Config:     g.config,
    }
    
    var generated []*Document
    for _, generator := range generators {
        docs, err := generator(ctx)
        if err != nil {
            return nil, fmt.Errorf("generation failed: %w", err)
        }
        generated = append(generated, docs...)
    }
    
    // Combine discovered + generated
    allDocs := append(discovered, generated...)
    
    // Phase 2: Transformation - Process all documents
    transforms := []FileTransform{
        computeBaseFrontMatter,      // 1. Initialize FrontMatter from file
        extractIndexTitle,           // 2. Extract H1 title from index files
        stripHeading,                // 3. Strip H1 if appropriate
        rewriteRelativeLinks,        // 4. Fix markdown links
        rewriteImageLinks,           // 5. Fix image paths
        generateFromKeywords,        // 6. Create new files based on keywords (e.g., @glossary)
        addRepositoryMetadata,       // 7. Add repo/commit/source metadata
        addEditLink,                 // 8. Generate edit URL
    }
    
    // Process documents iteratively - newly generated docs go through all transforms
    processedDocs := make([]*Document, 0, len(allDocs))
    queue := append([]*Document{}, allDocs...)
    
    for len(queue) > 0 {
        doc := queue[0]
        queue = queue[1:]
        
        // Run all transforms on this document
        for _, transform := range transforms {
            newDocs, err := transform(doc)
            if err != nil {
                return nil, fmt.Errorf("transform failed for %s: %w", doc.Path, err)
            }
            
            // Prevent generated documents from creating new documents
            if len(newDocs) > 0 && doc.Generated {
                return nil, fmt.Errorf(
                    "generated document %s attempted to create new documents (transforms should not generate from generated docs)",
                    doc.Path,
                )
            }
            
            // Queue new documents for full transform pipeline
            if len(newDocs) > 0 {
                queue = append(queue, newDocs...)
            }
        }
        
        processedDocs = append(processedDocs, doc)
    }
    
    return processedDocs, nil
}
```

**Example Generator (Creates New Files):**
```go
// generateSectionIndexes creates _index.md for sections that don't have one.
func generateSectionIndexes(ctx *GenerationContext) ([]*Document, error) {
    // Group discovered files by section
    sections := make(map[string][]*Document)
    for _, doc := range ctx.Discovered {
        section := filepath.Dir(doc.Path)
        sections[section] = append(sections[section], doc)
    }
    
    var generated []*Document
    for section, docs := range sections {
        // Check if index already exists
        hasIndex := false
        for _, doc := range docs {
            if doc.IsIndex {
                hasIndex = true
                break
            }
        }
        
        if !hasIndex {
            // Generate missing index
            indexDoc := &Document{
                Path:        filepath.Join(section, "_index.md"),
                IsIndex:     true,
                Generated:   true,
                Content:     generateIndexContent(section, docs),
                FrontMatter: map[string]any{
                    "title": titleCase(filepath.Base(section)),
                    "type":  "docs",
                },
            }
            generated = append(generated, indexDoc)
        }
    }
    
    return generated, nil
}
```

**Example Transform (Modifies Existing Files):**
```go
// extractIndexTitle extracts the first H1 heading as the title for index files.
// Only applies if no text exists before the H1.
func extractIndexTitle(doc *Document) ([]*Document, error) {
    if !doc.IsIndex {
        return nil, nil  // Only process index files, no new docs
    }
    
    h1Pattern := regexp.MustCompile(`(?m)^# (.+)$`)
    loc := h1Pattern.FindStringIndex(doc.Content)
    if loc == nil {
        return nil, nil  // No H1 found, no new docs
    }
    
    // Check for text before H1
    textBeforeH1 := strings.TrimSpace(doc.Content[:loc[0]])
    if textBeforeH1 != "" {
        return nil, nil  // Use filename as title, no new docs
    }
    
    // Extract title and set directly
    matches := h1Pattern.FindStringSubmatch(doc.Content)
    doc.FrontMatter["title"] = matches[1]
    
    return nil, nil  // Modified doc in-place, no new docs
}
```

**Example Transform (Generates New Files Based on Keywords):**
```go
// generateFromKeywords scans for special keywords and generates related files.
// Example: @glossary tag creates a glossary page from all terms.
// 
// If this transform returns new documents while processing a Generated document,
// the pipeline will return an error automatically - no need to check here.
func generateFromKeywords(doc *Document) ([]*Document, error) {
    var newDocs []*Document
    
    // Check for @glossary marker
    if strings.Contains(doc.Content, "@glossary") {
        // Extract all glossary terms from this document
        terms := extractGlossaryTerms(doc.Content)
        
        if len(terms) > 0 {
            // Generate glossary document
            // This will go through ALL transforms: front matter, link rewriting, etc.
            glossaryDoc := &Document{
                Path:        filepath.Join(doc.Repository, "glossary.md"),
                IsIndex:     false,
                Generated:   true,  // Mark as generated
                Content:     renderGlossary(terms),
                FrontMatter: map[string]any{
                    "title":      "Glossary",
                    "type":       "docs",
                    "generated":  true,
                    "source_doc": doc.Path,
                },
                Repository:   doc.Repository,
                SourceCommit: doc.SourceCommit,
                SourceURL:    doc.SourceURL,
            }
            
            newDocs = append(newDocs, glossaryDoc)
        }
        
        // Remove @glossary marker from original content
        doc.Content = strings.ReplaceAll(doc.Content, "@glossary", "")
    }
    
    // Check for other keywords...
    // if strings.Contains(doc.Content, "@api-reference") { ... }
    
    return newDocs, nil
}
```

### Migration Path

**Phase 1: Create New Pipeline (Parallel)**
1. Define `Document`, `FileTransform`, `FileGenerator`, `GenerationContext` types
2. Create `processContent()` with generation + transform phases
3. Convert existing index generation logic to generators
4. Convert existing transforms to new interface (one by one)
5. Add comprehensive tests for new pipeline

**Phase 2: Switch Over**
1. Update `copyContentFiles()` to use new pipeline
2. Run integration tests to verify behavior
3. Fix any discrepancies

**Phase 3: Cleanup**
1. Remove old `Transform` interface
2. Remove `TransformRegistry`
3. Remove topological sort logic
4. Remove patch system (`Patch`, `MergeMode`, protected keys)
5. Remove old transform files

**Phase 4: Documentation**
1. Update copilot instructions
2. Document transform pipeline in architecture docs
3. Add examples for adding new transforms

## Consequences

### Positive

- **Predictable**: Execution order is explicit in code  
- **Debuggable**: Set breakpoint in pipeline, step through transforms sequentially  
- **Testable**: Test individual transforms/generators or full pipeline easily  
- **Maintainable**: No magic, no hidden dependencies, no indirection  
- **Fast**: No registry lookups, no topological sorting, no patch merging  
- **Simple onboarding**: New developers see exact transform order immediately  
- **Reliable**: Fixed pipeline means consistent, reproducible behavior  
- **Separation of concerns**: Generation (creating files) separate from transformation (modifying files)  
- **Dynamic generation**: Transforms can create new files based on content analysis (keywords, patterns, etc.)  
- **Composable**: New documents flow through remaining transforms automatically  

### Negative

WARNING: **Less flexible**: Cannot dynamically add/remove transforms (but we don't need this)  
WARNING: **Migration effort**: Need to convert all existing transforms  

### Neutral

- Pipeline is now **explicitly ordered** instead of dependency-ordered
- Transforms **mutate directly** instead of returning patches
- **Code location** becomes important (pipeline defined in `generator.go`)

## Alternatives Considered

### 1. Keep Current System, Fix Bugs

**Description**: Continue using registry + patches, improve documentation

**Rejected because**:
- Doesn't address root cause (unnecessary complexity)
- Bug was symptom of overly complex system
- Future maintainers will face same issues

### 2. Plugin Architecture

**Description**: Make transforms truly pluggable with user configuration

**Rejected because**:
- Massive scope increase
- Users don't need this flexibility
- Introduces security/stability risks
- Not aligned with project goals

### 3. Middleware Pattern

**Description**: Chain of responsibility with explicit next() calls

**Rejected because**:
- More complex than simple function list
- Doesn't add value for our use case
- Makes testing harder (mocking next())

## Implementation Plan

- **Completed December 16, 2025**

**Phase 1: Core Pipeline (Completed)**
- Created `internal/hugo/pipeline/` package
- Implemented `Document` type with front matter and content fields
- Built `Processor` with two-phase execution (generators → transforms)
- Added queue-based processing for dynamic document generation

**Phase 2: Transforms Migration (Completed)**
- Converted all 10 essential transforms to `FileTransform` functions
- Implemented 3 generators for index file creation
- Removed dependency on registry, patches, and Page abstraction
- All transforms use direct mutation pattern

**Phase 3: Integration (Completed)**  
- Created `copyContentFilesPipeline()` integration function
- Added environment variable feature flag (`DOCBUILDER_NEW_PIPELINE=1`)
- Maintained backward compatibility with old system
- Updated copilot instructions

**Phase 4: Testing & Validation (Completed)**
- Unit tests for all generators and transforms
- Edge case coverage (empty FM, no FM, malformed FM)
- Integration via feature flag tested
- All tests passing, linter clean

**Remaining Work** (Separate from this ADR):
- Remove old registry/patch system
- Update golden test expectations (theme system issue)
- Make new pipeline the default
- Documentation updates

**Actual effort**: 1 day (vs estimated 3-5 days)

## Implementation Details

### File Structure

```
internal/hugo/pipeline/
├── document.go          # Document type, NewDocumentFromDocFile
├── processor.go         # Processor with ProcessContent
├── generators.go        # generateMainIndex, generateRepositoryIndex, generateSectionIndex
├── transforms.go        # All 10 transforms
└── pipeline_test.go     # Comprehensive unit tests
```

### Key Design Decisions

1. **Direct Mutation**: Documents are modified in-place, no patch merging
2. **Type Safety**: Compile-time verification of transform signatures
3. **Queue-Based**: Generators can add new documents during processing
4. **Stateless Transforms**: Pure functions with no global state
5. **Feature Flag**: Environment variable enables new pipeline without code changes

## Open Questions

All questions resolved during implementation:

1. **Error handling**: - Transforms return errors, pipeline fails fast
2. **Transform state**: - Pass context via RepositoryMetadata parameter
3. **Partial failures**: - Fail fast on first error (single-pass pipeline)
4. **Testing strategy**: - Both unit tests per transform and integration tests
5. **Front matter parsing**: - Handle edge cases (empty FM, no FM, malformed FM)
6. **Generator ordering**: - All generators run before any transforms

## References

- Issue: "README H1 duplicate headers" (revealed patch system complexity)
- ADR-002: In-Memory Content Pipeline (established single-pass architecture)
- Copilot Instructions: Transform pipeline section (needs update)
- Style Guide: Function naming conventions (already compatible)

## Decision Makers

- @inful (Lead Developer)

## Notes

This refactor aligns with DocBuilder's greenfield status and aggressive refactoring posture. We're optimizing for **clarity and maintainability** over theoretical flexibility we don't need.
