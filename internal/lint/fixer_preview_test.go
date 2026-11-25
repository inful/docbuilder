package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// countingRule counts how often each file is linted.
type countingRule struct {
	counts map[string]int
}

func (c *countingRule) Name() string { return "counting" }

func (c *countingRule) AppliesTo(_ string) bool { return true }

func (c *countingRule) Check(filePath string) ([]Issue, error) {
	c.counts[filePath]++
	return nil, nil
}

// The interactive fix path shows a preview and then applies the fixes. Both
// passes must come from the same lint run, otherwise the user approves one
// set of changes and a different set is applied.
func TestFixWithConfirmationLintsTheTreeOnce(t *testing.T) {
	docsDir := filepath.Join(t.TempDir(), "docs")
	if err := os.MkdirAll(docsDir, 0o750); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "API Guide.md"), []byte("# API Guide\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	linter := NewLinter(&Config{})
	counter := &countingRule{counts: map[string]int{}}
	linter.rules = append(linter.rules, counter)

	fixer := NewFixer(linter, false, false).WithAutoConfirm(true)
	result, err := fixer.FixWithConfirmation(docsDir)
	if err != nil {
		t.Fatalf("fix with confirmation: %v", err)
	}
	if len(result.FilesRenamed) == 0 {
		t.Fatal("expected the file to be renamed")
	}
	if len(counter.counts) == 0 {
		t.Fatal("expected the tree to be linted")
	}
	for file, count := range counter.counts {
		if count != 1 {
			t.Fatalf("file %s was linted %d times, want 1", file, count)
		}
	}
}
