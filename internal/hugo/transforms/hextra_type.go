package transforms

import (
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

// HextraTypeEnforcer ensures type: docs is set for Hextra theme pages
// This runs late (priority 95) to override any type values from repository tags/metadata
type HextraTypeEnforcer struct{}

func (t HextraTypeEnforcer) Name() string { return "hextra_type_enforcer" }

func (t HextraTypeEnforcer) Priority() int {
	// Run late (after defaults at 50, edit links at 60, shortcode escaper at 85)
	// but before serializer (90)
	return 89
}

func (t HextraTypeEnforcer) Transform(p PageAdapter) error {
	pg, ok := p.(*PageShim)
	if !ok {
		return fmt.Errorf("hextra_type_enforcer: unexpected page adapter type")
	}

	// Get config from generator provider
	var cfg *config.Config
	if generatorProvider != nil {
		if g, ok2 := generatorProvider().(interface{ Config() *config.Config }); ok2 {
			cfg = g.Config()
		}
	}

	if cfg == nil {
		return nil
	}

	// Only apply for Hextra theme
	if cfg.Hugo.ThemeType() != config.ThemeHextra {
		return nil
	}

	// Force type: docs for Hextra (replace any existing type value)
	// Use very high patch priority to override all other patches
	pg.AddPatch(fmcore.FrontMatterPatch{
		Source:   "hextra_type_enforcer",
		Mode:     fmcore.MergeReplace,
		Priority: 999,
		Data:     map[string]any{"type": "docs"},
	})

	return nil
}

func init() {
	Register(HextraTypeEnforcer{})
}
