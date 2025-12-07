package daemon

import (
	"os"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/services"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// BuildMetricsCollector handles metrics collection during build process
type BuildMetricsCollector interface {
	// RecordDeletions records detected file deletions
	RecordDeletions(deletionsDetected int, job *BuildJob)

	// UpdateRepositoryMetrics updates build counters and document counts
	UpdateRepositoryMetrics(
		repos []config.Repository,
		stateMgr services.StateManager,
		outDir string,
		genErr error,
		skipReport *hugo.BuildReport,
	) error
}

// BuildMetricsCollectorImpl implements BuildMetricsCollector
type BuildMetricsCollectorImpl struct{}

// NewBuildMetricsCollector creates a new build metrics collector
func NewBuildMetricsCollector() BuildMetricsCollector {
	return &BuildMetricsCollectorImpl{}
}

// RecordDeletions records detected file deletions
func (bmc *BuildMetricsCollectorImpl) RecordDeletions(deletionsDetected int, job *BuildJob) {
	if deletionsDetected <= 0 {
		return
	}

	EnsureTypedMeta(job)
	mc := job.TypedMeta.MetricsCollector
	if mc == nil {
		return
	}

	for i := 0; i < deletionsDetected; i++ {
		mc.IncrementCounter("doc_deletions_detected")
	}
}

// The repositoryBuildTracker interface was removed - using state.RepositoryBuildCounter and state.RepositoryMetadataWriter

// UpdateRepositoryMetrics updates build counters and document counts
func (bmc *BuildMetricsCollectorImpl) UpdateRepositoryMetrics(
	repos []config.Repository,
	stateMgr services.StateManager,
	outDir string,
	genErr error,
	skipReport *hugo.BuildReport,
) error {
	// Type assert to both needed interfaces
	counter, hasCounter := stateMgr.(state.RepositoryBuildCounter)
	writer, hasWriter := stateMgr.(state.RepositoryMetadataWriter)
	if (!hasCounter && !hasWriter) || skipReport != nil {
		return nil
	}

	success := genErr == nil
	contentRoot := filepath.Join(outDir, "content")

	perRepoDocCounts, err := bmc.calculateDocumentCounts(repos, contentRoot)
	if err != nil {
		return err
	}

	// Update metrics for each repository
	for _, r := range repos {
		if hasCounter {
			counter.IncrementRepoBuild(r.URL, success)
		}
		if hasWriter {
			if count, exists := perRepoDocCounts[r.URL]; exists {
				writer.SetRepoDocumentCount(r.URL, count)
			}
		}
	}

	return nil
}

// calculateDocumentCounts calculates document counts for each repository
// nolint:unparam // This helper currently never returns a non-nil error.
func (bmc *BuildMetricsCollectorImpl) calculateDocumentCounts(
	repos []config.Repository,
	contentRoot string,
) (map[string]int, error) {
	perRepoDocCounts := make(map[string]int, len(repos))

	for _, r := range repos {
		count, err := bmc.countRepositoryDocuments(r, contentRoot)
		if err != nil {
			// Set to 0 on error rather than failing completely
			count = 0
		}
		perRepoDocCounts[r.URL] = count
	}

	return perRepoDocCounts, nil //nolint:nilerr // errors are treated as zero counts; overall function returns map only
}

// countRepositoryDocuments counts markdown documents for a specific repository
func (bmc *BuildMetricsCollectorImpl) countRepositoryDocuments(
	repo config.Repository,
	contentRoot string,
) (int, error) {
	// Try direct repository path first
	repoPath := filepath.Join(contentRoot, repo.Name)
	if fi, err := os.Stat(repoPath); err == nil && fi.IsDir() {
		return bmc.countMarkdownFiles(repoPath), nil
	}

	// Try namespaced repository path (repo might be under organization folder)
	entries, err := os.ReadDir(contentRoot)
	if err != nil {
		return 0, err // Propagate error; caller handles by setting count=0
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		nsRepoPath := filepath.Join(contentRoot, entry.Name(), repo.Name)
		if fi, err := os.Stat(nsRepoPath); err == nil && fi.IsDir() {
			return bmc.countMarkdownFiles(nsRepoPath), nil
		}
	}

	return 0, nil
}

// countMarkdownFiles counts markdown files in a directory tree, excluding standard files
func (bmc *BuildMetricsCollectorImpl) countMarkdownFiles(root string) int {
	count := 0

	if werr := filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}

		if bmc.isDocumentationFile(d.Name()) {
			count++
		}

		return nil
	}); werr != nil {
		// best-effort counter; ignore traversal error
		return count //nolint:nilerr // traversal errors are intentionally ignored for metrics
	}

	return count
}

// isDocumentationFile checks if a file should be counted as documentation
func (bmc *BuildMetricsCollectorImpl) isDocumentationFile(filename string) bool {
	name := strings.ToLower(filename)

	// Check if it's a markdown file
	if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".markdown") {
		return false
	}

	// Exclude standard files that shouldn't be counted as documentation
	excludedFiles := []string{
		"readme.md",
		"license.md",
		"contributing.md",
		"changelog.md",
	}

	for _, excluded := range excludedFiles {
		if name == excluded {
			return false
		}
	}

	return true
}
