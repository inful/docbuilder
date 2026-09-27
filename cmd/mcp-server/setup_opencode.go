package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// opencodeTool implements toolSpec for OpenCode.
//
// OpenCode MCP config schema (from opencode.ai/docs/mcp-servers):
//
//	{
//	  "$schema": "https://opencode.ai/config.json",
//	  "mcp": {
//	    "<server-name>": {
//	      "type": "local",
//	      "command": ["<binary>", "--config", "<path>"],
//	      "enabled": true
//	    }
//	  }
//	}
type opencodeTool struct{}

// ID is the tool identifier users pass on the command line.
func (opencodeTool) ID() string { return "opencode" }

// DisplayName is the human-readable label.
func (opencodeTool) DisplayName() string { return "OpenCode" }

// DetectInstalled returns true if OpenCode looks present on this machine.
// Detection heuristics (any one matches):
//
//  1. Project-level: `opencode.json` or `opencode.jsonc` exists in cwd or any
//     ancestor directory.
//  2. User-level: `~/.config/opencode/` directory exists (Linux/macOS) or
//     `%APPDATA%\opencode\` (Windows).
//  3. CLI: `opencode` is on $PATH.
//
// False negatives are acceptable (a user with OpenCode installed in a
// non-standard location can still pass `init opencode` explicitly).
func (opencodeTool) DetectInstalled() bool {
	// 1. Project-level marker files.
	cwd, _ := os.Getwd()
	for dir := cwd; dir != filepath.Dir(dir); {
		for _, name := range []string{"opencode.json", "opencode.jsonc"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// 2. User-level config dir.
	if _, err := os.Stat(opencodeUserConfigDir()); err == nil {
		return true
	}
	// 3. CLI on PATH.
	if _, err := exec.LookPath("opencode"); err == nil {
		return true
	}
	return false
}

// ProjectTarget returns the project-level config path and OpenCode's
// server key ("mcp") under it. Relative to cwd.
func (opencodeTool) ProjectTarget() configTarget {
	return configTarget{
		Path:             "opencode.json",
		ServerKey:        "mcp",
		ServerEntryShape: shapeOpenCode,
	}
}

// GlobalTarget returns the user-level config path. Returns an empty Path
// if the per-OS user config directory cannot be resolved (e.g. $HOME unset).
func (opencodeTool) GlobalTarget() configTarget {
	return configTarget{
		Path:             filepath.Join(opencodeUserConfigDir(), "opencode.json"),
		ServerKey:        "mcp",
		ServerEntryShape: shapeOpenCode,
	}
}

// RenderServerEntry turns a serverSpec into the JSON object OpenCode expects
// under its `mcp.<name>` key. The entry is always a self-contained object
// with `type: "local"` (required for local servers), `command` as an array
// (NOT a string + args — OpenCode uses one combined array), and `enabled: true`.
func (opencodeTool) RenderServerEntry(spec serverSpec) ([]byte, error) {
	entry := map[string]any{
		"type":    "local",
		"command": append([]string{spec.Command}, spec.Args...),
		"enabled": true,
	}
	return json.Marshal(entry)
}

// opencodeUserConfigDir returns the OpenCode user-config directory for the
// current OS, or "" if it cannot be resolved. Linux/macOS use the XDG-ish
// ~/.config/opencode; Windows uses %APPDATA%\opencode.
func opencodeUserConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "opencode")
		}
	case "darwin", "linux", "freebsd", "openbsd", "netbsd":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".config", "opencode")
		}
	}
	return ""
}
