package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/build"
)

func TestIntegration_TemplateMetadataHeadersWithoutTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repoPath := setupTestRepo(t, "../../test/testdata/repos/transforms/template-metadata-headers")
	cfg := loadGoldenConfig(t, "../../test/testdata/configs/template-metadata-headers.yaml")

	require.Len(t, cfg.Repositories, 1, "expected exactly one repository in config")
	cfg.Repositories[0].URL = repoPath
	cfg.Hugo.EnablePageTransitions = false

	outputDir := t.TempDir()
	cfg.Output.Directory = outputDir

	result, err := runBuildPipeline(t, cfg, outputDir)
	require.NoError(t, err, "build pipeline failed")
	require.Equal(t, build.BuildStatusSuccess, result.Status, "build should succeed")

	customHeaderPath := filepath.Join(outputDir, "layouts", "partials", "custom-header.html")
	_, statErr := os.Stat(customHeaderPath)
	require.NoError(t, statErr, "custom header partial should be generated")

	// #nosec G304 -- test reads a deterministic file path under t.TempDir output.
	headerData, readErr := os.ReadFile(customHeaderPath)
	require.NoError(t, readErr, "failed to read generated custom header partial")
	headerContent := string(headerData)
	require.Contains(t, headerContent, `<meta property="docbuilder:template.type" content="{{ $tmpl.type }}">`)
	require.Contains(t, headerContent, `<meta property="docbuilder:template.name" content="{{ $tmpl.name }}">`)
	require.Contains(t, headerContent, `<meta property="docbuilder:template.output_path" content="{{ $tmpl.output_path }}">`)
}
