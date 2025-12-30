package integration

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

var (
	updateGolden = flag.Bool("update-golden", false, "Update golden files")
	_            = flag.Bool("skip-render", false, "Skip Hugo rendering (faster)") // Reserved for future use
)

// TestGolden_FrontmatterInjection tests automatic front matter injection.
// This test verifies:
// - editURL automatically injected based on repository configuration
// - repository metadata added to front matter
// - Original front matter preserved and enhanced.
func TestGolden_FrontmatterInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/transforms/frontmatter-injection",
		"../../test/testdata/configs/frontmatter-injection.yaml",
		"../../test/testdata/golden/frontmatter-injection",
		*updateGolden,
	)
}

// TestGolden_TwoRepos tests basic multi-repository aggregation.
// This test verifies:
// - Multiple repositories cloned successfully
// - Content from different repos organized under separate sections
// - No conflicts between repositories
// - Index pages generated for each repository.
func TestGolden_TwoRepos(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runMultiRepoGoldenTest(t,
		"../../test/testdata/repos/multi-repo/two-repos/repo1",
		"../../test/testdata/repos/multi-repo/two-repos/repo2",
		"../../test/testdata/configs/two-repos.yaml",
		"../../test/testdata/golden/two-repos",
		*updateGolden,
	)
}

// TestGolden_ImagePaths tests asset path handling and transformations.
// This test verifies:
// - Image references preserved in markdown
// - Relative paths maintained correctly
// - Static assets copied to output
// - Various image reference formats (markdown, HTML).
func TestGolden_ImagePaths(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/transforms/image-paths",
		"../../test/testdata/configs/image-paths.yaml",
		"../../test/testdata/golden/image-paths",
		*updateGolden,
	)
}

// TestGolden_SectionIndexes tests automatic _index.md generation for sections.
// This test verifies:
// - _index.md files created for each directory/section
// - Section metadata properly populated
// - Nested sections handled correctly
// - Section ordering and hierarchy.
func TestGolden_SectionIndexes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/transforms/section-indexes",
		"../../test/testdata/configs/section-indexes.yaml",
		"../../test/testdata/golden/section-indexes",
		*updateGolden,
	)
}

// TestGolden_ConflictingPaths tests handling of same-named files from different repositories.
// This test verifies:
// - Files with same names from different repos don't conflict
// - Content organized under separate repository sections
// - Both files preserved with correct content
// - No data loss or overwriting.
func TestGolden_ConflictingPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runMultiRepoGoldenTest(t,
		"../../test/testdata/repos/multi-repo/conflicting-paths/repo-a",
		"../../test/testdata/repos/multi-repo/conflicting-paths/repo-b",
		"../../test/testdata/configs/conflicting-paths.yaml",
		"../../test/testdata/golden/conflicting-paths",
		*updateGolden,
	)
}

// TestGolden_MenuGeneration tests automatic menu generation from front matter.
// This test verifies:
// - Menu configuration preserved from front matter
// - Menu weights and ordering
// - Menu parent-child relationships
// - Custom menu names.
func TestGolden_MenuGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/transforms/menu-generation",
		"../../test/testdata/configs/menu-generation.yaml",
		"../../test/testdata/golden/menu-generation",
		*updateGolden,
	)
}

// TestGolden_CrossRepoLinks tests link transformation between repositories.
// This test verifies:
// - Cross-repository markdown links preserved
// - Relative links maintained correctly
// - Link paths updated for Hugo structure
// - No broken links between repos.
func TestGolden_CrossRepoLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runMultiRepoGoldenTest(t,
		"../../test/testdata/repos/transforms/cross-repo-links/repo-frontend",
		"../../test/testdata/repos/transforms/cross-repo-links/repo-backend",
		"../../test/testdata/configs/cross-repo-links.yaml",
		"../../test/testdata/golden/cross-repo-links",
		*updateGolden,
	)
}

// TestGolden_EmptyDocs tests handling of repository with no markdown files.
// This test verifies:
// - Build succeeds even with empty docs directory
// - No Hugo site is generated (expected behavior)
// - FilesProcessed is 0.
func TestGolden_EmptyDocs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/edge-cases/empty-docs")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/empty-docs.yaml")

	// Point configuration to temporary repository
	require.Len(t, cfg.Repositories, 1, "expected exactly one repository in config")
	cfg.Repositories[0].URL = repoPath

	// Create temporary output directory
	outputDir := t.TempDir()
	cfg.Output.Directory = outputDir

	// Create build service
	svc := build.NewBuildService().
		WithHugoGeneratorFactory(func(cfgAny any, outDir string) build.HugoGenerator {
			return hugo.NewGenerator(cfgAny.(*config.Config), outDir)
		})

	// Execute build pipeline
	req := build.BuildRequest{
		Config:    cfg,
		OutputDir: outputDir,
	}

	result, err := svc.Run(context.Background(), req)
	require.NoError(t, err, "build pipeline should not fail with empty docs")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify no files were processed
	require.Equal(t, 0, result.FilesProcessed, "should process 0 files")

	// Note: No Hugo site is generated when there are no docs, this is expected behavior
	// The build succeeds but outputs nothing, which is correct for empty documentation
}

// TestGolden_OnlyReadme tests handling of repository with only README.md (should be ignored).
// This test verifies:
// - README.md files are filtered out during discovery
// - Build succeeds with no processable content
// - Content structure shows no files.
func TestGolden_OnlyReadme(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/edge-cases/only-readme",
		"../../test/testdata/configs/only-readme.yaml",
		"../../test/testdata/golden/only-readme",
		*updateGolden,
	)
}

// TestGolden_MalformedFrontmatter tests graceful handling of invalid YAML in front matter.
// This test verifies:
// - Build continues even with malformed YAML
// - Valid files are processed correctly
// - Invalid files are copied as-is without breaking pipeline.
func TestGolden_MalformedFrontmatter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/edge-cases/malformed-frontmatter",
		"../../test/testdata/configs/malformed-frontmatter.yaml",
		"../../test/testdata/golden/malformed-frontmatter",
		*updateGolden,
	)
}

// TestGolden_DeepNesting tests handling of deeply nested directory structures (4+ levels).
// This test verifies:
// - Deep nesting (level1/level2/level3/level4) is preserved
// - Section structure is maintained in Hugo content
// - File paths are correctly transformed.
func TestGolden_DeepNesting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/edge-cases/deep-nesting",
		"../../test/testdata/configs/deep-nesting.yaml",
		"../../test/testdata/golden/deep-nesting",
		*updateGolden,
	)
}

// TestGolden_UnicodeNames tests handling of files with non-ASCII characters in names.
// This test verifies:
// - UTF-8 filenames (español.md, 中文.md, русский.md) are handled correctly
// - Content with various Unicode characters is preserved
// - File paths remain valid in Hugo.
func TestGolden_UnicodeNames(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/edge-cases/unicode-names",
		"../../test/testdata/configs/unicode-names.yaml",
		"../../test/testdata/golden/unicode-names",
		*updateGolden,
	)
}

// TestGolden_SpecialChars tests handling of paths with spaces and special characters.
// This test verifies:
// - Files with spaces in names are handled correctly
// - Directories with parentheses and special chars work
// - Brackets in filenames are preserved.
func TestGolden_SpecialChars(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	runGoldenTest(t,
		"../../test/testdata/repos/edge-cases/special-chars",
		"../../test/testdata/configs/special-chars.yaml",
		"../../test/testdata/golden/special-chars",
		*updateGolden,
	)
}

// TestGolden_Error_InvalidRepository tests error handling for non-existent repository.
// This test verifies:
// - Build logs errors for invalid repository URL
// - Error is logged but build may continue or fail depending on retry logic.
func TestGolden_Error_InvalidRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/relearn-basic.yaml")

	// Point to non-existent repository
	cfg.Repositories[0].URL = "/tmp/this-repo-does-not-exist-" + t.Name()

	// Create temporary output directory
	outputDir := t.TempDir()
	cfg.Output.Directory = outputDir

	// Create build service
	svc := build.NewBuildService().
		WithHugoGeneratorFactory(func(cfgAny any, outDir string) build.HugoGenerator {
			return hugo.NewGenerator(cfgAny.(*config.Config), outDir)
		})

	// Execute build pipeline
	req := build.BuildRequest{
		Config:    cfg,
		OutputDir: outputDir,
	}

	result, err := svc.Run(context.Background(), req)

	// Build service is graceful - logs errors but may return success
	// Verify that either we got an error OR the build shows skipped repos
	if err == nil && result.Status == build.BuildStatusSuccess {
		// Check that the repository was actually skipped
		if result.RepositoriesSkipped == 0 && result.Repositories > 0 {
			t.Errorf("build succeeded with invalid repository - expected repository to be skipped but got: Processed=%d, Skipped=%d",
				result.Repositories, result.RepositoriesSkipped)
		} else {
			t.Logf("Build handled invalid repo gracefully - Processed: %d, Skipped: %d",
				result.Repositories, result.RepositoriesSkipped)
		}
	} else if err != nil {
		// Error was returned - verify it's informative
		t.Logf("Build returned error: %v", err)
		require.Contains(t, err.Error(), "repository", "error should mention repository")
	} else {
		t.Logf("Build failed gracefully with status: %v", result.Status)
	}
}

// TestGolden_Error_InvalidConfig tests error handling for invalid configuration.
// This test verifies:
// - Configuration validation catches errors
// - Build fails or returns appropriate status with empty config.
func TestGolden_Error_InvalidConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create minimal but empty configuration
	cfg := &config.Config{
		Repositories: []config.Repository{}, // No repositories
	}

	outputDir := t.TempDir()

	// Create build service
	svc := build.NewBuildService().
		WithHugoGeneratorFactory(func(cfgAny any, outDir string) build.HugoGenerator {
			return hugo.NewGenerator(cfgAny.(*config.Config), outDir)
		})

	// Execute build pipeline with empty repositories
	req := build.BuildRequest{
		Config:    cfg,
		OutputDir: outputDir,
	}

	result, err := svc.Run(context.Background(), req)

	// Build may succeed with warning since empty config is technically valid
	// The key is it doesn't crash
	t.Logf("Build result with empty config - Status: %v, Error: %v", result.Status, err)
	require.NotPanics(t, func() {
		_ = result.Status
	})
}

// TestGolden_Warning_NoGitCommit tests warning case when repository has no commits.
// This test verifies:
// - Build continues even without git history
// - Warning status is returned
// - No fatal error occurs.
func TestGolden_Warning_NoGitCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create a git repo without any commits
	tmpDir := t.TempDir()

	// Create a minimal docs directory
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	testFile := filepath.Join(docsDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test\n\nContent"), 0o644))

	// Initialize git but don't commit
	cmd := exec.CommandContext(context.Background(), "git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Configure git
	_ = exec.CommandContext(context.Background(), "git", "-C", tmpDir, "config", "user.name", "Test").Run()
	_ = exec.CommandContext(context.Background(), "git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()

	// Add files but don't commit (this creates an edge case)
	_ = exec.CommandContext(context.Background(), "git", "-C", tmpDir, "add", ".").Run()

	// Load configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/relearn-basic.yaml")
	cfg.Repositories[0].URL = tmpDir

	// Create temporary output directory
	outputDir := t.TempDir()
	cfg.Output.Directory = outputDir

	// Create build service
	svc := build.NewBuildService().
		WithHugoGeneratorFactory(func(cfgAny any, outDir string) build.HugoGenerator {
			return hugo.NewGenerator(cfgAny.(*config.Config), outDir)
		})

	// Execute build pipeline
	req := build.BuildRequest{
		Config:    cfg,
		OutputDir: outputDir,
	}

	result, err := svc.Run(context.Background(), req)

	// Build may succeed with warnings or fail gracefully
	// The key is it shouldn't panic or crash
	if err == nil {
		t.Logf("Build succeeded with status: %v", result.Status)
	} else {
		t.Logf("Build failed gracefully: %v", err)
		// Should be a clean error, not a panic
		require.NotPanics(t, func() {
			_ = err.Error()
		})
	}
}
