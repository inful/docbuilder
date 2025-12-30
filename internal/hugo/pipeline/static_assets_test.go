package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestGenerateViewTransitionsAssets_Disabled(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:                 "Test",
				EnablePageTransitions: false,
			},
		},
	}

	assets, err := generateViewTransitionsAssets(ctx)
	require.NoError(t, err)
	assert.Nil(t, assets, "should not generate assets when transitions disabled")
}

func TestGenerateViewTransitionsAssets_Enabled(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:                 "Test",
				EnablePageTransitions: true,
			},
		},
	}

	assets, err := generateViewTransitionsAssets(ctx)
	require.NoError(t, err)
	require.NotNil(t, assets, "should generate assets when transitions enabled")
	require.Len(t, assets, 2, "should generate 2 assets: CSS and HTML partial")

	// Verify CSS asset
	cssAsset := assets[0]
	assert.Equal(t, "static/view-transitions.css", cssAsset.Path)
	assert.NotEmpty(t, cssAsset.Content, "CSS content should not be empty")
	assert.Contains(t, string(cssAsset.Content), "@view-transition", "CSS should contain View Transitions directive")
	assert.Contains(t, string(cssAsset.Content), "::view-transition-old", "CSS should contain transition pseudo-elements")

	// Verify HTML partial asset
	htmlAsset := assets[1]
	assert.Equal(t, "layouts/partials/custom-header.html", htmlAsset.Path)
	assert.NotEmpty(t, htmlAsset.Content, "HTML content should not be empty")
	assert.Contains(t, string(htmlAsset.Content), ".Site.Params.enable_transitions", "HTML should check for enable_transitions param")
	assert.Contains(t, string(htmlAsset.Content), "/view-transitions.css", "HTML should reference CSS file")
}

func TestGenerateViewTransitionsAssets_NilConfig(t *testing.T) {
	ctx := &GenerationContext{
		Config: nil,
	}

	assets, err := generateViewTransitionsAssets(ctx)
	require.NoError(t, err)
	assert.Nil(t, assets, "should not generate assets when config is nil")
}

func TestGenerateStaticAssets_NoGenerators(t *testing.T) {
	processor := &Processor{
		config:                &config.Config{},
		staticAssetGenerators: []StaticAssetGenerator{},
	}

	assets, err := processor.GenerateStaticAssets()
	require.NoError(t, err)
	assert.Empty(t, assets, "should return empty assets when no generators registered")
}

func TestGenerateStaticAssets_WithTransitions(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:                 "Test Site",
			EnablePageTransitions: true,
		},
	}

	processor := NewProcessor(cfg)

	assets, err := processor.GenerateStaticAssets()
	require.NoError(t, err)
	require.Len(t, assets, 2, "should generate 2 assets when transitions enabled")

	// Verify assets structure
	var cssFound, htmlFound bool
	for _, asset := range assets {
		if asset.Path == "static/view-transitions.css" {
			cssFound = true
			assert.NotEmpty(t, asset.Content)
		}
		if asset.Path == "layouts/partials/custom-header.html" {
			htmlFound = true
			assert.NotEmpty(t, asset.Content)
		}
	}

	assert.True(t, cssFound, "CSS asset should be generated")
	assert.True(t, htmlFound, "HTML partial asset should be generated")
}

func TestGenerateStaticAssets_WithoutTransitions(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:                 "Test Site",
			EnablePageTransitions: false,
		},
	}

	processor := NewProcessor(cfg)

	assets, err := processor.GenerateStaticAssets()
	require.NoError(t, err)
	assert.Empty(t, assets, "should not generate assets when transitions disabled")
}

func TestDefaultStaticAssetGenerators(t *testing.T) {
	generators := defaultStaticAssetGenerators()
	require.Len(t, generators, 1, "should have exactly one default generator")

	// Test the generator with enabled transitions
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				EnablePageTransitions: true,
			},
		},
	}

	assets, err := generators[0](ctx)
	require.NoError(t, err)
	require.NotNil(t, assets)
	assert.Len(t, assets, 2, "default generator should produce 2 assets when enabled")
}

func TestStaticAssetContent(t *testing.T) {
	// Verify embedded assets are valid
	assert.NotEmpty(t, viewTransitionsCSS, "CSS asset should be embedded")
	assert.NotEmpty(t, viewTransitionsHeadPartial, "HTML partial should be embedded")

	// Verify CSS contains required View Transitions API directives
	cssContent := string(viewTransitionsCSS)
	assert.Contains(t, cssContent, "@view-transition", "CSS should define view-transition")
	assert.Contains(t, cssContent, "::view-transition-old(root)", "CSS should style old root")
	assert.Contains(t, cssContent, "::view-transition-new(root)", "CSS should style new root")
	assert.Contains(t, cssContent, "@keyframes", "CSS should define animations")

	// Verify HTML partial has correct Hugo template syntax
	htmlContent := string(viewTransitionsHeadPartial)
	assert.Contains(t, htmlContent, "{{-", "HTML should use Hugo template delimiters")
	assert.Contains(t, htmlContent, "-}}", "HTML should close Hugo template delimiters")
	assert.Contains(t, htmlContent, "if .Site.Params.enable_transitions", "HTML should conditionally load based on param")
	assert.Contains(t, htmlContent, "<link rel=\"stylesheet\"", "HTML should include stylesheet link")
}
