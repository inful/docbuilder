package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// LEGACY: buildContext encapsulates all mutable state for a single build execution.
// As of Dec 2025, the daemon uses BuildServiceAdapter wrapping build.DefaultBuildService.
// This type is retained for associated tests and as fallback via SiteBuilder.
// See ARCHITECTURE_MIGRATION_PLAN.md Phase D for migration status.
// TODO: Remove once BuildServiceAdapter is fully validated in production.
//
// buildContext enables a staged pipeline where each stage can access shared information
// without repeatedly recomputing derivations from the input job metadata.
type buildContext struct {
	ctx                     context.Context
	job                     *BuildJob
	cfg                     *cfg.Config      // defensive copy
	repos                   []cfg.Repository // repositories to process (possibly filtered later for partial builds)
	outDir                  string
	workspace               string
	generator               *hugo.Generator
	stateMgr                services.StateManager // properly typed state manager interface
	skipReport              *hugo.BuildReport
	deltaPlan               *DeltaPlan
	postPersistOrchestrator PostPersistOrchestrator
}

func newBuildContext(ctx context.Context, job *BuildJob) (*buildContext, error) {
	if job == nil {
		return nil, fmt.Errorf("nil job passed to builder")
	}
	// Get config from TypedMeta
	var rawCfg *cfg.Config
	if job.TypedMeta != nil && job.TypedMeta.V2Config != nil {
		rawCfg = job.TypedMeta.V2Config
	}
	if rawCfg == nil {
		return nil, fmt.Errorf("missing v2 configuration in job metadata")
	}
	// Get repositories from TypedMeta
	var repos []cfg.Repository
	if job.TypedMeta != nil && len(job.TypedMeta.Repositories) > 0 {
		repos = job.TypedMeta.Repositories
	}
	cpy := *rawCfg
	cpy.Repositories = repos
	outDir := cpy.Output.Directory
	if outDir == "" {
		outDir = "./site"
	}
	// Create generator and extract state manager with proper type assertion
	gen := hugo.NewGenerator(&cpy, outDir)
	var stateMgr services.StateManager
	// Get state manager from TypedMeta
	if job.TypedMeta != nil && job.TypedMeta.StateManager != nil {
		stateMgr = job.TypedMeta.StateManager
	}
	// Also update generator if state manager supports document operations
	if stateMgr != nil {
		if sm, ok2 := stateMgr.(interface {
			SetRepoDocumentCount(string, int)
			SetRepoDocFilesHash(string, string)
		}); ok2 {
			gen = gen.WithStateManager(sm)
		}
	}
	if stateMgr != nil {
		ensureRepositoriesInitialized(stateMgr, repos)
	}

	return &buildContext{
		ctx:                     ctx,
		job:                     job,
		cfg:                     &cpy,
		repos:                   repos,
		outDir:                  outDir,
		generator:               gen,
		stateMgr:                stateMgr,
		postPersistOrchestrator: NewPostPersistOrchestrator(),
	}, nil
}

// ensureRepositoriesInitialized proactively registers repository state entries when the state manager supports it.
func ensureRepositoriesInitialized(stateMgr services.StateManager, repos []cfg.Repository) {
	initializer, ok := stateMgr.(interface {
		EnsureRepositoryState(string, string, string)
	})
	if !ok {
		return
	}
	for _, repo := range repos {
		initializer.EnsureRepositoryState(repo.URL, repo.Name, repo.Branch)
	}
}

// stageEarlySkip executes the SkipEvaluator prior to any destructive filesystem actions.
// nolint:unparam // This stage currently never returns an error.
func (bc *buildContext) stageEarlySkip() error {
	if bc.cfg == nil || len(bc.repos) == 0 || !bc.cfg.Build.SkipIfUnchanged {
		return nil
	}
	if sm, ok := bc.stateMgr.(SkipStateAccess); ok && sm != nil {
		if rep, skipped := NewSkipEvaluator(bc.outDir, sm, bc.generator).Evaluate(bc.repos); skipped {
			bc.skipReport = rep
			return nil
		}
	}
	return nil
}

// stageDeltaAnalysis runs the delta analyzer scaffold. Future work will refine repos slice.
// nolint:unparam // This stage currently never returns an error.
func (bc *buildContext) stageDeltaAnalysis() error {
	if bc.skipReport != nil || len(bc.repos) == 0 {
		return nil
	}
	if st, ok := bc.stateMgr.(interface {
		GetLastGlobalDocFilesHash() string
		GetRepoDocFilesHash(string) string
		GetRepoLastCommit(string) string
	}); ok && st != nil {
		plan := NewDeltaAnalyzer(st).WithWorkspace(bc.cfg.Build.WorkspaceDir).Analyze(bc.generator.ComputeConfigHashForPersistence(), bc.repos)
		bc.deltaPlan = &plan
		// Expose per-repo reasons (only for changed repos) into job metadata for later report population
		if plan.RepoReasons != nil {
			m := make(map[string]string, len(plan.RepoReasons))
			for k, v := range plan.RepoReasons {
				m[k] = v
			}
			// Store in TypedMeta
			EnsureTypedMeta(bc.job).DeltaRepoReasons = m
		}
		// Get metrics collector from TypedMeta
		var mc *MetricsCollector
		if bc.job.TypedMeta != nil && bc.job.TypedMeta.MetricsCollector != nil {
			mc = bc.job.TypedMeta.MetricsCollector
		}
		if mc != nil {
			if plan.Decision == DeltaDecisionPartial {
				mc.IncrementCounter("builds_partial")
			} else {
				mc.IncrementCounter("builds_full")
			}
		}
		if bc.skipReport != nil { // edge case: skip already decided but still populate delta metadata
			if plan.Decision == DeltaDecisionPartial {
				bc.skipReport.DeltaDecision = "partial"
				bc.skipReport.DeltaChangedRepos = append([]string{}, plan.ChangedRepos...)
			} else {
				bc.skipReport.DeltaDecision = "full"
			}
		}
		if plan.Decision == DeltaDecisionPartial && len(plan.ChangedRepos) > 0 {
			// Prune repositories to only those needing rebuild.
			changedSet := make(map[string]struct{}, len(plan.ChangedRepos))
			for _, u := range plan.ChangedRepos {
				changedSet[u] = struct{}{}
			}
			filtered := make([]cfg.Repository, 0, len(plan.ChangedRepos))
			for _, r := range bc.repos {
				if _, ok := changedSet[r.URL]; ok {
					filtered = append(filtered, r)
				}
			}
			slog.Info("Applying partial rebuild repo pruning", "before", len(bc.repos), "after", len(filtered), "reason", plan.Reason)
			bc.repos = filtered
		} else if plan.Decision == DeltaDecisionPartial && len(plan.ChangedRepos) == 0 {
			slog.Warn("DeltaAnalyzer returned partial decision with empty repo set; ignoring")
		}
	}
	return nil
}

// stagePrepareFilesystem cleans output (if configured) and prepares workspace directory.
func (bc *buildContext) stagePrepareFilesystem() error {
	if bc.skipReport != nil {
		return nil
	}
	if bc.cfg.Output.Clean {
		if err := os.RemoveAll(bc.outDir); err != nil {
			slog.Warn("Failed to clean output directory", "dir", bc.outDir, "error", err)
		}
	}
	if err := os.MkdirAll(bc.outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	ws := bc.cfg.Build.WorkspaceDir
	if ws == "" {
		strategy := bc.cfg.Build.CloneStrategy
		if strategy == "" {
			strategy = cfg.CloneStrategyFresh
		}
		repoCache := ""
		if bc.cfg.Daemon != nil {
			repoCache = bc.cfg.Daemon.Storage.RepoCacheDir
		}
		if repoCache != "" {
			repoCache = filepath.Clean(repoCache)
		}
		if strategy == cfg.CloneStrategyFresh {
			ws = filepath.Join(bc.outDir, "_workspace")
		} else if repoCache != "" {
			ws = filepath.Join(repoCache, "working")
			slog.Info("Deriving workspace from repo_cache_dir", "repo_cache_dir", repoCache, "workspace", ws, "strategy", strategy)
		} else {
			ws = filepath.Clean(bc.outDir + "-workspace")
		}
	}
	if bc.cfg.Output.Clean && bc.cfg.Build.CloneStrategy == cfg.CloneStrategyFresh {
		if rel, err := filepath.Rel(bc.outDir, ws); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			if err := os.RemoveAll(ws); err != nil {
				slog.Warn("Failed to clean workspace directory", "dir", ws, "error", err)
			}
		}
	} else if bc.cfg.Build.CloneStrategy != cfg.CloneStrategyFresh && bc.cfg.Output.Clean {
		slog.Info("Preserving workspace for incremental updates", "dir", ws, "strategy", bc.cfg.Build.CloneStrategy)
	}
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	slog.Info("Using workspace directory", "dir", ws, "configured", bc.cfg.Build.WorkspaceDir != "")
	bc.workspace = ws
	return nil
}

// stageGenerateSite performs the (currently always full) Hugo generation.
func (bc *buildContext) stageGenerateSite() (*hugo.BuildReport, error) {
	if bc.skipReport != nil {
		return bc.skipReport, nil
	}
	report, err := bc.generator.GenerateFullSite(bc.ctx, bc.repos, bc.workspace)
	if err != nil {
		slog.Error("Full site generation error", "error", err)
	}
	// Attempt livereload injection (non-fatal)
	if ierr := bc.stageInjectLiveReload(); ierr != nil {
		slog.Debug("livereload injection skipped", "error", ierr)
	}
	return report, err
}

// stageInjectLiveReload post-processes generated HTML files inserting the livereload.js script tag
// right before </body> for development convenience when live reload is enabled.
func (bc *buildContext) stageInjectLiveReload() error {
	if bc.cfg == nil || !bc.cfg.Build.LiveReload {
		return nil
	}
	// Determine output public directory (after generation promotion)
	outRoot := bc.outDir
	// Prefer public/ if it exists (Hugo publishDir default)
	pub := filepath.Join(outRoot, "public")
	if fi, err := os.Stat(pub); err == nil && fi.IsDir() {
		outRoot = pub
	}
	inject := []byte("<script src=\"/livereload.js\"></script>")
	err := filepath.WalkDir(outRoot, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil || d.IsDir() {
			return nil //nolint:nilerr // ignore per-file traversal errors; best-effort inject
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".html") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil //nolint:nilerr // skip unreadable files; livereload injection is optional
		}
		if bytes.Contains(b, []byte("/livereload.js")) {
			return nil
		}
		// find closing body tag (case-insensitive)
		lower := strings.ToLower(string(b))
		idx := strings.LastIndex(lower, "</body>")
		if idx == -1 {
			return nil
		}
		// build new content
		var out bytes.Buffer
		out.Write(b[:idx])
		out.WriteByte('\n')
		out.Write(inject)
		out.WriteByte('\n')
		out.Write(b[idx:])
		// Public site asset; readable by others is intended.
		if werr := os.WriteFile(p, out.Bytes(), 0o644); werr != nil { //nolint:gosec // public HTML output, non-sensitive
			slog.Debug("livereload inject write failed", "file", p, "error", werr)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("livereload injection walk: %w", err)
	}
	return nil
}

// stagePostPersist updates metrics and state persistence after generation or skip.
func (bc *buildContext) stagePostPersist(report *hugo.BuildReport, genErr error) error {
	// Ensure orchestrator is initialized
	if bc.postPersistOrchestrator == nil {
		bc.postPersistOrchestrator = NewPostPersistOrchestrator()
	}

	context := &PostPersistContext{
		DeltaPlan:  bc.deltaPlan,
		Job:        bc.job,
		StateMgr:   bc.stateMgr,
		Workspace:  bc.workspace,
		OutDir:     bc.outDir,
		Config:     bc.cfg,
		Generator:  bc.generator,
		Repos:      bc.repos,
		SkipReport: bc.skipReport,
	}

	return bc.postPersistOrchestrator.ExecutePostPersistStage(report, genErr, context)
}
