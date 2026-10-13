package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixer_ApplyTemplateUIDBindingFixes_AddsMissingUID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.md")
	content := wrappingTemplate(`---
title: "{{ .Title }}"
---

# {{ .Title }}

Canonical: /_uid/{{ .UID }}/
`)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	_, err := fixer.Fix(path)
	require.NoError(t, err)

	// #nosec G304 -- test-managed path
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	gotStr := string(got)

	// YAML serializes {{ .UID }} with single quotes by default; accept
	// either single or double quoting.
	hasBinding := strings.Contains(gotStr, "uid: '{{ .UID }}'") ||
		strings.Contains(gotStr, "uid: \"{{ .UID }}\"")
	require.True(t, hasBinding, "fixer should add uid: {{ .UID }} to the body frontmatter; got:\n%s", gotStr)

	// Existing content preserved. YAML quotes may be single or double.
	require.True(t,
		strings.Contains(gotStr, "title: '{{ .Title }}'") || strings.Contains(gotStr, "title: \"{{ .Title }}\""),
		"body's title should be preserved; got:\n%s", gotStr)
	require.Contains(t, gotStr, "/_uid/{{ .UID }}/")
}

func TestFixer_ApplyTemplateUIDBindingFixes_RewritesLiteralUID(t *testing.T) {
	// A literal UUID in the body's uid field is a mismatch (the body
	// uses {{ .UID }} but the frontmatter pins a different
	// identifier). The rule only fires on template files whose body
	// already uses {{ .UID }}, so the author clearly wanted the
	// template reference — the fixer rewrites the literal to it.
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.md")
	content := wrappingTemplate(`---
title: "{{ .Title }}"
uid: 11111111-1111-1111-1111-111111111111
---

# {{ .Title }}

Canonical: /_uid/{{ .UID }}/
`)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	_, err := fixer.Fix(path)
	require.NoError(t, err)

	// #nosec G304 -- test-managed path
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	gotStr := string(got)

	// Literal must be gone.
	require.NotContains(t, gotStr, "uid: 11111111-1111-1111-1111-111111111111",
		"literal uid should be replaced; got:\n%s", gotStr)
	// Template reference must be in place.
	hasBinding := strings.Contains(gotStr, "uid: '{{ .UID }}'") ||
		strings.Contains(gotStr, "uid: \"{{ .UID }}\"")
	require.True(t, hasBinding,
		"body's uid should be rewritten to {{ .UID }}; got:\n%s", gotStr)
}

func TestFixer_ApplyTemplateUIDBindingFixes_NoChangeWhenBound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.md")
	content := wrappingTemplate(`---
title: "{{ .Title }}"
uid: "{{ .UID }}"
---

# {{ .Title }}
`)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	// Run only the template-uid-binding fixer phase so we can assert
	// it specifically does no work. The other fixers will still touch
	// the outer file, which is fine — they're orthogonal.
	original := content
	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	fixResult := &FixResult{}
	fixer.applyTemplateUIDBindingFixes(
		map[string]struct{}{path: {}},
		map[string]int{path: 1},
		fixResult,
		map[string]struct{}{},
	)
	assert.Equal(t, 0, fixResult.ErrorsFixed, "no issues should be reported as fixed when binding is already present")

	// #nosec G304 -- test-managed path
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, string(got), "fixer should not touch a file whose body already binds uid")
}

func TestFixer_ApplyTemplateUIDBindingFixes_NonTemplateUntouched(t *testing.T) {
	// The template-uid-binding rule must not modify a non-template
	// file. We test the fixer phase directly to isolate it from the
	// other fixers that would otherwise touch the file's frontmatter.
	dir := t.TempDir()
	path := filepath.Join(dir, "regular.md")
	content := `---
title: "Regular Doc"
uid: 11111111-1111-1111-1111-111111111111
---

# Regular Doc

Body references {{ .UID }} but this is not a template.
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	original := content

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	targets := map[string]struct{}{path: {}}
	issueCounts := map[string]int{path: 1}
	fixResult := &FixResult{}
	fixer.applyTemplateUIDBindingFixes(targets, issueCounts, fixResult, map[string]struct{}{})

	// #nosec G304 -- test-managed path
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, string(got), "non-template file must not be touched by template-uid-binding fixer")
}

func TestFixer_ApplyTemplateUIDBindingFixes_PreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.md")
	// CRLF line endings throughout.
	inner := "---\r\ntitle: \"{{ .Title }}\"\r\n---\r\n\r\n# {{ .Title }}\r\n\r\nCanonical: /_uid/{{ .UID }}/\r\n"
	content := "---\r\ntitle: \"Test Template\"\r\ncategories:\r\n  - templates\r\nparams:\r\n  docbuilder:\r\n    template:\r\n      type: \"test\"\r\n      name: \"Test\"\r\n      output_path: \"out/{{ .Slug }}.md\"\r\n---\r\n\r\n# Test Template\r\n\r\n```markdown\r\n" + inner + "```\r\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	_, err := fixer.Fix(path)
	require.NoError(t, err)

	// #nosec G304 -- test-managed path
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	gotStr := string(got)

	// YAML serializes the {{ .UID }} value with single quotes by
	// default; accept either single or double quoting.
	hasBinding := strings.Contains(gotStr, "uid: '{{ .UID }}'") ||
		strings.Contains(gotStr, "uid: \"{{ .UID }}\"")
	require.True(t, hasBinding, "fixer should add uid: {{ .UID }} to the body frontmatter; got:\n%s", gotStr)

	// CRLF preserved in the body.
	require.Contains(t, gotStr, "\r\n", "CRLF line endings must be preserved")
	require.False(t, strings.Contains(strings.ReplaceAll(gotStr, "\r\n", ""), "\n"),
		"file should not contain lone LF after fix")
}

func TestFixer_ApplyTemplateUIDBindingFixes_PreservesBodyH1(t *testing.T) {
	// The body H1 should not be re-extracted; the fix only touches
	// the embedded frontmatter.
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.md")
	content := wrappingTemplate(`---
title: "{{ .Title }}"
---

# {{ .Title }}

Canonical: /_uid/{{ .UID }}/
`)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	_, err := fixer.Fix(path)
	require.NoError(t, err)

	// #nosec G304 -- test-managed path
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	gotStr := string(got)
	// Count occurrences of the H1 — should be exactly one.
	assert.Equal(t, 1, strings.Count(gotStr, "# {{ .Title }}"),
		"fixer should not duplicate the H1; got:\n%s", gotStr)
}
