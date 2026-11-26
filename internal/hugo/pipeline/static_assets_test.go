package pipeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// assetPaths returns the paths of the generated assets.
func assetPaths(assets []*StaticAsset) []string {
	paths := make([]string, 0, len(assets))
	for _, a := range assets {
		paths = append(paths, a.Path)
	}
	return paths
}

// assetContent returns the content of the asset at the given path.
func assetContent(t *testing.T, assets []*StaticAsset, path string) string {
	t.Helper()
	for _, a := range assets {
		if a.Path == path {
			return string(a.Content)
		}
	}
	t.Fatalf("asset %s not found in %v", path, assetPaths(assets))
	return ""
}

func TestGenerateThemeAssets_Disabled(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:                 "Test",
				EnablePageTransitions: false,
			},
		},
	}

	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)

	paths := assetPaths(assets)
	assert.Contains(t, paths, "layouts/_partials/hooks/head-end.html")
	assert.Contains(t, paths, "layouts/_partials/head-css.html")
	assert.Contains(t, paths, docsyPageMetaLinksPath)
	assert.Contains(t, paths, "assets/css/docsy.css")
	assert.Contains(t, paths, "assets/vendor/bootstrap/dist/js/bootstrap.bundle.js")
	assert.NotContains(t, paths, "assets/css/view-transitions.css",
		"view-transitions stylesheet should be absent when transitions are disabled")

	content := assetContent(t, assets, "layouts/_partials/hooks/head-end.html")
	assert.Contains(t, content, "docbuilder:template", "hook should contain template metadata")
	assert.NotContains(t, content, "view-transitions.css", "view-transitions <link> should be absent when transitions disabled")
}

func TestGenerateThemeAssets_Enabled(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:                 "Test",
				EnablePageTransitions: true,
			},
		},
	}

	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)

	content := assetContent(t, assets, "layouts/_partials/hooks/head-end.html")
	assert.Contains(t, content, "view-transitions.css", "view-transitions <link> should be present when transitions enabled")
	assert.Contains(t, content, "docbuilder:template", "hook should contain template metadata")
	assert.Contains(t, assetContent(t, assets, "assets/css/view-transitions.css"), "@view-transition")
}

func TestGenerateThemeAssets_NilConfig(t *testing.T) {
	ctx := &GenerationContext{Config: nil}
	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)
	assert.Contains(t, assetPaths(assets), "layouts/_partials/hooks/head-end.html")
}

func TestGenerateThemeAssets_UserOverride(t *testing.T) {
	override := "/* my override */\n.x { color: red; }"
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:     "Test",
				CustomCSS: override,
			},
		},
	}
	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)

	content := assetContent(t, assets, "layouts/_partials/hooks/head-end.html")
	assert.Contains(t, content, "/* my override */", "user override should appear in the hook")
	assert.Contains(t, content, ".x { color: red; }", "user CSS body should appear in the hook")
}

func TestGenerateThemeAssets_WhitespaceOnlySuppressesStyle(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:     "Test",
				CustomCSS: "   \n\t  ",
			},
		},
	}
	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)

	content := assetContent(t, assets, "layouts/_partials/hooks/head-end.html")
	assert.NotContains(t, content, "<style>",
		"whitespace-only custom_css should suppress the <style> block entirely")
	// Template-metadata should still be present.
	assert.Contains(t, content, "docbuilder:template")
}

func TestGenerateThemeAssets_PartialLoadOrder(t *testing.T) {
	// Lock the order: <style> first, view-transitions <link> middle, template metadata last.
	// This keeps the override CSS winning on any specificity ties and
	// keeps the partial readable top-to-bottom.
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{
				Title:                 "Test",
				CustomCSS:             "/* override */",
				EnablePageTransitions: true,
			},
		},
	}
	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)

	content := assetContent(t, assets, "layouts/_partials/hooks/head-end.html")
	styleIdx := strings.Index(content, "<style>")
	vtIdx := strings.Index(content, "view-transitions.css")
	assert.True(t, styleIdx >= 0 && (vtIdx < 0 || styleIdx < vtIdx),
		"<style> block should appear before view-transitions link (style=%d, vt=%d)", styleIdx, vtIdx)
	assert.True(t, strings.Contains(content, "docbuilder:template"),
		"template metadata should be present")
}

func TestGenerateThemeAssets_PermalinkCopyScript(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo: config.HugoConfig{Title: "Test"},
		},
	}

	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)

	// The script of the copy buttons of the permalink is published,
	// and the head hook loads it.
	assert.Contains(t, assetPaths(assets), "assets/js/permalink-copy.js")
	script := assetContent(t, assets, "assets/js/permalink-copy.js")
	assert.Contains(t, script, "data-docbuilder-copy",
		"the script should bind the copy buttons of the permalink")

	hook := assetContent(t, assets, "layouts/_partials/hooks/head-end.html")
	assert.Contains(t, hook, "js/permalink-copy.js",
		"the head hook should load the script of the copy buttons")
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

	paths := assetPaths(assets)
	assert.Contains(t, paths, "assets/css/view-transitions.css")
	assert.Contains(t, paths, "layouts/_partials/hooks/head-end.html")

	content := assetContent(t, assets, "layouts/_partials/hooks/head-end.html")
	assert.Contains(t, content, "view-transitions", "should contain view transitions link")
	assert.Contains(t, content, "docbuilder:template", "should contain template metadata")
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
	assert.NotContains(t, assetPaths(assets), "assets/css/view-transitions.css")

	content := assetContent(t, assets, "layouts/_partials/hooks/head-end.html")
	assert.Contains(t, content, "docbuilder:template", "hook should contain template metadata")
	assert.NotContains(t, content, "view-transitions", "view-transitions link should not be present when disabled")
}

func TestDefaultStaticAssetGenerators(t *testing.T) {
	generators := defaultStaticAssetGenerators()
	require.Len(t, generators, 1, "should have one default generator: the theme asset builder")

	ctx := &GenerationContext{Config: &config.Config{Hugo: config.HugoConfig{EnablePageTransitions: false}}}
	assets, err := generators[0](ctx)
	require.NoError(t, err)
	assert.Contains(t, assetPaths(assets), "layouts/_partials/hooks/head-end.html")
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
