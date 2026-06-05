package pipeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestGenerateCustomHeaderAssets_Disabled(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:                 "Test",
				EnablePageTransitions: false,
			},
		},
	}

	assets, err := generateCustomHeaderAssets(ctx)
	require.NoError(t, err)
	require.Len(t, assets, 1, "should emit only custom-header.html when transitions disabled")
	assert.Equal(t, "layouts/partials/custom-header.html", assets[0].Path)
	content := string(assets[0].Content)
	assert.Contains(t, content, "<style>", "default custom CSS should be inlined as <style>")
	assert.Contains(t, content, "#R-sidebar .nav-title", "default override should reference the nav-title rule")
	assert.Contains(t, content, "docbuilder:template", "header should contain template metadata")
	assert.NotContains(t, content, "view-transitions.css", "view-transitions <link> should be absent when transitions disabled")
}

func TestGenerateCustomHeaderAssets_Enabled(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:                 "Test",
				EnablePageTransitions: true,
			},
		},
	}

	assets, err := generateCustomHeaderAssets(ctx)
	require.NoError(t, err)
	require.Len(t, assets, 2, "should emit custom-header.html and view-transitions.css when transitions enabled")

	var htmlFound, cssFound bool
	for _, a := range assets {
		switch a.Path {
		case "layouts/partials/custom-header.html":
			htmlFound = true
			content := string(a.Content)
			assert.Contains(t, content, "<style>", "default custom CSS should be inlined as <style>")
			assert.Contains(t, content, "view-transitions.css", "view-transitions <link> should be present when transitions enabled")
			assert.Contains(t, content, "docbuilder:template", "header should contain template metadata")
		case "static/view-transitions.css":
			cssFound = true
			assert.Contains(t, string(a.Content), "@view-transition", "CSS should contain View Transitions directive")
		}
	}
	assert.True(t, htmlFound)
	assert.True(t, cssFound)
}

func TestGenerateCustomHeaderAssets_NilConfig(t *testing.T) {
	ctx := &GenerationContext{Config: nil}
	assets, err := generateCustomHeaderAssets(ctx)
	require.NoError(t, err)
	require.Len(t, assets, 1, "nil config still produces the custom-header.html partial")
	assert.Equal(t, "layouts/partials/custom-header.html", assets[0].Path)
	assert.Contains(t, string(assets[0].Content), "<style>", "default custom CSS should be inlined even with nil config")
}

func TestGenerateCustomHeaderAssets_UserOverride(t *testing.T) {
	override := "/* my override */\n.x { color: red; }"
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:     "Test",
				CustomCSS: override,
			},
		},
	}
	assets, err := generateCustomHeaderAssets(ctx)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	content := string(assets[0].Content)
	assert.Contains(t, content, "/* my override */", "user override should appear in the partial")
	assert.Contains(t, content, ".x { color: red; }", "user CSS body should appear in the partial")
	assert.NotContains(t, content, defaultCustomCSS, "default should not be present when user overrides")
}

func TestGenerateCustomHeaderAssets_WhitespaceOnlySuppressesStyle(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:     "Test",
				CustomCSS: "   \n\t  ",
			},
		},
	}
	assets, err := generateCustomHeaderAssets(ctx)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.NotContains(t, string(assets[0].Content), "<style>",
		"whitespace-only custom_css should suppress the <style> block entirely")
	// Template-metadata should still be present.
	assert.Contains(t, string(assets[0].Content), "docbuilder:template")
}

func TestGenerateCustomHeaderAssets_PartialLoadOrder(t *testing.T) {
	// Lock the order: <style> first, view-transitions <link> middle, template metadata last.
	// This keeps the override CSS winning on any specificity ties and
	// keeps the partial readable top-to-bottom.
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:                 "Test",
				EnablePageTransitions: true,
			},
		},
	}
	assets, err := generateCustomHeaderAssets(ctx)
	require.NoError(t, err)
	require.Len(t, assets, 2)
	var content string
	for _, a := range assets {
		if a.Path == "layouts/partials/custom-header.html" {
			content = string(a.Content)
		}
	}
	require.NotEmpty(t, content)

	styleIdx := strings.Index(content, "<style>")
	vtIdx := strings.Index(content, "view-transitions.css")
	assert.True(t, styleIdx >= 0 && (vtIdx < 0 || styleIdx < vtIdx),
		"<style> block should appear before view-transitions link (style=%d, vt=%d)", styleIdx, vtIdx)
	assert.True(t, strings.Contains(content, "docbuilder:template"),
		"template metadata should be present")
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
	require.Len(t, assets, 2, "should generate 2 assets: view-transitions CSS and custom-header.html partial")

	var htmlFound, cssFound bool
	for _, asset := range assets {
		switch asset.Path {
		case "static/view-transitions.css":
			cssFound = true
			assert.NotEmpty(t, asset.Content)
		case "layouts/partials/custom-header.html":
			htmlFound = true
			assert.NotEmpty(t, asset.Content)
			content := string(asset.Content)
			assert.Contains(t, content, "view-transitions", "should contain view transitions link")
			assert.Contains(t, content, "docbuilder:template", "should contain template metadata")
			assert.Contains(t, content, "<style>", "should contain inlined custom CSS")
		}
	}

	assert.True(t, cssFound, "view-transitions CSS asset should be generated")
	assert.True(t, htmlFound, "HTML partial asset should be generated with merged content")
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
	require.Len(t, assets, 1, "should generate 1 asset: custom-header.html partial")

	asset := assets[0]
	assert.Equal(t, "layouts/partials/custom-header.html", asset.Path)
	content := string(asset.Content)
	assert.Contains(t, content, "docbuilder:template", "header should contain template metadata")
	assert.Contains(t, content, "<style>", "header should contain inlined custom CSS")
	assert.NotContains(t, content, "view-transitions", "view-transitions link should not be present when disabled")
}

func TestDefaultStaticAssetGenerators(t *testing.T) {
	generators := defaultStaticAssetGenerators()
	require.Len(t, generators, 1, "should have one default generator: custom-header builder")

	ctx := &GenerationContext{Config: &config.Config{Hugo: config.HugoConfig{EnablePageTransitions: false}}}
	assets, err := generators[0](ctx)
	require.NoError(t, err)
	require.Len(t, assets, 1, "should generate custom-header.html when transitions disabled")
	assert.Equal(t, "layouts/partials/custom-header.html", assets[0].Path)
	assert.Contains(t, string(assets[0].Content), "docbuilder:template", "header should contain template metadata")

	ctx.Config.Hugo.EnablePageTransitions = true
	assets, err = generators[0](ctx)
	require.NoError(t, err)
	require.NotNil(t, assets)
	assert.Len(t, assets, 2, "should generate 2 assets when transitions enabled")
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
