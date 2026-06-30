package runtime

import (
	"net/http"
	"sync"
)

// hub is a minimal in-memory LiveReloadHub implementation. It satisfies
// the queue.LiveReloadHub interface (http.Handler + Broadcast + Shutdown)
// without pulling in daemon.LiveReloadHub (which carries a metrics
// collector and is wired for the production daemon's telemetry path).
type hub struct {
	mu     sync.RWMutex
	closed bool
}

func newHub() *hub { return &hub{} }

// ServeHTTP is a no-op SSE endpoint. Preview's local HTTP server
// streams the actual SSE events; this hub exists only so callers
// (and tests) can Broadcast without wiring up the full daemon.
func (h *hub) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	closed := h.closed
	h.mu.RUnlock()
	if closed {
		http.Error(w, "preview livereload shutting down", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = w.Write([]byte(": ok\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// Broadcast is a no-op. The preview HTTP server reads SSE updates
// from a separate channel; the hub here exists only to satisfy the
// queue.LiveReloadHub interface so callers can be wired up.
func (h *hub) Broadcast(_ string) {}

// Shutdown marks the hub as closed so further ServeHTTP returns 503.
func (h *hub) Shutdown() {
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
}
