package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDocWithCategoriesInline(t *testing.T, dir, name string, categories []string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("  - ")
	b.WriteString(categories[0])
	b.WriteString("\n")
	for _, c := range categories[1:] {
		b.WriteString("  - ")
		b.WriteString(c)
		b.WriteString("\n")
	}
	catYAML := b.String()
	body := `---
title: DirCatTest
uid: aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
` + catYAML + `tags:
  - test
aliases:
  - /_uid/dircattest/
---

body
`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeDocNoCategories(t *testing.T, dir, name string) {
	t.Helper()
	body := `---
title: NoCats
uid: bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
tags:
  - test
aliases:
  - /_uid/nocats/
---

body
`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryCategoryConsistencyRule_Name(t *testing.T) {
	r := &DirectoryCategoryConsistencyRule{}
	if got := r.Name(); got != ruleDirectoryCategoryConsistency {
		t.Errorf("Name() = %q, want %q", got, ruleDirectoryCategoryConsistency)
	}
}

// TestDirectoryCategoryConsistencyRule_Match verifies how-to dir + how-to category passes.
func TestDirectoryCategoryConsistencyRule_Match(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "how-to")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeDocWithCategoriesInline(t, dir, "x.md", []string{"how-to"})

	r := &DirectoryCategoryConsistencyRule{}
	issues, err := r.Check(filepath.Join(dir, "x.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

// TestDirectoryCategoryConsistencyRule_Mismatch verifies how-to dir + reference category flags.
func TestDirectoryCategoryConsistencyRule_Mismatch(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "how-to")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeDocWithCategoriesInline(t, dir, "x.md", []string{"reference"})

	r := &DirectoryCategoryConsistencyRule{}
	issues, err := r.Check(filepath.Join(dir, "x.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
}

// TestDirectoryCategoryConsistencyRule_Missing verifies how-to dir + no categories flags.
func TestDirectoryCategoryConsistencyRule_Missing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "how-to")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeDocNoCategories(t, dir, "x.md")

	r := &DirectoryCategoryConsistencyRule{}
	issues, err := r.Check(filepath.Join(dir, "x.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
}

// TestDirectoryCategoryConsistencyRule_OutOfScope verifies a directory
// not in the map (e.g. misc) is not flagged.
func TestDirectoryCategoryConsistencyRule_OutOfScope(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "misc")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeDocNoCategories(t, dir, "x.md")

	r := &DirectoryCategoryConsistencyRule{}
	issues, err := r.Check(filepath.Join(dir, "x.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (out-of-scope dir), got %d", len(issues))
	}
}
