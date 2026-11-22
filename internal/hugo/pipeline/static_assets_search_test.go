package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// With the server-side search the site must ship our own search client
// and an empty index: the theme's index of the whole site would
// otherwise be downloaded by every visitor.
func TestGenerateThemeAssets_ServerSideSearch(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo:   config.HugoConfig{Title: "Test"},
			Search: &config.SearchConfig{Provider: config.SearchProviderServer},
		},
	}

	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)

	paths := assetPaths(assets)
	assert.Contains(t, paths, "assets/js/offline-search.js")
	assert.Contains(t, paths, "assets/json/offline-search-index.json")

	script := assetContent(t, assets, "assets/js/offline-search.js")
	assert.Contains(t, script, "/api/search", "the client must ask the server")

	index := assetContent(t, assets, "assets/json/offline-search-index.json")
	assert.Equal(t, "[]", index, "the index must be empty: nobody downloads it")
}

// A site that is served as a static tree keeps the theme's own search:
// the index travels with the site.
func TestGenerateThemeAssets_BrowserSideSearch(t *testing.T) {
	ctx := &GenerationContext{
		Config: &config.Config{
			Hugo:   config.HugoConfig{Title: "Test"},
			Search: &config.SearchConfig{Provider: config.SearchProviderLunr},
		},
	}

	assets, err := generateThemeAssets(ctx)
	require.NoError(t, err)

	paths := assetPaths(assets)
	assert.NotContains(t, paths, "assets/js/offline-search.js")
	assert.NotContains(t, paths, "assets/json/offline-search-index.json")
}
