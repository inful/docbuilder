package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// buildStatus tracks the current build state for error display.
type buildStatus struct {
	mu            sync.RWMutex
	lastError     error
	lastErrorTime time.Time
	hasGoodBuild  bool // true if at least one successful build exists
}

func (bs *buildStatus) setError(err error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.lastError = err
	bs.lastErrorTime = time.Now()
}

func (bs *buildStatus) setSuccess() {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.lastError = nil
	bs.hasGoodBuild = true
}

func (bs *buildStatus) getStatus() (hasError bool, err error, hasGoodBuild bool) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.lastError != nil, bs.lastError, bs.hasGoodBuild
}

// StartLocalPreview serves the generated site and watches a local docs directory for changes.
// It uses the daemon's HTTP server with built-in LiveReload support.
// If tempOutputDir is non-empty, it will be removed on shutdown.
func StartLocalPreview(ctx context.Context, cfg *config.Config, port int, tempOutputDir string) error {
	if len(cfg.Repositories) == 0 {
		return fmt.Errorf("preview requires at least one repository entry pointing to the docs dir")
	}
	docsDir := cfg.Repositories[0].URL
	if docsDir == "" {
		docsDir = "./docs"
	}
	absDocs, err := filepath.Abs(docsDir)
	if err != nil {
		return fmt.Errorf("resolve docs dir: %w", err)
	}
	if st, statErr := os.Stat(absDocs); statErr != nil || !st.IsDir() {
		return fmt.Errorf("docs dir not found or not a directory: %s", absDocs)
	}

	// Track build status for error display
	buildStat := &buildStatus{}

	// Initial build
	if _, err = buildFromLocal(cfg, absDocs); err != nil {
		slog.Error("initial build failed", "error", err)
		buildStat.setError(err)
	} else {
		buildStat.setSuccess()
	}

	// Create minimal daemon with HTTP server
	daemon := &Daemon{
		config:     cfg,
		startTime:  time.Now(),
		metrics:    NewMetricsCollector(),
		liveReload: NewLiveReloadHub(nil), // nil metrics collector for LiveReload
	}
	daemon.status.Store(StatusRunning)
	daemon.buildStatus = buildStat

	httpServer := NewHTTPServer(cfg, daemon)
	if err = httpServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	slog.Info("Preview server listening", "port", port, "docs_url", fmt.Sprintf("http://localhost:%d", port))

	// Watch filesystem
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	// Recursively add directories under docs
	if err := addDirsRecursive(watcher, absDocs); err != nil {
		return err
	}

	// Debounce + serialize rebuilds
	var mu sync.Mutex
	var timer *time.Timer
	rebuildReq := make(chan struct{}, 1)
	running := false
	pending := false

	// background worker to process rebuild requests sequentially
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

				slog.Info("Change detected; rebuilding site")
				if _, err := buildFromLocal(cfg, absDocs); err != nil {
					slog.Warn("rebuild failed", "error", err)
					buildStat.setError(err)
					// Notify browsers about error so they can display error overlay
					if daemon.liveReload != nil {
						daemon.liveReload.Broadcast(fmt.Sprintf("error:%d", time.Now().UnixNano()))
					}
				} else {
					buildStat.setSuccess()
					// Notify connected browsers via LiveReload
					if daemon.liveReload != nil {
						// Broadcast hash to trigger browser refresh
						daemon.liveReload.Broadcast(fmt.Sprintf("%d", time.Now().UnixNano()))
					}
				}

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

	trigger := func() {
		mu.Lock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(300*time.Millisecond, func() {
			select {
			case rebuildReq <- struct{}{}:
			default:
			}
		})
		mu.Unlock()
	}

	// Main loop: respond to fs events or shutdown
	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down preview server...")

			// Create a timeout context for graceful shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Skip events for hidden files, swap files, and temp files
			if shouldIgnoreEvent(ev.Name) {
				continue
			}
			if ev.Op&fsnotify.Create == fsnotify.Create {
				if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
					_ = addDirsRecursive(watcher, ev.Name)
				}
			}
			slog.Debug("File change detected", "path", ev.Name, "op", ev.Op.String())
			trigger()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("watcher error", "error", err)
		}
	}
}

func addDirsRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
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

func buildFromLocal(cfg *config.Config, docsPath string) (bool, error) {
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
		return false, err
	}
	if len(docFiles) == 0 {
		slog.Warn("no docs found in local directory", "dir", docsPath)
	}
	generator := hugo.NewGenerator(cfg, cfg.Output.Directory)
	if err := generator.GenerateSite(docFiles); err != nil {
		return false, err
	}
	return true, nil
}
