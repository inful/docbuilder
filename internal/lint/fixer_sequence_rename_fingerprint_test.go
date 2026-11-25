package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file that is renamed by the sequence-prefix fixer must not be
// fingerprinted under its old name: the old path is gone, the fixer
// read it, reported the error and the command exited with 2 after it
// had renamed the file.
func TestTheRenamedFileIsFingerprintedUnderItsNewName(t *testing.T) {
	tmpDir := t.TempDir()
	adrDir := filepath.Join(tmpDir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0o750); err != nil {
		t.Fatalf("make the directory: %v", err)
	}
	// A file without a fingerprint: the fixer writes one.
	doc := filepath.Join(adrDir, "adr-newthing.md")
	if err := os.WriteFile(doc, []byte(`---
title: New Thing
uid: 33333333-aaaa-aaaa-aaaa-333333333333
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
categories:
  - adr
tags:
  - test
---

# New Thing

body
`), 0o600); err != nil {
		t.Fatalf("write the document: %v", err)
	}

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	result, err := fixer.Fix(filepath.Join(tmpDir, "docs"))
	if err != nil {
		t.Fatalf("fix: %v", err)
	}

	if len(result.FilesRenamed) == 0 {
		t.Fatal("the file was not renamed")
	}
	if result.HasErrors() {
		t.Fatalf("the fixer reported errors: %v", result.Errors)
	}

	entries, err := os.ReadDir(adrDir)
	if err != nil {
		t.Fatalf("read the directory: %v", err)
	}
	renamed := ""
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "adr-0") {
			renamed = entry.Name()
		}
	}
	if renamed == "" {
		t.Fatalf("the file was not renamed: %v", entries)
	}

	// The new file holds the fingerprint.
	// #nosec G304 -- the path is built by the test itself.
	content, err := os.ReadFile(filepath.Join(adrDir, renamed))
	if err != nil {
		t.Fatalf("read the renamed file: %v", err)
	}
	if !strings.Contains(string(content), "fingerprint:") {
		t.Fatalf("the renamed file holds no fingerprint: %s", content)
	}
}
