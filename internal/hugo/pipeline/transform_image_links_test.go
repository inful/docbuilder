package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteImageLinks_Subdirectory(t *testing.T) {
	doc := &Document{
		Repository: "test-repo",
		Forge:      "",
		Section:    "guides", // Document is in docs/guides/
		Content: `# Tutorial

This is a guide with images.

![Test Image](images/test.png)
![Another](../assets/logo.png)
![Absolute](https://example.com/image.png)
![Root](/static/banner.jpg)
`,
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	expected := `# Tutorial

This is a guide with images.

![Test Image](/test-repo/guides/images/test.png)
![Another](/test-repo/guides/../assets/logo.png)
![Absolute](https://example.com/image.png)
![Root](/static/banner.jpg)
`

	assert.Equal(t, expected, doc.Content)
}

func TestRewriteImageLinks_RootLevel(t *testing.T) {
	doc := &Document{
		Repository: "test-repo",
		Forge:      "",
		Section:    "", // Document is at docs/ root
		Content: `# Guide

![Logo](images/logo.png)
`,
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	expected := `# Guide

![Logo](/test-repo/images/logo.png)
`

	assert.Equal(t, expected, doc.Content)
}

func TestRewriteImageLinks_WithForge(t *testing.T) {
	doc := &Document{
		Repository: "test-repo",
		Forge:      "gitlab",
		Section:    "api/v1",
		Content: `![Diagram](images/architecture.svg)`,
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	expected := `![Diagram](/gitlab/test-repo/api/v1/images/architecture.svg)`
	assert.Equal(t, expected, doc.Content)
}

func TestRewriteImageLinks_DeepSubdirectory(t *testing.T) {
	doc := &Document{
		Repository: "docs-repo",
		Forge:      "",
		Section:    "guides/advanced/security",
		Content: `![Security Model](diagrams/model.png)`,
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	// Image should be at /docs-repo/guides/advanced/security/diagrams/model.png
	expected := `![Security Model](/docs-repo/guides/advanced/security/diagrams/model.png)`
	assert.Equal(t, expected, doc.Content)
}

func TestRewriteImagePath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		repository string
		forge      string
		section    string
		expected   string
	}{
		{
			name:       "relative path in subdirectory",
			path:       "images/test.png",
			repository: "repo",
			forge:      "",
			section:    "guides",
			expected:   "/repo/guides/images/test.png",
		},
		{
			name:       "relative path at root",
			path:       "images/logo.png",
			repository: "repo",
			forge:      "",
			section:    "",
			expected:   "/repo/images/logo.png",
		},
		{
			name:       "relative path with forge",
			path:       "assets/icon.svg",
			repository: "repo",
			forge:      "github",
			section:    "docs",
			expected:   "/github/repo/docs/assets/icon.svg",
		},
		{
			name:       "absolute path unchanged",
			path:       "/static/image.png",
			repository: "repo",
			forge:      "",
			section:    "guides",
			expected:   "/static/image.png",
		},
		{
			name:       "deep section path",
			path:       "diagrams/flow.png",
			repository: "repo",
			forge:      "",
			section:    "api/v2/endpoints",
			expected:   "/repo/api/v2/endpoints/diagrams/flow.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteImagePath(tt.path, tt.repository, tt.forge, tt.section)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRewriteImageLinks_HTMLImgTags(t *testing.T) {
	doc := &Document{
		Repository: "test-repo",
		Forge:      "",
		Section:    "guides",
		Content: `# Guide

<img src="images/banner.jpg" alt="Banner" />
<img class="logo" src="assets/logo.png" alt="Logo" width="100" />
<img src="https://example.com/external.png" alt="External" />
<img src="/static/absolute.png" alt="Absolute" />
`,
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	expected := `# Guide

<img src="/test-repo/guides/images/banner.jpg" alt="Banner" />
<img class="logo" src="/test-repo/guides/assets/logo.png" alt="Logo" width="100" />
<img src="https://example.com/external.png" alt="External" />
<img src="/static/absolute.png" alt="Absolute" />
`

	assert.Equal(t, expected, doc.Content)
}

func TestRewriteImageLinks_MixedMarkdownAndHTML(t *testing.T) {
	doc := &Document{
		Repository: "docs",
		Forge:      "",
		Section:    "api",
		Content: `# API Documentation

![Architecture](diagrams/architecture.svg)

<img src="diagrams/flow.png" alt="Flow Diagram" />

Regular text with ![inline image](images/icon.png) continues here.
`,
	}

	_, err := rewriteImageLinks(doc)
	require.NoError(t, err)

	expected := `# API Documentation

![Architecture](/docs/api/diagrams/architecture.svg)

<img src="/docs/api/diagrams/flow.png" alt="Flow Diagram" />

Regular text with ![inline image](/docs/api/images/icon.png) continues here.
`

	assert.Equal(t, expected, doc.Content)
}
