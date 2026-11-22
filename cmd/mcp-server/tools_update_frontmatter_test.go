package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// A file whose frontmatter cannot be parsed must not lose it. The
// update writes the body alone when the frontmatter cannot be read,
// and the title, the uid and the aliases of the page are gone.
func TestUpdateDocKeepsAFrontmatterThatCannotBeParsed(t *testing.T) {
	docs := t.TempDir()
	path := filepath.Join(docs, "broken.md")
	original := "---\ntitle: Fix: the thing\nuid: abc-123\n---\n\nOld body\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	state := &serverState{docsDir: docs}
	ctx := context.WithValue(t.Context(), stateKey, state)

	req := mcp.CallToolRequest{}
	req.Params.Name = "update_doc"
	req.Params.Arguments = map[string]any{
		"path":    "broken.md",
		"content": "New body\n",
		"confirm": true,
	}

	result, err := handleUpdateDoc(ctx, req)
	if err != nil {
		t.Fatalf("handleUpdateDoc: %v", err)
	}
	if !result.IsError {
		t.Fatal("the tool did not refuse the write")
	}

	after, err := os.ReadFile(path) // #nosec G304 -- the path is the test's own file
	if err != nil {
		t.Fatalf("read the file: %v", err)
	}
	if string(after) != original {
		t.Fatalf("the file was changed:\n%s", after)
	}
}
