package hugo

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestRegisteredThemesOrFallback(t *testing.T) {
	// During modularization we allow fallback (unregistered) themes as long as deriveThemeFeatures still returns the expected module path.
	cases := []struct{ th config.Theme; expectPath string }{
		{config.ThemeHextra, "github.com/imfing/hextra"},
		{config.ThemeDocsy, "github.com/google/docsy"},
	}
	g := NewGenerator(&config.Config{Hugo: config.HugoConfig{Theme: string(config.ThemeHextra)}}, t.TempDir())
	for _, c := range cases {
		g.config.Hugo.Theme = string(c.th)
		g.cachedThemeFeatures = nil // reset cache to force recompute
		if got := getRegisteredTheme(c.th); got == nil {
			feats := g.deriveThemeFeatures()
			if feats.ModulePath != c.expectPath {
				t.Fatalf("theme %s fallback module path = %q want %q", c.th, feats.ModulePath, c.expectPath)
			}
		}
	}
}
