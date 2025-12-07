package theme

import (
    "git.home.luguber.info/inful/docbuilder/internal/config"
)

// Engine coordinates theme parameter application and final customization.
// It centralizes usage so generators can be simpler and themes remain pluggable.
type Engine struct{}

// ApplyPhases executes the standard theme phases against provided root and params maps.
// Returns the theme Features for downstream decisions (modules, menus, math, search).
// Phases:
//  1) Theme.ApplyParams(ctx, params)
//  2) User params deep-merge (performed by caller)
//  3) Theme.CustomizeRoot(ctx, root)
func (Engine) ApplyPhases(ctx ParamContext, cfg *config.Config, root map[string]any, params map[string]any) Features {
    var feats Features
    if t := Get(cfg.Hugo.ThemeType()); t != nil {
        // theme-provided defaults
        t.ApplyParams(ctx, params)
        // capture features for caller
        feats = t.Features()
        // final root customization
        t.CustomizeRoot(ctx, root)
        return feats
    }
    // unknown theme: safe defaults
    return Features{Name: cfg.Hugo.ThemeType()}
}
