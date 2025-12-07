package daemon

import (
	"git.home.luguber.info/inful/docbuilder/internal/build/delta"
)

// Type aliases for backward compatibility with daemon code that uses delta analysis.
// The canonical implementation is now in internal/build/delta/.
type (
	DeltaDecision    = delta.DeltaDecision
	DeltaPlan        = delta.DeltaPlan
	DeltaStateAccess = delta.DeltaStateAccess
	DeltaAnalyzer    = delta.DeltaAnalyzer
)

// Re-export constants
const (
	DeltaDecisionFull       = delta.DeltaDecisionFull
	DeltaDecisionPartial    = delta.DeltaDecisionPartial
	RepoReasonUnknown       = delta.RepoReasonUnknown
	RepoReasonQuickHashDiff = delta.RepoReasonQuickHashDiff
	RepoReasonUnchanged     = delta.RepoReasonUnchanged
)

// NewDeltaAnalyzer is a thin wrapper for backward compatibility.
func NewDeltaAnalyzer(st DeltaStateAccess) *DeltaAnalyzer {
	return delta.NewDeltaAnalyzer(st)
}
