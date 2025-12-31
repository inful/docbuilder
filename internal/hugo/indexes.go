package hugo

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// sortDocFiles sorts a slice of DocFile by path for deterministic ordering.
func sortDocFiles(files []docs.DocFile) {
	sort.Slice(files, func(i, j int) bool {
		// Sort by repository, then section, then name
		if files[i].Repository != files[j].Repository {
			return files[i].Repository < files[j].Repository
		}
		if files[i].Section != files[j].Section {
			return files[i].Section < files[j].Section
		}
		return files[i].Name < files[j].Name
	})
}

// sortedSectionMap creates a deterministically ordered slice of section entries.
type sectionEntry struct {
	Name  string
	Files []docs.DocFile
}

func makeSortedSections(sectionGroups map[string][]docs.DocFile) []sectionEntry {
	sections := make([]sectionEntry, 0, len(sectionGroups))
	for name, files := range sectionGroups {
		sortDocFiles(files)
		sections = append(sections, sectionEntry{Name: name, Files: files})
	}
	sort.Slice(sections, func(i, j int) bool {
		// "root" section always comes first, then alphabetical
		if sections[i].Name == "root" {
			return true
		}
		if sections[j].Name == "root" {
			return false
		}
		return sections[i].Name < sections[j].Name
	})
	return sections
}

// sortedTagMap creates a deterministically ordered slice of tag entries.
type tagEntry struct {
	Key   string
	Value string
}

func makeSortedTags(tags map[string]string) []tagEntry {
	if tags == nil {
		return nil
	}
	entries := make([]tagEntry, 0, len(tags))
	for k, v := range tags {
		entries = append(entries, tagEntry{Key: k, Value: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries
}

// generateIndexPages creates index pages for sections and the main site.
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
	for i := range docFiles {
		file := &docFiles[i]
		repoGroups[file.Repository] = append(repoGroups[file.Repository], *file)
	}
	// Use fixed epoch date for reproducible builds (user can override via custom index.md)
	frontMatter := map[string]any{"title": g.config.Hugo.Title, "description": g.config.Hugo.Description, "date": "2024-01-01T00:00:00Z", "type": "docs"}
	// Add cascade for all themes to ensure type: docs propagates to children
	frontMatter["cascade"] = map[string]any{"type": "docs"}
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
	for i := range docFiles {
		file := &docFiles[i]
		// Only include markdown files in repository indexes, not assets
		if !file.IsAsset {
			repoGroups[file.Repository] = append(repoGroups[file.Repository], *file)
		}
	}
	for repoName, files := range repoGroups {
		indexPath := filepath.Join(g.buildRoot(), "content", repoName, "_index.md")
		if err := os.MkdirAll(filepath.Dir(indexPath), 0o750); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
		}

		// Check if repository has index.md or README.md at root level to use instead
		// Precedence: index.md > README.md > auto-generate
		//
		// Case 1: Only README.md exists → README.md becomes repository _index.md
		// Case 2: Neither exists → auto-generate repository _index.md
		// Case 3: Only index.md exists → index.md becomes repository _index.md
		// Case 4: Both exist → index.md becomes _index.md, README.md copied as regular doc
		//
		// See docs/reference/index-files.md for complete documentation
		// Use fixed epoch date for reproducible builds (user can override via custom index.md)
		var userIndexFile *docs.DocFile
		var readmeFile *docs.DocFile

		for i := range files {
			if files[i].Section == "" && files[i].Extension == ".md" {
				if strings.EqualFold(files[i].Name, "index") {
					userIndexFile = &files[i]
				} else if strings.EqualFold(files[i].Name, "README") {
					readmeFile = &files[i]
				}
			}
		}

		// Use index.md if present, otherwise fall back to README.md
		if userIndexFile != nil {
			if err := g.handleUserIndexFile(userIndexFile, indexPath, repoName); err != nil {
				return err
			}
			continue
		}

		if readmeFile != nil {
			if err := g.handleReadmeFile(readmeFile, indexPath, repoName); err != nil {
				return err
			}
			continue
		}

		frontMatter := map[string]any{"title": titleCase(repoName), "repository": repoName, "type": "docs", "date": "2024-01-01T00:00:00Z"}
		fmData, err := yaml.Marshal(frontMatter)
		if err != nil {
			return fmt.Errorf("failed to marshal front matter: %w", err)
		}
		sectionGroups := make(map[string][]docs.DocFile)
		for i := range files {
			file := &files[i]
			// files already filtered to exclude assets, so no need to check again
			s := file.Section
			if s == "" {
				s = "root"
			}
			sectionGroups[s] = append(sectionGroups[s], *file)
		}

		// Convert to sorted sections for deterministic template output
		sortedSections := makeSortedSections(sectionGroups)

		tplRaw := g.mustIndexTemplate("repository")
		ctx := buildIndexTemplateContext(g, files, map[string][]docs.DocFile{repoName: files}, frontMatter)
		ctx["Sections"] = sortedSections
		// Add repository metadata if available
		if repoConfig := g.findRepositoryConfig(repoName); repoConfig != nil {
			repoInfo := map[string]any{
				"URL":         repoConfig.URL,
				"Branch":      repoConfig.Branch,
				"Description": repoConfig.Description,
			}
			// Add sorted tags for deterministic output
			if repoConfig.Tags != nil {
				repoInfo["Tags"] = makeSortedTags(repoConfig.Tags)
			}
			ctx["RepositoryInfo"] = repoInfo
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

// handleUserIndexFile processes user-provided index.md file for repository index.
func (g *Generator) handleUserIndexFile(userIndexFile *docs.DocFile, indexPath, repoName string) error {
	// Check if already written by copyContentFiles as _index.md
	if _, err := os.Stat(indexPath); err == nil {
		slog.Debug("Using user-provided index.md as repository index (already processed)",
			logfields.Repository(repoName),
			logfields.Path(indexPath))
		return nil
	}

	if err := g.useReadmeAsIndex(userIndexFile, indexPath, repoName); err != nil {
		return err
	}

	slog.Debug("Using user-provided index.md as repository index",
		logfields.Repository(repoName),
		logfields.Path(indexPath))
	return nil
}

// handleReadmeFile processes README.md file for repository index.
func (g *Generator) handleReadmeFile(readmeFile *docs.DocFile, indexPath, repoName string) error {
	// Check if README was already written as _index.md by copyContentFiles
	if _, err := os.Stat(indexPath); err == nil {
		slog.Debug("Using README.md as repository index (already processed)",
			logfields.Repository(repoName),
			logfields.Path(indexPath))
		return nil
	}

	// Use README.md as the repository index
	if err := g.useReadmeAsIndex(readmeFile, indexPath, repoName); err != nil {
		return err
	}

	slog.Debug("Using README.md as repository index",
		logfields.Repository(repoName),
		logfields.Path(indexPath))
	return nil
}

// useReadmeAsIndex reads a README.md file and writes it as a repository _index.md,
// ensuring proper front matter is added or preserved.
// It uses the already-transformed content from DocFile.TransformedBytes to ensure
// all pipeline transforms (link rewrites, front matter patches, etc.) are preserved.
// This prevents the index stage from bypassing the transform pipeline.
func (g *Generator) useReadmeAsIndex(readmeFile *docs.DocFile, indexPath, repoName string) error {
	// Use already-transformed content from the transform pipeline
	if len(readmeFile.TransformedBytes) == 0 {
		return fmt.Errorf("%w: README not yet transformed: %s (ensure copyContentFiles ran first)",
			herrors.ErrContentTransformFailed, readmeFile.Path)
	}

	slog.Debug("Using transformed README as index",
		slog.String("source", readmeFile.RelativePath),
		slog.String("index", indexPath),
		slog.Int("bytes", len(readmeFile.TransformedBytes)))

	contentStr := string(readmeFile.TransformedBytes)

	// Parse front matter if it exists
	fm, body, err := parseFrontMatterFromContent(contentStr)
	if err != nil {
		return fmt.Errorf("failed to parse front matter in README.md: %w", err)
	}

	// If no front matter exists, create it
	if fm == nil {
		fm = map[string]any{
			"title": titleCase(repoName),
		}
		body = contentStr
	}

	// Ensure required fields are present
	ensureRequiredIndexFields(fm, repoName)

	// Reconstruct content with updated front matter
	contentStr, err = reconstructContentWithFrontMatter(fm, body)
	if err != nil {
		return err
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o750); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	// Write the index file
	// #nosec G306 -- index pages are public content
	if err := os.WriteFile(indexPath, []byte(contentStr), 0o644); err != nil {
		return fmt.Errorf("failed to write repository index from README: %w", err)
	}

	// Remove the original readme.md file since we've promoted it to _index.md
	transformedPath := filepath.Join(g.buildRoot(), readmeFile.GetHugoPath())
	if err := os.Remove(transformedPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to remove original readme.md after promoting to _index.md", "path", transformedPath, "error", err)
	}

	return nil
}

// findRepositoryConfig looks up the config.Repository by name.
func (g *Generator) findRepositoryConfig(name string) *config.Repository {
	for i := range g.config.Repositories {
		if g.config.Repositories[i].Name == name {
			return &g.config.Repositories[i]
		}
	}
	return nil
}

func (g *Generator) generateSectionIndexes(docFiles []docs.DocFile) error {
	sectionGroups, allSections := g.groupFilesBySections(docFiles)

	for repoName, sections := range sectionGroups {
		if err := g.generateSectionIndexesForRepo(repoName, sections, allSections[repoName]); err != nil {
			return err
		}
	}

	return nil
}

// groupFilesBySections organizes files by repository and section, tracking all parent sections.
func (g *Generator) groupFilesBySections(docFiles []docs.DocFile) (map[string]map[string][]docs.DocFile, map[string]map[string]bool) {
	sectionGroups := make(map[string]map[string][]docs.DocFile)
	allSections := make(map[string]map[string]bool) // Track all sections including intermediate ones

	for i := range docFiles {
		file := &docFiles[i]
		if file.Section == "" {
			continue
		}
		if sectionGroups[file.Repository] == nil {
			sectionGroups[file.Repository] = make(map[string][]docs.DocFile)
			allSections[file.Repository] = make(map[string]bool)
		}
		sectionGroups[file.Repository][file.Section] = append(sectionGroups[file.Repository][file.Section], *file)

		// Track all parent sections to ensure intermediate directories get _index.md files
		section := file.Section
		for section != "" && section != "." {
			allSections[file.Repository][section] = true
			section = filepath.Dir(section)
		}
	}

	return sectionGroups, allSections
}

// generateSectionIndexesForRepo creates index files for a single repository's sections.
func (g *Generator) generateSectionIndexesForRepo(repoName string, sections map[string][]docs.DocFile, allSections map[string]bool) error {
	// Generate indexes for sections with files
	for sectionName, files := range sections {
		if err := g.generateSectionIndex(repoName, sectionName, files, allSections); err != nil {
			return err
		}
	}

	// Create _index.md for intermediate sections that don't have files directly in them
	for sectionName := range allSections {
		if _, hasFiles := sections[sectionName]; !hasFiles {
			if err := g.generateIntermediateSectionIndex(repoName, sectionName); err != nil {
				return err
			}
		}
	}

	return nil
}

// generateSectionIndex creates an index file for a section with files.
func (g *Generator) generateSectionIndex(repoName, sectionName string, files []docs.DocFile, allSections map[string]bool) error {
	// Check if section should be skipped
	if shouldSkip, reason := g.shouldSkipSectionIndex(files, sectionName); shouldSkip {
		slog.Debug(reason, logfields.Repository(repoName), logfields.Section(sectionName))
		return nil
	}

	indexPath := filepath.Join(g.buildRoot(), "content", repoName, sectionName, "_index.md")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o750); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
	}

	frontMatter := g.buildSectionFrontMatter(repoName, sectionName)
	fmData, err := yaml.Marshal(frontMatter)
	if err != nil {
		return fmt.Errorf("failed to marshal front matter: %w", err)
	}

	subsections := g.findImmediateChildSections(repoName, sectionName, allSections)
	body, err := g.renderSectionTemplate(files, repoName, sectionName, subsections, frontMatter)
	if err != nil {
		return err
	}

	content := g.assembleSectionContent(fmData, body)
	// #nosec G306 -- index pages are public content
	if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write section index: %w", err)
	}
	slog.Debug("Generated section index", logfields.Repository(repoName), logfields.Section(sectionName), logfields.Path(indexPath))
	return nil
}

// shouldSkipSectionIndex determines if a section index should be skipped and returns the reason.
func (g *Generator) shouldSkipSectionIndex(files []docs.DocFile, sectionName string) (bool, string) {
	hasMarkdown := false
	hasUserIndex := false
	for i := range files {
		f := &files[i]
		if !f.IsAsset {
			hasMarkdown = true
			if f.Name == "index" && f.Section == sectionName {
				hasUserIndex = true
			}
		}
	}
	if !hasMarkdown {
		return true, "Skipping section index for asset-only directory"
	}
	if hasUserIndex {
		return true, "Using user-provided index.md for section"
	}
	return false, ""
}

// buildSectionFrontMatter creates front matter for a section index.
func (g *Generator) buildSectionFrontMatter(repoName, sectionName string) map[string]any {
	sectionTitle := filepath.Base(sectionName)
	return map[string]any{
		"title":      titleCase(sectionTitle),
		"repository": repoName,
		"section":    sectionName,
		"date":       "2024-01-01T00:00:00Z",
	}
}

// findImmediateChildSections finds direct child sections of the given section.
func (g *Generator) findImmediateChildSections(repoName, sectionName string, allSections map[string]bool) []string {
	var subsections []string
	for otherSection := range allSections {
		if after, ok := strings.CutPrefix(otherSection, sectionName+"/"); ok {
			if !strings.Contains(after, "/") {
				subsections = append(subsections, after)
			}
		}
	}
	return subsections
}

// renderSectionTemplate renders the section template with the given context.
func (g *Generator) renderSectionTemplate(files []docs.DocFile, repoName, sectionName string, subsections []string, frontMatter map[string]any) (string, error) {
	tplRaw := g.mustIndexTemplate("section")
	ctx := buildIndexTemplateContext(g, files, map[string][]docs.DocFile{repoName: files}, frontMatter)
	ctx["SectionName"] = sectionName
	ctx["Files"] = files
	ctx["Subsections"] = subsections

	tpl, err := template.New("section_index").Funcs(template.FuncMap{
		"titleCase":  titleCase,
		"replaceAll": strings.ReplaceAll,
		"lower":      strings.ToLower,
	}).Parse(tplRaw)
	if err != nil {
		return "", fmt.Errorf("parse section index template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("exec section index template: %w", err)
	}
	return buf.String(), nil
}

// assembleSectionContent combines front matter and body into final content.
func (g *Generator) assembleSectionContent(fmData []byte, body string) string {
	if !strings.HasPrefix(body, "---\n") {
		return fmt.Sprintf("---\n%s---\n\n%s", string(fmData), body)
	}
	return body
}

// generateIntermediateSectionIndex creates an index for sections without direct files.
func (g *Generator) generateIntermediateSectionIndex(repoName, sectionName string) error {
	indexPath := filepath.Join(g.buildRoot(), "content", repoName, sectionName, "_index.md")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o750); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
	}

	frontMatter := g.buildSectionFrontMatter(repoName, sectionName)
	fmData, err := yaml.Marshal(frontMatter)
	if err != nil {
		return fmt.Errorf("failed to marshal front matter: %w", err)
	}

	// Render template with empty file list for intermediate sections
	tplRaw := g.mustIndexTemplate("section")
	ctx := buildIndexTemplateContext(g, []docs.DocFile{}, map[string][]docs.DocFile{}, frontMatter)
	ctx["SectionName"] = sectionName
	ctx["Files"] = []docs.DocFile{}
	ctx["Subsections"] = []string{} // Will be populated by Hugo based on actual structure

	tpl, err := template.New("section_index").Funcs(template.FuncMap{
		"titleCase":  titleCase,
		"replaceAll": strings.ReplaceAll,
		"lower":      strings.ToLower,
	}).Parse(tplRaw)
	if err != nil {
		return fmt.Errorf("parse section index template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("exec section index template: %w", err)
	}

	content := g.assembleSectionContent(fmData, buf.String())
	// #nosec G306 -- index pages are public content
	if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write intermediate section index: %w", err)
	}
	slog.Debug("Generated intermediate section index", logfields.Repository(repoName), logfields.Section(sectionName), logfields.Path(indexPath))
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
		"Theme":       "relearn",
	}
	ctx["FrontMatter"] = frontMatter
	ctx["Repositories"] = repoGroups
	ctx["Files"] = docFiles
	// Removed: ctx["Now"] = time.Now() - use fixed date in frontmatter instead for reproducible builds
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

// parseFrontMatterFromContent extracts front matter and body from content.
// Returns (frontMatter map, body string, error).
// If no front matter exists, returns (nil, originalContent, nil).
func parseFrontMatterFromContent(content string) (map[string]any, string, error) {
	if !strings.HasPrefix(content, "---\n") {
		return nil, content, nil
	}

	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 {
		return nil, content, nil
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return nil, "", fmt.Errorf("failed to parse front matter: %w", err)
	}

	return fm, parts[2], nil
}

// ensureRequiredIndexFields adds missing required fields to front matter.
// Modifies the provided map in place.
func ensureRequiredIndexFields(fm map[string]any, repoName string) {
	if fm["type"] == nil {
		fm["type"] = "docs"
	}
	if fm["repository"] == nil {
		fm["repository"] = repoName
	}
	if fm["date"] == nil {
		fm["date"] = "2024-01-01T00:00:00Z"
	}
}

// reconstructContentWithFrontMatter rebuilds content string from front matter and body.
func reconstructContentWithFrontMatter(fm map[string]any, body string) (string, error) {
	fmData, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal front matter: %w", err)
	}

	return fmt.Sprintf("---\n%s---\n%s", string(fmData), body), nil
}
