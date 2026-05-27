package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestHandleTaxonomiesJSON(t *testing.T) {
	root := t.TempDir()
	contentDir := filepath.Join(root, "content")
	if err := os.MkdirAll(contentDir, 0o750); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}

	page := `---
title: Demo
tags:
  - docs
categories:
  - guides
---

Body
`
	if err := os.WriteFile(filepath.Join(contentDir, "demo.md"), []byte(page), 0o600); err != nil {
		t.Fatalf("write demo.md: %v", err)
	}

	srv := &Server{cfg: &config.Config{Output: config.OutputConfig{Directory: root}}}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/taxonomies.json", nil)
	w := httptest.NewRecorder()

	srv.handleTaxonomiesJSON(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got taxonomiesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "docs" {
		t.Fatalf("unexpected tags: %#v", got.Tags)
	}
	if len(got.Categories) != 1 || got.Categories[0] != "guides" {
		t.Fatalf("unexpected categories: %#v", got.Categories)
	}
}
