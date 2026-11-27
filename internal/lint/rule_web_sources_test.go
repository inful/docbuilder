package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// writeDocWithWebSources writes a document whose front matter holds the
// given web_sources block.
func writeDocWithWebSources(t *testing.T, dir, name, block string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	body := `---
title: Web Sources
uid: 77777777-7777-7777-7777-777777777777
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
fingerprint: 0000000000000000000000000000000000000000000000000000000000000000
categories:
  - reference
tags:
  - test
aliases:
  - /_uid/websources/
` + block + `---

body
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWebSourcesRule_Name(t *testing.T) {
	r := &WebSourcesRule{}
	if got := r.Name(); got != ruleWebSources {
		t.Errorf("Name() = %q, want %q", got, ruleWebSources)
	}
}

// A list of http(s) URLs is what ragabast reads.
func TestWebSourcesRule_AcceptsAListOfWebURLs(t *testing.T) {
	dir := t.TempDir()
	path := writeDocWithWebSources(t, dir, "good.md", `web_sources:
  - https://semver.org/spec/v2.0.0.html
  - http://example.com/page
`)

	r := &WebSourcesRule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %+v", len(issues), issues)
	}
}

// A document without the field has nothing to declare.
func TestWebSourcesRule_AcceptsADocumentWithoutTheField(t *testing.T) {
	dir := t.TempDir()
	path := writeDocWithWebSources(t, dir, "none.md", "")

	r := &WebSourcesRule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %+v", len(issues), issues)
	}
}

// A lone URL is not a list, and ragabast does not read it.
func TestWebSourcesRule_ReportsALoneURL(t *testing.T) {
	dir := t.TempDir()
	path := writeDocWithWebSources(t, dir, "lone.md", "web_sources: https://semver.org/spec/v2.0.0.html\n")

	r := &WebSourcesRule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].Message != webSourcesNotAListMessage {
		t.Errorf("the issue says %q, want %q", issues[0].Message, webSourcesNotAListMessage)
	}
	if issues[0].Severity != SeverityError {
		t.Errorf("the issue is %s, want ERROR", issues[0].Severity)
	}
}

// Only http and https are pages of the web.
func TestWebSourcesRule_ReportsTheEntriesThatAreNotWebURLs(t *testing.T) {
	cases := map[string]string{
		"no scheme": "semver.org/spec/v2.0.0.html",
		"another":   "ftp://example.com/file",
		"a script":  "javascript:alert(1)",
		"empty":     `""`,
		"no host":   "https://",
		"a path":    "/spec/v2.0.0.html",
	}

	dir := t.TempDir()
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeDocWithWebSources(t, dir, "bad.md", "web_sources:\n  - "+value+"\n")

			r := &WebSourcesRule{}
			issues, err := r.Check(path)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if len(issues) != 1 {
				t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
			}
			if issues[0].Message != webSourcesNotAURLMessage {
				t.Errorf("the issue says %q, want %q", issues[0].Message, webSourcesNotAURLMessage)
			}
		})
	}
}

// An entry that is not a string is not an address.
func TestWebSourcesRule_ReportsAnEntryThatIsNotAString(t *testing.T) {
	dir := t.TempDir()
	path := writeDocWithWebSources(t, dir, "number.md", "web_sources:\n  - 42\n")

	r := &WebSourcesRule{}
	issues, err := r.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].Message != webSourcesNotAURLMessage {
		t.Errorf("the issue says %q, want %q", issues[0].Message, webSourcesNotAURLMessage)
	}
}
