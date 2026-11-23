package daemon

import (
	"bytes"
	"context"

	// #nosec G501 -- MD5 used for content change detection, not cryptographic security
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/linkverify"
)

// updateStateAfterBuild updates the state manager with build metadata for skip evaluation.
// This ensures subsequent builds can correctly detect when nothing has changed.
func (d *Daemon) updateStateAfterBuild(report *models.BuildReport) {
	// The hashes are written for a successful build only: a failed build
	// has partial documents, and the hash of a partial build would make
	// the next build skip a change.
	if report.Outcome == models.OutcomeSuccess {
		// Update config hash
		if report.ConfigHash != "" {
			d.state.stateManager.SetLastConfigHash(report.ConfigHash)
			slog.Debug("Updated config hash in state", "hash", report.ConfigHash)
		}

		// Update global doc files hash
		if report.DocFilesHash != "" {
			d.state.stateManager.SetLastGlobalDocFilesHash(report.DocFilesHash)
			slog.Debug("Updated global doc files hash in state", "hash", report.DocFilesHash)
		}
	}

	// Update repository commits and hashes.
	// The per-repository commit is recorded by the build itself (the
	// generator persists it where the repositories and their commits are
	// both known). It used to be recorded here from d.config.Repositories,
	// which is empty in forge/discovery mode — so discovered repositories
	// never got commit metadata and the next build could never be skipped.

	// Save state to disk
	if err := d.state.stateManager.Save(); err != nil {
		slog.Warn("Failed to save state after build", "error", err)
	}

	// The counters of the repositories are raised: without it every
	// repository stays "pending" on the status page, and the operator
	// cannot tell which of them was built.
	d.raiseRepositoryBuildCounters()

	// The time of the last build is shown on the status page. Without
	// it the operator cannot tell when the site was built.
	d.lifecycle.mu.Lock()
	now := time.Now()
	d.lifecycle.lastBuild = &now
	d.lifecycle.mu.Unlock()
}

// verifyLinksAfterBuild runs link verification in the background after a successful build.
// This is a low-priority task that doesn't block the build pipeline.
func (d *Daemon) verifyLinksAfterBuild(ctx context.Context, buildID string) {
	// Create background context with timeout (derived from parent ctx)
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	slog.Info("Starting link verification for build", "build_id", buildID)

	// Collect page metadata from build report
	pages, err := d.collectPageMetadata(buildID)
	if err != nil {
		slog.Error("Failed to collect page metadata for link verification",
			"build_id", buildID,
			"error", err)
		return
	}

	// Verify links
	if err := d.optional.linkVerifier.VerifyPages(verifyCtx, pages); err != nil {
		slog.Warn("Link verification encountered errors",
			"build_id", buildID,
			"error", err)
		return
	}

	slog.Info("Link verification completed successfully", "build_id", buildID)
}

// shouldRunLinkVerification returns true when it makes sense to run link verification.
// If the build was short-circuited due to no changes, the rendered output is unchanged
// and re-verifying links is wasted work.
func shouldRunLinkVerification(report *models.BuildReport) bool {
	if report == nil {
		return false
	}
	return report.SkipReason != "no_changes"
}

// collectPageMetadata collects metadata for all pages in the build.
func (d *Daemon) collectPageMetadata(buildID string) ([]*linkverify.PageMetadata, error) {
	// The build writes to the resolved output directory (base_directory +
	// directory), which is where the HTTP server reads from too. Using
	// daemon.storage.output_dir here pointed at a different folder and
	// the verification silently found nothing.
	outputDir := resolveOutputDir(d.config)
	publicDir, ok := resolvePublicDirForVerification(outputDir)
	if !ok {
		slog.Warn("No public directory available for link verification; skipping page metadata collection",
			"build_id", buildID,
			"output_dir", outputDir,
			"expected_public", filepath.Join(outputDir, "public"),
			"expected_backup", outputDir+".prev/public or "+outputDir+"_prev/public")
		return nil, nil
	}

	var pages []*linkverify.PageMetadata

	// Walk the public directory to find all HTML files
	err := filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process HTML files
		if info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Get relative path from public directory
		relPath, err := filepath.Rel(publicDir, path)
		if err != nil {
			return err
		}

		// Create basic DocFile structure (we don't have the original here).
		// The link verifier mostly needs the path information.
		docFile := &docs.DocFile{
			Path:         path,
			RelativePath: relPath,
			Repository:   extractRepoFromPath(relPath),
			Name:         strings.TrimSuffix(filepath.Base(path), ".html"),
		}

		// The source markdown of the page. It is not read here: the front
		// matter is only needed when a link turns out to be broken, and the
		// link verifier reads it on demand.
		contentPath := filepath.Join(outputDir, "content", strings.TrimSuffix(relPath, ".html")+".md")

		// Build rendered URL
		renderedURL := d.config.Hugo.BaseURL
		if !strings.HasSuffix(renderedURL, "/") {
			renderedURL += "/"
		}
		renderedURL += strings.TrimPrefix(relPath, "/")

		// Read the rendered page once: the hash detects changes, the links
		// are what gets verified. Reading it twice doubled the I/O of the
		// whole site on every build.
		var contentHash string
		var links []*linkverify.Link
		if htmlBytes, err := os.ReadFile(filepath.Clean(path)); err == nil { //nolint:gosec // walk within controlled output dir
			// #nosec G401 -- MD5 is used for content hashing, not cryptographic security
			hash := md5.New()
			hash.Write(htmlBytes)
			contentHash = hex.EncodeToString(hash.Sum(nil))

			extracted, extractErr := linkverify.ExtractLinksFromReader(bytes.NewReader(htmlBytes), d.config.Hugo.BaseURL)
			if extractErr != nil {
				slog.Warn("Failed to extract links from rendered page",
					"path", path,
					"error", extractErr)
			} else {
				links = extracted
			}
		}

		page := &linkverify.PageMetadata{
			DocFile:      docFile,
			HTMLPath:     path,
			HugoPath:     contentPath,
			RenderedPath: relPath,
			RenderedURL:  renderedURL,
			ContentPath:  contentPath,
			BaseURL:      d.config.Hugo.BaseURL,
			BuildID:      buildID,
			BuildTime:    time.Now(),
			ContentHash:  contentHash,
			Links:        links,
		}

		pages = append(pages, page)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk public directory %s: %w", publicDir, err)
	}

	slog.Debug("Collected page metadata for link verification",
		"build_id", buildID,
		"page_count", len(pages))

	return pages, nil
}

// resolvePublicDirForVerification mirrors the HTTP server docs-root selection.
// It prefers the primary rendered output (<output>/public). If that doesn't exist,
// it falls back to the previous backup directory used during atomic promotion.
func resolvePublicDirForVerification(outputDir string) (string, bool) {
	primary := filepath.Join(outputDir, "public")
	if st, err := os.Stat(primary); err == nil && st.IsDir() {
		return primary, true
	}
	for _, prev := range []string{outputDir + ".prev", outputDir + "_prev"} {
		prevPublic := filepath.Join(prev, "public")
		if st, err := os.Stat(prevPublic); err == nil && st.IsDir() {
			return prevPublic, true
		}
	}
	return "", false
}

// extractRepoFromPath attempts to extract repository name from rendered path.
// Rendered paths typically follow pattern: repo-name/section/file.html
// Hugo-generated pages (categories, tags, etc.) are marked with "_hugo" prefix.
func extractRepoFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) == 0 {
		return "unknown"
	}

	firstSegment := parts[0]

	// Recognize Hugo-generated taxonomy and special pages
	if isHugoGeneratedPath(firstSegment) {
		return "_hugo_" + firstSegment
	}

	// For root-level files (index.html, 404.html, sitemap.xml, etc.)
	if len(parts) == 1 {
		return "_hugo_root"
	}

	return firstSegment
}

// isHugoGeneratedPath checks if a path segment is a Hugo-generated taxonomy or special page.
func isHugoGeneratedPath(segment string) bool {
	hugoGeneratedPaths := map[string]bool{
		"categories":  true,
		"tags":        true,
		"authors":     true,
		"series":      true,
		"search":      true,
		"sitemap.xml": true,
	}
	return hugoGeneratedPaths[segment]
}

// raiseRepositoryBuildCounters raises the build counter of every
// repository the daemon knows.
//
// The daemon does not hold the list of the repositories in discovery
// mode, so the state is the list: every repository it knows was part
// of the build.
func (d *Daemon) raiseRepositoryBuildCounters() {
	if d.state.stateManager == nil {
		return
	}
	reader, ok := d.state.stateManager.(interface {
		RepositoryURLs() []string
	})
	if !ok {
		return
	}
	commits, ok := d.state.stateManager.(interface {
		GetRepoLastCommit(url string) string
	})
	for _, url := range reader.RepositoryURLs() {
		// A repository without a commit was not built: it counts as a
		// failure, and the status page says so.
		built := true
		if ok {
			built = commits.GetRepoLastCommit(url) != ""
		}
		d.state.stateManager.IncrementRepoBuild(url, built)
	}
}
