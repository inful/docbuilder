package daemon

import (
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

	ggit "github.com/go-git/go-git/v5"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/linkverify"
)

// updateStateAfterBuild updates the state manager with build metadata for skip evaluation.
// This ensures subsequent builds can correctly detect when nothing has changed.
func (d *Daemon) updateStateAfterBuild(report *models.BuildReport) {
	// Update config hash
	if report.ConfigHash != "" {
		d.stateManager.SetLastConfigHash(report.ConfigHash)
		slog.Debug("Updated config hash in state", "hash", report.ConfigHash)
	}

	// Update global doc files hash
	if report.DocFilesHash != "" {
		d.stateManager.SetLastGlobalDocFilesHash(report.DocFilesHash)
		slog.Debug("Updated global doc files hash in state", "hash", report.DocFilesHash)
	}

	// Update repository commits and hashes.
	// Read from persistent workspace (repo_cache_dir/working) to get current commit SHAs.
	workspacePath := filepath.Join(d.config.Daemon.Storage.RepoCacheDir, "working")
	for i := range d.config.Repositories {
		repo := &d.config.Repositories[i]
		repoPath := filepath.Join(workspacePath, repo.Name)

		// Check if repository exists
		if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
			continue // Skip if not a git repository
		}

		// Open git repository to get current commit
		gitRepo, err := ggit.PlainOpen(repoPath)
		if err != nil {
			slog.Warn("Failed to open git repository for state update",
				"repository", repo.Name,
				"path", repoPath,
				"error", err)
			continue
		}

		// Get HEAD reference
		ref, err := gitRepo.Head()
		if err != nil {
			slog.Warn("Failed to get HEAD for state update",
				"repository", repo.Name,
				"error", err)
			continue
		}

		commit := ref.Hash().String()

		// Initialize repository state if needed
		d.stateManager.EnsureRepositoryState(repo.URL, repo.Name, repo.Branch)

		// Update commit in state
		d.stateManager.SetRepoLastCommit(repo.URL, repo.Name, repo.Branch, commit)
		slog.Debug("Updated repository commit in state",
			"repository", repo.Name,
			"commit", commit[:8])
	}

	// Save state to disk
	if err := d.stateManager.Save(); err != nil {
		slog.Warn("Failed to save state after build", "error", err)
	}
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
	if err := d.linkVerifier.VerifyPages(verifyCtx, pages); err != nil {
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
	outputDir := d.config.Daemon.Storage.OutputDir
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

		// Try to find corresponding content file to extract front matter
		var frontMatter map[string]any
		contentPath := filepath.Join(outputDir, "content", strings.TrimSuffix(relPath, ".html")+".md")
		if contentBytes, err := os.ReadFile(filepath.Clean(contentPath)); err == nil {
			if fm, err := linkverify.ParseFrontMatter(contentBytes); err == nil {
				frontMatter = fm
			}
		}

		// Build rendered URL
		renderedURL := d.config.Hugo.BaseURL
		if !strings.HasSuffix(renderedURL, "/") {
			renderedURL += "/"
		}
		renderedURL += strings.TrimPrefix(relPath, "/")

		// Compute MD5 hash of HTML content for change detection
		var contentHash string
		if htmlBytes, err := os.ReadFile(filepath.Clean(path)); err == nil {
			// #nosec G401 -- MD5 is used for content hashing, not cryptographic security
			hash := md5.New()
			hash.Write(htmlBytes)
			contentHash = hex.EncodeToString(hash.Sum(nil))
		}

		page := &linkverify.PageMetadata{
			DocFile:      docFile,
			HTMLPath:     path,
			HugoPath:     contentPath,
			RenderedPath: relPath,
			RenderedURL:  renderedURL,
			FrontMatter:  frontMatter,
			BaseURL:      d.config.Hugo.BaseURL,
			BuildID:      buildID,
			BuildTime:    time.Now(),
			ContentHash:  contentHash,
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
