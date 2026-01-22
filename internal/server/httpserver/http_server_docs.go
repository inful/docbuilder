package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// parseHugoError extracts useful error information from Hugo build output.
// Hugo errors typically contain paths like: "/tmp/.../content/local/file.md:line:col": error message
// This function extracts: file.md:line:col: error message.
func parseHugoError(errStr string) string {
	// Pattern 1: Match Hugo error format in output:
	// Error: error building site: process: readAndProcessContent: "/path/to/content/file.md:123:45": error message
	re1 := regexp.MustCompile(`Error:.*?[":]\s*"([^"]+\.md):(\d+):(\d+)":\s*(.+?)(?:\n|$)`)

	matches := re1.FindStringSubmatch(errStr)
	if len(matches) >= 5 {
		// Extract just the filename without full path
		filePath := matches[1]
		// Remove temporary directory prefix if present
		if idx := strings.Index(filePath, "/content/"); idx >= 0 {
			filePath = filePath[idx+9:] // Skip "/content/"
		}
		line := matches[2]
		col := matches[3]
		message := strings.TrimSpace(matches[4])
		return fmt.Sprintf("%s:%s:%s: %s", filePath, line, col, message)
	}

	// Pattern 2: Legacy format from previous implementation
	// "/path/to/content/local/relative/path.md:123:45": error message
	re2 := regexp.MustCompile(`/content/local/([^"]+):(\d+):(\d+)[^"]*":\s*(.+)$`)

	matches = re2.FindStringSubmatch(errStr)
	if len(matches) >= 5 {
		filePath := matches[1]
		line := matches[2]
		col := matches[3]
		message := strings.TrimSpace(matches[4])
		return fmt.Sprintf("%s:%s:%s: %s", filePath, line, col, message)
	}

	// If no pattern matches, return original error
	return errStr
}

// resolveAbsoluteOutputDir resolves the output directory to an absolute path.
func (s *Server) resolveAbsoluteOutputDir() string {
	out := s.cfg.Output.Directory
	if out == "" {
		out = defaultSiteDir
	}
	if !filepath.IsAbs(out) {
		if abs, err := filepath.Abs(out); err == nil {
			return abs
		}
	}
	return out
}

// shouldShowStatusPage checks if we should show a status page instead of serving files.
func (s *Server) shouldShowStatusPage(root string) bool {
	out := s.resolveAbsoluteOutputDir()
	if root != out {
		return false
	}

	_, err := os.Stat(filepath.Join(out, "public"))
	return os.IsNotExist(err)
}

// handleStatusPage determines which status page to show and renders it.
func (s *Server) handleStatusPage(w http.ResponseWriter, r *http.Request, root string) {
	// Check if there's a build error
	if s.opts.BuildStatus != nil {
		if hasError, buildErr, hasGoodBuild := s.opts.BuildStatus.GetStatus(); hasError && !hasGoodBuild {
			// Build failed - show error page
			s.renderBuildErrorPage(w, buildErr)
			return
		}
	}

	// Show pending page for root path only
	if r.URL.Path == "/" || r.URL.Path == "" {
		s.renderBuildPendingPage(w)
		return
	}

	// For non-root paths, fall through to file server (will likely 404)
	http.FileServer(http.Dir(root)).ServeHTTP(w, r)
}

// renderBuildErrorPage renders an error page when build fails.
func (s *Server) renderBuildErrorPage(w http.ResponseWriter, buildErr error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)

	errorMsg := "Unknown error"
	if buildErr != nil {
		errorMsg = parseHugoError(buildErr.Error())
	}

	scriptTag := s.getLiveReloadScript()

	_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Build Failed</title><style>body{font-family:sans-serif;max-width:800px;margin:50px auto;padding:20px}h1{color:#d32f2f}pre{background:#f5f5f5;padding:15px;border-radius:4px;overflow-x:auto}</style></head><body><h1>⚠️ Build Failed</h1><p>The documentation site failed to build. Fix the error below and save to rebuild automatically.</p><h2>Error Details:</h2><pre>%s</pre><p><small>This page will refresh automatically when you fix the error.</small></p>%s</body></html>`, errorMsg, scriptTag)
}

// renderBuildPendingPage renders a page shown while build is in progress.
func (s *Server) renderBuildPendingPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)

	scriptTag := s.getLiveReloadScript()

	_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Site rendering</title></head><body><h1>Documentation is being prepared</h1><p>The site hasn't been rendered yet. This page will be replaced automatically once rendering completes.</p>%s</body></html>`, scriptTag)
}

// getLiveReloadScript returns the livereload script tag if enabled, empty string otherwise.
func (s *Server) getLiveReloadScript() string {
	if !s.cfg.Build.LiveReload {
		return ""
	}
	if s.opts.LiveReloadHub == nil {
		return ""
	}
	return fmt.Sprintf(`<script src="http://localhost:%d/livereload.js"></script>`, s.cfg.Daemon.HTTP.LiveReloadPort)
}

// startDocsServerWithListener allows injecting a pre-bound listener (for coordinated bind checks).
func (s *Server) startDocsServerWithListener(_ context.Context, ln net.Listener) error {
	mux := http.NewServeMux()
	// Health/readiness endpoints on docs port as well for compatibility with common probe configs
	mux.HandleFunc("/health", s.monitoringHandlers.HandleHealthCheck)
	mux.HandleFunc("/healthz", s.monitoringHandlers.HandleHealthCheck) // Kubernetes-style alias
	mux.HandleFunc("/ready", s.handleReadiness)
	mux.HandleFunc("/readyz", s.handleReadiness) // Kubernetes-style alias

	// VS Code edit link handler for local preview mode
	mux.HandleFunc("/_edit/", s.handleVSCodeEdit)

	// Root handler dynamically chooses between the Hugo output directory and the rendered "public" folder.
	// This lets us begin serving immediately (before a static render completes) while automatically
	// switching to the fully rendered site once available—without restarting the daemon.
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root := s.resolveDocsRoot()

		// Check if we need to show a status page instead of serving files
		if s.shouldShowStatusPage(root) {
			s.handleStatusPage(w, r, root)
			return
		}

		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})

	// Wrap with 404 fallback that redirects to nearest parent path on LiveReload
	rootWithFallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the response to detect 404s
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		rootHandler.ServeHTTP(rec, r)

		// If we got a 404 and this is a GET request from LiveReload, try to redirect to parent
		if rec.statusCode == http.StatusNotFound && r.Method == http.MethodGet {
			// Check if this is a LiveReload-triggered reload via Cookie
			if cookie, err := r.Cookie("docbuilder_lr_reload"); err == nil && cookie.Value == "1" {
				root := s.resolveDocsRoot()
				redirectPath := s.findNearestValidParent(root, r.URL.Path)
				if redirectPath != "" && redirectPath != r.URL.Path {
					// Clear the cookie and redirect
					http.SetCookie(w, &http.Cookie{
						Name:   "docbuilder_lr_reload",
						Value:  "",
						MaxAge: -1,
						Path:   "/",
					})
					w.Header().Set("Location", redirectPath)
					w.WriteHeader(http.StatusTemporaryRedirect)
					return
				}
			}
		}

		// If not redirecting, flush the captured response
		rec.Flush()
	})

	// Wrap with Cache-Control headers for static assets
	rootWithCaching := s.addCacheControlHeaders(rootWithFallback)

	// Wrap with LiveReload injection middleware if enabled
	rootWithMiddleware := rootWithCaching
	if s.cfg.Build.LiveReload && s.opts.LiveReloadHub != nil {
		rootWithMiddleware = s.injectLiveReloadScriptWithPort(rootWithCaching, s.cfg.Daemon.HTTP.LiveReloadPort)
	}

	mux.Handle("/", s.mchain(rootWithMiddleware))

	// API endpoint for documentation status
	mux.HandleFunc("/api/status", s.apiHandlers.HandleDocsStatus)

	// Docs server now uses standard timeouts since SSE moved to separate port
	s.docsServer = &http.Server{Handler: mux, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	return s.startServerWithListener("docs", s.docsServer, ln)
}

// resolveDocsRoot picks the directory to serve. Preference order:
// 1. <outputDir>/public if it exists (Hugo static render completed)
// 2. <outputDir> (Hugo project scaffold / in-progress).
func (s *Server) resolveDocsRoot() string {
	out := s.cfg.Output.Directory
	if out == "" {
		out = defaultSiteDir
	}
	// Combine with base_directory if set and path is relative
	if s.cfg.Output.BaseDirectory != "" && !filepath.IsAbs(out) {
		out = filepath.Join(s.cfg.Output.BaseDirectory, out)
	}
	// Normalize to absolute path once; failures just return original path
	if !filepath.IsAbs(out) {
		if abs, err := filepath.Abs(out); err == nil {
			out = abs
		}
	}

	// First, try the public directory (fully rendered site)
	public := filepath.Join(out, "public")
	if st, err := os.Stat(public); err == nil && st.IsDir() {
		slog.Debug("Serving from primary public directory",
			slog.String("path", public),
			slog.Time("modified", st.ModTime()))
		return public
	}

	// If public doesn't exist, check if we're in the middle of a rebuild
	// and the previous backup directory exists
	// NOTE: Hugo generator currently uses "<output>.prev" as the backup dir name during
	// atomic promotion. We also check "<output>_prev" for backward compatibility.
	for _, prev := range []string{out + ".prev", out + "_prev"} {
		prevPublic := filepath.Join(prev, "public")
		if st, err := os.Stat(prevPublic); err == nil && st.IsDir() {
			// Serve from previous backup to avoid empty responses during atomic rename
			slog.Warn("Serving from backup directory - primary public missing",
				slog.String("backup_path", prevPublic),
				slog.String("expected_path", public),
				slog.Time("backup_modified", st.ModTime()))
			return prevPublic
		}
	}

	slog.Warn("No public directory found, serving from output root",
		slog.String("path", out),
		slog.String("expected_public", public),
		slog.String("expected_backup", out+".prev/public or "+out+"_prev/public"))
	return out
}

// findNearestValidParent walks up the URL path hierarchy to find the nearest existing page.
func (s *Server) findNearestValidParent(root, urlPath string) string {
	// Clean the path
	urlPath = filepath.Clean(urlPath)

	// Try parent paths, working upward
	for urlPath != "/" && urlPath != "." {
		urlPath = filepath.Dir(urlPath)
		if urlPath == "." {
			urlPath = "/"
		}

		// Check if this path exists as index.html
		testPath := filepath.Join(root, urlPath, "index.html")
		if _, err := os.Stat(testPath); err == nil {
			// Ensure path ends with / for directory-style URLs
			if !strings.HasSuffix(urlPath, "/") {
				urlPath += "/"
			}
			return urlPath
		}

		// Also check direct file
		if urlPath != "/" {
			testPath = filepath.Join(root, urlPath)
			if stat, err := os.Stat(testPath); err == nil && !stat.IsDir() {
				return urlPath
			}
		}
	}

	// Fall back to root
	return "/"
}

// addCacheControlHeaders wraps a handler to add appropriate Cache-Control headers for static assets.
// Different asset types receive different cache durations:
// - Immutable assets (CSS, JS, fonts, images): 1 year (31536000s)
// - HTML pages: no cache (to ensure content updates are immediately visible)
// - Other assets: short cache (5 minutes).
func (s *Server) addCacheControlHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Set cache control header based on asset type
		setCacheControlForPath(w, path)

		next.ServeHTTP(w, r)
	})
}

// setCacheControlForPath sets appropriate Cache-Control header based on file type.
func setCacheControlForPath(w http.ResponseWriter, path string) {
	cacheControl := determineCacheControl(path)
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
}

// determineCacheControl returns the appropriate Cache-Control value for a path.
func determineCacheControl(path string) string {
	// CSS and JavaScript - cache for 1 year (Hugo typically uses content hashing)
	if strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") {
		return "public, max-age=31536000, immutable"
	}

	// Web fonts - cache for 1 year
	if strings.HasSuffix(path, ".woff") || strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".ttf") || strings.HasSuffix(path, ".eot") ||
		strings.HasSuffix(path, ".otf") {
		return "public, max-age=31536000, immutable"
	}

	// Images - cache for 1 week
	if strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".jpeg") || strings.HasSuffix(path, ".gif") ||
		strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".webp") ||
		strings.HasSuffix(path, ".ico") {
		return "public, max-age=604800"
	}

	// Downloadable files - cache for 1 day
	if strings.HasSuffix(path, ".pdf") || strings.HasSuffix(path, ".zip") ||
		strings.HasSuffix(path, ".tar") || strings.HasSuffix(path, ".gz") {
		return "public, max-age=86400"
	}

	// JSON data files (except search indices) - cache for 5 minutes
	if strings.HasSuffix(path, ".json") && !strings.Contains(path, "search") {
		return "public, max-age=300"
	}

	// XML files (RSS, sitemaps) - cache for 1 hour
	if strings.HasSuffix(path, ".xml") {
		return "public, max-age=3600"
	}

	// HTML pages and directories - no cache to ensure content updates are visible
	if strings.HasSuffix(path, ".html") || path == "/" || !strings.Contains(path, ".") {
		return "no-cache, must-revalidate"
	}

	// For all other files, don't set Cache-Control (let browser use default behavior)
	return ""
}

// responseRecorder captures the status code and body from the underlying handler.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
	body       []byte
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.written = true
	}
	// Don't write to underlying writer yet - we might redirect
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}

func (r *responseRecorder) Flush() {
	if r.written {
		r.ResponseWriter.WriteHeader(r.statusCode)
	}
	if len(r.body) > 0 {
		_, _ = r.ResponseWriter.Write(r.body)
	}
}
