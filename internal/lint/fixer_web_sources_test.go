package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A lone URL becomes a list of one: ragabast reads a list.
func TestFixer_WebSourcesTurnsALoneURLIntoAList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := `---
title: Demo
uid: 66666666-6666-6666-6666-666666666666
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - reference
tags:
  - test
aliases:
  - /_uid/x/
web_sources: https://semver.org/spec/v2.0.0.html
---

body
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	if _, err := fixer.Fix(path); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	out, err := os.ReadFile(path) // #nosec G304 -- test reads a temp file path under t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "web_sources:\n  - https://semver.org/spec/v2.0.0.html") {
		t.Errorf("the web source is not a list of one, got:\n%s", got)
	}

	// The lint passes now: the field is a list of a URL of the web.
	issues, err := linter.LintPath(path)
	if err != nil {
		t.Fatalf("LintPath: %v", err)
	}
	for _, issue := range issues.Issues {
		if issue.Rule == ruleWebSources {
			t.Errorf("the web sources are still reported: %+v", issue)
		}
	}
}

// The entries that are not URLs of the web are the author's decision:
// the fixer does not guess a scheme and does not drop a page.
func TestFixer_WebSourcesLeavesTheEntriesThatAreNotWebURLs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := `---
title: Demo
uid: 55555555-5555-5555-5555-555555555555
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - reference
tags:
  - test
aliases:
  - /_uid/x/
web_sources:
  - semver.org/spec/v2.0.0.html
---

body
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	if _, err := fixer.Fix(path); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	out, err := os.ReadFile(path) // #nosec G304 -- test reads a temp file path under t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "- semver.org/spec/v2.0.0.html") {
		t.Errorf("the entry was changed, got:\n%s", string(out))
	}

	// The lint still fails: the entry is not a URL of the web.
	issues, err := linter.LintPath(path)
	if err != nil {
		t.Fatalf("LintPath: %v", err)
	}
	found := false
	for _, issue := range issues.Issues {
		if issue.Rule == ruleWebSources {
			found = true
		}
	}
	if !found {
		t.Error("the entry that is not a URL of the web is not reported any more")
	}
}

// The dry run writes nothing and says what would happen.
func TestFixer_WebSourcesDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.md")
	body := `---
title: Demo
uid: 44444444-4444-4444-4444-444444444444
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - reference
tags:
  - test
aliases:
  - /_uid/x/
web_sources: https://semver.org/spec/v2.0.0.html
---

body
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	linter := NewLinter(&Config{})
	fixer := NewFixer(linter, true, false).WithAutoConfirm(true)
	if _, err := fixer.Fix(path); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	out, err := os.ReadFile(path) // #nosec G304 -- test reads a temp file path under t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "web_sources: https://semver.org/spec/v2.0.0.html") {
		t.Errorf("the dry run wrote the file, got:\n%s", string(out))
	}
}
