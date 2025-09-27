package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
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
	if job == nil {
		return nil, fmt.Errorf("nil job passed to builder")
	}
	rawCfg, ok := job.Metadata["v2_config"].(*cfg.Config)
	if !ok || rawCfg == nil {
		return nil, fmt.Errorf("missing v2 configuration in job metadata")
	}

	// Derive repositories slice (best-effort typed extraction)
	reposAny, ok := job.Metadata["repositories"].([]cfg.Repository)
	if !ok {
		if ra, ok2 := job.Metadata["repositories"].([]interface{}); ok2 {
			casted := make([]cfg.Repository, 0, len(ra))
			for _, v := range ra {
				if r, ok3 := v.(cfg.Repository); ok3 {
					casted = append(casted, r)
				}
			}
			reposAny = casted
		}
	}

	// Defensive copy of config to avoid shared mutation across concurrent builds
	cloneCfg := *rawCfg
	cloneCfg.Repositories = reposAny

	outDir := cloneCfg.Output.Directory
	if outDir == "" {
		outDir = "./site"
	}

	if cloneCfg.Output.Clean {
		if err := os.RemoveAll(outDir); err != nil {
			slog.Warn("Failed to clean output directory", "dir", outDir, "error", err)
		}
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	workspaceDir := cloneCfg.Build.WorkspaceDir
	if workspaceDir == "" {
		workspaceDir = filepath.Join(outDir, "_workspace")
	}
	// Only auto-clean workspace if it resides under the output directory and output.clean is enabled.
	if cloneCfg.Output.Clean {
		if rel, err := filepath.Rel(outDir, workspaceDir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			// Ensure directory fresh
			if err := os.RemoveAll(workspaceDir); err != nil {
				slog.Warn("Failed to clean workspace directory", "dir", workspaceDir, "error", err)
			}
		}
	}
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	slog.Info("Using workspace directory", "dir", workspaceDir, "configured", cloneCfg.Build.WorkspaceDir != "")

	gen := hugo.NewGenerator(&cloneCfg, outDir)
	// Recorder is optionally injected earlier by queue/daemon (prometheus tag variant).
	if err := os.Setenv("DOCBUILDER_RUN_HUGO", "1"); err != nil {
		slog.Warn("Failed to set DOCBUILDER_RUN_HUGO env", "error", err)
	}
	report, err := gen.GenerateFullSite(ctx, reposAny, workspaceDir)
	if err != nil {
		slog.Error("Full site generation error", "error", err)
	}
	return report, err
}
