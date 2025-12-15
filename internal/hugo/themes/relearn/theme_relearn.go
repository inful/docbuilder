package relearn

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	th "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
)

type Theme struct{}

func (Theme) Name() config.Theme { return config.ThemeRelearn }

func (Theme) Features() th.Features {
	return th.Features{
		Name:                     config.ThemeRelearn,
		UsesModules:              true,
		ModulePath:               "github.com/McShelby/hugo-theme-relearn",
		ModuleVersion:            "", // Let Hugo fetch latest compatible version
		EnableMathPassthrough:    true,
		EnableOfflineSearchJSON:  true,
		AutoMainMenu:             false, // Relearn builds menu from content structure
		SupportsPerPageEditLinks: true,
		DefaultSearchType:        "lunr",
		ProvidesMermaidSupport:   true,
	}
}

func (Theme) ApplyParams(_ th.ParamContext, params map[string]any) {
	// Search configuration - Relearn v8+ uses search.disable instead of disableSearch
	// Don't set editURL by default - it requires a valid URL pattern
	// Users should configure it in their config if they want edit links

	// Theme variant/color scheme
	if params["themeVariant"] == nil {
		params["themeVariant"] = "auto" // auto, relearn-light, relearn-dark, etc.
	}

	// Disable generator notice in footer
	if params["disableGeneratorVersion"] == nil {
		params["disableGeneratorVersion"] = false
	}

	// Breadcrumb navigation
	if params["disableBreadcrumb"] == nil {
		params["disableBreadcrumb"] = false
	}

	// Show visited checkmarks
	if params["showVisitedLinks"] == nil {
		params["showVisitedLinks"] = true
	}

	// Collapse menu sections
	if params["collapsibleMenu"] == nil {
		params["collapsibleMenu"] = true
	}

	// Always open menu on start
	if params["alwaysopen"] == nil {
		params["alwaysopen"] = false
	}

	// Disable landing page button
	if params["disableLandingPageButton"] == nil {
		params["disableLandingPageButton"] = true
	}

	// Disable shortcuts menu in sidebar
	if params["disableShortcutsTitle"] == nil {
		params["disableShortcutsTitle"] = false
	}

	// Additional functionality configurations
	if params["disableTagHiddenPages"] == nil {
		params["disableTagHiddenPages"] = false
	}

	// Mermaid diagrams support
	if _, ok := params["mermaid"]; !ok {
		params["mermaid"] = map[string]any{
			"enable": true,
		}
	}

	// Math support (using MathJax by default in Relearn)
	if _, ok := params["math"]; !ok {
		params["math"] = map[string]any{
			"enable": true,
		}
	}
}

func (Theme) CustomizeRoot(_ th.ParamContext, _ map[string]any) {}

func init() { th.RegisterTheme(Theme{}) }
