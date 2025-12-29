---
title: "Golden Test Implementation"
date: 2025-12-29
categories:
  - testing
tags:
  - golden-tests
  - implementation
  - testing-strategy
---

# Golden Test Implementation Summary

## Overview

This document describes the comprehensive golden test framework for DocBuilder's integration testing. The framework verifies end-to-end functionality including Git cloning, document discovery, Hugo generation, and configuration output.

## Implementation Status

### ✅ Foundation (Complete)
- **Infrastructure**: Test helpers, golden config loading, build execution
- **Files**:
  - `test/integration/helpers.go` (437 lines) - Core test utilities
  - `test/integration/golden_test.go` (813 lines) - Test implementations
  - `test/testdata/golden/` (12 directories) - Golden file storage
  - `test/testdata/configs/` (14 files) - Test configurations
- **Key Functions**:
  - `loadGoldenConfig()` - Load and validate test configs
  - `setupTestRepo()` - Create temporary git repositories for testing
  - `verifyHugoConfig()` - Compare Hugo YAML configuration
  - `verifyContentStructure()` - Check content files and front matter
  - `parseFrontMatter()` - Extract and validate YAML front matter

### ✅ Core Test Coverage (Complete - 16 tests)

#### Content Transformation Tests
1. **TestGolden_FrontmatterInjection** - Front matter generation with editURL
2. **TestGolden_ImagePaths** - Image path transformation
3. **TestGolden_SectionIndexes** - Section index page generation
4. **TestGolden_MenuGeneration** - Automatic menu generation

#### Repository Tests
5. **TestGolden_TwoRepos** - Multi-repository aggregation
6. **TestGolden_ConflictingPaths** - Path conflict resolution
7. **TestGolden_CrossRepoLinks** - Cross-repository linking

#### Edge Case Tests
8. **TestGolden_EmptyDocs** - Repository with no markdown files
9. **TestGolden_OnlyReadme** - Only README.md (should be skipped)
10. **TestGolden_MalformedFrontmatter** - Invalid YAML front matter
11. **TestGolden_DeepNesting** - Deep directory nesting (10+ levels)
12. **TestGolden_UnicodeNames** - Unicode filenames and paths
13. **TestGolden_SpecialChars** - Special characters in filenames

#### Error Handling Tests
14. **TestGolden_Error_InvalidRepository** - Invalid repository URL handling
15. **TestGolden_Error_InvalidConfig** - Empty/minimal configuration
16. **TestGolden_Warning_NoGitCommit** - Repository without commits

> **Note**: All tests use the Relearn theme. Error tests use `relearn-basic.yaml` as a base configuration.

## Test Results

```
Total Tests: 16
Status: All Passing ✅

Test Execution Time: ~1.1s (without Hugo rendering)
Error Case Tests: 3 tests with graceful degradation verification
Theme Support: Relearn only
```

## Golden File Structure

Each golden directory contains verification artifacts:

```
test/testdata/golden/<test-name>/
├── hugo-config.golden.yaml           # Expected Hugo configuration
├── content-structure.golden.json     # Content files and front matter
└── rendered-samples.golden.json      # HTML verification samples (optional)
```

### Available Golden Directories

All golden directories are actively used for Relearn theme testing:

- `conflicting-paths/` - Path conflict scenarios
- `cross-repo-links/` - Inter-repository linking
- `deep-nesting/` - Deep directory hierarchies
- `empty-docs/` - Empty repository handling
- `frontmatter-injection/` - Front matter generation
- `image-paths/` - Image path transformation
- `malformed-frontmatter/` - Invalid YAML handling
- `menu-generation/` - Automatic menu creation
- `only-readme/` - README-only repositories
- `section-indexes/` - Section index pages
- `special-chars/` - Special character filenames
- `two-repos/` - Multi-repository builds
- `unicode-names/` - Unicode filename support

## Key Technical Decisions

### 1. Build Service Integration
- **Choice**: Use `build.BuildService` for test execution
- **Rationale**: Tests verify the actual production pipeline, not mocked components
- **Implementation**: Each test creates a BuildService with real HugoGenerator

### 2. Error Handling Philosophy
- **Choice**: Verify graceful degradation, not hard failures
- **Rationale**: Build service logs errors but continues when possible
- **Implementation**: Check `RepositoriesSkipped` counter rather than expecting errors

### 3. Golden File Format
- **Hugo Config**: YAML files with normalized timestamps
- **Content Structure**: JSON with file paths, front matter, and content hashes
- **Rationale**: Structured formats allow precise, diffable comparisons

### 4. Test Organization
- **Pattern**: One test per scenario with descriptive names
- **Naming**: `TestGolden_{Feature}` for discoverability
- **Isolation**: Each test uses `t.TempDir()` for complete isolation
- **No HTML Rendering**: Tests verify Hugo config/content, not rendered HTML (too slow)

## Usage

### Running Tests

```bash
# Run all golden tests
go test ./test/integration -run=TestGolden -v

# Run specific test
go test ./test/integration -run=TestGolden_FrontmatterInjection -v

# Update golden files (when changes are expected)
go test ./test/integration -run=TestGolden_FrontmatterInjection -update-golden

# Run in short mode (skips golden tests)
go test ./test/integration -short
```

### Adding New Tests

1. **Create test repository structure** in `test/testdata/repos/<category>/<test-name>/`
   - Add markdown files, images, and other assets
   - Structure should reflect a real documentation repository

2. **Create test configuration** in `test/testdata/configs/<test-name>.yaml`
   - Define repository URLs (will be replaced by `setupTestRepo`)
   - Configure Hugo with Relearn theme and parameters
   - Set output directory (will be replaced by `t.TempDir()`)

3. **Add test function** to `test/integration/golden_test.go`
   ```go
   func TestGolden_YourFeature(t *testing.T) {
       if testing.Short() {
           t.Skip("Skipping golden test in short mode")
       }
       
       // Setup test repo
       repoPath := setupTestRepo(t, "../../test/testdata/repos/<path>")
       
       // Load and configure
       cfg := loadGoldenConfig(t, "../../test/testdata/configs/your-feature.yaml")
       cfg.Repositories[0].URL = repoPath
       outputDir := t.TempDir()
       cfg.Output.Directory = outputDir
       
       // Run build
       svc := build.NewBuildService().
           WithHugoGeneratorFactory(func(cfgAny any, outDir string) build.HugoGenerator {
               return hugo.NewGenerator(cfgAny.(*config.Config), outDir)
           })
       result, err := svc.Run(context.Background(), build.BuildRequest{
           Config:    cfg,
           OutputDir: outputDir,
       })
       require.NoError(t, err)
       require.Equal(t, build.BuildStatusSuccess, result.Status)
       
       // Verify outputs
       goldenDir := "../../test/testdata/golden/your-feature"
       verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)
       verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
   }
   ```

4. **Generate golden files** with `-update-golden` flag
   ```bash
   go test ./test/integration -run=TestGolden_YourFeature -update-golden
   ```

5. **Review golden files** - Don't blindly accept generated output
   - Check `hugo-config.golden.yaml` for correct configuration
   - Verify `content-structure.golden.json` has expected files and front matter
   - Commit golden files with your test

6. **Verify test passes** without `-update-golden`
   ```bash
   go test ./test/integration -run=TestGolden_YourFeature -v
   ```

## Future Enhancements

### Potential Additions
- **Relearn Theme Features**: Tests for Relearn-specific features (tabs, attachments, shortcodes)
- **Performance Benchmarks**: Track build time trends with `testing.B`
- **Incremental Build Testing**: Verify cache behavior and rebuild optimization
- **Plugin System Testing**: Verify transform plugins work correctly
- **Build Report Validation**: Parse and verify `build-report.json` accuracy
- **HTML Rendering Tests**: Add optional Hugo build + HTML verification (currently skipped for speed)

### CI Integration
- Golden tests run on every PR
- Flag golden file changes for manual review
- Generate test coverage reports
- Detect regressions in test execution time

## Helper Functions Reference

### Test Repository Setup
- **`setupTestRepo(t, path)`** - Creates temporary git repository from testdata
  - Copies files from source path
  - Initializes git repository
  - Creates initial commit
  - Returns temporary directory path

### Configuration Loading
- **`loadGoldenConfig(t, path)`** - Loads YAML test configuration
  - Validates required fields
  - Returns `*config.Config`

### Verification Functions
- **`verifyHugoConfig(t, outputDir, goldenPath, update)`** - Compares Hugo YAML
  - Normalizes dynamic fields (build_date, timestamps)
  - Updates golden file if `update=true`
  - Asserts YAML equality

- **`verifyContentStructure(t, outputDir, goldenPath, update)`** - Verifies content files
  - Walks content directory
  - Extracts front matter from each file
  - Computes content hashes
  - Compares structure and metadata

- **`parseFrontMatter(data)`** - Extracts YAML front matter
  - Returns front matter map and remaining content
  - Handles files without front matter

### Build Execution
Tests use `build.BuildService` directly:
```go
svc := build.NewBuildService().
    WithHugoGeneratorFactory(func(cfgAny any, outDir string) build.HugoGenerator {
        return hugo.NewGenerator(cfgAny.(*config.Config), outDir)
    })

result, err := svc.Run(context.Background(), build.BuildRequest{
    Config:    cfg,
    OutputDir: outputDir,
})
```

## Related Documentation

- [Test Architecture](../explanation/renderer-testing.md)
- [CI/CD Setup](../ci-cd-setup.md)
- [Configuration Reference](../reference/configuration.md)
- [Style Guide](../style_guide.md)

## Maintenance Notes

### Golden File Review
- **Never blindly regenerate** golden files - always review changes
- Golden files represent expected behavior, not actual output
- Changes to golden files should be justified in PR descriptions
- Hugo config changes should be theme-compatible

### Test Design Guidelines
- Keep tests fast (target < 2s for full suite)
- Don't run Hugo build in tests (too slow, config is sufficient)
- Use `t.TempDir()` for complete test isolation
- Test one scenario per test function
- Error tests should verify graceful degradation, not hard failures

### File Organization
```
test/
├── integration/
│   ├── golden_test.go          # Test implementations (813 lines)
│   ├── helpers.go              # Test utilities (437 lines)
│   └── README.md               # Integration test overview
├── testdata/
│   ├── configs/                # YAML test configurations (14 files)
│   ├── golden/                 # Expected outputs (12 directories)
│   └── repos/                  # Test repository structures
│       ├── multi-repo/         # Multi-repository scenarios
│       └── transforms/         # Content transformation tests
```

### Common Pitfalls
1. **Dynamic timestamps**: Use `verifyHugoConfig` which normalizes `build_date`
2. **Absolute paths**: Golden files should not contain temp directory paths
3. **Git state**: Use `setupTestRepo` to ensure consistent git history
4. **Configuration placeholders**: Test configs use `PLACEHOLDER` for dynamic values
