package hugo

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
	"gopkg.in/yaml.v3"
)

// generateIndexPages creates index pages for sections and the main site
func (g *Generator) generateIndexPages(docFiles []docs.DocFile) error {
	if err := g.generateMainIndex(docFiles); err != nil {
		return err
	}
	if err := g.generateRepositoryIndexes(docFiles); err != nil {
		return err
	}
	if err := g.generateSectionIndexes(docFiles); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateMainIndex(docFiles []docs.DocFile) error {
	indexPath := filepath.Join(g.buildRoot(), "content", "_index.md")
	repoGroups := make(map[string][]docs.DocFile)
	for _, file := range docFiles {
		repoGroups[file.Repository] = append(repoGroups[file.Repository], file)
	}
	frontMatter := map[string]any{"title": g.config.Hugo.Title, "description": g.config.Hugo.Description, "date": time.Now().Format("2006-01-02T15:04:05-07:00"), "type": "docs"}
	if g.config.Hugo.ThemeType() == config.ThemeHextra {
		frontMatter["cascade"] = map[string]any{"type": "docs"}
	}
	fmData, err := yaml.Marshal(frontMatter)
	if err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrIndexGenerationFailed, err)
	}
	// File-based template overrides
	tplRaw := g.mustIndexTemplate("main")
	ctx := buildIndexTemplateContext(g, docFiles, repoGroups, frontMatter)
	tpl, err := template.New("main_index").Funcs(template.FuncMap{"titleCase": titleCase, "replaceAll": strings.ReplaceAll, "lower": strings.ToLower}).Parse(tplRaw)
	if err != nil {
		return fmt.Errorf("parse main index template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("exec main index template: %w", err)
	}
	body := buf.String()
	var content string
	if !strings.HasPrefix(body, "---\n") {
		content = fmt.Sprintf("---\n%s---\n\n%s", string(fmData), body)
	} else {
		content = body
	}
	// #nosec G306 -- index pages are public content
	if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write index at %s: %w", indexPath, err)
	}
	slog.Info("Generated main index page", logfields.Path(indexPath))
	return nil
}

func (g *Generator) generateRepositoryIndexes(docFiles []docs.DocFile) error {
	repoGroups := make(map[string][]docs.DocFile)
	for _, file := range docFiles {
		// Only include markdown files in repository indexes, not assets
		if !file.IsAsset {
			repoGroups[file.Repository] = append(repoGroups[file.Repository], file)
		}
	}
	for repoName, files := range repoGroups {
		indexPath := filepath.Join(g.buildRoot(), "content", repoName, "_index.md")
		if err := os.MkdirAll(filepath.Dir(indexPath), 0o750); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
		}
		frontMatter := map[string]any{"title": titleCase(repoName), "repository": repoName, "type": "docs", "date": time.Now().Format("2006-01-02T15:04:05-07:00")}
		fmData, err := yaml.Marshal(frontMatter)
		if err != nil {
			return fmt.Errorf("failed to marshal front matter: %w", err)
		}
		sectionGroups := make(map[string][]docs.DocFile)
		for _, file := range files {
			// files already filtered to exclude assets, so no need to check again
			s := file.Section
			if s == "" {
				s = "root"
			}
			sectionGroups[s] = append(sectionGroups[s], file)
		}
		tplRaw := g.mustIndexTemplate("repository")
		ctx := buildIndexTemplateContext(g, files, map[string][]docs.DocFile{repoName: files}, frontMatter)
		ctx["Sections"] = sectionGroups
		// Add repository metadata if available
		if repoConfig := g.findRepositoryConfig(repoName); repoConfig != nil {
			ctx["RepositoryInfo"] = map[string]any{
				"URL":         repoConfig.URL,
				"Branch":      repoConfig.Branch,
				"Description": repoConfig.Description,
				"Tags":        repoConfig.Tags,
			}
		}
		tpl, err := template.New("repo_index").Funcs(template.FuncMap{"titleCase": titleCase, "replaceAll": strings.ReplaceAll, "lower": strings.ToLower}).Parse(tplRaw)
		if err != nil {
			return fmt.Errorf("parse repository index template: %w", err)
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, ctx); err != nil {
			return fmt.Errorf("exec repository index template: %w", err)
		}
		body := buf.String()
		var content string
		if !strings.HasPrefix(body, "---\n") {
			content = fmt.Sprintf("---\n%s---\n\n%s", string(fmData), body)
		} else {
			content = body
		}
		// #nosec G306 -- index pages are public content
		if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write repository index: %w", err)
		}
		slog.Debug("Generated repository index", logfields.Repository(repoName), logfields.Path(indexPath))
	}
	return nil
}

// findRepositoryConfig looks up the config.Repository by name
func (g *Generator) findRepositoryConfig(name string) *config.Repository {
	for i := range g.config.Repositories {
		if g.config.Repositories[i].Name == name {
			return &g.config.Repositories[i]
		}
	}
	return nil
}

func (g *Generator) generateSectionIndexes(docFiles []docs.DocFile) error {
	sectionGroups := make(map[string]map[string][]docs.DocFile)
	allSections := make(map[string]map[string]bool) // Track all sections including intermediate ones

	for _, file := range docFiles {
		if file.Section == "" {
			continue
		}
		if sectionGroups[file.Repository] == nil {
			sectionGroups[file.Repository] = make(map[string][]docs.DocFile)
			allSections[file.Repository] = make(map[string]bool)
		}
		sectionGroups[file.Repository][file.Section] = append(sectionGroups[file.Repository][file.Section], file)

		// Track all parent sections to ensure intermediate directories get _index.md files
		section := file.Section
		for section != "" && section != "." {
			allSections[file.Repository][section] = true
			section = filepath.Dir(section)
		}
	}

	for repoName, sections := range sectionGroups {
		for sectionName, files := range sections {
			// Skip sections that only contain assets (no markdown files)
			hasMarkdown := false
			hasUserIndex := false
			for _, f := range files {
				if !f.IsAsset {
					hasMarkdown = true
					// Check if this section has a user-provided index.md file
					if f.Name == "index" && f.Section == sectionName {
						hasUserIndex = true
					}
				}
			}
			if !hasMarkdown {
				slog.Debug("Skipping section index for asset-only directory", logfields.Repository(repoName), logfields.Section(sectionName))
				continue
			}

			// Skip generating _index.md if user provided an index.md
			if hasUserIndex {
				slog.Debug("Using user-provided index.md for section", logfields.Repository(repoName), logfields.Section(sectionName))
				continue
			}

			indexPath := filepath.Join(g.buildRoot(), "content", repoName, sectionName, "_index.md")
			if err := os.MkdirAll(filepath.Dir(indexPath), 0o750); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
			}
			// Use only the last segment of the section path for the title
			sectionTitle := filepath.Base(sectionName)
			frontMatter := map[string]any{"title": titleCase(sectionTitle), "repository": repoName, "section": sectionName, "date": time.Now().Format("2006-01-02T15:04:05-07:00")}
			fmData, err := yaml.Marshal(frontMatter)
			if err != nil {
				return fmt.Errorf("failed to marshal front matter: %w", err)
			}

			// Find immediate child subsections
			var subsections []string
			for otherSection := range allSections[repoName] {
				// Check if otherSection is a direct child of sectionName
				if strings.HasPrefix(otherSection, sectionName+"/") {
					// Get the relative path from this section
					relPath := strings.TrimPrefix(otherSection, sectionName+"/")
					// Only include if it's an immediate child (no further slashes)
					if !strings.Contains(relPath, "/") {
						subsections = append(subsections, relPath)
					}
				}
			}

			tplRaw := g.mustIndexTemplate("section")
			ctx := buildIndexTemplateContext(g, files, map[string][]docs.DocFile{repoName: files}, frontMatter)
			ctx["SectionName"] = sectionName
			ctx["Files"] = files
			ctx["Subsections"] = subsections
			tpl, err := template.New("section_index").Funcs(template.FuncMap{"titleCase": titleCase, "replaceAll": strings.ReplaceAll, "lower": strings.ToLower}).Parse(tplRaw)
			if err != nil {
				return fmt.Errorf("parse section index template: %w", err)
			}
			var buf bytes.Buffer
			if err := tpl.Execute(&buf, ctx); err != nil {
				return fmt.Errorf("exec section index template: %w", err)
			}
			body := buf.String()
			var content string
			if !strings.HasPrefix(body, "---\n") {
				content = fmt.Sprintf("---\n%s---\n\n%s", string(fmData), body)
			} else {
				content = body
			}
			// #nosec G306 -- index pages are public content
			if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
				return fmt.Errorf("failed to write section index: %w", err)
			}
			slog.Debug("Generated section index", logfields.Repository(repoName), logfields.Section(sectionName), logfields.Path(indexPath))
		}

		// Create _index.md for intermediate sections that don't have files directly in them
		// This ensures proper nesting in Hugo's navigation
		for sectionName := range allSections[repoName] {
			// Skip if we already created an index for this section
			if _, hasFiles := sections[sectionName]; hasFiles {
				continue
			}

			indexPath := filepath.Join(g.buildRoot(), "content", repoName, sectionName, "_index.md")
			if err := os.MkdirAll(filepath.Dir(indexPath), 0o750); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
			}

			// Use only the last segment of the section path for the title
			sectionTitle := filepath.Base(sectionName)
			frontMatter := map[string]any{"title": titleCase(sectionTitle), "repository": repoName, "section": sectionName, "date": time.Now().Format("2006-01-02T15:04:05-07:00")}
			fmData, err := yaml.Marshal(frontMatter)
			if err != nil {
				return fmt.Errorf("failed to marshal front matter: %w", err)
			}

			// Find immediate child subsections for intermediate sections
			var subsections []string
			for otherSection := range allSections[repoName] {
				// Check if otherSection is a direct child of sectionName
				if strings.HasPrefix(otherSection, sectionName+"/") {
					// Get the relative path from this section
					relPath := strings.TrimPrefix(otherSection, sectionName+"/")
					// Only include if it's an immediate child (no further slashes)
					if !strings.Contains(relPath, "/") {
						subsections = append(subsections, relPath)
					}
				}
			}

			// Use a simple template for intermediate sections
			tplRaw := g.mustIndexTemplate("section")
			ctx := buildIndexTemplateContext(g, []docs.DocFile{}, map[string][]docs.DocFile{}, frontMatter)
			ctx["SectionName"] = sectionName
			ctx["Files"] = []docs.DocFile{}
			ctx["Subsections"] = subsections
			tpl, err := template.New("section_index").Funcs(template.FuncMap{"titleCase": titleCase, "replaceAll": strings.ReplaceAll, "lower": strings.ToLower}).Parse(tplRaw)
			if err != nil {
				return fmt.Errorf("parse section index template: %w", err)
			}
			var buf bytes.Buffer
			if err := tpl.Execute(&buf, ctx); err != nil {
				return fmt.Errorf("exec section index template: %w", err)
			}
			body := buf.String()
			var content string
			if !strings.HasPrefix(body, "---\n") {
				content = fmt.Sprintf("---\n%s---\n\n%s", string(fmData), body)
			} else {
				content = body
			}
			// #nosec G306 -- index pages are public content
			if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
				return fmt.Errorf("failed to write intermediate section index: %w", err)
			}
			slog.Debug("Generated intermediate section index", logfields.Repository(repoName), logfields.Section(sectionName), logfields.Path(indexPath))
		}
	}
	return nil
}

// buildIndexTemplateContext assembles a reusable context for index templates exposing
// site metadata, repositories, files, and simple aggregate stats.
func buildIndexTemplateContext(g *Generator, docFiles []docs.DocFile, repoGroups map[string][]docs.DocFile, frontMatter map[string]any) map[string]any {
	ctx := map[string]any{}
	ctx["Site"] = map[string]any{
		"Title":       g.config.Hugo.Title,
		"Description": g.config.Hugo.Description,
		"BaseURL":     g.config.Hugo.BaseURL,
		"Theme":       g.config.Hugo.ThemeType(),
	}
	ctx["FrontMatter"] = frontMatter
	ctx["Repositories"] = repoGroups
	ctx["Files"] = docFiles
	ctx["Now"] = time.Now()
	ctx["Stats"] = map[string]any{
		"TotalFiles":        len(docFiles),
		"TotalRepositories": len(repoGroups),
	}
	return ctx
}

// loadIndexTemplate attempts to locate a template override for index pages.
// Search order (first hit wins):
//  1. <outputDir>/templates/index/<kind>.md.tmpl
//  2. <outputDir>/templates/index/<kind>.tmpl
//  3. <outputDir>/templates/<kind>_index.tmpl
//
// Returns content or an error if no file found (caller treats missing as fallback trigger).
func (g *Generator) loadIndexTemplate(kind string) (string, error) {
	base := g.outputDir
	candidates := []string{
		filepath.Join(base, "templates", "index", kind+".md.tmpl"),
		filepath.Join(base, "templates", "index", kind+".tmpl"),
		filepath.Join(base, "templates", kind+"_index.tmpl"),
	}
	for _, p := range candidates {
		// #nosec G304 - p is from predefined template paths, base is controlled
		b, err := os.ReadFile(p)
		if err == nil {
			slog.Debug("Loaded index template override", slog.String("kind", kind), logfields.Path(p))
			if g != nil && g.indexTemplateUsage != nil {
				g.indexTemplateUsage[kind] = IndexTemplateInfo{Source: "file", Path: p}
			}
			return string(b), nil
		}
	}
	return "", fmt.Errorf("no template override for kind %s", kind)
}

//go:embed templates_defaults/index/*.tmpl
var embeddedIndexTemplates embed.FS

// mustIndexTemplate returns either a user override template body or the embedded default.
// Panics only if embedded defaults are missing (programmer error), not on user absence.
func (g *Generator) mustIndexTemplate(kind string) string {
	if raw, err := g.loadIndexTemplate(kind); err == nil && strings.TrimSpace(raw) != "" {
		return raw
	}
	// fall back to embedded default
	name := fmt.Sprintf("templates_defaults/index/%s.tmpl", kind)
	b, err := embeddedIndexTemplates.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("embedded default index template missing for kind %s: %v", kind, err))
	}
	if g != nil && g.indexTemplateUsage != nil {
		// Only set if not already recorded by file override
		if _, exists := g.indexTemplateUsage[kind]; !exists {
			g.indexTemplateUsage[kind] = IndexTemplateInfo{Source: "embedded"}
		}
	}
	return string(b)
}
