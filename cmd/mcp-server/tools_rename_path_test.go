package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// The fixer renames a file whose name does not follow the conventions.
// The answer must hold the path of the file as it is now, or the agent
// loses track of the file it just wrote.
func TestCreateDocReportsThePathAfterTheRename(t *testing.T) {
	docs := t.TempDir()
	state := &serverState{docsDir: docs}
	ctx := context.WithValue(t.Context(), stateKey, state)

	req := mcp.CallToolRequest{}
	req.Params.Name = "create_doc"
	req.Params.Arguments = map[string]any{
		"path":    "Guide.md",
		"content": "# Guide\n",
		"confirm": true,
	}

	result, err := handleCreateDoc(ctx, req)
	if err != nil {
		t.Fatalf("handleCreateDoc: %v", err)
	}
	if result.IsError {
		t.Fatalf("the tool failed: %s", toolText(result))
	}

	// The file is at the lowercased name.
	renamed := filepath.Join(docs, "guide.md")
	if _, err := os.Stat(renamed); err != nil {
		t.Fatalf("the file was not renamed: %v", err)
	}

	// The answer holds the path of the file as it is now.
	text := toolText(result)
	if !strings.Contains(text, "guide.md") {
		t.Fatalf("the answer does not hold the new path: %s", text)
	}
	if strings.Contains(text, "Guide.md") {
		t.Fatalf("the answer holds the old path: %s", text)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("read the answer: %v", err)
	}
	if out["path"] != renamed {
		t.Fatalf("the answer holds %v, want %q", out["path"], renamed)
	}
}
