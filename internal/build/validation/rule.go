package validation

import (
	"context"
	"log/slog"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// SkipStateAccess encapsulates the subset of state manager methods required for validation.
type SkipStateAccess interface {
	GetRepoLastCommit(string) string
	GetLastConfigHash() string
	GetLastReportChecksum() string
	SetLastReportChecksum(string)
	GetRepoDocFilesHash(string) string
	GetLastGlobalDocFilesHash() string
	SetLastGlobalDocFilesHash(string)
}

// Context contains all the data needed by validation rules.
type Context struct {
	OutDir     string
	State      SkipStateAccess
	Generator  *hugo.Generator
	Repos      []cfg.Repository
	PrevReport *PreviousReport
	Logger     *slog.Logger
}

// PreviousReport holds parsed data from the previous build report.
type PreviousReport struct {
	Repositories      int    `json:"repositories"`
	Files             int    `json:"files"`
	RenderedPages     int    `json:"rendered_pages"`
	DocFilesHash      string `json:"doc_files_hash"`
	DocBuilderVersion string `json:"doc_builder_version"`
	HugoVersion       string `json:"hugo_version"`
	RawData           []byte `json:"-"` // original JSON bytes for checksum
}

// Result indicates whether validation passed and provides context.
type Result struct {
	Passed bool
	Reason string // human-readable reason for failure
}

// Success returns a successful validation result.
func Success() Result {
	return Result{Passed: true}
}

// Failure returns a failed validation result with a reason.
func Failure(reason string) Result {
	return Result{Passed: false, Reason: reason}
}

// SkipValidationRule represents a single validation rule for skip evaluation.
type SkipValidationRule interface {
	// Name returns a short identifier for this rule (for logging/debugging).
	Name() string

	// Validate checks if this rule allows skipping the build.
	// Returns Result indicating pass/fail and optional reason.
	Validate(ctx context.Context, vctx Context) Result
}

// RuleChain executes validation rules in sequence, stopping at the first failure.
type RuleChain struct {
	rules []SkipValidationRule
}

// NewRuleChain creates a new rule chain with the given rules.
func NewRuleChain(rules ...SkipValidationRule) *RuleChain {
	return &RuleChain{rules: rules}
}

// Validate executes all rules in order, returning the first failure or success if all pass.
func (rc *RuleChain) Validate(ctx context.Context, vctx Context) Result {
	for _, rule := range rc.rules {
		result := rule.Validate(ctx, vctx)
		if !result.Passed {
			if vctx.Logger != nil {
				vctx.Logger.Warn("Skip validation failed",
					"rule", rule.Name(),
					"reason", result.Reason)
			}
			return result
		}
	}
	return Success()
}
