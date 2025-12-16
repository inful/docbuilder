package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"gopkg.in/yaml.v3"
)

// generateHugoConfig creates the Hugo configuration file with Relearn theme
func (g *Generator) generateHugoConfig() error {
	configPath := filepath.Join(g.buildRoot(), "hugo.yaml")

	// Phase 1: core defaults
	params := map[string]any{}
	root := &models.RootConfig{
		Title:         g.config.Hugo.Title,
		Description:   g.config.Hugo.Description,
		BaseURL:       g.config.Hugo.BaseURL,
		EnableGitInfo: false, // Disabled by default; output dir isn't a git repo
		Markup:        map[string]any{},
		Params:        params,
		Taxonomies:    g.config.Hugo.Taxonomies,
	}

	// Apply default markup settings
	root.EnsureGoldmarkRendererUnsafe()
	root.EnsureHighlightDefaults()

	// Phase 2: Apply Relearn theme defaults
	g.applyRelearnThemeDefaults(params)

	// Phase 3: User overrides (deep merge)
	if g.config.Hugo.Params != nil {
		mergeParams(params, g.config.Hugo.Params)
	}

	// Phase 4: Dynamic fields
	params["build_date"] = time.Now().Format("2006-01-02 15:04:05")

	// Phase 4.5: Version metadata collection
	if g.config.Versioning != nil && !g.config.Versioning.DefaultBranchOnly {
		versionInfo := g.collectVersionMetadata()
		if len(versionInfo) > 0 {
			params["versions"] = versionInfo
			slog.Debug("Added version metadata to Hugo config", "repo_count", len(versionInfo))
		}
	}

	// Phase 5: Configure Relearn theme via Hugo Modules
	root.Module = &models.ModuleConfig{
		Imports: []models.ModuleImport{{Path: "github.com/McShelby/hugo-theme-relearn"}},
	}

	// Enable math passthrough for Relearn
	root.EnableMathPassthrough()

	// In preview/live-reload mode, disable GitInfo
	if g.config.Build.LiveReload {
		root.EnableGitInfo = false
	}

	// Enable search JSON (Relearn uses Lunr search)
	if !g.config.Build.LiveReload {
		root.SetHomeOutputsHTMLRSSJSON()
	}

	// Phase 5.5: Taxonomies configuration
	if len(g.config.Hugo.Taxonomies) > 0 {
		root.Taxonomies = g.config.Hugo.Taxonomies
	} else {
		// Use Hugo's default taxonomies
		root.Taxonomies = map[string]string{
			"tag":      "tags",
			"category": "categories",
		}
	}

	// Phase 6: Language configuration (required by Relearn)
	root.DefaultContentLanguage = "en"
	root.Languages = map[string]any{
		"en": map[string]any{
			"languageName": "English",
			"weight":       1,
		},
	}

	// Phase 7: Menu configuration (if provided)
	if g.config.Hugo.Menu != nil {
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

	data, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrConfigMarshalFailed, err)
	}

	// #nosec G306 -- hugo.yaml is a public configuration file
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write hugo config: %w", err)
	}

	// Ensure go.mod for Hugo Modules (Relearn requires this)
	if err := g.ensureGoModForModules(); err != nil {
		slog.Warn("Failed to ensure go.mod for Hugo Modules", "error", err)
	}

	slog.Info("Generated Hugo configuration with Relearn theme", logfields.Path(configPath))
	slog.Debug("Hugo configuration content:\n" + string(data))

	return nil
}

// applyRelearnThemeDefaults applies Relearn-specific parameter defaults
func (g *Generator) applyRelearnThemeDefaults(params map[string]any) {
	// Theme variant/color scheme - auto mode with zen-light/zen-dark
	if params["themeVariant"] == nil {
		params["themeVariant"] = []any{"auto", "zen-light", "zen-dark"}
	}

	// Configure auto mode fallbacks
	if params["themeVariantAuto"] == nil {
		hasAuto := false
		if variants, ok := params["themeVariant"].([]any); ok {
			for _, v := range variants {
				if str, ok := v.(string); ok && str == "auto" {
					hasAuto = true
					break
				}
			}
		} else if str, ok := params["themeVariant"].(string); ok && str == "auto" {
			hasAuto = true
		}
		if hasAuto {
			params["themeVariantAuto"] = []string{"zen-light", "zen-dark"}
		}
	}

	// Disable generator notice in footer
	if params["disableGeneratorVersion"] == nil {
		params["disableGeneratorVersion"] = false
	}

	// Breadcrumb navigation
	if params["disableBreadcrumb"] == nil {
		params["disableBreadcrumb"] = false
	}

	// Show visited checkmarks
	if params["showVisitedLinks"] == nil {
		params["showVisitedLinks"] = true
	}

	// Collapse menu sections
	if params["collapsibleMenu"] == nil {
		params["collapsibleMenu"] = true
	}

	// Always open menu on start
	if params["alwaysopen"] == nil {
		params["alwaysopen"] = false
	}

	// Disable landing page button
	if params["disableLandingPageButton"] == nil {
		params["disableLandingPageButton"] = true
	}

	// Disable shortcuts menu in sidebar
	if params["disableShortcutsTitle"] == nil {
		params["disableShortcutsTitle"] = false
	}

	// Disable language switching button
	if params["disableLanguageSwitchingButton"] == nil {
		params["disableLanguageSwitchingButton"] = true
	}

	// Disable tag hidden pages
	if params["disableTagHiddenPages"] == nil {
		params["disableTagHiddenPages"] = false
	}

	// Mermaid diagrams support
	if _, ok := params["mermaid"]; !ok {
		params["mermaid"] = map[string]any{
			"enable": true,
		}
	}

	// Math support (using MathJax by default in Relearn)
	if _, ok := params["math"]; !ok {
		params["math"] = map[string]any{
			"enable": true,
		}
	}

	// View Transitions API support
	if g.config.Hugo.EnablePageTransitions {
		params["enable_transitions"] = true
	}
}

// collectVersionMetadata collects version information from versioned repositories
// Returns a map of base repository names to their available versions
func (g *Generator) collectVersionMetadata() map[string]any {
	versionsByBase := make(map[string][]map[string]any)

	for _, repo := range g.config.Repositories {
		// Skip non-versioned repos
		if !repo.IsVersioned {
			continue
		}

		// Extract base repo name from tags
		baseRepo := repo.Name
		if base, ok := repo.Tags["base_repo"]; ok {
			baseRepo = base
		}

		// Create version entry
		versionEntry := map[string]any{
			"name":    repo.Name,
			"version": repo.Version,
			"branch":  repo.Branch,
		}

		// Add optional metadata from tags
		if vtype, ok := repo.Tags["version_type"]; ok {
			versionEntry["type"] = vtype
		}
		if repo.Description != "" {
			versionEntry["description"] = repo.Description
		}

		versionsByBase[baseRepo] = append(versionsByBase[baseRepo], versionEntry)
	}

	// Convert to generic map for YAML serialization
	result := make(map[string]any)
	for base, versions := range versionsByBase {
		result[base] = versions
	}

	return result
}

// (legacy param helpers removed)
