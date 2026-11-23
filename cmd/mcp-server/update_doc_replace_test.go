package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// A replacement of the frontmatter keeps the identity of the page.
//
// The uid and the aliases are what the links of the other pages point
// at: a replacement that dropped them made the /_uid/ address of the
// page 404, and the next lint gave the page another uid.
func TestAReplacementKeepsTheUIDAndTheAliases(t *testing.T) {
	docs := t.TempDir()
	path := filepath.Join(docs, "guide.md")

	content := "---\ntitle: Guide\nuid: 11111111-2222-3333-4444-555555555555\naliases:\n  - /_uid/11111111-2222-3333-4444-555555555555/\n---\n\n# Guide\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	state := &serverState{docsDir: docs}
	ctx := context.WithValue(t.Context(), stateKey, state)

	req := mcp.CallToolRequest{}
	req.Params.Name = "update_doc"
	req.Params.Arguments = map[string]any{
		"path":              "guide.md",
		"merge_strategy":    "replace",
		"frontmatter_patch": map[string]any{"title": "The guide"},
		"confirm":           true,
	}

	result, err := handleUpdateDoc(ctx, req)
	if err != nil {
		t.Fatalf("handleUpdateDoc: %v", err)
	}
	if result.IsError {
		t.Fatalf("the tool refused the replacement: %+v", result)
	}

	after, err := os.ReadFile(path) // #nosec G304 -- the path is the test's own file
	if err != nil {
		t.Fatalf("read the file: %v", err)
	}

	if !strings.Contains(string(after), "11111111-2222-3333-4444-555555555555") {
		t.Fatalf("the uid of the page is gone:\n%s", after)
	}
	if !strings.Contains(string(after), "The guide") {
		t.Fatalf("the patch was not applied:\n%s", after)
	}
}
