---
title: "Golden Test Implementation"
date: 2025-12-15
categories:
  - testing
tags:
  - golden-tests
  - implementation
  - testing-strategy
---

# Golden Test Implementation Summary

## Overview

This document tracks the implementation of a comprehensive golden test framework for DocBuilder's integration testing. The framework verifies end-to-end functionality including Git cloning, document discovery, Hugo generation, and HTML rendering.

## Implementation Status

### ✅ Phase 1: Foundation (Complete)
- **Infrastructure**: Test helpers, golden config loading, build execution
- **Files Created**:
  - `test/integration/helpers.go` (~609 lines) - Core test utilities
  - `test/integration/golden_test.go` (~1,107 lines) - Test implementations
  - `test/testdata/golden/` - Golden file storage
- **Key Functions**:
  - `loadGoldenConfig()` - Load and validate test configs
  - `runBuild()` - Execute build with BuildService
  - `verifyDirectoryStructure()` - Check output file tree
  - `verifyFileContent()` - Compare files against golden snapshots
  - `updateGoldenFiles()` - Regenerate golden snapshots

### ✅ Phase 2: Core Test Coverage (Complete - 13 tests)

#### Theme Tests
1. **TestGolden_HextraBasic** - Basic Hextra theme generation
2. **TestGolden_HextraMath** - Math rendering support
3. **TestGolden_DocsyBasic** - Basic Docsy theme generation
4. **TestGolden_HextraSearch** - Search index generation
5. **TestGolden_DocsyAPI** - Docsy API documentation style

#### Content Tests
6. **TestGolden_FrontmatterInjection** - Front matter generation
7. **TestGolden_ImagePaths** - Image path transformation
8. **TestGolden_SectionIndexes** - Section index page generation
9. **TestGolden_MenuGeneration** - Automatic menu generation
10. **TestGolden_HextraMultilang** - Multi-language support

#### Repository Tests
11. **TestGolden_TwoRepos** - Multi-repository aggregation
12. **TestGolden_ConflictingPaths** - Path conflict resolution
13. **TestGolden_CrossRepoLinks** - Cross-repository linking

### ✅ Phase 3: Edge Cases & Enhanced Verification (Complete - 9 tests)

#### Edge Case Tests
14. **TestGolden_EmptyDocs** - Repository with no markdown files
15. **TestGolden_OnlyReadme** - Only README.md (should be skipped)
16. **TestGolden_MalformedFrontmatter** - Invalid YAML front matter
17. **TestGolden_DeepNesting** - Deep directory nesting (10+ levels)
18. **TestGolden_UnicodeNames** - Unicode filenames and paths
19. **TestGolden_SpecialChars** - Special characters in filenames

#### HTML Verification
- **Framework**: DOM parsing with `golang.org/x/net/html`
- **Capabilities**:
  - CSS selector matching (tag, .class, #id)
  - Element counting
  - Text content verification
  - Attribute checking
- **Implementation**:
  - `verifyRenderedSamples()` - Parse and verify HTML structure
  - `findElements()` - Recursive DOM tree traversal
  - `parseSimpleSelector()` - CSS selector parsing
  - `renderHugoSite()` - Execute Hugo build
- **Golden Files**: `rendered-samples.golden.json` - HTML verification samples

#### Error Handling Tests
20. **TestGolden_Error_InvalidRepository** - Invalid repository URL handling
21. **TestGolden_Error_InvalidConfig** - Empty/minimal configuration
22. **TestGolden_Warning_NoGitCommit** - Repository without commits

## Test Results

```
Total Tests: 22
Status: All Passing ✅

Test Execution Time: ~0.6s
HTML Rendering Tests: 1 test with 3 subtests
Error Case Tests: 3 tests
```

## Golden File Structure

```
test/testdata/golden/
├── hextra-basic/
│   ├── config.yaml                    # Test configuration
│   ├── structure.golden.json          # Expected directory structure
│   ├── content.golden.json            # File content snapshots
│   └── rendered-samples.golden.json   # HTML verification samples
├── hextra-math/
├── frontmatter-injection/
├── two-repos/
├── docsy-basic/
├── hextra-search/
├── image-paths/
├── section-indexes/
├── conflicting-paths/
├── menu-generation/
├── docsy-api/
├── hextra-multilang/
├── cross-repo-links/
├── empty-docs/
├── only-readme/
├── malformed-frontmatter/
├── deep-nesting/
├── unicode-names/
└── special-chars/
```

## Key Technical Decisions

### 1. HTML Verification Approach
- **Choice**: Parse HTML DOM with `golang.org/x/net/html`
- **Rationale**: More reliable than string matching, verifies actual rendered structure
- **Trade-off**: Requires Hugo execution during tests (slower but more accurate)

### 2. Error Handling Philosophy
- **Choice**: Verify graceful degradation, not hard failures
- **Rationale**: Build service logs errors but continues when possible
- **Implementation**: Check `RepositoriesSkipped` counter rather than expecting errors

### 3. Golden File Format
- **Structure Files**: JSON array of file paths
- **Content Files**: JSON map of path→content
- **HTML Samples**: JSON array of selector-based verifications
- **Rationale**: JSON is structured, diffable, and language-agnostic

### 4. Test Organization
- **Pattern**: One test per scenario, subtests for verification steps
- **Naming**: `TestGolden_{Feature}` for discoverability
- **Isolation**: Each test uses `t.TempDir()` for complete isolation

## Usage

### Running Tests

```bash
# Run all golden tests
go test ./test/integration -run=TestGolden -v

# Run specific test
go test ./test/integration -run=TestGolden_HextraBasic -v

# Update golden files (when changes are expected)
go test ./test/integration -run=TestGolden_HextraBasic -update-golden
```

### Adding New Tests

1. Create test configuration in `test/testdata/configs/{name}.yaml`
2. Create golden directory in `test/testdata/golden/{name}/`
3. Add test function to `test/integration/golden_test.go`
4. Run with `-update-golden` to generate initial snapshots
5. Review and commit golden files

## Future Enhancements

### Potential Additions
- **Performance Benchmarks**: Track build time trends
- **Incremental Build Testing**: Verify cache behavior
- **Theme Variations**: Test theme-specific features
- **Plugin System Testing**: Verify transform plugins
- **Build Report Validation**: Parse and verify build-report.json

### CI Integration
- Run golden tests on every PR
- Flag golden file changes for review
- Generate test coverage reports
- Performance regression detection

## Related Documentation

- [Test Architecture](../explanation/renderer-testing.md)
- [CI/CD Setup](../ci-cd-setup.md)
- [Configuration Reference](../reference/configuration.md)
- [How to Add Theme Support](../how-to/add-theme-support.md)

## Maintenance Notes

- Golden files should be reviewed when updated (not blindly regenerated)
- HTML verification samples should check structure, not exact content
- Error tests should verify logging and graceful handling, not hard failures
- Test execution should remain fast (< 1s for full suite without Hugo)
