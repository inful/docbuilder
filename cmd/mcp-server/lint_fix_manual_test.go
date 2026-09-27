package main

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/lint"
)

// TestComputeManualRequired_BodyH1 verifies that a doc with a body
// H1 produces a manual_required entry (since body-h1 isn't auto-fixable).
func TestComputeManualRequired_BodyH1(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "how-to"), 0o750); err != nil {
		t.Fatal(err)
	}
	h1Path := filepath.Join(dir, "docs", "how-to", "h1.md")
	if err := os.WriteFile(h1Path, []byte(`---
title: H1
uid: 22222222-aaaa-aaaa-aaaa-222222222222
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - how-to
tags:
  - test
aliases:
  - /_uid/x/
---

# Body H1
body
`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := computeManualRequired(filepath.Join(dir, "docs"), nil)
	found := false
	for _, iss := range got {
		if iss.Rule == "body-h1" {
			found = true
			if iss.Fix == "" {
				t.Errorf("body-h1 manual_required entry missing Fix text")
			}
		}
	}
	if !found {
		t.Errorf("expected body-h1 in manual_required; got %+v", got)
	}
}

// TestComputeManualRequired_AutoFixable verifies that an auto-fixable
// issue (missing tags) is NOT in manual_required.
func TestComputeManualRequired_AutoFixable(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "how-to"), 0o750); err != nil {
		t.Fatal(err)
	}
	missingTagsPath := filepath.Join(dir, "docs", "how-to", "missing-tags.md")
	if err := os.WriteFile(missingTagsPath, []byte(`---
title: MissingTags
uid: 44444444-aaaa-aaaa-aaaa-444444444444
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - how-to
aliases:
  - /_uid/x/
---

body
`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := computeManualRequired(filepath.Join(dir, "docs"), nil)
	for _, iss := range got {
		if iss.Rule == "frontmatter-required-fields" {
			t.Errorf("frontmatter-required-fields should not be in manual_required; auto-fixable")
		}
	}
}

// TestComputeManualRequired_CrossMode verifies that a doc with two
// categories (cross-mode) is in manual_required.
func TestComputeManualRequired_CrossMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "how-to"), 0o750); err != nil {
		t.Fatal(err)
	}
	twoCatPath := filepath.Join(dir, "docs", "how-to", "twocat.md")
	if err := os.WriteFile(twoCatPath, []byte(`---
title: TwoCat
uid: 33333333-aaaa-aaaa-aaaa-333333333333
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - how-to
  - reference
tags:
  - test
aliases:
  - /_uid/x/
---

body
`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := computeManualRequired(filepath.Join(dir, "docs"), nil)
	found := false
	for _, iss := range got {
		if iss.Rule == "cross-mode-category" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cross-mode-category in manual_required; got %+v", got)
	}
}

// TestAutoFixableRules_ContainsExpected sanity-checks the closed set
// of auto-fixable rule names. Adding/removing a rule here should be
// intentional — this test ensures the set doesn't drift silently.
func TestAutoFixableRules_ContainsExpected(t *testing.T) {
	expected := []string{
		"filename-conventions",
		"frontmatter-uid",
		"frontmatter-fingerprint",
		"frontmatter-required-fields",
		"directory-category-consistency",
		"category-naming",
		"internal-link-style",
		"sequence-prefix-filename",
		"missing-index-page",
		"broken-links",
	}
	for _, e := range expected {
		if !autoFixableRules[e] {
			t.Errorf("autoFixableRules missing %q", e)
		}
	}
}

// TestAutoFixableRules_ExcludesManualOnly sanity-checks that manual-only
// rules aren't accidentally added to the auto-fixable set.
func TestAutoFixableRules_ExcludesManualOnly(t *testing.T) {
	manualOnly := []string{
		"body-h1",
		"tag-count",
		"cross-mode-category",
	}
	for _, m := range manualOnly {
		if autoFixableRules[m] {
			t.Errorf("autoFixableRules should not contain manual-only rule %q", m)
		}
	}
}

// silence unused-import warnings if the test file is later trimmed.
var _ = lint.SeverityError
