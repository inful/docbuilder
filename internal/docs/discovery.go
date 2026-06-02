package docs

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/docs/errors"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

const (
	markdownExtension = ".md"
	readmeFilename    = "README.md"
	imageExtensionPng = ".png"
)

// DocFile represents a discovered documentation file or asset.
type DocFile struct {
	Path             string            // Absolute path to the file
	RelativePath     string            // Path relative to the docs directory
	DocsBase         string            // The configured docs base path for this repo (e.g., "docs" or ".")
	Repository       string            // Repository name
	Group            string            // GitLab/GitHub group for collision resolution
	Forge            string            // Optional forge namespace (empty when single or not namespaced)
	Section          string            // Documentation section/directory
	Name             string            // File name without extension
	Extension        string            // File extension
	Content          []byte            // File content (loaded on demand)
	TransformedBytes []byte            // Content after transform pipeline (populated during copyContentFiles)
	Metadata         map[string]string // Additional metadata from config
	IsAsset          bool              // True for images and other non-markdown files
}

// Discovery handles documentation file discovery.
type Discovery struct {
	repositories map[string]config.Repository
	docFiles     []DocFile
	buildConfig  *config.BuildConfig
	isSingleRepo bool // True when building a single repository (skip repo namespace)
}

// NewDiscovery creates a new documentation discovery instance.
func NewDiscovery(repositories []config.Repository, buildCfg *config.BuildConfig) *Discovery {
	repoMap := make(map[string]config.Repository)
	for i := range repositories {
		repo := &repositories[i]
		repoMap[repo.Name] = *repo
	}

	return &Discovery{
		repositories: repoMap,
		docFiles:     make([]DocFile, 0),
		buildConfig:  buildCfg,
	}
}

// DiscoverDocs finds all documentation files in the specified repositories.
func (d *Discovery) DiscoverDocs(repoPaths map[string]string) ([]DocFile, error) {
	d.docFiles = make([]DocFile, 0)

	// Determine if this is a single-repository build
	d.isSingleRepo = len(repoPaths) == 1

	// Determine forge namespacing policy using global build config.
	mode := config.NamespacingAuto
	if d.buildConfig != nil && d.buildConfig.NamespaceForges != "" {
		mode = d.buildConfig.NamespaceForges
	}
	forgeCount := 0
	forgeSeen := map[string]struct{}{}
	//nolint:gocritic // rangeValCopy: d.repositories is a map - cannot index map values in Go, must use value iteration
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

	// Determine which repository names collide (case-insensitive). Group
	// is an *opt-in* collision-resolution segment: it only appears in the
	// content path when two repositories would otherwise produce the same
	// directory. Computing this set up front keeps DocFile.Group empty
	// for the common, non-colliding case so the content tree is not
	// polluted with a `<group>/` directory that is not needed for
	// disambiguation.
	collidingRepos := d.findCollidingRepositoryNames(repoPaths)
	if err := d.validateGroupCoverage(repoPaths, collidingRepos); err != nil {
		return nil, err
	}

	for repoName, repoPath := range repoPaths {
		repo, exists := d.repositories[repoName]
		if !exists {
			slog.Warn("Repository configuration not found", logfields.Name(repoName))
			continue
		}

		filesBeforeRepo := len(d.docFiles)

		// Check for .docignore file in repository root
		if hasDocIgnore, err := d.checkDocIgnore(repoPath); err != nil {
			slog.Warn("Failed to check .docignore", slog.String("repository", repoName), logfields.Error(err))
		} else if hasDocIgnore {
			slog.Info("Repository filtered during docs discovery",
				logfields.Repository(repoName),
				slog.String("forge", repo.Tags["forge_type"]),
				slog.String("reason", "docignore_present"))
			continue
		}

		slog.Info("Discovering documentation", logfields.Repository(repoName), slog.Any("paths", repo.Paths))

		forgeNS := ""
		if namespaceForges {
			// Use Namespace as alias for forge_type (preferred)
			if repo.Namespace != "" {
				forgeNS = repo.Namespace
			} else if ft, ok := repo.Tags["forge_type"]; ok && ft != "" {
				forgeNS = ft
			}
		}
		missingDocsPaths := 0
		for _, docsPath := range repo.Paths {
			fullDocsPath := filepath.Join(repoPath, docsPath)

			if _, err := os.Stat(fullDocsPath); os.IsNotExist(err) {
				missingDocsPaths++
				slog.Warn("Documentation path not found",
					logfields.Repository(repoName),
					logfields.Path(docsPath),
					slog.String("full_path", fullDocsPath))
				continue
			}

			// Group is opt-in: only include it in the content path when
			// the repository name collides with another repository. For
			// the common non-colliding case, the group segment would
			// otherwise turn into a spurious top-level directory under
			// content/.
			group := ""
			if collidingRepos[strings.ToLower(repoName)] {
				group = repo.Group
			}
			files, err := d.walkDocsDirectory(fullDocsPath, repoName, forgeNS, group, docsPath, repo.Tags)
			if err != nil {
				return nil, errors.WrapError(err, errors.CategoryDocs, "documentation directory walk failed").
					WithContext("path", docsPath).
					WithContext("repository", repoName).
					WithCause(derrors.ErrDocsDirWalkFailed).
					Build()
			}

			d.docFiles = append(d.docFiles, files...)
		}

		filesAfterRepo := len(d.docFiles)
		if filesAfterRepo == filesBeforeRepo {
			reason := "no_docs_files_found"
			if len(repo.Paths) > 0 && missingDocsPaths == len(repo.Paths) {
				reason = "docs_paths_missing"
			}
			slog.Info("Repository filtered during docs discovery",
				logfields.Repository(repoName),
				slog.String("forge", repo.Tags["forge_type"]),
				slog.String("reason", reason),
				slog.Any("paths", repo.Paths))
		}
		slog.Info("Documentation discovered", logfields.Repository(repoName), slog.Int("files", len(d.docFiles)))
	}

	slog.Info("Total documentation files discovered", slog.Int("count", len(d.docFiles)))

	// Detect case-insensitive path collisions
	if err := d.detectPathCollisions(); err != nil {
		return d.docFiles, err
	}

	return d.docFiles, nil
}

// walkDocsDirectory recursively walks a documentation directory.
func (d *Discovery) walkDocsDirectory(docsPath, repoName, forgeNS, group, relativePath string, metadata map[string]string) ([]DocFile, error) {
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
			return errors.WrapError(err, errors.CategoryDocs, "failed to calculate relative path").
				WithContext("docs_path", docsPath).
				WithContext("file_path", path).
				WithCause(derrors.ErrInvalidRelativePath).
				Build()
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
			Group:        group,
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

// LoadContent loads the content of a documentation file.
func (df *DocFile) LoadContent() error {
	if df.Content != nil {
		return nil // Already loaded
	}

	content, err := os.ReadFile(df.Path)
	if err != nil {
		return errors.WrapError(err, errors.CategoryDocs, "failed to read documentation file").
			WithContext("path", df.Path).
			WithCause(derrors.ErrFileReadFailed).
			Build()
	}

	df.Content = content
	return nil
}

// GetHugoPath returns the Hugo-compatible path for this documentation file.
func (df *DocFile) GetHugoPath(isSingleRepo bool) string {
	return HugoContentPath(df.Forge, df.Group, df.Repository, df.Section, df.Name, df.Extension, isSingleRepo)
}

// HugoContentPath returns the Hugo content path for a file described by its
// component parts. This is the single source of truth for the case-normalized
// content-tree path used throughout the pipeline and the index generators.
//
// Path shapes:
//
//	Single repository:            content/{section}/{name}{ext}
//	With Group:                   content/{group}/{repository}/{section}/{name}{ext}
//	With Forge:                   content/{forge}/{group}/{repository}/{section}/{name}{ext}
//	Multiple repos, single forge: content/{repository}/{section}/{name}{ext}
//	Multiple forges:              content/{forge}/{repository}/{section}/{name}{ext}
//
// All non-empty components are lowercased. The filename is rewritten to
// "_index" when the leaf name is "index" (so user-provided index.md becomes
// Hugo's _index.md convention). The result is always rooted at "content/".
//
// All code that writes a file into the content/ tree MUST go through this
// helper (or DocFile.GetHugoPath, which delegates to it) to guarantee that
// the doc-copy stage and the index generators agree on the case of the
// directory. Otherwise case-insensitive filesystems alias "Drift" and
// "drift" to the same directory and Hugo produces doubled publish paths.
func HugoContentPath(forge, group, repository, section, name, ext string, isSingleRepo bool) string {
	parts := []string{"content"}
	if forge != "" {
		parts = append(parts, strings.ToLower(forge))
	}

	// Include Group if present for collision resolution. Group is a
	// GitLab/GitHub organization segment that disambiguates repos
	// sharing a name across different orgs.
	if group != "" {
		parts = append(parts, strings.ToLower(group))
	}

	if !isSingleRepo && repository != "" {
		parts = append(parts, strings.ToLower(repository))
	}

	if section != "" {
		parts = append(parts, strings.ToLower(section))
	}

	// Convert user-provided index.md to _index.md for Hugo section pages
	filename := strings.ToLower(name)
	if filename == "index" {
		filename = "_index"
	}

	// Lowercase the filename for URL compatibility
	parts = append(parts, filename+ext)
	return filepath.Join(parts...)
}

// isMarkdownFile checks if a file is a markdown file.
func isMarkdownFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == markdownExtension || ext == ".markdown" || ext == ".mdown" || ext == ".mkd"
}

// isAsset checks if a file is an asset (image, etc.)
func isAsset(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	assetExtensions := []string{
		// Images
		imageExtensionPng, ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".ico",
		// Documents
		".pdf",
		// Video
		".mp4", ".webm", ".ogv",
		// Other
		".csv", ".json", ".yaml", ".yml", ".xml",
	}
	return slices.Contains(assetExtensions, ext)
}

// isIgnoredFile checks if a file should be ignored.
func isIgnoredFile(filename string) bool {
	ignored := []string{
		readmeFilename,    // Usually repository readme, not docs
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

// copyMetadata creates a copy of metadata map.
func copyMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}

	copyMap := make(map[string]string)
	maps.Copy(copyMap, metadata)

	return copyMap
}

// GetDocFiles returns all discovered documentation files.
func (d *Discovery) GetDocFiles() []DocFile {
	return d.docFiles
}

// GetDocFilesByRepository returns documentation files grouped by repository.
func (d *Discovery) GetDocFilesByRepository() map[string][]DocFile {
	result := make(map[string][]DocFile)
	for i := range d.docFiles {
		file := &d.docFiles[i]
		key := file.Repository
		if file.Forge != "" {
			key = file.Forge + "/" + key
		}
		result[key] = append(result[key], *file)
	}
	return result
}

// GetDocFilesBySection returns documentation files grouped by section.
func (d *Discovery) GetDocFilesBySection() map[string][]DocFile {
	result := make(map[string][]DocFile)
	for i := range d.docFiles {
		file := &d.docFiles[i]
		repoKey := file.Repository
		if file.Forge != "" {
			repoKey = file.Forge + "/" + repoKey
		}
		key := repoKey + "/" + file.Section
		result[key] = append(result[key], *file)
	}
	return result
}

// checkDocIgnore checks if a repository has a .docignore file in its root.
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

	return false, errors.WrapError(err, errors.CategoryDocs, "docignore check failed").
		WithContext("path", docIgnorePath).
		WithCause(derrors.ErrDocIgnoreCheckFailed).
		Build()
}

// detectPathCollisions checks for case-insensitive Hugo path collisions.
// Returns an error if duplicate paths are found that would cause ambiguous page references in Hugo.
func (d *Discovery) detectPathCollisions() error {
	// Track Hugo path -> source file paths
	seen := make(map[string][]string)

	for i := range d.docFiles {
		file := &d.docFiles[i]
		hugoPath := file.GetHugoPath(d.isSingleRepo)
		sourcePath := filepath.Join(file.Repository, file.RelativePath)
		seen[hugoPath] = append(seen[hugoPath], sourcePath)
	}

	// Check for collisions
	var collisions []string
	for hugoPath, sourcePaths := range seen {
		if len(sourcePaths) > 1 {
			// Remove duplicates (shouldn't happen, but be defensive)
			unique := make(map[string]struct{})
			for _, sp := range sourcePaths {
				unique[sp] = struct{}{}
			}

			if len(unique) > 1 {
				// True collision - different source files map to same Hugo path
				sources := make([]string, 0, len(unique))
				for sp := range unique {
					sources = append(sources, sp)
				}
				collisions = append(collisions, fmt.Sprintf("  Hugo path: %s\n    Source files: %v", hugoPath, sources))
			}
		}
	}

	if len(collisions) > 0 {
		slog.Warn("Case-insensitive path collisions detected",
			slog.Int("count", len(collisions)))

		for _, collision := range collisions {
			slog.Warn("Path collision", slog.String("details", collision))
		}

		return errors.DocsError("case-insensitive path collision(s) detected").
			WithContext("count", len(collisions)).
			WithContext("details", strings.Join(collisions, "\n")).
			WithContext("hint", "rename or remove conflicting files in source repositories").
			WithCause(derrors.ErrPathCollision).
			Build()
	}

	return nil
}

// IsSingleRepo returns true if this discovery was for a single repository build.
// Used by Hugo generator to determine if repository namespace should be skipped in paths.
func (d *Discovery) IsSingleRepo() bool {
	return d.isSingleRepo
}

// findCollidingRepositoryNames returns the set of lowercased repository
// names that appear more than once in the active build. The returned map
// is keyed by lowercased name so callers can do case-insensitive
// lookups. Repositories whose names do not collide never need a group
// segment in their content path; including one would add a spurious
// top-level sidebar entry in the rendered Hugo site.
func (d *Discovery) findCollidingRepositoryNames(repoPaths map[string]string) map[string]bool {
	seen := make(map[string]string) // lowercased name -> original (first occurrence)
	colliding := make(map[string]bool)
	for repoName := range repoPaths {
		// Skip repos that discovery has already filtered out (e.g. via
		// .docignore) - they are not in repoPaths, so they are absent
		// here and do not contribute to collisions.
		key := strings.ToLower(repoName)
		if _, exists := seen[key]; exists {
			colliding[key] = true
			continue
		}
		seen[key] = repoName
	}
	return colliding
}

// validateGroupCoverage returns an error if any repository whose name
// collides with another repository does not have a Group set. Without
// a group, the two colliding repos would land at the same content path
// and the case-insensitive path-collision check downstream would fail
// with a less actionable error. Surfacing the misconfiguration here
// gives the user a clear remediation: set `group:` on the colliding
// repositories.
func (d *Discovery) validateGroupCoverage(repoPaths map[string]string, collidingRepos map[string]bool) error {
	var missing []string
	for repoName := range repoPaths {
		key := strings.ToLower(repoName)
		if !collidingRepos[key] {
			continue
		}
		repo, ok := d.repositories[repoName]
		if !ok {
			continue
		}
		if strings.TrimSpace(repo.Group) == "" {
			missing = append(missing, repoName)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return errors.DocsError("colliding repositories are missing 'group' for disambiguation").
		WithContext("repositories", missing).
		WithContext("hint", "set a distinct 'group' value on each colliding repository in the config so the content tree can disambiguate them").
		WithCause(derrors.ErrPathCollision).
		Build()
}
