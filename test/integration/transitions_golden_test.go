package integration

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"github.com/stretchr/testify/require"
)

// TestGolden_HextraTransitions tests View Transitions API integration with Hextra theme.
// This test verifies:
// - View Transitions enabled in Hugo configuration
// - Transition assets (CSS/JS) copied to static directory
// - Transitions partial included in layouts
// - Configurable transition duration parameter
func TestGolden_HextraTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden test in short mode")
	}

	// Create temporary git repository from testdata
	repoPath := setupTestRepo(t, "../../test/testdata/repos/themes/hextra-transitions")

	// Load test configuration
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/hextra-transitions.yaml")

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
	goldenDir := "../../test/testdata/golden/hextra-transitions"
	verifyHugoConfig(t, outputDir, goldenDir+"/hugo-config.golden.yaml", *updateGolden)

	// Verify content structure and front matter
	verifyContentStructure(t, outputDir, goldenDir+"/content-structure.golden.json", *updateGolden)

	// Verify View Transitions assets are present
	verifyTransitionsAssets(t, outputDir, goldenDir+"/transitions-assets.golden.json", *updateGolden)
}
