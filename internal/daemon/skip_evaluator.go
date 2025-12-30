package daemon

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/build/validation"
	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// SkipStateAccess encapsulates the subset of state manager methods required to evaluate a skip.
// This interface is kept for backward compatibility.
type SkipStateAccess = validation.SkipStateAccess

// SkipEvaluator decides whether a build can be safely skipped based on
// persisted state + prior build report + filesystem probes.
// This is now a thin wrapper around the validation-based evaluator.
type SkipEvaluator struct {
	validator *validation.SkipEvaluator
}

// NewSkipEvaluator constructs a new evaluator.
func NewSkipEvaluator(outDir string, st SkipStateAccess, gen *hugo.Generator) *SkipEvaluator {
	return &SkipEvaluator{
		validator: validation.NewSkipEvaluator(outDir, st, gen),
	}
}

// Evaluate returns (report, true) when the build can be skipped, otherwise (nil, false).
// It never returns an error; corrupt/missing data simply disables the skip and a full rebuild proceeds.
func (se *SkipEvaluator) Evaluate(ctx context.Context, repos []cfg.Repository) (*hugo.BuildReport, bool) {
	return se.validator.Evaluate(ctx, repos)
}
