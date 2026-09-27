package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixer_FrontmatterFieldsFillMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "how-to", "demo.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	// Doc missing title, date, lastmod, tags (only has uid).
	body := `---
uid: 11111111-aaaa-aaaa-aaaa-111111111111
categories:
  - how-to
---

# Demo Title

body
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	result, err := fixer.Fix(path)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(result.FrontmatterFields) == 0 {
		t.Errorf("expected FrontmatterFields to be populated, got 0")
	}

	// Re-read the file and confirm fields are present.
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after fix: %v", err)
	}
	for _, want := range []string{"title:", "date:", "lastmod:", "tags:"} {
		if !containsBytes(out, want) {
			t.Errorf("expected %q in fixed file, got:\n%s", want, string(out))
		}
	}
}

func TestFixer_FrontmatterFieldsDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := `---
uid: 22222222-aaaa-aaaa-aaaa-222222222222
---

body
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, true /* dryRun */, false).WithAutoConfirm(true)
	result, err := fixer.Fix(path)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(result.FrontmatterFields) == 0 {
		t.Errorf("expected dry-run to still report FrontmatterFields operations")
	}

	// File content should be unchanged.
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if containsBytes(out, "title:") {
		t.Errorf("dry-run should not modify file, got:\n%s", string(out))
	}
}

func TestFixer_DirectoryCategoryInject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "how-to", "demo.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	body := `---
title: Demo
uid: 33333333-aaaa-aaaa-aaaa-333333333333
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
tags:
  - test
aliases:
  - /_uid/x/
---

body
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	result, err := fixer.Fix(path)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(result.CategoryInjects) == 0 {
		t.Errorf("expected CategoryInjects to be populated, got 0")
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !containsBytes(out, "  - how-to") {
		t.Errorf("expected how-to category after fix, got:\n%s", string(out))
	}
}

// containsBytes is a tiny helper since strings.Contains is not in scope.
func containsBytes(haystack []byte, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == needle {
			return true
		}
	}
	return false
}
