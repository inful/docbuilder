package hugo

import (
	"context"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// A document can ask ragabast to mirror the external pages it names.
//
// The request travels in the front matter of the document, and the
// daemon uploads the document as it is: the bytes that reach ragabast
// have to carry the field, or the request is lost.
func TestTheDocumentThatGoesToRagabastCarriesTheWebSources(t *testing.T) {
	content := `---
uid: 3f2a1b4c-5d6e-4f70-8192-a3b4c5d6e7f8
title: Semantic Versioning
categories:
  - reference
tags: []
date: 2026-01-01T00:00:00Z
lastmod: "2026-01-01"
web_sources:
  - https://semver.org/spec/v2.0.0.html
---

# Semantic Versioning
`

	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"},
	}
	gen := NewGenerator(cfg, t.TempDir())

	var uploaded string
	gen.WithDocumentReady(func(content []byte, _ string) {
		uploaded = string(content)
	})

	bs := models.NewBuildState(gen, []docs.DocFile{{
		Repository: "repo",
		Name:       "semver",
		Extension:  ".md",
		Content:    []byte(content),
	}}, models.NewBuildReport(context.Background(), 0, 0))

	if err := gen.copyContentFilesPipeline(context.Background(), bs.Docs.Files, bs); err != nil {
		t.Fatalf("copy the content: %v", err)
	}

	if !strings.Contains(uploaded, "web_sources:") {
		t.Fatalf("the uploaded document does not carry the web sources:\n%s", uploaded)
	}
	if !strings.Contains(uploaded, "https://semver.org/spec/v2.0.0.html") {
		t.Fatalf("the uploaded document does not carry the web source:\n%s", uploaded)
	}
}
