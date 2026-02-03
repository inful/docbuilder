package pipeline

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestRewriteLinkPath(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		repository   string
		forge        string
		isIndex      bool
		docPath      string
		isSingleRepo bool
		want         string
	}{
		{
			name:         "Index file in subdirectory - relative link preserves directory",
			path:         "configure-env-exposure.md",
			repository:   "servejs",
			forge:        "",
			isIndex:      true,
			docPath:      "servejs/how-to/_index.md",
			isSingleRepo: false,
			want:         "/servejs/how-to/configure-env-exposure",
		},
		{
			name:       "Index file in subdirectory with forge - relative link preserves directory",
			path:       "authentication.md",
			repository: "docs",
			forge:      "gitlab",
			isIndex:    true,
			docPath:    "gitlab/docs/how-to/_index.md",
			want:       "/gitlab/docs/how-to/authentication",
		},
		{
			name:       "Index file at repository root - relative link",
			path:       "getting-started.md",
			repository: "myrepo",
			forge:      "",
			isIndex:    true,
			docPath:    "myrepo/_index.md",
			want:       "/myrepo/getting-started",
		},
		{
			name:       "Index file with ../ navigation",
			path:       "../other-section/file.md",
			repository: "myrepo",
			forge:      "",
			isIndex:    true,
			docPath:    "myrepo/section/_index.md",
			want:       "/myrepo/other-section/file",
		},
		{
			name:       "Regular file - relative link preserves directory context",
			path:       "sibling.md",
			repository: "myrepo",
			forge:      "",
			isIndex:    false,
			docPath:    "myrepo/section/page.md",
			want:       "/myrepo/section/sibling",
		},
		{
			name:       "Regular file in subdirectory - relative link preserves directory (servejs api.md case)",
			path:       "config.md",
			repository: "servejs",
			forge:      "",
			isIndex:    false,
			docPath:    "content/servejs/reference/api.md",
			want:       "/servejs/reference/config",
		},
		{
			name:       "Regular file in subdirectory - sibling link",
			path:       "other.md",
			repository: "docs",
			forge:      "",
			isIndex:    false,
			docPath:    "docs/guides/tutorial.md",
			want:       "/docs/guides/other",
		},
		{
			name:       "Index file - subdirectory link",
			path:       "guide/setup.md",
			repository: "docs",
			forge:      "",
			isIndex:    true,
			docPath:    "docs/_index.md",
			want:       "/docs/guide/setup",
		},
		{
			name:       "Index file with content/ prefix - relative link (servejs case)",
			path:       "tutorials/index.md",
			repository: "servejs",
			forge:      "",
			isIndex:    true,
			docPath:    "content/servejs/_index.md",
			want:       "/servejs/tutorials/",
		},
		{
			name:       "Index file with content/ prefix in subdirectory",
			path:       "configure.md",
			repository: "servejs",
			forge:      "",
			isIndex:    true,
			docPath:    "content/servejs/how-to/_index.md",
			want:       "/servejs/how-to/configure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteLinkPath(tt.path, tt.repository, tt.forge, tt.isIndex, tt.docPath, tt.isSingleRepo)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractDirectory(t *testing.T) {
	tests := []struct {
		name     string
		hugoPath string
		forge    string
		want     string
	}{
		{
			name:     "Index in subdirectory",
			hugoPath: "servejs/how-to/_index.md",
			forge:    "",
			want:     "how-to",
		},
		{
			name:     "Index at repository root",
			hugoPath: "servejs/_index.md",
			forge:    "",
			want:     "",
		},
		{
			name:     "Regular file in subdirectory",
			hugoPath: "myrepo/api/reference.md",
			forge:    "",
			want:     "api",
		},
		{
			name:     "Nested subdirectory",
			hugoPath: "myproject/guide/advanced/_index.md",
			forge:    "",
			want:     "guide/advanced",
		},
		{
			name:     "File with forge namespace",
			hugoPath: "gitlab/myrepo/how-to/index.md",
			forge:    "gitlab",
			want:     "how-to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDirectory(tt.hugoPath, false, tt.forge) // Multi-repo tests
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestRewriteRelativeLinks_CatastrophicBacktracking tests that the regex doesn't
// suffer from catastrophic backtracking with edge cases that caused hangs.
func TestRewriteRelativeLinks_CatastrophicBacktracking(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedSubstr string // substring that should appear in output
		maxDuration    time.Duration
	}{
		{
			name: "many brackets without matching link",
			content: strings.Repeat("[", 100) + "text" + strings.Repeat("]", 100) +
				" [valid link](./path.md)",
			expectedSubstr: "[valid link](",
			maxDuration:    100 * time.Millisecond,
		},
		{
			name: "many parentheses without matching link",
			content: strings.Repeat("(", 100) + "text" + strings.Repeat(")", 100) +
				" [valid link](./path.md)",
			expectedSubstr: "[valid link](",
			maxDuration:    100 * time.Millisecond,
		},
		{
			name:           "unmatched bracket in text followed by link",
			content:        "Some text [ with unmatched bracket\n\n[valid link](./path.md)",
			expectedSubstr: "[valid link](",
			maxDuration:    100 * time.Millisecond,
		},
		{
			name:           "code block with many brackets",
			content:        "```go\nfor i := range items {\n    process(items[i])\n}\n```\n[link](./doc.md)",
			expectedSubstr: "[link](",
			maxDuration:    100 * time.Millisecond,
		},
		{
			name:           "nested structures",
			content:        "[[nested]] [more [nested] stuff] (not a link)\n[real link](./file.md)",
			expectedSubstr: "[real link](",
			maxDuration:    100 * time.Millisecond,
		},
		{
			name:           "long text between brackets and parens",
			content:        "[text with " + strings.Repeat("very ", 200) + "long content](./path.md)",
			expectedSubstr: "[text with",
			maxDuration:    100 * time.Millisecond,
		},
	}

	cfg := &config.Config{}
	transform := rewriteRelativeLinks(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{
				Path:       "content/docs/test.md",
				Repository: "docs",
				Content:    tt.content,
			}

			// Run with timeout to catch hangs
			done := make(chan bool)
			var resultDocs []*Document
			var err error

			go func() {
				resultDocs, err = transform(doc)
				done <- true
			}()

			select {
			case <-done:
				assert.NoError(t, err)
				assert.Nil(t, resultDocs) // transform doesn't create new docs
				assert.Contains(t, doc.Content, tt.expectedSubstr)
			case <-time.After(tt.maxDuration):
				t.Fatalf("Transform took longer than %v - likely catastrophic backtracking", tt.maxDuration)
			}
		})
	}
}

// TestRewriteRelativeLinks_RealWorldContent tests with content similar to what caused hangs.
func TestRewriteRelativeLinks_RealWorldContent(t *testing.T) {
	// Load actual ADR-002 file that caused the hang
	content, err := os.ReadFile("../../../docs/adr/adr-002-in-memory-content-pipeline.md")
	assert.NoError(t, err)

	cfg := &config.Config{}
	transform := rewriteRelativeLinks(cfg)

	doc := &Document{
		Path:       "content/docs/adr/adr-002.md",
		Repository: "docs",
		Section:    "adr",
		Content:    string(content),
	}

	// Measure the time directly
	start := time.Now()
	_, err = transform(doc)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration.Milliseconds(), int64(100), "Transform should complete in under 100ms but took %v", duration)
	t.Logf("Transform completed in %v", duration)
}

func TestRewriteLinkPath_SingleRepo(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		repository   string
		forge        string
		isIndex      bool
		docPath      string
		isSingleRepo bool
		want         string
	}{
		{
			name:         "Single repo - parent directory navigation",
			path:         "../reference/configuration.md",
			repository:   "local",
			forge:        "",
			isIndex:      false,
			docPath:      "content/how-to/enable-page-transitions.md",
			isSingleRepo: true,
			want:         "/reference/configuration",
		},
		{
			name:         "Single repo - same directory link",
			path:         "./enable-hugo-render.md",
			repository:   "local",
			forge:        "",
			isIndex:      false,
			docPath:      "content/how-to/enable-page-transitions.md",
			isSingleRepo: true,
			want:         "/how-to/enable-hugo-render",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteLinkPath(tt.path, tt.repository, tt.forge, tt.isIndex, tt.docPath, tt.isSingleRepo)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRewriteLinkPath_RootRelativeMixedCase_SingleRepo(t *testing.T) {
	got := rewriteLinkPath("/Drift/gitlab-profile-ssh.png", "local", "", false, "content/drift/page.md", true)
	assert.Equal(t, "/drift/gitlab-profile-ssh.png", got)
}

func TestRewriteLinkPath_RootRelativeMarkdownMixedCase_MultiRepo(t *testing.T) {
	got := rewriteLinkPath("/How-To/Authentication.MD#SSH", "DocsRepo", "GitLab", false, "content/gitlab/docsrepo/guide/page.md", false)
	assert.Equal(t, "/gitlab/docsrepo/how-to/authentication#SSH", got)
}
