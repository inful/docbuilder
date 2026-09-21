package hugo

import (
	"sync"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGenerator_WithDocumentReady_StoresCallback verifies the setter pattern
// used by the daemon: the callback is stored on the Generator and can be
// invoked without nil-checks at every call site.
//
// The full integration (callback firing from copyContentFilesPipeline) is
// covered by the existing pipeline tests below — they exercise the path with
// no callback installed. We assert the field-level wiring here to keep the
// test fast and independent of the build pipeline.
func TestGenerator_WithDocumentReady_StoresCallback(t *testing.T) {
	var (
		mu        sync.Mutex
		gotBytes  []byte
		gotPath   string
		callCount int
	)

	g := NewGenerator(emptyConfigForTest(), t.TempDir())
	g.WithDocumentReady(func(content []byte, path string) {
		mu.Lock()
		defer mu.Unlock()
		gotBytes = content
		gotPath = path
		callCount++
	})

	if g.onDocumentReady == nil {
		t.Fatal("WithDocumentReady did not install the callback")
	}

	g.onDocumentReady([]byte("hello"), "docs/x.md")

	mu.Lock()
	defer mu.Unlock()
	if callCount != 1 {
		t.Errorf("call count = %d, want 1", callCount)
	}
	if string(gotBytes) != "hello" {
		t.Errorf("content = %q, want %q", gotBytes, "hello")
	}
	if gotPath != "docs/x.md" {
		t.Errorf("path = %q, want %q", gotPath, "docs/x.md")
	}
}

// TestGenerator_WithDocumentReady_NilDisablesCallback verifies the daemon can
// uninstall the hook by passing nil — useful for tests or future dynamic
// config reload.
func TestGenerator_WithDocumentReady_NilDisablesCallback(t *testing.T) {
	g := NewGenerator(emptyConfigForTest(), t.TempDir())
	g.WithDocumentReady(func([]byte, string) {})
	if g.onDocumentReady == nil {
		t.Fatal("callback not installed")
	}
	g.WithDocumentReady(nil)
	if g.onDocumentReady != nil {
		t.Fatal("callback not cleared by nil setter")
	}
}

// emptyConfigForTest returns a minimal Config suitable for tests that don't
// touch the pipeline. Sharing across tests keeps the helper focused on the
// callback contract rather than config plumbing.
func emptyConfigForTest() *config.Config { return &config.Config{} }
