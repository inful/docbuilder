package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The categories of a document are its doc mode: a document has one.
// The fixer joined the wrong category to the expected one, and the
// document ended up with two modes - an error that no fixer could
// remove.
func TestTheWrongCategoryIsReplacedAndNotJoined(t *testing.T) {
	tmpDir := t.TempDir()
	howTo := filepath.Join(tmpDir, "docs", "how-to")
	if err := os.MkdirAll(howTo, 0o750); err != nil {
		t.Fatalf("make the directory: %v", err)
	}
	doc := filepath.Join(howTo, "foo.md")
	if err := os.WriteFile(doc, []byte(`---
title: Foo
uid: 44444444-aaaa-aaaa-aaaa-444444444444
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - reference
tags:
  - test
---

# Foo

body
`), 0o600); err != nil {
		t.Fatalf("write the document: %v", err)
	}

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	if _, err := fixer.Fix(filepath.Join(tmpDir, "docs")); err != nil {
		t.Fatalf("fix: %v", err)
	}

	// #nosec G304 -- the path is built by the test itself.
	content, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("read the document: %v", err)
	}
	if strings.Contains(string(content), "reference") {
		t.Fatalf("the wrong category was kept: %s", content)
	}
	if !strings.Contains(string(content), "how-to") {
		t.Fatalf("the expected category is missing: %s", content)
	}

	// The document has one mode: the cross-mode rule is happy.
	res, err := linter.LintPath(filepath.Join(tmpDir, "docs"))
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	for _, issue := range res.Issues {
		if issue.Rule == ruleCrossModeCategory {
			t.Errorf("the document has two doc modes: %s", issue.Message)
		}
	}
}
