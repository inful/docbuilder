package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A fix must not rewrite the whole frontmatter: the comments of the
// file and the order of its keys survive the fix, and the diff is the
// field that was added (ADR-014).
//
// The fingerprint fix, the uid fix and the fix that fills the required
// fields all patch the raw frontmatter.
func TestTheFixesKeepTheCommentsAndTheOrder(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "page.md")
	content := `---
# NOTE: this comment must survive a fix
title: Page
uid: 55555555-aaaa-aaaa-aaaa-555555555555
categories:
  - reference
---

# Page

body
`
	if err := os.WriteFile(doc, []byte(content), 0o600); err != nil {
		t.Fatalf("write the document: %v", err)
	}

	linter := NewLinter(&Config{})
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

	if !strings.Contains(text, "# NOTE: this comment must survive a fix") {
		t.Fatalf("the comment is gone: %s", text)
	}
	// The fields the fixers added are there.
	for _, field := range []string{"fingerprint:", "aliases:", "date:", "lastmod:", "tags:"} {
		if !strings.Contains(text, field) {
			t.Fatalf("the fixer did not add %q: %s", field, text)
		}
	}

	// The order of the keys is the order of the file.
	titleAt := strings.Index(text, "title:")
	uidAt := strings.Index(text, "uid:")
	categoriesAt := strings.Index(text, "categories:")
	if titleAt >= uidAt || uidAt >= categoriesAt {
		t.Fatalf("the keys were reordered: %s", text)
	}
}
