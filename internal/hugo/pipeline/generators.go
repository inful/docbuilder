package pipeline

import (
	"fmt"
	"path/filepath"
	"strings"
)

// generateMainIndex creates the site root _index.md if it doesn't exist.
func generateMainIndex(ctx *GenerationContext) ([]*Document, error) {
	// In daemon public-only mode, publish an empty site when no public pages exist.
	if ctx.Config.IsDaemonPublicOnlyEnabled() && len(ctx.Discovered) == 0 {
		return nil, nil
	}

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
		Path:      "content/_index.md",
		IsIndex:   true,
		Generated: true,
		Content:   fmt.Sprintf("# %s\n\n%s\n\n{{%% children description=\"true\" %%}}\n", title, description),
		FrontMatter: map[string]any{
			"title":       title,
			"description": description,
			"type":        "docs",
		},
		Repository: "",
		Section:    "",
	}
	if ctx.Config.IsDaemonPublicOnlyEnabled() {
		doc.FrontMatter["public"] = true
	}

	return []*Document{doc}, nil
}

// generateRepositoryIndex creates _index.md for repositories that don't have one.
func generateRepositoryIndex(ctx *GenerationContext) ([]*Document, error) {
	// Skip repository indexes entirely for single-repository builds
	if ctx.IsSingleRepo {
		return nil, nil
	}

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
				Path:       filepath.Join("content", repoPath, "_index.md"),
				IsIndex:    true,
				Generated:  true,
				Repository: repo,
				Forge:      repoMeta.Forge,
				Section:    "",
				Content:    fmt.Sprintf("# %s\n\n%s\n\n{{%% children description=\"true\" %%}}\n", title, description),
				FrontMatter: map[string]any{
					"title":       title,
					"description": description,
					"type":        "docs",
				},
			}
			if ctx.Config.IsDaemonPublicOnlyEnabled() {
				doc.FrontMatter["public"] = true
			}
			generated = append(generated, doc)
		}
	}

	return generated, nil
}

// generateSectionIndex creates _index.md for sections that don't have one.
// Ensures all intermediate directories in deep hierarchies get index files.
func generateSectionIndex(ctx *GenerationContext) ([]*Document, error) {
	// Collect all unique section paths (including intermediate directories)
	allSections := make(map[string]bool)

	for _, doc := range ctx.Discovered {
		if doc.Section != "" {
			section := filepath.Join(doc.Repository, doc.Section)

			// Add this section and all parent sections
			allSections[section] = true

			// Add all intermediate parent directories
			parts := strings.Split(section, string(filepath.Separator))
			for i := 2; i < len(parts); i++ {
				parentSection := filepath.Join(parts[:i]...)
				allSections[parentSection] = true
			}
		}
	}

	// Check which sections already have indexes
	existingIndexes := make(map[string]bool)
	for _, doc := range ctx.Discovered {
		if doc.IsIndex && doc.Section != "" {
			section := filepath.Join(doc.Repository, doc.Section)
			existingIndexes[section] = true
		}
	}

	generated := make([]*Document, 0)

	// Generate indexes for missing sections

	for section := range allSections {
		// Skip if index already exists
		if existingIndexes[section] {
			continue
		}

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
		// If section is a configured docs path, use repository name as title
		// Otherwise, use the base directory name as-is (without titleCase transformation)
		title := filepath.Base(sectionName)
		if isConfiguredDocsPath(sectionName, repoMeta.DocsPaths) {
			title = repoMeta.Name // Use actual repository name from config
		}
		description := fmt.Sprintf("Documentation for %s", sectionName)

		// Build section path (handle forge namespacing and single-repo mode)
		var sectionPath string
		if ctx.IsSingleRepo {
			// Single repository: skip repository namespace
			sectionPath = sectionName
		} else {
			// Multiple repositories: include repository in path
			sectionPath = filepath.Join(repo, sectionName)
			if repoMeta.Namespace != "" {
				sectionPath = filepath.Join(repoMeta.Namespace, repo, sectionName)
			}
		}

		doc := &Document{
			Path:       filepath.Join("content", sectionPath, "_index.md"),
			IsIndex:    true,
			Generated:  true,
			Repository: repo,
			Forge:      repoMeta.Forge,
			Section:    sectionName,
			Content:    fmt.Sprintf("# %s\n\n%s\n\n{{%% children description=\"true\" %%}}\n", title, description),
			FrontMatter: map[string]any{
				"title":       title,
				"description": description,
				"type":        "docs",
			},
		}
		if ctx.Config.IsDaemonPublicOnlyEnabled() {
			doc.FrontMatter["public"] = true
		}
		generated = append(generated, doc)
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

// isConfiguredDocsPath checks if a section matches a configured documentation path.
// This identifies top-level documentation directories that should use the repository name
// as their title instead of the directory name (e.g., "docs" → "Repository Name").
// Also handles nested docs directories (e.g., "docs/docs" → "Repository Name").
func isConfiguredDocsPath(sectionName string, docsPaths []string) bool {
	// Get the last segment of the section path
	lastSegment := filepath.Base(sectionName)

	for _, path := range docsPaths {
		// Exact match (e.g., section "docs" matches path "docs")
		if sectionName == path {
			return true
		}
		// Last segment matches a docs path (e.g., section "docs/docs" has last segment "docs")
		// This handles nested directories with the same name as the docs path
		if lastSegment == filepath.Base(path) {
			return true
		}
	}
	return false
}
