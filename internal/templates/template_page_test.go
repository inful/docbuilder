package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTemplatePage_Valid(t *testing.T) {
	html := `
		<html>
			<head>
				<meta property="docbuilder:template.type" content="adr">
				<meta property="docbuilder:template.name" content="Architecture Decision Record">
				<meta property="docbuilder:template.output_path" content='adr/adr-{{ printf "%03d" (nextInSequence "adr") }}-{{ .Slug }}.md'>
				<meta property="docbuilder:template.description" content="Scaffold ADR">
			</head>
			<body>
				<pre><code class="language-markdown">---
title: {{ .Title }}
---</code></pre>
			</body>
		</html>`

	page, err := ParseTemplatePage(strings.NewReader(html))
	require.NoError(t, err)
	require.Equal(t, "adr", page.Meta.Type)
	require.Equal(t, "Architecture Decision Record", page.Meta.Name)
	require.Equal(t, "adr/adr-{{ printf \"%03d\" (nextInSequence \"adr\") }}-{{ .Slug }}.md", page.Meta.OutputPath)
	require.Equal(t, "Scaffold ADR", page.Meta.Description)
	require.Equal(t, "---\ntitle: {{ .Title }}\n---", page.Body)
}

func TestParseTemplatePage_MissingRequiredMeta(t *testing.T) {
	html := `
		<html>
			<head>
				<meta property="docbuilder:template.type" content="adr">
				<meta property="docbuilder:template.name" content="Architecture Decision Record">
			</head>
			<body>
				<pre><code class="language-markdown"># body</code></pre>
			</body>
		</html>`

	_, err := ParseTemplatePage(strings.NewReader(html))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required template metadata")
	require.Contains(t, err.Error(), "docbuilder:template.output_path")
}

func TestParseTemplatePage_MultipleMarkdownBlocks(t *testing.T) {
	html := `
		<html>
			<head>
				<meta property="docbuilder:template.type" content="adr">
				<meta property="docbuilder:template.name" content="Architecture Decision Record">
				<meta property="docbuilder:template.output_path" content="adr/adr-001.md">
			</head>
			<body>
				<pre><code class="language-markdown"># body</code></pre>
				<pre><code class="language-md"># body2</code></pre>
			</body>
		</html>`

	_, err := ParseTemplatePage(strings.NewReader(html))
	require.Error(t, err)
}

func TestIsMarkdownCodeNode_VariousClasses(t *testing.T) {
	testCases := []struct {
		name  string
		html  string
		valid bool
	}{
		{
			name:  "language-markdown",
			html:  `<pre><code class="language-markdown"># test</code></pre>`,
			valid: true,
		},
		{
			name:  "language-md",
			html:  `<pre><code class="language-md"># test</code></pre>`,
			valid: true,
		},
		{
			name:  "lang-markdown",
			html:  `<pre><code class="lang-markdown"># test</code></pre>`,
			valid: true,
		},
		{
			name:  "lang-md",
			html:  `<pre><code class="lang-md"># test</code></pre>`,
			valid: true,
		},
		{
			name:  "markdown",
			html:  `<pre><code class="markdown"># test</code></pre>`,
			valid: true,
		},
		{
			name:  "not in pre",
			html:  `<code class="language-markdown"># test</code>`,
			valid: false,
		},
		{
			name:  "wrong language",
			html:  `<pre><code class="language-python">print("test")</code></pre>`,
			valid: false,
		},
		{
			name:  "no class",
			html:  `<pre><code># test</code></pre>`,
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			html := `<html><head>
				<meta property="docbuilder:template.type" content="adr">
				<meta property="docbuilder:template.name" content="ADR">
				<meta property="docbuilder:template.output_path" content="adr/adr-001.md">
			</head><body>` + tc.html + `</body></html>`

			page, err := ParseTemplatePage(strings.NewReader(html))
			if tc.valid {
				require.NoError(t, err)
				require.NotNil(t, page)
			} else if err == nil {
				// If not valid, it should either error or not find the code block
				require.Empty(t, page.Body)
			}
		})
	}
}
