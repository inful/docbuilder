package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// PathCmd shows where the MCP config file would be written for a given
// tool. Useful when scripting around docbuilder-mcp init or when the user
// just wants to know what file to edit by hand.
type PathCmd struct {
	Tool   string `name:"tool" help:"Tool to show the path for (opencode, vscode)" arg:""`
	Global bool   `name:"global" help:"Show global/user-level path instead of project-level"`
}

// Run executes the path command. Emits the absolute target path to stdout.
func (p *PathCmd) Run() error {
	cwd, _ := os.Getwd()
	tools, err := resolveTools([]string{p.Tool}, cwd)
	if err != nil {
		return err
	}
	t := tools[0]
	target := pickTarget(t, p.Global)
	if target.Path == "" {
		scope := "project"
		if p.Global {
			scope = "global"
		}
		return fmt.Errorf("tool %q does not support %s-level config", t.ID(), scope)
	}
	out := target.Path
	if !filepath.IsAbs(out) {
		if abs, err := filepath.Abs(out); err == nil {
			out = abs
		}
	}
	_, _ = fmt.Fprintln(os.Stdout, out)
	return nil
}
