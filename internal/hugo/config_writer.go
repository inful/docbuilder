package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"gopkg.in/yaml.v3"
)

// generateHugoConfig creates the Hugo configuration file
func (g *Generator) generateHugoConfig() error {
	configPath := filepath.Join(g.buildRoot(), "hugo.yaml")
	features := g.deriveThemeFeatures()

	// Phase 1: core defaults
	params := map[string]any{}
	root := map[string]any{
		"title":         g.config.Hugo.Title,
		"description":   g.config.Hugo.Description,
		"baseURL":       g.config.Hugo.BaseURL,
		"languageCode":  "en",
		"enableGitInfo": true,
		"markup": map[string]any{
			"goldmark":  map[string]any{"renderer": map[string]any{"unsafe": true}},
			"highlight": map[string]any{"style": "github", "lineNos": true, "tabWidth": 4, "noClasses": false},
		},
		"params": params,
	}

	// Phase 2: theme param injection (themes self-register). If theme not found or not registered, use legacy fallback logic.
	if th := g.activeTheme(); th != nil {
		th.ApplyParams(g, params)
	} else {
		// Fallback until theme packages can register without import cycles.
		if g.config.Hugo.ThemeType() == "docsy" { legacyAddDocsyParams(g, params) }
		if g.config.Hugo.ThemeType() == "hextra" { legacyAddHextraParams(g, params) }
	}

	// Phase 3: user overrides (deep merge)
	if g.config.Hugo.Params != nil {
		mergeParams(params, g.config.Hugo.Params)
	}

	// Phase 4: dynamic fields
	params["build_date"] = time.Now().Format("2006-01-02 15:04:05")

	// Phase 5: module/theme block
	if g.config.Hugo.Theme != "" {
		if features.UsesModules && features.ModulePath != "" {
			root["module"] = map[string]any{"imports": []map[string]any{{"path": features.ModulePath}}}
		} else {
			root["theme"] = g.config.Hugo.Theme
		}
	}

	// Math passthrough
	if features.EnableMathPassthrough {
		if m, ok := root["markup"].(map[string]any); ok {
			gm, _ := m["goldmark"].(map[string]any)
			if gm == nil {
				gm = map[string]any{}
				m["goldmark"] = gm
			}
			ext, _ := gm["extensions"].(map[string]any)
			if ext == nil {
				ext = map[string]any{}
				gm["extensions"] = ext
			}
			ext["passthrough"] = map[string]any{
				"delimiters": map[string]any{
					"block":  [][]string{{"\\[", "\\]"}, {"$$", "$$"}},
					"inline": [][]string{{"\\(", "\\)"}},
				},
				"enable": true,
			}
		}
	}

	if features.EnableOfflineSearchJSON {
		root["outputs"] = map[string]any{"home": []string{"HTML", "RSS", "JSON"}}
	}

	// Phase 6: menu
	if features.AutoMainMenu {
		if g.config.Hugo.Menu == nil {
			mainMenu := []map[string]any{{"name": "Search", "weight": 4, "params": map[string]any{"type": "search"}}, {"name": "Theme", "weight": 98, "params": map[string]any{"type": "theme-toggle", "label": false}}}
			for _, repo := range g.config.Repositories {
				if strings.Contains(repo.URL, "github.com") {
					mainMenu = append(mainMenu, map[string]any{"name": "GitHub", "weight": 99, "url": repo.URL, "params": map[string]any{"icon": "github"}})
					break
				}
			}
			root["menu"] = map[string]any{"main": mainMenu}
		} else {
			root["menu"] = g.config.Hugo.Menu
		}
	} else if g.config.Hugo.Menu != nil {
		root["menu"] = g.config.Hugo.Menu
	}

	// Phase 7: theme final customization
	if th := g.activeTheme(); th != nil {
		th.CustomizeRoot(g, root)
	}

	data, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("failed to marshal Hugo config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write Hugo config: %w", err)
	}

	if features.UsesModules {
		if err := g.ensureGoModForModules(); err != nil {
			slog.Warn("Failed to ensure go.mod for Hugo Modules", "error", err)
		}
	}
	slog.Info("Generated Hugo configuration", logfields.Path(configPath))
	return nil
}

// legacyAddDocsyParams replicates previous inline theme param defaults (temporary fallback during theme modularization).
func legacyAddDocsyParams(g *Generator, params map[string]any) {
	if params["version"] == nil { params["version"] = "main" }
	if params["github_repo"] == nil && len(g.config.Repositories) > 0 {
		first := g.config.Repositories[0]
		if strings.Contains(first.URL, "github.com") { params["github_repo"] = first.URL }
	}
	if params["github_branch"] == nil && len(g.config.Repositories) > 0 { params["github_branch"] = g.config.Repositories[0].Branch }
	if params["edit_page"] == nil { params["edit_page"] = true }
	if params["search"] == nil { params["search"] = true }
	if params["offlineSearch"] == nil { params["offlineSearch"] = true }
	if params["offlineSearchSummaryLength"] == nil { params["offlineSearchSummaryLength"] = 200 }
	if params["offlineSearchMaxResults"] == nil { params["offlineSearchMaxResults"] = 25 }
	if params["ui"] == nil { params["ui"] = map[string]any{"sidebar_menu_compact": false, "sidebar_menu_foldable": true, "breadcrumb_disable": false, "taxonomy_breadcrumb_disable": false, "footer_about_disable": false, "navbar_logo": true, "navbar_translucent_over_cover_disable": false, "sidebar_search_disable": false} }
	if params["links"] == nil {
		links := map[string]any{"user": []map[string]any{}, "developer": []map[string]any{}}
		for _, repo := range g.config.Repositories {
			if strings.Contains(repo.URL, "github.com") {
				link := map[string]any{"name": fmt.Sprintf("%s Repository", TitleCase(repo.Name)), "url": repo.URL, "icon": "fab fa-github", "desc": fmt.Sprintf("Development happens here for %s", repo.Name)}
				if dev, ok := links["developer"].([]map[string]any); ok { links["developer"] = append(dev, link) }
			}
		}
		params["links"] = links
	}
}

func legacyAddHextraParams(g *Generator, params map[string]any) {
	if params["search"] == nil {
		params["search"] = map[string]any{"enable": true, "type": "flexsearch", "flexsearch": map[string]any{"index": "content", "tokenize": "forward", "version": "0.8.143"}}
	} else if b, ok := params["search"].(bool); ok {
		params["search"] = map[string]any{"enable": b}
	} else if m, ok := params["search"].(map[string]any); ok {
		if _, ok := m["enable"]; !ok { m["enable"] = true }
		if _, ok := m["type"]; !ok { m["type"] = "flexsearch" }
		if _, ok := m["flexsearch"]; !ok { m["flexsearch"] = map[string]any{"index": "content", "tokenize": "forward", "version": "0.8.143"} } else if fm, ok := m["flexsearch"].(map[string]any); ok {
			if _, ok := fm["index"]; !ok { fm["index"] = "content" }
			if _, ok := fm["tokenize"]; !ok { fm["tokenize"] = "forward" }
			if _, ok := fm["version"]; !ok { fm["version"] = "0.8.143" }
		}
	}
	if params["offlineSearch"] == nil { params["offlineSearch"] = true }
	if params["offlineSearchSummaryLength"] == nil { params["offlineSearchSummaryLength"] = 200 }
	if params["offlineSearchMaxResults"] == nil { params["offlineSearchMaxResults"] = 25 }
	if _, ok := params["theme"].(map[string]any); !ok { params["theme"] = map[string]any{"default": "system", "displayToggle": true} }
	if params["ui"] == nil { params["ui"] = map[string]any{"navbar_logo": true, "sidebar_menu_foldable": true, "sidebar_menu_compact": false, "sidebar_search_disable": false} }
	if _, ok := params["mermaid"]; !ok { params["mermaid"] = map[string]any{} }
	if v, ok := params["editURL"]; !ok { params["editURL"] = map[string]any{"enable": true} } else if m, ok := v.(map[string]any); ok { if _, exists := m["enable"]; !exists { m["enable"] = true } }
	if _, ok := params["navbar"].(map[string]any); !ok { params["navbar"] = map[string]any{"width": "normal"} }
}
