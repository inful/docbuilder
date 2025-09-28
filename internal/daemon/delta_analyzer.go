package daemon

import (
    "log/slog"
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
    ChangedRepos []string // repositories requiring rebuild (empty implies none or full rebuild depending on Decision)
    Reason       string   // human readable explanation of decision
}

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
}

// NewDeltaAnalyzer constructs a new analyzer instance.
func NewDeltaAnalyzer(st DeltaStateAccess) *DeltaAnalyzer { return &DeltaAnalyzer{state: st} }

// Analyze returns a DeltaPlan describing whether a partial rebuild could be attempted.
// currentConfigHash: hash of current configuration (same value used by skip logic)
// repos: repositories requested for this build.
func (da *DeltaAnalyzer) Analyze(currentConfigHash string, repos []cfg.Repository) DeltaPlan {
    if da == nil || da.state == nil || len(repos) == 0 {
        return DeltaPlan{Decision: DeltaDecisionFull, Reason: "insufficient_context"}
    }

    // Placeholder heuristic (future work):
    // 1. If config hash changed in ways affecting site structure -> full
    // 2. If only a subset of repositories have doc_files_hash changes -> partial
    // 3. If any repository missing prior doc hash or commit -> full
    // 4. If global hash unchanged -> candidate partial (currently still full until implemented)

    // For now emit a single structured log for observability.
    slog.Debug("DeltaAnalyzer: partial build path not yet implemented; falling back to full rebuild")
    return DeltaPlan{Decision: DeltaDecisionFull, Reason: "not_implemented"}
}
