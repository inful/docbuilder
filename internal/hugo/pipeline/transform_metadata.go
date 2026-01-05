package pipeline

import (
	"fmt"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

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
// Only generates URLs for documents with repository metadata.
// For preview mode in VS Code, VS Code edit URLs are handled separately.
func generateEditURL(doc *Document) string {
	// Preview mode with VS Code: generate local edit URL
	if doc.IsPreviewMode && doc.IsSingleRepo {
		// Check if we're in VS Code environment (VSCODE_* env vars)
		// This handler opens files directly in the VS Code editor
		if isVSCodeEnvironment() {
			return fmt.Sprintf("/_edit/%s", doc.RelativePath)
		}
	}

	// Production builds: generate forge-specific edit URLs
	// Use EditURLBase override if provided, otherwise use SourceURL
	var baseURL string
	switch {
	case doc.EditURLBase != "":
		// CLI override provided
		baseURL = strings.TrimSuffix(doc.EditURLBase, ".git")
	case doc.SourceURL != "" && isForgeURL(doc.SourceURL):
		// Use SourceURL if it's a real forge URL
		baseURL = strings.TrimSuffix(doc.SourceURL, ".git")
	default:
		// No valid base URL for edit links
		return ""
	}

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
	case config.ForgeLocal:
		// Local forges don't have web UI edit URLs
		return ""
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

// isVSCodeEnvironment checks if we're running inside VS Code or a devcontainer.
func isVSCodeEnvironment() bool {
	// Check for VS Code specific environment variables
	// VSCODE_GIT_IPC_HANDLE, TERM_PROGRAM=vscode, or REMOTE_CONTAINERS=true
	if os.Getenv("TERM_PROGRAM") == "vscode" {
		return true
	}
	if os.Getenv("VSCODE_GIT_IPC_HANDLE") != "" {
		return true
	}
	if os.Getenv("REMOTE_CONTAINERS") == "true" {
		return true
	}
	if os.Getenv("VSCODE_INJECTION") == "1" {
		return true
	}
	return false
}
