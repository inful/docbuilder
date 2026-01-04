# Contributing to DocBuilder

Thank you for your interest in contributing to DocBuilder! This guide will help you understand how to extend the system with new stages, transforms, and themes.

## Development Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/your-org/docbuilder.git
   cd docbuilder
   ```

2. **Install dependencies**:
   ```bash
   go mod download
   ```

3. **Build and test**:
   ```bash
   make build
   make test
   ```

4. **Run with example config**:
   ```bash
   ./bin/docbuilder init -c example.yaml
   ./bin/docbuilder build -c example.yaml -v
   ```

## Architecture Overview

DocBuilder uses a pipeline-based architecture with three main extension points:

- **Pipeline Stages**: Sequential build steps (clone, discover, Hugo site generation, etc.)
- **Content Transforms**: Processing steps for markdown files
- **Hugo Themes**: Theme-specific configuration and capabilities

## Adding a New Pipeline Stage

Pipeline stages handle major build phases. Each stage receives a `BuildState` and can modify it.

### 1. Define the Stage

Add your stage constant to `internal/hugo/stage.go`:

```go
const (
    // ... existing stages
    StageMyNewStep StageName = "my_new_step"
)
```

### 2. Implement the Stage Function

Create your stage function in `internal/hugo/stage_my_new_step.go`:

```go
package hugo

import (
    "context"
    "fmt"
)

func stageMyNewStep(ctx context.Context, bs *BuildState) error {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return newCanceledStageError(StageMyNewStep, ctx.Err())
    default:
    }
    
    // Perform your stage logic
    if err := doMyWork(bs); err != nil {
        return newFatalStageError(StageMyNewStep, err)
    }
    
    // Update build state
    bs.Report.MyMetric++
    
    return nil
}

func doMyWork(bs *BuildState) error {
    // Your implementation here
    return nil
}
```

### 3. Add to Pipeline

Insert your stage in the pipeline in `internal/hugo/generator.go`:

```go
stages := NewPipeline().
    Add(StagePrepareOutput, stagePrepareOutput).
    Add(StageCloneRepos, stageCloneRepos).
    Add(StageDiscoverDocs, stageDiscoverDocs).
    Add(StageMyNewStep, stageMyNewStep).  // Add your stage
    Add(StageGenerateConfig, stageGenerateConfig).
    // ... rest of stages
```

### 4. Add Error Classification

If your stage can produce specific errors, add them to the appropriate error package:

```go
// In internal/docs/errors/errors.go or internal/hugo/errors/errors.go
var (
    ErrMyNewStepFailed = errors.New("my new step failed")
)
```

### 5. Update Build Report

Add any metrics or counters to `internal/hugo/report.go`:

```go
type BuildReport struct {
    // ... existing fields
    MyMetric int `json:"my_metric"`
}
```

### 6. Write Tests

Create tests for your stage in `internal/hugo/stage_my_new_step_test.go`:

```go
func TestStageMyNewStep(t *testing.T) {
    ctx := context.Background()
    gen := createTestGenerator()
    bs := newBuildState(gen, nil, NewBuildReport())
    
    err := stageMyNewStep(ctx, bs)
    require.NoError(t, err)
    
    // Assert expected changes to build state
    assert.Equal(t, 1, bs.Report.MyMetric)
}
```

## Adding a New Content Transform

Content transforms process individual markdown files during the copy stage.

### 1. Create Transform Package

Create `internal/hugo/transforms/my_transform.go`:

```go
package transforms

import (
    "git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

type MyTransform struct{}

func (t MyTransform) Name() string {
    return "my_transform"
}

func (t MyTransform) Stage() TransformStage {
    return StageTransform // Choose appropriate stage
}

func (t MyTransform) Dependencies() TransformDependencies {
    return TransformDependencies{
        MustRunAfter: []string{"relative_link_rewriter"}, // Declare dependencies
    }
}

func (t MyTransform) Transform(p PageAdapter) error {
    // Type assert to access page data
    shim, ok := p.(*PageShim)
    if !ok {
        return nil
    }
    
    // Modify content or add front matter patches
    shim.AddPatch(fmcore.FrontMatterPatch{
        Key:   "my_field",
        Value: "my_value",
        Mode:  fmcore.MergeSetIfMissing,
    })
    return nil
}

func init() {
    // Register with the transform registry
    Register(MyTransform{})
}
```

### 2. Choose Stage and Dependencies

Transforms are organized into stages:
- **StageParse**: Extract/parse source content (e.g., front matter parsing)
- **StageBuild**: Generate base metadata (e.g., repository info, titles)
- **StageEnrich**: Add computed fields (e.g., edit links)
- **StageMerge**: Combine/merge data (e.g., merge user + generated metadata)
- **StageTransform**: Modify content (e.g., link rewriting, syntax conversion)
- **StageFinalize**: Post-process (e.g., strip headings, escape shortcodes)
- **StageSerialize**: Output generation (e.g., write final YAML + content)

Within each stage, declare dependencies to control ordering:
```go
func (t MyTransform) Dependencies() TransformDependencies {
    return TransformDependencies{
        MustRunAfter:  []string{"other_transform"},  // Run after these
        MustRunBefore: []string{"another_transform"}, // Run before these
    }
}
```

Choose a stage that fits your transform's role in the pipeline.

### 3. Import the Transform

Add your transform to the imports in `internal/hugo/transforms/registry.go` or ensure it's imported somewhere:

```go
import (
    _ "git.home.luguber.info/inful/docbuilder/internal/hugo/transforms/my_transform"
)
```

### 4. Write Tests

Test your transform in isolation:

```go
func TestMyTransform(t *testing.T) {
    transform := MyTransform{}
    file := docs.DocFile{
        Name: "test.md",
        Content: []byte("# Test"),
    }
    fm := fmcore.NewFrontMatter()
    
    err := transform.Transform(file, fm)
    require.NoError(t, err)
    
    assert.Equal(t, "my_value", fm.Get("my_field"))
}
```

### 5. Add to Golden Test

Update the ordering golden test to include your transform:

```go
// The golden test will automatically detect your transform
// and fail if the ordering changes unexpectedly
```

## Testing Guidelines

### Unit Tests
- Test individual functions in isolation
- Use table-driven tests for multiple scenarios
- Mock external dependencies (Git operations, file system)

### Integration Tests
- Test complete workflows end-to-end
- Use temporary directories for file operations
- Clean up resources in test teardown

### Golden Tests
- Used for complex output verification (HTML, YAML, JSON)
- Update with `go test -update` when output changes intentionally
- Review golden file changes carefully in PRs

### Test Naming
```go
func TestFunctionName(t *testing.T)                    // Unit test
func TestFunctionName_SpecificScenario(t *testing.T)  // Specific case
func TestIntegration_FeatureName(t *testing.T)        // Integration test
```

## Code Style

### Go Conventions
- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add doc comments for exported functions
- Handle errors explicitly

### Error Handling
```go
// Good: Use internal/foundation/errors package with context
return errors.ValidationError("invalid file type").
    WithContext("filename", filename).
    WithContext("expected", []string{".md", ".markdown"}).
    Build()

// Good: Wrap errors with category and context
return errors.WrapError(err, errors.CategoryGit, "failed to clone repository").
    WithContext("url", repo.URL).
    Build()

// Good: Extract and check classified errors
if classified, ok := errors.AsClassified(err); ok {
    if classified.Category() == errors.CategoryAuth {
        // Handle authentication error
    }
}

// Avoid: Raw errors without classification
return fmt.Errorf("failed to process: %w", err)  // Missing category
```

See [docs/STYLE_GUIDE.md](docs/STYLE_GUIDE.md#error-handling) and [docs/adr/ADR-000-uniform-error-handling.md](docs/adr/ADR-000-uniform-error-handling.md) for complete guidelines.

### Logging
```go
// Use structured logging
slog.Info("Stage completed", 
    "stage", "my_stage", 
    "files_processed", count,
    "duration", time.Since(start))

// Use appropriate log levels
slog.Debug("Detailed debugging info")
slog.Info("Normal operation info") 
slog.Warn("Recoverable issues")
slog.Error("Serious errors")
```

## Pull Request Process

1. **Fork and Branch**:
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Develop and Test**:
   ```bash
   go test ./...
   go build ./cmd/docbuilder
   ```

3. **Update Documentation**:
   - Update README.md if adding user-facing features
   - Add/update relevant documentation files
   - Update CHANGELOG.md with your changes

4. **Commit and Push**:
   ```bash
   git add .
   git commit -m "feat: add my new feature"
   git push origin feature/my-new-feature
   ```

5. **Create Pull Request**:
   - Describe the problem your change solves
   - Include testing steps and examples
   - Reference any related issues

### Commit Message Format

Follow conventional commits:
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Test additions/changes
- `chore:` - Maintenance tasks

## Release Process

Releases are automated using [GoReleaser](https://goreleaser.com/) and GitHub Actions. The workflow builds binaries for multiple platforms and creates multi-architecture Docker images.

### Creating a Release

1. **Ensure CHANGELOG.md is Updated**:
   - Document all changes since the last release
   - Group changes by category (Features, Bug Fixes, etc.)
   - Follow [Keep a Changelog](https://keepachangelog.com/) format

2. **Tag the Release**:
   ```bash
   # Create an annotated tag
   git tag -a v1.2.3 -m "Release v1.2.3"
   
   # Push the tag
   git push origin v1.2.3
   ```

3. **Automated Build**:
   - GitHub Actions automatically triggers the release workflow
   - GoReleaser builds binaries for:
     - Linux (amd64, arm64, arm)
     - macOS (amd64, arm64)
   - Multi-architecture Docker images are built and pushed to GitHub Container Registry

4. **Verify the Release**:
   - Check GitHub releases page for the new release
   - Verify binaries are attached
   - Test Docker images:
     ```bash
     docker pull ghcr.io/inful/docbuilder:v1.2.3
     docker run ghcr.io/inful/docbuilder:v1.2.3 --version
     ```

### Release Artifacts

Each release includes:
- **Binaries**: Pre-compiled executables for all supported platforms
- **Archives**: Tar.gz archives with LICENSE, README, and example config
- **Checksums**: SHA256 checksums for all artifacts
- **Docker Images**: Multi-arch images tagged with:
  - `v1.2.3` - Specific version
  - `v1.2` - Minor version
  - `v1` - Major version
  - `latest` - Latest release

### Docker Image Usage

The release Docker images use pre-built binaries from GoReleaser:

```bash
# Pull the image
docker pull ghcr.io/inful/docbuilder:latest

# Run the daemon
docker run -v ./config.yaml:/config/config.yaml \
  -v ./output:/data \
  -p 8080:8080 \
  ghcr.io/inful/docbuilder:latest

# Run a one-time build
docker run -v ./config.yaml:/config/config.yaml \
  -v ./output:/data \
  ghcr.io/inful/docbuilder:latest build --config /config/config.yaml
```

### Testing Releases Locally

Before tagging a release, you can test the GoReleaser configuration:

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Test release build (snapshot mode)
goreleaser build --snapshot --clean

# Test full release process (dry run)
goreleaser release --snapshot --clean

# Check the artifacts
ls -la dist/
```

### Pre-releases

For beta or release candidate versions, use appropriate tag formats:
- `v1.2.3-beta.1` - Beta release
- `v1.2.3-rc.1` - Release candidate

GoReleaser automatically detects these as pre-releases on GitHub.

## Getting Help

- **Issues**: Open an issue for bugs or feature requests
- **Discussions**: Use GitHub discussions for questions
- **Code Review**: Maintainers will review PRs and provide feedback

## Development Tips

### Debugging
```bash
# Verbose logging
./bin/docbuilder build -c config.yaml -v

# Enable debug logging
DOCBUILDER_LOG_LEVEL=debug ./bin/docbuilder build -c config.yaml

# Test with minimal config
./bin/docbuilder init -c minimal.yaml
```

### Local Development Workflow
1. Make changes to source code
2. Run `go build ./cmd/docbuilder` to compile
3. Test with example configuration
4. Run test suite: `go test ./...`
5. Check for lint issues: `golangci-lint run`

### Performance Testing
```bash
# Measure build performance
time ./bin/docbuilder build -c config.yaml

# Profile memory usage
go build -gcflags="-m" ./cmd/docbuilder

# CPU profiling (if implemented)
./bin/docbuilder build -c config.yaml -profile=cpu
```

Thank you for contributing to DocBuilder! Your improvements help make documentation generation better for everyone.