package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// A document that holds two doc modes is an error - unless the pair is
// allowed. The pairs come from the configuration: the rule had a field
// for them that nobody could set.
func TestTheAllowedCategoryPairsSuppressTheError(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "page.md")
	if err := os.WriteFile(doc, []byte(`---
title: Page
uid: 77777777-aaaa-aaaa-aaaa-777777777777
aliases:
  - /_uid/77777777-aaaa-aaaa-aaaa-777777777777/
categories:
  - reference
  - tutorials
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
tags:
  - test
---

# Page
`), 0o600); err != nil {
		t.Fatalf("write the document: %v", err)
	}

	// Without the pair the document is an error.
	strict := NewLinter(&Config{Format: formatText})
	res, err := strict.LintPath(dir)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	found := false
	for _, issue := range res.Issues {
		if issue.Rule == ruleCrossModeCategory {
			found = true
		}
	}
	if !found {
		t.Fatal("the document with two doc modes was not reported")
	}

	// With the pair the document is fine.
	lenient := NewLinter(&Config{
		Format: formatText,
		AllowedCategoryPairs: map[string]map[string]bool{
			"reference": {"tutorials": true},
		},
	})
	res, err = lenient.LintPath(dir)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	for _, issue := range res.Issues {
		if issue.Rule == ruleCrossModeCategory {
			t.Errorf("the allowed pair was reported: %s", issue.Message)
		}
	}
}
