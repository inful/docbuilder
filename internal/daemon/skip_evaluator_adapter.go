package daemon

import (
	"context"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// skipEvaluatorAdapter adapts the typed daemon.SkipEvaluator to the generic build.SkipEvaluator interface.
type skipEvaluatorAdapter struct {
	inner *SkipEvaluator
}

// Evaluate implements build.SkipEvaluator by converting types.
func (a *skipEvaluatorAdapter) Evaluate(ctx context.Context, repos []any) (report any, canSkip bool) {
	if a.inner == nil {
		return nil, false
	}

	// Convert []any to []cfg.Repository
	typedRepos := make([]cfg.Repository, 0, len(repos))
	for _, r := range repos {
		if repo, ok := r.(cfg.Repository); ok {
			typedRepos = append(typedRepos, repo)
		} else {
			// Type mismatch - cannot skip
			return nil, false
		}
	}

	// Call typed evaluator
	buildReport, canSkip := a.inner.Evaluate(ctx, typedRepos)

	// Return as any
	return buildReport, canSkip
}
