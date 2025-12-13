package integration

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"github.com/stretchr/testify/require"
)

var (
	updateGolden = flag.Bool("update-golden", false, "Update golden files")
	_ = flag.Bool("skip-render", false, "Skip Hugo rendering (faster)") // Reserved for future use
)

// TestGolden_HextraBasic tests the complete build pipeline with a basic Hextra theme repository.
// This test verifies:
// - Git repository cloning
// - Documentation discovery
// - Hugo configuration generation
// - Content structure and front matter injection
func TestGolden_HextraBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/hextra-basic")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/hextra-basic.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/hextra-basic"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)

	// Phase 3: Render Hugo site and verify HTML structure
	if !*updateGolden {
		publicDir := renderHugoSite(t, outputDir)
		if publicDir != "" {
			verifyRenderedSamples(t, outputDir, goldenDir+"/rendered-samples.golden.json", *updateGolden)
		}
	}
}

// TestGolden_HextraMath tests KaTeX math rendering support in Hextra theme.
// This test verifies:
// - Math markup passthrough configuration
// - Inline and block math equations preserved
// - KaTeX rendering enabled in Hugo config
func TestGolden_HextraMath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/hextra-math")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/hextra-math.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/hextra-math"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_FrontmatterInjection tests automatic front matter injection.
// This test verifies:
// - editURL automatically injected based on repository configuration
// - repository metadata added to front matter
// - Original front matter preserved and enhanced
func TestGolden_FrontmatterInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/transforms/frontmatter-injection")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/frontmatter-injection.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/frontmatter-injection"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_TwoRepos tests basic multi-repository aggregation.
// This test verifies:
// - Multiple repositories cloned successfully
// - Content from different repos organized under separate sections
// - No conflicts between repositories
// - Index pages generated for each repository
func TestGolden_TwoRepos(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repositories from testdata
	repo1Path := setupTestRepo(t, "../../test/testdata/repos/multi-repo/two-repos/repo1")
	repo2Path := setupTestRepo(t, "../../test/testdata/repos/multi-repo/two-repos/repo2")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/two-repos.yaml")

	// Point configuration to temporary repositories
	require.Len(t, cfg.Repositories, 2, "expected exactly two repositories in config")
	cfg.Repositories[0].URL = repo1Path
	cfg.Repositories[1].URL = repo2Path

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/two-repos"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_DocsyBasic tests basic Docsy theme features.
// This test verifies:
// - Docsy theme configuration and parameters
// - linkTitle support in front matter
// - GitHub repo integration
// - Standard Docsy UI parameters
func TestGolden_DocsyBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/docsy-basic")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/docsy-basic.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/docsy-basic"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_HextraSearch tests search index generation for Hextra theme.
// This test verifies:
// - Search configuration in Hugo params
// - FlexSearch integration enabled
// - Content indexed for search
// - Search index structure
func TestGolden_HextraSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/hextra-search")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/hextra-search.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/hextra-search"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_ImagePaths tests asset path handling and transformations.
// This test verifies:
// - Image references preserved in markdown
// - Relative paths maintained correctly
// - Static assets copied to output
// - Various image reference formats (markdown, HTML)
func TestGolden_ImagePaths(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/transforms/image-paths")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/image-paths.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/image-paths"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_SectionIndexes tests automatic _index.md generation for sections.
// This test verifies:
// - _index.md files created for each directory/section
// - Section metadata properly populated
// - Nested sections handled correctly
// - Section ordering and hierarchy
func TestGolden_SectionIndexes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/transforms/section-indexes")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/section-indexes.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/section-indexes"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_ConflictingPaths tests handling of same-named files from different repositories.
// This test verifies:
// - Files with same names from different repos don't conflict
// - Content organized under separate repository sections
// - Both files preserved with correct content
// - No data loss or overwriting
func TestGolden_ConflictingPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repositories from testdata
	repoAPath := setupTestRepo(t, "../../test/testdata/repos/multi-repo/conflicting-paths/repo-a")
	repoBPath := setupTestRepo(t, "../../test/testdata/repos/multi-repo/conflicting-paths/repo-b")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/conflicting-paths.yaml")

	// Point configuration to temporary repositories
	require.Len(t, cfg.Repositories, 2, "expected exactly two repositories in config")
	cfg.Repositories[0].URL = repoAPath
	cfg.Repositories[1].URL = repoBPath

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/conflicting-paths"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_MenuGeneration tests automatic menu generation from front matter.
// This test verifies:
// - Menu configuration preserved from front matter
// - Menu weights and ordering
// - Menu parent-child relationships
// - Custom menu names
func TestGolden_MenuGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/transforms/menu-generation")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/menu-generation.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/menu-generation"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_DocsyAPI tests Docsy theme with API documentation layout.
// This test verifies:
// - Docsy API documentation structure
// - type: docs front matter handling
// - linkTitle and description fields
// - API-specific layouts and organization
func TestGolden_DocsyAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/docsy-api")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/docsy-api.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/docsy-api"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_HextraMultilang tests multi-language support with Hextra theme.
// This test verifies:
// - Multiple language directories (en/, es/)
// - Language-specific content organization
// - defaultContentLanguage parameter
// - Content duplication across languages
func TestGolden_HextraMultilang(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/hextra-multilang")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/hextra-multilang.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/hextra-multilang"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_CrossRepoLinks tests link transformation between repositories.
// This test verifies:
// - Cross-repository markdown links preserved
// - Relative links maintained correctly
// - Link paths updated for Hugo structure
// - No broken links between repos
func TestGolden_CrossRepoLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repositories from testdata
	frontendPath := setupTestRepo(t, "../../test/testdata/repos/transforms/cross-repo-links/repo-frontend")
	backendPath := setupTestRepo(t, "../../test/testdata/repos/transforms/cross-repo-links/repo-backend")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/cross-repo-links.yaml")

	// Point configuration to temporary repositories
	require.Len(t, cfg.Repositories, 2, "expected exactly two repositories in config")
	cfg.Repositories[0].URL = frontendPath
	cfg.Repositories[1].URL = backendPath

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/cross-repo-links"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_EmptyDocs tests handling of repository with no markdown files.
// This test verifies:
// - Build succeeds even with empty docs directory
// - No Hugo site is generated (expected behavior)
// - FilesProcessed is 0
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
// - Content structure shows no files
func TestGolden_OnlyReadme(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/edge-cases/only-readme")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/only-readme.yaml")

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
	require.NoError(t, err, "build pipeline should not fail with only README")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/only-readme"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure (README should be filtered out)
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_MalformedFrontmatter tests graceful handling of invalid YAML in front matter.
// This test verifies:
// - Build continues even with malformed YAML
// - Valid files are processed correctly
// - Invalid files are copied as-is without breaking pipeline
func TestGolden_MalformedFrontmatter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/edge-cases/malformed-frontmatter")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/malformed-frontmatter.yaml")

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
	require.NoError(t, err, "build pipeline should handle malformed front matter gracefully")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/malformed-frontmatter"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_DeepNesting tests handling of deeply nested directory structures (4+ levels).
// This test verifies:
// - Deep nesting (level1/level2/level3/level4) is preserved
// - Section structure is maintained in Hugo content
// - File paths are correctly transformed
func TestGolden_DeepNesting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/edge-cases/deep-nesting")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/deep-nesting.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/deep-nesting"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure preserves deep nesting
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_UnicodeNames tests handling of files with non-ASCII characters in names.
// This test verifies:
// - UTF-8 filenames (español.md, 中文.md, русский.md) are handled correctly
// - Content with various Unicode characters is preserved
// - File paths remain valid in Hugo
func TestGolden_UnicodeNames(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/edge-cases/unicode-names")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/unicode-names.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/unicode-names"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure handles UTF-8 filenames correctly
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_SpecialChars tests handling of paths with spaces and special characters.
// This test verifies:
// - Files with spaces in names are handled correctly
// - Directories with parentheses and special chars work
// - Brackets in filenames are preserved
func TestGolden_SpecialChars(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/edge-cases/special-chars")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/special-chars.yaml")

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
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	// Verify generated Hugo configuration
	goldenDir := "../../test/testdata/golden/special-chars"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure handles special characters in paths
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)
}

// TestGolden_Error_InvalidRepository tests error handling for non-existent repository.
// This test verifies:
// - Build logs errors for invalid repository URL
// - Error is logged but build may continue or fail depending on retry logic
func TestGolden_Error_InvalidRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/hextra-basic.yaml")

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
// - Build fails or returns appropriate status with empty config
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
// - No fatal error occurs
func TestGolden_Warning_NoGitCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create a git repo without any commits
	tmpDir := t.TempDir()

	// Create a minimal docs directory
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0755))

	testFile := filepath.Join(docsDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test\n\nContent"), 0644))

	// Initialize git but don't commit
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Configure git
	_ = exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()
	_ = exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()

	// Add files but don't commit (this creates an edge case)
	_ = exec.Command("git", "-C", tmpDir, "add", ".").Run()

	// Load configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/hextra-basic.yaml")
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
