package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/services"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// deltaManager provides delta-related helper functions for build operations.
// This is a stateless helper that was previously an interface with single implementation.
type deltaManager struct{}

// NewDeltaManager creates a delta manager helper.
// Kept for backward compatibility with existing tests.
func NewDeltaManager() *deltaManager {
	return &deltaManager{}
}

// AttachDeltaMetadata adds delta information to the build report.
func (dm *deltaManager) AttachDeltaMetadata(report *hugo.BuildReport, deltaPlan *DeltaPlan, job *BuildJob) {
	if deltaPlan == nil {
		return
	}

	if deltaPlan.Decision == DeltaDecisionPartial {
		report.DeltaDecision = "partial"
		report.DeltaChangedRepos = append([]string{}, deltaPlan.ChangedRepos...)
	} else {
		report.DeltaDecision = "full"
	}

	// Attach per-repo reasons if provided via deltaPlan extension
	if report.DeltaRepoReasons == nil {
		report.DeltaRepoReasons = map[string]string{}
	}
	// Get reasons from TypedMeta
	var reasons map[string]string
	if job.TypedMeta != nil && job.TypedMeta.DeltaRepoReasons != nil {
		reasons = job.TypedMeta.DeltaRepoReasons
	}
	for k, v := range reasons {
		report.DeltaRepoReasons[k] = v
	}
}

// pathGetter interface for reading repository document file paths
// The following interfaces were removed as they're already covered by state.RepositoryMetadataStore
// which is the proper abstraction for repository metadata operations.

// RecomputeGlobalDocHash recalculates the global documentation hash for partial builds.
func (dm *deltaManager) RecomputeGlobalDocHash(
	report *hugo.BuildReport,
	deltaPlan *DeltaPlan,
	stateMgr services.StateManager,
	job *BuildJob,
	workspace string,
	cfg *config.Config,
) (int, error) {
	if deltaPlan == nil || deltaPlan.Decision != DeltaDecisionPartial || report.DocFilesHash == "" {
		return 0, nil
	}

	// Type assert to RepositoryMetadataStore for repository metadata operations
	metaStore, ok := stateMgr.(state.RepositoryMetadataStore)
	if !ok {
		return 0, nil
	}

	changedSet := make(map[string]struct{}, len(deltaPlan.ChangedRepos))
	for _, u := range deltaPlan.ChangedRepos {
		changedSet[u] = struct{}{}
	}

	// Get repositories from TypedMeta
	var orig []config.Repository
	if job.TypedMeta != nil && len(job.TypedMeta.Repositories) > 0 {
		orig = job.TypedMeta.Repositories
	}
	allPaths := make([]string, 0, 2048)
	deletionsDetected := 0

	for _, r := range orig {
		paths := metaStore.GetRepoDocFilePaths(r.URL)

		// For unchanged repos, optionally detect deletions by scanning workspace clone
		if _, isChanged := changedSet[r.URL]; !isChanged &&
			workspace != "" && cfg != nil && cfg.Build.DetectDeletions {
			freshPaths, deleted, err := dm.scanForDeletions(r, workspace, paths)
			if err != nil {
				continue // Skip on error, use existing paths
			}

			if len(freshPaths) != len(paths) {
				metaStore.SetRepoDocFilePaths(r.URL, freshPaths)
				hash := dm.computePathsHash(freshPaths)
				metaStore.SetRepoDocFilesHash(r.URL, hash)
				paths = freshPaths
				deletionsDetected += deleted
			}
		}

		if len(paths) > 0 {
			allPaths = append(allPaths, paths...)
		}
	}

	if len(allPaths) > 0 {
		sort.Strings(allPaths)
		report.DocFilesHash = dm.computePathsHash(allPaths)
	}

	return deletionsDetected, nil
}

// scanForDeletions scans a repository for current markdown files and compares with persisted paths.
func (dm *deltaManager) scanForDeletions(repo config.Repository, workspace string, persistedPaths []string) ([]string, int, error) {
	repoRoot := filepath.Join(workspace, repo.Name)

	fi, err := os.Stat(repoRoot)
	if err != nil || !fi.IsDir() {
		return persistedPaths, 0, err
	}

	fresh := make([]string, 0, len(persistedPaths))
	docRoots := []string{"docs", "documentation"}

	for _, dr := range docRoots {
		base := filepath.Join(repoRoot, dr)
		sfi, serr := os.Stat(base)
		if serr != nil || !sfi.IsDir() {
			continue
		}

		err := filepath.WalkDir(base, func(p string, d os.DirEntry, werr error) error {
			if werr != nil || d == nil || d.IsDir() {
				return nil
			}

			name := strings.ToLower(d.Name())
			if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown") {
				if rel, rerr := filepath.Rel(repoRoot, p); rerr == nil {
					fresh = append(fresh, filepath.ToSlash(filepath.Join(repo.Name, rel)))
				}
			}
			return nil
		})

		if err != nil {
			return persistedPaths, 0, fmt.Errorf("walking directory %s: %w", base, err)
		}
	}

	sort.Strings(fresh)

	// Check if paths changed
	pathsChanged := len(fresh) != len(persistedPaths)
	if !pathsChanged {
		for i := range fresh {
			if i >= len(persistedPaths) || fresh[i] != persistedPaths[i] {
				pathsChanged = true
				break
			}
		}
	}

	deletions := 0
	if pathsChanged && len(fresh) < len(persistedPaths) {
		deletions = len(persistedPaths) - len(fresh)
	}

	if pathsChanged {
		return fresh, deletions, nil
	}

	return persistedPaths, 0, nil
}

// computePathsHash computes a SHA256 hash of file paths.
func (dm *deltaManager) computePathsHash(paths []string) string {
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
