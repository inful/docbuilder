package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

func (s *HTTPServer) startLiveReloadServerWithListener(_ context.Context, ln net.Listener) error {
	mux := http.NewServeMux()

	// CORS middleware for LiveReload server (allows cross-origin requests from docs port)
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	// LiveReload SSE endpoint
	if s.daemon != nil && s.daemon.liveReload != nil {
		mux.Handle("/livereload", corsMiddleware(s.daemon.liveReload))
		mux.HandleFunc("/livereload.js", func(w http.ResponseWriter, _ *http.Request) {
			// Add CORS headers for script loading
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			// Generate script that connects to this dedicated port
			script := fmt.Sprintf(`(() => {
  if (window.__DOCBUILDER_LR__) return;
  window.__DOCBUILDER_LR__=true;
  function connect(){
    const es = new EventSource('http://localhost:%d/livereload');
    let first=true; let current=null;
    es.onmessage = (e)=>{ try { const p=JSON.parse(e.data); if(first){ current=p.hash; first=false; return;} if(p.hash && p.hash!==current){ console.log('[docbuilder] change detected, reloading'); document.cookie='docbuilder_lr_reload=1; path=/; max-age=5'; location.reload(); } } catch(_){} };
    es.onerror = ()=>{ console.warn('[docbuilder] livereload error - retrying'); es.close(); setTimeout(connect,2000); };
  }
  connect();
})();`, s.config.Daemon.HTTP.LiveReloadPort)
			if _, err := w.Write([]byte(script)); err != nil {
				slog.Error("failed to write livereload script", "error", err)
			}
		})
		slog.Info("LiveReload dedicated server registered")
	}

	// LiveReload server needs no timeouts for long-lived SSE connections
	s.liveReloadServer = &http.Server{Handler: mux, ReadTimeout: 0, WriteTimeout: 0, IdleTimeout: 300 * time.Second}
	return s.startServerWithListener("livereload", s.liveReloadServer, ln)
}

// injectLiveReloadScriptWithPort is a middleware that injects the LiveReload client script
// into HTML responses, configured to connect to the specified port.
func (s *HTTPServer) injectLiveReloadScriptWithPort(next http.Handler, port int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only inject into HTML pages (not assets, API endpoints, etc.)
		path := r.URL.Path
		isHTMLPage := path == "/" || path == "" || strings.HasSuffix(path, "/") || strings.HasSuffix(path, ".html")

		if !isHTMLPage {
			// Not an HTML page, serve normally
			next.ServeHTTP(w, r)
			return
		}

		injector := newLiveReloadInjectorWithPort(w, r, port)
		next.ServeHTTP(injector, r)
		injector.finalize()
	})
}

// liveReloadInjector wraps an http.ResponseWriter to inject the LiveReload client script
// into HTML responses before </body> tag. Uses buffering with a size limit to prevent stalls.
type liveReloadInjector struct {
	http.ResponseWriter
	statusCode    int
	buffer        []byte
	headerWritten bool
	passthrough   bool
	maxSize       int
	port          int
}

func newLiveReloadInjectorWithPort(w http.ResponseWriter, _ *http.Request, port int) *liveReloadInjector {
	return &liveReloadInjector{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		maxSize:        512 * 1024, // 512KB max - typical HTML page
		port:           port,
	}
}

func (l *liveReloadInjector) WriteHeader(code int) {
	l.statusCode = code
	// Don't write header yet unless in passthrough mode
	if l.passthrough {
		l.ResponseWriter.WriteHeader(code)
		l.headerWritten = true
	}
}

func (l *liveReloadInjector) Write(data []byte) (int, error) {
	// Check Content-Type on first write
	if !l.headerWritten && !l.passthrough && l.buffer == nil {
		contentType := l.ResponseWriter.Header().Get("Content-Type")
		isHTML := contentType == "" || strings.Contains(contentType, "text/html")

		if !isHTML {
			// Not HTML - passthrough
			l.passthrough = true
			l.ResponseWriter.WriteHeader(l.statusCode)
			l.headerWritten = true
			return l.ResponseWriter.Write(data)
		}

		l.buffer = make([]byte, 0, 64*1024) // Start with 64KB
	}

	if l.passthrough {
		return l.ResponseWriter.Write(data)
	}

	// Check if buffering would exceed limit
	if len(l.buffer)+len(data) > l.maxSize {
		// Too large - switch to passthrough, flush buffer, write remaining
		l.passthrough = true
		l.ResponseWriter.Header().Del("Content-Length")
		l.ResponseWriter.WriteHeader(l.statusCode)
		l.headerWritten = true

		if len(l.buffer) > 0 {
			if _, err := l.ResponseWriter.Write(l.buffer); err != nil {
				return 0, err
			}
		}
		return l.ResponseWriter.Write(data)
	}

	// Buffer the data
	l.buffer = append(l.buffer, data...)
	return len(data), nil
}

// finalize must be called after the handler completes to inject the script.
func (l *liveReloadInjector) finalize() {
	if l.passthrough || len(l.buffer) == 0 {
		if !l.headerWritten {
			l.ResponseWriter.WriteHeader(l.statusCode)
		}
		return
	}

	// Inject script before </body>
	html := string(l.buffer)
	script := fmt.Sprintf(`<script async src="http://localhost:%d/livereload.js"></script></body>`, l.port)
	modified := strings.Replace(html, "</body>", script, 1)

	l.ResponseWriter.Header().Del("Content-Length")
	l.ResponseWriter.WriteHeader(l.statusCode)
	_, _ = l.ResponseWriter.Write([]byte(modified))
}
