package lint

import (
	"path/filepath"
	"testing"
)

func TestSequencePrefixFilenameRule_Name(t *testing.T) {
	r := &SequencePrefixFilenameRule{}
	if got := r.Name(); got != ruleSequencePrefixFilename {
		t.Errorf("Name() = %q, want %q", got, ruleSequencePrefixFilename)
	}
}

// TestSequencePrefixFilenameRule_AppliesTo_AdrDir verifies the rule
// applies to files under the adr directory.
func TestSequencePrefixFilenameRule_AppliesTo_AdrDir(t *testing.T) {
	r := &SequencePrefixFilenameRule{}
	if !r.AppliesTo("docs/adr/adr-001-foo.md") {
		t.Error("AppliesTo(adr/adr-001-foo.md) = false, want true")
	}
}

// TestSequencePrefixFilenameRule_AppliesTo_OtherDir verifies the rule
// does not apply to files in non-sequence directories.
func TestSequencePrefixFilenameRule_AppliesTo_OtherDir(t *testing.T) {
	r := &SequencePrefixFilenameRule{}
	if r.AppliesTo("docs/how-to/foo.md") {
		t.Error("AppliesTo(how-to/foo.md) = true, want false")
	}
}

// TestSequencePrefixFilenameRule_ValidADR verifies a valid ADR filename passes.
func TestSequencePrefixFilenameRule_ValidADR(t *testing.T) {
	r := &SequencePrefixFilenameRule{}
	issues, err := r.Check(filepath.Join("docs", "adr", "adr-001-foo.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

// TestSequencePrefixFilenameRule_NoZeroPad verifies a non-zero-padded sequence is flagged.
func TestSequencePrefixFilenameRule_NoZeroPad(t *testing.T) {
	r := &SequencePrefixFilenameRule{}
	issues, err := r.Check(filepath.Join("docs", "adr", "adr-1-foo.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Rule != ruleSequencePrefixFilename {
		t.Errorf("Rule = %q, want %q", issues[0].Rule, ruleSequencePrefixFilename)
	}
}

// TestSequencePrefixFilenameRule_NoSequence verifies a file without a sequence is flagged.
func TestSequencePrefixFilenameRule_NoSequence(t *testing.T) {
	r := &SequencePrefixFilenameRule{}
	issues, err := r.Check(filepath.Join("docs", "adr", "adr-foo.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (no sequence), got %d", len(issues))
	}
}

// TestSequencePrefixFilenameRule_NonSequenceDir verifies a file outside
// any sequence directory is not flagged even if its filename doesn't
// match a sequence pattern.
func TestSequencePrefixFilenameRule_NonSequenceDir(t *testing.T) {
	r := &SequencePrefixFilenameRule{}
	issues, err := r.Check(filepath.Join("docs", "how-to", "foo.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues (non-sequence dir), got %d", len(issues))
	}
}

// TestSequencePrefixFilenameRule_CustomSequence verifies the SequenceDirs
// field allows extending to other sequence types.
func TestSequencePrefixFilenameRule_CustomSequence(t *testing.T) {
	r := &SequencePrefixFilenameRule{
		SequenceDirs: map[string]string{
			"runbook": "runbook-",
		},
	}
	if !r.AppliesTo("docs/runbook/runbook-001-foo.md") {
		t.Error("AppliesTo(runbook) = false, want true")
	}
	issues, err := r.Check(filepath.Join("docs", "runbook", "runbook-1-foo.md"))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (non-zero-pad in custom dir), got %d", len(issues))
	}
}
