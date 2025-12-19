package pipeline

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"gopkg.in/yaml.v3"
)

// parseFrontMatter extracts YAML front matter from content.
// Sets OriginalFrontMatter and removes front matter from Content.
func parseFrontMatter(doc *Document) ([]*Document, error) {
	content := doc.Content

	// Check for YAML front matter (--- ... ---)
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		// No front matter
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		doc.FrontMatter = make(map[string]any)
		return nil, nil
	}

	// Determine line ending
	var lineEnd string
	var startLen int
	if strings.HasPrefix(content, "---\r\n") {
		lineEnd = "\r\n"
		startLen = 5
	} else {
		lineEnd = "\n"
		startLen = 4
	}

	// Find end of front matter (search for closing ---\n or ---\r\n)
	endMarker := lineEnd + "---" + lineEnd
	endIdx := strings.Index(content[startLen:], endMarker)

	if endIdx == -1 {
		// Try to find just "---" followed by line ending (for content like "---\n---\n...")
		altMarker := "---" + lineEnd
		endIdx = strings.Index(content[startLen:], altMarker)
		if endIdx != -1 {
			// Adjust for the different marker length
			endMarker = altMarker
		}
	}

	if endIdx == -1 {
		// Malformed front matter - no closing delimiter
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		doc.FrontMatter = make(map[string]any)
		return nil, nil
	}

	// Extract front matter YAML
	fmYAML := content[startLen : startLen+endIdx]
	bodyStart := startLen + endIdx + len(endMarker)

	// Always remove front matter delimiters from content, even if empty
	doc.Content = content[bodyStart:]

	// Parse YAML (handle empty front matter)
	if strings.TrimSpace(fmYAML) == "" {
		// Empty front matter - no fields but delimiters were present
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		doc.FrontMatter = make(map[string]any)
		return nil, nil
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(fmYAML), &fm); err != nil {
		// Invalid YAML - treat as no front matter but content already stripped
		doc.HadFrontMatter = false
		doc.OriginalFrontMatter = make(map[string]any)
		doc.FrontMatter = make(map[string]any)
		return nil, nil
	}

	doc.HadFrontMatter = true
	doc.OriginalFrontMatter = fm
	// Deep copy to FrontMatter (transforms will modify this)
	doc.FrontMatter = deepCopyMap(fm)

	return nil, nil
}

// normalizeIndexFiles renames README files to _index for Hugo compatibility.
// This must run early before other transforms depend on the file name.
func normalizeIndexFiles(doc *Document) ([]*Document, error) {
	// Check if this is a README file at any level
	if strings.EqualFold(doc.Name, "README") {
		// Rename to _index for Hugo
		// Update both Name and Path
		doc.Name = "_index"

		// Update Path: replace README.md with _index.md at end
		if strings.HasSuffix(doc.Path, "/README.md") || strings.HasSuffix(doc.Path, "/readme.md") {
			doc.Path = doc.Path[:len(doc.Path)-len("README.md")] + "_index.md"
		} else if strings.HasSuffix(doc.Path, "README.md") {
			doc.Path = strings.TrimSuffix(doc.Path, "README.md") + "_index.md"
		}

		// Mark as index file
		doc.IsIndex = true
	}

	return nil, nil
}

// buildBaseFrontMatter ensures basic front matter fields exist.
// Adds title, type, and other base fields if not present.
func buildBaseFrontMatter(doc *Document) ([]*Document, error) {
	// Set type: docs if not present
	if _, ok := doc.FrontMatter["type"]; !ok {
		doc.FrontMatter["type"] = "docs"
	}

	// Set title from filename if not present
	if _, ok := doc.FrontMatter["title"]; !ok {
		doc.FrontMatter["title"] = titleCase(doc.Name)
	}

	// Set date if not present (use current time as fallback)
	// Hugo requires a date field for proper sorting and organization
	if _, ok := doc.FrontMatter["date"]; !ok {
		// Use current time in RFC3339 format (Hugo standard)
		doc.FrontMatter["date"] = time.Now().Format(time.RFC3339)
	}

	return nil, nil
}

// extractIndexTitle extracts the first H1 heading as the title for index files.
// Only applies if no text exists before the H1.
func extractIndexTitle(doc *Document) ([]*Document, error) {
	if !doc.IsIndex {
		return nil, nil // Only process index files
	}

	// Pattern to match H1 heading
	h1Pattern := regexp.MustCompile(`(?m)^# (.+)$`)
	matches := h1Pattern.FindStringSubmatchIndex(doc.Content)
	if matches == nil {
		return nil, nil // No H1 found
	}

	// Check for text before H1
	textBeforeH1 := strings.TrimSpace(doc.Content[:matches[0]])
	if textBeforeH1 != "" {
		return nil, nil // Use filename as title
	}

	// Extract title
	title := doc.Content[matches[2]:matches[3]]
	doc.FrontMatter["title"] = title

	return nil, nil
}

// stripHeading removes the first H1 heading from content if appropriate.
// Only strips if H1 matches the title in front matter.
func stripHeading(doc *Document) ([]*Document, error) {
	// Check if we have a title in front matter
	title, hasTitle := doc.FrontMatter["title"].(string)
	if !hasTitle {
		return nil, nil
	}

	// Pattern to match H1 heading
	h1Pattern := regexp.MustCompile(`(?m)^# (.+)\n?`)
	matches := h1Pattern.FindStringSubmatch(doc.Content)
	if matches == nil {
		return nil, nil // No H1 found
	}

	h1Title := strings.TrimSpace(matches[1])
	fmTitle := strings.TrimSpace(title)

	// Only strip if H1 matches front matter title
	if h1Title == fmTitle {
		doc.Content = h1Pattern.ReplaceAllString(doc.Content, "")
	}

	return nil, nil
}

// escapeShortcodesInCodeBlocks escapes Hugo shortcodes within markdown code blocks
// to prevent Hugo from trying to process them as actual shortcodes.
// This is essential for documentation that explains or demonstrates shortcode usage.
func escapeShortcodesInCodeBlocks(doc *Document) ([]*Document, error) {
	// Pattern to match code blocks (both ``` and indented)
	// We need to find {{< shortcode >}} and {{< /shortcode >}} patterns within code blocks

	var result strings.Builder
	lines := strings.Split(doc.Content, "\n")
	inFencedCodeBlock := false
	var fenceMarker string

	for _, line := range lines {
		// Check for fenced code block markers (```  or ~~~)
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "```") || strings.HasPrefix(trimmedLine, "~~~") {
			if !inFencedCodeBlock {
				// Starting a code block
				inFencedCodeBlock = true
				if strings.HasPrefix(trimmedLine, "```") {
					fenceMarker = "```"
				} else {
					fenceMarker = "~~~"
				}
			} else if strings.HasPrefix(trimmedLine, fenceMarker) {
				// Ending a code block
				inFencedCodeBlock = false
			}
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// If we're in a code block, escape shortcodes (but skip already-escaped ones)
		if inFencedCodeBlock {
			// Check if shortcodes are already escaped (contains {{</* or */>}})
			if strings.Contains(line, "{{</*") || strings.Contains(line, "*/>}}") {
				// Already escaped, leave as-is
				result.WriteString(line)
			} else if strings.Contains(line, "{{<") {
				// Only escape if line contains shortcode opening {{<
				// Escape both opening {{< and closing >}}
				escapedLine := strings.ReplaceAll(line, "{{<", "{{</*")
				escapedLine = strings.ReplaceAll(escapedLine, ">}}", "*/>}}")
				result.WriteString(escapedLine)
			} else {
				// No shortcodes to escape
				result.WriteString(line)
			}
		} else {
			result.WriteString(line)
		}
		result.WriteString("\n")
	}

	// Remove trailing newline if original didn't have one
	content := result.String()
	if !strings.HasSuffix(doc.Content, "\n") && strings.HasSuffix(content, "\n") {
		content = strings.TrimSuffix(content, "\n")
	}

	doc.Content = content
	// This transform only modifies content, doesn't generate new documents
	return nil, nil
}

// rewriteRelativeLinks rewrites relative markdown links to work with Hugo.
func rewriteRelativeLinks(cfg *config.Config) FileTransform {
	return func(doc *Document) ([]*Document, error) {
		// Pattern to match markdown links: [text](path)
		linkPattern := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

		doc.Content = linkPattern.ReplaceAllStringFunc(doc.Content, func(match string) string {
			submatches := linkPattern.FindStringSubmatch(match)
			if len(submatches) < 3 {
				return match
			}

			text := submatches[1]
			path := submatches[2]

			// Skip absolute URLs, anchors, mailto, etc.
			if strings.HasPrefix(path, "http://") ||
				strings.HasPrefix(path, "https://") ||
				strings.HasPrefix(path, "#") ||
				strings.HasPrefix(path, "mailto:") ||
				strings.HasPrefix(path, "/") {
				return match
			}

			// Rewrite relative link to Hugo-compatible path
			newPath := rewriteLinkPath(path, doc.Repository, doc.Forge, doc.IsIndex, doc.Path)
			return fmt.Sprintf("[%s](%s)", text, newPath)
		})

		return nil, nil
	}
}

// rewriteImageLinks rewrites image paths to work with Hugo.
func rewriteImageLinks(doc *Document) ([]*Document, error) {
	// Pattern to match markdown images: ![alt](path)
	imagePattern := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

	doc.Content = imagePattern.ReplaceAllStringFunc(doc.Content, func(match string) string {
		submatches := imagePattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		alt := submatches[1]
		path := submatches[2]

		// Skip absolute URLs
		if strings.HasPrefix(path, "http://") ||
			strings.HasPrefix(path, "https://") ||
			strings.HasPrefix(path, "/") {
			return match
		}

		// Rewrite relative image path
		newPath := rewriteImagePath(path, doc.Repository, doc.Forge)
		return fmt.Sprintf("![%s](%s)", alt, newPath)
	})

	return nil, nil
}

// generateFromKeywords scans for special keywords and generates related files.
// Example: @glossary tag creates a glossary page from all terms.
func generateFromKeywords(doc *Document) ([]*Document, error) {
	// Skip generated documents (prevent infinite loops)
	if doc.Generated {
		return nil, nil
	}

	var newDocs []*Document

	// Check for @glossary marker (placeholder - would need actual implementation)
	if strings.Contains(doc.Content, "@glossary") {
		// For now, just remove the marker
		doc.Content = strings.ReplaceAll(doc.Content, "@glossary", "")
		// TODO: Implement actual glossary generation
	}

	return newDocs, nil
}

// addRepositoryMetadata adds repository metadata to front matter.
func addRepositoryMetadata(cfg *config.Config) FileTransform {
	return func(doc *Document) ([]*Document, error) {
		// Add repository name
		if doc.Repository != "" {
			doc.FrontMatter["repository"] = doc.Repository
		}

		// Add forge namespace if present
		if doc.Forge != "" {
			doc.FrontMatter["forge"] = doc.Forge
		}

		// Add source commit if present
		if doc.SourceCommit != "" {
			doc.FrontMatter["source_commit"] = doc.SourceCommit
		}

		return nil, nil
	}
}

// addEditLink generates edit URL for the document using forge-specific patterns.
func addEditLink(cfg *config.Config) FileTransform {
	return func(doc *Document) ([]*Document, error) {
		// Skip if edit URL already exists
		if _, exists := doc.FrontMatter["editURL"]; exists {
			return nil, nil
		}

		// Skip generated documents
		if doc.Generated {
			return nil, nil
		}

		// Generate edit URL if we have repository URL and relative path
		if doc.SourceURL != "" && doc.RelativePath != "" {
			editURL := generateEditURL(doc)
			if editURL != "" {
				doc.FrontMatter["editURL"] = editURL
			}
		}

		return nil, nil
	}
}

// generateEditURL creates a forge-appropriate edit URL for a document.
func generateEditURL(doc *Document) string {
	// Get base URL by stripping .git suffix if present
	baseURL := strings.TrimSuffix(doc.SourceURL, ".git")

	// Determine branch (fallback to "main" if not set)
	branch := doc.SourceBranch
	if branch == "" {
		branch = "main"
	}

	// Build path relative to repository root
	// RelativePath is already relative to docs base, need to prepend DocsBase if it's not already there
	filePath := doc.RelativePath
	if doc.DocsBase != "" && !strings.HasPrefix(filePath, doc.DocsBase+"/") {
		filePath = doc.DocsBase + "/" + filePath
	}

	// Determine forge type from the Forge field or URL patterns
	forgeType := detectForgeType(doc.Forge, baseURL)

	// Generate URL based on forge type
	switch forgeType {
	case config.ForgeGitHub:
		return fmt.Sprintf("%s/edit/%s/%s", baseURL, branch, filePath)
	case config.ForgeGitLab:
		return fmt.Sprintf("%s/-/edit/%s/%s", baseURL, branch, filePath)
	case config.ForgeForgejo:
		// Forgejo and Gitea both use /_edit/ pattern
		return fmt.Sprintf("%s/_edit/%s/%s", baseURL, branch, filePath)
	default:
		// Fallback to GitHub-style for unknown forges
		return fmt.Sprintf("%s/edit/%s/%s", baseURL, branch, filePath)
	}
}

// detectForgeType determines the forge type from metadata or URL patterns.
func detectForgeType(forgeField, baseURL string) config.ForgeType {
	// First check if we have explicit forge metadata
	if forgeField != "" {
		switch strings.ToLower(forgeField) {
		case "github":
			return config.ForgeGitHub
		case "gitlab":
			return config.ForgeGitLab
		case "forgejo", "gitea":
			return config.ForgeForgejo
		}
	}

	// Fallback to URL pattern detection
	lowerURL := strings.ToLower(baseURL)
	if strings.Contains(lowerURL, "github.com") {
		return config.ForgeGitHub
	}
	if strings.Contains(lowerURL, "gitlab.com") || strings.Contains(lowerURL, "gitlab") {
		return config.ForgeGitLab
	}
	// Forgejo and Gitea use similar patterns - check for common hostnames
	if strings.Contains(lowerURL, "forgejo") || strings.Contains(lowerURL, "gitea") {
		return config.ForgeForgejo
	}

	// For self-hosted instances that aren't GitHub/GitLab, default to Forgejo/Gitea pattern
	// as it's becoming the most common self-hosted option
	if !strings.Contains(lowerURL, "github.com") && !strings.Contains(lowerURL, "gitlab.com") {
		return config.ForgeForgejo
	}

	// Final fallback to GitHub
	return config.ForgeGitHub
}

// serializeDocument serializes front matter and content into final bytes.
func serializeDocument(doc *Document) ([]*Document, error) {
	// Serialize front matter to YAML
	var fmYAML []byte
	var err error

	if len(doc.FrontMatter) > 0 {
		fmYAML, err = yaml.Marshal(doc.FrontMatter)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal front matter: %w", err)
		}
	}

	// Combine front matter and content
	if len(fmYAML) > 0 {
		doc.Raw = []byte(fmt.Sprintf("---\n%s---\n%s", string(fmYAML), doc.Content))
	} else {
		doc.Raw = []byte(doc.Content)
	}

	return nil, nil
}

// Helper functions

func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyMap(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = deepCopyValue(item)
		}
		return result
	default:
		return v
	}
}

func rewriteLinkPath(path, repository, forge string, isIndex bool, docPath string) string {
	// Remove .md extension
	path = strings.TrimSuffix(path, ".md")
	path = strings.TrimSuffix(path, ".markdown")

	// Handle README/index special case
	if strings.HasSuffix(path, "/README") || strings.HasSuffix(path, "/readme") {
		path = strings.TrimSuffix(path, "/README")
		path = strings.TrimSuffix(path, "/readme")
	}

	// For index files in subdirectories, relative links need to preserve directory context
	// Example: docs/how-to/index.md linking to configure-env-exposure.md
	// Should resolve to /repo/how-to/configure-env-exposure, not /repo/configure-env-exposure
	if isIndex && !strings.HasPrefix(path, "../") && !strings.HasPrefix(path, "/") {
		// Extract directory from document path
		// docPath is like "servejs/how-to/_index.md"
		// We want to preserve "how-to/" in the link
		dirPath := extractDirectory(docPath)
		if dirPath != "" && dirPath != repository && dirPath != repository+"/" {
			// Link is relative to current directory, prepend the directory
			path = dirPath + "/" + path
		}
	}

	// Handle relative paths that navigate up directories (../)
	// For paths starting with ../, we know they're relative to the current document's location
	// Since all documents are flattened into /{forge}/{repo}/ structure,
	// any ../ navigation stays within the repository namespace
	if strings.HasPrefix(path, "../") {
		// Strip all leading ../ sequences - the path is relative to repository root
		for strings.HasPrefix(path, "../") {
			path = strings.TrimPrefix(path, "../")
		}

		// Now prepend repository path
		if repository != "" {
			if forge != "" {
				path = fmt.Sprintf("/%s/%s/%s", forge, repository, path)
			} else {
				path = fmt.Sprintf("/%s/%s", repository, path)
			}
		}
		return path
	}

	// Prepend repository path if relative (not starting with /)
	if !strings.HasPrefix(path, "/") && repository != "" {
		if forge != "" {
			path = fmt.Sprintf("/%s/%s/%s", forge, repository, path)
		} else {
			path = fmt.Sprintf("/%s/%s", repository, path)
		}
	}

	return path
}

// extractDirectory returns the directory portion of a Hugo path, relative to repository.
// Example: "servejs/how-to/_index.md" returns "how-to"
//
//	"servejs/api/_index.md" returns "api"
//	"servejs/_index.md" returns ""
//	"docs/guide/advanced/_index.md" returns "guide/advanced"
//	"gitlab/docs/how-to/_index.md" returns "how-to" (handles forge namespace)
func extractDirectory(hugoPath string) string {
	// Remove file name (_index.md or other.md)
	dir := hugoPath
	if lastSlash := strings.LastIndex(dir, "/"); lastSlash >= 0 {
		dir = dir[:lastSlash]
	}

	// Split into segments
	segments := strings.Split(dir, "/")

	// Path patterns:
	// - repo (no subdirs): segments = [repo] -> return ""
	// - repo/section: segments = [repo, section] -> return "section"
	// - repo/section/subsection: segments = [repo, section, subsection] -> return "section/subsection"
	// - forge/repo/section: segments = [forge, repo, section] -> return "section"

	if len(segments) <= 1 {
		// Just repository, no subdirectory
		return ""
	}

	// Heuristic: if there are 3+ segments and first segment length is relatively short (common forge names),
	// assume it's a forge namespace. Otherwise, treat first segment as repository.
	// Common forge names: gitlab, github, forgejo, gitea (all <= 8 chars)
	hasForge := len(segments) >= 3 && len(segments[0]) <= 8

	if hasForge {
		// forge/repo/section... format
		// Return everything after repo (index 1)
		if len(segments) > 2 {
			return strings.Join(segments[2:], "/")
		}
		return ""
	}

	// repo/section... format
	// Return everything after repo (index 0)
	return strings.Join(segments[1:], "/")
}

func rewriteImagePath(path, repository, forge string) string {
	// Prepend repository path if relative
	if !strings.HasPrefix(path, "/") && repository != "" {
		if forge != "" {
			path = fmt.Sprintf("/%s/%s/%s", forge, repository, path)
		} else {
			path = fmt.Sprintf("/%s/%s", repository, path)
		}
	}

	return path
}
