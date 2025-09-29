package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// buildContext encapsulates all mutable state for a single build execution.
// It enables a staged pipeline where each stage can access shared information
// without repeatedly recomputing derivations from the input job metadata.
type buildContext struct {
	ctx        context.Context
	job        *BuildJob
	cfg        *cfg.Config      // defensive copy
	repos      []cfg.Repository // repositories to process (possibly filtered later for partial builds)
	outDir     string
	workspace  string
	generator  *hugo.Generator
	stateMgr   interface{} // loosely typed; narrowed via on-demand interface assertions
	skipReport *hugo.BuildReport
	deltaPlan  *DeltaPlan
}

func newBuildContext(ctx context.Context, job *BuildJob) (*buildContext, error) {
	if job == nil {
		return nil, fmt.Errorf("nil job passed to builder")
	}
	rawCfg, ok := job.Metadata["v2_config"].(*cfg.Config)
	if !ok || rawCfg == nil {
		return nil, fmt.Errorf("missing v2 configuration in job metadata")
	}
	// Extract repo slice with best-effort conversion
	repos, _ := job.Metadata["repositories"].([]cfg.Repository)
	if repos == nil {
		if ra, ok2 := job.Metadata["repositories"].([]interface{}); ok2 {
			casted := make([]cfg.Repository, 0, len(ra))
			for _, v := range ra {
				if r, ok3 := v.(cfg.Repository); ok3 {
					casted = append(casted, r)
				}
			}
			repos = casted
		}
	}
	cpy := *rawCfg
	cpy.Repositories = repos
	outDir := cpy.Output.Directory
	if outDir == "" {
		outDir = "./site"
	}
	gen := hugo.NewGenerator(&cpy, outDir)
	if smAny, ok := job.Metadata["state_manager"]; ok {
		if sm, ok2 := smAny.(interface {
			SetRepoDocumentCount(string, int)
			SetRepoDocFilesHash(string, string)
		}); ok2 {
			gen = gen.WithStateManager(sm)
		}
	}
	return &buildContext{ctx: ctx, job: job, cfg: &cpy, repos: repos, outDir: outDir, generator: gen, stateMgr: job.Metadata["state_manager"]}, nil
}

// stageEarlySkip executes the SkipEvaluator prior to any destructive filesystem actions.
func (bc *buildContext) stageEarlySkip() error {
	if bc.cfg == nil || len(bc.repos) == 0 || !bc.cfg.Build.SkipIfUnchanged {
		return nil
	}
	if sm, ok := bc.stateMgr.(SkipStateAccess); ok && sm != nil {
		if rep, skipped := NewSkipEvaluator(bc.outDir, sm, bc.generator).Evaluate(bc.repos); skipped {
			bc.skipReport = rep
		}
	}
	return nil
}

// stageDeltaAnalysis runs the delta analyzer scaffold. Future work will refine repos slice.
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
			bc.job.Metadata["delta_repo_reasons"] = m
		}
		if mc, okm := bc.job.Metadata["metrics_collector"].(*MetricsCollector); okm && mc != nil {
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
	if err := os.MkdirAll(bc.outDir, 0755); err != nil {
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
	if err := os.MkdirAll(ws, 0755); err != nil {
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
	if err := os.Setenv("DOCBUILDER_RUN_HUGO", "1"); err != nil {
		slog.Warn("Failed to set DOCBUILDER_RUN_HUGO env", "error", err)
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
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".html") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
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
		if werr := os.WriteFile(p, out.Bytes(), 0o644); werr != nil {
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
	if report == nil {
		return nil
	}
	// Attach delta metadata (if evaluated) before other persistence for external observability.
	if bc.deltaPlan != nil {
		if bc.deltaPlan.Decision == DeltaDecisionPartial {
			report.DeltaDecision = "partial"
			report.DeltaChangedRepos = append([]string{}, bc.deltaPlan.ChangedRepos...)
		} else {
			report.DeltaDecision = "full"
		}
		// Attach per-repo reasons if provided via deltaPlan extension (future-proof: expect optional map in job metadata)
		if report.DeltaRepoReasons == nil {
			report.DeltaRepoReasons = map[string]string{}
		}
		if m, ok := bc.job.Metadata["delta_repo_reasons"].(map[string]string); ok {
			for k, v := range m {
				report.DeltaRepoReasons[k] = v
			}
		}
	}
	// Recompute global doc_files_hash for partial builds by merging unchanged + changed repo path lists.
	if bc.deltaPlan != nil && bc.deltaPlan.Decision == DeltaDecisionPartial && report.DocFilesHash != "" {
		// Support interfaces for path list read/update & per-repo hash update.
		getter, gOK := bc.stateMgr.(interface{ GetRepoDocFilePaths(string) []string })
		setter, sOK := bc.stateMgr.(interface{ SetRepoDocFilePaths(string, []string) })
		hasher, hOK := bc.stateMgr.(interface{ SetRepoDocFilesHash(string, string) })
		if gOK {
			changedSet := map[string]struct{}{}
			for _, u := range bc.deltaPlan.ChangedRepos {
				changedSet[u] = struct{}{}
			}
			orig, _ := bc.job.Metadata["repositories"].([]cfg.Repository)
			allPaths := make([]string, 0, 2048)
			deletionsDetected := 0
			for _, r := range orig {
				paths := getter.GetRepoDocFilePaths(r.URL)
				// For unchanged repos, optionally detect deletions by scanning workspace clone (if available)
				if _, isChanged := changedSet[r.URL]; !isChanged && bc.workspace != "" && bc.cfg != nil && bc.cfg.Build.DetectDeletions {
					repoRoot := filepath.Join(bc.workspace, r.Name)
					if fi, err := os.Stat(repoRoot); err == nil && fi.IsDir() {
						fresh := make([]string, 0, len(paths))
						// Scan doc roots for current files, mirroring quick hash logic
						docRoots := []string{"docs", "documentation"}
						for _, dr := range docRoots {
							base := filepath.Join(repoRoot, dr)
							if sfi, serr := os.Stat(base); serr != nil || !sfi.IsDir() {
								continue
							}
							_ = filepath.WalkDir(base, func(p string, d os.DirEntry, werr error) error {
								if werr != nil || d == nil || d.IsDir() {
									return nil
								}
								ln := strings.ToLower(d.Name())
								if strings.HasSuffix(ln, ".md") || strings.HasSuffix(ln, ".markdown") {
									if rel, rerr := filepath.Rel(repoRoot, p); rerr == nil {
										fresh = append(fresh, filepath.ToSlash(filepath.Join(r.Name, rel)))
									}
								}
								return nil
							})
						}
						// Compare with persisted list; if different (including zero-doc case), persist new list & hash.
						sort.Strings(fresh)
						update := false
						if len(fresh) != len(paths) {
							update = true
						} else {
							for i := range fresh {
								if i >= len(paths) || fresh[i] != paths[i] {
									update = true
									break
								}
							}
						}
						if update {
							if len(fresh) < len(paths) {
								deletionsDetected += len(paths) - len(fresh)
							}
							if sOK {
								setter.SetRepoDocFilePaths(r.URL, fresh)
							}
							if hOK {
								hh := sha256.New()
								for _, p := range fresh {
									hh.Write([]byte(p))
									hh.Write([]byte{0})
								}
								hasher.SetRepoDocFilesHash(r.URL, hex.EncodeToString(hh.Sum(nil)))
							}
							paths = fresh
						}
					}
				}
				if len(paths) > 0 {
					allPaths = append(allPaths, paths...)
				}
			}
			if len(allPaths) > 0 {
				sort.Strings(allPaths)
				h := sha256.New()
				for _, p := range allPaths {
					h.Write([]byte(p))
					h.Write([]byte{0})
				}
				report.DocFilesHash = hex.EncodeToString(h.Sum(nil))
			}
			if deletionsDetected > 0 {
				if mc, ok := bc.job.Metadata["metrics_collector"].(*MetricsCollector); ok && mc != nil {
					for i := 0; i < deletionsDetected; i++ {
						mc.IncrementCounter("doc_deletions_detected")
					}
				}
			}
		}
	}
	// Repo build counters & document counts
	if sm, ok := bc.stateMgr.(interface {
		IncrementRepoBuild(string, bool)
		SetRepoDocumentCount(string, int)
	}); ok && sm != nil && bc.skipReport == nil {
		success := genErr == nil
		contentRoot := filepath.Join(bc.outDir, "content")
		countMarkdown := func(root string) int {
			cnt := 0
			_ = filepath.WalkDir(root, func(p string, d os.DirEntry, werr error) error {
				if werr != nil || d == nil || d.IsDir() {
					return nil
				}
				name := strings.ToLower(d.Name())
				if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown") {
					ln := name
					if ln == "readme.md" || ln == "license.md" || ln == "contributing.md" || ln == "changelog.md" {
						return nil
					}
					cnt++
				}
				return nil
			})
			return cnt
		}
		perRepoDocCounts := make(map[string]int, len(bc.repos))
		for _, r := range bc.repos {
			repoPath := filepath.Join(contentRoot, r.Name)
			if fi, err := os.Stat(repoPath); err == nil && fi.IsDir() {
				perRepoDocCounts[r.URL] = countMarkdown(repoPath)
				continue
			}
			entries, derr := os.ReadDir(contentRoot)
			if derr == nil {
				found := false
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					nsRepoPath := filepath.Join(contentRoot, e.Name(), r.Name)
					if fi2, err2 := os.Stat(nsRepoPath); err2 == nil && fi2.IsDir() {
						perRepoDocCounts[r.URL] = countMarkdown(nsRepoPath)
						found = true
						break
					}
				}
				if !found {
					perRepoDocCounts[r.URL] = 0
				}
			} else {
				perRepoDocCounts[r.URL] = 0
			}
		}
		for _, r := range bc.repos {
			sm.IncrementRepoBuild(r.URL, success)
			if c, okc := perRepoDocCounts[r.URL]; okc {
				sm.SetRepoDocumentCount(r.URL, c)
			}
		}
	}
	// Persist commit heads + config hash + report checksum + global doc hash
	if sm, ok := bc.stateMgr.(interface {
		SetRepoLastCommit(string, string, string, string)
		SetLastConfigHash(string)
		SetLastReportChecksum(string)
		SetLastGlobalDocFilesHash(string)
	}); ok && sm != nil && genErr == nil {
		for _, r := range bc.repos {
			repoPath := filepath.Join(bc.workspace, r.Name)
			if head, herr := hugoReadRepoHead(repoPath); herr == nil && head != "" {
				sm.SetRepoLastCommit(r.URL, r.Name, r.Branch, head)
			}
		}
		if h := bc.generator.ComputeConfigHashForPersistence(); h != "" {
			sm.SetLastConfigHash(h)
		}
		if brData, rerr := os.ReadFile(filepath.Join(bc.outDir, "build-report.json")); rerr == nil {
			sum := sha256.Sum256(brData)
			sm.SetLastReportChecksum(hex.EncodeToString(sum[:]))
		}
		if report.DocFilesHash != "" {
			sm.SetLastGlobalDocFilesHash(report.DocFilesHash)
		}
	}
	// LiveReload broadcast: if hub provided via job metadata under key "live_reload_hub", emit hash after persistence
	if report.DocFilesHash != "" { // report already non-nil earlier
		if hubAny, ok := bc.job.Metadata["live_reload_hub"]; ok {
			if hub, ok2 := hubAny.(*LiveReloadHub); ok2 && hub != nil {
				hub.Broadcast(report.DocFilesHash)
			}
		}
	}
	return nil
}

// Optional helper for future partial timing or debug snapshots.
func (bc *buildContext) debugSnapshot(tag string) {
	slog.Debug("buildContext snapshot", "tag", tag, "repos", len(bc.repos), "out", bc.outDir, "workspace", bc.workspace, "skip", bc.skipReport != nil, "deltaDecision", func() string {
		if bc.deltaPlan == nil {
			return ""
		}
		return fmt.Sprintf("%v", bc.deltaPlan.Decision)
	}(), "time", time.Now().Format(time.RFC3339))
}
