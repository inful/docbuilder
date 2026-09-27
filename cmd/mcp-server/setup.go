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

// serverSpec is the rendered "docbuilder" entry that goes inside each tool's
// MCP config JSON. Different tools wrap it differently (different top-level
// keys, different field names), but the binary path + args are the same.
type serverSpec struct {
	// Command is the binary name or absolute path. Defaults to "docbuilder-mcp"
	// (resolves via $PATH at spawn time) but can be overridden to an absolute
	// path with --binary.
	Command string
	// Args are the flags passed to the binary at spawn time.
	Args []string
}

// configTarget is one location where an MCP client's config can be written.
type configTarget struct {
	// Path is either absolute (user-level / global) or relative to the
	// current working directory (project-level).
	Path string
	// ServerKey is the top-level key under which MCP server entries live in
	// the JSON file (e.g. "servers" for VS Code, "mcp" for OpenCode).
	ServerKey string
	// ServerEntryShape describes how this tool wraps a serverSpec into JSON.
	// Per-tool: see renderVSCodeServerEntry / renderOpenCodeServerEntry.
	ServerEntryShape serverEntryShape
}

// serverEntryShape tags how to render a single serverSpec into the tool's
// JSON. Each tool has its own renderer because the field names differ
// (e.g. OpenCode wants `command: [...]` and `type: "local"`).
type serverEntryShape int

const (
	// shapeVSCode matches VS Code's `.vscode/mcp.json` schema:
	//   "servers": { "<name>": { "type": "stdio", "command": "<cmd>",
	//                             "args": ["--config", "..."] } }
	shapeVSCode serverEntryShape = iota
	// shapeOpenCode matches OpenCode's `opencode.json` schema:
	//   "mcp": { "<name>": { "type": "local",
	//                         "command": ["<cmd>", "--config", "..."],
	//                         "enabled": true } }
	shapeOpenCode
)

// toolSpec describes one MCP client tool that this binary knows how to
// configure. Add a new tool by implementing this interface and registering
// it in the allTools slice.
type toolSpec interface {
	// ID is the kebab-case identifier the user passes to `init`/`print`/`path`.
	ID() string
	// DisplayName is the human-readable name used in prompts and error messages.
	DisplayName() string
	// DetectInstalled returns true if the tool looks installed on this
	// machine. Detection is intentionally cheap (filesystem checks for known
	// config dirs or CLIs); false negatives are acceptable, false positives
	// mean we offer to configure a tool that's not actually present.
	DetectInstalled() bool
	// ProjectTarget returns the workspace-relative path where the tool reads
	// its MCP config. Empty if the tool doesn't support project-level config.
	ProjectTarget() configTarget
	// GlobalTarget returns the user-level absolute path where the tool reads
	// its MCP config. Empty if the tool doesn't support user-level config.
	GlobalTarget() configTarget
	// RenderServerEntry turns a serverSpec into the JSON object this tool
	// expects under its server key.
	RenderServerEntry(spec serverSpec) ([]byte, error)
}

// mergeServerEntry merges a server spec into an existing JSON config file
// under the given serverKey. It preserves any existing top-level keys
// (settings, theme, model, etc.) and other servers. If the file does not
// exist it is created as {"<serverKey>": {"<name>": <entry>}}.
//
// This is intentionally tool-agnostic at the file-shape level: each tool's
// toolSpec.RenderServerEntry owns the per-server JSON shape; this function
// only handles the "merge into existing JSON / create new" boilerplate.
func mergeServerEntry(configPath, serverKey, serverName string, entry []byte) error {
	// Pretty-print the entry: read the raw bytes from RenderServerEntry, which
	// may be a JSON object, then re-marshal the merged result so the file
	// stays readable.
	rawEntry := map[string]any{}
	if err := json.Unmarshal(entry, &rawEntry); err != nil {
		return fmt.Errorf("invalid server entry JSON for %s/%s: %w", serverKey, serverName, err)
	}

	existing := map[string]any{}
	data, readErr := os.ReadFile(configPath) // #nosec G304 -- caller-controlled path
	if readErr == nil {
		if len(data) > 0 {
			if err := json.Unmarshal(data, &existing); err != nil {
				return fmt.Errorf("existing config %s is not valid JSON: %w", configPath, err)
			}
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	}

	container, _ := existing[serverKey].(map[string]any)
	if container == nil {
		container = map[string]any{}
	}
	container[serverName] = rawEntry
	existing[serverKey] = container

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	// End with a newline so the file is friendly to POSIX tooling.
	out = append(out, '\n')

	// Atomic write: write to a temp file alongside the target, then rename.
	if mkErr := os.MkdirAll(filepath.Dir(configPath), 0o750); mkErr != nil {
		return mkErr
	}
	tmp, err := os.CreateTemp(filepath.Dir(configPath), ".mcp-setup-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath) // no-op if rename succeeded
	}()
	if _, wErr := tmp.Write(out); wErr != nil {
		_ = tmp.Close()
		return wErr
	}
	if cErr := tmp.Close(); cErr != nil {
		return cErr
	}
	return os.Rename(tmpPath, configPath)
}

// resolveTools returns the requested tools (or auto-detected ones if none
// were requested) in a deterministic order. Unknown tool IDs surface as
// an error so users get a clear message rather than a silent fallback.
func resolveTools(requested []string, cwd string) ([]toolSpec, error) {
	if len(requested) == 0 {
		detected := detectInstalledTools(cwd)
		if len(detected) == 0 {
			return nil, errors.New(
				"no MCP clients detected on this machine; pass tool names explicitly " +
					"(e.g. `docbuilder-mcp init opencode`) or install one of: " + strings.Join(knownToolIDs(), ", "),
			)
		}
		return detected, nil
	}
	byID := map[string]toolSpec{}
	for _, t := range allTools() {
		byID[t.ID()] = t
	}
	var resolved []toolSpec
	for _, id := range requested {
		t, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf(
				"unknown tool %q (supported: %s)",
				id, strings.Join(knownToolIDs(), ", "),
			)
		}
		resolved = append(resolved, t)
	}
	return resolved, nil
}

// detectInstalledTools returns the subset of allTools that look installed
// on this machine. Order is the registration order (sorted for determinism).
func detectInstalledTools(_ string) []toolSpec {
	var found []toolSpec
	for _, t := range allTools() {
		if t.DetectInstalled() {
			found = append(found, t)
		}
	}
	// Deterministic order for prompts and tests.
	sort.SliceStable(found, func(i, j int) bool { return found[i].ID() < found[j].ID() })
	return found
}

// knownToolIDs returns the registered tool IDs in a stable order.
func knownToolIDs() []string {
	tools := allTools()
	ids := make([]string, 0, len(tools))
	for _, t := range tools {
		ids = append(ids, t.ID())
	}
	sort.Strings(ids)
	return ids
}
