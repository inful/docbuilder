package daemon

import (
	"bufio"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// LiveReloadHub manages SSE clients for hash-change broadcasts.
type LiveReloadHub struct {
	mu       sync.RWMutex
	nextID   int
	clients  map[int]*lrClient
	metrics  *MetricsCollector
	closed   bool
	lastHash string
}

type lrClient struct {
	id   int
	ch   chan string
	done chan struct{}
}

func NewLiveReloadHub(mc *MetricsCollector) *LiveReloadHub {
	return &LiveReloadHub{clients: map[int]*lrClient{}, metrics: mc}
}

// ServeHTTP implements the SSE endpoint at /livereload
func (h *LiveReloadHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	closed := h.closed
	h.mu.RUnlock()
	if closed {
		http.Error(w, "livereload shutting down", http.StatusServiceUnavailable)
		return
	}
	// Prepare SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	// Register client
	client := &lrClient{ch: make(chan string, 8), done: make(chan struct{})}
	h.mu.Lock()
	client.id = h.nextID
	h.nextID++
	h.clients[client.id] = client
	current := h.lastHash
	h.mu.Unlock()
	if h.metrics != nil {
		h.metrics.IncrementCounter("livereload_connections_total")
	}
	if h.metrics != nil {
		h.metrics.SetGauge("livereload_clients", int64(len(h.clients)))
	}

	// Initial comment / optional last hash event
	bw := bufio.NewWriter(w)
	if _, err := bw.WriteString(": connected\n\n"); err != nil {
		slog.Debug("livereload write", "error", err)
		return
	}
	if current != "" {
		if _, err := bw.WriteString("data: {\"hash\":\"" + current + "\"}\n\n"); err != nil {
			slog.Debug("livereload write", "error", err)
			return
		}
	}
	if err := bw.Flush(); err == nil {
		flusher.Flush()
	}

	// Heartbeat ticker
	hb := time.NewTicker(30 * time.Second)
	defer hb.Stop()

	// Use request context for disconnect notification
	ctx := r.Context()
	notify := make(chan bool, 1)
	go func() { <-ctx.Done(); notify <- true }()

	for {
		select {
		case <-notify:
			h.removeClient(client.id)
			return
		case <-client.done:
			h.removeClient(client.id)
			return
		case <-hb.C:
			if _, err := bw.WriteString(": ping\n\n"); err == nil {
				bw.Flush()
				flusher.Flush()
			} else {
				slog.Debug("livereload ping write", "error", err)
			}
		case hash := <-client.ch:
			if _, err := bw.WriteString("data: {\"hash\":\"" + hash + "\"}\n\n"); err == nil {
				bw.Flush()
				flusher.Flush()
			} else {
				slog.Debug("livereload broadcast write", "error", err)
			}
		}
	}
}

func (h *LiveReloadHub) removeClient(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.clients[id]; ok {
		delete(h.clients, id)
		close(c.done)
		if h.metrics != nil {
			h.metrics.IncrementCounter("livereload_disconnections_total")
		}
		if h.metrics != nil {
			h.metrics.SetGauge("livereload_clients", int64(len(h.clients)))
		}
	}
}

// Broadcast new hash to all clients (drops clients whose channels are full / closed).
func (h *LiveReloadHub) Broadcast(hash string) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	if hash == "" || hash == h.lastHash {
		h.mu.Unlock()
		return
	}
	h.lastHash = hash
	snapshot := make([]*lrClient, 0, len(h.clients))
	for _, c := range h.clients {
		snapshot = append(snapshot, c)
	}
	h.mu.Unlock()
	dropped := 0
	for _, c := range snapshot {
		select {
		case c.ch <- hash:
		default:
			dropped++
			h.removeClient(c.id)
		}
	}
	if h.metrics != nil {
		h.metrics.IncrementCounter("livereload_broadcasts_total")
	}
	if dropped > 0 && h.metrics != nil {
		for i := 0; i < dropped; i++ {
			h.metrics.IncrementCounter("livereload_dropped_clients_total")
		}
	}
	slog.Debug("livereload broadcast", "hash", hash, "clients", len(snapshot), "dropped", dropped)
}

// Shutdown closes all clients and prevents future broadcasts.
func (h *LiveReloadHub) Shutdown() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	clients := h.clients
	h.clients = map[int]*lrClient{}
	h.mu.Unlock()
	for _, c := range clients {
		close(c.done)
	}
	if h.metrics != nil {
		h.metrics.SetGauge("livereload_clients", 0)
	}
}

// ScriptContent returns the JS snippet clients can include.
const LiveReloadScript = `(() => {\n  if (window.__DOCBUILDER_LR__) return;\n  window.__DOCBUILDER_LR__=true;\n  function connect(){\n    const es = new EventSource('/livereload');\n    let first=true; let current=null;\n    es.onmessage = (e)=>{ try { const p=JSON.parse(e.data); if(first){ current=p.hash; first=false; return;} if(p.hash && p.hash!==current){ console.log('[docbuilder] change detected, reloading'); location.reload(); } } catch(_){} };\n    es.onerror = ()=>{ console.warn('[docbuilder] livereload error - retrying'); es.close(); setTimeout(connect,2000); };\n  }\n  connect();\n})();`
