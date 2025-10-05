package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// DeltaDecision represents the analyzer's chosen build strategy.
type DeltaDecision int

const (
	// DeltaDecisionFull indicates a full rebuild of all repositories is required.
	DeltaDecisionFull DeltaDecision = iota
	// DeltaDecisionPartial indicates a partial rebuild (subset of repos / docs) is possible.
	DeltaDecisionPartial
)

// DeltaPlan is the output of DeltaAnalyzer.Analyze.
// For now this is scaffolding: logic will evolve to include per-file deltas, removed docs, etc.
type DeltaPlan struct {
	Decision     DeltaDecision
	ChangedRepos []string          // repositories requiring rebuild (empty implies none or full rebuild depending on Decision)
	Reason       string            // human readable explanation of decision
	RepoReasons  map[string]string // per-repo inclusion reason (see RepoReason* constants)
}

// RepoReason values define why a repository is (or is not) included in a delta/partial build.
// Maintained as constants to prevent string drift across analyzer, reporting, and tests.
const (
	RepoReasonUnknown       = "unknown"         // insufficient persisted metadata (hash and/or commit missing)
	RepoReasonQuickHashDiff = "quick_hash_diff" // workspace quick hash differed from stored per-repo doc files hash
	RepoReasonUnchanged     = "unchanged"       // determined unchanged (metadata present; quick hash parity or optimistic default)
)

// DeltaStateAccess is the narrow interface DeltaAnalyzer needs from state.
// (Intentionally minimal for easier testing and future extension.)
type DeltaStateAccess interface {
	GetLastGlobalDocFilesHash() string
	GetRepoDocFilesHash(string) string
	GetRepoLastCommit(string) string
}

// DeltaAnalyzer compares current intent (config hash + requested repos) with persisted state
// to determine if a partial rebuild is feasible. Initial implementation always returns Full.
type DeltaAnalyzer struct {
	state DeltaStateAccess
	// quickHashRoots provides root path(s) to look for cloned repositories (workspace dir). Optional.
	workspaceDir string
}

// NewDeltaAnalyzer constructs a new analyzer instance.
func NewDeltaAnalyzer(st DeltaStateAccess) *DeltaAnalyzer { return &DeltaAnalyzer{state: st} }

// WithWorkspace configures the analyzer with a workspace directory containing cloned repositories.
// This enables a lightweight pre-discovery doc path hash by scanning known doc roots (default: 'docs', 'documentation').
func (da *DeltaAnalyzer) WithWorkspace(dir string) *DeltaAnalyzer {
	if da != nil {
		da.workspaceDir = dir
	}
	return da
}

// computeQuickRepoHash walks a small set of known doc directories inside the repository returning
// a stable hash of relative file paths. Intended as a pre-discovery approximation to detect changes quickly.
func (da *DeltaAnalyzer) computeQuickRepoHash(repoName string) string {
	if da.workspaceDir == "" || repoName == "" {
		return ""
	}
	root := filepath.Join(da.workspaceDir, repoName)
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return ""
	}
	docRoots := []string{"docs", "documentation"}
	paths := make([]string, 0, 32)
	for _, dr := range docRoots {
		base := filepath.Join(root, dr)
		if fi, err := os.Stat(base); err == nil && fi.IsDir() {
			if walkErr := filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
				if err != nil || d == nil || d.IsDir() {
					return nil
				}
				name := d.Name()
				ln := strings.ToLower(name)
				if strings.HasSuffix(ln, ".md") || strings.HasSuffix(ln, ".markdown") {
					rel, rerr := filepath.Rel(root, p)
					if rerr == nil {
						paths = append(paths, filepath.ToSlash(rel))
					}
				}
				return nil
			}); walkErr != nil {
				// On walk error, return empty quick hash (acts as unknown); log at info for visibility
				slog.Info("quick hash walk error", "repo", repoName, "root", base, "err", walkErr)
				return ""
			}
		}
	}
	if len(paths) == 0 {
		return ""
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Analyze returns a DeltaPlan describing whether a partial rebuild could be attempted.
// currentConfigHash: hash of current configuration (same value used by skip logic)
// repos: repositories requested for this build.
func (da *DeltaAnalyzer) Analyze(currentConfigHash string, repos []cfg.Repository) DeltaPlan {
	if da == nil || da.state == nil || len(repos) == 0 {
		return DeltaPlan{Decision: DeltaDecisionFull, Reason: "insufficient_context"}
	}

	changed := make([]string, 0, len(repos))
	unknown := 0
	reasons := make(map[string]string, len(repos))
	for _, r := range repos {
		docHash := da.state.GetRepoDocFilesHash(r.URL)
		commit := da.state.GetRepoLastCommit(r.URL)
		// Case 1: incomplete metadata forces rebuild
		if docHash == "" || commit == "" {
			if docHash == "" && commit == "" {
				unknown++
			}
			reasons[r.URL] = RepoReasonUnknown
			changed = append(changed, r.URL)
			continue
		}
		// Case 2: attempt quick hash diff if workspace available
		if da.workspaceDir != "" {
			if quick := da.computeQuickRepoHash(r.Name); quick != "" {
				if quick != docHash { // mismatch => changed
					reasons[r.URL] = RepoReasonQuickHashDiff
					changed = append(changed, r.URL)
					continue
				}
				// match => unchanged
				reasons[r.URL] = RepoReasonUnchanged
				continue
			}
		}
		// Case 3: have metadata but no quick hash available; treat as unchanged (optimistic) for now
		reasons[r.URL] = RepoReasonUnchanged
	}

	if len(changed) == 0 {
		return DeltaPlan{Decision: DeltaDecisionFull, Reason: "no_detected_repo_change"}
	}
	if len(changed) == len(repos) {
		reason := "all_repos_changed"
		if unknown == len(repos) {
			reason = "all_repos_unknown_state"
		}
		return DeltaPlan{Decision: DeltaDecisionFull, Reason: reason, RepoReasons: reasons}
	}
	slog.Info("DeltaAnalyzer: partial rebuild candidate", "changed_repos", strings.Join(changed, ","), "total_repos", len(repos))
	return DeltaPlan{Decision: DeltaDecisionPartial, ChangedRepos: changed, Reason: "subset_changed", RepoReasons: reasons}
}
