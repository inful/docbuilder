package main

import (
	"strings"
	"testing"
)

// TestServerInstructions_ContainsKeyPhases locks down the structure of the
// instructions string. If you intentionally change a workflow or a safety
// guarantee, update both the const and this test in the same commit.
func TestServerInstructions_ContainsKeyPhases(t *testing.T) {
	instr := serverInstructions

	// Workflow sections — each top-level task should be reachable.
	wantPhrases := []string{
		// Header / scope
		"--docs-dir",
		"out of scope",
		// Create-from-template workflow
		"Workflow for creating a doc from a template",
		"list_templates",
		"describe_template",
		"resolve_template_inputs",
		"create_from_template",
		// Edit workflow
		"Workflow for editing an existing doc",
		"read_doc",
		"update_doc",
		// From-scratch workflow
		"Workflow for a from-scratch doc",
		"create_doc",
		// Safety
		"confirm: true",
		"confined to --docs-dir",
		"\"***\"",
		// Pitfalls
		"Pitfalls",
		"lint_fix",
		"overwrite",
		"merge_strategy",
	}
	for _, want := range wantPhrases {
		if !strings.Contains(instr, want) {
			t.Errorf("serverInstructions missing %q", want)
		}
	}
}

// TestServerInstructions_NotEmpty is a smoke test — the const must be
// non-empty so the LLM actually gets guidance. Catches the case where
// someone deletes the body during refactoring.
func TestServerInstructions_NotEmpty(t *testing.T) {
	if strings.TrimSpace(serverInstructions) == "" {
		t.Fatal("serverInstructions must not be empty")
	}
	if len(serverInstructions) > 8000 {
		t.Errorf("serverInstructions is %d bytes; keep under 8000 to avoid bloating the LLM context", len(serverInstructions))
	}
}
