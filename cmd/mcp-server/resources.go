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

	// structure://doc-page — canonical shape of a docbuilder doc, in a
	// form an LLM can act on. LLMs that are about to call create_doc /
	// update_doc should read this first so they know the required and
	// optional frontmatter fields, body rules, and filename conventions.
	// The full human-readable counterpart lives in
	// docs/reference/doc-page-structure.md; the test
	// TestDocPageStructure_StaysInSyncWithDoc keeps the two in step.
	srv.AddResource(
		mcp.NewResource(
			"structure://doc-page",
			"documentation page structure",
			mcp.WithResourceDescription("Canonical schema for a docbuilder documentation page: required and optional frontmatter fields, body rules, filename conventions, and a worked example. Read this before calling create_doc / create_from_template / update_doc."),
			mcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return resourceDocPageStructure(ctx, state)
		},
	)

	// structure://template — canonical shape of a docbuilder template.
	// LLMs that are authoring templates or invoking them via
	// create_from_template should read this for the schema-field types,
	// output-path template syntax, sequence configuration, and the
	// one-fenced-markdown-block body rule.
	srv.AddResource(
		mcp.NewResource(
			"structure://template",
			"documentation template structure",
			mcp.WithResourceDescription("Canonical schema for a docbuilder documentation template: required/optional params.docbuilder.template.* frontmatter fields, schema field types and properties, output-path template variables, sequence configuration, and the one-block body rule. Read this before invoking or authoring templates."),
			mcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return resourceDocTemplateStructure(ctx, state)
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

// resourceDocPageStructure returns the canonical schema for a docbuilder
// documentation page. The content lives in `docPageStructureSchema`
// (structure_resource.go); the full human-readable counterpart is
// `docs/reference/doc-page-structure.md`.
func resourceDocPageStructure(_ context.Context, _ *serverState) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "structure://doc-page",
			MIMEType: "text/markdown",
			Text:     docPageStructureSchema,
		},
	}, nil
}

// resourceDocTemplateStructure returns the canonical schema for a
// docbuilder documentation template. The content lives in
// `docTemplateStructureSchema` (structure_template_resource.go); the
// full human-readable counterpart is
// `docs/reference/doc-template-structure.md`.
func resourceDocTemplateStructure(_ context.Context, _ *serverState) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "structure://template",
			MIMEType: "text/markdown",
			Text:     docTemplateStructureSchema,
		},
	}, nil
}
