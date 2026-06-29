package pipeline

import (
	"fmt"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
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

// generateEditURL creates a forge-appropriate edit URL for a document.
// Only generates URLs for documents with repository metadata.
// For preview mode in VS Code, VS Code edit URLs are handled separately.
//
// Splits the (clone|override) URL once via forge.SplitCloneURL and delegates
// the per-forge URL formatting to forge.GenerateEditURL, which is shared
// with internal/hugo/editlink.
func generateEditURL(doc *Document) string {
	// Preview mode with VS Code: generate local edit URL
	if doc.VSCodeEditLinks && doc.IsSingleRepo {
		// VS Code edit links enabled via --vscode flag
		// This handler opens files directly in the VS Code editor
		return fmt.Sprintf("/_edit/%s", doc.RelativePath)
	}

	// Production builds: generate forge-specific edit URLs.
	// Use EditURLBase override if provided, otherwise use SourceURL.
	var cloneURL string
	switch {
	case doc.EditURLBase != "":
		cloneURL = doc.EditURLBase
	case doc.SourceURL != "" && isForgeURL(doc.SourceURL):
		cloneURL = doc.SourceURL
	default:
		return ""
	}

	baseURL, fullName := forge.SplitCloneURL(cloneURL)
	if baseURL == "" || fullName == "" {
		return ""
	}

	// Determine branch (fallback to "main" if not set)
	branch := doc.SourceBranch
	if branch == "" {
		branch = "main"
	}

	// Build path relative to repository root. RelativePath is already
	// relative to docs base; prepend DocsBase when it isn't already.
	// Skip when DocsBase is "." (current-directory marker for local builds).
	filePath := doc.RelativePath
	if doc.DocsBase != "" && doc.DocsBase != "." && !strings.HasPrefix(filePath, doc.DocsBase+"/") {
		filePath = doc.DocsBase + "/" + filePath
	}

	// ForgeType is sourced from the explicit forge field (if set),
	// otherwise classified via the centralized forge.DetectForgeTypeFromURL.
	forgeType := detectForgeTypeFromField(doc.Forge, cloneURL)

	return forge.GenerateEditURL(forgeType, baseURL, fullName, branch, filePath)
}

// detectForgeTypeFromField consults the explicit forge metadata field
// first; on a miss or empty field it falls back to
// forge.DetectForgeTypeFromURL on the clone URL.
func detectForgeTypeFromField(forgeField, cloneURL string) config.ForgeType {
	switch strings.ToLower(forgeField) {
	case "github":
		return config.ForgeGitHub
	case "gitlab":
		return config.ForgeGitLab
	case "forgejo", "gitea":
		return config.ForgeForgejo
	}
	return forge.DetectForgeTypeFromURL(cloneURL)
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
