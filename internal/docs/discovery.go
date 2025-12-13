package docs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/docs/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// DocFile represents a discovered documentation file or asset
type DocFile struct {
	Path             string            // Absolute path to the file
	RelativePath     string            // Path relative to the docs directory
	DocsBase         string            // The configured docs base path for this repo (e.g., "docs" or ".")
	Repository       string            // Repository name
	Forge            string            // Optional forge namespace (empty when single or not namespaced)
	Section          string            // Documentation section/directory
	Name             string            // File name without extension
	Extension        string            // File extension
	Content          []byte            // File content (loaded on demand)
	TransformedBytes []byte            // Content after transform pipeline (populated during copyContentFiles)
	Metadata         map[string]string // Additional metadata from config
	IsAsset          bool              // True for images and other non-markdown files
}

// Discovery handles documentation file discovery
type Discovery struct {
	repositories map[string]config.Repository
	docFiles     []DocFile
	buildConfig  *config.BuildConfig
}

// NewDiscovery creates a new documentation discovery instance
func NewDiscovery(repositories []config.Repository, buildCfg *config.BuildConfig) *Discovery {
	repoMap := make(map[string]config.Repository)
	for _, repo := range repositories {
		repoMap[repo.Name] = repo
	}

	return &Discovery{
		repositories: repoMap,
		docFiles:     make([]DocFile, 0),
		buildConfig:  buildCfg,
	}
}

// DiscoverDocs finds all documentation files in the specified repositories
func (d *Discovery) DiscoverDocs(repoPaths map[string]string) ([]DocFile, error) {
	d.docFiles = make([]DocFile, 0)

	// Determine forge namespacing policy using global build config.
	mode := config.NamespacingAuto
	if d.buildConfig != nil && d.buildConfig.NamespaceForges != "" {
		mode = d.buildConfig.NamespaceForges
	}
	forgeCount := 0
	forgeSeen := map[string]struct{}{}
	for _, r := range d.repositories {
		if ft, ok := r.Tags["forge_type"]; ok && ft != "" {
			if _, exists := forgeSeen[ft]; !exists {
				forgeSeen[ft] = struct{}{}
				forgeCount++
			}
		}
	}
	namespaceForges := false
	switch mode {
	case config.NamespacingAlways:
		namespaceForges = true
	case config.NamespacingNever:
		namespaceForges = false
	case config.NamespacingAuto:
		namespaceForges = forgeCount > 1
	}

	for repoName, repoPath := range repoPaths {
		repo, exists := d.repositories[repoName]
		if !exists {
			slog.Warn("Repository configuration not found", logfields.Name(repoName))
			continue
		}

		// Check for .docignore file in repository root
		if hasDocIgnore, err := d.checkDocIgnore(repoPath); err != nil {
			slog.Warn("Failed to check .docignore", slog.String("repository", repoName), logfields.Error(err))
		} else if hasDocIgnore {
			slog.Info("Skipping repository due to .docignore file", slog.String("repository", repoName))
			continue
		}

		slog.Info("Discovering documentation", logfields.Repository(repoName), slog.Any("paths", repo.Paths))

		forgeNS := ""
		if namespaceForges {
			forgeNS = repo.Tags["forge_type"]
		}
		for _, docsPath := range repo.Paths {
			fullDocsPath := filepath.Join(repoPath, docsPath)

			if _, err := os.Stat(fullDocsPath); os.IsNotExist(err) {
				slog.Warn("Documentation path not found",
					logfields.Repository(repoName),
					logfields.Path(docsPath),
					slog.String("full_path", fullDocsPath))
				continue
			}

			files, err := d.walkDocsDirectory(fullDocsPath, repoName, forgeNS, docsPath, repo.Tags)
			if err != nil {
				return nil, fmt.Errorf("%w: %s in %s: %w", derrors.ErrDocsDirWalkFailed, docsPath, repoName, err)
			}

			d.docFiles = append(d.docFiles, files...)
		}

		slog.Info("Documentation discovered", logfields.Repository(repoName), slog.Int("files", len(d.docFiles)))
	}

	slog.Info("Total documentation files discovered", slog.Int("count", len(d.docFiles)))
	return d.docFiles, nil
}

// walkDocsDirectory recursively walks a documentation directory
func (d *Discovery) walkDocsDirectory(docsPath, repoName, forgeNS, relativePath string, metadata map[string]string) ([]DocFile, error) {
	var files []DocFile

	err := filepath.Walk(docsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's a markdown file or an asset
		isMarkdown := isMarkdownFile(path)
		isAssetFile := isAsset(path)

		// Skip files that are neither markdown nor assets
		if !isMarkdown && !isAssetFile {
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Calculate relative path from docs directory
		relPath, err := filepath.Rel(docsPath, path)
		if err != nil {
			return fmt.Errorf("%w: %w", derrors.ErrInvalidRelativePath, err)
		}

		// Determine section from directory structure
		section := filepath.Dir(relPath)
		if section == "." {
			section = "" // Root level
		}

		// Only ignore certain files at the root level (but keep README.md for use as repository index)
		if section == "" && isIgnoredFile(info.Name()) && !strings.EqualFold(info.Name(), "README.md") {
			return nil
		}

		docFile := DocFile{
			Path:         path,
			RelativePath: relPath,
			DocsBase:     relativePath,
			Repository:   repoName,
			Forge:        forgeNS,
			Section:      section,
			Name:         strings.TrimSuffix(info.Name(), filepath.Ext(info.Name())),
			Extension:    filepath.Ext(info.Name()),
			Metadata:     copyMetadata(metadata),
			IsAsset:      isAssetFile,
		}

		files = append(files, docFile)

		fileType := "documentation"
		if isAssetFile {
			fileType = "asset"
		}
		slog.Debug("Discovered file",
			logfields.File(relPath),
			logfields.Repository(repoName),
			slog.String("section", section),
			slog.String("type", fileType))

		return nil
	})

	return files, err
}

// LoadContent loads the content of a documentation file
func (df *DocFile) LoadContent() error {
	if df.Content != nil {
		return nil // Already loaded
	}

	content, err := os.ReadFile(df.Path)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", derrors.ErrFileReadFailed, df.Path, err)
	}

	df.Content = content
	return nil
}

// GetHugoPath returns the Hugo-compatible path for this documentation file
func (df *DocFile) GetHugoPath() string {
	// Path shape:
	//   Single forge (no namespace): content/{repository}/{section}/{name}.md
	//   Multiple forges:             content/{forge}/{repository}/{section}/{name}.md
	parts := []string{"content"}
	if df.Forge != "" {
		parts = append(parts, strings.ToLower(df.Forge))
	}
	parts = append(parts, strings.ToLower(df.Repository))

	if df.Section != "" {
		parts = append(parts, strings.ToLower(df.Section))
	}

	// Convert user-provided index.md to _index.md for Hugo section pages
	filename := strings.ToLower(df.Name)
	if filename == "index" {
		filename = "_index"
	}

	// Lowercase the filename for URL compatibility
	parts = append(parts, filename+df.Extension)
	return filepath.Join(parts...)
}

// isMarkdownFile checks if a file is a markdown file
func isMarkdownFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown" || ext == ".mdown" || ext == ".mkd"
}

// isAsset checks if a file is an asset (image, etc.)
func isAsset(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	assetExtensions := []string{
		// Images
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico",
		// Documents
		".pdf",
		// Video
		".mp4", ".webm", ".ogv",
		// Other
		".csv", ".json", ".yaml", ".yml", ".xml",
	}
	for _, assetExt := range assetExtensions {
		if ext == assetExt {
			return true
		}
	}
	return false
}

// isIgnoredFile checks if a file should be ignored
func isIgnoredFile(filename string) bool {
	ignored := []string{
		"README.md",       // Usually repository readme, not docs
		"CONTRIBUTING.md", // Contributing guidelines
		"CHANGELOG.md",    // Changelog
		"LICENSE.md",      // License file
	}

	for _, ignore := range ignored {
		if strings.EqualFold(filename, ignore) {
			return true
		}
	}

	return false
}

// copyMetadata creates a copy of metadata map
func copyMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}

	copyMap := make(map[string]string)
	for k, v := range metadata {
		copyMap[k] = v
	}

	return copyMap
}

// GetDocFiles returns all discovered documentation files
func (d *Discovery) GetDocFiles() []DocFile {
	return d.docFiles
}

// GetDocFilesByRepository returns documentation files grouped by repository
func (d *Discovery) GetDocFilesByRepository() map[string][]DocFile {
	result := make(map[string][]DocFile)
	for _, file := range d.docFiles {
		key := file.Repository
		if file.Forge != "" {
			key = file.Forge + "/" + key
		}
		result[key] = append(result[key], file)
	}
	return result
}

// GetDocFilesBySection returns documentation files grouped by section
func (d *Discovery) GetDocFilesBySection() map[string][]DocFile {
	result := make(map[string][]DocFile)
	for _, file := range d.docFiles {
		repoKey := file.Repository
		if file.Forge != "" {
			repoKey = file.Forge + "/" + repoKey
		}
		key := repoKey + "/" + file.Section
		result[key] = append(result[key], file)
	}
	return result
}

// checkDocIgnore checks if a repository has a .docignore file in its root
func (d *Discovery) checkDocIgnore(repoPath string) (bool, error) {
	docIgnorePath := filepath.Join(repoPath, ".docignore")

	_, err := os.Stat(docIgnorePath)
	if err == nil {
		slog.Debug("Found .docignore file", logfields.Path(docIgnorePath))
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("%w: %w", derrors.ErrDocIgnoreCheckFailed, err)
}
