package hugo

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
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
	for _, file := range docFiles { repoGroups[file.Repository] = append(repoGroups[file.Repository], file) }
	frontMatter := map[string]any{"title": g.config.Hugo.Title, "description": g.config.Hugo.Description, "date": time.Now().Format("2006-01-02T15:04:05-07:00"), "type": "docs"}
	if g.config.Hugo.ThemeType() == config.ThemeHextra { frontMatter["cascade"] = map[string]any{"type": "docs"} }
	fmData, err := yaml.Marshal(frontMatter); if err != nil { return fmt.Errorf("failed to marshal front matter: %w", err) }
	// User template (params.index_template)
	var content string
	if tplRaw, ok := g.config.Hugo.Params["index_template"].(string); ok && strings.TrimSpace(tplRaw) != "" {
		ctx := buildIndexTemplateContext(g, docFiles, repoGroups, frontMatter)
		tpl, err := template.New("main_index").Funcs(template.FuncMap{"titleCase": titleCase}).Parse(tplRaw)
		if err != nil { return fmt.Errorf("parse main index template: %w", err) }
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, ctx); err != nil { return fmt.Errorf("exec main index template: %w", err) }
		body := buf.String()
		if !strings.HasPrefix(body, "---\n") { content = fmt.Sprintf("---\n%s---\n\n%s", string(fmData), body) } else { content = body }
	} else {
		content = fmt.Sprintf("---\n%s---\n\n# %s\n\n%s\n\n", string(fmData), g.config.Hugo.Title, g.config.Hugo.Description)
		content += "## Repositories\n\n"
		for repoName, files := range repoGroups { content += fmt.Sprintf("- [%s](./%s/) (%d files)\n", repoName, repoName, len(files)) }
	}
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil { return fmt.Errorf("failed to write main index: %w", err) }
	slog.Info("Generated main index page", logfields.Path(indexPath))
	return nil
}

func (g *Generator) generateRepositoryIndexes(docFiles []docs.DocFile) error {
	repoGroups := make(map[string][]docs.DocFile)
	for _, file := range docFiles { repoGroups[file.Repository] = append(repoGroups[file.Repository], file) }
	for repoName, files := range repoGroups {
		indexPath := filepath.Join(g.buildRoot(), "content", repoName, "_index.md")
		if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil { return fmt.Errorf("failed to create directory for %s: %w", indexPath, err) }
		frontMatter := map[string]any{"title": fmt.Sprintf("%s Documentation", titleCase(repoName)), "repository": repoName, "date": time.Now().Format("2006-01-02T15:04:05-07:00")}
		fmData, err := yaml.Marshal(frontMatter); if err != nil { return fmt.Errorf("failed to marshal front matter: %w", err) }
		sectionGroups := make(map[string][]docs.DocFile)
		for _, file := range files { s := file.Section; if s == "" { s = "root" }; sectionGroups[s] = append(sectionGroups[s], file) }
		var content string
		if tplRaw, ok := g.config.Hugo.Params["repository_index_template"].(string); ok && strings.TrimSpace(tplRaw) != "" {
			ctx := buildIndexTemplateContext(g, files, map[string][]docs.DocFile{repoName: files}, frontMatter)
			ctx["Sections"] = sectionGroups
			tpl, err := template.New("repo_index").Funcs(template.FuncMap{"titleCase": titleCase}).Parse(tplRaw)
			if err != nil { return fmt.Errorf("parse repository index template: %w", err) }
			var buf bytes.Buffer
			if err := tpl.Execute(&buf, ctx); err != nil { return fmt.Errorf("exec repository index template: %w", err) }
			body := buf.String()
			if !strings.HasPrefix(body, "---\n") { content = fmt.Sprintf("---\n%s---\n\n%s", string(fmData), body) } else { content = body }
		} else {
			content = fmt.Sprintf("---\n%s---\n\n# %s Documentation\n\n", string(fmData), titleCase(repoName))
			for section, sectionFiles := range sectionGroups {
				if section == "root" { content += "## Documentation Files\n\n" } else { content += fmt.Sprintf("## %s\n\n", titleCase(section)) }
				for _, file := range sectionFiles {
					title := titleCase(strings.ReplaceAll(file.Name, "-", " "))
					var relativePath string
					if file.Section != "" { relativePath = filepath.Join(file.Section, file.Name) } else { relativePath = file.Name }
					relativePath = filepath.ToSlash(relativePath) + "/"
					content += fmt.Sprintf("- [%s](./%s)\n", title, relativePath)
				}
				content += "\n"
			}
		}
		if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil { return fmt.Errorf("failed to write repository index: %w", err) }
		slog.Debug("Generated repository index", logfields.Repository(repoName), logfields.Path(indexPath))
	}
	return nil
}

func (g *Generator) generateSectionIndexes(docFiles []docs.DocFile) error {
	sectionGroups := make(map[string]map[string][]docs.DocFile)
	for _, file := range docFiles {
		if file.Section == "" {
			continue
		}
		if sectionGroups[file.Repository] == nil {
			sectionGroups[file.Repository] = make(map[string][]docs.DocFile)
		}
		sectionGroups[file.Repository][file.Section] = append(sectionGroups[file.Repository][file.Section], file)
	}
	for repoName, sections := range sectionGroups {
		for sectionName, files := range sections {
			indexPath := filepath.Join(g.buildRoot(), "content", repoName, sectionName, "_index.md")
			if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
			}
			frontMatter := map[string]any{"title": fmt.Sprintf("%s - %s", titleCase(repoName), titleCase(sectionName)), "repository": repoName, "section": sectionName, "date": time.Now().Format("2006-01-02T15:04:05-07:00")}
			fmData, err := yaml.Marshal(frontMatter); if err != nil { return fmt.Errorf("failed to marshal front matter: %w", err) }
			var content string
			if tplRaw, ok := g.config.Hugo.Params["section_index_template"].(string); ok && strings.TrimSpace(tplRaw) != "" {
				ctx := buildIndexTemplateContext(g, files, map[string][]docs.DocFile{repoName: files}, frontMatter)
				ctx["SectionName"] = sectionName
				ctx["Files"] = files
				tpl, err := template.New("section_index").Funcs(template.FuncMap{"titleCase": titleCase}).Parse(tplRaw)
				if err != nil { return fmt.Errorf("parse section index template: %w", err) }
				var buf bytes.Buffer
				if err := tpl.Execute(&buf, ctx); err != nil { return fmt.Errorf("exec section index template: %w", err) }
				body := buf.String()
				if !strings.HasPrefix(body, "---\n") { content = fmt.Sprintf("---\n%s---\n\n%s", string(fmData), body) } else { content = body }
			} else {
				content = fmt.Sprintf("---\n%s---\n\n# %s\n\n", string(fmData), titleCase(sectionName))
				for _, file := range files {
					title := titleCase(strings.ReplaceAll(file.Name, "-", " "))
					content += fmt.Sprintf("- [%s](./%s/)\n", title, file.Name)
				}
			}
			if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write section index: %w", err)
			}
			slog.Debug("Generated section index", logfields.Repository(repoName), logfields.Section(sectionName), logfields.Path(indexPath))
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
