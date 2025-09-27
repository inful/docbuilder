package hugo

// Theme abstraction & registry to allow pluggable theme support without scattering conditionals.
// Initial built-in themes: docsy, hextra. Additional themes register themselves in init().

import (
    "sync"
    "git.home.luguber.info/inful/docbuilder/internal/config"
)

// Theme defines hooks & declared capabilities for a Hugo theme.
type Theme interface {
    Name() config.Theme
    Features() ThemeFeatures
    // ApplyParams mutates params map with theme-specific defaults (called before user overrides).
    ApplyParams(g *Generator, params map[string]any)
    // CustomizeRoot may mutate top-level hugo config map (module imports, outputs, etc.). Optional.
    CustomizeRoot(g *Generator, root map[string]any)
}

var (
    themeRegistryMu sync.RWMutex
    themeRegistry   = map[config.Theme]Theme{}
)

// RegisterTheme registers a Theme implementation. Overwrites are ignored to prevent accidental double registration.
func RegisterTheme(t Theme) {
    if t == nil { return }
    themeRegistryMu.Lock()
    defer themeRegistryMu.Unlock()
    if _, exists := themeRegistry[t.Name()]; exists { return }
    themeRegistry[t.Name()] = t
}

// getRegisteredTheme returns a registered theme or nil.
func getRegisteredTheme(name config.Theme) Theme {
    themeRegistryMu.RLock()
    defer themeRegistryMu.RUnlock()
    return themeRegistry[name]
}

// activeTheme returns the Theme implementation for the generator's configured theme, if any.
func (g *Generator) activeTheme() Theme {
    return getRegisteredTheme(g.config.Hugo.ThemeType())
}

// ---- Built-in themes (implemented using existing param helpers) ----

type docsyTheme struct{}

func (docsyTheme) Name() config.Theme { return config.ThemeDocsy }
func (docsyTheme) Features() ThemeFeatures {
    return ThemeFeatures{
        Name:                    config.ThemeDocsy,
        UsesModules:             true,
        ModulePath:              "github.com/google/docsy",
        EnableMathPassthrough:   false,
        EnableOfflineSearchJSON: true,
        AutoMainMenu:            false,
        SupportsPerPageEditLinks:false,
        DefaultSearchType:       "",
        ProvidesMermaidSupport:  false,
    }
}
func (d docsyTheme) ApplyParams(g *Generator, params map[string]any) { g.addDocsyParams(params) }
func (d docsyTheme) CustomizeRoot(g *Generator, root map[string]any) { /* outputs already handled centrally */ }

type hextraTheme struct{}

func (hextraTheme) Name() config.Theme { return config.ThemeHextra }
func (hextraTheme) Features() ThemeFeatures {
    return ThemeFeatures{
        Name:                    config.ThemeHextra,
        UsesModules:             true,
        ModulePath:              "github.com/imfing/hextra",
        ModuleVersion:           "v0.11.0",
        EnableMathPassthrough:   true,
        EnableOfflineSearchJSON: false,
        AutoMainMenu:            true,
        SupportsPerPageEditLinks:true,
        DefaultSearchType:       "flexsearch",
        ProvidesMermaidSupport:  true,
    }
}
func (h hextraTheme) ApplyParams(g *Generator, params map[string]any) { g.addHextraParams(params) }
func (h hextraTheme) CustomizeRoot(g *Generator, root map[string]any) { /* none */ }

func init() {
    RegisterTheme(docsyTheme{})
    RegisterTheme(hextraTheme{})
}
