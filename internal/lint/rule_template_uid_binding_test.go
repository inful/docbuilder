package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// templateFile writes a template file to a temp dir and returns the path.
func templateFile(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

// wrappingTemplate wraps a body in a valid template frontmatter + fenced
// markdown block, with the body being the content of the fenced block.
func wrappingTemplate(body string) string {
	return `---
title: "Test Template"
categories:
  - templates
params:
  docbuilder:
    template:
      type: "test"
      name: "Test"
      output_path: "out/{{ .Slug }}.md"
---

# Test Template

` + "```markdown\n" + body + "\n```\n"
}

func TestTemplateUIDBindingRule_AppliesTo(t *testing.T) {
	rule := &TemplateUIDBindingRule{}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "applies to .md file", path: "test.md", want: true},
		{name: "applies to .markdown file", path: "test.markdown", want: true},
		{name: "excludes .png", path: "test.png", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rule.AppliesTo(tt.path))
		})
	}
}

func TestTemplateUIDBindingRule_Check_NonTemplate(t *testing.T) {
	// File is a regular doc, not a template. Rule should not flag it.
	rule := &TemplateUIDBindingRule{}
	path := templateFile(t, "regular.md", `---
title: "Regular Doc"
uid: 11111111-1111-1111-1111-111111111111
---

# Regular Doc

Body references {{ .UID }} but that's fine — this isn't a template.
`)
	issues, err := rule.Check(path)
	require.NoError(t, err)
	assert.Empty(t, issues, "non-template files should be skipped, got: %+v", issues)
}

func TestTemplateUIDBindingRule_Check_NoUIDReference(t *testing.T) {
	// Template body uses no .UID references — no issue.
	rule := &TemplateUIDBindingRule{}
	content := wrappingTemplate(`---
title: "{{ .Title }}"
---

# {{ .Title }}
`)
	path := templateFile(t, "tpl.md", content)
	issues, err := rule.Check(path)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestTemplateUIDBindingRule_Check_UIDInFrontmatter(t *testing.T) {
	// Body references {{ .UID }} and frontmatter binds it. No issue.
	rule := &TemplateUIDBindingRule{}
	content := wrappingTemplate(`---
title: "{{ .Title }}"
uid: "{{ .UID }}"
---

# {{ .Title }}

Canonical: /_uid/{{ .UID }}/
`)
	path := templateFile(t, "tpl.md", content)
	issues, err := rule.Check(path)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestTemplateUIDBindingRule_Check_UIDMissingInFrontmatter(t *testing.T) {
	// Body references {{ .UID }} but the embedded frontmatter doesn't
	// bind it. Should flag.
	rule := &TemplateUIDBindingRule{}
	content := wrappingTemplate(`---
title: "{{ .Title }}"
---

# {{ .Title }}

Canonical: /_uid/{{ .UID }}/
`)
	path := templateFile(t, "tpl.md", content)
	issues, err := rule.Check(path)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, ruleTemplateUIDBinding, issues[0].Rule)
	assert.Equal(t, SeverityError, issues[0].Severity)
	assert.Contains(t, issues[0].Message, "uid")
	assert.Contains(t, issues[0].Explanation, "{{ .UID }}")
	assert.Contains(t, issues[0].Fix, "uid: \"{{ .UID }}\"")
}

func TestTemplateUIDBindingRule_Check_UIDLiteralInFrontmatter(t *testing.T) {
	// Body references {{ .UID }} but the embedded frontmatter has a
	// literal UUID. That's a mismatch — flag without auto-fix.
	rule := &TemplateUIDBindingRule{}
	content := wrappingTemplate(`---
title: "{{ .Title }}"
uid: 11111111-1111-1111-1111-111111111111
---

# {{ .Title }}

Canonical: /_uid/{{ .UID }}/
`)
	path := templateFile(t, "tpl.md", content)
	issues, err := rule.Check(path)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, ruleTemplateUIDBinding, issues[0].Rule)
	assert.Contains(t, issues[0].Explanation, "literal")
}

func TestTemplateUIDBindingRule_Check_NoFencedBlock(t *testing.T) {
	// Template has no fenced code block. The parser itself rejects
	// such templates, but the rule should not crash.
	rule := &TemplateUIDBindingRule{}
	content := `---
title: "Test Template"
params:
  docbuilder:
    template:
      type: "test"
      name: "Test"
      output_path: "out/{{ .Slug }}.md"
---

# No body here
`
	path := templateFile(t, "tpl.md", content)
	issues, err := rule.Check(path)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestTemplateUIDBindingRule_Check_UIDInOutputPath(t *testing.T) {
	// The .UID reference is in the output_path, not the body. The body
	// itself doesn't use it. The rule should NOT flag — only the body
	// matters for the binding check.
	rule := &TemplateUIDBindingRule{}
	// The template meta references .UID via output_path; the body does
	// not. Wrap with a custom outer block that doesn't use .UID in the
	// body.
	content := `---
title: "Test Template"
params:
  docbuilder:
    template:
      type: "test"
      name: "Test"
      output_path: "out/{{ .UID }}.md"
---

# Body

` + "```markdown\n---\ntitle: \"{{ .Title }}\"\n---\n\n# {{ .Title }}\n```\n"
	path := templateFile(t, "tpl.md", content)
	issues, err := rule.Check(path)
	require.NoError(t, err)
	assert.Empty(t, issues, "UID only in output_path, not body — should be fine")
}

func TestTemplateUIDBindingRule_Check_MarkdownFenceVariant(t *testing.T) {
	// The fence language is `md` instead of `markdown`.
	rule := &TemplateUIDBindingRule{}
	content := stringsReplace(wrappingTemplate(`---
title: "{{ .Title }}"
---

# {{ .Title }}

Canonical: /_uid/{{ .UID }}/
`), "```markdown", "```md")
	path := templateFile(t, "tpl.md", content)
	issues, err := rule.Check(path)
	require.NoError(t, err)
	require.Len(t, issues, 1, "rule should still trigger with ```md fence")
}

// stringsReplace is a tiny helper to keep test inputs readable.
func stringsReplace(s, old, replacement string) string {
	out := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out = append(out, replacement...)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}
