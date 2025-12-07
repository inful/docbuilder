package hextra

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	th "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
	hextraTheme "git.home.luguber.info/inful/docbuilder/internal/hugo/themes/hextra"
	"git.home.luguber.info/inful/docbuilder/internal/plugin"
)

// HextraPlugin wraps the Hextra theme as a plugin.
type HextraPlugin struct {
	plugin.BasePlugin
	theme th.Theme
}

// NewHextraPlugin creates a new Hextra theme plugin.
func NewHextraPlugin() *HextraPlugin {
	return &HextraPlugin{
		theme: hextraTheme.Theme{},
	}
}

// Metadata returns the plugin metadata for Hextra.
func (p *HextraPlugin) Metadata() plugin.PluginMetadata {
	features := p.theme.Features()
	capabilities := []string{}

	if features.EnableMathPassthrough {
		capabilities = append(capabilities, string(plugin.CapabilityMath))
	}
	if features.EnableOfflineSearchJSON {
		capabilities = append(capabilities, string(plugin.CapabilitySearch))
	}
	if features.ProvidesMermaidSupport {
		capabilities = append(capabilities, string(plugin.CapabilityMermaid))
	}

	return plugin.PluginMetadata{
		Name:         "hextra",
		Version:      features.ModuleVersion,
		Type:         plugin.PluginTypeTheme,
		Description:  "Modern, responsive Hugo theme with built-in search and dark mode",
		Author:       "imfing",
		Capabilities: capabilities,
	}
}

// Execute applies the Hextra theme to the Hugo configuration.
func (p *HextraPlugin) Execute(ctx context.Context, pluginCtx *plugin.PluginContext) error {
	// The actual theme application happens during Hugo config generation
	// This is a no-op for theme plugins at execution time
	return nil
}

// ThemeName returns the Hugo theme name.
func (p *HextraPlugin) ThemeName() string {
	return string(config.ThemeHextra)
}

// ModulePath returns the Hugo module path.
func (p *HextraPlugin) ModulePath() string {
	return p.theme.Features().ModulePath
}

// ApplyParams adds Hextra-specific parameters to Hugo configuration.
func (p *HextraPlugin) ApplyParams(params map[string]interface{}) error {
	// Create a minimal ParamContext for the theme
	ctx := &themeParamContext{config: nil} // Config may not be available at plugin execution time
	p.theme.ApplyParams(ctx, params)
	return nil
}

// CustomizeConfig allows Hextra to modify the root Hugo configuration.
func (p *HextraPlugin) CustomizeConfig(hugoConfig map[string]interface{}) error {
	// Create a minimal ParamContext for the theme
	ctx := &themeParamContext{config: nil}
	p.theme.CustomizeRoot(ctx, hugoConfig)
	return nil
}

// themeParamContext is a minimal implementation of theme.ParamContext.
type themeParamContext struct {
	config *config.Config
}

func (t *themeParamContext) Config() *config.Config {
	return t.config
}

// GetTheme returns the underlying theme.Theme implementation.
// This allows existing code to continue using the theme interface.
func (p *HextraPlugin) GetTheme() th.Theme {
	return p.theme
}

func init() {
	// Register the plugin in the global plugin registry
	if err := plugin.Register(NewHextraPlugin()); err != nil {
		// Log error but don't panic during init
		// In production, this should use proper logging
		_ = err
	}
}
