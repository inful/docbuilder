package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The date of a document is written as the author wrote it: the fixer
// must not turn "2024-01-02" into "2024-01-02T00:00:00Z". The
// frontmatter is patched, not written again.
func TestTheDateOfTheDocumentKeepsItsForm(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "page.md")
	content := `---
# NOTE: this comment must survive a fix
title: Page
date: 2024-01-02
lastmod: "2024-01-02"
categories:
  - Reference
tags:
  - test
---

# Page

body
`
	if err := os.WriteFile(doc, []byte(content), 0o600); err != nil {
		t.Fatalf("write the document: %v", err)
	}

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	if _, err := fixer.Fix(dir); err != nil {
		t.Fatalf("fix: %v", err)
	}

	// #nosec G304 -- the path is built by the test itself.
	fixed, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("read the document: %v", err)
	}
	text := string(fixed)

	if !strings.Contains(text, "date: 2024-01-02\n") {
		t.Fatalf("the date changed its form: %s", text)
	}
	if !strings.Contains(text, "# NOTE: this comment must survive a fix") {
		t.Fatalf("the comment is gone: %s", text)
	}
}
