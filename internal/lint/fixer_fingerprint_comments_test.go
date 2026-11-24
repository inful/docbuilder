package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A fingerprint fix must not rewrite the whole frontmatter: the
// comments of the file and the order of its keys survive the fix, and
// the diff is the fingerprint (ADR-014).
func TestTheFingerprintFixKeepsTheCommentsAndTheOrder(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "page.md")
	content := `---
# NOTE: this comment must survive a fix
title: Page
uid: 55555555-aaaa-aaaa-aaaa-555555555555
aliases:
  - /_uid/55555555-aaaa-aaaa-aaaa-555555555555/
categories:
  - reference
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
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

	if !strings.Contains(text, "# NOTE: this comment must survive a fix") {
		t.Fatalf("the comment is gone: %s", text)
	}
	if !strings.Contains(text, "fingerprint:") {
		t.Fatalf("the fingerprint is missing: %s", text)
	}

	// The order of the keys is the order of the file.
	titleAt := strings.Index(text, "title:")
	uidAt := strings.Index(text, "uid:")
	categoriesAt := strings.Index(text, "categories:")
	if titleAt >= uidAt || uidAt >= categoriesAt {
		t.Fatalf("the keys were reordered: %s", text)
	}
}
