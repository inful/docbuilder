package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"gopkg.in/yaml.v3"
)

// titleCase converts a string to title case (replacement for deprecated strings.Title)
func titleCase(s string) string {
	if s == "" {
		return s
	}
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// Generator handles Hugo site generation
type Generator struct {
	config    *config.Config
	outputDir string
}

// NewGenerator creates a new Hugo site generator
func NewGenerator(cfg *config.Config, outputDir string) *Generator {
	return &Generator{
		config:    cfg,
		outputDir: outputDir,
	}
}

// GenerateSite creates a complete Hugo site from discovered documentation
func (g *Generator) GenerateSite(docFiles []docs.DocFile) error {
	_, err := g.GenerateSiteWithReport(docFiles)
	return err
}

// GenerateSiteWithReport performs site generation and returns a BuildReport with metrics.
func (g *Generator) GenerateSiteWithReport(docFiles []docs.DocFile) (*BuildReport, error) {
	slog.Info("Starting Hugo site generation", "output", g.outputDir, "files", len(docFiles))
	repoSet := map[string]struct{}{}
	for _, f := range docFiles { repoSet[f.Repository] = struct{}{} }
	report := newBuildReport(len(repoSet), len(docFiles))

	// Helper to record an error but continue (non-fatal phases)
	addErr := func(err error) {
		if err != nil { report.Errors = append(report.Errors, err); slog.Warn("Generation phase error", "error", err) }
	}

	// Create Hugo directory structure
	if err := g.createHugoStructure(); err != nil {
		return nil, fmt.Errorf("failed to create Hugo structure: %w", err)
	}

	// Generate Hugo configuration
	if err := g.generateHugoConfig(); err != nil {
		return nil, fmt.Errorf("failed to generate Hugo config: %w", err)
	}

	// Generate basic layouts if no theme is specified
	if g.config.Hugo.Theme == "" {
		if err := g.generateBasicLayouts(); err != nil {
			return nil, fmt.Errorf("failed to generate layouts: %w", err)
		}
	}

	// Copy documentation files to content directory
	if err := g.copyContentFiles(docFiles); err != nil {
		return nil, fmt.Errorf("failed to copy content files: %w", err)
	}

	// Generate index pages
	if err := g.generateIndexPages(docFiles); err != nil {
		return nil, fmt.Errorf("failed to generate index pages: %w", err)
	}

	// Optionally execute Hugo to render static site (non-fatal)
	if shouldRunHugo() {
		if err := g.runHugoBuild(); err != nil {
			addErr(fmt.Errorf("hugo build failed: %w", err))
		} else {
			slog.Info("Hugo static site build completed", "public", filepath.Join(g.outputDir, "public"))
		}
	} else {
		slog.Info("Skipping Hugo executable run (set DOCBUILDER_RUN_HUGO=1 to enable)")
	}

	report.finish()
	slog.Info("Hugo site generation completed", "output", g.outputDir, "repos", report.Repositories, "files", report.Files, "errors", len(report.Errors))
	return report, nil
}

// shouldRunHugo determines if we should invoke the external hugo binary.
// Enabled when DOCBUILDER_RUN_HUGO=1 and hugo binary exists in PATH, unless DOCBUILDER_SKIP_HUGO=1.
func shouldRunHugo() bool {
	if os.Getenv("DOCBUILDER_SKIP_HUGO") == "1" {
		return false
	}
	if os.Getenv("DOCBUILDER_RUN_HUGO") != "1" {
		return false
	}
	_, err := exec.LookPath("hugo")
	return err == nil
}

// runHugoBuild executes `hugo` inside the output directory to produce the static site under public/.
func (g *Generator) runHugoBuild() error {
	cmd := exec.Command("hugo")
	cmd.Dir = g.outputDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	slog.Info("Running Hugo binary to render static site", "dir", g.outputDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hugo command failed: %w", err)
	}
	return nil
}

// createHugoStructure creates the basic Hugo directory structure
func (g *Generator) createHugoStructure() error {
	dirs := []string{
		"content",
		"layouts",
		"layouts/_default",
		"layouts/partials",
		"static",
		"data",
		"assets",
		"archetypes",
	}

	for _, dir := range dirs {
		path := filepath.Join(g.outputDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	slog.Debug("Created Hugo directory structure", "output", g.outputDir)
	return nil
}

// generateHugoConfig creates the Hugo configuration file
func (g *Generator) generateHugoConfig() error {
	configPath := filepath.Join(g.outputDir, "hugo.yaml")

	// Initialize params if nil
	params := make(map[string]interface{})
	if g.config.Hugo.Params != nil {
		params = g.config.Hugo.Params
	}

	// Add build date
	params["build_date"] = time.Now().Format("2006-01-02 15:04:05")

	// Add Docsy- or Hextra-specific parameters based on theme
	if g.config.Hugo.Theme == "docsy" {
		g.addDocsyParams(params)
	} else if g.config.Hugo.Theme == "hextra" {
		g.addHextraParams(params)
	}

	hugoConfig := map[string]interface{}{
		"title":         g.config.Hugo.Title,
		"description":   g.config.Hugo.Description,
		"baseURL":       g.config.Hugo.BaseURL,
		"languageCode":  "en",
		"enableGitInfo": true,
		"markup": map[string]interface{}{
			"goldmark": map[string]interface{}{
				"renderer": map[string]interface{}{
					"unsafe": true,
				},
			},
			"highlight": map[string]interface{}{
				"style":     "github",
				"lineNos":   true,
				"tabWidth":  4,
				"noClasses": false,
			},
		},
		"params": params,
	}

	// Theme configuration
	// If using Docsy/Hextra, prefer Hugo Modules to ensure dependencies are resolved.
	if g.config.Hugo.Theme != "" {
		if g.config.Hugo.Theme == "docsy" {
			hugoConfig["module"] = map[string]interface{}{
				"imports": []map[string]interface{}{
					{"path": "github.com/google/docsy"},
				},
			}
			// Do NOT set "theme" when using modules for Docsy to avoid filesystem theme lookup
		} else if g.config.Hugo.Theme == "hextra" {
			hugoConfig["module"] = map[string]interface{}{
				"imports": []map[string]interface{}{
					{"path": "github.com/imfing/hextra"},
				},
			}
			// Do NOT set "theme" when using modules for Hextra
		} else {
			// For non-Docsy themes, use the traditional theme approach
			hugoConfig["theme"] = g.config.Hugo.Theme
		}
	}

	// Hextra-specific: enable Goldmark passthrough for LaTeX math delimiters
	if g.config.Hugo.Theme == "hextra" {
		if m, ok := hugoConfig["markup"].(map[string]interface{}); ok {
			gm, _ := m["goldmark"].(map[string]interface{})
			if gm == nil {
				gm = map[string]interface{}{}
				m["goldmark"] = gm
			}
			ext, _ := gm["extensions"].(map[string]interface{})
			if ext == nil {
				ext = map[string]interface{}{}
				gm["extensions"] = ext
			}
			passthrough := map[string]interface{}{
				"delimiters": map[string]interface{}{
					"block":  [][]string{{"\\[", "\\]"}, {"$$", "$$"}},
					"inline": [][]string{{"\\(", "\\)"}},
				},
				"enable": true,
			}
			ext["passthrough"] = passthrough
		}
	}

	// If using Docsy, ensure JSON output for home to support offline search index
	// Hextra doesn't ship a home JSON layout and doesn't need it, so skip to avoid warnings.
	if g.config.Hugo.Theme == "docsy" {
		outputs := map[string]interface{}{
			"home": []string{"HTML", "RSS", "JSON"},
		}
		hugoConfig["outputs"] = outputs
	}

	// Menu handling
	if g.config.Hugo.Theme == "hextra" {
		// If user hasn't provided a menu, add a sensible default navbar for Hextra
		if g.config.Hugo.Menu == nil {
			mainMenu := []map[string]interface{}{
				{
					"name":   "Search",
					"weight": 4,
					"params": map[string]interface{}{"type": "search"},
				},
				{
					"name":   "Theme",
					"weight": 98,
					"params": map[string]interface{}{"type": "theme-toggle", "label": false},
				},
			}
			// Add GitHub icon to menu if any repository URL points to GitHub
			for _, repo := range g.config.Repositories {
				if strings.Contains(repo.URL, "github.com") {
					mainMenu = append(mainMenu, map[string]interface{}{
						"name":   "GitHub",
						"weight": 99,
						"url":    repo.URL,
						"params": map[string]interface{}{"icon": "github"},
					})
					break
				}
			}

			hugoConfig["menu"] = map[string]interface{}{
				"main": mainMenu,
			}
		} else {
			// Respect user-provided menu when present
			hugoConfig["menu"] = g.config.Hugo.Menu
		}
	} else if g.config.Hugo.Menu != nil {
		// For non-Hextra themes, only add menu if specified by user
		hugoConfig["menu"] = g.config.Hugo.Menu
	}

	data, err := yaml.Marshal(hugoConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal Hugo config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write Hugo config: %w", err)
	}

	// Ensure go.mod exists when using Docsy/Hextra via Hugo Modules so dependencies resolve
	if g.config.Hugo.Theme == "docsy" || g.config.Hugo.Theme == "hextra" {
		if err := g.ensureGoModForModules(); err != nil {
			slog.Warn("Failed to ensure go.mod for Hugo Modules", "error", err)
		}
	}

	slog.Info("Generated Hugo configuration", "path", configPath)
	return nil
}

// ensureGoModForModules creates a minimal go.mod to allow Hugo Modules to work
func (g *Generator) ensureGoModForModules() error {
	goModPath := filepath.Join(g.outputDir, "go.mod")

	deriveModuleName := func() string {
		moduleName := "docbuilder-site"
		if g.config.Hugo.BaseURL != "" {
			s := strings.TrimPrefix(strings.TrimPrefix(g.config.Hugo.BaseURL, "https://"), "http://")
			host := s
			if idx := strings.IndexByte(s, '/'); idx >= 0 {
				host = s[:idx]
			}
			// Strip port if present (e.g., localhost:8080)
			if p := strings.IndexByte(host, ':'); p >= 0 {
				host = host[:p]
			}
			if host != "" {
				moduleName = strings.ReplaceAll(host, ".", "-")
			}
		}
		return moduleName
	}

	if _, err := os.Stat(goModPath); err == nil {
		// Already exists – validate module line; rewrite if invalid (e.g., contains ':')
		b, readErr := os.ReadFile(goModPath)
		if readErr == nil {
			lines := strings.SplitN(string(b), "\n", 2)
			if len(lines) > 0 && strings.HasPrefix(lines[0], "module ") {
				existing := strings.TrimSpace(strings.TrimPrefix(lines[0], "module "))
				if strings.Contains(existing, ":") { // invalid char for module path
					sanitized := deriveModuleName()
					rest := ""
					if len(lines) > 1 {
						rest = lines[1]
					}
					newContent := fmt.Sprintf("module %s\n", sanitized)
					if !strings.Contains(rest, "go ") {
						newContent += "\ngo 1.21\n"
					} else {
						newContent += rest
					}
					if writeErr := os.WriteFile(goModPath, []byte(newContent), 0644); writeErr != nil {
						slog.Warn("Failed to rewrite invalid go.mod module line", "error", writeErr)
					} else {
						slog.Debug("Rewrote go.mod with sanitized module name", "path", goModPath, "module", sanitized)
					}
				}
			}
		}
		// Still ensure required theme versions are present
		return g.ensureThemeVersionRequires(goModPath)
	}

	moduleName := deriveModuleName()
	content := fmt.Sprintf("module %s\n\ngo 1.21\n", moduleName)
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	slog.Debug("Created go.mod for Hugo Modules", "path", goModPath)
	return g.ensureThemeVersionRequires(goModPath)
}

// ensureThemeVersionRequires appends require directives for known themes to pin versions
func (g *Generator) ensureThemeVersionRequires(goModPath string) error {
	b, err := os.ReadFile(goModPath)
	if err != nil {
		return err
	}
	s := string(b)

	// Pin Hextra to a stable version if selected
	if g.config.Hugo.Theme == "hextra" {
		const hextraModule = "github.com/imfing/hextra"
		const hextraVersion = "v0.11.0"
		if !strings.Contains(s, hextraModule) {
			s += fmt.Sprintf("\nrequire %s %s\n", hextraModule, hextraVersion)
		}
	}

	// It's safe to let Hugo decide Docsy version dynamically unless we need pinning later
	// (We can add a similar block for Docsy if necessary.)

	return os.WriteFile(goModPath, []byte(s), 0644)
}

// addDocsyParams adds Docsy theme-specific parameters to Hugo config
func (g *Generator) addDocsyParams(params map[string]interface{}) {
	// Set default Docsy parameters if not already specified
	if params["version"] == nil {
		params["version"] = "main"
	}

	if params["github_repo"] == nil && len(g.config.Repositories) > 0 {
		// Use the first repository as the main GitHub repo
		firstRepo := g.config.Repositories[0]
		if strings.Contains(firstRepo.URL, "github.com") {
			params["github_repo"] = firstRepo.URL
		}
	}

	if params["github_branch"] == nil && len(g.config.Repositories) > 0 {
		params["github_branch"] = g.config.Repositories[0].Branch
	}

	// Enable editing links
	if params["edit_page"] == nil {
		params["edit_page"] = true
	}

	// Enable search
	if params["search"] == nil {
		params["search"] = true
	}

	// Enable offline search (Lunr.js) by default
	if params["offlineSearch"] == nil {
		params["offlineSearch"] = true
	}
	// Provide sensible defaults for offline search behavior if not set
	if params["offlineSearchSummaryLength"] == nil {
		params["offlineSearchSummaryLength"] = 200
	}
	if params["offlineSearchMaxResults"] == nil {
		params["offlineSearchMaxResults"] = 25
	}

	// Add UI configuration
	if params["ui"] == nil {
		params["ui"] = map[string]interface{}{
			"sidebar_menu_compact":                  false,
			"sidebar_menu_foldable":                 true,
			"breadcrumb_disable":                    false,
			"taxonomy_breadcrumb_disable":           false,
			"footer_about_disable":                  false,
			"navbar_logo":                           true,
			"navbar_translucent_over_cover_disable": false,
			"sidebar_search_disable":                false,
		}
	}

	// Add links configuration
	if params["links"] == nil {
		links := map[string]interface{}{
			"user":      []map[string]interface{}{},
			"developer": []map[string]interface{}{},
		}

		// Add repository links if available
		if len(g.config.Repositories) > 0 {
			for _, repo := range g.config.Repositories {
				if strings.Contains(repo.URL, "github.com") {
					repoLink := map[string]interface{}{
						"name": fmt.Sprintf("%s Repository", titleCase(repo.Name)),
						"url":  repo.URL,
						"icon": "fab fa-github",
						"desc": fmt.Sprintf("Development happens here for %s", repo.Name),
					}

					if developerLinks, ok := links["developer"].([]map[string]interface{}); ok {
						links["developer"] = append(developerLinks, repoLink)
					}
				}
			}
		}

		params["links"] = links
	}
}

// addHextraParams adds Hextra theme-specific parameters to Hugo config
func (g *Generator) addHextraParams(params map[string]interface{}) {
	// Enable and configure search by default (Hextra expects params.search.enable)
	if params["search"] == nil {
		params["search"] = map[string]interface{}{
			"enable": true,
			"type":   "flexsearch",
			"flexsearch": map[string]interface{}{
				"index":    "content", // content | summary | heading | title
				"tokenize": "forward", // full | forward | reverse | strict
				"version":  "0.8.143", // default per theme
			},
		}
	} else {
		// Normalize boolean to map form if user set search: true/false
		if b, ok := params["search"].(bool); ok {
			params["search"] = map[string]interface{}{"enable": b}
		} else if m, ok := params["search"].(map[string]interface{}); ok {
			if _, exists := m["enable"]; !exists {
				m["enable"] = true
			}
			// Backfill search.type and flexsearch defaults if missing
			if _, ok := m["type"]; !ok {
				m["type"] = "flexsearch"
			}
			if _, ok := m["flexsearch"]; !ok {
				m["flexsearch"] = map[string]interface{}{"index": "content", "tokenize": "forward", "version": "0.8.143"}
			} else if fm, ok := m["flexsearch"].(map[string]interface{}); ok {
				if _, ok := fm["index"]; !ok {
					fm["index"] = "content"
				}
				if _, ok := fm["tokenize"]; !ok {
					fm["tokenize"] = "forward"
				}
				if _, ok := fm["version"]; !ok {
					fm["version"] = "0.8.143"
				}
			}
		}
	}

	// Provide generic offline search defaults (harmless if unused)
	if params["offlineSearch"] == nil {
		params["offlineSearch"] = true
	}
	if params["offlineSearchSummaryLength"] == nil {
		params["offlineSearchSummaryLength"] = 200
	}
	if params["offlineSearchMaxResults"] == nil {
		params["offlineSearchMaxResults"] = 25
	}

	// Add theme defaults and UI basics (safe no-ops if theme ignores)
	if _, ok := params["theme"].(map[string]interface{}); !ok {
		params["theme"] = map[string]interface{}{
			"default":       "system", // light | dark | system
			"displayToggle": true,
		}
	}
	if params["ui"] == nil {
		params["ui"] = map[string]interface{}{
			"navbar_logo":            true,
			"sidebar_menu_foldable":  true,
			"sidebar_menu_compact":   false,
			"sidebar_search_disable": false,
		}
	}

	// Ensure mermaid params exist (theme will pick sensible defaults)
	if _, ok := params["mermaid"]; !ok {
		params["mermaid"] = map[string]interface{}{}
	}

	// Enable "Edit this page" by default unless user disables or configures differently
	if v, ok := params["editURL"]; !ok {
		params["editURL"] = map[string]interface{}{"enable": true}
	} else if m, ok := v.(map[string]interface{}); ok {
		if _, exists := m["enable"]; !exists {
			m["enable"] = true
		}
	}

	// Optional: Navbar width control (normal | full | custom rem string)
	if _, ok := params["navbar"].(map[string]interface{}); !ok {
		params["navbar"] = map[string]interface{}{
			"width": "normal",
		}
	}
}

// copyContentFiles copies documentation files to Hugo content directory
func (g *Generator) copyContentFiles(docFiles []docs.DocFile) error {
	pipeline := NewTransformerPipeline(
		&FrontMatterParser{},
		&RelativeLinkRewriter{},
		&FrontMatterBuilder{ConfigProvider: func() *Generator { return g }},
		&FinalFrontMatterSerializer{},
	)

	for _, file := range docFiles {
		if err := file.LoadContent(); err != nil {
			return fmt.Errorf("failed to load content for %s: %w", file.Path, err)
		}
		p := &Page{File: file, Raw: file.Content, Content: string(file.Content), FrontMatter: map[string]any{}}
		if err := pipeline.Run(p); err != nil {
			return fmt.Errorf("pipeline failed for %s: %w", file.Path, err)
		}
		outputPath := filepath.Join(g.outputDir, file.GetHugoPath())
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", outputPath, err)
		}
		if err := os.WriteFile(outputPath, p.Raw, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", outputPath, err)
		}
		slog.Debug("Copied content file", "source", file.RelativePath, "destination", file.GetHugoPath())
	}
	slog.Info("Copied all content files", "count", len(docFiles))
	return nil
}

// processMarkdownFile processes a markdown file and adds Hugo front matter
// processMarkdownFile deprecated: replaced by transformer pipeline inside copyContentFiles.
// TODO: remove after callers (if any external) are migrated.
func (g *Generator) processMarkdownFile(file docs.DocFile) ([]byte, error) {
	p := &Page{File: file, Raw: file.Content, Content: string(file.Content), FrontMatter: map[string]any{}}
	pipeline := NewTransformerPipeline(
		&FrontMatterParser{},
		&RelativeLinkRewriter{},
		&FrontMatterBuilder{ConfigProvider: func() *Generator { return g }},
		&FinalFrontMatterSerializer{},
	)
	if err := pipeline.Run(p); err != nil { return nil, err }
	return p.Raw, nil
}

// generateIndexPages creates index pages for sections and the main site
func (g *Generator) generateIndexPages(docFiles []docs.DocFile) error {
	// Generate main index page
	if err := g.generateMainIndex(docFiles); err != nil {
		return err
	}

	// Generate repository index pages
	if err := g.generateRepositoryIndexes(docFiles); err != nil {
		return err
	}

	// Generate section index pages
	if err := g.generateSectionIndexes(docFiles); err != nil {
		return err
	}

	return nil
}

// generateMainIndex creates the main site index page
func (g *Generator) generateMainIndex(docFiles []docs.DocFile) error {
	indexPath := filepath.Join(g.outputDir, "content", "_index.md")

	// Group files by repository
	repoGroups := make(map[string][]docs.DocFile)
	for _, file := range docFiles {
		repoGroups[file.Repository] = append(repoGroups[file.Repository], file)
	}

	frontMatter := map[string]interface{}{
		"title":       g.config.Hugo.Title,
		"description": g.config.Hugo.Description,
		"date":        time.Now().Format("2006-01-02T15:04:05-07:00"),
		"type":        "docs",
	}

	// For Hextra, cascade docs type to all descendants so the sidebar persists without per-page front matter
	if g.config.Hugo.Theme == "hextra" {
		frontMatter["cascade"] = map[string]interface{}{
			"type": "docs",
		}
	}

	fmData, err := yaml.Marshal(frontMatter)
	if err != nil {
		return fmt.Errorf("failed to marshal front matter: %w", err)
	}

	content := fmt.Sprintf("---\n%s---\n\n# %s\n\n%s\n\n",
		string(fmData),
		g.config.Hugo.Title,
		g.config.Hugo.Description)

	content += "## Repositories\n\n"
	for repoName, files := range repoGroups {
		// Use extensionless pretty URL (directory) for repository root
		content += fmt.Sprintf("- [%s](./%s/) (%d files)\n", repoName, repoName, len(files))
	}

	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write main index: %w", err)
	}

	slog.Info("Generated main index page", "path", indexPath)
	return nil
}

// generateRepositoryIndexes creates index pages for each repository
func (g *Generator) generateRepositoryIndexes(docFiles []docs.DocFile) error {
	repoGroups := make(map[string][]docs.DocFile)
	for _, file := range docFiles {
		repoGroups[file.Repository] = append(repoGroups[file.Repository], file)
	}

	for repoName, files := range repoGroups {
		indexPath := filepath.Join(g.outputDir, "content", repoName, "_index.md")

		if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
		}

		frontMatter := map[string]interface{}{
			"title":      fmt.Sprintf("%s Documentation", titleCase(repoName)),
			"repository": repoName,
			"date":       time.Now().Format("2006-01-02T15:04:05-07:00"),
		}

		fmData, err := yaml.Marshal(frontMatter)
		if err != nil {
			return fmt.Errorf("failed to marshal front matter: %w", err)
		}

		content := fmt.Sprintf("---\n%s---\n\n# %s Documentation\n\n",
			string(fmData),
			titleCase(repoName))

		// Group by sections
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
					relativePath = filepath.Join(file.Section, file.Name) // drop extension for pretty URL
				} else {
					relativePath = file.Name // drop extension
				}
				// Normalize to forward slashes for Hugo even on Windows
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

// generateSectionIndexes creates index pages for sections within repositories
func (g *Generator) generateSectionIndexes(docFiles []docs.DocFile) error {
	// Group by repository and section
	sectionGroups := make(map[string]map[string][]docs.DocFile)

	for _, file := range docFiles {
		if file.Section == "" {
			continue // Skip root level files
		}

		if sectionGroups[file.Repository] == nil {
			sectionGroups[file.Repository] = make(map[string][]docs.DocFile)
		}

		sectionGroups[file.Repository][file.Section] = append(
			sectionGroups[file.Repository][file.Section],
			file)
	}

	for repoName, sections := range sectionGroups {
		for sectionName, files := range sections {
			indexPath := filepath.Join(g.outputDir, "content", repoName, sectionName, "_index.md")

			if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", indexPath, err)
			}

			frontMatter := map[string]interface{}{
				"title":      fmt.Sprintf("%s - %s", titleCase(repoName), titleCase(sectionName)),
				"repository": repoName,
				"section":    sectionName,
				"date":       time.Now().Format("2006-01-02T15:04:05-07:00"),
			}

			fmData, err := yaml.Marshal(frontMatter)
			if err != nil {
				return fmt.Errorf("failed to marshal front matter: %w", err)
			}

			content := fmt.Sprintf("---\n%s---\n\n# %s\n\n",
				string(fmData),
				titleCase(sectionName))

			for _, file := range files {
				title := titleCase(strings.ReplaceAll(file.Name, "-", " "))
				// Use extensionless pretty link with trailing slash for consistency
				content += fmt.Sprintf("- [%s](./%s/)\n", title, file.Name)
			}

			if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write section index: %w", err)
			}

			slog.Debug("Generated section index",
				"repository", repoName,
				"section", sectionName,
				"path", indexPath)
		}
	}

	return nil
}
