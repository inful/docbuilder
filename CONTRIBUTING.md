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

- **Pipeline Stages**: Sequential build steps (clone, discover, generate, etc.)
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

Create `internal/hugo/transforms/my_transform/`:

```go
package my_transform

import (
    "git.home.luguber.info/inful/docbuilder/internal/docs"
    "git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

const Priority = 25 // Choose appropriate priority

type MyTransform struct{}

func (t MyTransform) Name() string {
    return "my_transform"
}

func (t MyTransform) Priority() int {
    return Priority
}

func (t MyTransform) Transform(file docs.DocFile, fm *fmcore.FrontMatter) error {
    // Modify file content or front matter
    fm.Set("my_field", "my_value")
    return nil
}

func init() {
    // Register with the transform registry
    transforms.Register(MyTransform{})
}
```

### 2. Choose Priority

Transform priorities determine execution order:
- **10**: Front matter parsing
- **20**: Front matter building  
- **30**: Edit link injection
- **40**: Front matter merge
- **50**: Link rewriting
- **90**: Front matter serialization

Choose a priority that fits your transform's role in the pipeline.

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

## Adding a New Hugo Theme

Hugo themes provide theme-specific configuration and capabilities.

### 1. Create Theme Package

Create `internal/hugo/themes/mytheme/theme_mytheme.go`:

```go
package mytheme

import (
    "git.home.luguber.info/inful/docbuilder/internal/config"
    "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
)

type MyTheme struct{}

func (t MyTheme) Name() config.Theme {
    return config.ThemeMyTheme
}

func (t MyTheme) Features() theme.ThemeFeatures {
    return theme.ThemeFeatures{
        UsesModules:            true,
        ModulePath:            "github.com/user/mytheme",
        WantsPerPageEditLinks: true,
        SupportsSearchJSON:    true,
        AutoMainMenu:          true,
    }
}

func (t MyTheme) ApplyParams(ctx theme.ParamContext, params map[string]any) {
    // Set theme-specific defaults
    if _, exists := params["mytheme"]; !exists {
        params["mytheme"] = map[string]any{
            "search": map[string]any{
                "enabled": true,
            },
        }
    }
}

func init() {
    theme.RegisterTheme(MyTheme{})
}
```

### 2. Add Theme Constant

Add to `internal/config/theme.go`:

```go
const (
    ThemeHextra Theme = "hextra"
    ThemeDocsy  Theme = "docsy"
    ThemeMyTheme Theme = "mytheme" // Add your theme
)
```

### 3. Write Tests

Test theme parameter application:

```go
func TestMyThemeApplyParams(t *testing.T) {
    theme := MyTheme{}
    params := make(map[string]any)
    ctx := theme.ParamContext{}
    
    theme.ApplyParams(ctx, params)
    
    assert.Contains(t, params, "mytheme")
}
```

For detailed theme integration guidance, see [`THEME_INTEGRATION.md`](./THEME_INTEGRATION.md).

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
// Good: Wrap errors with context
return fmt.Errorf("failed to process file %s: %w", filename, err)

// Good: Use typed errors where appropriate
return newFatalStageError(StageName, err)

// Avoid: Silent error ignoring
_ = someFunction() // Only if truly safe
```

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