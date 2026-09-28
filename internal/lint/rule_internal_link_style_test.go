package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDocWithLinks(t *testing.T, dir, name, bodyLinks string) {
	t.Helper()
	body := `---
title: LinksTest
uid: 77777777-7777-7777-7777-777777777777
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - explanation
tags:
  - test
aliases:
  - /_uid/linkstest/
---

` + bodyLinks + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestInternalLinkStyleRule_Name(t *testing.T) {
	r := &InternalLinkStyleRule{}
	if got := r.Name(); got != ruleInternalLinkStyle {
		t.Errorf("Name() = %q, want %q", got, ruleInternalLinkStyle)
	}
}

// TestInternalLinkStyleRule_ValidCanonical verifies a `.md` link passes.
func TestInternalLinkStyleRule_ValidCanonical(t *testing.T) {
	dir := t.TempDir()
	writeDocWithLinks(t, dir, "a.md", "See [CLI](cli.md).")

	r := &InternalLinkStyleRule{}
	issues, err := r.Check(filepath.Join(dir, "a.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

// TestInternalLinkStyleRule_NonMarkdownExtension verifies that links with an
// explicit non-markdown extension (for example images) are not flagged.
func TestInternalLinkStyleRule_NonMarkdownExtension(t *testing.T) {
	dir := t.TempDir()
	writeDocWithLinks(t, dir, "asset.md", "See [Screenshot](img/ubuntu_ssh01.png).")

	r := &InternalLinkStyleRule{}
	issues, err := r.Check(filepath.Join(dir, "asset.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (explicit extension), got %d", len(issues))
	}
}

// TestInternalLinkStyleRule_SiteRooted verifies a site-rooted link
// (starting with `/`) produces a WARNING.
func TestInternalLinkStyleRule_SiteRooted(t *testing.T) {
	dir := t.TempDir()
	writeDocWithLinks(t, dir, "c.md", "See [Other](/other-page).")

	r := &InternalLinkStyleRule{}
	issues, err := r.Check(filepath.Join(dir, "c.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}

// TestInternalLinkStyleRule_External verifies an https URL passes.
func TestInternalLinkStyleRule_External(t *testing.T) {
	dir := t.TempDir()
	writeDocWithLinks(t, dir, "d.md", "See [Hugo](https://gohugo.io).")

	r := &InternalLinkStyleRule{}
	issues, err := r.Check(filepath.Join(dir, "d.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (external URL), got %d", len(issues))
	}
}

// TestInternalLinkStyleRule_Anchor verifies an in-page anchor passes.
func TestInternalLinkStyleRule_Anchor(t *testing.T) {
	dir := t.TempDir()
	writeDocWithLinks(t, dir, "e.md", "See [Next](#next).")

	r := &InternalLinkStyleRule{}
	issues, err := r.Check(filepath.Join(dir, "e.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (anchor), got %d", len(issues))
	}
}

// TestInternalLinkStyleRule_NoWarningForDirectoryLink is the
// characterization test for removing the missing-`.md` half of the
// rule. Hugo resolves `[text](./api/)` to `api/_index.md`; the rule
// used to flag this with "Internal link missing .md extension", which
// contradicted the README's documented `./name/` pattern (README.md:402)
// and produced false positives that operators had to silence.
//
// Desired end state: relative directory links (with or without trailing
// slash) produce no issue from this rule. Pre-change, the rule fires
// here because `./api/` has no `.md` extension.
func TestInternalLinkStyleRule_NoWarningForDirectoryLink(t *testing.T) {
	dir := t.TempDir()
	// Directory link with trailing slash — Hugo resolves to dir/_index.md.
	writeDocWithLinks(t, dir, "index.md", "See [API](./api/) for details.")
	// Directory link without trailing slash — same target, slightly
	// different syntax. Both should be ignored by the rule.
	writeDocWithLinks(t, dir, "index2.md", "See [API](./api) for details.")

	r := &InternalLinkStyleRule{}
	for _, name := range []string{"index.md", "index2.md"} {
		issues, err := r.Check(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("Check(%s): %v", name, err)
		}
		for _, iss := range issues {
			if iss.Rule == ruleInternalLinkStyle {
				t.Errorf("%s: rule %q should not fire on directory link; got: %+v",
					name, iss.Rule, iss)
			}
		}
	}
}

// TestInternalLinkStyleRule_NoWarningForExtensionlessLink is the
// characterization test for the second case the user flagged:
// relative links without `.md` extension. The broken-link detector
// already does the right existence check (it tries the path as-is,
// then with `.md` appended, then with `.markdown`), so this rule's
// missing-extension warning was redundant and produced false positives
// on links that resolved correctly.
//
// Desired end state: relative extensionless links produce no issue
// from this rule. Pre-change, the rule fires here because `./foo` has
// no `.md` extension.
func TestInternalLinkStyleRule_NoWarningForExtensionlessLink(t *testing.T) {
	dir := t.TempDir()
	writeDocWithLinks(t, dir, "index.md", "See [Foo](./foo) for details.")

	r := &InternalLinkStyleRule{}
	issues, err := r.Check(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, iss := range issues {
		if iss.Rule == ruleInternalLinkStyle {
			t.Errorf("rule %q should not fire on extensionless link; got: %+v", iss.Rule, iss)
		}
	}
}

// TestInternalLinkStyleRule_StillWarnsForSiteRooted guards against the
// site-rooted half of the rule being accidentally deleted along with
// the missing-`.md` half. The site-rooted warning is still useful
// (cross-renderer portability) and is the only remaining concern of
// the rule after the cleanup.
func TestInternalLinkStyleRule_StillWarnsForSiteRooted(t *testing.T) {
	dir := t.TempDir()
	writeDocWithLinks(t, dir, "index.md", "See [Other](/other-page) for details.")

	r := &InternalLinkStyleRule{}
	issues, err := r.Check(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.Rule == ruleInternalLinkStyle && strings.Contains(iss.Message, "Site-rooted") {
			found = true
		}
	}
	if !found {
		t.Errorf("rule %q should still fire site-rooted warning; got issues: %+v",
			ruleInternalLinkStyle, issues)
	}
}
