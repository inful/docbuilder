package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	// Instantiate generator early so we can compute config hash for skip decision before destructive clean.
	gen := hugo.NewGenerator(&cloneCfg, outDir)
	if smAny, ok := job.Metadata["state_manager"]; ok {
		if sm, ok2 := smAny.(interface {
			SetRepoDocumentCount(string, int)
			SetRepoDocFilesHash(string, string)
		}); ok2 {
			gen = gen.WithStateManager(sm)
		}
	}

	// Pre-clone cross-run skip optimization via SkipEvaluator (must occur BEFORE output cleaning).
	if cloneCfg.Build.SkipIfUnchanged && len(reposAny) > 0 {
		if smAny, ok := job.Metadata["state_manager"]; ok {
			if sm, ok2 := smAny.(SkipStateAccess); ok2 {
				if rep, skipped := NewSkipEvaluator(outDir, sm, gen).Evaluate(reposAny); skipped {
					return rep, nil
				}
			}
		}
	}
	// Pre-clone cross-run skip optimization via SkipEvaluator (must occur BEFORE output cleaning).
	if cloneCfg.Build.SkipIfUnchanged && len(reposAny) > 0 {
		if smAny, ok := job.Metadata["state_manager"]; ok {
			if sm, ok2 := smAny.(SkipStateAccess); ok2 {
				if rep, skipped := NewSkipEvaluator(outDir, sm, gen).Evaluate(reposAny); skipped {
					return rep, nil
				}
			}
		}
	}

	// Only clean if we are definitely going to build.
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
		strategy := cloneCfg.Build.CloneStrategy
		if strategy == "" { // default applied earlier but keep defensive fallback
			strategy = cfg.CloneStrategyFresh
		}

		// Prefer daemon.storage.repo_cache_dir for persistent strategies when present.
		repoCache := ""
		if cloneCfg.Daemon != nil {
			repoCache = cloneCfg.Daemon.Storage.RepoCacheDir
		}
		if repoCache != "" {
			repoCache = filepath.Clean(repoCache)
		}

		if strategy == cfg.CloneStrategyFresh {
			// Ephemeral: workspace inside output so output.clean wipes it.
			workspaceDir = filepath.Join(outDir, "_workspace")
		} else if repoCache != "" {
			// Use repo_cache_dir (add a subfolder 'working' to avoid mixing state files & repos directly)
			workspaceDir = filepath.Join(repoCache, "working")
			slog.Info("Deriving workspace from repo_cache_dir", "repo_cache_dir", repoCache, "workspace", workspaceDir, "strategy", strategy)
		} else {
			// Fallback: sibling directory next to output
			workspaceDir = filepath.Clean(outDir + "-workspace")
		}
	}
	// Only auto-clean workspace contents when strategy=fresh and it lives under output.
	if cloneCfg.Output.Clean && cloneCfg.Build.CloneStrategy == cfg.CloneStrategyFresh {
		if rel, err := filepath.Rel(outDir, workspaceDir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			if err := os.RemoveAll(workspaceDir); err != nil {
				slog.Warn("Failed to clean workspace directory", "dir", workspaceDir, "error", err)
			}
		}
	} else if cloneCfg.Build.CloneStrategy != cfg.CloneStrategyFresh && cloneCfg.Output.Clean {
		slog.Info("Preserving workspace for incremental updates", "dir", workspaceDir, "strategy", cloneCfg.Build.CloneStrategy)
	}
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	slog.Info("Using workspace directory", "dir", workspaceDir, "configured", cloneCfg.Build.WorkspaceDir != "")

	// Recorder is optionally injected earlier by queue/daemon (prometheus tag variant).
	if err := os.Setenv("DOCBUILDER_RUN_HUGO", "1"); err != nil {
		slog.Warn("Failed to set DOCBUILDER_RUN_HUGO env", "error", err)
	}
	report, err := gen.GenerateFullSite(ctx, reposAny, workspaceDir)
	if err != nil {
		slog.Error("Full site generation error", "error", err)
	}

	// Update per-repository build and document statistics when state manager available.
	if smAny, ok := job.Metadata["state_manager"]; ok && report != nil {
		if sm, ok2 := smAny.(interface {
			IncrementRepoBuild(string, bool)
			SetRepoDocumentCount(string, int)
		}); ok2 {
			success := err == nil
			// Derive per-repository document counts by scanning the generated content directory.
			perRepoDocCounts := make(map[string]int, len(reposAny))
			contentRoot := filepath.Join(outDir, "content")
			// Helper to count markdown files recursively under a directory.
			countMarkdown := func(root string) int {
				count := 0
				_ = filepath.WalkDir(root, func(p string, d os.DirEntry, werr error) error {
					if werr != nil || d == nil || d.IsDir() {
						return nil
					}
					name := strings.ToLower(d.Name())
					if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown") {
						// Ignore typical root docs per discovery rules (README, LICENSE etc.) to align with discovery count semantics.
						ln := strings.ToLower(name)
						if ln == "readme.md" || ln == "license.md" || ln == "contributing.md" || ln == "changelog.md" {
							return nil
						}
						count++
					}
					return nil
				})
				return count
			}

			// Strategy: First look for content/<repoName>. If not present, assume forge namespace and search content/*/<repoName>.
			for _, r := range reposAny {
				repoPath := filepath.Join(contentRoot, r.Name)
				if fi, statErr := os.Stat(repoPath); statErr == nil && fi.IsDir() {
					perRepoDocCounts[r.URL] = countMarkdown(repoPath)
					continue
				}
				// Fallback: look one level deeper (namespaced by forge). We don't know forge name here without additional metadata, so brute force immediate children.
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
			for _, r := range reposAny {
				sm.IncrementRepoBuild(r.URL, success)
				if c, okc := perRepoDocCounts[r.URL]; okc {
					sm.SetRepoDocumentCount(r.URL, c)
				}
			}
		}
	}

	// Persist last repo heads & config hash if available and build succeeded (or skipped with no changes)
	if err == nil && report != nil {
		// Access optional state manager passed via metadata to avoid global coupling.
		if smAny, ok := job.Metadata["state_manager"]; ok {
			if sm, ok2 := smAny.(interface {
				SetRepoLastCommit(string, string, string, string)
				SetLastConfigHash(string)
				SetLastReportChecksum(string)
				SetLastGlobalDocFilesHash(string)
			}); ok2 {
				// Iterate repos to capture current heads if workspace still present.
				for _, r := range reposAny {
					repoPath := filepath.Join(workspaceDir, r.Name)
					if head, herr := hugoReadRepoHead(repoPath); herr == nil && head != "" {
						sm.SetRepoLastCommit(r.URL, r.Name, r.Branch, head)
					}
				}
				// Store config hash from build state if obtainable via reflection-free accessor (report has no hash; use generator compute again).
				if h := gen.ComputeConfigHashForPersistence(); h != "" {
					sm.SetLastConfigHash(h)
				}
				// Persist checksum of freshly written build report
				if brData, rerr := os.ReadFile(filepath.Join(outDir, "build-report.json")); rerr == nil {
					sum := sha256.Sum256(brData)
					sm.SetLastReportChecksum(hex.EncodeToString(sum[:]))
				}
				// Persist global doc files hash for next-run cross-repo early skip comparisons.
				if report.DocFilesHash != "" {
					sm.SetLastGlobalDocFilesHash(report.DocFilesHash)
				}
			}
		}
	}
	return report, err
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
