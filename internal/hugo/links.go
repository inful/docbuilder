package hugo

// Backward compatible wrapper forwarding to new content package implementation.
import (
	c "git.home.luguber.info/inful/docbuilder/internal/hugo/content"
)

// RewriteRelativeMarkdownLinks delegates to content.RewriteRelativeMarkdownLinks.
// If repositoryName is provided, links starting with / are treated as repository-root-relative.
func RewriteRelativeMarkdownLinks(content string, repositoryName string, forgeName string) string {
	return c.RewriteRelativeMarkdownLinks(content, repositoryName, forgeName)
}
