package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A link in a code block is an example, not a link: ADR-012 keeps the
// links of the code blocks out of the rewriting. A rewritten example
// shows a path that does not exist.
func TestRewriteRelativeLinks_TheCodeBlocksAreLeftAlone(t *testing.T) {
	cfg := &config.Config{}
	transform := rewriteRelativeLinks(cfg)

	doc := &Document{
		Path:       "content/docs/test.md",
		Repository: "docs",
		Section:    "section",
		Content:    "```markdown\nSee [other](other.md)\n```\n\nSee [other](other.md)\n",
	}

	_, err := transform(doc)

	assert.NoError(t, err)
	assert.Contains(t, doc.Content, "```markdown\nSee [other](other.md)\n```")
	assert.Contains(t, doc.Content, "\n\nSee [other](/docs/other)")
}

// An indented code block is code as well.
func TestRewriteRelativeLinks_TheIndentedCodeBlocksAreLeftAlone(t *testing.T) {
	cfg := &config.Config{}
	transform := rewriteRelativeLinks(cfg)

	doc := &Document{
		Path:       "content/docs/test.md",
		Repository: "docs",
		Section:    "section",
		Content:    "Text:\n\n    See [other](other.md)\n\nSee [other](other.md)\n",
	}

	_, err := transform(doc)

	assert.NoError(t, err)
	assert.Contains(t, doc.Content, "    See [other](other.md)\n")
	assert.Contains(t, doc.Content, "\n\nSee [other](/docs/other)")
}

// The images of a code block are examples too.
func TestRewriteImageLinks_TheCodeBlocksAreLeftAlone(t *testing.T) {
	doc := &Document{
		Path:       "content/docs/test.md",
		Repository: "docs",
		Section:    "section",
		Content:    "```markdown\n![diagram](img/diagram.png)\n```\n\n![diagram](img/diagram.png)\n",
	}

	_, err := rewriteImageLinks(doc)

	assert.NoError(t, err)
	assert.Contains(t, doc.Content, "```markdown\n![diagram](img/diagram.png)\n```")
	assert.Contains(t, doc.Content, "\n\n![diagram](/docs/section/img/diagram.png)")
}
