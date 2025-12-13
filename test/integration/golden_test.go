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
