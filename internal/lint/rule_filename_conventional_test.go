package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// The git forges recognize the conventional names: the home page, the
// contribution guide, the license. A README.md that the fixer renames
// to readme.md is no home page any more.
func TestTheConventionalNamesAreNotRenamed(t *testing.T) {
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs", "sub")
	if err := os.MkdirAll(docs, 0o750); err != nil {
		t.Fatalf("make the directory: %v", err)
	}
	readme := filepath.Join(docs, "README.md")
	if err := os.WriteFile(readme, []byte("# Home\n"), 0o600); err != nil {
		t.Fatalf("write the readme: %v", err)
	}

	l := NewLinter(&Config{Format: formatText})
	res, err := l.LintPath(filepath.Join(dir, "docs"))
	if err != nil {
		t.Fatalf("lint: %v", err)
	}

	for _, issue := range res.Issues {
		if issue.Rule == ruleFilenameConventions && issue.Severity == SeverityError {
			t.Errorf("the conventional name was reported: %s", issue.Message)
		}
	}
}

// A name that is not conventional is still reported.
func TestANameThatIsNotConventionalIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Guide.md"), []byte("# Guide\n"), 0o600); err != nil {
		t.Fatalf("write the guide: %v", err)
	}

	l := NewLinter(&Config{Format: formatText})
	res, err := l.LintPath(dir)
	if err != nil {
		t.Fatalf("lint: %v", err)
	}

	found := false
	for _, issue := range res.Issues {
		if issue.Rule == ruleFilenameConventions && issue.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatal("the name with uppercase letters was not reported")
	}
}
