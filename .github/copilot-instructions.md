# DocBuilder AI Coding Instructions

DocBuilder is a Go CLI tool that aggregates documentation from multiple Git repositories into a single Hugo static site. It uses the Relearn theme with intelligent theme-specific configuration.

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
- `build` - Full pipeline execution (clone → discover → generate Hugo site)
- `init` - Creates example configuration
- `discover` - Discovery-only mode for testing/debugging

Use `go run ./cmd/docbuilder <command> -v` for verbose logging during development.

### Configuration System
- YAML configuration with `${ENV_VAR}` expansion
- Loads environment variables from the first existing file: `.env` then `.env.local` (does not overwrite existing process env)
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
DocBuilder currently **hard-pins** the Hugo theme to **Relearn**.

- The Hugo Modules import is always `github.com/McShelby/hugo-theme-relearn`.
- Any user-provided `hugo.theme` (or older theme fields) should be treated as legacy/no-op; internally the theme is normalized to `relearn`.

Theme configuration is generated in `internal/hugo/config_writer.go` and (at a high level) follows these phases:
1. Core defaults (title/description/baseURL, markup defaults)
2. Apply Relearn theme defaults via `applyRelearnThemeDefaults()`
3. Deep-merge user overrides from `hugo.params`
4. Add dynamic params (eg. `build_date`, optional version metadata)
5. Configure Hugo Modules import for Relearn and ensure a `go.mod` exists
6. Enable math passthrough (Relearn)
7. Configure outputs/taxonomies defaults and language config required by Relearn
8. Convert `hugo.menu` into Hugo config format (if provided)

If you need support for another Hugo theme, treat it as a larger refactor (new config model + generator behavior). Do not add partial “multi-theme” scaffolding.

### File Discovery
Documentation discovery (`internal/docs/discovery.go`) walks configured paths and:
- Discovers markdown files (`.md`, `.markdown`, plus a few common variants) and a small allowlist of static assets (images, pdf, etc.)
- Skips hidden files (leading `.`)
- Ignores standard files **at the docs root**, with one exception: root `README.md` is kept so it can be used as repository index content
- Skips entire repositories that contain a `.docignore` file at the repository root
- Preserves directory structure as Hugo sections
- Normalizes `index.md` → `_index.md` for Hugo section pages
- Detects case-insensitive Hugo path collisions (to prevent Hugo ambiguous reference errors)

Hugo path shapes:
- Single-repository build: `content/{section}/{file}.md`
- Multi-repo build (single forge): `content/{repository}/{section}/{file}.md`
- Multi-forge build (namespaced): `content/{forge}/{repository}/{section}/{file}.md`

### Authentication Handling
Git operations use the auth manager/providers (`internal/auth/`, wired via `internal/git/`):
- **SSH**: Uses `~/.ssh/id_rsa` by default or specified `key_path`
- **Token**: Username="token", Password=token (GitHub/GitLab pattern)
- **Basic**: Standard username/password auth

Environment variables are commonly used: `${GIT_ACCESS_TOKEN}`, `${GITHUB_TOKEN}`

## Common Development Tasks

### Searching the Codebase

**Prefer ripgrep (`rg`) when available; otherwise use the editor search/Go tools.**

If you are running in this Copilot agent environment, you can also use the workspace search tool instead of shelling out.

```bash
# Search for pattern (automatically respects .gitignore)
rg "pattern"

# Case-insensitive search
rg -i "pattern"

# Search with statistics
rg -i "pattern" --stats

# Search specific file types
rg -t go "pattern"

# List files containing pattern
rg -l "pattern"
```

**Why ripgrep:**
- Automatically respects `.gitignore` (excludes build artifacts, caches, etc.)
- Much faster than grep on large codebases
- Better defaults (recursive, colored output, line numbers)
- Correctly handles binary files and UTF-8

### Adding Hugo Theme Support
DocBuilder is Relearn-only today. Do not introduce theme-selection or a theme registry unless the change explicitly includes:
- Config schema changes + normalization
- Generator changes (config, layouts, assets, content assumptions)
- Golden tests covering the new theme behavior

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

### TDD Workflow (Strict)
- Follow strict TDD: write the test first, watch it fail, then implement the change.
- When fixing a bug, always add a test that reproduces the issue **before** the fix.
- The reproducer test must remain after the fix as a long-term regression test (do not add temporary/throwaway tests).

### Test File Organization

**Follow Go standard conventions for organizing test files:**

#### Unit Tests (Match Source Files)

Unit tests should be co-located with source code in files matching the source filename with `_test.go` suffix:

```
internal/lint/
├── fixer.go                    # Main fixer logic
├── fixer_test.go              # Tests for fixer.go
├── fixer_result.go            # Result formatting
├── fixer_result_test.go       # Tests for fixer_result.go
├── fixer_utils.go             # Utility functions
├── fixer_utils_test.go        # Tests for fixer_utils.go
├── fixer_file_ops.go          # File operations
└── fixer_file_ops_test.go     # Tests for fixer_file_ops.go
```

**Unit test guidelines:**
- File name: `<source_file>_test.go` (e.g., `fixer_result.go` → `fixer_result_test.go`)
- Test name: `Test<Type>_<Method>` or `Test<FunctionName>` (e.g., `TestFixResult_PreviewChanges`, `TestPathsEqualCaseInsensitive`)
- Scope: Test individual functions, methods, or types from a single source file
- Purpose: Fast, focused tests for specific functionality
- Dependencies: Minimal - use mocks/stubs where needed

#### Integration/Workflow Tests (Feature-Based)

Integration tests should be named by the feature or workflow they test:

```
internal/lint/
├── fixer_workflow_test.go     # End-to-end fix workflow tests
└── golden_test.go             # Golden file comparison tests
```

**Integration test guidelines:**
- File name: `<feature>_test.go` (e.g., `fixer_workflow_test.go`, `link_update_test.go`)
- Test name: `Test<Feature>_<Scenario>` (e.g., `TestFix_SuccessfulRenameWithLinkUpdates`)
- Scope: Test multiple components working together
- Purpose: Verify end-to-end workflows and integration points
- Dependencies: May use real file systems, temp directories, full objects

#### When to Split Test Files

Split a monolithic test file when:
1. **It exceeds ~500 lines** and contains tests for multiple source files
2. **Tests cover distinct modules** that have been split into separate source files
3. **Mixing unit and integration tests** in the same file makes organization unclear
4. **Adding tests becomes difficult** due to unrelated test setup/helpers in the file

**Splitting procedure:**
1. Identify which tests belong to which source files
2. Create new `<source_file>_test.go` files for unit tests
3. Move tests to corresponding files based on what they test
4. Rename remaining integration tests to `<feature>_test.go`
5. Run full test suite to verify: `go test ./...`
6. Check no tests were lost: compare test counts before/after

**Example split:**
```go
// Before: fixer_test.go (500 lines, 30 tests, mixed)
// After:
// - fixer_test.go (12 tests for main fixer.go)
// - fixer_result_test.go (7 tests for fixer_result.go)
// - fixer_utils_test.go (2 tests for fixer_utils.go)
// - fixer_file_ops_test.go (2 tests for fixer_file_ops.go)
// - fixer_workflow_test.go (7 tests for end-to-end workflows)
```

#### Test Organization Benefits

Following this pattern provides:
- **Easy navigation**: Tests for `fixer_result.go`? Check `fixer_result_test.go`
- **Clear scope**: File name indicates what's being tested
- **Better maintainability**: Changes to a module only affect its test file
- **Standard Go conventions**: Familiar pattern for Go developers
- **CI/CD efficiency**: Can run unit tests separately from integration tests

#### Test File Checklist

When creating or reorganizing test files:
- [ ] Unit test files match source file names (`filename_test.go`)
- [ ] Integration test files named by feature (`workflow_test.go`)
- [ ] Tests are in the same package as source code
- [ ] Each test file has clear, focused scope
- [ ] Test names follow conventions (`Test<Type>_<Method>`)
- [ ] All tests pass: `go test ./...`
- [ ] No duplicate tests across files
- [ ] Test count matches before refactoring

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

Follow the existing patterns in the repo (eg. the Hugo config golden tests under `internal/hugo/`).

Avoid hard-coding a theme in test configs/snippets: DocBuilder always normalizes to Relearn.

Golden file location: `internal/hugo/testdata/hugo_config/feature_name.yaml`

#### 3. Create Integration Golden Test

**Required Files:**
1. Test repository: `test/testdata/repos/<feature>/docs/*.md`
2. Test configuration: `test/testdata/configs/<feature>.yaml`
3. Test function: `test/integration/<feature>_golden_test.go`
4. Golden directory: `test/testdata/golden/<feature>/`

**Test Repository Structure:**
```bash
test/testdata/repos/<feature>/
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
func TestGolden_<Feature>(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping golden test in short mode")
    }

    // Setup test repository (automatically initializes git)
    repoPath := setupTestRepo(t, "../../test/testdata/repos/<feature>")

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
    goldenDir := "../../test/testdata/golden/<feature>"
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
go test ./test/integration -run TestGolden_<Feature> -v -update-golden

# Verify test passes without update flag
go test ./test/integration -run TestGolden_<Feature> -v

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
- **Descriptive Names**: Test names should clearly indicate feature: `TestGolden_RelearnTransitions`
- **Realistic Content**: Test repositories should have realistic markdown content with proper frontmatter
- **Documentation**: Add comments explaining what each golden file verifies
- **Coverage**: Test both enabled and disabled states of features
- **Regression Testing**: Run all tests after changes to ensure no existing functionality breaks

## Task Completion Checklist

**Before marking any task as complete, you MUST complete all steps in this checklist:**

### 1. Run golangci-lint and Fix All Issues

```bash
# DocBuilder uses golangci-lint v2.8.0
golangci-lint version

# Run linter and format code
golangci-lint run --fix

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
- [ ] `golangci-lint run --fix` passes with no issues
- [ ] `go test ./test/integration -v` passes (all golden tests)
- [ ] `go test ./...` passes (full test suite)
- [ ] Only task-related files are staged (`git diff --cached`)
- [ ] Commit message follows Conventional Commits format

**Do not mark a task as complete until all checklist items are verified.**

### Debugging Git Issues
- Use `incremental` flag to avoid re-cloning during development
- Check authentication with verbose logging: `-v` flag
- Test with both public and private repositories

### Debugging Build Failures

When debugging Hugo build failures or investigating content generation issues, use the `--keep-workspace` flag to preserve intermediate build artifacts:

```bash
# Preserve workspace and staging directories on build failure
go run ./cmd/docbuilder build -o /tmp/output --keep-workspace -v

# Local mode with workspace preservation (no git required)
go run ./cmd/docbuilder build -o /tmp/output --docs-dir ./docs --keep-workspace -v
```

**What gets preserved:**
- **Git workspace**: `/tmp/docbuilder-{timestamp}/` - Contains cloned repositories
- **Staging directory**: `{output}_stage` - Hugo site before atomic promotion
- **All intermediate files**: Discovered docs, generated content, Hugo config

**When to use:**
- Hugo build errors (ambiguous references, template errors, broken links)
- Content transformation issues (frontmatter, path collisions)
- Theme configuration problems
- Investigating why specific files aren't being included

**Inspection workflow:**
1. Run build with `--keep-workspace`
2. On failure, paths to preserved directories are displayed
3. Inspect Hugo config: `cat {output}_stage/hugo.yaml`
4. Check content structure: `tree {output}_stage/content/`
5. Verify frontmatter: `head -n 20 {output}_stage/content/_index.md`
6. Try manual Hugo build: `cd {output}_stage && hugo`

**Cleanup:**
Preserved directories are not automatically removed. Clean up manually when done:
```bash
rm -rf /tmp/docbuilder-*
rm -rf /tmp/output_stage
```

### Working with Configuration
- Always test environment variable expansion with `.env` files  
- Repository names become Hugo content sections - avoid spaces/special chars
- The `paths` array allows multiple doc directories per repo

### Refactoring Monolithic Files

When a file grows too large (>1000 lines) or has multiple responsibilities, split it into focused modules:

#### Module Splitting Strategy

**1. Identify distinct responsibilities** in the monolithic file:
```go
// fixer.go (1487 lines) - TOO LARGE
// Contains: main logic, result formatting, utils, file ops, link detection, broken link checking
```

**2. Create separate files for each responsibility:**
```go
fixer.go             // Core fixer logic (main struct, public API)
fixer_result.go      // Result types and formatting
fixer_utils.go       // Utility functions (path comparison, file existence)
fixer_file_ops.go    // File operations (rename, backup, git detection)
fixer_link_detection.go   // Link parsing and discovery
fixer_broken_links.go     // Broken link detection
```

**3. Move code systematically:**
- Start with clear, independent modules (utils, types)
- Move related functions together
- Maintain all public APIs in the main file or export from new modules
- Keep all code in the same package (no new packages unless necessary)

**4. Update tests in parallel:**
- Split test files following same structure (see Test File Organization)
- Ensure all tests still pass after each move
- Verify test coverage remains the same

**5. Refactoring checklist:**
- [ ] When adding a new feature, use a strict TDD approach.
- [ ] When fixing a bug, first add a failing reproducer test, then fix the bug; keep the test as a permanent regression test.
- [ ] Each file has single, clear responsibility
- [ ] File names clearly indicate content
- [ ] No circular dependencies between new files
- [ ] All tests pass: `go test ./...`
- [ ] Linter passes: `golangci-lint run --fix`
- [ ] Public API unchanged (no breaking changes)
- [ ] Documentation updated if needed

#### File Naming for Split Modules

Use descriptive suffixes that indicate responsibility:
- `<base>_result.go` - Result types and formatting
- `<base>_utils.go` - Utility/helper functions
- `<base>_<feature>.go` - Feature-specific logic (e.g., `fixer_broken_links.go`)
- `<base>_ops.go` - Operations (e.g., `fixer_file_ops.go`)

**Example split:**
```
# Before (1 file, 1487 lines)
internal/lint/fixer.go

# After (7 files, 1487 lines total)
internal/lint/fixer.go              # 387 lines - core logic
internal/lint/fixer_result.go       # 203 lines - result types
internal/lint/fixer_utils.go        # 89 lines - utilities
internal/lint/fixer_file_ops.go     # 187 lines - file operations
internal/lint/fixer_link_detection.go   # 312 lines - link parsing
internal/lint/fixer_broken_links.go     # 197 lines - broken links
internal/lint/fixer_confirmation.go     # 112 lines - user confirmation
```

#### Benefits of Module Splitting

- **Readability**: Easier to understand focused, single-purpose files
- **Maintainability**: Changes to one feature don't affect unrelated code
- **Testing**: Each module can have dedicated unit tests
- **Collaboration**: Reduces merge conflicts when multiple developers work on same package
- **Code review**: Smaller, focused files are easier to review

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