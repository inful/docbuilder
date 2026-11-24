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
//
// The base directory holds the output when the directory is relative, so
// the two are joined here. Everything that looks for the output - the
// docs root, the status page, the content of the site - has to look in
// the same place, or it looks where the site is not.
func (s *Server) resolveAbsoluteOutputDir() string {
	out := s.cfg.ResolveOutputDir()
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

	// No rendered site exists yet (first build still running, or the
	// previous one was lost). Serve the pending page for every
	// documentation URL, not just the root: falling through to the file
	// server would serve the Hugo scaffold, which 404s for every real
	// documentation path. Asset-looking paths keep the file server so
	// they 404 instead of returning an HTML page.
	if filepath.Ext(r.URL.Path) != "" {
		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
		return
	}

	s.renderBuildPendingPage(w)
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

	// Show what the daemon is doing, so the page reads as "working on it"
	// rather than a dead end.
	state := ""
	if s.runtime != nil {
		state = fmt.Sprintf(`<p><small>Daemon: %s &middot; %d build(s) queued &middot; %d active</small></p>`,
			s.runtime.GetStatus(), s.runtime.GetQueueLength(), s.runtime.GetActiveJobs())
	}

	_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Site rendering</title></head><body><h1>Documentation is being prepared</h1><p>The site hasn't been rendered yet. This page will be replaced automatically once rendering completes.</p>%s%s</body></html>`, state, scriptTag)
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
	mux.HandleFunc("/site-ready", s.handleSiteReadiness)

	// VS Code edit link handler for local preview mode
	mux.HandleFunc("/_edit/", s.handleVSCodeEdit)
	// Machine-readable taxonomy endpoint for template/snippet tooling.
	mux.HandleFunc("/api/taxonomies.json", s.handleTaxonomiesJSON)
	// The search of the site. The daemon answers the queries with its
	// own index, so the visitor never downloads the index itself.
	if s.opts.SearchHandle != nil {
		mux.HandleFunc("/api/search", s.opts.SearchHandle)
	}
	// The pages of the site: the inventory of what is published.
	if s.opts.PagesHandle != nil {
		mux.HandleFunc("/api/pages", s.opts.PagesHandle)
	}

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
		// Only a LiveReload request can end up at the nearest parent
		// page, and only that request needs its response captured.
		// Every other request goes straight to the file server: the
		// capture holds the whole file in memory until the response
		// has been written, so a big file and a few readers at once
		// is a lot of memory for nothing.
		if r.Method != http.MethodGet || !s.cfg.Build.LiveReload || !hasLiveReloadCookie(r) {
			rootHandler.ServeHTTP(w, r)
			return
		}

		// Capture the response to detect 404s
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		rootHandler.ServeHTTP(rec, r)

		// If we got a 404, try to redirect to the nearest parent
		if rec.statusCode == http.StatusNotFound {
			root := s.resolveDocsRoot()
			redirectPath := s.findNearestValidParent(root, r.URL.Path)
			if redirectPath != "" && redirectPath != r.URL.Path {
				// Clear the cookie and redirect
				//nolint:gosec // local preview commonly runs on HTTP; Secure cookies would not be sent
				http.SetCookie(w, &http.Cookie{
					Name:     "docbuilder_lr_reload",
					Value:    "",
					MaxAge:   -1,
					Path:     "/",
					HttpOnly: true,
					Secure:   r.TLS != nil,
					SameSite: http.SameSiteLaxMode,
				})
				w.Header().Set("Location", redirectPath)
				w.WriteHeader(http.StatusTemporaryRedirect)
				return
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
	out := s.resolveAbsoluteOutputDir()

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

	// The URL path is the only user input here, so every candidate is
	// resolved under the docs root and has to stay there before it
	// reaches the filesystem. The edit handler resolves its paths the
	// same way.
	rootPrefix := filepath.Clean(root)
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}

	// Try parent paths, working upward
	for urlPath != "/" && urlPath != "." {
		urlPath = filepath.Dir(urlPath)
		if urlPath == "." {
			urlPath = "/"
		}

		// Check if this path exists as index.html.
		testPath := filepath.Join(root, urlPath, "index.html")
		cleanPath := filepath.Clean(testPath)
		if !strings.HasPrefix(cleanPath, rootPrefix) {
			return "/"
		}
		//nolint:gosec // the path is resolved under the docs root and stays there
		if _, err := os.Stat(testPath); err == nil {
			// Ensure path ends with / for directory-style URLs
			if !strings.HasSuffix(urlPath, "/") {
				urlPath += "/"
			}
			return urlPath
		}

		// Also check direct file.
		if urlPath != "/" {
			testPath = filepath.Join(root, urlPath)
			cleanPath = filepath.Clean(testPath)
			if !strings.HasPrefix(cleanPath, rootPrefix) {
				return "/"
			}
			//nolint:gosec // the path is resolved under the docs root and stays there
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
	// CSS and JavaScript - cache for 1 year when the URL is
	// content-addressed, because then the URL changes with the content.
	// A stable URL, like the theme's prism.css, has to be revalidated:
	// an immutable year-long cache would keep the old file after a
	// theme bump.
	if strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") {
		if isContentAddressed(path) {
			return "public, max-age=31536000, immutable"
		}
		return cacheControlFiveMinute
	}

	// Web fonts - cache for 1 year when the URL is content-addressed, for
	// the same reason as CSS and JavaScript: a stable URL has to be
	// revalidated, or a theme bump keeps the old font in the browser.
	if strings.HasSuffix(path, ".woff") || strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".ttf") || strings.HasSuffix(path, ".eot") ||
		strings.HasSuffix(path, ".otf") {
		if isContentAddressed(path) {
			return "public, max-age=31536000, immutable"
		}
		return cacheControlFiveMinute
	}

	// Images - cache for 1 week
	if strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".jpeg") || strings.HasSuffix(path, ".gif") ||
		strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".webp") ||
		strings.HasSuffix(path, ".ico") {
		return cacheControlWeek
	}

	// Downloadable files - cache for 1 day
	if strings.HasSuffix(path, ".pdf") || strings.HasSuffix(path, ".zip") ||
		strings.HasSuffix(path, ".tar") || strings.HasSuffix(path, ".gz") {
		return cacheControlDay
	}

	// JSON data files (except search indices) - cache for 5 minutes
	if strings.HasSuffix(path, ".json") && !strings.Contains(path, "search") {
		return cacheControlFiveMinute
	}

	// XML files (RSS, sitemaps) - cache for 1 hour
	if strings.HasSuffix(path, ".xml") {
		return cacheControlHour
	}

	// HTML pages and directories - no cache to ensure content updates are visible
	if strings.HasSuffix(path, ".html") || path == "/" || !strings.Contains(path, ".") {
		return cacheControlNoCache
	}

	// For all other files, don't set Cache-Control (let browser use default behavior)
	return ""
}

// isContentAddressed reports whether a path names a fingerprinted
// asset, one whose URL carries a hash of its own content. Hugo writes
// those as name.<hash>.ext, with the hash a long run of hex digits.
// Only such a URL changes when the content does, so only such a URL
// may be cached immutably.
func isContentAddressed(path string) bool {
	name := path
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	// The extension is the last segment; the hash is the one before it.
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return false
	}
	stem := name[:dot]
	dot = strings.LastIndex(stem, ".")
	if dot < 0 {
		return false
	}
	hash := stem[dot+1:]
	if len(hash) < 32 {
		return false
	}
	for _, c := range hash {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// hasLiveReloadCookie reports whether the request carries the LiveReload
// reload cookie, the one case where a missing page is redirected to the
// nearest page that does exist.
func hasLiveReloadCookie(r *http.Request) bool {
	cookie, err := r.Cookie("docbuilder_lr_reload")
	return err == nil && cookie.Value == "1"
}

// maxRecorderBody is the amount of a response the recorder holds in
// memory. The recorder is only there to see whether the page is
// missing: a bigger response is written through, so one request cannot
// hold a whole big file in the memory of the server.
const maxRecorderBody = 512 << 10

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
	if len(r.body)+len(b) > maxRecorderBody {
		// Too big to hold: what was captured goes to the client and
		// the rest follows it straight away.
		r.flushToClient()
		return r.ResponseWriter.Write(b)
	}
	r.body = append(r.body, b...)
	return len(b), nil
}

// flushToClient writes the header and the captured body to the client.
func (r *responseRecorder) flushToClient() {
	if r.written {
		r.ResponseWriter.WriteHeader(r.statusCode)
		r.written = false
	}
	if len(r.body) > 0 {
		_, _ = r.ResponseWriter.Write(r.body)
		r.body = nil
	}
}

func (r *responseRecorder) Flush() {
	r.flushToClient()
}
