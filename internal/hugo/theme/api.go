package theme

import (
    "strings"
    "sync"
    "git.home.luguber.info/inful/docbuilder/internal/config"
)

// ThemeFeatures describes capability flags & module path for a theme.
type ThemeFeatures struct {
    Name                     config.Theme
    UsesModules              bool
    ModulePath               string
    ModuleVersion            string
    EnableMathPassthrough    bool
    EnableOfflineSearchJSON  bool
    AutoMainMenu             bool
    SupportsPerPageEditLinks bool
    DefaultSearchType        string
    ProvidesMermaidSupport   bool
}

// ParamContext is the minimal surface a theme needs from the generator.
type ParamContext interface { Config() *config.Config }

// Theme provides hooks for configuring Hugo via DocBuilder.
type Theme interface {
    Name() config.Theme
    Features() ThemeFeatures
    ApplyParams(ctx ParamContext, params map[string]any)
    CustomizeRoot(ctx ParamContext, root map[string]any)
}

var (
    regMu sync.RWMutex
    reg   = map[config.Theme]Theme{}
)

// RegisterTheme registers a Theme implementation (idempotent).
func RegisterTheme(t Theme) { if t == nil { return }; regMu.Lock(); if _, ok := reg[t.Name()]; !ok { reg[t.Name()] = t }; regMu.Unlock() }

// Get retrieves a theme by name.
func Get(name config.Theme) Theme { regMu.RLock(); defer regMu.RUnlock(); return reg[name] }

// TitleCase helper (localized to avoid importing hugo package).
func TitleCase(s string) string {
    if s == "" { return s }
    parts := strings.Fields(s)
    for i, p := range parts { if len(p) > 0 { parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:]) } }
    return strings.Join(parts, " ")
}
