package delta

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/state"
)

// Manager provides delta-related helper functions used during build reporting.
//
// It is intentionally stateless and pure-ish (all state interactions happen via
// narrow state interfaces).
type Manager struct{}

func NewManager() *Manager { return &Manager{} }

// AttachDeltaMetadata adds delta information to the build report.
func (m *Manager) AttachDeltaMetadata(report *models.BuildReport, plan *DeltaPlan, repoReasons map[string]string) {
	if report == nil || plan == nil {
		return
	}

	if plan.Decision == DeltaDecisionPartial {
		report.DeltaDecision = "partial"
		report.DeltaChangedRepos = append([]string{}, plan.ChangedRepos...)
	} else {
		report.DeltaDecision = "full"
	}

	if report.DeltaRepoReasons == nil {
		report.DeltaRepoReasons = map[string]string{}
	}
	maps.Copy(report.DeltaRepoReasons, repoReasons)
}

// RecomputeGlobalDocHash recomputes the global doc-files hash for partial builds by
// unioning doc paths from unchanged repos with doc paths from changed repos.
//
// If deletion detection is enabled, unchanged repos are scanned on disk to refresh
// persisted doc path lists before computing the union hash.
func (m *Manager) RecomputeGlobalDocHash(
	report *models.BuildReport,
	plan *DeltaPlan,
	meta state.RepositoryMetadataStore,
	repos []cfg.Repository,
	workspace string,
	cfgAny *cfg.Config,
) (int, error) {
	if report == nil || plan == nil || plan.Decision != DeltaDecisionPartial || report.DocFilesHash == "" {
		return 0, nil
	}
	if meta == nil {
		return 0, nil
	}

	changedSet := make(map[string]struct{}, len(plan.ChangedRepos))
	for _, u := range plan.ChangedRepos {
		changedSet[u] = struct{}{}
	}

	allPaths := make([]string, 0, 2048)
	deletionsDetected := 0

	for i := range repos {
		repo := &repos[i]
		paths := meta.GetRepoDocFilePaths(repo.URL)

		// For unchanged repos, optionally detect deletions by scanning workspace clone.
		if _, isChanged := changedSet[repo.URL]; !isChanged && workspace != "" && cfgAny != nil && cfgAny.Build.DetectDeletions {
			freshPaths, deleted, err := m.scanForDeletions(*repo, workspace, paths)
			if err == nil {
				if len(freshPaths) != len(paths) {
					meta.SetRepoDocFilePaths(repo.URL, freshPaths)
					meta.SetRepoDocFilesHash(repo.URL, m.computePathsHash(freshPaths))
					paths = freshPaths
					deletionsDetected += deleted
				}
			}
		}

		if len(paths) > 0 {
			allPaths = append(allPaths, paths...)
		}
	}

	if len(allPaths) > 0 {
		sort.Strings(allPaths)
		report.DocFilesHash = m.computePathsHash(allPaths)
	}

	return deletionsDetected, nil
}

func (m *Manager) scanForDeletions(repo cfg.Repository, workspace string, persistedPaths []string) ([]string, int, error) {
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
			if werr != nil {
				return werr
			}
			if d == nil || d.IsDir() {
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

func (m *Manager) computePathsHash(paths []string) string {
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
