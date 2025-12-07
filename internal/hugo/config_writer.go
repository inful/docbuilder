package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	th "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"gopkg.in/yaml.v3"
)

// generateHugoConfig creates the Hugo configuration file
func (g *Generator) generateHugoConfig() error {
	configPath := filepath.Join(g.buildRoot(), "hugo.yaml")
	// Apply theme via centralized engine (params injection + root customization)
	// Capture returned features and cache for downstream usage.

	// Phase 1: core defaults (typed root)
	params := map[string]any{}
	root := &models.RootConfig{
		Title:         g.config.Hugo.Title,
		Description:   g.config.Hugo.Description,
		BaseURL:       g.config.Hugo.BaseURL,
		LanguageCode:  "en",
		EnableGitInfo: true,
		Markup:        map[string]any{},
		Params:        params,
	}
	// Apply default markup settings via helpers (same YAML shape)
	root.EnsureGoldmarkRendererUnsafe()
	root.EnsureHighlightDefaults()

	// Phase 2: theme param injection via engine.
	// Note: Engine expects root as map[string]any; pass a temporary view during phase application.
	tmpRoot := map[string]any{
		"title":         root.Title,
		"description":   root.Description,
		"baseURL":       root.BaseURL,
		"languageCode":  root.LanguageCode,
		"enableGitInfo": root.EnableGitInfo,
		"markup":        root.Markup,
		"params":        root.Params,
	}
	feats := th.Engine{}.ApplyPhases(g, g.config, tmpRoot, params)
	// Sync any changes back to typed root (only known fields handled here)
	if v, ok := tmpRoot["markup"].(map[string]any); ok {
		root.Markup = v
	}
	g.cachedThemeFeatures = &feats

	// Phase 3: user overrides (deep merge)
	if g.config.Hugo.Params != nil {
		mergeParams(params, g.config.Hugo.Params)
	}

	// Phase 4: dynamic fields
	params["build_date"] = time.Now().Format("2006-01-02 15:04:05")

	// Phase 5: module/theme block
	if g.config.Hugo.Theme != "" {
		if feats.UsesModules && feats.ModulePath != "" {
			root.Module = &models.ModuleConfig{Imports: []models.ModuleImport{{Path: feats.ModulePath}}}
		} else {
			root.Theme = g.config.Hugo.Theme
		}
	}

	// Math passthrough
	if feats.EnableMathPassthrough {
		root.EnableMathPassthrough()
	}

	// In preview/live-reload mode, disable GitInfo (staging isn't a git repo)
	if g.config.Build.LiveReload {
		root.EnableGitInfo = false
	}

	// Enable search JSON only when not in live preview to avoid missing layouts
	if feats.EnableOfflineSearchJSON && !g.config.Build.LiveReload {
		root.SetHomeOutputsHTMLRSSJSON()
	}

	// Phase 6: menu
	if feats.AutoMainMenu {
		if g.config.Hugo.Menu == nil {
			mainMenu := []map[string]any{{"name": "Search", "weight": 4, "params": map[string]any{"type": "search"}}, {"name": "Theme", "weight": 98, "params": map[string]any{"type": "theme-toggle", "label": false}}}
			for _, repo := range g.config.Repositories {
				if strings.Contains(repo.URL, "github.com") {
					mainMenu = append(mainMenu, map[string]any{"name": "GitHub", "weight": 99, "url": repo.URL, "params": map[string]any{"icon": "github"}})
					break
				}
			}
			root.Menu = map[string]any{"main": mainMenu}
		} else {
			// Convert typed menu map to loose map for YAML
			converted := map[string]any{}
			for k, items := range g.config.Hugo.Menu {
				list := make([]map[string]any, 0, len(items))
				for _, it := range items {
					m := map[string]any{"name": it.Name, "url": it.URL}
					if it.Weight != 0 {
						m["weight"] = it.Weight
					}
					list = append(list, m)
				}
				converted[k] = list
			}
			root.Menu = converted
		}
	} else if g.config.Hugo.Menu != nil {
		converted := map[string]any{}
		for k, items := range g.config.Hugo.Menu {
			list := make([]map[string]any, 0, len(items))
			for _, it := range items {
				m := map[string]any{"name": it.Name, "url": it.URL}
				if it.Weight != 0 {
					m["weight"] = it.Weight
				}
				list = append(list, m)
			}
			converted[k] = list
		}
		root.Menu = converted
	}

	// Phase 7 handled by engine above

	data, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrConfigMarshalFailed, err)
	}
	// #nosec G306 -- hugo.yaml is a public configuration file
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write hugo config: %w", err)
	}

	if feats.UsesModules {
		if err := g.ensureGoModForModules(); err != nil {
			slog.Warn("Failed to ensure go.mod for Hugo Modules", "error", err)
		}
	}
	slog.Info("Generated Hugo configuration", logfields.Path(configPath))
	return nil
}

// (legacy param helpers removed)
