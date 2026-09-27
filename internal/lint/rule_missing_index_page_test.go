package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// writeDocFile writes a small markdown file under dir/name. It is the
// helper for setting up fixture trees for the missing-index-page rule.
func writeDocFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("---\ntitle: x\nuid: 00000000-0000-0000-0000-000000000000\ndate: 2026-01-01T00:00:00Z\nlastmod: \"2026-01-01\"\nfingerprint: 0000000000000000000000000000000000000000000000000000000000000000\ncategories:\n  - explanation\ntags:\n  - test\naliases:\n  - /_uid/x/\n---\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestMissingIndexPageRule_Name(t *testing.T) {
	r := &MissingIndexPageRule{}
	if got := r.Name(); got != ruleMissingIndexPage {
		t.Errorf("Name() = %q, want %q", got, ruleMissingIndexPage)
	}
}

func TestMissingIndexPageRule_AppliesTo(t *testing.T) {
	r := &MissingIndexPageRule{}
	if !r.AppliesTo("docs/foo.md") {
		t.Errorf("AppliesTo(.md) = false, want true")
	}
	if r.AppliesTo("docs/foo.txt") {
		t.Errorf("AppliesTo(.txt) = true, want false")
	}
}

// TestMissingIndexPageRule_NoIndexAboveThreshold verifies a directory
// with ≥3 .md children and no _index.md produces one issue.
func TestMissingIndexPageRule_NoIndexAboveThreshold(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "how-to")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeDocFile(t, dir, "a.md")
	writeDocFile(t, dir, "b.md")
	writeDocFile(t, dir, "c.md")

	r := &MissingIndexPageRule{}
	issues, err := r.CheckDirectory(root)
	if err != nil {
		t.Fatalf("CheckDirectory: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Rule != ruleMissingIndexPage {
		t.Errorf("Rule = %q, want %q", issues[0].Rule, ruleMissingIndexPage)
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("Severity = %v, want %v", issues[0].Severity, SeverityWarning)
	}
}

// TestMissingIndexPageRule_WithIndex verifies an _index.md suppresses the warning.
func TestMissingIndexPageRule_WithIndex(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "how-to")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeDocFile(t, dir, "a.md")
	writeDocFile(t, dir, "b.md")
	writeDocFile(t, dir, "c.md")
	writeDocFile(t, dir, "_index.md")

	r := &MissingIndexPageRule{}
	issues, err := r.CheckDirectory(root)
	if err != nil {
		t.Fatalf("CheckDirectory: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (has _index.md), got %d", len(issues))
	}
}

// TestMissingIndexPageRule_BelowThreshold verifies a directory with <3
// .md children is not flagged even without _index.md.
func TestMissingIndexPageRule_BelowThreshold(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "foo")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeDocFile(t, dir, "a.md")
	writeDocFile(t, dir, "b.md")

	r := &MissingIndexPageRule{}
	issues, err := r.CheckDirectory(root)
	if err != nil {
		t.Fatalf("CheckDirectory: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (<3 children), got %d", len(issues))
	}
}

// TestMissingIndexPageRule_FlatCollectionSkipped verifies the flat
// collections (adr/, examples/) are skipped.
func TestMissingIndexPageRule_FlatCollectionSkipped(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"adr", "examples"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 5; i++ {
			writeDocFile(t, dir, "doc"+string(rune('a'+i))+".md")
		}
	}

	r := &MissingIndexPageRule{}
	issues, err := r.CheckDirectory(root)
	if err != nil {
		t.Fatalf("CheckDirectory: %v", err)
	}
	for _, iss := range issues {
		if filepath.Base(filepath.Dir(iss.FilePath)) == "adr" ||
			filepath.Base(filepath.Dir(iss.FilePath)) == "examples" {
			t.Errorf("flat collection %q should be skipped; got issue: %v",
				filepath.Dir(iss.FilePath), iss)
		}
	}
}

// TestMissingIndexPageRule_CustomThreshold verifies MinChildren is honoured.
func TestMissingIndexPageRule_CustomThreshold(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "small")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeDocFile(t, dir, "a.md")

	r := &MissingIndexPageRule{MinChildren: 1}
	issues, err := r.CheckDirectory(root)
	if err != nil {
		t.Fatalf("CheckDirectory: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("MinChildren=1 with 1 child: expected 1 issue, got %d", len(issues))
	}
}
