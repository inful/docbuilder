package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDocWithCatsRaw(t *testing.T, dir, name string, categoriesYAML string) {
	t.Helper()
	body := `---
title: CrossModeTest
uid: 99999999-9999-9999-9999-999999999999
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
` + categoriesYAML + `tags:
  - test
aliases:
  - /_uid/crossmodetest/
---

body
`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCrossModeCategoryRule_Name(t *testing.T) {
	r := &CrossModeCategoryRule{}
	if got := r.Name(); got != ruleCrossModeCategory {
		t.Errorf("Name() = %q, want %q", got, ruleCrossModeCategory)
	}
}

// TestCrossModeCategoryRule_Single verifies one category passes.
func TestCrossModeCategoryRule_Single(t *testing.T) {
	dir := t.TempDir()
	writeDocWithCatsRaw(t, dir, "single.md", "  - reference\n")

	r := &CrossModeCategoryRule{}
	issues, err := r.Check(filepath.Join(dir, "single.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

// TestCrossModeCategoryRule_TwoDistinct verifies two distinct modes flag.
func TestCrossModeCategoryRule_TwoDistinct(t *testing.T) {
	dir := t.TempDir()
	writeDocWithCatsRaw(t, dir, "two.md", "  - how-to\n  - reference\n")

	r := &CrossModeCategoryRule{}
	issues, err := r.Check(filepath.Join(dir, "two.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (one pair), got %d", len(issues))
	}
	if issues[0].Severity != SeverityError {
		t.Errorf("Severity = %v, want %v", issues[0].Severity, SeverityError)
	}
}

// TestCrossModeCategoryRule_ThreeDistinct verifies three distinct modes flag all pairs.
func TestCrossModeCategoryRule_ThreeDistinct(t *testing.T) {
	dir := t.TempDir()
	writeDocWithCatsRaw(t, dir, "three.md", "  - how-to\n  - reference\n  - explanation\n")

	r := &CrossModeCategoryRule{}
	issues, err := r.Check(filepath.Join(dir, "three.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// 3 pairs: how-to/reference, how-to/explanation, reference/explanation
	if len(issues) != 3 {
		t.Errorf("expected 3 issues (3 pairs), got %d", len(issues))
	}
}

// TestCrossModeCategoryRule_PairAllowedByConfig verifies AllowedPairs suppresses.
func TestCrossModeCategoryRule_PairAllowedByConfig(t *testing.T) {
	dir := t.TempDir()
	writeDocWithCatsRaw(t, dir, "allowed.md", "  - reference\n  - tutorials\n")

	r := &CrossModeCategoryRule{
		AllowedPairs: map[string]map[string]bool{
			"reference": {"tutorials": true},
		},
	}
	issues, err := r.Check(filepath.Join(dir, "allowed.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (pair allowed by config), got %d", len(issues))
	}
}
