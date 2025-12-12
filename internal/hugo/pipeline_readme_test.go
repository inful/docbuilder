package hugo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestPipelineReadmeLinks validates that README.md files (which become _index.md) have links rewritten correctly.
func TestPipelineReadmeLinks(t *testing.T) {
	cfg := &config.Config{}
	gen := &Generator{config: cfg}

	// README.md content with various link types
	md := `# Main Documentation

Links to sibling pages:
- [Guide](guide.md)
- [API Reference](api-reference.md)

Links to subdirectories:
- [How-to Guide](how-to/authentication.md)

Relative parent links:
- [Back](../other.md)
`
	file := docs.DocFile{
		Path:         "/repo/docs/README.md",
		RelativePath: "README.md",
		Repository:   "myrepo",
		Name:         "README", // This will become _index.md
		Extension:    ".md",
		Content:      []byte(md),
	}
	_ = file.LoadContent()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	if err := gen.copyContentFiles(ctx, []docs.DocFile{file}); err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	out := filepath.Join(gen.buildRoot(), file.GetHugoPath())
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	content := string(b)

	// For README.md (becomes _index.md), links should be rewritten WITHOUT extra ../
	// because _index.md is at the section root, not one level deeper
	expectations := []struct {
		shouldContain string
		reason        string
	}{
		{"[Guide](guide/)", "sibling page link should have .md removed and trailing slash added"},
		{"[API Reference](api-reference/)", "sibling link with dash should work"},
		{"[How-to Guide](how-to/authentication/)", "subdirectory link should work"},
		{"[Back](../other/)", "parent link should stay as ../ (not ../../) for index pages"},
	}

	for _, exp := range expectations {
		if !strings.Contains(content, exp.shouldContain) {
			t.Errorf("expected content to contain %q (%s); got:\n%s", exp.shouldContain, exp.reason, content)
		}
	}

	// These should NOT be in the output
	badPatterns := []string{
		"guide.md",         // .md extension should be removed
		"../../other/",     // Should be ../other/ not ../../other/ for index pages
		"api-reference.md", // .md extension should be removed
	}
	for _, bad := range badPatterns {
		if strings.Contains(content, bad) {
			t.Errorf("content should NOT contain %q; got:\n%s", bad, content)
		}
	}
}
