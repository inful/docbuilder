package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// Builder defines an abstraction for executing a build job and returning a BuildReport.
// It decouples queue execution from the concrete site generation pipeline, enabling
// future swapping (e.g., distributed builders, parallel clone variants, dry-run builder).
type Builder interface {
	Build(ctx context.Context, job *BuildJob) (*hugo.BuildReport, error)
}

// SiteBuilder is the default implementation that uses the existing Hugo pipeline.
type SiteBuilder struct{}

// NewSiteBuilder returns a new SiteBuilder instance.
func NewSiteBuilder() *SiteBuilder { return &SiteBuilder{} }

// Build executes the full site generation for the given job.
// Expected metadata inputs (populated by daemon enqueue logic):
//   - v2_config: *config.Config (base configuration)
//   - repositories: []config.Repository (explicit repos to process; may be discovery result)
//
// Metrics collector and other optional metadata keys are passed through unmodified.
func (sb *SiteBuilder) Build(ctx context.Context, job *BuildJob) (*hugo.BuildReport, error) {
	bc, err := newBuildContext(ctx, job)
	if err != nil {
		return nil, err
	}
	// Stage: early skip (pre-clean) --------------------------------------
	if err := bc.stageEarlySkip(); err != nil {
		slog.Warn("Skip evaluation error (continuing with build)", "error", err)
	}
	if bc.skipReport != nil { // fast exit
		return bc.skipReport, nil
	}
	// Stage: delta analysis (currently scaffold) -------------------------
	if err := bc.stageDeltaAnalysis(); err != nil {
		slog.Warn("Delta analysis failed; full rebuild fallback", "error", err)
	}
	// Stage: filesystem prep (clean + workspace) -------------------------
	if err := bc.stagePrepareFilesystem(); err != nil {
		return nil, err
	}
	// Future: conditional partial execution based on bc.deltaPlan --------
	// Stage: full (or partial) site generation ---------------------------
	report, genErr := bc.stageGenerateSite()
	// Stage: metrics & state persistence ---------------------------------
	if pErr := bc.stagePostPersist(report, genErr); pErr != nil {
		slog.Warn("Post-build persistence issue", "error", pErr)
	}
	return report, genErr
}

// hugoReadRepoHead duplicates internal hugo.readRepoHead without exporting the entire build package surface.
func hugoReadRepoHead(repoPath string) (string, error) {
	headPath := filepath.Join(repoPath, ".git", "HEAD")
	b, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(b))
	if strings.HasPrefix(line, "ref:") {
		ref := strings.TrimSpace(strings.TrimPrefix(line, "ref:"))
		refPath := filepath.Join(repoPath, ".git", filepath.FromSlash(ref))
		if rb, rerr := os.ReadFile(refPath); rerr == nil {
			return strings.TrimSpace(string(rb)), nil
		}
	}
	return line, nil
}
