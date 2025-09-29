package hugo

// Backward compatible wrapper forwarding to new content package implementation.
import (
	c "git.home.luguber.info/inful/docbuilder/internal/hugo/content"
)

// RewriteRelativeMarkdownLinks delegates to content.RewriteRelativeMarkdownLinks.
func RewriteRelativeMarkdownLinks(content string) string {
	return c.RewriteRelativeMarkdownLinks(content)
}
