package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDocWithCategories(t *testing.T, dir, name string, categories []string) {
	t.Helper()
	catYAML := "  - " + categories[0] + "\n"
	for _, c := range categories[1:] {
		catYAML += "  - " + c + "\n"
	}
	body := `---
title: CatTest
uid: 88888888-8888-8888-8888-888888888888
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
` + catYAML + `tags:
  - test
aliases:
  - /_uid/cattest/
---

body
`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCategoryNamingRule_Name(t *testing.T) {
	r := &CategoryNamingRule{}
	if got := r.Name(); got != ruleCategoryNaming {
		t.Errorf("Name() = %q, want %q", got, ruleCategoryNaming)
	}
}

// TestCategoryNamingRule_Passthrough verifies all well-formed categories pass.
func TestCategoryNamingRule_Passthrough(t *testing.T) {
	dir := t.TempDir()
	writeDocWithCategories(t, dir, "good.md", []string{"how-to", "reference", "explanation"})

	r := &CategoryNamingRule{}
	issues, err := r.Check(filepath.Join(dir, "good.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

// TestCategoryNamingRule_SingleBad verifies a single capitalized category is flagged.
func TestCategoryNamingRule_SingleBad(t *testing.T) {
	dir := t.TempDir()
	writeDocWithCategories(t, dir, "bad.md", []string{"Templates"})

	r := &CategoryNamingRule{}
	issues, err := r.Check(filepath.Join(dir, "bad.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Rule != ruleCategoryNaming {
		t.Errorf("Rule = %q, want %q", issues[0].Rule, ruleCategoryNaming)
	}
	if issues[0].Severity != SeverityError {
		t.Errorf("Severity = %v, want %v", issues[0].Severity, SeverityError)
	}
}

// TestCategoryNamingRule_Mixed verifies mixed categories flag only the bad one.
func TestCategoryNamingRule_Mixed(t *testing.T) {
	dir := t.TempDir()
	writeDocWithCategories(t, dir, "mixed.md", []string{"Foo", "bar-baz"})

	r := &CategoryNamingRule{}
	issues, err := r.Check(filepath.Join(dir, "mixed.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (only Foo), got %d", len(issues))
	}
}

// TestCategoryNamingRule_EmptyList verifies no issues when categories is empty.
func TestCategoryNamingRule_EmptyList(t *testing.T) {
	dir := t.TempDir()
	writeDocWithCategories(t, dir, "empty.md", []string{"how-to"})

	r := &CategoryNamingRule{}
	issues, err := r.Check(filepath.Join(dir, "empty.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}
