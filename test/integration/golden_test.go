package integration

import (
	"context"
	"flag"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"github.com/stretchr/testify/require"
)

var (
	updateGolden = flag.Bool("update-golden", false, "Update golden files")
	skipRender   = flag.Bool("skip-render", false, "Skip Hugo rendering (faster)")
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

	// Note: Rendered HTML verification can be added in Phase 3
	// if *updateGolden || fileExists(goldenDir+"/rendered-samples.golden.json") {
	//     verifyRenderedSamples(t, outputDir, goldenDir+"/rendered-samples.golden.json", *updateGolden)
	// }
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
