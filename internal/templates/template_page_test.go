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
