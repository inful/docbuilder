package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"

	"gopkg.in/yaml.v3"

	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

const autoVariant = "auto"

// GenerateHugoConfig creates the Hugo configuration file with Relearn theme.
func (g *Generator) GenerateHugoConfig() error {
	configPath := filepath.Join(g.BuildRoot(), "hugo.yaml")

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
	root.EnsureGoldmarkParserAttributeBlockEnabled()
	root.EnsureHighlightDefaults()

	// Phase 2: Apply Relearn theme defaults
	g.applyRelearnThemeDefaults(params)

	// Phase 3: User overrides (deep merge)
	if g.config.Hugo.Params != nil {
		mergeParamsInline(params, g.config.Hugo.Params)
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
		Imports: []models.ModuleImport{{Path: "github.com/McShelby/hugo-theme-relearn", Version: "9.0.3"}},
	}

	// Enable math passthrough for Relearn
	root.EnableMathPassthrough()

	// In preview/live-reload mode, disable GitInfo
	if g.config.Build.LiveReload {
		root.EnableGitInfo = false
	}

	// Enable search JSON (Relearn uses Lunr search) and taxonomy JSON outputs
	if !g.config.Build.LiveReload {
		root.SetHomeOutputsHTMLRSSJSON()
		root.SetTaxonomyOutputsHTMLJSON()
	}

	// Phase 5.5: Taxonomies configuration
	if len(g.config.Hugo.Taxonomies) > 0 {
		root.Taxonomies = g.config.Hugo.Taxonomies
	} else {
		// Use Hugo's default taxonomies. `project` is included so a
		// doc's `projects: [a, b]` front matter is indexed as a
		// taxonomy term, which gives Hugo auto-generated listing
		// pages at `/project/<value>/` and consistent cross-page
		// navigation. The key/value pair follows the Hugo
		// convention (singular key → plural value, mirroring
		// `tag: tags` and `category: categories`); the front-matter
		// field name is the plural form. Authors who don't want
		// `project` treated as a taxonomy can override via
		// `hugo.taxonomies` in their config (the user-set map above
		// wins).
		root.Taxonomies = map[string]string{
			"tag":      "tags",
			"category": "categories",
			"project":  "projects",
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

	// Phase 7.5: Categories-menu wiring (only when the sidebar mode
	// is "categories"). This must run after Phase 7 so it can merge
	// the user-supplied menu on top, and before yaml.Marshal so the
	// final hugo.yaml includes the categories menus and sidebarmenus
	// blocks. The build is a no-op when the mode is "default" or
	// unset.
	g.ApplyCategoriesMenuToConfig(root)

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

// applyRelearnThemeDefaults applies Relearn-specific parameter defaults.
func (g *Generator) applyRelearnThemeDefaults(params map[string]any) {
	// Theme variant/color scheme - auto mode with zen-light/zen-dark
	if params["themeVariant"] == nil {
		params["themeVariant"] = []any{"auto", "zen-light", "zen-dark"}
	}

	// Configure auto mode fallbacks
	if params["themeVariantAuto"] == nil {
		if hasAutoVariant(params["themeVariant"]) {
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

	// Edit link configuration - per-page editURLs in frontmatter are enabled by default
	// Only set this if not already configured by user (to avoid suppressing per-page links).
	// In daemon public-only mode, we explicitly disable edit link UI.
	if g.config != nil && g.config.IsDaemonPublicOnlyEnabled() {
		delete(params, "editURL")
	} else {
		if _, ok := params["editURL"]; !ok {
			// Empty object enables edit link UI without suppressing per-page URLs
			params["editURL"] = map[string]any{}
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
// Returns a map of base repository names to their available versions.
func (g *Generator) collectVersionMetadata() map[string]any {
	versionsByBase := make(map[string][]map[string]any)

	for i := range g.config.Repositories {
		repo := &g.config.Repositories[i]
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

// hasAutoVariant checks if themeVariant contains "auto" value.
func hasAutoVariant(themeVariant any) bool {
	if variants, ok := themeVariant.([]any); ok {
		for _, v := range variants {
			if str, ok := v.(string); ok && str == autoVariant {
				return true
			}
		}
		return false
	}

	if str, ok := themeVariant.(string); ok {
		return str == autoVariant
	}

	return false
}

// (legacy param helpers removed)

// mergeParamsInline deep-merges src into dst (map[string]any).
// - Maps: merged recursively
// - Slices & scalars: replaced.
func mergeParamsInline(dst, src map[string]any) {
	if src == nil {
		return
	}
	for k, v := range src {
		if mv, ok := v.(map[string]any); ok {
			if existing, ok2 := dst[k].(map[string]any); ok2 {
				mergeParamsInline(existing, mv)
			} else {
				cp := map[string]any{}
				mergeParamsInline(cp, mv)
				dst[k] = cp
			}
			continue
		}
		dst[k] = v
	}
}
