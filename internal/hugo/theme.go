package hugo

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/pipeline"
)

// themeModuleImport returns the Hugo module import for the theme the
// build targets.
//
// The Docsy theme lives in the "theme" subdirectory of the Docsy
// repository and is its own Go module, so the import path carries the
// subdirectory and the version the "theme/vX.Y.Z" tag.
func themeModuleImport(theme string) models.ModuleImport {
	if theme == config.ThemeDocsy {
		return models.ModuleImport{Path: pipeline.DocsyModulePath, Version: pipeline.DocsyVersion}
	}
	return models.ModuleImport{Path: "github.com/McShelby/hugo-theme-relearn", Version: relearnVersion}
}

// applyDocsyThemeDefaults applies the Docsy-specific parameter
// defaults.
//
// Most of Docsy's configuration comes from the theme's own config
// file, which Hugo merges into the site configuration: the KaTeX
// version, the plugin versions and the output formats all arrive that
// way. Only the site-level opt-ins are set here.
func (g *Generator) applyDocsyThemeDefaults(params map[string]any) {
	// Docsy's search: the theme ships the index, the site opts in.
	// Relearn's Lunr search is built into the theme itself.
	if params["offlineSearch"] == nil {
		params["offlineSearch"] = true
	}
}
