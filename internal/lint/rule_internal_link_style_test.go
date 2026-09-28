package lint

import (
	"os"
	"path/filepath"
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

// TestInternalLinkStyleRule_MissingExtension verifies a relative link
// without `.md` extension produces a WARNING.
func TestInternalLinkStyleRule_MissingExtension(t *testing.T) {
	dir := t.TempDir()
	writeDocWithLinks(t, dir, "b.md", "See [CLI](cli).")

	r := &InternalLinkStyleRule{}
	issues, err := r.Check(filepath.Join(dir, "b.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Rule != ruleInternalLinkStyle {
		t.Errorf("Rule = %q, want %q", issues[0].Rule, ruleInternalLinkStyle)
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("Severity = %v, want %v", issues[0].Severity, SeverityWarning)
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
