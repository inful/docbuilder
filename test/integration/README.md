# Integration Golden Tests

This directory contains integration tests that verify the complete DocBuilder pipeline using golden files.

## Overview

Golden tests work by:
1. Creating temporary Git repositories from `testdata/repos/`
2. Running the full build pipeline (git clone → docs discovery → Hugo generation)
3. Comparing output against verified "golden" files

## Directory Structure

```
test/
├── integration/          # Test code
│   ├── golden_test.go   # Golden test cases
│   └── helpers.go       # Test helper functions
└── testdata/
    ├── repos/           # Source repository structures
    │   └── themes/
    │       └── relearn-basic/  # Test repository
    ├── configs/         # Test configurations
    │   └── relearn-basic.yaml
    └── golden/          # Verified output snapshots
        └── relearn-basic/
            ├── hugo-config.golden.yaml        # Hugo configuration
            └── content-structure.golden.json  # Content files + front matter
```

## Running Tests

```bash
# Run all golden tests
go test ./test/integration

# Run specific test
go test ./test/integration -run TestGolden_RelearnBasic

# Skip in short mode (for quick test runs)
go test -short ./...

# Verbose output
go test ./test/integration -v
```

## Updating Golden Files

When you intentionally change the output (e.g., adding new features or fixing bugs):

```bash
# Update all golden files
go test ./test/integration -update-golden

# Update specific test
go test ./test/integration -run TestGolden_RelearnBasic -update-golden
```

**Important**: Always review the diff before committing updated golden files!

```bash
git diff test/testdata/golden/
```

## Creating New Golden Tests

### 1. Create Test Repository

Add a new directory under `test/testdata/repos/` with sample documentation:

```bash
mkdir -p test/testdata/repos/themes/my-feature
mkdir -p test/testdata/repos/themes/my-feature/docs
```

Add markdown files with appropriate front matter and content.

### 2. Create Test Configuration

Add a YAML config in `test/testdata/configs/`:

```yaml
version: "2.0"

repositories:
  - name: my-feature
    url: PLACEHOLDER  # Will be replaced in test
    branch: main
    paths: [docs]

hugo:
  title: My Feature Test
  theme: relearn
  # ... other config

output:
  directory: PLACEHOLDER
  clean: true
```

### 3. Write Test Case

Add a test function in `golden_test.go`:

```go
func TestGolden_MyFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping golden test in short mode")
    }

    repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/my-feature")
    cfg := loadGoldenConfig(t, "../../test/testdata/configs/my-feature.yaml")
    
    cfg.Repositories[0].URL = repoPath
    outputDir := t.TempDir()
    cfg.Output.Directory = outputDir

    svc := build.NewBuildService().
        WithHugoGeneratorFactory(func(cfgAny any, outDir string) build.HugoGenerator {
            return hugo.NewGenerator(cfgAny.(*config.Config), outDir)
        })

    req := build.BuildRequest{
        Config:    cfg,
        OutputDir: outputDir,
    }

    result, err := svc.Run(context.Background(), req)
    require.NoError(t, err)
    require.Equal(t, build.BuildStatusSuccess, result.Status)

    goldenDir := "../../test/testdata/golden/my-feature"
    verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)
    verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}
```

### 4. Generate Golden Files

```bash
go test ./test/integration -run TestGolden_MyFeature -update-golden
```

### 5. Verify Test Passes

```bash
go test ./test/integration -run TestGolden_MyFeature
```

## What Gets Verified

### Hugo Configuration (`hugo-config.golden.yaml`)

Complete generated Hugo configuration including:
- Basic settings (title, baseURL, theme)
- Theme-specific parameters
- Module imports (for themes using Hugo modules)
- Menu configuration
- Markup settings

**Note**: Dynamic fields like `build_date` are automatically normalized.

### Content Structure (`content-structure.golden.json`)

For each markdown file:
- **Front matter**: All metadata fields (title, description, weight, editURL, etc.)
- **Content hash**: SHA-256 hash of the markdown body (excluding front matter)
- **Directory structure**: Nested map of folders and files

**Note**: Date fields in front matter (date, lastmod, publishDate, expiryDate) are normalized.

## Best Practices

### When to Add Golden Tests

- **New themes**: Add a basic test for each supported theme
- **Transformations**: Test content transformations (links, images, front matter injection)
- **Bug fixes**: Create minimal reproducible test case
- **Features**: Demonstrate new features work end-to-end

### When to Update Golden Files

- Intentional output changes (new features, fixes)
- Theme system refactoring
- Configuration schema changes

### When NOT to Update

- Build failures (fix the code first!)
- Flaky tests (normalize dynamic fields)
- Unrelated changes

## Troubleshooting

### Test Fails After Code Changes

1. Check if the change was intentional:
   ```bash
   git diff test/testdata/golden/
   ```

2. If intentional, update golden files:
   ```bash
   go test ./test/integration -update-golden
   ```

3. Review and commit the changes

### Test Fails Randomly

Check for non-deterministic output:
- Timestamps in front matter (should be normalized)
- Random UUIDs or hashes
- Order-dependent operations

Add normalization in `helpers.go` if needed.

### Path Errors

Ensure paths are relative to the test file location:
```go
setupTestRepo(t, "../../test/testdata/repos/themes/relearn-basic")
```

## CI Integration

Golden tests run automatically in CI on every PR. The workflow:

1. Runs all tests
2. Fails if golden files are out of date
3. Requires golden file updates to be in the same commit as code changes

## Related Documentation

- [ADR-001: Golden Testing Strategy](../../docs/adr/ADR-001-golden-testing-strategy.md)
- [Implementation Plan](../../docs/plan/golden-testing-implementation.md)
- [Renderer Testing Explanation](../../docs/explanation/renderer-testing.md)
