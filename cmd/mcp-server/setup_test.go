package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempCwd creates a temp directory and chdir's into it for the duration
// of the test. Returns the dir and a cleanup func.
func withTempCwd(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir, func() {}
}

func TestOpenCodeTool_RenderServerEntry(t *testing.T) {
	t.Parallel()
	spec := serverSpec{Command: "docbuilder-mcp", Args: []string{"--config", "x.yaml"}}
	got, err := opencodeTool{}.RenderServerEntry(spec)
	if err != nil {
		t.Fatalf("RenderServerEntry: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v["type"] != "local" {
		t.Errorf("type = %v, want local", v["type"])
	}
	if v["enabled"] != true {
		t.Errorf("enabled = %v, want true", v["enabled"])
	}
	cmd, ok := v["command"].([]any)
	if !ok {
		t.Fatalf("command not an array: %T", v["command"])
	}
	if len(cmd) != 3 || cmd[0] != "docbuilder-mcp" || cmd[1] != "--config" || cmd[2] != "x.yaml" {
		t.Errorf("command = %v, want [docbuilder-mcp --config x.yaml]", cmd)
	}
}

func TestVSCodeTool_RenderServerEntry(t *testing.T) {
	t.Parallel()
	spec := serverSpec{Command: "docbuilder-mcp", Args: []string{"--config", "x.yaml"}}
	got, err := vscodeTool{}.RenderServerEntry(spec)
	if err != nil {
		t.Fatalf("RenderServerEntry: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v["type"] != "stdio" {
		t.Errorf("type = %v, want stdio", v["type"])
	}
	if v["command"] != "docbuilder-mcp" {
		t.Errorf("command = %v, want docbuilder-mcp", v["command"])
	}
	args, ok := v["args"].([]any)
	if !ok {
		t.Fatalf("args not an array: %T", v["args"])
	}
	if len(args) != 2 || args[0] != "--config" || args[1] != "x.yaml" {
		t.Errorf("args = %v, want [--config x.yaml]", args)
	}
}

func TestOpenCodeTool_ProjectTarget(t *testing.T) {
	t.Parallel()
	tr := opencodeTool{}.ProjectTarget()
	if tr.ServerKey != "mcp" {
		t.Errorf("ServerKey = %q, want mcp", tr.ServerKey)
	}
	if tr.Path != "opencode.json" {
		t.Errorf("Path = %q, want opencode.json", tr.Path)
	}
}

func TestVSCodeTool_ProjectTarget(t *testing.T) {
	t.Parallel()
	tr := vscodeTool{}.ProjectTarget()
	if tr.ServerKey != "servers" {
		t.Errorf("ServerKey = %q, want servers", tr.ServerKey)
	}
	if tr.Path != filepath.Join(".vscode", "mcp.json") {
		t.Errorf("Path = %q, want .vscode/mcp.json", tr.Path)
	}
}

func TestMergeServerEntry_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	entry := json.RawMessage(`{"type":"local","command":["b","--x"],"enabled":true}`)
	if err := mergeServerEntry(path, "mcp", "docbuilder", entry); err != nil {
		t.Fatalf("merge: %v", err)
	}

	data, err := os.ReadFile(path) // #nosec G304 -- test-controlled path
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, data)
	}
	container := got["mcp"].(map[string]any)
	if _, ok := container["docbuilder"]; !ok {
		t.Errorf("docbuilder entry missing; got %v", container)
	}
}

func TestMergeServerEntry_PreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	existing := map[string]any{
		"editor":  map[string]any{"formatOnSave": true},
		"servers": map[string]any{"other-tool": map[string]any{"type": "stdio", "command": "other"}},
		"$schema": "https://example.com/schema.json",
	}
	data, mErr := json.MarshalIndent(existing, "", "  ")
	if mErr != nil {
		t.Fatalf("marshal: %v", mErr)
	}
	if wErr := os.WriteFile(path, append(data, '\n'), 0o600); wErr != nil {
		t.Fatalf("write existing: %v", wErr)
	}

	entry := json.RawMessage(`{"type":"stdio","command":"docbuilder-mcp","args":["--config","x.yaml"]}`)
	if mErr := mergeServerEntry(path, "servers", "docbuilder", entry); mErr != nil {
		t.Fatalf("merge: %v", mErr)
	}

	updated, rErr := os.ReadFile(path) // #nosec G304 -- test-controlled path
	if rErr != nil {
		t.Fatalf("read: %v", rErr)
	}
	var got map[string]any
	if uErr := json.Unmarshal(updated, &got); uErr != nil {
		t.Fatalf("unmarshal: %v\n%s", uErr, updated)
	}
	// Existing keys must survive.
	if got["$schema"] != "https://example.com/schema.json" {
		t.Errorf("lost $schema key: %v", got["$schema"])
	}
	if got["editor"].(map[string]any)["formatOnSave"] != true {
		t.Errorf("lost editor.formatOnSave: %v", got["editor"])
	}
	// Both servers must coexist.
	servers := got["servers"].(map[string]any)
	if _, ok := servers["other-tool"]; !ok {
		t.Errorf("lost other-tool entry: %v", servers)
	}
	if _, ok := servers["docbuilder"]; !ok {
		t.Errorf("docbuilder entry missing: %v", servers)
	}
}

func TestMergeServerEntry_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	entry := json.RawMessage(`{"type":"stdio","command":"docbuilder-mcp","args":["--config","x.yaml"]}`)

	for range 3 {
		if err := mergeServerEntry(path, "servers", "docbuilder", entry); err != nil {
			t.Fatalf("merge: %v", err)
		}
	}
	data, _ := os.ReadFile(path) // #nosec G304 -- test-controlled path
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	servers := got["servers"].(map[string]any)
	if _, exists := servers["docbuilder"]; !exists {
		t.Errorf("docbuilder entry missing on idempotent run")
	}
}

func TestMergeServerEntry_InvalidExistingJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	entry := json.RawMessage(`{"type":"stdio","command":"x"}`)
	if err := mergeServerEntry(path, "servers", "docbuilder", entry); err == nil {
		t.Errorf("expected error for invalid JSON; got nil")
	}
}

func TestResolveTools_EmptyArgs_NoDetection_ReturnsError(t *testing.T) {
	dir, cleanup := withTempCwd(t)
	defer cleanup()
	// Force the detection step to find nothing by stripping every signal it
	// uses: create an empty HOME so ~/.config/opencode is absent, point
	// $PATH at an empty dir so neither `opencode` nor `code` resolves.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	emptyPath := t.TempDir()
	t.Setenv("PATH", emptyPath)

	// Ensure no project-level marker files exist in or above the temp dir.
	// (withTempCwd chdir'd into a fresh t.TempDir so nothing is present.)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(""), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := resolveTools(nil, dir)
	if err == nil {
		t.Fatalf("expected error when no tools detected; got nil")
	}
	if !strings.Contains(err.Error(), "no MCP clients detected") {
		t.Errorf("error message = %q; want it to mention 'no MCP clients detected'", err.Error())
	}
}

func TestResolveTools_UnknownID_ReturnsError(t *testing.T) {
	_, cleanup := withTempCwd(t)
	defer cleanup()
	_, err := resolveTools([]string{"made-up-tool"}, "/tmp")
	if err == nil {
		t.Fatalf("expected error for unknown tool; got nil")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error message = %q; want 'unknown tool'", err.Error())
	}
}

func TestResolveTools_KnownID_ReturnsTool(t *testing.T) {
	_, cleanup := withTempCwd(t)
	defer cleanup()
	tools, err := resolveTools([]string{"opencode"}, "/tmp")
	if err != nil {
		t.Fatalf("resolveTools: %v", err)
	}
	if len(tools) != 1 || tools[0].ID() != "opencode" {
		t.Errorf("got %v; want [opencode]", tools)
	}
}

func TestRunSetup_DryRun_DoesNotWrite(t *testing.T) {
	dir, cleanup := withTempCwd(t)
	defer cleanup()

	var stdout strings.Builder
	if err := runSetup([]string{"opencode"}, false, true, "/tmp/x.yaml", "/tmp/docs", "docbuilder-mcp", &stdout, nil); err != nil {
		t.Fatalf("runSetup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote file; stat err = %v", err)
	}
	if !strings.Contains(stdout.String(), "Would merge") {
		t.Errorf("stdout missing 'Would merge'; got: %s", stdout.String())
	}
}

func TestRunSetup_WritesOpenCodeConfig(t *testing.T) {
	dir, cleanup := withTempCwd(t)
	defer cleanup()

	var stdout strings.Builder
	if err := runSetup([]string{"opencode"}, false, false, "/tmp/x.yaml", "/tmp/docs", "docbuilder-mcp", &stdout, nil); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "opencode.json")) // #nosec G304 -- test-controlled path
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	var got struct {
		MCP map[string]struct {
			Type    string   `json:"type"`
			Command []string `json:"command"`
			Enabled bool     `json:"enabled"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	e, ok := got.MCP["docbuilder"]
	if !ok {
		t.Fatalf("docbuilder entry missing; got %v", got)
	}
	if e.Type != "local" {
		t.Errorf("type = %q, want local", e.Type)
	}
	if !e.Enabled {
		t.Errorf("enabled = false, want true")
	}
	if len(e.Command) != 5 || e.Command[0] != "docbuilder-mcp" {
		t.Errorf("command = %v; want [docbuilder-mcp --config /tmp/x.yaml --docs-dir /tmp/docs]", e.Command)
	}
}

func TestRunSetup_WritesVSCodeConfig(t *testing.T) {
	dir, cleanup := withTempCwd(t)
	defer cleanup()

	var stdout strings.Builder
	if err := runSetup([]string{"vscode"}, false, false, "/tmp/x.yaml", "/tmp/docs", "docbuilder-mcp", &stdout, nil); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".vscode", "mcp.json")) // #nosec G304 -- test-controlled path
	if err != nil {
		t.Fatalf("read .vscode/mcp.json: %v", err)
	}
	var got struct {
		Servers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	e, ok := got.Servers["docbuilder"]
	if !ok {
		t.Fatalf("docbuilder entry missing; got %v", got)
	}
	if e.Type != "stdio" {
		t.Errorf("type = %q, want stdio", e.Type)
	}
	if e.Command != "docbuilder-mcp" {
		t.Errorf("command = %q, want docbuilder-mcp", e.Command)
	}
	if len(e.Args) != 4 || e.Args[0] != "--config" {
		t.Errorf("args = %v; want [--config /tmp/x.yaml --docs-dir /tmp/docs]", e.Args)
	}
}

func TestRunSetup_GlobalFlag(t *testing.T) {
	_, cleanup := withTempCwd(t)
	defer cleanup()

	// Use a fake HOME so global paths point somewhere predictable and don't
	// touch the real filesystem.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var stdout strings.Builder
	if err := runSetup([]string{"opencode"}, true /* global */, false, "/tmp/x.yaml", "/tmp/docs", "docbuilder-mcp", &stdout, nil); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	// Project-level file should NOT have been written.
	// (cwd is the temp dir; we use it as the project root)
	cwd, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(cwd, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("project-level file written under --global; stat err = %v", err)
	}

	// Global file should exist under the fake HOME.
	expectedDir := filepath.Join(tmpHome, ".config", "opencode")
	wantPath := filepath.Join(expectedDir, "opencode.json")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("global file %s not written: %v", wantPath, err)
	}
}

func TestRunSetup_IdempotentOverwrites(t *testing.T) {
	dir, cleanup := withTempCwd(t)
	defer cleanup()

	var stdout strings.Builder
	for range 2 {
		if err := runSetup([]string{"opencode"}, false, false, "/tmp/x.yaml", "/tmp/docs", "docbuilder-mcp", &stdout, nil); err != nil {
			t.Fatalf("runSetup: %v", err)
		}
	}
	// The single opencode.json should still exist and be valid.
	data, err := os.ReadFile(filepath.Join(dir, "opencode.json")) // #nosec G304 -- test-controlled path
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestPathCmd_OpenCodeProject(t *testing.T) {
	_, cleanup := withTempCwd(t)
	defer cleanup()

	cmd := &PathCmd{Tool: "opencode"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestPickTarget_GlobalPrefersUserLevel(t *testing.T) {
	t.Parallel()
	got := pickTarget(opencodeTool{}, true)
	if !filepath.IsAbs(got.Path) {
		t.Errorf("global target path %q is not absolute", got.Path)
	}
}

func TestPickTarget_ProjectFallsBackToGlobal(t *testing.T) {
	// A tool that doesn't implement project-level should fall back to global.
	// All our current tools do implement both, so simulate by using a stub.
	stub := stubTool{projectPath: ""}
	got := pickTarget(stub, false)
	if got.ServerKey != stub.GlobalTarget().ServerKey {
		t.Errorf("expected fallback to global; got %+v", got)
	}
}

// stubTool is a toolSpec for testing pickTarget's fallback logic.
type stubTool struct {
	projectPath string
}

func (s stubTool) ID() string            { return "stub" }
func (s stubTool) DisplayName() string   { return "Stub" }
func (s stubTool) DetectInstalled() bool { return false }
func (s stubTool) ProjectTarget() configTarget {
	return configTarget{Path: s.projectPath, ServerKey: "stub"}
}

func (s stubTool) GlobalTarget() configTarget {
	return configTarget{Path: "/tmp/global.json", ServerKey: "stub"}
}

func (s stubTool) RenderServerEntry(serverSpec) ([]byte, error) {
	return json.Marshal(map[string]any{"stub": true})
}
