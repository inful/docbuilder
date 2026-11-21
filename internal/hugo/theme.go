package hugo

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// applyDocsyThemeDefaults applies the theme's parameter defaults.
//
// Most of the theme's configuration comes from its own config file,
// which Hugo merges into the site configuration: the KaTeX version,
// the plugin versions and the output formats all arrive that way.
// Only the site-level opt-ins are set here.
func (g *Generator) applyDocsyThemeDefaults(params map[string]any) {
	// The search: the theme ships the index, the site opts in.
	if params["offlineSearch"] == nil {
		params["offlineSearch"] = true
	}

	// The light/dark menu and the foldable sidebar: a variant switch
	// in the navbar and menu sections that fold.
	ui, _ := params["ui"].(map[string]any)
	if ui == nil {
		ui = map[string]any{}
		params["ui"] = ui
	}
	if ui["showLightDarkModeMenu"] == nil {
		ui["showLightDarkModeMenu"] = true
	}
	if ui["sidebar_menu_foldable"] == nil {
		ui["sidebar_menu_foldable"] = true
	}

	// The View Transitions API: our own stylesheet and link, read by
	// the head hook.
	if g.config.Hugo.EnablePageTransitions {
		params["enable_transitions"] = true
	}

	// The navigation chrome: "full" (the default) renders it on
	// every page, "shared" renders it on the donor page only and
	// restores it in the browser. The theme's shared mode is
	// experimental, so it is opt-in.
	if g.config.Hugo.Chrome == config.ChromeShared {
		td, _ := params["td"].(map[string]any)
		if td == nil {
			td = map[string]any{}
			params["td"] = td
		}
		if td["chrome"] == nil {
			td["chrome"] = config.ChromeShared
		}
	}
}
