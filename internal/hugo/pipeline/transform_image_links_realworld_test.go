package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteImageLinks_DeepNestedRealWorld(t *testing.T) {
	// Real-world case from test-doc repository
	// File: docs/lvl1/lvl2/level2.md
	// Section: lvl1/lvl2
	// References: same_dir.png (should resolve to /test-doc/lvl1/lvl2/same_dir.png)
	doc := &Document{
		Repository: "test-doc",
		Forge:      "",
		Section:    "lvl1/lvl2",
		Name:       "level2",
		Content: `# This is level 2

We have some images linked:

- ![the_same_directory](same_dir.png)
- ![different_dir](images/different_dir.png)
- ![relative_dir](../images/logo-2.png)
`,
	}

	t.Logf("Before transform - Section: '%s'", doc.Section)
	t.Logf("Before transform - Repository: '%s'", doc.Repository)
	t.Logf("Before transform:\n%s", doc.Content)

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	t.Logf("After transform:\n%s", doc.Content)

	// Check that the image paths are correctly rewritten
	assert.Contains(t, doc.Content, "![the_same_directory](/test-doc/lvl1/lvl2/same_dir.png)",
		"same_dir.png should resolve to /test-doc/lvl1/lvl2/same_dir.png")
	assert.Contains(t, doc.Content, "![different_dir](/test-doc/lvl1/lvl2/images/different_dir.png)",
		"images/different_dir.png should resolve to /test-doc/lvl1/lvl2/images/different_dir.png")
	assert.Contains(t, doc.Content, "![relative_dir](/test-doc/lvl1/lvl2/../images/logo-2.png)",
		"../images/logo-2.png should resolve with ../ preserved")
}

func TestFullPipelineWithDeepNesting(t *testing.T) {
	// Test the FULL pipeline to see if something else is stripping the section
	doc := &Document{
		Repository: "test-doc",
		Forge:      "",
		Section:    "lvl1/lvl2",
		Name:       "level2",
		Path:       "content/test-doc/lvl1/lvl2/level2.md",
		Content: `# This is level 2

- ![the_same_directory](same_dir.png)
- ![different_dir](images/different_dir.png)
`,
		FrontMatter: make(map[string]any),
	}

	t.Logf("Initial Document - Section: '%s', Repository: '%s'", doc.Section, doc.Repository)

	// Just test image rewriting
	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	t.Logf("After image rewriting:\n%s\n", doc.Content)

	// Final content should have correct paths
	assert.Contains(t, doc.Content, "/test-doc/lvl1/lvl2/same_dir.png",
		"Content should have correct image path with section")
	assert.Contains(t, doc.Content, "/test-doc/lvl1/lvl2/images/different_dir.png",
		"Content should have correct image path for subdirectory")
}
