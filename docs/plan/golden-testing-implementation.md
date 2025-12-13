# Golden Testing Implementation Plan

**ADR**: [ADR-001: Golden Testing Strategy](../docs/adr/ADR-001-golden-testing-strategy.md)  
**Created**: 2025-12-12  
**Status**: In Progress

## Overview

Implement a golden testing framework for end-to-end verification of DocBuilder's output quality. This enables confident refactoring, systematic bug reproduction, and feature documentation through tests.

## Goals

1. ✅ Verify complete build pipeline (git → docs → hugo → render)
2. ✅ Catch regressions in content transformations
3. ✅ Document supported features through test cases
4. ✅ Enable safe refactoring with automated verification
5. ✅ Standardize bug reproduction workflow

## Non-Goals

- Testing Hugo theme internals (trust upstream)
- Visual regression testing (screenshot comparison)
- Performance benchmarking (separate concern)
- Load testing (daemon-specific)

## Phases

### Phase 1: Foundation (Week 1) ✅ COMPLETED

**Goal**: Core framework + one working test case

#### Tasks

- [x] Create directory structure
  - `test/testdata/repos/` for source repositories
  - `test/testdata/configs/` for test configurations
  - `test/testdata/golden/` for verified outputs
  - `test/integration/` for test code

- [x] Implement test helpers
  - `setupTestRepo(t, path) string` - creates temp git repo from directory
  - `cleanupTestRepo(t, path)` - automatic via `t.TempDir()`
  - `loadGoldenConfig(t, configPath) *config.Config` - loads test config
  - `verifyHugoConfig(t, outputDir, goldenPath)` - compares hugo.yaml
  - `verifyContentStructure(t, outputDir, goldenPath)` - checks content files
  - `normalizeDynamicFields()` - normalizes timestamps and build dates
  - `normalizeFrontMatter()` - normalizes date fields in front matter
  - `parseFrontMatter()` - extracts YAML front matter
  - `buildStructureTree()` - creates directory structure map
  - `copyDir()` / `copyFile()` - file system utilities

- [x] Add CLI flag support
  - `-update-golden` flag in test framework
  - `-skip-render` flag declared (for future use)
  - Short mode detection (`testing.Short()`)

- [x] Create first test case
  - Source repo: `testdata/repos/themes/hextra-basic/`
    - 3 markdown files (index.md, guide.md, api.md)
    - Varied content with front matter, code blocks, headings
  - Config: `testdata/configs/hextra-basic.yaml`
  - Golden files: `testdata/golden/hextra-basic/`
    - `hugo-config.golden.yaml` ✅ Generated (1.7KB)
    - `content-structure.golden.json` ✅ Generated (1.5KB)

- [x] Documentation
  - Created comprehensive `test/integration/README.md`
  - Includes usage guide, best practices, troubleshooting
  - How to create new golden tests documented

**Deliverable**: ✅ `go test ./test/integration -run TestGolden_HextraBasic` passes

---

### Phase 2: Core Test Coverage (Week 2)

**Goal**: Cover essential themes and transformations

#### Test Cases to Add

**Theme Tests** (`testdata/repos/themes/`)
- [ ] `hextra-math/` - KaTeX math rendering
- [ ] `hextra-search/` - Search index generation
- [ ] `hextra-multilang/` - Multi-language support
- [ ] `docsy-basic/` - Basic Docsy theme features
- [ ] `docsy-api/` - Docsy API documentation layout

**Transformation Tests** (`testdata/repos/transforms/`)
- [ ] `frontmatter-injection/` - editURL, metadata injection
- [ ] `cross-repo-links/` - Link transformation between repos
- [ ] `image-paths/` - Asset path handling
- [ ] `section-indexes/` - `_index.md` generation
- [ ] `menu-generation/` - Automatic menu creation

**Multi-Repo Tests** (`testdata/repos/multi-repo/`)
- [ ] `two-repos/` - Basic multi-repo aggregation
- [ ] `conflicting-paths/` - Same-named files from different repos
- [ ] `different-themes/` - Verify single theme applies to all

**Configuration**: Create corresponding YAML configs in `testdata/configs/`

**Golden Files**: Generate and verify for each test case

**Deliverable**: 10+ golden tests covering major features

---

### Phase 3: Edge Cases & Regression Tests (Week 3)

**Goal**: Handle errors and reproduce historical bugs

#### Test Cases to Add

**Edge Cases** (`testdata/repos/edge-cases/`)
- [ ] `empty-docs/` - Repository with no markdown files
- [ ] `only-readme/` - Repository with only README.md
- [ ] `malformed-frontmatter/` - Invalid YAML in front matter
- [ ] `binary-files/` - Non-text files in docs folder
- [ ] `deep-nesting/` - Many subdirectory levels
- [ ] `unicode-names/` - Files with non-ASCII characters
- [ ] `special-chars/` - Paths with spaces, symbols

**Regression Tests** (`testdata/repos/regression/`)
- [ ] Template for issue reproduction: `issue-XXX/`
  - One test per notable bug
  - Link to GitHub issue in test comment
  - Minimal reproducible case

#### Enhanced Verification

- [ ] Implement `verifyRenderedSamples(t, outputDir, goldenPath)`
  - Parse HTML with `golang.org/x/net/html`
  - Verify specific DOM elements (CSS selectors)
  - Check for presence of transformed content

- [ ] Add error case tests
  - Verify error messages and codes
  - Check build reports contain expected issues
  - Validate graceful degradation

**Deliverable**: Comprehensive edge case coverage + regression test framework

---

### Phase 4: CI Integration & Optimization (Week 4)

**Goal**: Fast, reliable tests in CI pipeline

#### Tasks

- [ ] CI configuration
  - Add golden tests to GitHub Actions workflow
  - Run on every PR and merge to main
  - Cache test repositories and Hugo binary
  - Parallel test execution

- [ ] Performance optimization
  - Benchmark test execution time
  - Implement test fixtures sharing (reuse git repos)
  - Add progress reporting for long-running tests
  - Profile and optimize slow helpers

- [ ] Test organization
  - Group tests by category (theme, transform, edge)
  - Use `t.Parallel()` for independent tests
  - Implement test suite cleanup (temp file management)

- [ ] Documentation updates
  - Add CI badge to README
  - Document CI test execution
  - Create troubleshooting guide for test failures

- [ ] Quality gates
  - Require golden test updates in same commit as code changes
  - PR template checklist for golden file updates
  - Document when to create new golden tests

**Deliverable**: CI runs all golden tests in <5 minutes

---

## Implementation Details

### Helper Functions

#### `setupTestRepo(t *testing.T, repoPath string) string`

```go
// Creates a temporary git repository from a directory structure
func setupTestRepo(t *testing.T, repoPath string) string {
    t.Helper()
    
    // Create temp directory
    tmpDir := t.TempDir()
    
    // Copy files from testdata
    err := copyDir(repoPath, tmpDir)
    require.NoError(t, err)
    
    // Initialize git repo
    cmd := exec.Command("git", "init")
    cmd.Dir = tmpDir
    require.NoError(t, cmd.Run())
    
    // Configure git user (required for commit)
    exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()
    exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
    
    // Add all files
    exec.Command("git", "-C", tmpDir, "add", ".").Run()
    
    // Create initial commit
    exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial test commit").Run()
    
    return tmpDir
}
```

#### `verifyHugoConfig(t *testing.T, outputDir, goldenPath string)`

```go
func verifyHugoConfig(t *testing.T, outputDir, goldenPath string) {
    t.Helper()
    
    actualPath := filepath.Join(outputDir, "hugo.yaml")
    
    // Read actual config
    actualData, err := os.ReadFile(actualPath)
    require.NoError(t, err)
    
    var actual map[string]interface{}
    err = yaml.Unmarshal(actualData, &actual)
    require.NoError(t, err)
    
    // Normalize dynamic fields
    delete(actual, "build_date")
    if params, ok := actual["params"].(map[string]interface{}); ok {
        delete(params, "build_date")
    }
    
    if *updateGolden {
        // Write golden file
        data, _ := yaml.Marshal(actual)
        os.WriteFile(goldenPath, data, 0644)
        t.Logf("Updated golden file: %s", goldenPath)
        return
    }
    
    // Read golden config
    goldenData, err := os.ReadFile(goldenPath)
    require.NoError(t, err)
    
    var expected map[string]interface{}
    err = yaml.Unmarshal(goldenData, &expected)
    require.NoError(t, err)
    
    // Deep comparison
    assert.Equal(t, expected, actual, "Hugo config mismatch")
}
```

#### `verifyContentStructure(t *testing.T, outputDir, goldenPath string)`

```go
type ContentStructure struct {
    Files     map[string]ContentFile `json:"files"`
    Structure map[string]interface{} `json:"structure"`
}

type ContentFile struct {
    FrontMatter map[string]interface{} `json:"frontmatter"`
    ContentHash string                 `json:"contentHash"`
}

func verifyContentStructure(t *testing.T, outputDir, goldenPath string) {
    t.Helper()
    
    contentDir := filepath.Join(outputDir, "content")
    
    actual := &ContentStructure{
        Files:     make(map[string]ContentFile),
        Structure: make(map[string]interface{}),
    }
    
    // Walk content directory
    err := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }
        
        if !strings.HasSuffix(path, ".md") {
            return nil
        }
        
        relPath, _ := filepath.Rel(outputDir, path)
        
        // Parse front matter
        data, _ := os.ReadFile(path)
        fm, content := parseFrontMatter(data)
        
        // Hash content (excluding front matter)
        hash := sha256.Sum256(content)
        
        actual.Files[relPath] = ContentFile{
            FrontMatter: fm,
            ContentHash: fmt.Sprintf("sha256:%x", hash[:8]),
        }
        
        return nil
    })
    require.NoError(t, err)
    
    // Build structure tree
    actual.Structure = buildStructureTree(contentDir)
    
    if *updateGolden {
        data, _ := json.MarshalIndent(actual, "", "  ")
        os.WriteFile(goldenPath, data, 0644)
        t.Logf("Updated golden file: %s", goldenPath)
        return
    }
    
    // Compare with golden
    goldenData, err := os.ReadFile(goldenPath)
    require.NoError(t, err)
    
    var expected ContentStructure
    err = json.Unmarshal(goldenData, &expected)
    require.NoError(t, err)
    
    assert.Equal(t, expected, *actual, "Content structure mismatch")
}
```

### Test Flags

```go
var (
    updateGolden = flag.Bool("update-golden", false, "Update golden files")
    skipRender   = flag.Bool("skip-render", false, "Skip Hugo rendering (faster)")
)
```

### CI Workflow Addition

```yaml
# .github/workflows/test.yml (add to existing)
  golden-tests:
    name: Golden Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      
      - name: Install Hugo
        run: |
          wget https://github.com/gohugoio/hugo/releases/download/v0.140.1/hugo_extended_0.140.1_linux-amd64.deb
          sudo dpkg -i hugo_extended_0.140.1_linux-amd64.deb
      
      - name: Run Golden Tests
        run: go test -v ./test/integration -timeout 10m
      
      - name: Check for uncommitted golden files
        run: |
          git diff --exit-code test/testdata/golden/ || \
            (echo "Golden files were updated but not committed!" && exit 1)
```

## Success Criteria

### Phase 1 ✅ COMPLETED
- ✅ One complete golden test passes
- ✅ `-update-golden` flag works
- ✅ Documentation written
- ✅ Test helpers implemented with normalization
- ✅ Golden files generated and verified

### Phase 2
- ✅ 10+ test cases covering major features
- ✅ Both Hextra and Docsy themes tested
- ✅ Content transformations verified

### Phase 3
- ✅ Edge cases handled gracefully
- ✅ Regression test framework established
- ✅ HTML verification implemented

### Phase 4
- ✅ CI runs all tests on every PR
- ✅ Test execution < 5 minutes
- ✅ Zero flaky tests

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Golden files grow too large | Git bloat | Use JSON, not archives; store selectively |
| Tests too slow for local dev | Poor DX | Support `-short` and `-skip-render` flags |
| Hugo version changes break tests | Flaky CI | Pin Hugo version in tests |
| Dynamic content differs per run | False failures | Normalize timestamps, build dates |
| Difficult to debug failures | Slow iteration | Add detailed diff output, save actual files |

## Timeline

- **Week 1** (Phase 1): Foundation + first test ✅ **COMPLETED 2025-12-12**
- **Week 2** (Phase 2): Core coverage (10+ tests) ← NEXT
- **Week 3** (Phase 3): Edge cases + regression framework
- **Week 4** (Phase 4): CI integration + optimization

**Total**: 4 weeks to full implementation

## Phase 1 Completion Summary

**Completed**: 2025-12-12

### Deliverables ✅

1. **Test Framework**
   - `test/integration/helpers.go` - 298 lines, 10+ helper functions
   - `test/integration/golden_test.go` - Complete test implementation
   - `test/integration/README.md` - Comprehensive documentation (200+ lines)

2. **Test Data**
   - `test/testdata/repos/themes/hextra-basic/` - Sample repository with 3 markdown files
   - `test/testdata/configs/hextra-basic.yaml` - Test configuration
   - `test/testdata/golden/hextra-basic/` - 2 golden files (3.2KB total)

3. **Test Results**
   - `TestGolden_HextraBasic` passes consistently
   - Execution time: ~30-40ms
   - Zero flaky failures
   - Golden files update and verify correctly

### Key Features Implemented

- ✅ Temporary Git repository creation from directory structures
- ✅ Full build pipeline execution (git → docs → hugo)
- ✅ Hugo configuration verification with YAML deep comparison
- ✅ Content structure verification with front matter parsing
- ✅ Dynamic field normalization (dates, timestamps, build IDs)
- ✅ `-update-golden` flag support
- ✅ `-short` mode support
- ✅ Comprehensive error messages and diff output

### Files Created/Modified

**New Files:**
- `test/integration/helpers.go`
- `test/integration/golden_test.go`
- `test/integration/README.md`
- `test/testdata/repos/themes/hextra-basic/docs/index.md`
- `test/testdata/repos/themes/hextra-basic/docs/guide.md`
- `test/testdata/repos/themes/hextra-basic/docs/api.md`
- `test/testdata/configs/hextra-basic.yaml`
- `test/testdata/golden/hextra-basic/hugo-config.golden.yaml`
- `test/testdata/golden/hextra-basic/content-structure.golden.json`
- `docs/adr/ADR-001-golden-testing-strategy.md`
- `docs/plan/golden-testing-implementation.md`

**Total**: 11 new files, ~1000 lines of code and documentation

## Next Steps (Phase 2)

**Ready to begin**: 2025-12-13

### Immediate Tasks

1. ~~Create directory structure~~ ✅
2. ~~Implement `setupTestRepo` helper~~ ✅
3. ~~Create first test repo~~ ✅
4. ~~Write `TestGolden_HextraBasic`~~ ✅
5. ~~Generate first golden files~~ ✅
6. ~~Verify test passes~~ ✅
7. **→ Begin Phase 2: Add hextra-math test case**
8. **→ Add frontmatter-injection test case**
9. **→ Add multi-repo test case**
10. **→ Expand to 10+ test cases**

## References

- ADR: [ADR-001: Golden Testing Strategy](../docs/adr/ADR-001-golden-testing-strategy.md)
- Existing pattern: `internal/hugo/generator_golden_test.go`
- Go testing: https://go.dev/wiki/TableDrivenTests
- Golden files: https://ieftimov.com/posts/testing-in-go-golden-files/
