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

func TestParseTemplatePage_ValidFromMetaName(t *testing.T) {
	html := `
		<html>
			<head>
				<meta name="docbuilder:template.type" content="general-meeting-minutes">
				<meta name="docbuilder:template.name" content="Meeting Minutes">
				<meta name="docbuilder:template.output_path" content="minutes/{{ .Date }}-{{ .Slug }}.md">
			</head>
			<body>
				<pre><code class="language-markdown">---
title: {{ .Title }}
---</code></pre>
			</body>
		</html>`

	page, err := ParseTemplatePage(strings.NewReader(html))
	require.NoError(t, err)
	require.Equal(t, "general-meeting-minutes", page.Meta.Type)
	require.Equal(t, "Meeting Minutes", page.Meta.Name)
	require.Equal(t, "minutes/{{ .Date }}-{{ .Slug }}.md", page.Meta.OutputPath)
}

func TestParseTemplatePage_ValidFromParamsMetaName(t *testing.T) {
	html := `
		<html>
			<head>
				<meta name="params.docbuilder.template.type" content="general-meeting-minutes">
				<meta name="params.docbuilder.template.name" content="Meeting Minutes">
				<meta name="params.docbuilder.template.output_path" content="minutes/{{ .Date }}-{{ .Slug }}.md">
				<meta name="params.docbuilder.template.description" content="Create a meeting minutes document">
			</head>
			<body>
				<pre><code class="language-markdown">---
title: {{ .Title }}
---</code></pre>
			</body>
		</html>`

	page, err := ParseTemplatePage(strings.NewReader(html))
	require.NoError(t, err)
	require.Equal(t, "general-meeting-minutes", page.Meta.Type)
	require.Equal(t, "Meeting Minutes", page.Meta.Name)
	require.Equal(t, "minutes/{{ .Date }}-{{ .Slug }}.md", page.Meta.OutputPath)
	require.Equal(t, "Create a meeting minutes document", page.Meta.Description)
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

func TestParseTemplatePage_PreservesNewlinesWithHighlightedCode(t *testing.T) {
	html := `
		<html>
			<head>
				<meta property="docbuilder:template.type" content="adr">
				<meta property="docbuilder:template.name" content="Architecture Decision Record">
				<meta property="docbuilder:template.output_path" content="adr/adr-001.md">
			</head>
			<body>
				<pre><code class="language-markdown"><span class="line">---</span>
<span class="line">title: "{{ .Title }}"</span>
<span class="line">categories:</span>
<span class="line">  - {{ index .categories 0 }}</span>
<span class="line">---</span>

<span class="line"># {{ .Title }}</span>
</code></pre>
			</body>
		</html>`

	page, err := ParseTemplatePage(strings.NewReader(html))
	require.NoError(t, err)
	require.Contains(t, page.Body, "---\n")
	require.Contains(t, page.Body, "title: \"{{ .Title }}\"\n")
	require.Contains(t, page.Body, "# {{ .Title }}")
	// Most importantly: we should still have newlines between lines.
	require.Contains(t, page.Body, "---\n\n#")
}

// A syntax highlighter that numbers the lines puts the number of the
// line in front of the code. The number belongs to the page and not to
// the template: a template that carried the numbers would write them
// in front of every line of every document it creates.
//
// This is the markup that Hugo and Chroma write when the site numbers
// the lines of its code blocks.
func TestParseTemplatePage_DropsTheLineNumbersOfTheHighlighter(t *testing.T) {
	html := `
		<html>
			<head>
				<meta property="docbuilder:template.type" content="adr">
				<meta property="docbuilder:template.name" content="Architecture Decision Record">
				<meta property="docbuilder:template.output_path" content="adr/adr-001.md">
			</head>
			<body>
				<div class="highlight"><pre tabindex="0" class="chroma"><code class="language-markdown" data-lang="markdown"><span class="line"><span class="ln">1</span><span class="cl"><span class="nn">---</span><span class="w">
</span></span></span><span class="line"><span class="ln">2</span><span class="cl"><span class="nt">title</span><span class="p">:</span><span class="w"> </span><span class="s2">&#34;{{ .Title }}&#34;</span><span class="w">
</span></span></span></code></pre></div>
			</body>
		</html>`

	page, err := ParseTemplatePage(strings.NewReader(html))
	require.NoError(t, err)
	require.Equal(t, "---\ntitle: \"{{ .Title }}\"", page.Body)
}

// A highlighter that puts the numbers in a column of their own writes
// them in a code block of their own. The block of the template is the
// one that carries the language.
func TestParseTemplatePage_DropsTheColumnOfLineNumbers(t *testing.T) {
	html := `
		<html>
			<head>
				<meta property="docbuilder:template.type" content="adr">
				<meta property="docbuilder:template.name" content="Architecture Decision Record">
				<meta property="docbuilder:template.output_path" content="adr/adr-001.md">
			</head>
			<body>
				<div class="highlight"><div class="chroma">
<table class="lntable"><tr><td class="lntd">
<pre tabindex="0" class="chroma"><code><span class="lnt">1
</span><span class="lnt">2
</span></code></pre></td>
<td class="lntd">
<pre tabindex="0" class="chroma"><code class="language-markdown" data-lang="markdown"><span class="line"><span class="cl"><span class="nn">---</span><span class="w">
</span></span></span><span class="line"><span class="cl"><span class="nt">title</span><span class="p">:</span><span class="w"> </span><span class="s2">&#34;{{ .Title }}&#34;</span><span class="w">
</span></span></span></code></pre></td>
</tr></table></div></div>
			</body>
		</html>`

	page, err := ParseTemplatePage(strings.NewReader(html))
	require.NoError(t, err)
	require.Equal(t, "---\ntitle: \"{{ .Title }}\"", page.Body)
}

// A highlighter that gives its spans the class of a line number may
// hold the code of the line in them. The number is a number: an
// element that holds something else is the code, and the code stays.
func TestParseTemplatePage_KeepsTheCodeOfALineNumberClass(t *testing.T) {
	html := `
		<html>
			<head>
				<meta property="docbuilder:template.type" content="adr">
				<meta property="docbuilder:template.name" content="Architecture Decision Record">
				<meta property="docbuilder:template.output_path" content="adr/adr-001.md">
			</head>
			<body>
				<pre><code class="language-markdown"><span class="ln">---</span>
<span class="ln">title: "{{ .Title }}"</span>
</code></pre>
			</body>
		</html>`

	page, err := ParseTemplatePage(strings.NewReader(html))
	require.NoError(t, err)
	require.Equal(t, "---\ntitle: \"{{ .Title }}\"", page.Body)
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
