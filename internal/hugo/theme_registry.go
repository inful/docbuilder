package hugo

import (
    "sync"
    "git.home.luguber.info/inful/docbuilder/internal/config"
)

// Theme abstraction & registry to allow pluggable theme support without scattering conditionals.
// Built-in themes (docsy, hextra) now live under internal/hugo/themes and register via their own init().

// Theme defines hooks & declared capabilities for a Hugo theme.
type Theme interface {
    Name() config.Theme
    Features() ThemeFeatures
    ApplyParams(g *Generator, params map[string]any)
    CustomizeRoot(g *Generator, root map[string]any)
}

var (
    themeRegistryMu sync.RWMutex
    themeRegistry   = map[config.Theme]Theme{}
)

// RegisterTheme registers a Theme implementation. Duplicate names are ignored.
func RegisterTheme(t Theme) {
    if t == nil { return }
    themeRegistryMu.Lock(); defer themeRegistryMu.Unlock()
    if _, exists := themeRegistry[t.Name()]; exists { return }
    themeRegistry[t.Name()] = t
}

func getRegisteredTheme(name config.Theme) Theme {
    themeRegistryMu.RLock(); defer themeRegistryMu.RUnlock()
    return themeRegistry[name]
}

// activeTheme returns the current theme or nil.
func (g *Generator) activeTheme() Theme { return getRegisteredTheme(g.config.Hugo.ThemeType()) }

