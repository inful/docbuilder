package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
	"git.home.luguber.info/inful/docbuilder/internal/lint"
	templating "git.home.luguber.info/inful/docbuilder/internal/templates"
)

// registerTools wires every tool the v1 catalog exposes into the MCP server.
//
// Read-only tools advertise ReadOnlyHintAnnotation so the host can render
// them with a non-destructive affordance. Mutating tools advertise
// DestructiveHintAnnotation + require an explicit confirm=true in their
// arguments so a stray "yes" from the LLM is still gated by host-level
// permission dialogs.
func registerTools(srv *server.MCPServer, state *serverState) {
	// Read-only tools ------------------------------------------------------
	srv.AddTool(toolGetConfig(), stateMiddleware(state)(handleGetConfig))
	srv.AddTool(toolListTemplates(), stateMiddleware(state)(handleListTemplates))
	srv.AddTool(toolDescribeTemplate(), stateMiddleware(state)(handleDescribeTemplate))
	srv.AddTool(toolResolveTemplateInputs(), stateMiddleware(state)(handleResolveTemplateInputs))
	srv.AddTool(toolLintDocs(), stateMiddleware(state)(handleLintDocs))
	srv.AddTool(toolReadDoc(), stateMiddleware(state)(handleReadDoc))

	// Mutating tools -------------------------------------------------------
	srv.AddTool(toolCreateFromTemplate(), stateMiddleware(state)(handleCreateFromTemplate))
	srv.AddTool(toolLintFix(), stateMiddleware(state)(handleLintFix))
	srv.AddTool(toolCreateDoc(), stateMiddleware(state)(handleCreateDoc))
	srv.AddTool(toolUpdateDoc(), stateMiddleware(state)(handleUpdateDoc))
}

// ----- tool definitions ---------------------------------------------------

func toolGetConfig() mcp.Tool {
	return mcp.NewTool("get_config",
		mcp.WithDescription("Return the current docbuilder configuration with secrets redacted."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func toolListTemplates() mcp.Tool {
	return mcp.NewTool("list_templates",
		mcp.WithDescription("Discover templates available from the configured documentation site. "+
			"Each template has a type, name, and URL you can pass to describe_template or create_from_template."),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func toolDescribeTemplate() mcp.Tool {
	return mcp.NewTool("describe_template",
		mcp.WithDescription("Fetch a template page and return its schema (required fields, types, options) "+
			"and defaults. Use this to understand what a template expects before calling resolve_template_inputs "+
			"or create_from_template."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Template URL from list_templates.")),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func toolResolveTemplateInputs() mcp.Tool {
	return mcp.NewTool("resolve_template_inputs",
		mcp.WithDescription("Render the would-be output path and body for a template given field values, "+
			"without writing anything to disk. Use this to preview the result of create_from_template."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Template URL from list_templates.")),
		mcp.WithObject("inputs",
			mcp.Description("Field overrides keyed by schema field name. Any required field not provided will cause an error."),
		),
		mcp.WithBoolean("use_defaults", mcp.Description("When true, fall back to template defaults for any field not in `inputs`. Defaults to true.")),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func toolLintDocs() mcp.Tool {
	return mcp.NewTool("lint_docs",
		mcp.WithDescription("Lint a file or directory and return structured findings. Does not modify files. "+
			"Use lint_fix to apply fixes. Each issue carries a `fix` field with a concrete remediation "+
			"hint — surface it verbatim when reporting issues to the user, especially for rules "+
			"that lint_fix does not auto-fix (body-h1, tag-count, cross-mode-category)."),
		mcp.WithString("path", mcp.Description("Path to lint (file or directory). Defaults to the configured docs dir.")),
		mcp.WithBoolean("quiet", mcp.Description("Suppress warnings, only return errors.")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func toolReadDoc() mcp.Tool {
	return mcp.NewTool("read_doc",
		mcp.WithDescription("Read a documentation file and return its parsed frontmatter and body. "+
			"Path must be inside the configured docs directory."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path relative to the docs dir, or absolute path inside it.")),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func toolCreateFromTemplate() mcp.Tool {
	return mcp.NewTool("create_from_template",
		mcp.WithDescription("Create a new documentation file from a template, write it to the docs directory, "+
			"and run lint-fix on the result. Equivalent to `docbuilder template new --defaults` "+
			"but driven entirely by arguments (no interactive prompts)."),
		mcp.WithString("url", mcp.Required(), mcp.Description("Template URL from list_templates.")),
		mcp.WithObject("inputs", mcp.Description("Field overrides for the template. Required fields must all be supplied.")),
		mcp.WithBoolean("use_defaults", mcp.Description("Use template defaults for fields not in `inputs`. Defaults to false — the host must supply every required field.")),
		mcp.WithBoolean("confirm", mcp.Required(), mcp.Description("Must be true. The host's MCP permission prompt must already have approved this write.")),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
	)
}

func toolLintFix() mcp.Tool {
	return mcp.NewTool("lint_fix",
		mcp.WithDescription("Run the lint fixer on a file or directory. Will rename files, rewrite links, "+
			"and update frontmatter fingerprints where applicable."),
		mcp.WithString("path", mcp.Description("Path to fix (file or directory). Defaults to the configured docs dir.")),
		mcp.WithBoolean("dry_run", mcp.Description("If true, show what would change without writing.")),
		mcp.WithBoolean("confirm", mcp.Required(), mcp.Description("Must be true.")),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func toolCreateDoc() mcp.Tool {
	return mcp.NewTool("create_doc",
		mcp.WithDescription("Create a new documentation file with the given content and frontmatter. "+
			"Path must be inside the configured docs directory. Will not overwrite an existing file unless `overwrite` is true."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path relative to the docs dir, or absolute path inside it.")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Markdown body of the document.")),
		mcp.WithObject("frontmatter", mcp.Description("YAML frontmatter as a JSON object. Omit to create a file with no frontmatter.")),
		mcp.WithBoolean("overwrite", mcp.Description("Allow overwriting an existing file. Defaults to false.")),
		mcp.WithBoolean("lint_after", mcp.Description("Run lint-fix on the new file. Defaults to true.")),
		mcp.WithBoolean("confirm", mcp.Required(), mcp.Description("Must be true.")),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
	)
}

func toolUpdateDoc() mcp.Tool {
	return mcp.NewTool("update_doc",
		mcp.WithDescription("Modify an existing documentation file. Either replace the body, merge/patch the frontmatter, or both. "+
			"Path must be inside the configured docs directory."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path relative to the docs dir, or absolute path inside it.")),
		mcp.WithString("content", mcp.Description("New markdown body. If omitted, body is left unchanged.")),
		mcp.WithObject("frontmatter_patch", mcp.Description("Frontmatter keys to add or overwrite.")),
		mcp.WithString("merge_strategy",
			mcp.Enum("merge", "replace"),
			mcp.Description("How to apply frontmatter_patch: 'merge' (default) keeps existing keys, 'replace' overwrites the entire frontmatter."),
		),
		mcp.WithBoolean("lint_after", mcp.Description("Run lint-fix on the result. Defaults to false.")),
		mcp.WithBoolean("confirm", mcp.Required(), mcp.Description("Must be true.")),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

// ----- tool handlers ------------------------------------------------------

func handleGetConfig(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	redacted := redactConfig(state.cfg)
	// Marshal with indentation so the LLM gets something readable.
	b, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		return toolErr("marshal config", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handleListTemplates(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	if state.baseURL == "" {
		return toolErrMsg("no template base URL configured; pass --base-url or DOCBUILDER_TEMPLATE_BASE_URL, or set hugo.base_url in config.yaml")
	}

	httpClient := templating.NewTemplateHTTPClient()
	templates, err := templating.FetchTemplateDiscovery(ctx, state.baseURL, httpClient)
	if err != nil {
		return toolErr("fetch templates", err)
	}

	type out struct {
		Type string `json:"type"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	items := make([]out, len(templates))
	for i, t := range templates {
		items[i] = out{Type: t.Type, Name: t.Name, URL: t.URL}
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return toolErr("marshal templates", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handleDescribeTemplate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url, err := req.RequireString("url")
	if err != nil {
		return toolErrMsg("missing required argument: url")
	}

	page, err := fetchTemplatePage(ctx, url)
	if err != nil {
		return toolErr("fetch template page", err)
	}

	schema, err := templating.ParseTemplateSchema(page.Meta.Schema)
	if err != nil {
		return toolErr("parse schema", err)
	}
	defaults, err := templating.ParseTemplateDefaults(page.Meta.Defaults)
	if err != nil {
		return toolErr("parse defaults", err)
	}

	out := map[string]any{
		"type":        page.Meta.Type,
		"name":        page.Meta.Name,
		"output_path": page.Meta.OutputPath,
		"schema":      schema,
		"defaults":    defaults,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return toolErr("marshal", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handleResolveTemplateInputs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	url, err := req.RequireString("url")
	if err != nil {
		return toolErrMsg("missing required argument: url")
	}
	page, err := fetchTemplatePage(ctx, url)
	if err != nil {
		return toolErr("fetch template page", err)
	}

	schema, err := templating.ParseTemplateSchema(page.Meta.Schema)
	if err != nil {
		return toolErr("parse schema", err)
	}
	defaults, err := templating.ParseTemplateDefaults(page.Meta.Defaults)
	if err != nil {
		return toolErr("parse defaults", err)
	}

	overrides := stringMapFromArgs(req.GetArguments(), "inputs")
	useDefaults := req.GetBool("use_defaults", true)

	resolved, err := templating.ResolveTemplateInputs(schema, defaults, overrides, useDefaults, nil)
	if err != nil {
		return toolErr("resolve inputs", err)
	}

	// Render the would-be path and body, without writing.
	nextSeq, err := buildSequenceResolver(page, state.docsDir)
	if err != nil {
		return toolErr("build sequence resolver", err)
	}
	outputPath, err := templating.RenderOutputPath(page.Meta.OutputPath, resolved, nextSeq)
	if err != nil {
		return toolErr("render output path", err)
	}
	body, err := templating.RenderTemplateBody(page.Body, resolved, nextSeq)
	if err != nil {
		return toolErr("render body", err)
	}

	out := map[string]any{
		"resolved_inputs": resolved,
		"output_path":     outputPath,
		"full_path":       filepath.Join(state.docsDir, outputPath),
		"body":            body,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return toolErr("marshal", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handleCreateFromTemplate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	if !req.GetBool("confirm", false) {
		return toolErrMsg("confirm must be true to create a file")
	}
	url, err := req.RequireString("url")
	if err != nil {
		return toolErrMsg("missing required argument: url")
	}

	page, err := fetchTemplatePage(ctx, url)
	if err != nil {
		return toolErr("fetch template page", err)
	}

	schema, err := templating.ParseTemplateSchema(page.Meta.Schema)
	if err != nil {
		return toolErr("parse schema", err)
	}
	defaults, err := templating.ParseTemplateDefaults(page.Meta.Defaults)
	if err != nil {
		return toolErr("parse defaults", err)
	}

	overrides := stringMapFromArgs(req.GetArguments(), "inputs")
	useDefaults := req.GetBool("use_defaults", false)

	resolved, err := templating.ResolveTemplateInputs(schema, defaults, overrides, useDefaults, nil)
	if err != nil {
		return toolErr("resolve inputs", err)
	}

	nextSeq, err := buildSequenceResolver(page, state.docsDir)
	if err != nil {
		return toolErr("build sequence resolver", err)
	}
	outputPath, err := templating.RenderOutputPath(page.Meta.OutputPath, resolved, nextSeq)
	if err != nil {
		return toolErr("render output path", err)
	}
	body, err := templating.RenderTemplateBody(page.Body, resolved, nextSeq)
	if err != nil {
		return toolErr("render body", err)
	}

	writtenPath, err := templating.WriteGeneratedFile(state.docsDir, outputPath, body)
	if err != nil {
		return toolErr("write file", err)
	}

	// Match the CLI: run lint-fix on the new file.
	//nolint:contextcheck // lint.Fixer uses context.Background() internally; fix is fast and local.
	lintResult := runLintFixOn(writtenPath)

	out := map[string]any{
		"path":        writtenPath,
		"lint_result": lintResult,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return toolErr("marshal", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handleLintDocs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	path := req.GetString("path", "")
	if path == "" {
		path = state.docsDir
	}
	quiet := req.GetBool("quiet", false)

	cfg := &lint.Config{Quiet: quiet, Format: "json", Excludes: lintExcludes(state.cfg)}
	linter := lint.NewLinter(cfg)
	result, err := linter.LintPath(path)
	if err != nil {
		return toolErr("lint", err)
	}
	return lintResultToJSON(result)
}

func handleLintFix(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	if !req.GetBool("confirm", false) {
		return toolErrMsg("confirm must be true to apply fixes")
	}
	path := req.GetString("path", "")
	if path == "" {
		path = state.docsDir
	}
	dryRun := req.GetBool("dry_run", false)

	// lint.Fixer and lint.NewFixer don't accept context.Context. The fix
	// runs are local file operations that complete quickly, so using
	// context.Background() internally is acceptable here.
	//nolint:contextcheck // lint fixer is local-only and short-lived; context.Background() is appropriate
	fixer := lint.NewFixer(lint.NewLinter(&lint.Config{Yes: true, Excludes: lintExcludes(state.cfg)}), dryRun, false).WithAutoConfirm(true)
	//nolint:contextcheck // same rationale as above
	fixResult, err := fixer.Fix(path)
	if err != nil {
		return toolErr("fix", err)
	}

	// After the fix completes, surface the issues that couldn't be
	// auto-fixed so the LLM can present them to the user with their
	// per-issue `fix` hints. We re-lint (read-only) and filter to rules
	// the fixer knows how to handle.
	manualRequired := computeManualRequired(path, state)

	type fixResponse struct {
		*lint.FixResult
		ManualRequired []manualIssueOut `json:"manual_required,omitempty"`
	}
	resp := fixResponse{
		FixResult:       fixResult,
		ManualRequired: manualRequired,
	}
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return toolErr("marshal", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

// autoFixableRules is the closed set of lint rules for which the fixer
// has a built-in remediation. Issues for rules NOT in this set are
// surfaced to the caller as `manual_required` (they need a human
// decision: removing an H1, curating tags, picking a primary doc mode).
var autoFixableRules = map[string]bool{
	"filename-conventions":            true,
	"frontmatter-uid":                 true,
	"frontmatter-fingerprint":         true,
	"frontmatter-required-fields":     true,
	"directory-category-consistency":  true,
	"category-naming":                 true,
	"internal-link-style":             true,
	"sequence-prefix-filename":        true,
	"missing-index-page":              true,
	"broken-links":                    true,
}

// manualIssueOut is a slimmed-down view of an issue for the
// `manual_required` array in the lint_fix response. We omit the
// heavy `explanation` text and surface only the high-signal fields
// the LLM needs to action the fix.
type manualIssueOut struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Severity    string `json:"severity"`
	Rule        string `json:"rule"`
	Message     string `json:"message"`
	Fix         string `json:"fix"`
}

// computeManualRequired re-lints the docs tree after a fix and
// returns issues whose rule isn't auto-fixable. It runs as a read-only
// lint pass so the fixer's own output isn't re-fixed in a loop.
func computeManualRequired(path string, _ *serverState) []manualIssueOut {
	//nolint:contextcheck // local file walk; context.Background() is appropriate
	linter := lint.NewLinter(&lint.Config{Format: "text"})
	lintResult, err := linter.LintPath(path)
	if err != nil {
		// Lint failures are non-fatal here: the manual_required list
		// is an enhancement, not a critical path.
		return nil
	}
	var out []manualIssueOut
	for _, iss := range lintResult.Issues {
		if iss.Severity != lint.SeverityError && iss.Severity != lint.SeverityWarning {
			continue
		}
		if autoFixableRules[iss.Rule] {
			continue
		}
		out = append(out, manualIssueOut{
			File:     iss.FilePath,
			Line:     iss.Line,
			Severity: iss.Severity.String(),
			Rule:     iss.Rule,
			Message:  iss.Message,
			Fix:      iss.Fix,
		})
	}
	return out
}

func handleReadDoc(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	relPath, err := req.RequireString("path")
	if err != nil {
		return toolErrMsg("missing required argument: path")
	}
	abs, err := resolveInDocs(state.docsDir, relPath)
	if err != nil {
		return toolErrMsg(err.Error())
	}

	data, err := os.ReadFile(abs) // #nosec G304 -- path resolved + contained above
	if err != nil {
		return toolErr("read file", err)
	}

	fm, body, had, _, splitErr := frontmatter.Split(data)
	if splitErr != nil {
		return toolErr("split frontmatter", splitErr)
	}

	out := map[string]any{
		"path":            abs,
		"had_frontmatter": had,
		"raw_frontmatter": string(fm),
		"body":            string(body),
	}
	if had {
		parsed, parseErr := frontmatter.ParseYAML(fm)
		if parseErr != nil {
			out["frontmatter_error"] = parseErr.Error()
		} else {
			out["frontmatter"] = parsed
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return toolErr("marshal", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handleCreateDoc(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	if !req.GetBool("confirm", false) {
		return toolErrMsg("confirm must be true to create a file")
	}
	relPath, err := req.RequireString("path")
	if err != nil {
		return toolErrMsg("missing required argument: path")
	}
	content, err := req.RequireString("content")
	if err != nil {
		return toolErrMsg("missing required argument: content")
	}
	overwrite := req.GetBool("overwrite", false)
	lintAfter := req.GetBool("lint_after", true)

	abs, err := resolveInDocs(state.docsDir, relPath)
	if err != nil {
		return toolErrMsg(err.Error())
	}
	if _, statErr := os.Stat(abs); statErr == nil && !overwrite {
		return toolErrMsg("file exists; pass overwrite=true to replace it")
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(abs), 0o750); mkdirErr != nil {
		return toolErr("mkdir", mkdirErr)
	}

	fm := mapFromArgs(req.GetArguments(), "frontmatter")
	full := composeDoc(fm, content)
	if writeErr := os.WriteFile(abs, full, 0o600); writeErr != nil { // #nosec G304
		return toolErr("write file", writeErr)
	}

	out := map[string]any{"path": abs, "wrote": true}
	if lintAfter {
		//nolint:contextcheck // lint fixer uses context.Background() internally; fix is local
		out["lint_result"] = runLintFixOn(abs)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return toolErr("marshal", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func handleUpdateDoc(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	state := stateFromContext(ctx)
	if !req.GetBool("confirm", false) {
		return toolErrMsg("confirm must be true to update a file")
	}
	relPath, err := req.RequireString("path")
	if err != nil {
		return toolErrMsg("missing required argument: path")
	}

	abs, err := resolveInDocs(state.docsDir, relPath)
	if err != nil {
		return toolErrMsg(err.Error())
	}
	existing, err := os.ReadFile(abs) // #nosec G304
	if err != nil {
		return toolErr("read file", err)
	}
	oldFM, oldBody, had, style, splitErr := frontmatter.Split(existing)
	if splitErr != nil {
		return toolErr("split existing", splitErr)
	}

	newBody := string(oldBody)
	if c, ok := req.GetArguments()["content"]; ok && c != nil {
		newBody = req.GetString("content", newBody)
	}

	strategy := req.GetString("merge_strategy", "merge")
	patch := mapFromArgs(req.GetArguments(), "frontmatter_patch")

	// Match the on-disk style so the rewritten file doesn't drift.
	writeStyle := frontmatter.Style{Newline: style.Newline, HasTrailingNewline: style.HasTrailingNewline}

	var newFM []byte
	switch strategy {
	case "replace":
		newFM = marshalYAML(patch, writeStyle)
	case "merge", "":
		existingFM := map[string]any{}
		if had {
			if parsed, perr := frontmatter.ParseYAML(oldFM); perr == nil {
				existingFM = parsed
			}
		}
		maps.Copy(existingFM, patch)
		newFM = marshalYAML(existingFM, writeStyle)
	default:
		return toolErrMsg("merge_strategy must be 'merge' or 'replace'")
	}

	full := frontmatter.Join(newFM, []byte(newBody), len(newFM) > 0, style)
	if writeErr := os.WriteFile(abs, full, 0o600); writeErr != nil { // #nosec G304,G703 -- path resolved + contained inside docsDir via resolveInDocs
		return toolErr("write file", writeErr)
	}

	out := map[string]any{"path": abs, "wrote": true, "merge_strategy": strategy}
	if req.GetBool("lint_after", false) {
		//nolint:contextcheck // lint fixer uses context.Background() internally; fix is local
		out["lint_result"] = runLintFixOn(abs)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return toolErr("marshal", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

// ----- shared helpers -----------------------------------------------------

// fetchTemplatePage is the common boilerplate for any tool that needs to
// load a template by URL.
func fetchTemplatePage(ctx context.Context, url string) (*templating.TemplatePage, error) {
	client := templating.NewTemplateHTTPClient()
	page, err := templating.FetchTemplatePage(ctx, url, client)
	if err != nil {
		return nil, err
	}
	return page, nil
}

// buildSequenceResolver mirrors the CLI's logic: pull the page's sequence
// definition (plus the implicit ADR one when type==adr) so RenderOutputPath
// and RenderTemplateBody can number things automatically.
func buildSequenceResolver(page *templating.TemplatePage, docsDir string) (func(string) (int, error), error) {
	defs := map[string]templating.SequenceDefinition{}

	if page.Meta.Sequence != "" {
		def, err := templating.ParseSequenceDefinition(page.Meta.Sequence)
		if err != nil && !errors.Is(err, templating.ErrNoSequenceDefinition) {
			return nil, err
		}
		if def != nil {
			defs[def.Name] = *def
		}
	}
	if _, ok := defs["adr"]; !ok && strings.EqualFold(page.Meta.Type, "adr") {
		defs["adr"] = templating.SequenceDefinition{
			Name:  "adr",
			Dir:   "adr",
			Glob:  "adr-*.md",
			Regex: `^adr-(\d{3})-`,
			Width: 3,
			Start: 1,
		}
	}
	return func(name string) (int, error) {
		def, ok := defs[name]
		if !ok {
			return 0, fmt.Errorf("unknown sequence: %s", name)
		}
		return templating.ComputeNextInSequence(def, docsDir)
	}, nil
}

// runLintFixOn applies lint-fix to a single file. Mirrors the CLI's
// runLintFix helper. Failures are non-fatal — the caller decides whether to
// surface them.
func runLintFixOn(path string) map[string]any {
	cfg := &lint.Config{Format: "text", Fix: true, Yes: true}
	fixer := lint.NewFixer(lint.NewLinter(cfg), false, false).WithAutoConfirm(true)
	res, err := fixer.Fix(path)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{
		"renames":       len(res.FilesRenamed),
		"links_updated": len(res.LinksUpdated),
		"fingerprints":  len(res.Fingerprints),
		"has_errors":    res.HasErrors(),
		"summary":       res.Summary(),
	}
}

// lintExcludes extracts the lint exclude patterns from a server config,
// returning nil (not an empty slice) when the config has no lint block.
// nil means "use the linter's built-in defaults".
func lintExcludes(cfg *config.Config) []string {
	if cfg == nil || cfg.Lint == nil {
		return nil
	}
	return cfg.Lint.Excludes
}

// resolveInDocs turns a relative or absolute path into one guaranteed to be
// inside docsDir. Returns a clear error otherwise.
func resolveInDocs(docsDir, requested string) (string, error) {
	cleanDocs := filepath.Clean(docsDir)
	candidate := requested
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(cleanDocs, candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	// Containment check: prefix with trailing separator so /docs-evil
	// doesn't match /docs.
	if !strings.HasPrefix(abs, cleanDocs+string(filepath.Separator)) && abs != cleanDocs {
		return "", fmt.Errorf("path %q is outside docs directory %q", requested, docsDir)
	}
	return abs, nil
}

// lintResultToJSON formats a lint.Result into a small, LLM-friendly JSON
// shape that captures counts and per-issue details.
func lintResultToJSON(result *lint.Result) (*mcp.CallToolResult, error) {
	type issueOut struct {
		File        string `json:"file"`
		Line        int    `json:"line"`
		Severity    string `json:"severity"`
		Rule        string `json:"rule"`
		Message     string `json:"message"`
		Explanation string `json:"explanation,omitempty"`
		Fix         string `json:"fix,omitempty"`
	}
	out := struct {
		FilesTotal   int        `json:"files_total"`
		ErrorCount   int        `json:"error_count"`
		WarningCount int        `json:"warning_count"`
		Issues       []issueOut `json:"issues"`
	}{
		FilesTotal: result.FilesTotal,
	}
	for _, iss := range result.Issues {
		out.Issues = append(out.Issues, issueOut{
			File:        iss.FilePath,
			Line:        iss.Line,
			Severity:    iss.Severity.String(),
			Rule:        iss.Rule,
			Message:     iss.Message,
			Explanation: iss.Explanation,
			Fix:         iss.Fix,
		})
		switch iss.Severity {
		case lint.SeverityError:
			out.ErrorCount++
		case lint.SeverityWarning:
			out.WarningCount++
		case lint.SeverityInfo:
			// Info findings are surfaced in the issues array but do not
			// contribute to the error or warning counts.
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return toolErr("marshal lint result", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

// stringMapFromArgs pulls a JSON object out of the arguments map and treats
// every value as a string — what ResolveTemplateInputs expects.
func stringMapFromArgs(args map[string]any, key string) map[string]string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(obj))
	for k, v := range obj {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// mapFromArgs pulls a JSON object out of the arguments map and returns it as
// map[string]any (what we want for frontmatter manipulation).
func mapFromArgs(args map[string]any, key string) map[string]any {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	if obj, ok := raw.(map[string]any); ok {
		return obj
	}
	return nil
}

// composeDoc writes frontmatter + body into a single file body. We round-trip
// through frontmatter.Join so YAML quoting and style detection match the
// rest of the codebase.
func composeDoc(fm map[string]any, content string) []byte {
	if len(fm) == 0 {
		return []byte(content)
	}
	style := frontmatter.Style{Newline: "\n", HasTrailingNewline: true}
	return frontmatter.Join(marshalYAML(fm, style), []byte(content), true, style)
}

// marshalYAML is a thin wrapper over frontmatter.SerializeYAML that swallows
// the error and returns empty bytes — callers that hit a marshal failure are
// usually about to write something they constructed themselves; the error
// surfaces at the next step.
func marshalYAML(fm map[string]any, style frontmatter.Style) []byte {
	if len(fm) == 0 {
		return nil
	}
	out, err := frontmatter.SerializeYAML(fm, style)
	if err != nil {
		return nil
	}
	return out
}

// toolErr is a small wrapper so handlers can return a single line on error.
func toolErr(op string, err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(fmt.Sprintf("%s: %v", op, err)), nil
}

func toolErrMsg(msg string) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(msg), nil
}
