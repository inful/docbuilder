package preview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/preview/runtime"
	"git.home.luguber.info/inful/docbuilder/internal/server/httpserver"
)

// buildStatus tracks the current build state for error display.
type buildStatus struct {
	mu           sync.RWMutex
	lastError    error
	hasGoodBuild bool // true if at least one successful build exists
}

func (bs *buildStatus) setError(err error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.lastError = err
}

func (bs *buildStatus) setSuccess() {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.lastError = nil
	bs.hasGoodBuild = true
}

func (bs *buildStatus) GetStatus() (hasError bool, err error, hasGoodBuild bool) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.lastError != nil, bs.lastError, bs.hasGoodBuild
}

// StartLocalPreview serves the generated site and watches a local docs directory for changes.
// It uses the daemon's HTTP server with built-in LiveReload support.
// If tempOutputDir is non-empty, it will be removed on shutdown.
func StartLocalPreview(ctx context.Context, cfg *config.Config, port int, tempOutputDir string) error {
	absDocs, err := validateAndResolveDocsDir(cfg)
	if err != nil {
		return err
	}

	buildStat := &buildStatus{}
	previewRt := initializePreviewRuntime(ctx, cfg, absDocs, buildStat)

	httpServer, err := startHTTPServer(ctx, cfg, previewRt, port, buildStat)
	if err != nil {
		return err
	}

	watcher, err := setupFileWatcher(absDocs)
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()

	rebuildReq, trigger := setupRebuildDebouncer()
	startRebuildWorker(ctx, cfg, absDocs, previewRt, buildStat, rebuildReq)

	return runPreviewLoop(ctx, watcher, trigger, rebuildReq, httpServer, tempOutputDir)
}

// validateAndResolveDocsDir validates and resolves the absolute path of the docs directory.
func validateAndResolveDocsDir(cfg *config.Config) (string, error) {
	if len(cfg.Repositories) == 0 {
		return "", errors.New("preview requires at least one repository entry pointing to the docs dir")
	}
	docsDir := cfg.Repositories[0].URL
	if docsDir == "" {
		docsDir = "./docs"
	}
	absDocs, err := filepath.Abs(docsDir)
	if err != nil {
		return "", derrors.WrapError(err, derrors.CategoryFileSystem, "resolve docs dir").Build()
	}
	if st, statErr := os.Stat(absDocs); statErr != nil || !st.IsDir() {
		return "", derrors.NewError(derrors.CategoryNotFound, "docs dir not found or not a directory").
			WithContext("path", absDocs).
			Build()
	}
	return absDocs, nil
}

// initializePreviewRuntime performs the initial build and returns the
// preview Runtime (an in-process replacement for the daemon's
// NewPreviewDaemon stub).
func initializePreviewRuntime(ctx context.Context, cfg *config.Config, absDocs string, buildStat *buildStatus) *runtime.Runtime {
	// Initial build
	if err := buildFromLocal(ctx, cfg, absDocs); err != nil {
		slog.Error("initial build failed", "error", err)
		buildStat.setError(err)
	} else {
		writeVSCodeArtifactsIfEnabled(ctx, cfg, absDocs)
		buildStat.setSuccess()
	}

	return runtime.New()
}

// startHTTPServer initializes and starts the HTTP server.
func startHTTPServer(ctx context.Context, cfg *config.Config, previewRt *runtime.Runtime, port int, buildStat *buildStatus) (*httpserver.Server, error) {
	httpServer := httpserver.New(cfg, previewRt, httpserver.Options{
		LiveReloadHub: previewRt.LiveReloadHub(),
		BuildStatus:   buildStat,
	})
	if err := httpServer.Start(ctx); err != nil {
		return nil, derrors.WrapError(err, derrors.CategoryNetwork, "failed to start HTTP server").Build()
	}
	slog.Info("Preview server listening", "port", port, "docs_url", fmt.Sprintf("http://localhost:%d", port))
	return httpServer, nil
}

// setupFileWatcher creates and configures the filesystem watcher.
func setupFileWatcher(absDocs string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, derrors.WrapError(err, derrors.CategoryInternal, "create fsnotify watcher").Build()
	}
	if err := addDirsRecursive(watcher, absDocs); err != nil {
		_ = watcher.Close()
		return nil, err
	}
	return watcher, nil
}

// setupRebuildDebouncer creates rebuild channel and trigger function with debouncing.
func setupRebuildDebouncer() (chan struct{}, func()) {
	var mu sync.Mutex
	var timer *time.Timer
	rebuildReq := make(chan struct{}, 1)

	trigger := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(300*time.Millisecond, func() {
			select {
			case rebuildReq <- struct{}{}:
			default:
			}
		})
	}

	return rebuildReq, trigger
}

// startRebuildWorker starts background goroutine to process rebuild requests.
func startRebuildWorker(ctx context.Context, cfg *config.Config, absDocs string, previewRt *runtime.Runtime, buildStat *buildStatus, rebuildReq chan struct{}) {
	var mu sync.Mutex
	running := false
	pending := false

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-rebuildReq:
				if !ok {
					return
				}
				mu.Lock()
				if running {
					pending = true
					mu.Unlock()
					continue
				}
				running = true
				mu.Unlock()

				processRebuild(ctx, cfg, absDocs, previewRt, buildStat)

				mu.Lock()
				running = false
				if pending {
					pending = false
					mu.Unlock()
					select {
					case rebuildReq <- struct{}{}:
					default:
					}
				} else {
					mu.Unlock()
				}
			}
		}
	}()
}

// processRebuild performs the actual rebuild and notifies browsers.
func processRebuild(ctx context.Context, cfg *config.Config, absDocs string, previewRt *runtime.Runtime, buildStat *buildStatus) {
	slog.Info("Change detected; rebuilding site")
	if err := buildFromLocal(ctx, cfg, absDocs); err != nil {
		slog.Warn("rebuild failed", "error", err)
		buildStat.setError(err)
		if lr := previewRt.LiveReloadHub(); lr != nil {
			lr.Broadcast(fmt.Sprintf("error:%d", time.Now().UnixNano()))
		}
	} else {
		writeVSCodeArtifactsIfEnabled(ctx, cfg, absDocs)
		buildStat.setSuccess()
		if lr := previewRt.LiveReloadHub(); lr != nil {
			lr.Broadcast(strconv.FormatInt(time.Now().UnixNano(), 10))
		}
	}
}

func writeVSCodeArtifactsIfEnabled(ctx context.Context, cfg *config.Config, absDocs string) {
	if cfg == nil || !cfg.Build.VSCodeEditLinks {
		return
	}

	contentDir := filepath.Join(cfg.Output.Directory, "content")
	repoRoot := filepath.Dir(absDocs)
	snippetsPath := filepath.Join(repoRoot, ".vscode", "docbuilder-taxonomies.code-snippets")
	settingsPath := filepath.Join(repoRoot, ".vscode", "settings.json")

	if err := docs.WriteVSCodeTaxonomySnippets(ctx, contentDir, snippetsPath); err != nil {
		slog.Warn("failed to write VS Code taxonomy snippets", "error", err, "path", snippetsPath)
	} else {
		slog.Debug("updated VS Code taxonomy snippets", "path", snippetsPath)
	}

	if err := ensureVSCodeMarkdownSnippetSettings(settingsPath); err != nil {
		slog.Warn("failed to ensure VS Code markdown settings", "error", err, "path", settingsPath)
		return
	}

	slog.Debug("ensured VS Code markdown settings", "path", settingsPath)
}

func ensureVSCodeMarkdownSnippetSettings(settingsPath string) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o750); err != nil {
		return derrors.WrapError(err, derrors.CategoryFileSystem, "create vscode settings directory").Build()
	}

	settings := map[string]any{}
	// #nosec G304 -- settingsPath is derived from the local docs repository root in preview mode.
	if data, readErr := os.ReadFile(settingsPath); readErr == nil {
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := json.Unmarshal(data, &settings); err != nil {
				return derrors.WrapError(err, derrors.CategoryValidation, "parse vscode settings").Build()
			}
		}
	} else if !os.IsNotExist(readErr) {
		return derrors.WrapError(readErr, derrors.CategoryFileSystem, "read vscode settings").Build()
	}

	markdownSettings := map[string]any{}
	if existing, ok := settings["[markdown]"]; ok {
		if existingMap, ok := existing.(map[string]any); ok {
			markdownSettings = existingMap
		}
	}

	markdownSettings["editor.quickSuggestions"] = map[string]any{
		"other":    false,
		"comments": false,
		"strings":  true,
	}
	markdownSettings["editor.snippetSuggestions"] = "top"
	markdownSettings["editor.suggest.showSnippets"] = true
	markdownSettings["editor.wordBasedSuggestions"] = "off"

	settings["[markdown]"] = markdownSettings

	serialized, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryInternal, "marshal vscode settings").Build()
	}
	serialized = append(serialized, '\n')

	if err := os.WriteFile(settingsPath, serialized, 0o600); err != nil {
		return derrors.WrapError(err, derrors.CategoryFileSystem, "write vscode settings").Build()
	}

	return nil
}

// runPreviewLoop handles filesystem events and graceful shutdown.
func runPreviewLoop(ctx context.Context, watcher *fsnotify.Watcher, trigger func(), rebuildReq chan struct{}, httpServer *httpserver.Server, tempOutputDir string) error {
	for {
		select {
		case <-ctx.Done():
			return handleShutdown(ctx, httpServer, rebuildReq, tempOutputDir)
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			handleFileEvent(watcher, ev, trigger)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("watcher error", "error", err)
		}
	}
}

// handleShutdown performs graceful shutdown cleanup.
func handleShutdown(ctx context.Context, httpServer *httpserver.Server, rebuildReq chan struct{}, tempOutputDir string) error {
	slog.Info("Shutting down preview server...")

	// Create a timeout context for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	// Stop HTTP server
	if err := httpServer.Stop(shutdownCtx); err != nil {
		slog.Warn("HTTP server shutdown error", "error", err)
	}

	// Close rebuild channel to stop worker goroutine
	close(rebuildReq)

	// Clean up temp directory if needed
	if tempOutputDir != "" {
		if err := os.RemoveAll(tempOutputDir); err != nil {
			slog.Warn("failed to remove temp output", "dir", tempOutputDir, "error", err)
		} else {
			slog.Info("removed temp output directory", "dir", tempOutputDir)
		}
	}
	return nil
}

// handleFileEvent processes a filesystem event and triggers rebuild if needed.
func handleFileEvent(watcher *fsnotify.Watcher, ev fsnotify.Event, trigger func()) {
	// Skip events for hidden files, swap files, and temp files
	if shouldIgnoreEvent(ev.Name) {
		return
	}
	if ev.Op&fsnotify.Create == fsnotify.Create {
		if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
			_ = addDirsRecursive(watcher, ev.Name)
		}
	}
	slog.Debug("File change detected", "path", ev.Name, "op", ev.Op.String())
	trigger()
}

func addDirsRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if err := w.Add(path); err != nil {
				slog.Warn("watch add failed", "dir", path, "error", err)
			}
		}
		return nil
	})
}

// shouldIgnoreEvent returns true for filesystem events that should not trigger rebuilds.
func shouldIgnoreEvent(path string) bool {
	base := filepath.Base(path)

	// Ignore hidden files
	if strings.HasPrefix(base, ".") {
		return true
	}

	// Ignore editor temp/swap files
	if strings.HasSuffix(base, "~") ||
		strings.HasSuffix(base, ".swp") ||
		strings.HasSuffix(base, ".swx") ||
		strings.HasPrefix(base, ".#") ||
		strings.HasPrefix(base, "#") && strings.HasSuffix(base, "#") {
		return true
	}

	// Ignore common lock files
	if base == ".DS_Store" || base == "Thumbs.db" {
		return true
	}

	return false
}

func buildFromLocal(ctx context.Context, cfg *config.Config, docsPath string) error {
	// Prepare discovery objects
	repos := []config.Repository{{
		URL:    docsPath,
		Name:   "local",
		Branch: "",
		Paths:  []string{"."},
	}}
	discovery := docs.NewDiscovery(repos, &cfg.Build)
	repoPaths := map[string]string{"local": docsPath}
	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return err
	}
	if len(docFiles) == 0 {
		slog.Warn("no docs found in local directory", "dir", docsPath)
	}
	generator := hugo.NewGenerator(cfg, cfg.Output.Directory)
	if _, err := generator.GenerateSiteWithReportContext(ctx, docFiles); err != nil {
		return err
	}
	return nil
}
