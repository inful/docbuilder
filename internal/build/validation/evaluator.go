package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// SkipEvaluator decides whether a build can be safely skipped based on
// persisted state + prior build report + filesystem probes using validation rules.
type SkipEvaluator struct {
	outDir    string
	state     SkipStateAccess
	generator *hugo.Generator
	rules     *RuleChain
}

// NewSkipEvaluator constructs a new evaluator with the standard validation rules.
func NewSkipEvaluator(outDir string, st SkipStateAccess, gen *hugo.Generator) *SkipEvaluator {
	// Rules are created on-demand in the evaluator since PreviousReportRule
	// requires special handling to populate the context
	return &SkipEvaluator{
		outDir:    outDir,
		state:     st,
		generator: gen,
		rules:     nil, // Rules created dynamically
	}
}

// Evaluate returns (report, true) when the build can be skipped, otherwise (nil, false).
// It never returns an error; corrupt/missing data simply disables the skip and a full rebuild proceeds.
func (se *SkipEvaluator) Evaluate(repos []cfg.Repository) (*hugo.BuildReport, bool) {
	ctx := Context{
		OutDir:    se.outDir,
		State:     se.state,
		Generator: se.generator,
		Repos:     repos,
		Logger:    slog.Default(),
	}

	// Special handling for PreviousReportRule since it needs to populate context
	if !se.validateAndPopulateContext(&ctx) {
		return nil, false
	}

	// Execute remaining validation rules
	remainingRules := NewRuleChain(
		ContentIntegrityRule{},
		GlobalDocHashRule{},
		PerRepoDocHashRule{},
		CommitMetadataRule{},
	)

	result := remainingRules.Validate(ctx)
	if !result.Passed {
		return nil, false
	}

	// All validation rules passed - construct skip report
	return se.constructSkipReport(ctx)
}

// validateAndPopulateContext runs the initial validation rules and populates the context.
func (se *SkipEvaluator) validateAndPopulateContext(ctx *Context) bool {
	initialRules := NewRuleChain(
		BasicPrerequisitesRule{},
		ConfigHashRule{},
		PublicDirectoryRule{},
	)

	result := initialRules.Validate(*ctx)
	if !result.Passed {
		return false
	}

	// Handle PreviousReportRule separately to populate context
	return se.loadPreviousReport(ctx)
}

// loadPreviousReport loads and validates the previous build report, populating the context.
func (se *SkipEvaluator) loadPreviousReport(ctx *Context) bool {
	prevPath := filepath.Join(ctx.OutDir, "build-report.json")
	// #nosec G304 - prevPath is internal, OutDir is controlled by application
	data, err := os.ReadFile(prevPath)
	if err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Warn("Skip validation failed", "rule", "previous_report", "reason", "no previous build report found")
		}
		return false
	}

	// Validate checksum if stored
	sum := sha256.Sum256(data)
	currentSum := hex.EncodeToString(sum[:])
	if stored := ctx.State.GetLastReportChecksum(); stored != "" && stored != currentSum {
		if ctx.Logger != nil {
			ctx.Logger.Warn("Skip validation failed", "rule", "previous_report", "reason", "previous build report checksum mismatch")
		}
		return false
	}

	// Parse the report
	var report PreviousReport
	if err := json.Unmarshal(data, &report); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Warn("Skip validation failed", "rule", "previous_report", "reason", "failed to parse previous build report")
		}
		return false
	}

	// Store parsed data in context for other rules
	report.RawData = data
	ctx.PrevReport = &report

	return true
}

// constructSkipReport creates and persists a skip report based on the previous report data.
func (se *SkipEvaluator) constructSkipReport(ctx Context) (*hugo.BuildReport, bool) {
	if ctx.PrevReport == nil {
		slog.Warn("Cannot construct skip report: no previous report data")
		return nil, false
	}

	// Create skip report reusing prior counts
	report := &hugo.BuildReport{
		SchemaVersion: 1,
		Start:         time.Now(),
		End:           time.Now(),
		SkipReason:    "no_changes",
		Outcome:       hugo.OutcomeSuccess,
		Repositories:  ctx.PrevReport.Repositories,
		Files:         ctx.PrevReport.Files,
		RenderedPages: ctx.PrevReport.RenderedPages,
		DocFilesHash:  ctx.PrevReport.DocFilesHash,
	}

	// Persist the skip report
	if err := report.Persist(se.outDir); err != nil {
		slog.Warn("Failed to persist skip report", "error", err)
		return report, true // Still return success even if persistence fails
	}

	// Update state with checksums
	se.updateStateAfterSkip(ctx, report)

	slog.Info("Skipping build (unchanged) without cleaning output",
		"repos", report.Repositories,
		"files", report.Files,
		"content_probe", "ok")

	return report, true
}

// updateStateAfterSkip updates the state manager with current checksums after a successful skip.
func (se *SkipEvaluator) updateStateAfterSkip(ctx Context, report *hugo.BuildReport) {
	// Update report checksum
	if ctx.PrevReport != nil && len(ctx.PrevReport.RawData) > 0 {
		prevPath := filepath.Join(se.outDir, "build-report.json")
		// #nosec G304 - prevPath is internal, outDir is controlled by application
		if rb, err := os.ReadFile(prevPath); err == nil {
			hs := sha256.Sum256(rb)
			se.state.SetLastReportChecksum(hex.EncodeToString(hs[:]))
		}
	}

	// Update global doc files hash
	if report.DocFilesHash != "" {
		se.state.SetLastGlobalDocFilesHash(report.DocFilesHash)
	}
}
