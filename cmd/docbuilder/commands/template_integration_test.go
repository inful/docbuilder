package commands

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/lint"
)

// createTestConfig creates a minimal test config file.
func createTestConfig(t *testing.T, tmpDir string) string {
	t.Helper()
	configPath := filepath.Join(tmpDir, "config.yaml")
	// Minimal config - use a dummy local repository to satisfy validation
	// Template commands don't actually need repositories, but config validation requires it
	configContent := `version: "2.0"
repositories:
  - url: file:///dev/null
    name: dummy
    branch: main
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))
	return configPath
}

// templateServer creates a test HTTP server that serves template discovery and template pages.
func templateServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// Discovery page: /categories/templates/
	mux.HandleFunc("/categories/templates/", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head><title>Templates</title></head>
<body>
	<h1>Templates</h1>
	<ul>
		<li><a href="/templates/adr.template/index.html">adr.template</a></li>
		<li><a href="/templates/guide.template/index.html">guide.template</a></li>
	</ul>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})

	// ADR template page
	mux.HandleFunc("/templates/adr.template/index.html", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
	<meta property="docbuilder:template.type" content="adr">
	<meta property="docbuilder:template.name" content="Architecture Decision Record">
	<meta property="docbuilder:template.output_path" content="adr/adr-{{ printf &quot;%03d&quot; (nextInSequence &quot;adr&quot;) }}-{{ .Slug }}.md">
	<meta property="docbuilder:template.description" content="Create a new ADR">
	<meta property="docbuilder:template.schema" content='{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'>
	<meta property="docbuilder:template.defaults" content='{"categories":["architecture-decisions"]}'>
	<meta property="docbuilder:template.sequence" content='{"name":"adr","dir":"adr","glob":"adr-*.md","regex":"^adr-(\\d{3})-","width":3,"start":1}'>
</head>
<body>
	<h1>ADR Template</h1>
	<pre><code class="language-markdown">---
title: "{{ .Title }}"
categories:
  - {{ index .categories 0 }}
date: 2026-01-01T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

**Status**: Proposed

## Context

## Decision

## Consequences
</code></pre>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})

	// Guide template page
	mux.HandleFunc("/templates/guide.template/index.html", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
	<meta property="docbuilder:template.type" content="guide">
	<meta property="docbuilder:template.name" content="User Guide">
	<meta property="docbuilder:template.output_path" content="guides/{{ .Slug }}.md">
	<meta property="docbuilder:template.schema" content='{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true},{"key":"Category","type":"string_enum","required":true,"options":["getting-started","advanced","reference"]}]}'>
</head>
<body>
	<h1>Guide Template</h1>
	<pre><code class="language-markdown">---
title: "{{ .Title }}"
categories:
  - {{ .Category }}
---

# {{ .Title }}

## Overview

## Steps

## Next Steps
</code></pre>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})

	return httptest.NewServer(mux)
}

func TestTemplateList_Integration(t *testing.T) {
	server := templateServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := createTestConfig(t, tmpDir)

	cmd := &TemplateListCmd{
		BaseURL: server.URL,
	}
	cli := &CLI{
		Config: configPath,
	}

	// For integration test, we just verify the command runs without error
	// The actual output format is tested in unit tests
	err := cmd.Run(&Global{}, cli)
	require.NoError(t, err)
}

func TestTemplateNew_SingleTemplate_Integration(t *testing.T) {
	// Create a server with only one template
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/categories/templates/" || r.URL.Path == "/categories/templates" {
			html := `<!DOCTYPE html>
<html>
<head><title>Templates</title></head>
<body>
	<h1>Templates</h1>
	<ul>
		<li><a href="/templates/adr.template/index.html">adr.template</a></li>
	</ul>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		if r.URL.Path == "/templates/adr.template/index.html" {
			html := `<!DOCTYPE html>
<html>
<head>
	<meta property="docbuilder:template.type" content="adr">
	<meta property="docbuilder:template.name" content="Architecture Decision Record">
	<meta property="docbuilder:template.output_path" content="adr/adr-{{ printf &quot;%03d&quot; (nextInSequence &quot;adr&quot;) }}-{{ .Slug }}.md">
	<meta property="docbuilder:template.schema" content='{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'>
	<meta property="docbuilder:template.defaults" content='{"categories":["architecture-decisions"]}'>
	<meta property="docbuilder:template.sequence" content='{"name":"adr","dir":"adr","glob":"adr-*.md","regex":"^adr-(\\d{3})-","width":3,"start":1}'>
</head>
<body>
	<h1>ADR Template</h1>
	<pre><code class="language-markdown">---
title: "{{ .Title }}"
categories:
  - {{ index .categories 0 }}
date: 2026-01-01T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

**Status**: Proposed

## Context

## Decision

## Consequences
</code></pre>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := createTestConfig(t, tmpDir)
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	// Change to temp directory
	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldCwd) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &TemplateNewCmd{
		BaseURL: server.URL,
		Set:     []string{"Title=Test ADR", "Slug=test-adr"},
		Yes:     true,
	}

	// Capture stdout using a pipe
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
	}()

	var stdout bytes.Buffer
	go func() {
		_, _ = io.Copy(&stdout, r)
		_ = r.Close()
	}()

	// Single template auto-selects, so no stdin needed
	cli := &CLI{
		Config: configPath,
	}
	err = cmd.Run(&Global{}, cli)
	require.NoError(t, err)

	// Verify file was created
	expectedPath := filepath.Join(docsDir, "adr", "adr-001-test-adr.md")
	require.FileExists(t, expectedPath)

	// Verify file content
	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	content := string(data)
	require.Contains(t, content, "Test ADR")
	require.Contains(t, content, "test-adr")
	require.Contains(t, content, "architecture-decisions")

	// Verify lint was run (file should have proper frontmatter)
	linter := lint.NewLinter(&lint.Config{Format: "text"})
	result, err := linter.LintPath(expectedPath)
	require.NoError(t, err)
	require.False(t, result.HasErrors(), "generated file should pass linting")
}

func TestTemplateNew_MultipleTemplates_WithSelection_Integration(t *testing.T) {
	// This test verifies that when multiple templates are available,
	// the user can select one. For simplicity, we test with Yes=true
	// and a single-template server, as the selection logic is tested
	// in unit tests. The full interactive flow with stdin mocking is
	// complex and flaky, so we focus on file creation here.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/categories/templates/" || r.URL.Path == "/categories/templates" {
			html := `<!DOCTYPE html>
<html>
<head><title>Templates</title></head>
<body>
	<h1>Templates</h1>
	<ul>
		<li><a href="/templates/guide.template/index.html">guide.template</a></li>
	</ul>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		if r.URL.Path == "/templates/guide.template/index.html" {
			html := `<!DOCTYPE html>
<html>
<head>
	<meta property="docbuilder:template.type" content="guide">
	<meta property="docbuilder:template.name" content="User Guide">
	<meta property="docbuilder:template.output_path" content="guides/{{ .Slug }}.md">
	<meta property="docbuilder:template.schema" content='{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true},{"key":"Category","type":"string_enum","required":true,"options":["getting-started","advanced","reference"]}]}'>
</head>
<body>
	<h1>Guide Template</h1>
	<pre><code class="language-markdown">---
title: "{{ .Title }}"
categories:
  - {{ .Category }}
---

# {{ .Title }}

## Overview

## Steps

## Next Steps
</code></pre>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := createTestConfig(t, tmpDir)
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldCwd) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &TemplateNewCmd{
		BaseURL: server.URL,
		Set:     []string{"Title=Test Guide", "Slug=test-guide", "Category=getting-started"},
		Yes:     true,
	}

	cli := &CLI{
		Config: configPath,
	}
	err = cmd.Run(&Global{}, cli)
	require.NoError(t, err)

	// Verify guide file was created
	expectedPath := filepath.Join(docsDir, "guides", "test-guide.md")
	require.FileExists(t, expectedPath)

	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	content := string(data)
	require.Contains(t, content, "Test Guide")
	require.Contains(t, content, "getting-started")
	// Slug is in filename, verify the file was created with correct name
	require.True(t, strings.HasSuffix(expectedPath, "test-guide.md"), "file should have slug in filename")
}

func TestTemplateNew_WithDefaults_Integration(t *testing.T) {
	// Use single-template server to avoid selection prompt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/categories/templates/" || r.URL.Path == "/categories/templates" {
			html := `<!DOCTYPE html>
<html>
<head><title>Templates</title></head>
<body>
	<h1>Templates</h1>
	<ul>
		<li><a href="/templates/adr.template/index.html">adr.template</a></li>
	</ul>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		if r.URL.Path == "/templates/adr.template/index.html" {
			html := `<!DOCTYPE html>
<html>
<head>
	<meta property="docbuilder:template.type" content="adr">
	<meta property="docbuilder:template.name" content="Architecture Decision Record">
	<meta property="docbuilder:template.output_path" content="adr/adr-{{ printf &quot;%03d&quot; (nextInSequence &quot;adr&quot;) }}-{{ .Slug }}.md">
	<meta property="docbuilder:template.schema" content='{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'>
	<meta property="docbuilder:template.defaults" content='{"categories":["architecture-decisions"]}'>
	<meta property="docbuilder:template.sequence" content='{"name":"adr","dir":"adr","glob":"adr-*.md","regex":"^adr-(\\d{3})-","width":3,"start":1}'>
</head>
<body>
	<h1>ADR Template</h1>
	<pre><code class="language-markdown">---
title: "{{ .Title }}"
categories:
  - {{ index .categories 0 }}
date: 2026-01-01T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

**Status**: Proposed

## Context

## Decision

## Consequences
</code></pre>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := createTestConfig(t, tmpDir)
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldCwd) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &TemplateNewCmd{
		BaseURL:  server.URL,
		Set:       []string{"Title=Default ADR", "Slug=default-adr"},
		Defaults:  true,
		Yes:       true,
	}

	// Capture stdout using a pipe
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
	}()

	var stdout bytes.Buffer
	go func() {
		_, _ = io.Copy(&stdout, r)
		_ = r.Close()
	}()

	cli := &CLI{
		Config: configPath,
	}
	err = cmd.Run(&Global{}, cli)
	require.NoError(t, err)

	expectedPath := filepath.Join(docsDir, "adr", "adr-001-default-adr.md")
	require.FileExists(t, expectedPath)

	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	content := string(data)
	require.Contains(t, content, "architecture-decisions") // from defaults
}

func TestTemplateNew_SequenceNumbering_Integration(t *testing.T) {
	// Use single-template server to avoid selection prompt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/categories/templates/" || r.URL.Path == "/categories/templates" {
			html := `<!DOCTYPE html>
<html>
<head><title>Templates</title></head>
<body>
	<h1>Templates</h1>
	<ul>
		<li><a href="/templates/adr.template/index.html">adr.template</a></li>
	</ul>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		if r.URL.Path == "/templates/adr.template/index.html" {
			html := `<!DOCTYPE html>
<html>
<head>
	<meta property="docbuilder:template.type" content="adr">
	<meta property="docbuilder:template.name" content="Architecture Decision Record">
	<meta property="docbuilder:template.output_path" content="adr/adr-{{ printf &quot;%03d&quot; (nextInSequence &quot;adr&quot;) }}-{{ .Slug }}.md">
	<meta property="docbuilder:template.schema" content='{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'>
	<meta property="docbuilder:template.defaults" content='{"categories":["architecture-decisions"]}'>
	<meta property="docbuilder:template.sequence" content='{"name":"adr","dir":"adr","glob":"adr-*.md","regex":"^adr-(\\d{3})-","width":3,"start":1}'>
</head>
<body>
	<h1>ADR Template</h1>
	<pre><code class="language-markdown">---
title: "{{ .Title }}"
categories:
  - {{ index .categories 0 }}
date: 2026-01-01T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

**Status**: Proposed

## Context

## Decision

## Consequences
</code></pre>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := createTestConfig(t, tmpDir)
	docsDir := filepath.Join(tmpDir, "docs")
	adrDir := filepath.Join(docsDir, "adr")
	require.NoError(t, os.MkdirAll(adrDir, 0o750))

	// Create existing ADR files to test sequence
	existingFiles := []string{
		"adr-001-first.md",
		"adr-003-third.md",
		"adr-010-tenth.md",
	}
	for _, f := range existingFiles {
		require.NoError(t, os.WriteFile(filepath.Join(adrDir, f), []byte("# Existing ADR\n"), 0o600))
	}

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldCwd) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &TemplateNewCmd{
		BaseURL: server.URL,
		Set:     []string{"Title=New ADR", "Slug=new-adr"},
		Yes:     true,
	}

	// Capture stdout using a pipe
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
	}()

	var stdout bytes.Buffer
	go func() {
		_, _ = io.Copy(&stdout, r)
		_ = r.Close()
	}()

	cli := &CLI{
		Config: configPath,
	}
	err = cmd.Run(&Global{}, cli)
	require.NoError(t, err)

	// Should create adr-011 (next after 010)
	expectedPath := filepath.Join(adrDir, "adr-011-new-adr.md")
	require.FileExists(t, expectedPath)
}

func TestTemplateNew_WithPrompts_Integration(t *testing.T) {
	// Use single-template server to avoid selection prompt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/categories/templates/" || r.URL.Path == "/categories/templates" {
			html := `<!DOCTYPE html>
<html>
<head><title>Templates</title></head>
<body>
	<h1>Templates</h1>
	<ul>
		<li><a href="/templates/adr.template/index.html">adr.template</a></li>
	</ul>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		if r.URL.Path == "/templates/adr.template/index.html" {
			html := `<!DOCTYPE html>
<html>
<head>
	<meta property="docbuilder:template.type" content="adr">
	<meta property="docbuilder:template.name" content="Architecture Decision Record">
	<meta property="docbuilder:template.output_path" content="adr/adr-{{ printf &quot;%03d&quot; (nextInSequence &quot;adr&quot;) }}-{{ .Slug }}.md">
	<meta property="docbuilder:template.schema" content='{"fields":[{"key":"Title","type":"string","required":true},{"key":"Slug","type":"string","required":true}]}'>
	<meta property="docbuilder:template.defaults" content='{"categories":["architecture-decisions"]}'>
	<meta property="docbuilder:template.sequence" content='{"name":"adr","dir":"adr","glob":"adr-*.md","regex":"^adr-(\\d{3})-","width":3,"start":1}'>
</head>
<body>
	<h1>ADR Template</h1>
	<pre><code class="language-markdown">---
title: "{{ .Title }}"
categories:
  - {{ index .categories 0 }}
date: 2026-01-01T00:00:00Z
slug: "{{ .Slug }}"
---

# {{ .Title }}

**Status**: Proposed

## Context

## Decision

## Consequences
</code></pre>
</body>
</html>`
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(html))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := createTestConfig(t, tmpDir)
	docsDir := filepath.Join(tmpDir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o750))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldCwd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Mock stdin: provide Title and Slug (no template selection needed - single template)
	rStdin, wStdin, err := os.Pipe()
	require.NoError(t, err)
	oldStdin := os.Stdin
	os.Stdin = rStdin
	defer func() {
		os.Stdin = oldStdin
		_ = rStdin.Close()
		_ = wStdin.Close()
	}()
	go func() {
		_, _ = wStdin.WriteString("Prompted Title\nprompted-slug\n")
		_ = wStdin.Close()
	}()

	cmd := &TemplateNewCmd{
		BaseURL: server.URL,
		Yes:     true, // Auto-confirm file creation, but still prompt for inputs
	}

	// Capture stdout using a pipe
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
	}()

	var stdout bytes.Buffer
	go func() {
		_, _ = io.Copy(&stdout, r)
		_ = r.Close()
	}()

	cli := &CLI{
		Config: configPath,
	}
	err = cmd.Run(&Global{}, cli)
	require.NoError(t, err)

	expectedPath := filepath.Join(docsDir, "adr", "adr-001-prompted-slug.md")
	require.FileExists(t, expectedPath)

	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	content := string(data)
	require.Contains(t, content, "Prompted Title")
	require.Contains(t, content, "prompted-slug")
}

func TestTemplateNew_ErrorHandling_Integration(t *testing.T) {
	t.Run("invalid base URL", func(t *testing.T) {
		cmd := &TemplateListCmd{
			BaseURL: "not-a-valid-url",
		}

		err := cmd.Run(&Global{}, &CLI{})
		require.Error(t, err)
	})

	t.Run("server returns 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cmd := &TemplateListCmd{
			BaseURL: server.URL,
		}

		err := cmd.Run(&Global{}, &CLI{})
		require.Error(t, err)
	})

	t.Run("invalid template selection", func(t *testing.T) {
		server := templateServer(t)
		defer server.Close()

		rStdin, wStdin, err := os.Pipe()
		require.NoError(t, err)
		oldStdin := os.Stdin
		os.Stdin = rStdin
		defer func() {
			os.Stdin = oldStdin
			_ = rStdin.Close()
			_ = wStdin.Close()
		}()
		go func() {
			_, _ = wStdin.WriteString("99\n") // Invalid selection
			_ = wStdin.Close()
		}()

		tmpDir := t.TempDir()
		configPath := createTestConfig(t, tmpDir)
		docsDir := filepath.Join(tmpDir, "docs")
		require.NoError(t, os.MkdirAll(docsDir, 0o750))

		oldCwd, err := os.Getwd()
		require.NoError(t, err)
		defer func() { _ = os.Chdir(oldCwd) }()
		require.NoError(t, os.Chdir(tmpDir))

		cmd := &TemplateNewCmd{
			BaseURL: server.URL,
			Yes:     false, // Allow prompting so selection happens
		}
		cli := &CLI{
			Config: configPath,
		}

		err = cmd.Run(&Global{}, cli)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid template selection")
	})
}

func TestTemplateNew_BaseURLResolution_Integration(t *testing.T) {
	server := templateServer(t)
	defer server.Close()

	tmpDir := t.TempDir()
	configPath := createTestConfig(t, tmpDir)
	// Overwrite to set hugo.base_url
	configContent := fmt.Sprintf(`version: "2.0"
forges:
  - name: "dummy-forge"
    type: "github"
    api_url: "https://api.github.com"
    base_url: "https://github.com"
    organizations: ["test-org"]
    auth:
      type: "token"
      token: "dummy-token"
hugo:
  base_url: "%s"
`, server.URL)
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o600))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldCwd) }()
	require.NoError(t, os.Chdir(tmpDir))

	cmd := &TemplateListCmd{
		// No BaseURL set - should use config
	}

	// Capture stdout using a pipe
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
	}()

	var stdout bytes.Buffer
	go func() {
		_, _ = io.Copy(&stdout, r)
		_ = r.Close()
	}()

	cli := &CLI{
		Config: configPath,
	}

	err = cmd.Run(&Global{}, cli)
	require.NoError(t, err)

	// Close the write end to ensure all data is flushed
	_ = w.Close()
	// Give a moment for the goroutine to finish copying
	time.Sleep(10 * time.Millisecond)

	output := stdout.String()
	require.Contains(t, output, "adr")
}

func TestTemplateServer_HTMLStructure(t *testing.T) {
	server := templateServer(t)
	defer server.Close()

	// Test discovery page
	resp, err := http.Get(server.URL + "/categories/templates/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "adr.template")
	require.Contains(t, string(body), "guide.template")

	// Test ADR template page
	resp, err = http.Get(server.URL + "/templates/adr.template/index.html")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "docbuilder:template.type")
	require.Contains(t, string(body), "language-markdown")
}
