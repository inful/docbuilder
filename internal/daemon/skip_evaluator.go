package daemon

import (
	"context"

	"git.home.luguber.info/inful/docbuilder/internal/build/validation"
	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// SkipStateAccess encapsulates the subset of state manager methods required to evaluate a skip.
// This interface is kept for backward compatibility.
type SkipStateAccess = validation.SkipStateAccess

// SkipEvaluator decides whether a build can be safely skipped based on
// persisted state + prior build report + filesystem probes.
// This is a thin wrapper around the validation-based evaluator; it owns the
// responsibility of detecting the current Hugo version and forwarding it.
type SkipEvaluator struct {
	validator *validation.SkipEvaluator
}

// NewSkipEvaluator constructs a new evaluator. The current Hugo version is
// detected up front and passed into the validator; empty string when Hugo
// is unavailable (validation will treat that as "no Hugo used previously"
// if the previous report's HugoVersion is also empty).
func NewSkipEvaluator(outDir string, st SkipStateAccess, gen *hugo.Generator) *SkipEvaluator {
	hugoVersion := ""
	if gen != nil {
		hugoVersion = hugo.DetectHugoVersion(context.Background())
	}
	return &SkipEvaluator{
		validator: validation.NewSkipEvaluator(outDir, st, gen, hugoVersion),
	}
}

// Evaluate returns (report, true) when the build can be skipped, otherwise (nil, false).
// It never returns an error; corrupt/missing data simply disables the skip and a full rebuild proceeds.
func (se *SkipEvaluator) Evaluate(ctx context.Context, repos []cfg.Repository) (*models.BuildReport, bool) {
	return se.validator.Evaluate(ctx, repos)
}
