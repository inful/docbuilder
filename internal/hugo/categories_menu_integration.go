package hugo

import (
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// ComputeCategoriesMenu is the public entry point used by the
// categories-menu pipeline stage.
func (g *Generator) ComputeCategoriesMenu(bs *models.BuildState) (*models.CategoriesMenu, error) {
	return g.computeCategoriesMenu(bs)
}

// AttachCategoriesMenu stores the result of the categories-menu
// stage so the config writer can include it in hugo.yaml.
func (g *Generator) AttachCategoriesMenu(cm *models.CategoriesMenu) {
	g.attachCategoriesMenu(cm)
}

// ApplyCategoriesMenuToConfig merges the categories-menu result into
// the typed RootConfig that the config writer marshals to YAML. It is
// a no-op when the categories menu has not been built.
//
// The merge is additive: the categories menu is added alongside any
// user-supplied menu entries (e.g., the user-defined `shortcuts` menu
// is preserved), and a default main page menu is appended after the
// category menus when hugo.sidebar.keep_default_menu is true.
func (g *Generator) ApplyCategoriesMenuToConfig(root *models.RootConfig) {
	if g.categoriesMenu == nil || !g.categoriesMenu.Built {
		return
	}
	if root.Menu == nil {
		root.Menu = map[string]any{}
	}
	for cat, entries := range g.categoriesMenu.Menus {
		existing, _ := root.Menu[cat].([]any)
		merged := make([]any, 0, len(existing)+len(entries))
		merged = append(merged, existing...)
		for i := range entries {
			entry := entries[i]
			m := map[string]any{
				"name": entry.Name,
			}
			if entry.Identifier != "" {
				m["identifier"] = entry.Identifier
			}
			if entry.URL != "" {
				m["url"] = entry.URL
			}
			if entry.PageRef != "" {
				m["pageRef"] = entry.PageRef
			}
			if entry.Parent != "" {
				m["parent"] = entry.Parent
			}
			if entry.Weight != 0 {
				m["weight"] = entry.Weight
			}
			if entry.Title != "" {
				m["title"] = entry.Title
			}
			merged = append(merged, m)
		}
		root.Menu[cat] = merged
	}
	// Wire sidebarmenus under params.
	if root.Params == nil {
		root.Params = map[string]any{}
	}
	existing, _ := root.Params["sidebarmenus"].([]any)
	out := make([]any, 0, len(existing)+len(g.categoriesMenu.SidebarEntries)+2)
	for _, e := range g.categoriesMenu.SidebarEntries {
		block := map[string]any{
			"identifier":   e.Identifier,
			"type":         e.Type,
			"disableTitle": e.DisableTitle,
		}
		if e.PageRef != "" {
			block["pageRef"] = e.PageRef
		}
		if e.Weight != 0 {
			block["weight"] = e.Weight
		}
		out = append(out, block)
	}
	// Preserve any pre-existing sidebarmenus (e.g., the user's main
	// page menu) AFTER the category menus.
	out = append(out, existing...)
	// When keep_default_menu is true and the user has not already
	// provided a "main" page menu, append the Relearn default.
	if g.config.Hugo.Sidebar.ShouldKeepDefaultMenu() && !hasMainPageMenu(existing) {
		out = append(out, map[string]any{
			"identifier": "main",
			"type":       "page",
			"pageRef":    "/",
		})
	}
	// The Hugo "shortcuts" menu is conventionally the second sidebar
	// block; preserve it if the user has defined one.
	if _, hasShortcuts := root.Menu["shortcuts"]; hasShortcuts {
		out = append(out, map[string]any{
			"identifier": "shortcuts",
			"type":       "menu",
		})
	}
	root.Params["sidebarmenus"] = out
}

// hasMainPageMenu reports whether the existing sidebarmenus list
// already contains a "page" menu with identifier "main" (the Relearn
// default). When it does, we do not append a duplicate.
func hasMainPageMenu(sidebars []any) bool {
	for _, s := range sidebars {
		m, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "page" && m["identifier"] == "main" {
			return true
		}
	}
	return false
}
