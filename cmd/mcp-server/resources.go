package main

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerResources wires the resource providers the server advertises.
//
// We keep this list deliberately small for v1: config is the only thing an
// LLM reliably needs before doing anything else. Add `docbuilder://info`,
// `docbuilder://repositories`, etc. as follow-ups once the tool set is stable.
func registerResources(srv *server.MCPServer, state *serverState) {
	srv.AddResource(
		mcp.NewResource(
			"config://current",
			"docbuilder config",
			mcp.WithResourceDescription("Current resolved docbuilder configuration with secrets redacted."),
			mcp.WithMIMEType("application/json"),
		),
		func(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return resourceConfigCurrent(ctx, state)
		},
	)
}

// resourceConfigCurrent returns the redacted config as a JSON resource so the
// host can show it to the user even without invoking a tool.
func resourceConfigCurrent(_ context.Context, state *serverState) ([]mcp.ResourceContents, error) {
	redacted := redactConfig(state.cfg)
	b, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "config://current",
			MIMEType: "application/json",
			Text:     string(b),
		},
	}, nil
}
