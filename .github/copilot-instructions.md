# DocBuilder AI Coding Instructions

DocBuilder is a Go CLI tool that aggregates documentation from multiple Git repositories into a single Hugo static site. It supports themes like Hextra and Docsy with intelligent theme-specific configuration.

## Architecture Overview

The application follows a pipeline pattern:
1. **Configuration** (`internal/config/`) - Loads YAML config with environment variable expansion
2. **Workspace** (`internal/workspace/`) - Creates temporary directories for Git operations  
3. **Git Client** (`internal/git/`) - Handles repository cloning/updating with authentication
4. **Discovery** (`internal/docs/`) - Finds markdown files in configured paths within repos
5. **Hugo Generator** (`internal/hugo/`) - Creates Hugo sites with theme-specific optimizations

Key data flow: `Config → Git Clone → Doc Discovery → Hugo Site Generation`

## Development Patterns

### Command Structure
The CLI uses [Kong](https://github.com/alecthomas/kong) for command parsing. Main commands in `cmd/docbuilder/main.go`:
- `build` - Full pipeline execution (clone → discover → generate)
- `init` - Creates example configuration
- `discover` - Discovery-only mode for testing/debugging

Use `go run ./cmd/docbuilder <command> -v` for verbose logging during development.

### Configuration System
- YAML configuration with `${ENV_VAR}` expansion
- Auto-loads `.env` and `.env.local` files
- Repository-specific paths (defaults to `["docs"]`)
- Three auth types: `ssh`, `token`, `basic`

Example repository config:
```yaml
repositories:
  - url: https://github.com/org/repo.git
    name: repo-name
    branch: main
    paths: ["docs", "documentation"]
    auth:
      type: token
      token: "${GITHUB_TOKEN}"
```

### Theme System
Theme logic is implemented via a lightweight interface in `internal/hugo/theme` with concrete packages under `internal/hugo/themes/` (e.g. `hextra`, `docsy`). Each theme registers itself in `init()` and exposes:

```
type Theme interface {
  Name() config.Theme
  Features() ThemeFeatures             // capability flags (modules, math, search json, etc.)
  ApplyParams(ctx ParamContext, params map[string]any)  // inject default/normalized params
  CustomizeRoot(ctx ParamContext, root map[string]any)  // final root-level tweaks (optional)
}
```

Generation phases (`generateHugoConfig`):
1. Core defaults (title, baseURL, markup)
2. `ApplyParams` (theme fills or normalizes `params`)
3. User param deep-merge (config overrides)
4. Dynamic fields (`build_date`)
5. Theme module import block (if `Features().UsesModules`)
6. Automatic menu (if `Features().AutoMainMenu`)
7. `CustomizeRoot` (final adjustments)

Adding a new theme:
1. Create `internal/hugo/themes/<name>/theme_<name>.go`
2. Implement the interface and call `theme.RegisterTheme(Theme{})` in `init()`
3. Populate `ThemeFeatures` (set `UsesModules`, `ModulePath`, defaults)
4. Provide sane defaults in `ApplyParams` (avoid overwriting user-provided keys)
5. (Optional) adjust root in `CustomizeRoot`
6. Add/extend golden config test for `hugo.yaml`

Legacy helper functions (`addHextraParams`, `addDocsyParams`) have been removed; all new logic should go through the theme interface.

### File Discovery
Documentation discovery (`internal/docs/discovery.go`) walks configured paths and:
- Only processes `.md`/`.markdown` files
- Ignores standard files: `README.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `LICENSE.md`
- Preserves directory structure as Hugo sections
- Builds Hugo-compatible paths: `content/{repository}/{section}/{file}.md`

### Authentication Handling
Git client (`internal/git/git.go`) supports multiple auth methods:
- **SSH**: Uses `~/.ssh/id_rsa` by default or specified `key_path`
- **Token**: Username="token", Password=token (GitHub/GitLab pattern)
- **Basic**: Standard username/password auth

Environment variables are commonly used: `${GIT_ACCESS_TOKEN}`, `${GITHUB_TOKEN}`

## Common Development Tasks

### Adding New Hugo Theme Support
1. Update `addThemeParams()` logic in `hugo/generator.go`
2. Add module import pattern if theme supports Hugo Modules
3. Set theme-specific defaults (search, UI, etc.)
4. Test with example configuration

### Testing Changes
```bash
# Test with example config
make build
./bin/docbuilder init -c test-config.yaml
# Edit test-config.yaml with local repos
./bin/docbuilder build -c test-config.yaml -v

# Test discovery only
./bin/docbuilder discover -c test-config.yaml -v
```

### Creating Golden Tests for New Features

**When adding features that modify output (Hugo config, content structure, assets, etc.), always create golden tests following this procedure:**

#### 1. Determine Test Type

**Unit Golden Test** (for isolated component changes):
- Location: `internal/hugo/*_golden_test.go` (same package as feature)
- Purpose: Verify specific output format (e.g., Hugo config YAML generation)
- Example: `TestHugoConfigGolden_Transitions` for View Transitions config params

**Integration Golden Test** (for end-to-end verification):
- Location: `test/integration/golden_test.go` or separate `*_golden_test.go`
- Purpose: Verify complete build pipeline output
- Pattern: Follow existing `TestGolden_*` tests

#### 2. Create Unit Golden Test (if applicable)

```go
// In internal/hugo/*_golden_test.go
func TestHugoConfigGolden_FeatureName(t *testing.T) {
    cfg := &config.Config{
        Hugo: config.HugoConfig{
            Theme:          config.ThemeHextra,
            EnableFeature:  true,
            FeatureOption:  "value",
        },
    }
    cfg.Init()
    
    actual := generateHugoConfig(cfg)
    
    goldenPath := "testdata/hugo_config/feature_name.yaml"
    compareGolden(t, actual, goldenPath)
}
```

Golden file location: `internal/hugo/testdata/hugo_config/feature_name.yaml`

#### 3. Create Integration Golden Test

**Required Files:**
1. Test repository: `test/testdata/repos/themes/<theme>-<feature>/docs/*.md`
2. Test configuration: `test/testdata/configs/<theme>-<feature>.yaml`
3. Test function: `test/integration/<feature>_golden_test.go`
4. Golden directory: `test/testdata/golden/<theme>-<feature>/`

**Test Repository Structure:**
```bash
test/testdata/repos/themes/<theme>-<feature>/
└── docs/
    ├── README.md          # Main documentation
    ├── getting-started.md # Additional content
    └── configuration.md   # Feature-specific docs
```

**Test Configuration Pattern:**
```yaml
version: "2.0"

repositories:
  - name: feature-demo
    url: PLACEHOLDER  # Replaced by setupTestRepo()
    branch: main
    paths:
      - docs

hugo:
  title: "Feature Demo"
  description: "Test documentation site for <feature>"
  base_url: "http://localhost:1313/"
  theme: "hextra"
  enable_feature: true      # Feature-specific config
  feature_option: "value"   # Feature-specific options
  params:
    navbar:
      displayTitle: true
    footer:
      displayPoweredBy: false

output:
  directory: PLACEHOLDER  # Replaced by t.TempDir()
  clean: true
```

**Test Function Pattern:**
```go
func TestGolden_<Theme><Feature>(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping golden test in short mode")
    }

    // Setup test repository (automatically initializes git)
    repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/<theme>-<feature>")

    // Load and configure
    cfg := loadGoldenConfig(t, "../../test/testdata/configs/<theme>-<feature>.yaml")
    cfg.Repositories[0].URL = repoPath
    outputDir := t.TempDir()
    cfg.Output.Directory = outputDir

    // Run build pipeline
    svc := build.NewBuildService().
        WithHugoGeneratorFactory(func(cfgAny any, outDir string) build.HugoGenerator {
            return hugo.NewGenerator(cfgAny.(*config.Config), outDir)
        })

    result, err := svc.Run(context.Background(), build.BuildRequest{
        Config:    cfg,
        OutputDir: outputDir,
    })
    require.NoError(t, err, "build pipeline failed")
    require.Equal(t, build.BuildStatusSuccess, result.Status)

    // Verify outputs against golden files
    goldenDir := "../../test/testdata/golden/<theme>-<feature>"
    verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)
    verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
    
    // Feature-specific verification (if needed)
    verifyFeatureAssets(t, outputDir, goldenDir+"/feature-assets.golden.json", *updateGolden)
}
```

#### 4. Create Custom Verification Helper (if needed)

If your feature generates custom assets or outputs, add a verification helper to `test/integration/helpers.go`:

```go
// FeatureAssets represents custom feature output for golden testing.
type FeatureAssets struct {
    Files  map[string]FeatureFile `json:"files"`
    Config map[string]interface{} `json:"config"`
}

type FeatureFile struct {
    Exists      bool     `json:"exists"`
    Size        int64    `json:"size"`
    ContentHash string   `json:"contentHash"`
    Markers     []string `json:"markers"`  // Key content markers to verify
}

// verifyFeatureAssets verifies feature-specific assets were generated correctly.
func verifyFeatureAssets(t *testing.T, outputDir, goldenPath string, updateGolden bool) {
    t.Helper()

    // Collect actual output
    actual := FeatureAssets{
        Files: make(map[string]FeatureFile),
    }
    
    // Check feature files exist and contain expected markers
    featurePath := filepath.Join(outputDir, "static", "feature-file.ext")
    actual.Files["feature-file.ext"] = verifyAssetFile(t, featurePath, 
        []string{"marker1", "marker2"})

    // Update or compare golden file
    if updateGolden {
        data, err := json.MarshalIndent(actual, "", "  ")
        require.NoError(t, err)
        err = os.MkdirAll(filepath.Dir(goldenPath), 0755)
        require.NoError(t, err)
        err = os.WriteFile(goldenPath, data, 0644)
        require.NoError(t, err)
        t.Logf("Updated golden file: %s", goldenPath)
        return
    }

    goldenData, err := os.ReadFile(goldenPath)
    require.NoError(t, err, "failed to read golden file: %s", goldenPath)
    
    var expected FeatureAssets
    err = json.Unmarshal(goldenData, &expected)
    require.NoError(t, err)

    actualJSON, _ := json.MarshalIndent(actual, "", "  ")
    expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
    require.JSONEq(t, string(expectedJSON), string(actualJSON))
}
```

#### 5. Generate Golden Files

```bash
# Create golden directory
mkdir -p test/testdata/golden/<theme>-<feature>

# Generate golden files with update flag
go test ./test/integration -run TestGolden_<Theme><Feature> -v -update-golden

# Verify test passes without update flag
go test ./test/integration -run TestGolden_<Theme><Feature> -v

# Run all integration tests to ensure no regressions
go test ./test/integration -v
```

#### 6. Verify Golden Files

Check that generated golden files contain expected content:
```bash
# Hugo config should include feature parameters
grep -i "feature" test/testdata/golden/<theme>-<feature>/hugo-config.golden.yaml

# Content structure should include all expected files
cat test/testdata/golden/<theme>-<feature>/content-structure.golden.json

# Feature-specific assets should exist
cat test/testdata/golden/<theme>-<feature>/feature-assets.golden.json
```

#### 7. Common Verification Helpers

Available in `test/integration/helpers.go`:

- `verifyHugoConfig()` - Compare Hugo config YAML (auto-normalizes dynamic fields like build_date)
- `verifyContentStructure()` - Compare content files and frontmatter (auto-normalizes dates, temp paths)
- `verifyAssetFile()` - Check file exists, size, hash, and content markers
- `setupTestRepo()` - Create git repository from testdata (handles git init automatically)
- `loadGoldenConfig()` - Load test configuration with placeholder support

#### 8. Best Practices

- **Normalization**: Dynamic fields (timestamps, temp paths) must be normalized before comparison
- **Markers**: Use content markers (e.g., specific CSS/JS patterns) instead of full content comparison
- **Descriptive Names**: Test names should clearly indicate theme and feature: `TestGolden_HextraTransitions`
- **Realistic Content**: Test repositories should have realistic markdown content with proper frontmatter
- **Documentation**: Add comments explaining what each golden file verifies
- **Coverage**: Test both enabled and disabled states of features
- **Regression Testing**: Run all tests after changes to ensure no existing functionality breaks

## Task Completion Checklist

**Before marking any task as complete, you MUST complete all steps in this checklist:**

### 1. Run golangci-lint and Fix All Issues

```bash
# Run linter
golangci-lint run

# Fix any issues reported
# Re-run until no issues remain
golangci-lint run
```

All linting issues must be resolved. No warnings or errors should remain.

### 2. Verify Golden Tests Pass

```bash
# Run all golden tests (integration tests)
go test ./test/integration -v

# All tests must pass - no failures or skips
```

If golden tests fail:
- Check if feature changes require updating golden files
- Use `-update-golden` flag if output changes are intentional and correct
- Re-run tests to verify they pass against updated golden files

### 3. Run Full Test Suite

```bash
# Run all tests in the project
go test ./... -v

# Verify 100% pass rate
go test ./... -count=1  # Disable caching to ensure fresh run
```

All tests must pass without failures. If any test fails:
- Fix the issue causing the failure
- Do not commit broken tests
- Re-run full suite until all tests pass

### 4. Stage Only Task-Related Files

```bash
# Review what files changed
git status

# Stage only files modified for this specific task
git add <file1> <file2> ...

# OR use interactive staging
git add -p

# Verify staged files are correct
git diff --cached --name-only
```

**Do not stage:**
- Unrelated changes from other work
- Temporary files or debug output
- Configuration files not part of the task

### 5. Commit with Conventional Commit Message

Use the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Common types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring without behavior changes
- `perf`: Performance improvements
- `style`: Code style changes (formatting, missing semicolons, etc.)
- `chore`: Build process or auxiliary tool changes
- `ci`: CI/CD configuration changes

**Examples:**
```bash
# Feature with scope
git commit -m "feat(hugo): add View Transitions API support

- Add EnableTransitions config option
- Implement CSS assets with go:embed
- Add copyTransitionAssets() function
- Update Hugo config generation to inject transition params
- Add comprehensive unit and integration golden tests"

# Bug fix
git commit -m "fix(git): handle SSH authentication for private repos"

# Documentation
git commit -m "docs(copilot): add golden test creation procedure"

# Tests only
git commit -m "test(hugo): add integration tests for View Transitions"

# Breaking change
git commit -m "feat(config)!: rename theme field to hugo_theme

BREAKING CHANGE: Configuration field 'theme' renamed to 'hugo_theme' for clarity"
```

**Commit message guidelines:**
- Use present tense ("add feature" not "added feature")
- Use imperative mood ("move cursor to..." not "moves cursor to...")
- First line should be 50-72 characters
- Separate subject from body with blank line
- Use body to explain what and why, not how
- Reference issues/PRs in footer if applicable

### Checklist Summary

Before completing any task, verify:
- [ ] `golangci-lint run` passes with no issues
- [ ] `go test ./test/integration -v` passes (all golden tests)
- [ ] `go test ./...` passes (full test suite)
- [ ] Only task-related files are staged (`git diff --cached`)
- [ ] Commit message follows Conventional Commits format

**Do not mark a task as complete until all checklist items are verified.**

### Debugging Git Issues
- Use `incremental` flag to avoid re-cloning during development
- Check authentication with verbose logging: `-v` flag
- Test with both public and private repositories

### Working with Configuration
- Always test environment variable expansion with `.env` files  
- Repository names become Hugo content sections - avoid spaces/special chars
- The `paths` array allows multiple doc directories per repo

## Code Conventions

**See [docs/STYLE_GUIDE.md](../docs/STYLE_GUIDE.md) for complete naming conventions and style rules.**

### Quick Reference

**Variable Naming:**
- Use consistent abbreviations: `repo` (not `repository`), `cfg` (not `config`/`configuration`), `auth` (not `authentication`), `dir` (not `directory`)
- Scope-based naming: single letters for very short scopes (< 10 lines), abbreviated for function parameters, descriptive for struct fields
- Boolean variables: prefix with `is`, `has`, `should`, `can`, `enable`
- Examples: `authCfg *config.AuthConfig`, `repoURL string`, `buildCfg *BuildConfig`

**Function Naming:**
- Public functions: descriptive, full words (e.g., `CloneRepo`, `UpdateRepo`, `ComputeRepoHash`)
- Private functions: can use abbreviations (e.g., `getAuth`, `fetchOrigin`)
- No `Get`/`Set` prefixes for simple accessors; use `Get` when fetching requires computation/I/O
- Predicate functions read like questions: `IsAncestor()`, `HasAuth()`, `ShouldRetry()`

**Type Naming:**
- Use full words without abbreviations: `RemoteHeadCache`, `BuildConfig`, `AuthProvider`
- Error types suffix with `Error`: `AuthError`, `NotFoundError`, `NetworkTimeoutError`
- Interface types prefer `-er` suffix: `Cloner`, `Generator`

**Error Handling:**
- Use `internal/foundation/errors` package for all error construction
- Package-level sentinel errors prefix with `Err`: `ErrNotFound`, `ErrUnauthorized`
- Build errors with fluent API: `errors.ValidationError("msg").WithContext("key", val).Build()`
- Error categories are type-safe: `errors.CategoryValidation`, `errors.CategoryAuth`, `errors.CategoryNotFound`
- Extract classified errors: `classified, ok := errors.AsClassified(err)`
- Error messages start with lowercase, be specific, include context
- Example: `errors.WrapError(err, errors.CategoryGit, "failed to clone repository").WithContext("url", repo.URL).Build()`

**Other Conventions:**
- Use structured logging with `slog` package throughout
- File paths must be absolute when passed between packages
- Hugo paths use forward slashes even on Windows (`filepath.ToSlash()`)
- Error wrapping uses the errors package builder pattern
- Configuration validation happens in `config.Load()` with sensible defaults

## Integration Points

**External Dependencies:**
- `github.com/go-git/go-git/v5` for Git operations
- `github.com/alecthomas/kong` for CLI parsing  
- `gopkg.in/yaml.v3` for configuration
- Hugo must be available in PATH for final site building

**File System Layout:**
- Temporary workspaces in `/tmp/docbuilder-{timestamp}/`
- Hugo sites generated in configured output directory
- Repository clones are ephemeral unless using incremental mode