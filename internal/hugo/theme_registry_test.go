package hugo

import (
	"testing"
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestRegisteredThemes(t *testing.T) {
	// Extend this slice when new built-in theme enums are added.
	known := []config.Theme{config.ThemeHextra, config.ThemeDocsy}
	for _, th := range known {
		if getRegisteredTheme(th) == nil {
			t.Fatalf("theme %s not registered", th)
		}
	}
}
