package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The title of a document that has no title of its own is the title of
// the file name. The H1 of the document replaces it - whatever the case
// of the file name is. "install.md" and "Install.md" are the same page.
func TestExtractH1AsTitle_TheH1ReplacesTheTitleOfTheFileName(t *testing.T) {
	for _, name := range []string{"install", "Install"} {
		doc := &Document{
			Name:        name,
			Content:     "# Installation Guide\n\nThe guide.\n",
			FrontMatter: map[string]any{"title": formatTitle(name)},
		}

		_, err := extractH1AsTitle(doc)

		assert.NoError(t, err)
		assert.Equal(t, "Installation Guide", doc.FrontMatter["title"])
	}
}

// A title of its own is kept.
func TestExtractH1AsTitle_ATitleOfItsOwnIsKept(t *testing.T) {
	doc := &Document{
		Name:        "install",
		Content:     "# Installation Guide\n\nThe guide.\n",
		FrontMatter: map[string]any{"title": "Own Title"},
	}

	_, err := extractH1AsTitle(doc)

	assert.NoError(t, err)
	assert.Equal(t, "Own Title", doc.FrontMatter["title"])
}
