package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The project recommends the repository-root-relative links for the
// links across sections: the how-to guide lists them as the best
// practice and the broken-link detector understands them. The rule
// that flagged every one of them is gone.
func TestTheRootRelativeLinkIsNotReported(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "page.md")
	if err := os.WriteFile(doc, []byte(`---
title: Page
uid: 66666666-aaaa-aaaa-aaaa-666666666666
aliases:
  - /_uid/66666666-aaaa-aaaa-aaaa-666666666666/
categories:
  - reference
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
tags:
  - test
---

# Page

[Other](/reference/other.md)
`), 0o600); err != nil {
		t.Fatalf("write the document: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other\n"), 0o600); err != nil {
		t.Fatalf("write the other document: %v", err)
	}

	l := NewLinter(&Config{Format: formatText})
	res, err := l.LintPath(dir)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}

	for _, issue := range res.Issues {
		if strings.Contains(strings.ToLower(issue.Message), "site-rooted") {
			t.Errorf("the recommended link style was reported: %s", issue.Message)
		}
	}
}
