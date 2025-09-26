package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
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
	frontMatter := map[string]interface{}{"title": g.config.Hugo.Title, "description": g.config.Hugo.Description, "date": time.Now().Format("2006-01-02T15:04:05-07:00"), "type": "docs"}
	if g.config.Hugo.Theme == config.ThemeHextra {
		frontMatter["cascade"] = map[string]interface{}{"type": "docs"}
	}
	fmData, err := yaml.Marshal(frontMatter)
	if err != nil {
		return fmt.Errorf("failed to marshal front matter: %w", err)
	}
	content := fmt.Sprintf("---\n%s---\n\n# %s\n\n%s\n\n", string(fmData), g.config.Hugo.Title, g.config.Hugo.Description)
	content += "## Repositories\n\n"
	for repoName, files := range repoGroups {
		content += fmt.Sprintf("- [%s](./%s/) (%d files)\n", repoName, repoName, len(files))
	}
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write main index: %w", err)
	}
	slog.Info("Generated main index page", "path", indexPath)
	return nil
}

func (g *Generator) generateRepositoryIndexes(docFiles []docs.DocFile) error {
	repoGroups := make(map[string][]docs.DocFile)
	for _, file := range docFiles {
		repoGroups[file.Repository] = append(repoGroups[file.Repository], file)
	}
	for repoName, files := range repoGroups {
		indexPath := filepath.Join(g.buildRoot(), "content", repoName, "_index.md")
		if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
		}
		frontMatter := map[string]interface{}{"title": fmt.Sprintf("%s Documentation", titleCase(repoName)), "repository": repoName, "date": time.Now().Format("2006-01-02T15:04:05-07:00")}
		fmData, err := yaml.Marshal(frontMatter)
		if err != nil {
			return fmt.Errorf("failed to marshal front matter: %w", err)
		}
		content := fmt.Sprintf("---\n%s---\n\n# %s Documentation\n\n", string(fmData), titleCase(repoName))
		sectionGroups := make(map[string][]docs.DocFile)
		for _, file := range files {
			section := file.Section
			if section == "" {
				section = "root"
			}
			sectionGroups[section] = append(sectionGroups[section], file)
		}
		for section, sectionFiles := range sectionGroups {
			if section == "root" {
				content += "## Documentation Files\n\n"
			} else {
				content += fmt.Sprintf("## %s\n\n", titleCase(section))
			}
			for _, file := range sectionFiles {
				title := titleCase(strings.ReplaceAll(file.Name, "-", " "))
				var relativePath string
				if file.Section != "" {
					relativePath = filepath.Join(file.Section, file.Name)
				} else {
					relativePath = file.Name
				}
				relativePath = filepath.ToSlash(relativePath) + "/"
				content += fmt.Sprintf("- [%s](./%s)\n", title, relativePath)
			}
			content += "\n"
		}
		if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write repository index: %w", err)
		}
		slog.Debug("Generated repository index", "repository", repoName, "path", indexPath)
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
			frontMatter := map[string]interface{}{"title": fmt.Sprintf("%s - %s", titleCase(repoName), titleCase(sectionName)), "repository": repoName, "section": sectionName, "date": time.Now().Format("2006-01-02T15:04:05-07:00")}
			fmData, err := yaml.Marshal(frontMatter)
			if err != nil {
				return fmt.Errorf("failed to marshal front matter: %w", err)
			}
			content := fmt.Sprintf("---\n%s---\n\n# %s\n\n", string(fmData), titleCase(sectionName))
			for _, file := range files {
				title := titleCase(strings.ReplaceAll(file.Name, "-", " "))
				content += fmt.Sprintf("- [%s](./%s/)\n", title, file.Name)
			}
			if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write section index: %w", err)
			}
			slog.Debug("Generated section index", "repository", repoName, "section", sectionName, "path", indexPath)
		}
	}
	return nil
}
