package pipeline

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// generateMainIndex creates the site root _index.md if it doesn't exist.
func generateMainIndex(ctx *GenerationContext) ([]*Document, error) {
	// In daemon public-only mode, publish an empty site when no public pages exist.
	if ctx.Config.IsDaemonPublicOnlyEnabled() && len(ctx.Discovered) == 0 {
		return nil, nil
	}

	// Check if root index already exists
	var readme *Document
	kept := make([]*Document, 0, len(ctx.Discovered))
	for _, doc := range ctx.Discovered {
		if doc.Path == contentIndexPath || doc.Path == contentLegacyIndexPath {
			return nil, nil // Already exists
		}
		// A README of the root is the index of the site: the transform
		// phase renames it to _index, and the generated index is
		// written to the same path. The content of the README would be
		// lost: it becomes the content of the landing page.
		if filepath.Dir(doc.Path) == "content" && strings.EqualFold(doc.Name, "README") {
			readme = doc
			continue
		}
		kept = append(kept, doc)
	}
	if readme != nil {
		ctx.Discovered = kept
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

	// Use a deterministic epoch date so reproducible builds don't churn
	// the fingerprint across runs. The fixers in the lint pipeline
	// (applyFingerprintFixes) will refresh this once the doc is written
	// by a user; the auto-generated index is owned by DocBuilder and
	// uses a fixed value to avoid spurious drift reports.
	frontMatter := map[string]any{
		"title":                   title,
		frontMatterKeyDescription: description,
		"date":                    "2024-01-01T00:00:00Z",
		"lastmod":                 "2024-01-01",
		"type":                    "docs",
		"categories":              []string{"documentation"},
		"tags":                    []string{},
	}
	content := indexContent(title, description)
	if readme != nil {
		// The landing page of the site is the page of the portal, with
		// the title and the description of the configuration, and the
		// words of the repository.
		content += readme.Content
	}
	doc := &Document{
		Path:        contentIndexPath,
		IsIndex:     true,
		Generated:   true,
		Content:     content,
		FrontMatter: frontMatter,
		Repository:  "",
		Section:     "",
	}
	if ctx.Config.IsDaemonPublicOnlyEnabled() {
		doc.FrontMatter["public"] = true
	}

	return []*Document{doc}, nil
}

// repositoryKey identifies a repository in the content tree.
//
// The name alone is not enough: two repositories can share a name, the
// same name on two forges. The forge is part of the identity, because
// the forge is a directory of the content path.
type repositoryKey struct {
	forge      string
	repository string
}

// generateRepositoryIndex creates _index.md for repositories that don't have one.
func generateRepositoryIndex(ctx *GenerationContext) ([]*Document, error) {
	// Skip repository indexes entirely for single-repository builds
	if ctx.IsSingleRepo {
		return nil, nil
	}

	// Group documents by repository
	repoFiles := make(map[repositoryKey][]*Document)
	for _, doc := range ctx.Discovered {
		if doc.Repository != "" {
			key := repositoryKey{forge: doc.Forge, repository: doc.Repository}
			repoFiles[key] = append(repoFiles[key], doc)
		}
	}

	var generated []*Document

	for key, repoDocs := range repoFiles {
		// Check if repository index already exists
		hasIndex := false
		for _, doc := range repoDocs {
			if doc.IsIndex && doc.Section == "" {
				hasIndex = true
				break
			}
		}

		if !hasIndex {
			// Generate repository index
			title := titleCase(key.repository)
			description := fmt.Sprintf("Documentation for %s", key.repository)

			// Build the content path via the single source of truth
			// (docs.HugoContentPath) so this stage agrees with the
			// doc-copy stage on the case of every path component. The
			// forge is the one of the documents: the index belongs
			// next to them, and the metadata of a name cannot say
			// which of the repositories of that name it describes.
			doc := &Document{
				Path:       docs.HugoContentPath(key.forge, key.repository, "", indexFileSuffix, markdownExtension, ctx.IsSingleRepo),
				IsIndex:    true,
				Generated:  true,
				Repository: key.repository,
				Forge:      key.forge,
				Section:    "",
				Content:    indexContent(title, description),
				FrontMatter: map[string]any{
					"title":                   title,
					frontMatterKeyDescription: description,
					"type":                    "docs",
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

// sectionKey identifies a section in the content tree.
//
// The forge is part of the identity: the same repository name on two
// forges has two trees, and a section of one is not a section of the
// other.
type sectionKey struct {
	forge      string
	repository string
	section    string
}

// generateSectionIndex creates _index.md for sections that don't have one.
// Ensures all intermediate directories in deep hierarchies get index files.
func generateSectionIndex(ctx *GenerationContext) ([]*Document, error) {
	// Collect all unique section paths (including intermediate directories)
	allSections := make(map[sectionKey]bool)

	for _, doc := range ctx.Discovered {
		if doc.Section != "" {
			key := sectionKey{forge: doc.Forge, repository: doc.Repository, section: doc.Section}
			allSections[key] = true

			// Add all intermediate parent directories
			parts := strings.Split(doc.Section, string(filepath.Separator))
			for i := 1; i < len(parts); i++ {
				parent := sectionKey{
					forge:      doc.Forge,
					repository: doc.Repository,
					section:    strings.Join(parts[:i], string(filepath.Separator)),
				}
				allSections[parent] = true
			}
		}
	}

	// Check which sections already have indexes
	existingIndexes := make(map[sectionKey]bool)
	for _, doc := range ctx.Discovered {
		if doc.IsIndex && doc.Section != "" {
			key := sectionKey{forge: doc.Forge, repository: doc.Repository, section: doc.Section}
			existingIndexes[key] = true
		}
	}

	generated := make([]*Document, 0)

	// The documents that publish at a page of their own. A document
	// with the name of a section publishes at the page of that section.
	documentsAtPage := make(map[string]bool, len(ctx.Discovered))
	for _, doc := range ctx.Discovered {
		documentsAtPage[strings.ToLower(doc.Path)] = true
	}

	// Generate indexes for missing sections

	for key := range allSections {
		// Skip if index already exists
		if existingIndexes[key] {
			continue
		}

		repo := key.repository
		sectionName := key.section

		// Get repository metadata
		repoMeta := ctx.RepositoryMetadata[repo]

		// Generate section index
		// If section is a configured docs path, use repository name as title
		// Otherwise, use the base directory name as-is (without titleCase transformation)
		title := filepath.Base(sectionName)
		if isConfiguredDocsPath(sectionName, repoMeta.DocsPaths) {
			title = repoMeta.Name // Use actual repository name from config
			if title == "" {
				title = repo
			}
		}
		description := fmt.Sprintf("Documentation for %s", sectionName)

		// Build section path via the single source of truth
		// (docs.HugoContentPath) so this stage agrees with the doc-copy
		// stage on the case of every path component. The helper handles
		// the single-repo skip and the forge namespacing internally. The
		// forge is the one of the documents: the index belongs next to
		// them, and the metadata of a name cannot say which of the
		// repositories of that name it describes.
		sectionPath := docs.HugoContentPath(key.forge, repo, sectionName, indexFileSuffix, markdownExtension, ctx.IsSingleRepo)

		// The index publishes at the page of the section, and a
		// document with the name of the section publishes at the same
		// page. Hugo keeps one of the two and says nothing about the
		// other: the index would be a file in the content tree that
		// publishes nowhere, and it can take the page of the document
		// away. The document is the page of the section.
		sectionDir := filepath.Dir(sectionPath)
		documentAtPage := filepath.Join(filepath.Dir(sectionDir), filepath.Base(sectionDir)+markdownExtension)
		if documentsAtPage[strings.ToLower(documentAtPage)] {
			slog.Warn("The section has a document with the same name: the document is the page of the section",
				slog.String("document", documentAtPage),
				slog.String("section", sectionPath))
			continue
		}

		// Use a deterministic epoch date so reproducible builds don't churn
		// the fingerprint across runs. Auto-generated section indexes are
		// owned by DocBuilder and use a fixed value (mirroring generateMainIndex
		// above) instead of time.Now() — otherwise today's date bleeds into
		// rendered Hugo output on every build.
		doc := &Document{
			Path:       sectionPath,
			IsIndex:    true,
			Generated:  true,
			Repository: repo,
			Forge:      key.forge,
			Section:    sectionName,
			Content:    indexContent(title, description),
			FrontMatter: map[string]any{
				"title":       title,
				"description": description,
				"type":        "docs",
				"date":        "2024-01-01T00:00:00Z",
				"lastmod":     "2024-01-01",
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
