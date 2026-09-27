package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDocWithTags(t *testing.T, dir, name string, tags []string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("  - ")
	b.WriteString(tags[0])
	b.WriteString("\n")
	for _, tg := range tags[1:] {
		b.WriteString("  - ")
		b.WriteString(tg)
		b.WriteString("\n")
	}
	tagsYAML := b.String()
	body := `---
title: TagsTest
uid: 66666666-6666-6666-6666-666666666666
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
` + tagsYAML + `aliases:
  - /_uid/tagstest/
---

body
`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTagCountRule_Name(t *testing.T) {
	r := &TagCountRule{}
	if got := r.Name(); got != ruleTagCount {
		t.Errorf("Name() = %q, want %q", got, ruleTagCount)
	}
}

// TestTagCountRule_UnderThreshold verifies a doc with 5 tags passes
// (default threshold 10).
func TestTagCountRule_UnderThreshold(t *testing.T) {
	dir := t.TempDir()
	writeDocWithTags(t, dir, "under.md", []string{"a", "b", "c", "d", "e"})

	r := &TagCountRule{}
	issues, err := r.Check(filepath.Join(dir, "under.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (5 ≤ 10), got %d", len(issues))
	}
}

// TestTagCountRule_AtBoundary verifies a doc with exactly the threshold (10) passes.
func TestTagCountRule_AtBoundary(t *testing.T) {
	dir := t.TempDir()
	tags := []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9", "t10"}
	writeDocWithTags(t, dir, "boundary.md", tags)

	r := &TagCountRule{}
	issues, err := r.Check(filepath.Join(dir, "boundary.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (10 = boundary), got %d", len(issues))
	}
}

// TestTagCountRule_OverThreshold verifies a doc with 11 tags produces a WARNING.
func TestTagCountRule_OverThreshold(t *testing.T) {
	dir := t.TempDir()
	tags := []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9", "t10", "t11"}
	writeDocWithTags(t, dir, "over.md", tags)

	r := &TagCountRule{}
	issues, err := r.Check(filepath.Join(dir, "over.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue (11 > 10), got %d", len(issues))
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("Severity = %v, want %v", issues[0].Severity, SeverityWarning)
	}
}

// TestTagCountRule_CustomThreshold verifies Threshold is honored.
func TestTagCountRule_CustomThreshold(t *testing.T) {
	dir := t.TempDir()
	writeDocWithTags(t, dir, "x.md", []string{"a", "b", "c"})

	r := &TagCountRule{Threshold: 2}
	issues, err := r.Check(filepath.Join(dir, "x.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("Threshold=2 with 3 tags: expected 1 issue, got %d", len(issues))
	}
}
