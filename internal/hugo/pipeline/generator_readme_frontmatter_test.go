package pipeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The front matter of the readme is not part of the landing page. The
// generation phase runs before the front matter is parsed, so the
// block would show up as text in the page.
func TestTheFrontMatterOfTheReadmeIsNotInTheLandingPage(t *testing.T) {
	readme := &Document{
		Path:      "content/readme.md",
		Name:      "readme",
		Extension: ".md",
		Content:   "---\ntitle: My Project\nweight: 5\n---\n\n# My Project\n\nThe landing page of the project.\n",
	}

	ctx := &GenerationContext{
		Discovered:   []*Document{readme},
		IsSingleRepo: true,
	}

	generated, err := generateMainIndex(ctx)
	assert.NoError(t, err)
	assert.Len(t, generated, 1)

	content := generated[0].Content
	assert.Contains(t, content, "The landing page of the project.")
	assert.NotContains(t, content, "weight: 5", "the front matter of the readme is in the landing page")
	assert.False(t, strings.Contains(content, "---"), "the front matter of the readme is in the landing page")
}
