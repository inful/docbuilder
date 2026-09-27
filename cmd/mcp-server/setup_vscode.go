package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// vscodeTool implements toolSpec for VS Code (and the GitHub Copilot Chat
// extension that ships with VS Code, which reads the same MCP config).
//
// VS Code MCP config schema (from code.visualstudio.com/docs/copilot/chat/mcp-servers):
//
//	// .vscode/mcp.json (workspace, VS Code format)
//	{
//	  "servers": {
//	    "<server-name>": {
//	      "type": "stdio",
//	      "command": "<binary>",
//	      "args": ["--config", "<path>"]
//	    }
//	  }
//	}
//
// User-level config lives at `~/.copilot/mcp-config.json` on recent VS Code
// versions (the doc mentions this for Agent Host compatibility); older
// versions read `mcp.json` from the user-profile folder.
type vscodeTool struct{}

// ID is the tool identifier users pass on the command line.
func (vscodeTool) ID() string { return "vscode" }

// DisplayName is the human-readable label.
func (vscodeTool) DisplayName() string { return "VS Code" }

// DetectInstalled returns true if VS Code looks present on this machine.
// Detection heuristics (any one matches):
//
//  1. Project-level: `.vscode/` directory exists in cwd or any ancestor.
//  2. User-level: the user-profile folder exists (created when VS Code
//     first runs) — different paths per OS.
//  3. CLI: `code` is on $PATH.
//
// False negatives are acceptable (a user with VS Code installed in a
// non-standard location can still pass `init vscode` explicitly).
func (vscodeTool) DetectInstalled() bool {
	// 1. Project-level `.vscode/` directory.
	cwd, _ := os.Getwd()
	for dir := cwd; dir != filepath.Dir(dir); {
		if info, err := os.Stat(filepath.Join(dir, ".vscode")); err == nil && info.IsDir() {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// 2. User-profile folder.
	if _, err := os.Stat(vscodeUserProfileDir()); err == nil {
		return true
	}
	// 3. CLI on PATH.
	if _, err := exec.LookPath("code"); err == nil {
		return true
	}
	return false
}

// ProjectTarget returns the workspace-relative path VS Code reads for
// MCP servers and the top-level key it uses ("servers").
func (vscodeTool) ProjectTarget() configTarget {
	return configTarget{
		Path:             filepath.Join(".vscode", "mcp.json"),
		ServerKey:        "servers",
		ServerEntryShape: shapeVSCode,
	}
}

// GlobalTarget returns the user-level MCP config path. Per VS Code docs,
// the Agent Host reads `~/.copilot/mcp-config.json` directly; older VS Code
// reads `mcp.json` inside the user-profile folder. We pick the modern
// path (cross-platform via $HOME / $APPDATA) since the user-profile
// `mcp.json` is also read by newer VS Code.
func (vscodeTool) GlobalTarget() configTarget {
	// Prefer the modern path; fall back to the legacy user-profile mcp.json.
	candidates := []string{
		filepath.Join(vscodeCopilotConfigDir(), "mcp-config.json"),
		filepath.Join(vscodeUserProfileDir(), "mcp.json"),
	}
	return configTarget{
		// Always pick the first candidate for write target. The other one is
		// a fallback discovery location, not a write target.
		Path:             candidates[0],
		ServerKey:        "servers",
		ServerEntryShape: shapeVSCode,
	}
}

// RenderServerEntry turns a serverSpec into the JSON object VS Code expects
// under its `servers.<name>` key. Includes `type: "stdio"` so VS Code's
// type-discriminated schema validates cleanly.
func (vscodeTool) RenderServerEntry(spec serverSpec) ([]byte, error) {
	entry := map[string]any{
		"type":    "stdio",
		"command": spec.Command,
		"args":    spec.Args,
	}
	return json.Marshal(entry)
}

// vscodeUserProfileDir returns the VS Code user-profile directory for the
// current OS. Linux: ~/.config/Code; macOS: ~/Library/Application Support/Code;
// Windows: %APPDATA%\Code.
func vscodeUserProfileDir() string {
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "Code")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "Library", "Application Support", "Code")
		}
	case "linux", "freebsd", "openbsd", "netbsd":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			// Honor $XDG_CONFIG_HOME per the XDG Base Directory spec.
			if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
				return filepath.Join(xdg, "Code")
			}
			return filepath.Join(home, ".config", "Code")
		}
	}
	return ""
}

// vscodeCopilotConfigDir returns the GitHub Copilot config directory used
// for `mcp-config.json` on the Agent Host path. Linux/macOS: ~/.copilot;
// Windows: %APPDATA%\copilot. We resolve conservatively even though VS
// Code's user profile uses different per-OS paths — the Copilot config
// directory is consistent across platforms.
func vscodeCopilotConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "copilot")
		}
	case "darwin", "linux", "freebsd", "openbsd", "netbsd":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".copilot")
		}
	}
	return ""
}
