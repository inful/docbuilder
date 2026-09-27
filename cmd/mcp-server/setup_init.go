package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InitCmd sets up the docbuilder-mcp MCP server entry for one or more
// MCP clients (OpenCode, VS Code, ...). With no positional argument it
// auto-detects installed tools; with one or more arguments it configures
// just those tools.
//
// Default scope is project-level (writes `.vscode/mcp.json`,
// `opencode.json`, etc. inside the project so the whole team gets them
// from `git pull`). Pass --global for user-level config
// (`~/.copilot/mcp-config.json`, `~/.config/opencode/opencode.json`).
type InitCmd struct {
	Tools   []string `name:"tools" help:"Tools to configure (opencode, vscode). Auto-detect when empty." arg:"" optional:""`
	Global  bool     `name:"global" help:"Write user-level config instead of project-level"`
	DryRun  bool     `name:"dry-run" help:"Print what would happen without writing anything"`
	Config  string   `short:"c" name:"config" default:"config.yaml" help:"Path to docbuilder config.yaml to embed in the MCP server args"`
	DocsDir string   `name:"docs-dir" default:"./docs" help:"Docs directory to embed in the MCP server args"`
	Binary  string   `name:"binary" default:"docbuilder-mcp" help:"Path to the docbuilder-mcp binary (defaults to resolving 'docbuilder-mcp' from $PATH)"`
}

// Run executes the init command. Emits human-readable progress to stdout
// and returns errors with enough context for the user to fix and retry.
func (i *InitCmd) Run() error {
	return runSetup(i.Tools, i.Global, i.DryRun, i.Config, i.DocsDir, i.Binary, os.Stdout, promptYesNo)
}

// runSetup is the testable inner form of InitCmd.Run. It takes the input
// parameters plus output sinks and a prompt function so tests can drive
// it deterministically.
func runSetup(
	requested []string,
	global, dryRun bool,
	configPath, docsDir, binary string,
	stdout io.Writer,
	prompt func(io.Writer, string, bool) (bool, error),
) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	tools, err := resolveTools(requested, cwd)
	if err != nil {
		return err
	}
	tools, err = maybePromptForTools(tools, stdout, prompt)
	if err != nil {
		return err
	}

	spec := buildServerSpec(binary, configPath, docsDir)
	scopeLabel := "project"
	if global {
		scopeLabel = "user"
	}
	_, _ = fmt.Fprintf(stdout, "Setting up docbuilder-mcp (%s scope) for %s\n",
		scopeLabel, joinToolNames(tools))

	var failures []string
	for _, t := range tools {
		target := pickTarget(t, global)
		if target.Path == "" {
			failures = append(failures, fmt.Sprintf("%s: no path available", t.ID()))
			continue
		}
		if err := writeToolConfig(t, target, spec, dryRun, stdout); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", t.ID(), err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("setup failed for some tools:\n  %s", strings.Join(failures, "\n  "))
	}
	_, _ = fmt.Fprintln(stdout, "Done.")
	return nil
}

// pickTarget returns the project-level or user-level target for a tool.
func pickTarget(t toolSpec, global bool) configTarget {
	if global {
		return t.GlobalTarget()
	}
	proj := t.ProjectTarget()
	if proj.Path == "" {
		// Tool doesn't support project-level; fall back to global.
		return t.GlobalTarget()
	}
	return proj
}

// writeToolConfig renders the per-tool server entry and either merges it
// into the existing config file or prints what it would do.
func writeToolConfig(t toolSpec, target configTarget, spec serverSpec, dryRun bool, stdout io.Writer) error {
	entry, err := t.RenderServerEntry(spec)
	if err != nil {
		return fmt.Errorf("render server entry: %w", err)
	}

	var writePath string
	if filepath.IsAbs(target.Path) {
		writePath = target.Path
	} else {
		// ProjectTarget is relative to cwd; resolve against cwd.
		writePath = filepath.Join(cwdOrDefault(), target.Path)
	}

	action := "Would merge"
	if !dryRun {
		action = "Merged"
	}
	_, _ = fmt.Fprintf(stdout, "  %s server '%s' under '%s' into %s\n",
		action, serverNameForShape(target.ServerEntryShape), target.ServerKey, writePath)

	if dryRun {
		_, _ = fmt.Fprintf(stdout, "    + %s\n", indentJSON(entry))
		return nil
	}
	if err := mergeServerEntry(writePath, target.ServerKey, serverNameForShape(target.ServerEntryShape), entry); err != nil {
		return err
	}
	return nil
}

// buildServerSpec returns the serverSpec that every tool's MCP config
// will reference. The Command is the binary name (or absolute path), and
// the Args carry the project's --config and --docs-dir so the spawned
// server boots with the right context.
func buildServerSpec(binary, configPath, docsDir string) serverSpec {
	return serverSpec{
		Command: binary,
		Args:    []string{"--config", configPath, "--docs-dir", docsDir},
	}
}

// serverNameForShape returns the key used to register docbuilder inside
// the tool's config map. Currently always "docbuilder" — kept as a
// separate function so future versions can name per-tool if needed.
func serverNameForShape(_ serverEntryShape) string {
	return "docbuilder"
}

// joinToolNames renders "opencode, vscode" or just "vscode" for the
// progress message at the top of runSetup.
func joinToolNames(tools []toolSpec) string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.DisplayName())
	}
	return strings.Join(names, ", ")
}

// maybePromptForTools is the interactive step. If multiple tools were
// detected and we have a prompt function, ask the user to confirm which
// to configure. With a single detected tool, no prompt; with zero, the
// caller already handled that case.
func maybePromptForTools(tools []toolSpec, stdout io.Writer, prompt func(io.Writer, string, bool) (bool, error)) ([]toolSpec, error) {
	if len(tools) <= 1 || prompt == nil {
		return tools, nil
	}
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.ID())
	}
	sort.Strings(names)
	question := fmt.Sprintf("Multiple MCP clients detected (%s). Configure all of them? [Y/n] ", strings.Join(names, ", "))
	yes, err := prompt(stdout, question, true)
	if err != nil {
		return nil, err
	}
	if yes {
		return tools, nil
	}
	// User said no: filter to the first detected tool (deterministic order
	// from sort.Strings) so they can re-run with explicit IDs.
	if len(tools) > 0 {
		first := tools[0]
		for _, t := range tools {
			if t.ID() < first.ID() {
				first = t
			}
		}
		_, _ = fmt.Fprintf(stdout, "  -> configuring only %s\n", first.ID())
		return []toolSpec{first}, nil
	}
	return tools, nil
}

// cwdOrDefault returns the current working directory or "." if it
// cannot be resolved. Used to resolve project-relative targets without
// crashing on environments where $PWD is unset.
func cwdOrDefault() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// indentJSON returns a one-line-per-level rendering of a JSON byte slice
// suitable for embedding in human-readable progress output. Falls back
// to the raw string if the input isn't valid JSON.
func indentJSON(raw []byte) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(out)
}

// promptYesNo asks the user a yes/no question on stdin. Returns the
// default value if input is empty. Errors on EOF or read failure.
func promptYesNo(_ io.Writer, question string, defaultYes bool) (bool, error) {
	_, _ = fmt.Fprint(os.Stderr, question)
	var line string
	_, err := fmt.Scanln(&line)
	if err != nil {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return defaultYes, nil
	}
	return line == "y" || line == "yes", nil
}
