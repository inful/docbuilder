package docs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteVSCodeTaxonomySnippets(t *testing.T) {
	t.Setenv("DOCBUILDER_TEMPLATE_BASE_URL", "")

	root := t.TempDir()
	contentDir := filepath.Join(root, "content")
	if err := os.MkdirAll(contentDir, 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}

	page := `---
title: Demo
tags:
  - docs
  - Web Development
categories:
  - Guides
---

Body
`
	if err := os.WriteFile(filepath.Join(contentDir, "demo.md"), []byte(page), 0o600); err != nil {
		t.Fatalf("write demo.md: %v", err)
	}

	snippetsPath := filepath.Join(root, ".vscode", "docbuilder-taxonomies.code-snippets")
	if err := WriteVSCodeTaxonomySnippets(context.Background(), contentDir, snippetsPath); err != nil {
		t.Fatalf("WriteVSCodeTaxonomySnippets: %v", err)
	}

	// #nosec G304 -- test uses a temporary file path controlled by the test.
	data, err := os.ReadFile(snippetsPath)
	if err != nil {
		t.Fatalf("read snippets: %v", err)
	}

	var snippets map[string]map[string]any
	if err := json.Unmarshal(data, &snippets); err != nil {
		t.Fatalf("unmarshal snippets: %v", err)
	}

	catSnippet, ok := snippets["Category: Guides"]
	if !ok {
		t.Fatalf("missing category snippet")
	}
	if got, _ := catSnippet["prefix"].(string); got != "guides" {
		t.Fatalf("unexpected category prefix: %#v", catSnippet["prefix"])
	}

	tagSnippet, ok := snippets["Tag: Web Development"]
	if !ok {
		t.Fatalf("missing tag snippet")
	}
	prefixes, ok := tagSnippet["prefix"].([]any)
	if !ok || len(prefixes) != 2 {
		t.Fatalf("unexpected tag prefixes: %#v", tagSnippet["prefix"])
	}
	if prefixes[0] != "web development" || prefixes[1] != "webdevelopment" {
		t.Fatalf("unexpected tag prefixes: %#v", prefixes)
	}
}

func TestWriteVSCodeTaxonomySnippets_UsesTemplateBaseURLWhenSet(t *testing.T) {
	root := t.TempDir()
	contentDir := filepath.Join(root, "does-not-need-to-exist")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/taxonomies.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tags":["remote-tag","Remote Tag"],"categories":["Remote Category"]}`))
	}))
	defer srv.Close()

	t.Setenv("DOCBUILDER_TEMPLATE_BASE_URL", srv.URL)

	snippetsPath := filepath.Join(root, ".vscode", "docbuilder-taxonomies.code-snippets")
	if err := WriteVSCodeTaxonomySnippets(context.Background(), contentDir, snippetsPath); err != nil {
		t.Fatalf("WriteVSCodeTaxonomySnippets: %v", err)
	}

	// #nosec G304 -- test uses a temporary file path controlled by the test.
	data, err := os.ReadFile(snippetsPath)
	if err != nil {
		t.Fatalf("read snippets: %v", err)
	}

	var snippets map[string]map[string]any
	if err := json.Unmarshal(data, &snippets); err != nil {
		t.Fatalf("unmarshal snippets: %v", err)
	}

	if _, ok := snippets["Tag: remote-tag"]; !ok {
		t.Fatalf("missing remote tag snippet")
	}
	if _, ok := snippets["Category: Remote Category"]; !ok {
		t.Fatalf("missing remote category snippet")
	}
}
