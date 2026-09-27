package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PrintCmd prints the MCP config JSON for one or more tools without
// writing anything to disk. Useful for copy/paste setup or for piping
// into other tooling.
type PrintCmd struct {
	Tools   []string `name:"tools" help:"Tools to print config for (opencode, vscode). Required." arg:"" optional:""`
	Config  string   `short:"c" name:"config" default:"config.yaml" help:"Path to docbuilder config.yaml to embed in the MCP server args"`
	DocsDir string   `name:"docs-dir" default:"./docs" help:"Docs directory to embed in the MCP server args"`
	Binary  string   `name:"binary" default:"docbuilder-mcp" help:"Path to the docbuilder-mcp binary"`
	Global  bool     `name:"global" help:"Print global/user-level path instead of project-level"`
}

// Run executes the print command. Emits one JSON object per tool to stdout
// in a deterministic order, separated by header lines so the user can
// tell which tool each block targets.
func (p *PrintCmd) Run() error {
	if len(p.Tools) == 0 {
		return errors.New("at least one tool id is required (e.g. `docbuilder-mcp print opencode`)")
	}
	cwd, _ := os.Getwd()
	tools, err := resolveTools(p.Tools, cwd)
	if err != nil {
		return err
	}
	spec := buildServerSpec(p.Binary, p.Config, p.DocsDir)

	// Sort by ID for deterministic output.
	sort.SliceStable(tools, func(i, j int) bool { return tools[i].ID() < tools[j].ID() })

	for _, t := range tools {
		target := pickTarget(t, p.Global)
		entry, err := t.RenderServerEntry(spec)
		if err != nil {
			return fmt.Errorf("render %s: %w", t.ID(), err)
		}
		writePath := target.Path
		if !strings.HasPrefix(writePath, "/") {
			if abs, absErr := os.Getwd(); absErr == nil {
				writePath = filepath.Join(abs, writePath)
			}
		}
		_, _ = fmt.Fprintf(os.Stdout, "# %s MCP config (path: %s)\n", t.DisplayName(), writePath)
		// Pretty-print the single entry to make copy/paste easy.
		var v any
		_ = json.Unmarshal(entry, &v)
		pretty, err := json.MarshalIndent(map[string]any{
			target.ServerKey: map[string]any{
				serverNameForShape(target.ServerEntryShape): v,
			},
		}, "", "  ")
		if err != nil {
			_, _ = fmt.Fprintln(os.Stdout, string(entry))
		} else {
			_, _ = fmt.Fprintln(os.Stdout, string(pretty))
		}
		_, _ = fmt.Fprintln(os.Stdout)
	}
	return nil
}
