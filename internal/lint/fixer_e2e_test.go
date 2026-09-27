package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFixer_EndToEnd runs the fixer over a tree of docs that exercise
// every auto-fixable rule and verifies the resulting tree has zero
// ERROR-severity lint findings.
func TestFixer_EndToEnd(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up directory layout that triggers all the fixers.
	mustMkdirAll(t, filepath.Join(tmpDir, "docs", "how-to"))
	mustMkdirAll(t, filepath.Join(tmpDir, "docs", "adr"))

	// Doc missing title/date/lastmod/tags → frontmatter-required-fields fixer
	mustWrite(t, filepath.Join(tmpDir, "docs", "how-to", "how-to-1.md"), `---
uid: 11111111-aaaa-aaaa-aaaa-111111111111
categories:
  - how-to
---

# How To 1

body
`)

	// Doc with non-kebab-case category → category-naming fixer
	mustWrite(t, filepath.Join(tmpDir, "docs", "how-to", "how-to-2.md"), `---
title: How To 2
uid: 22222222-aaaa-aaaa-aaaa-222222222222
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

# How To 2

body
`)

	// ADR with bad sequence prefix → sequence-prefix-filename fixer
	mustWrite(t, filepath.Join(tmpDir, "docs", "adr", "adr-foo.md"), `---
title: ADR Foo
uid: 33333333-aaaa-aaaa-aaaa-333333333333
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - architecture-decisions
tags:
  - test
aliases:
  - /_uid/x/
---

# ADR Foo

body
`)

	linter := NewLinter(&Config{Format: formatText})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	result, err := fixer.Fix(filepath.Join(tmpDir, "docs"))
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}

	// Re-lint and confirm no auto-fixable ERRORs remain.
	// (cross-mode-category is intentionally not auto-fixable — user
	// must resolve category conflicts manually.)
	lintResult, err := linter.LintPath(filepath.Join(tmpDir, "docs"))
	if err != nil {
		t.Fatalf("re-lint: %v", err)
	}
	for _, iss := range lintResult.Issues {
		if iss.Severity != SeverityError {
			continue
		}
		// Skip rules that the fixer doesn't auto-fix.
		if iss.Rule == "cross-mode-category" {
			continue
		}
		t.Errorf("post-fix ERROR remaining: rule=%s severity=%v msg=%q file=%s",
			iss.Rule, iss.Severity, iss.Message, iss.FilePath)
	}

	// Sanity check: at least one of each fixer phase fired.
	if len(result.FrontmatterFields) == 0 {
		t.Errorf("expected FrontmatterFields operations")
	}
	if len(result.CategoryRenames) == 0 {
		t.Errorf("expected CategoryRenames operations")
	}
	if len(result.FilesRenamed) == 0 {
		t.Errorf("expected FilesRenamed operations (sequence-prefix)")
	}

	// Verify the renamed ADR follows the required pattern.
	entries, _ := os.ReadDir(filepath.Join(tmpDir, "docs", "adr"))
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "adr-") {
			continue
		}
		if e.Name() == "_index.md" {
			continue
		}
		if !strings.HasPrefix(e.Name(), "adr-001-") && !strings.HasPrefix(e.Name(), "adr-002-") {
			t.Errorf("ADR not in expected pattern: %s", e.Name())
		}
	}

	// Verify all categories are kebab-case after fix.
	for _, path := range []string{
		filepath.Join(tmpDir, "docs", "how-to", "how-to-1.md"),
		filepath.Join(tmpDir, "docs", "how-to", "how-to-2.md"),
	} {
		// Use the rule to check
		catRule := &CategoryNamingRule{}
		issues, err := catRule.Check(path)
		if err != nil {
			t.Fatalf("check %s: %v", path, err)
		}
		for _, iss := range issues {
			t.Errorf("%s: category-naming still flags %q", path, iss.Message)
		}
	}

	// Verify the renamed ADR follows the required pattern (use glob
	// since we don't know the exact sequence number).
	renamedADR := findADRFile(t, filepath.Join(tmpDir, "docs", "adr"))
	if renamedADR == "" {
		t.Errorf("expected renamed ADR file in docs/adr/")
	}
	catRule := &CategoryNamingRule{}
	issues, err := catRule.Check(renamedADR)
	if err != nil {
		t.Fatalf("check %s: %v", renamedADR, err)
	}
	for _, iss := range issues {
		t.Errorf("%s: category-naming still flags %q", renamedADR, iss.Message)
	}
}

// findADRFile returns the first .md file in dir that doesn't match
// the required sequence-prefix pattern (used to verify the rename
// fixer produced a valid filename).
func findADRFile(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".md" {
			continue
		}
		return filepath.Join(dir, name)
	}
	return ""
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
