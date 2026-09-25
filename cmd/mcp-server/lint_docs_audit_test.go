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
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAudit_LintDocs runs the docbuilder-mcp binary as a subprocess and
// invokes the `lint_docs` tool over MCP against this repo's `docs/`
// directory. The goal is to surface any lint findings the just-shipped
// doc fixes may have introduced or left behind.
//
// This is intentionally a separate test from the canonical integration
// suite: it depends on the repo layout (it lints ../docs from the test's
// working directory), so it is gated behind a build tag if you ever want
// to skip it in CI.
//
// Run with: go test ./cmd/mcp-server/ -run TestAudit_LintDocs -v
func TestAudit_LintDocs(t *testing.T) {
	if testing.Short() {
		t.Skip("audit test skipped in short mode")
	}

	// Build the binary into a temp dir.
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "docbuilder-mcp")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Spawn with --docs-dir pointing at the repo's docs/ folder.
	repoRoot, err := repoRootFromHere()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	docsDir := filepath.Join(repoRoot, "docs")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath,
		"--config", filepath.Join(repoRoot, "config.yaml"),
		"--docs-dir", docsDir,
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

	client := newAuditClient(stdin, stdout)

	// initialize
	if _, err := client.request(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "audit-test", "version": "0"},
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := client.notify("notifications/initialized", map[string]any{}); err != nil {
		t.Fatalf("initialized: %v", err)
	}

	// lint_docs over the entire docs/ tree.
	resp, err := client.request(ctx, "tools/call", map[string]any{
		"name": "lint_docs",
		"arguments": map[string]any{
			"path": docsDir,
		},
	})
	if err != nil {
		t.Fatalf("tools/call lint_docs: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("lint_docs error: %+v", resp.Error)
	}

	// Extract the text payload from the response.
	content, ok := resp.Result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content in lint_docs response: %v", resp.Result)
	}
	text := content[0].(map[string]any)["text"].(string)

	// Parse the structured result back out.
	var parsed struct {
		FilesTotal   int `json:"files_total"`
		ErrorCount   int `json:"error_count"`
		WarningCount int `json:"warning_count"`
		Issues       []struct {
			File        string `json:"file"`
			Line        int    `json:"line"`
			Severity    string `json:"severity"`
			Rule        string `json:"rule"`
			Message     string `json:"message"`
			Explanation string `json:"explanation,omitempty"`
		} `json:"issues"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("unmarshal lint payload: %v\npayload:\n%s", err, text)
	}

	// Print a human-readable summary so the test log is useful even without -v.
	t.Logf("lint_docs summary: %d files, %d errors, %d warnings",
		parsed.FilesTotal, parsed.ErrorCount, parsed.WarningCount)

	if len(parsed.Issues) == 0 {
		t.Log("lint_docs: no findings")
		return
	}

	// Group by rule for easier scanning.
	byRule := map[string][]string{}
	for _, iss := range parsed.Issues {
		key := fmt.Sprintf("%s:%s", iss.Rule, iss.Severity)
		rel, _ := filepath.Rel(repoRoot, iss.File)
		loc := rel
		if iss.Line > 0 {
			loc = fmt.Sprintf("%s:%d", rel, iss.Line)
		}
		byRule[key] = append(byRule[key], fmt.Sprintf("  %s — %s", loc, iss.Message))
	}

	// Sort rule keys for deterministic output.
	keys := make([]string, 0, len(byRule))
	for k := range byRule {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		t.Logf("\n[%s] %d occurrence(s):", k, len(byRule[k]))
		for _, line := range byRule[k] {
			t.Log(line)
		}
	}

	// Second pass: ask the MCP server to fix the fingerprints via lint_fix
	// (dry_run: false), then re-lint and confirm the error count dropped.
	if parsed.ErrorCount > 0 {
		t.Log("\n--- applying lint_fix via MCP ---")
		fixResp, err := client.request(ctx, "tools/call", map[string]any{
			"name": "lint_fix",
			"arguments": map[string]any{
				"path":    docsDir,
				"confirm": true,
			},
		})
		if err != nil {
			t.Fatalf("lint_fix: %v", err)
		}
		if fixResp.Error != nil {
			t.Fatalf("lint_fix error: %+v", fixResp.Error)
		}
		t.Logf("lint_fix result: %v", fixResp.Result["content"])

		// Re-lint.
		reResp, err := client.request(ctx, "tools/call", map[string]any{
			"name": "lint_docs",
			"arguments": map[string]any{"path": docsDir},
		})
		if err != nil {
			t.Fatalf("re-lint_docs: %v", err)
		}
		if reResp.Error != nil {
			t.Fatalf("re-lint_docs error: %+v", reResp.Error)
		}
		reText := reResp.Result["content"].([]any)[0].(map[string]any)["text"].(string)
		var reParsed struct {
			FilesTotal   int `json:"files_total"`
			ErrorCount   int `json:"error_count"`
			WarningCount int `json:"warning_count"`
		}
		if err := json.Unmarshal([]byte(reText), &reParsed); err != nil {
			t.Fatalf("re-unmarshal: %v\n%s", err, reText)
		}
		t.Logf("after fix: %d files, %d errors, %d warnings",
			reParsed.FilesTotal, reParsed.ErrorCount, reParsed.WarningCount)
		if reParsed.ErrorCount > 0 {
			// lint_fix didn't take us to zero. Fail loudly: leaving
			// errors in the tree after a fix means a doc audit
			// round-trip is broken, which is exactly what this test
			// exists to catch.
			t.Fatalf("lint_fix left %d error(s) in the docs tree; original error count was %d — fix is incomplete",
				reParsed.ErrorCount, parsed.ErrorCount)
		}
	}
}

// repoRootFromHere walks up from the test's working directory looking for
// go.mod. We can't rely on runtime.Caller because `go test` rewrites the
// working directory; the simpler heuristic is fine for a one-off audit.
func repoRootFromHere() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found above %s", wd)
}

// ---- minimal JSON-RPC client (mirror of integration_test.go's; kept
// ---- independent so the canonical suite stays untouched).

type auditClient struct {
	w          *bufio.Writer
	r          *bufio.Reader
	nextID     int
	pending    map[int]chan auditResp
	pendingMu  sync.Mutex
	readerOnce sync.Once
}

type auditResp struct {
	ID      json.RawMessage `json:"id,omitempty"`
	Result  map[string]any  `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func newAuditClient(w interface{ Write([]byte) (int, error) }, r interface{ Read([]byte) (int, error) }) *auditClient {
	return &auditClient{
		w:       bufio.NewWriter(struct{ io.Writer }{w.(io.Writer)}),
		r:       bufio.NewReader(struct{ io.Reader }{r.(io.Reader)}),
		pending: map[int]chan auditResp{},
	}
}

func (c *auditClient) request(ctx context.Context, method string, params any) (*auditResp, error) {
	id := c.nextID
	c.nextID++
	c.readerOnce.Do(func() { go c.readLoop() })

	ch := make(chan auditResp, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	})
	if err != nil {
		return nil, err
	}
	if _, err := c.w.Write(append(body, '\n')); err != nil {
		return nil, err
	}
	if err := c.w.Flush(); err != nil {
		return nil, err
	}
	select {
	case r := <-ch:
		return &r, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *auditClient) notify(method string, params any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "method": method, "params": params,
	})
	if err != nil {
		return err
	}
	if _, err := c.w.Write(append(body, '\n')); err != nil {
		return err
	}
	return c.w.Flush()
}

func (c *auditClient) readLoop() {
	for {
		line, err := c.r.ReadBytes('\n')
		if err != nil {
			c.pendingMu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = map[int]chan auditResp{}
			c.pendingMu.Unlock()
			return
		}
		var r auditResp
		if err := json.Unmarshal(line, &r); err != nil {
			fmt.Fprintf(os.Stderr, "audit: bad json from server: %s\n", strings.TrimSpace(string(line)))
			continue
		}
		var id int
		if err := json.Unmarshal(r.ID, &id); err != nil {
			continue
		}
		c.pendingMu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
		if ok {
			ch <- r
			close(ch)
		}
	}
}
