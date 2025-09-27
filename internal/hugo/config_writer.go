package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"strings"

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

	// Phase 2: theme param injection via registered theme only.
	if at := g.activeTheme(); at != nil { at.ApplyParams(g, params) }

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
		} else { root["menu"] = g.config.Hugo.Menu }
	} else if g.config.Hugo.Menu != nil { root["menu"] = g.config.Hugo.Menu }

	// Phase 7: theme final customization
	if at := g.activeTheme(); at != nil { at.CustomizeRoot(g, root) }

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

// (legacy param helpers removed)
