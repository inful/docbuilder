package pipeline

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// addRepositoryMetadata adds repository, section, and custom metadata to front matter.
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

		// Add section if present
		if doc.Section != "" {
			doc.FrontMatter["section"] = doc.Section
		}

		// Add source commit if present
		if doc.SourceCommit != "" {
			doc.FrontMatter["source_commit"] = doc.SourceCommit
		}

		// Metadata passthrough from discovery phase (if not already set in frontmatter)
		for k, v := range doc.CustomMetadata {
			if _, exists := doc.FrontMatter[k]; !exists {
				doc.FrontMatter[k] = v
			}
		}

		return nil, nil
	}
}

// addEditLink generates edit URL for the document using forge-specific patterns.
func addEditLink(cfg *config.Config) FileTransform {
	return func(doc *Document) ([]*Document, error) {
		// In daemon public-only mode, do not emit edit links.
		// Rationale: public-only is typically used for unauthenticated/public publishing,
		// and edit links often point at authenticated endpoints.
		if cfg != nil && cfg.IsDaemonPublicOnlyEnabled() {
			return nil, nil
		}

		// Skip if edit URL already exists
		if _, exists := doc.FrontMatter["editURL"]; exists {
			return nil, nil
		}

		// Skip generated documents
		if doc.Generated {
			return nil, nil
		}

		// For VS Code preview mode, generate edit URL even without SourceURL
		if doc.VSCodeEditLinks && doc.IsSingleRepo && doc.RelativePath != "" {
			doc.FrontMatter["editURL"] = fmt.Sprintf("/_edit/%s", doc.RelativePath)
			fmt.Fprintf(os.Stderr, "DEBUG: Generated VS Code edit URL: /_edit/%s (VSCodeEditLinks=%v, IsSingleRepo=%v, RelativePath=%q)\n",
				doc.RelativePath, doc.VSCodeEditLinks, doc.IsSingleRepo, doc.RelativePath)
			return nil, nil
		}

		// Debug: Log why VS Code edit URL was not generated
		if doc.RelativePath != "" && !doc.Generated {
			fmt.Fprintf(os.Stderr, "DEBUG: VS Code edit URL NOT generated for %s: VSCodeEditLinks=%v, IsSingleRepo=%v, RelativePath=%q\n",
				doc.RelativePath, doc.VSCodeEditLinks, doc.IsSingleRepo, doc.RelativePath)
		}

		// Generate forge edit URL if we have repository URL and relative path
		if doc.SourceURL != "" && doc.RelativePath != "" {
			editURL := generateEditURL(doc)
			if editURL != "" {
				doc.FrontMatter["editURL"] = editURL
			}
		}

		return nil, nil
	}
}

// forgeBase returns the web address of the repository a document comes
// from and the forge that hosts it.
//
// The address is the clone URL of the repository without its ".git"
// suffix, unless the operator overrode it. An empty address means the
// document has no repository behind it - a local build, or a page that
// DocBuilder generated itself.
func forgeBase(doc *Document) (string, config.ForgeType) {
	var baseURL string
	switch {
	case doc.EditURLBase != "":
		// CLI override provided
		baseURL = strings.TrimSuffix(doc.EditURLBase, ".git")
	case doc.SourceURL != "" && isForgeURL(doc.SourceURL):
		// Use SourceURL if it's a real forge URL
		baseURL = strings.TrimSuffix(doc.SourceURL, ".git")
	default:
		// No valid base URL for links to the forge
		return "", ""
	}
	baseURL = webURL(baseURL)
	return baseURL, detectForgeType(doc.Forge, baseURL)
}

// webURL turns an SSH address into the web address of the same
// repository: "git@github.com:org/repo" is
// "https://github.com/org/repo". A link has to open in a browser.
func webURL(rawURL string) string {
	if !strings.HasPrefix(rawURL, "git@") {
		return rawURL
	}
	parts := strings.SplitN(strings.TrimPrefix(rawURL, "git@"), ":", 2)
	if len(parts) != 2 {
		return rawURL
	}
	return "https://" + parts[0] + "/" + parts[1]
}

// generateEditURL creates a forge-appropriate edit URL for a document.
// Only generates URLs for documents with repository metadata.
// For preview mode in VS Code, VS Code edit URLs are handled separately.
func generateEditURL(doc *Document) string {
	// Preview mode with VS Code: generate local edit URL
	if doc.VSCodeEditLinks && doc.IsSingleRepo {
		// VS Code edit links enabled via --vscode flag
		// This handler opens files directly in the VS Code editor
		return fmt.Sprintf("/_edit/%s", doc.RelativePath)
	}

	// Production builds: generate forge-specific edit URLs
	baseURL, forgeType := forgeBase(doc)
	if baseURL == "" {
		return ""
	}

	// Determine branch (fallback to "main" if not set)
	branch := doc.SourceBranch
	if branch == "" {
		branch = "main"
	}

	// Build path relative to repository root
	// RelativePath is already relative to docs base, need to prepend DocsBase if it's not already there
	// Skip if DocsBase is "." (current directory marker used in local builds)
	filePath := doc.RelativePath
	if doc.DocsBase != "" && doc.DocsBase != "." && !strings.HasPrefix(filePath, doc.DocsBase+"/") {
		filePath = doc.DocsBase + "/" + filePath
	}

	// Generate URL based on forge type
	switch forgeType {
	case config.ForgeGitHub:
		return fmt.Sprintf("%s/edit/%s/%s", baseURL, branch, filePath)
	case config.ForgeGitLab:
		return fmt.Sprintf("%s/-/edit/%s/%s", baseURL, branch, filePath)
	case config.ForgeForgejo:
		// Forgejo and Gitea both use /_edit/ pattern
		return fmt.Sprintf("%s/_edit/%s/%s", baseURL, branch, filePath)
	case config.ForgeLocal:
		// Local forges don't have web UI edit URLs
		return ""
	default:
		// Fallback to GitHub-style for unknown forges
		return fmt.Sprintf("%s/edit/%s/%s", baseURL, branch, filePath)
	}
}

// addIssueLink generates the address of the new-issue page of the
// forge a document comes from.
//
// The issue belongs to the repository the page was written in: a site
// that gathers several repositories sends the reader to the tracker of
// the one they are reading.
func addIssueLink(cfg *config.Config) FileTransform {
	return func(doc *Document) ([]*Document, error) {
		// In daemon public-only mode, do not emit links to the forge.
		// Rationale: the same as for the edit links.
		if cfg != nil && cfg.IsDaemonPublicOnlyEnabled() {
			return nil, nil
		}

		// Skip if the issue URL already exists
		if _, exists := doc.FrontMatter["issueURL"]; exists {
			return nil, nil
		}

		// Skip generated documents
		if doc.Generated {
			return nil, nil
		}

		if issueURL := generateIssueURL(doc); issueURL != "" {
			doc.FrontMatter["issueURL"] = issueURL
		}

		return nil, nil
	}
}

// generateIssueURL creates the address of the forge's new-issue page
// for a document.
//
// Every forge names that page differently and takes the title of the
// issue under a different parameter, so the address is built per
// forge.
func generateIssueURL(doc *Document) string {
	// A local repository has no tracker to file an issue in. The forge
	// of the document says so directly: the detection by address would
	// mistake it for a self-hosted forge.
	if strings.EqualFold(doc.Forge, string(config.ForgeLocal)) {
		return ""
	}

	baseURL, forgeType := forgeBase(doc)
	if baseURL == "" {
		return ""
	}

	title := issueTitle(doc)
	if title == "" {
		return ""
	}

	baseURL = strings.TrimSuffix(baseURL, "/")
	switch forgeType {
	case config.ForgeGitHub, config.ForgeForgejo:
		// GitHub and Forgejo both take the title as "title".
		return baseURL + "/issues/new?" + url.Values{"title": {title}}.Encode()
	case config.ForgeGitLab:
		// GitLab names its issue parameters after the issue fields.
		return baseURL + "/-/issues/new?" + url.Values{"issue[title]": {title}}.Encode()
	case config.ForgeLocal:
		// A local repository has no tracker to file an issue in.
		return ""
	default:
		// An unknown forge has no known tracker.
		return ""
	}
}

// issueTitle is the title the new issue carries: the title of the
// page, or its file name when the page has none.
func issueTitle(doc *Document) string {
	if title, ok := doc.FrontMatter["title"].(string); ok {
		if title = strings.TrimSpace(title); title != "" {
			return title
		}
	}
	return strings.TrimSpace(doc.Name)
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

// isForgeURL checks if a URL is a real forge URL (not a local path).
func isForgeURL(url string) bool {
	// Real forge URLs start with http://, https://, or git@
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return true
	}
	if strings.HasPrefix(url, "git@") {
		return true
	}
	// Anything else (./path, /path, relative paths) is local
	return false
}
