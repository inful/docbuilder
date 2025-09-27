package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"gopkg.in/yaml.v3"
)

// generateHugoConfig creates the Hugo configuration file
func (g *Generator) generateHugoConfig() error {
	configPath := filepath.Join(g.buildRoot(), "hugo.yaml")

	params := make(map[string]interface{})
	if g.config.Hugo.Params != nil {
		params = g.config.Hugo.Params
	}
	params["build_date"] = time.Now().Format("2006-01-02 15:04:05")

	features := g.deriveThemeFeatures()
	if th := g.activeTheme(); th != nil {
		th.ApplyParams(g, params)
	} else {
		// Legacy fallback: retain existing conditional to avoid regression for unregistered themes.
		if features.Name == config.ThemeDocsy { g.addDocsyParams(params) } 
		if features.Name == config.ThemeHextra { g.addHextraParams(params) }
	}

	hugoConfig := map[string]interface{}{
		"title":         g.config.Hugo.Title,
		"description":   g.config.Hugo.Description,
		"baseURL":       g.config.Hugo.BaseURL,
		"languageCode":  "en",
		"enableGitInfo": true,
		"markup": map[string]interface{}{
			"goldmark": map[string]interface{}{
				"renderer": map[string]interface{}{"unsafe": true},
			},
			"highlight": map[string]interface{}{
				"style":     "github",
				"lineNos":   true,
				"tabWidth":  4,
				"noClasses": false,
			},
		},
		"params": params,
	}

	if g.config.Hugo.Theme != "" {
		if features.UsesModules && features.ModulePath != "" {
			hugoConfig["module"] = map[string]interface{}{"imports": []map[string]interface{}{{"path": features.ModulePath}}}
		} else {
			hugoConfig["theme"] = g.config.Hugo.Theme
		}
	}

	if features.EnableMathPassthrough { // math passthrough
		if m, ok := hugoConfig["markup"].(map[string]interface{}); ok {
			gm, _ := m["goldmark"].(map[string]interface{})
			if gm == nil {
				gm = map[string]interface{}{}
				m["goldmark"] = gm
			}
			ext, _ := gm["extensions"].(map[string]interface{})
			if ext == nil {
				ext = map[string]interface{}{}
				gm["extensions"] = ext
			}
			ext["passthrough"] = map[string]interface{}{
				"delimiters": map[string]interface{}{
					"block":  [][]string{{"\\[", "\\]"}, {"$$", "$$"}},
					"inline": [][]string{{"\\(", "\\)"}},
				},
				"enable": true,
			}
		}
	}

	if features.EnableOfflineSearchJSON { // offline search JSON
		hugoConfig["outputs"] = map[string]interface{}{"home": []string{"HTML", "RSS", "JSON"}}
	}

	// Allow theme hook to mutate root after core assembly.
	if th := g.activeTheme(); th != nil { th.CustomizeRoot(g, hugoConfig) }

	// Menu handling
	if features.AutoMainMenu {
		if g.config.Hugo.Menu == nil {
			mainMenu := []map[string]interface{}{
				{"name": "Search", "weight": 4, "params": map[string]interface{}{"type": "search"}},
				{"name": "Theme", "weight": 98, "params": map[string]interface{}{"type": "theme-toggle", "label": false}},
			}
			for _, repo := range g.config.Repositories { // add GitHub icon if any
				if strings.Contains(repo.URL, "github.com") {
					mainMenu = append(mainMenu, map[string]interface{}{"name": "GitHub", "weight": 99, "url": repo.URL, "params": map[string]interface{}{"icon": "github"}})
					break
				}
			}
			hugoConfig["menu"] = map[string]interface{}{"main": mainMenu}
		} else {
			hugoConfig["menu"] = g.config.Hugo.Menu
		}
	} else if g.config.Hugo.Menu != nil { // explicit menu retained
		hugoConfig["menu"] = g.config.Hugo.Menu
	}

	data, err := yaml.Marshal(hugoConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal Hugo config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write Hugo config: %w", err)
	}

	if features.UsesModules { // all current module-using themes
		if err := g.ensureGoModForModules(); err != nil {
			slog.Warn("Failed to ensure go.mod for Hugo Modules", "error", err)
		}
	}
	slog.Info("Generated Hugo configuration", logfields.Path(configPath))
	return nil
}
