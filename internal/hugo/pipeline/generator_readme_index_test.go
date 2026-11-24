package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The README of a repository is the index of the site: the transform
// phase renames it to _index. The generated index must not be written
// to the same path - the content of the README was lost.
func TestTheReadmeOfTheRepositoryIsTheIndexOfTheSite(t *testing.T) {
	readme := &Document{
		Path:        "content/readme.md",
		Name:        "readme",
		Extension:   ".md",
		IsIndex:     true,
		Content:     "# My Project\n\nThe landing page of the project.\n",
		FrontMatter: map[string]any{"title": "My Project"},
	}

	ctx := &GenerationContext{
		Discovered:   []*Document{readme},
		IsSingleRepo: true,
	}

	generated, err := generateMainIndex(ctx)
	assert.NoError(t, err)
	assert.Len(t, generated, 1, "the landing page of the site is the generated index")
	assert.Contains(t, generated[0].Content, "The landing page of the project.",
		"the content of the README is the content of the landing page")
	assert.NotContains(t, ctx.Discovered, readme, "the README is the landing page now")
}

// Without a README the index is generated.
func TestTheIndexIsGeneratedWithoutAReadme(t *testing.T) {
	page := &Document{
		Path:        "content/guide.md",
		Name:        "guide",
		Extension:   ".md",
		Content:     "# Guide\n",
		FrontMatter: map[string]any{"title": "Guide"},
	}

	ctx := &GenerationContext{
		Discovered:   []*Document{page},
		IsSingleRepo: true,
	}

	generated, err := generateMainIndex(ctx)
	assert.NoError(t, err)
	assert.Len(t, generated, 1)
	assert.Equal(t, contentIndexPath, generated[0].Path)
}
