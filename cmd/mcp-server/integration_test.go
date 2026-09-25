package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestIntegration_InitializeAndToolsList is a smoke test that the binary
// actually speaks JSON-RPC over stdio. It:
//
//  1. Builds the docbuilder-mcp binary into a temp dir.
//  2. Spawns it with stdin/stdout piped.
//  3. Sends an `initialize` request, waits for the response.
//  4. Sends `initialized` notification, then `tools/list`.
//  5. Asserts the server returned our 10 tools.
//
// We intentionally keep this test offline (no template base URL, no git
// activity) — the goal is protocol correctness, not functional coverage.
func TestIntegration_InitializeAndToolsList(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "docbuilder-mcp")

	// Build the binary from this package.
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}

	// Spawn it.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath,
		"--config", filepath.Join(binDir, "nonexistent.yaml"), // missing config is fine
		"--docs-dir", t.TempDir(),
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	client := newMCPTestClient(stdin, stdout)

	// initialize
	initResp, err := client.request(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "0.0.0",
		},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("initialize returned error: %+v", initResp.Error)
	}
	if got := initResp.Result["serverInfo"].(map[string]any)["name"]; got != "docbuilder-mcp" {
		t.Errorf("serverInfo.name: got %v want docbuilder-mcp", got)
	}

	// The server should advertise instructions telling the LLM how to use
	// it. Verify a few signature phrases — the full text is asserted in a
	// separate test against the const directly.
	instr, _ := initResp.Result["instructions"].(string)
	for _, want := range []string{
		"--docs-dir",
		"create_from_template",
		"confirm: true",
	} {
		if !strings.Contains(instr, want) {
			t.Errorf("instructions missing %q; got: %s", want, instr)
		}
	}

	// initialized notification (no response expected)
	if err := client.notify("notifications/initialized", map[string]any{}); err != nil {
		t.Fatalf("initialized notification: %v", err)
	}

	// tools/list
	listResp, err := client.request(ctx, "tools/list", map[string]any{})
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	if listResp.Error != nil {
		t.Fatalf("tools/list returned error: %+v", listResp.Error)
	}

	tools, ok := listResp.Result["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list result missing 'tools' array: %v", listResp.Result)
	}

	want := []string{
		"get_config",
		"list_templates",
		"describe_template",
		"resolve_template_inputs",
		"lint_docs",
		"read_doc",
		"create_from_template",
		"lint_fix",
		"create_doc",
		"update_doc",
	}
	got := map[string]bool{}
	for _, tool := range tools {
		name := tool.(map[string]any)["name"].(string)
		got[name] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("tools/list missing %q", w)
		}
	}
}

// TestIntegration_GetConfigTool sanity-checks a real tool invocation:
// get_config should return redacted JSON, even with no config file.
func TestIntegration_GetConfigTool(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "docbuilder-mcp")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath,
		"--config", filepath.Join(binDir, "nonexistent.yaml"),
		"--docs-dir", t.TempDir(),
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	client := newMCPTestClient(stdin, stdout)

	if _, err := client.request(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0"},
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := client.notify("notifications/initialized", map[string]any{}); err != nil {
		t.Fatalf("initialized: %v", err)
	}

	resp, err := client.request(ctx, "tools/call", map[string]any{
		"name":      "get_config",
		"arguments": map[string]any{},
	})
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/call error: %+v", resp.Error)
	}

	// Result.content is an array of content blocks; we asked for text.
	content, ok := resp.Result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected content array, got %v", resp.Result)
	}
	block := content[0].(map[string]any)
	text, _ := block["text"].(string)
	if !strings.Contains(text, "Version") {
		t.Errorf("get_config text missing 'Version' field: %s", text)
	}
}

// ----- minimal JSON-RPC test client ---------------------------------------

type mcpTestClient struct {
	w          *bufio.Writer
	r          *bufio.Reader
	nextID     int
	pending    map[int]chan mcpResponse
	pendingMu  sync.Mutex
	readerOnce sync.Once
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  map[string]any  `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func newMCPTestClient(w io.Writer, r io.Reader) *mcpTestClient {
	return &mcpTestClient{
		w:       bufio.NewWriter(w),
		r:       bufio.NewReader(r),
		pending: map[int]chan mcpResponse{},
	}
}

// request sends a JSON-RPC request and waits for the matching response.
// The MCP server writes one JSON object per line on stdout, so we read by
// newline-terminated JSON.
func (c *mcpTestClient) request(ctx context.Context, method string, params any) (*mcpResponse, error) {
	id := c.nextID
	c.nextID++

	// Start a reader goroutine if this is the first call.
	//
	// We keep one goroutine alive for the duration of the test that loops
	// over stdout and dispatches to pending channels by ID.
	c.startReaderOnce()

	ch := make(chan mcpResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	if _, err := c.w.Write(body); err != nil {
		return nil, err
	}
	if err := c.w.WriteByte('\n'); err != nil {
		return nil, err
	}
	if err := c.w.Flush(); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		return &resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// notify sends a JSON-RPC notification (no id, no response expected).
func (c *mcpTestClient) notify(method string, params any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	if _, err := c.w.Write(body); err != nil {
		return err
	}
	if err := c.w.WriteByte('\n'); err != nil {
		return err
	}
	return c.w.Flush()
}

// startReaderOnce launches the single goroutine that demuxes responses
// from stdout by JSON-RPC id. Subsequent calls are no-ops.
func (c *mcpTestClient) startReaderOnce() {
	c.readerOnce.Do(func() {
		go c.readLoop()
	})
}

// readLoop is the demuxer goroutine. It reads newline-terminated JSON
// messages from stdout, looks up the pending request by id, and delivers
// the response to the caller's channel.
func (c *mcpTestClient) readLoop() {
	for {
		line, err := c.r.ReadBytes('\n')
		if err != nil {
			c.pendingMu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = map[int]chan mcpResponse{}
			c.pendingMu.Unlock()
			return
		}
		var resp mcpResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "test: bad json from server: %s\n", line)
			continue
		}
		var id int
		if err := json.Unmarshal(resp.ID, &id); err != nil {
			continue // notification or other non-response; ignore
		}
		c.pendingMu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
		if ok {
			ch <- resp
			close(ch)
		}
	}
}
