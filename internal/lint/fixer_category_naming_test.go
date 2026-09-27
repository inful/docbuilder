package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixer_CategoryNamingFix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := `---
title: Demo
uid: 44444444-aaaa-aaaa-aaaa-444444444444
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - Templates
  - foo_bar
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
	if len(result.CategoryRenames) == 0 {
		t.Errorf("expected CategoryRenames to be populated, got 0")
	}

	out, err := os.ReadFile(path) // #nosec G304 -- test reads a temp file path under t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !containsBytes(out, "  - templates") {
		t.Errorf("expected lowercased templates category, got:\n%s", got)
	}
	if !containsBytes(out, "  - foo-bar") {
		t.Errorf("expected foo-bar (underscore replaced), got:\n%s", got)
	}
	if containsBytes(out, "  - Templates") {
		t.Errorf("expected Templates to be lowercased away, got:\n%s", got)
	}
}

func TestFixer_CategoryNamingDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := `---
title: Demo
uid: 55555555-aaaa-aaaa-aaaa-555555555555
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - Templates
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
	fixer := NewFixer(linter, true, false).WithAutoConfirm(true)
	result, err := fixer.Fix(path)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(result.CategoryRenames) == 0 {
		t.Errorf("expected dry-run to still report CategoryRenames operations")
	}
	out, err := os.ReadFile(path) // #nosec G304 -- test reads a temp file path under t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	if !containsBytes(out, "  - Templates") {
		t.Errorf("dry-run should not modify file, got:\n%s", string(out))
	}
}
