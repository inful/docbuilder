package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/pipeline"

	"gopkg.in/yaml.v3"

	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// GenerateHugoConfig creates the Hugo configuration file for the theme.
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

	// Phase 2: Apply theme defaults
	g.applyDocsyThemeDefaults(params)

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

	// Phase 5: Configure the theme via Hugo Modules
	root.Module = &models.ModuleConfig{
		Imports: []models.ModuleImport{{Path: pipeline.DocsyModulePath, Version: pipeline.DocsyVersion}},
	}

	// Enable math passthrough for the theme
	root.EnableMathPassthrough()

	// In preview/live-reload mode, disable GitInfo
	if g.config.Build.LiveReload {
		root.EnableGitInfo = false
	}

	// The theme's search reads its own index, so no JSON output format
	// is needed; asking for one would leave Hugo warning about a
	// missing layout.
	if !g.config.Build.LiveReload {
		root.SetHomeOutputsHTMLRSS()
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

	// Phase 6: Language configuration (required by the theme)
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

	// Ensure go.mod for Hugo Modules (the theme is imported as one)
	if err := g.ensureGoModForModules(); err != nil {
		slog.Warn("Failed to ensure go.mod for Hugo Modules", "error", err)
	}

	slog.Info("Generated Hugo configuration with the Docsy theme", logfields.Path(configPath))
	slog.Debug("Hugo configuration content:\n" + string(data))

	return nil
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
