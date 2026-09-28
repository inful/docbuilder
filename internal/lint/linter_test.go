package lint

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeLinterTestDoc writes a minimal valid doc file for the linter.
// Mirrors the helper that used to live in rule_missing_index_page_test.go
// so we can build a fixture tree without depending on the deleted rule.
func writeLinterTestDoc(t *testing.T, dir, name string) {
	t.Helper()
	body := "---\n" +
		"title: x\n" +
		"uid: 00000000-0000-0000-0000-000000000000\n" +
		"date: 2026-01-01T00:00:00Z\n" +
		"lastmod: \"2026-01-01\"\n" +
		"fingerprint: 0000000000000000000000000000000000000000000000000000000000000000\n" +
		"categories:\n" +
		"  - explanation\n" +
		"tags:\n" +
		"  - test\n" +
		"aliases:\n" +
		"  - /_uid/x/\n" +
		"---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestLinter_NoMissingIndexPageIssues is the characterization test for
// the removal of the missing-index-page rule. The rule used to flag any
// directory with ≥3 .md children and no _index.md as a WARNING, with an
// auto-fixer that generated a placeholder landing page. The placeholder
// repeatedly failed downstream rules (frontmatter-required-fields,
// Detect Lint Rule Drift) and the WARNING noise outweighed its value.
//
// The desired end state: no registered lint rule produces an issue
// whose Rule field equals "missing-index-page", no matter what fixture
// tree the linter is pointed at.
//
// Pre-conditions for failure before this change: the rule IS registered,
// so this tree produces a "missing-index-page" issue and the assertion
// fires.
func TestLinter_NoMissingIndexPageIssues(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "how-to")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	// Five children, no _index.md — a tree that would have triggered the rule.
	writeLinterTestDoc(t, dir, "a.md")
	writeLinterTestDoc(t, dir, "b.md")
	writeLinterTestDoc(t, dir, "c.md")
	writeLinterTestDoc(t, dir, "d.md")
	writeLinterTestDoc(t, dir, "e.md")

	l := NewLinter(&Config{Format: "text"})
	result, err := l.LintPath(root)
	if err != nil {
		t.Fatalf("LintPath: %v", err)
	}
	for _, issue := range result.Issues {
		if issue.Rule == "missing-index-page" {
			t.Errorf("rule %q should not produce issues; got: %+v", issue.Rule, issue)
		}
	}
}

// TestLinter_NoMissingIndexPageRuleRegistered is a narrower companion
// to the test above. Even if some future refactor changed how the rule
// ran (e.g. moved CheckDirectory out of the lint path), the rule
// registration itself should not exist anymore.
func TestLinter_NoMissingIndexPageRuleRegistered(t *testing.T) {
	l := NewLinter(&Config{Format: "text"})
	for _, r := range l.rules {
		if r.Name() == "missing-index-page" {
			t.Errorf("rule %q should not be registered; it was deprecated", r.Name())
		}
	}
}

// TestFixResult_NoMissingIndexPagesField asserts the FixResult struct
// no longer carries a MissingIndexPages field. The field only existed
// to record the auto-fixer's output; with the fixer gone, the field is
// dead weight. Verified via reflection so the test compiles regardless
// of whether the field exists.
func TestFixResult_NoMissingIndexPagesField(t *testing.T) {
	rt := reflect.TypeFor[FixResult]()
	if _, found := rt.FieldByName("MissingIndexPages"); found {
		t.Errorf("FixResult should not have a MissingIndexPages field")
	}
}

// TestNoRuleImplementsDirectoryRule asserts no registered rule carries
// the CheckDirectory method that the DirectoryRule interface used to
// declare. Uses an inline anonymous interface so the test compiles
// whether or not DirectoryRule exists as a named type.
func TestNoRuleImplementsDirectoryRule(t *testing.T) {
	l := NewLinter(&Config{Format: "text"})
	for _, r := range l.rules {
		if _, ok := r.(interface {
			CheckDirectory(string) ([]Issue, error)
		}); ok {
			t.Errorf("rule %q still implements CheckDirectory; the DirectoryRule interface should be removed", r.Name())
		}
	}
}

// TestDocs_NoReferencesToRemovedRule asserts that no user-facing docs
// reference the removed missing-index-page rule by name. Stale doc
// references are the typical failure mode after a rule removal — the
// code goes away, the docs don't, and operators reading the migration
// guide or rules reference see a rule that no longer exists.
//
// The list is intentionally hardcoded (rather than a `docs/` walk)
// because we only want to guard against references to the specific
// rule id, not filename-level `_index.md` mentions elsewhere.
func TestDocs_NoReferencesToRemovedRule(t *testing.T) {
	// Walk up from this test file to the repo root.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	repoRoot := filepath.Join(wd, "..", "..")

	docFiles := []string{
		"docs/reference/lint-rules.md",
		"docs/reference/lint-rules-changelog.md",
		"docs/how-to/migrate-to-linting.md",
		"docs/explanation/docbuilder-best-practices.md",
	}

	for _, rel := range docFiles {
		path := filepath.Join(repoRoot, rel)
		// #nosec G304 -- path is a hardcoded list of doc paths, repoRoot
		// is derived by walking up from the test file.
		data, err := os.ReadFile(path)
		if err != nil {
			// File may not exist; skip rather than fail — the test's
			// job is to catch stale references, not to verify the
			// doc set is complete.
			t.Logf("skip %s: %v", rel, err)
			continue
		}
		// Catch references by the rule id (machine-readable) or by
		// user-facing phrasings ("Missing Section Index",
		// "missing section index detection"). Comparison is
		// case-insensitive to catch both "Missing Section Index"
		// (rule heading) and "missing section index detection"
		// (planned-item checkbox).
		lower := bytes.ToLower(data)
		for _, marker := range []string{"missing-index-page", "missing section index"} {
			if bytes.Contains(lower, []byte(marker)) {
				t.Errorf("%s still references the removed rule via %q; remove the reference", rel, marker)
			}
		}
	}
}
