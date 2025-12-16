package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestViewTransitionsIntegration tests the end-to-end flow of View Transitions
// from config to generated assets on disk.
func TestViewTransitionsIntegration(t *testing.T) {
	// Create temporary output directory
	outputDir := t.TempDir()

	// Create config with transitions enabled
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:                 "Integration Test Site",
			Description:           "Testing View Transitions integration",
			BaseURL:               "http://localhost:1313",
			EnablePageTransitions: true,
		},
		Output: config.OutputConfig{
			Directory: outputDir,
			Clean:     true,
		},
	}

	// Create generator
	gen := NewGenerator(cfg, outputDir)

	// Generate site (empty docs)
	err := gen.GenerateSite([]docs.DocFile{})
	require.NoError(t, err, "should generate site successfully")

	// Verify CSS asset was created
	cssPath := filepath.Join(outputDir, "static", "view-transitions.css")
	assert.FileExists(t, cssPath, "CSS asset should be written to static directory")

	cssContent, err := os.ReadFile(cssPath)
	require.NoError(t, err)
	assert.Contains(t, string(cssContent), "@view-transition", "CSS should contain View Transitions directives")
	assert.Contains(t, string(cssContent), "::view-transition-old(root)", "CSS should style transition states")

	// Verify HTML partial was created
	partialPath := filepath.Join(outputDir, "layouts", "partials", "custom-header.html")
	assert.FileExists(t, partialPath, "HTML partial should be written to layouts directory")

	htmlContent, err := os.ReadFile(partialPath)
	require.NoError(t, err)
	assert.Contains(t, string(htmlContent), ".Site.Params.enable_transitions", "HTML should check for enable_transitions param")
	assert.Contains(t, string(htmlContent), "/view-transitions.css", "HTML should reference CSS file")

	// Verify Hugo config has enable_transitions param
	hugoConfigPath := filepath.Join(outputDir, "hugo.yaml")
	assert.FileExists(t, hugoConfigPath, "hugo.yaml should be generated")

	hugoContent, err := os.ReadFile(hugoConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(hugoContent), "enable_transitions: true", "Hugo config should enable transitions param")
}

// TestViewTransitionsDisabled tests that assets are NOT generated when disabled
func TestViewTransitionsDisabled(t *testing.T) {
	outputDir := t.TempDir()

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:                 "Test Site No Transitions",
			BaseURL:               "http://localhost:1313",
			EnablePageTransitions: false, // Disabled
		},
		Output: config.OutputConfig{
			Directory: outputDir,
			Clean:     true,
		},
	}

	gen := NewGenerator(cfg, outputDir)
	err := gen.GenerateSite([]docs.DocFile{})
	require.NoError(t, err)

	// Verify assets were NOT created
	cssPath := filepath.Join(outputDir, "static", "view-transitions.css")
	assert.NoFileExists(t, cssPath, "CSS asset should not be created when transitions disabled")

	partialPath := filepath.Join(outputDir, "layouts", "partials", "custom-header.html")
	assert.NoFileExists(t, partialPath, "HTML partial should not be created when transitions disabled")

	// Verify Hugo config does NOT have enable_transitions param
	hugoConfigPath := filepath.Join(outputDir, "hugo.yaml")
	hugoContent, err := os.ReadFile(hugoConfigPath)
	require.NoError(t, err)
	assert.NotContains(t, string(hugoContent), "enable_transitions:", "Hugo config should not have transitions param when disabled")
}

// TestViewTransitionsWithContent tests that transitions work with actual content
func TestViewTransitionsWithContent(t *testing.T) {
	outputDir := t.TempDir()

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:                 "Test Site With Content",
			BaseURL:               "http://localhost:1313",
			EnablePageTransitions: true,
		},
		Output: config.OutputConfig{
			Directory: outputDir,
			Clean:     true,
		},
	}

	// Create sample doc file
	docFiles := []docs.DocFile{
		{
			Repository:   "test-repo",
			Name:         "test.md",
			RelativePath: "test.md",
			DocsBase:     "docs",
			Extension:    ".md",
			Content:      []byte("# Test Document\n\nTest content."),
		},
	}

	gen := NewGenerator(cfg, outputDir)
	err := gen.GenerateSite(docFiles)
	require.NoError(t, err)

	// Assets should still be generated with content
	cssPath := filepath.Join(outputDir, "static", "view-transitions.css")
	assert.FileExists(t, cssPath, "CSS asset should be created with content")

	partialPath := filepath.Join(outputDir, "layouts", "partials", "custom-header.html")
	assert.FileExists(t, partialPath, "HTML partial should be created with content")
}
