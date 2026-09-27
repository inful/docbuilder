package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempFile writes content to a temp .md file and returns its path.
// The caller does not need to clean up; t.TempDir() handles teardown.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestBodyH1Rule_Name(t *testing.T) {
	r := &BodyH1Rule{}
	if got := r.Name(); got != ruleBodyH1 {
		t.Errorf("Name() = %q, want %q", got, ruleBodyH1)
	}
}

func TestBodyH1Rule_AppliesTo(t *testing.T) {
	r := &BodyH1Rule{}
	cases := []struct {
		path string
		want bool
	}{
		{"docs/foo.md", true},
		{"docs/foo.markdown", true},
		{"docs/foo.txt", false},
		{"README", false},
	}
	for _, tc := range cases {
		if got := r.AppliesTo(tc.path); got != tc.want {
			t.Errorf("AppliesTo(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestBodyH1Rule_Check_H1Present verifies an H1 at body start is flagged.
func TestBodyH1Rule_Check_H1Present(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "h1.md", `---
title: Hello
uid: 11111111-1111-1111-1111-111111111111
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/hello/
---

# Hello

Body text.
`)

	r := &BodyH1Rule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Rule != ruleBodyH1 {
		t.Errorf("Rule = %q, want %q", issues[0].Rule, ruleBodyH1)
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("Severity = %v, want %v", issues[0].Severity, SeverityWarning)
	}
}

// TestBodyH1Rule_Check_H2First verifies a body that starts with H2 passes.
func TestBodyH1Rule_Check_H2First(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "h2.md", `---
title: Hello
uid: 22222222-2222-2222-2222-222222222222
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/hello/
---

## A section

Body text.
`)

	r := &BodyH1Rule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d: %+v", len(issues), issues)
	}
}

// TestBodyH1Rule_Check_BlankLinesBeforeBody verifies leading blank lines
// are tolerated — the H1 is still flagged if it follows the blanks.
func TestBodyH1Rule_Check_BlankLinesBeforeBody(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "blanks.md", `---
title: Hello
uid: 33333333-3333-3333-3333-333333333333
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/hello/
---



# Hello

Body text.
`)

	r := &BodyH1Rule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (blank lines should be skipped), got %d", len(issues))
	}
}

// TestBodyH1Rule_Check_NoFrontmatter verifies a body that opens with
// an H1 but has no frontmatter is still flagged.
func TestBodyH1Rule_Check_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "nofm.md", "# Hello\n\nBody.\n")

	r := &BodyH1Rule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
}

// TestBodyH1Rule_Check_EmptyBody verifies an empty body returns no issues.
func TestBodyH1Rule_Check_EmptyBody(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "empty.md", `---
title: Empty
uid: 44444444-4444-4444-4444-444444444444
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/empty/
---
`)

	r := &BodyH1Rule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for empty body, got %d", len(issues))
	}
}

// TestBodyH1Rule_Check_ListFirst verifies a body that opens with a
// list/prose/code block (no H1) passes.
func TestBodyH1Rule_Check_ListFirst(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "list.md", `---
title: List
uid: 55555555-5555-5555-5555-555555555555
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/list/
---

- a list item
- another item
`)

	r := &BodyH1Rule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}
