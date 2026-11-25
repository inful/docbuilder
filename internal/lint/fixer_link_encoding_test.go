package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// A link target is a URL: the space of a file name is written as %20.
// The file on disk has the space, so the link is not broken - and the
// fixer can heal it when the file is renamed.
func TestTheLinterDoesNotReportAPercentEncodedLinkAsBroken(t *testing.T) {
	docsDir := t.TempDir()

	indexPath := filepath.Join(docsDir, "index.md")
	if err := os.WriteFile(indexPath, []byte("# Index\n\n[Other](Other%20Name.md)\n[With anchor](Other%20Name.md#section)\n"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "Other Name.md"), []byte("# Other\n"), 0o600); err != nil {
		t.Fatalf("write other: %v", err)
	}

	l := NewLinter(&Config{})
	res, err := l.LintPath(docsDir)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}

	for _, issue := range res.Issues {
		if issue.Rule == "broken-links" {
			t.Errorf("the link to the encoded name was reported as broken: %s", issue.Message)
		}
	}
}
