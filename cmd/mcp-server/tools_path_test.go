package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// The MCP server hands the fixer a path. Every other tool that touches a
// file keeps it inside the docs directory; the fixer did not, so a caller
// could rename and rewrite files anywhere on the machine.
func TestLintFixStaysInsideTheDocsDirectory(t *testing.T) {
	docsDir := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "API Guide.md")
	if err := os.WriteFile(outsideFile, []byte("# API Guide\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ctx := context.WithValue(context.Background(), stateKey, &serverState{docsDir: docsDir})
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "lint_fix",
			Arguments: map[string]any{"confirm": true, "path": outside},
		},
	}

	res, err := handleLintFix(ctx, req)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected the fixer to refuse a path outside the docs directory")
	}

	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("the file outside the docs directory must not be touched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "api-guide.md")); err == nil {
		t.Fatal("the file outside the docs directory must not be renamed")
	}
}
