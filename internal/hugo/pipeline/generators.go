package pipeline

import (
	"fmt"
	"path/filepath"
	"strings"
)

// generateMainIndex creates the site root _index.md if it doesn't exist.
func generateMainIndex(ctx *GenerationContext) ([]*Document, error) {
	// Check if root index already exists
	for _, doc := range ctx.Discovered {
		if doc.Path == "content/_index.md" || doc.Path == "content/index.md" {
			return nil, nil // Already exists
		}
	}

	// Create main index
	title := "Documentation"
	if ctx.Config != nil && ctx.Config.Hugo.Title != "" {
		title = ctx.Config.Hugo.Title
	}

	description := "Documentation hub"
	if ctx.Config != nil && ctx.Config.Hugo.Description != "" {
		description = ctx.Config.Hugo.Description
	}

	doc := &Document{
		Path:        "content/_index.md",
		IsIndex:     true,
		Generated:   true,
		Content:     fmt.Sprintf("# %s\n\n%s\n", title, description),
		FrontMatter: map[string]any{
			"title":       title,
			"description": description,
			"type":        "docs",
		},
		Repository: "",
		Section:    "",
	}

	return []*Document{doc}, nil
}

// generateRepositoryIndex creates _index.md for repositories that don't have one.
func generateRepositoryIndex(ctx *GenerationContext) ([]*Document, error) {
	// Group documents by repository
	repoFiles := make(map[string][]*Document)
	for _, doc := range ctx.Discovered {
		if doc.Repository != "" {
			repoFiles[doc.Repository] = append(repoFiles[doc.Repository], doc)
		}
	}

	var generated []*Document

	for repo, docs := range repoFiles {
		// Check if repository index already exists
		hasIndex := false
		for _, doc := range docs {
			if doc.IsIndex && doc.Section == "" {
				hasIndex = true
				break
			}
		}

		if !hasIndex {
			// Generate repository index
			repoMeta := ctx.RepositoryMetadata[repo]
			title := titleCase(repo)
			description := fmt.Sprintf("Documentation for %s", repo)

			// Build repository path (handle forge namespacing)
			repoPath := repo
			if repoMeta.Namespace != "" {
				repoPath = filepath.Join(repoMeta.Namespace, repo)
			}

			doc := &Document{
				Path:        filepath.Join("content", repoPath, "_index.md"),
				IsIndex:     true,
				Generated:   true,
				Repository:  repo,
				Forge:       repoMeta.Forge,
				Section:     "",
				Content:     fmt.Sprintf("# %s\n\n%s\n", title, description),
				FrontMatter: map[string]any{
					"title":       title,
					"description": description,
					"type":        "docs",
				},
			}
			generated = append(generated, doc)
		}
	}

	return generated, nil
}

// generateSectionIndex creates _index.md for sections that don't have one.
func generateSectionIndex(ctx *GenerationContext) ([]*Document, error) {
	// Group documents by section
	sections := make(map[string][]*Document)
	for _, doc := range ctx.Discovered {
		if doc.Section != "" {
			section := filepath.Join(doc.Repository, doc.Section)
			sections[section] = append(sections[section], doc)
		}
	}

	var generated []*Document

	for section, docs := range sections {
		// Check if section index already exists
		hasIndex := false
		for _, doc := range docs {
			if doc.IsIndex {
				hasIndex = true
				break
			}
		}

		if !hasIndex {
			// Extract repository and section name
			parts := strings.SplitN(section, string(filepath.Separator), 2)
			if len(parts) != 2 {
				continue // Skip malformed sections
			}
			repo := parts[0]
			sectionName := parts[1]

			// Get repository metadata
			repoMeta := ctx.RepositoryMetadata[repo]

			// Generate section index
			title := titleCase(filepath.Base(sectionName))
			description := fmt.Sprintf("Documentation for %s", sectionName)

			// Build section path (handle forge namespacing)
			sectionPath := filepath.Join(repo, sectionName)
			if repoMeta.Namespace != "" {
				sectionPath = filepath.Join(repoMeta.Namespace, repo, sectionName)
			}

			doc := &Document{
				Path:        filepath.Join("content", sectionPath, "_index.md"),
				IsIndex:     true,
				Generated:   true,
				Repository:  repo,
				Forge:       repoMeta.Forge,
				Section:     sectionName,
				Content:     fmt.Sprintf("# %s\n\n%s\n", title, description),
				FrontMatter: map[string]any{
					"title":       title,
					"description": description,
					"type":        "docs",
				},
			}
			generated = append(generated, doc)
		}
	}

	return generated, nil
}

// titleCase converts a string to title case (simple version).
// Replaces dashes and underscores with spaces and capitalizes words.
func titleCase(s string) string {
	// Replace separators with spaces
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")

	// Capitalize first letter of each word
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}
