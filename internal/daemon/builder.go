package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// Pre-clone cross-run skip optimization must occur BEFORE output cleaning, otherwise we might delete the site then skip rebuilding it.
	if cloneCfg.Build.SkipIfUnchanged && len(reposAny) > 0 {
		if smAny, ok := job.Metadata["state_manager"]; ok {
			if sm, ok2 := smAny.(interface {
				GetRepoLastCommit(string) string
				GetLastConfigHash() string
				GetLastReportChecksum() string
				SetLastReportChecksum(string)
			}); ok2 {
				currentHash := gen.ComputeConfigHashForPersistence()
				lastHash := sm.GetLastConfigHash()
				if currentHash != "" && currentHash == lastHash {
					prevPath := filepath.Join(outDir, "build-report.json")
					if data, rerr := os.ReadFile(prevPath); rerr == nil {
						h := sha256.Sum256(data)
						prevSum := hex.EncodeToString(h[:])
						if stored := sm.GetLastReportChecksum(); stored != "" && stored != prevSum {
							slog.Warn("Previous build report checksum mismatch; forcing rebuild", "stored", stored, "current", prevSum)
						} else {
							publicDir := filepath.Join(outDir, "public")
							if fi, err := os.Stat(publicDir); err != nil || !fi.IsDir() {
								if err != nil {
									slog.Warn("Public directory missing; forcing rebuild", "dir", publicDir, "error", err)
								} else {
									slog.Warn("Public path is not directory; forcing rebuild", "dir", publicDir)
								}
							} else if entries, eerr := os.ReadDir(publicDir); eerr != nil || len(entries) == 0 {
								if eerr != nil {
									slog.Warn("Failed to read public directory; forcing rebuild", "dir", publicDir, "error", eerr)
								} else {
									slog.Warn("Public directory empty; forcing rebuild", "dir", publicDir)
								}
							} else {
								// Additional guard: ensure content directory looks healthy if previous build reported files.
								contentDir := filepath.Join(outDir, "content")
								contentStat, cErr := os.Stat(contentDir)
								prev := struct {
									Repositories  int `json:"repositories"`
									Files         int `json:"files"`
									RenderedPages int `json:"rendered_pages"`
								}{}
								parseErr := json.Unmarshal(data, &prev)
								if parseErr != nil {
									// If we cannot parse previous report safely, force rebuild (defensive).
									slog.Warn("Failed to parse previous build report; forcing rebuild", "error", parseErr)
								} else if prev.Files > 0 { // Only enforce content checks if we previously had files.
									if cErr != nil || !contentStat.IsDir() {
										if cErr != nil {
											slog.Warn("Content directory missing; forcing rebuild", "dir", contentDir, "error", cErr)
										} else {
											slog.Warn("Content path is not directory; forcing rebuild", "dir", contentDir)
										}
									} else {
										// Quick probe: ensure at least one markdown file exists. Walk stops early.
										foundMD := false
										if walkErr := filepath.Walk(contentDir, func(p string, info os.FileInfo, err error) error {
											if err != nil || foundMD {
												return nil
											}
											if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
												foundMD = true
											}
											return nil
										}); walkErr != nil {
											slog.Warn("Error walking content directory during skip probe", "error", walkErr)
										}
										if !foundMD {
											slog.Warn("No markdown files found in existing content directory; forcing rebuild")
										} else {
											allHaveCommits := true
											for _, r := range reposAny {
												if sm.GetRepoLastCommit(r.URL) == "" {
													allHaveCommits = false
													break
												}
											}
											if allHaveCommits {
												reuseRepos, reuseFiles, reuseRendered := prev.Repositories, prev.Files, prev.RenderedPages
												report := &hugo.BuildReport{SchemaVersion: 1, Start: time.Now(), End: time.Now(), SkipReason: "no_changes", Outcome: hugo.OutcomeSuccess, Repositories: reuseRepos, Files: reuseFiles, RenderedPages: reuseRendered}
												if err := report.Persist(outDir); err != nil {
													slog.Warn("Failed to persist skip report", "error", err)
												} else if rb, rberr := os.ReadFile(prevPath); rberr == nil {
													hs := sha256.Sum256(rb)
													sm.SetLastReportChecksum(hex.EncodeToString(hs[:]))
												}
												slog.Info("Skipping build (unchanged) without cleaning output", "repos", reuseRepos, "files", reuseFiles, "content_probe", "ok")
												return report, nil
											}
											// Missing commit data for at least one repository - fall through to rebuild.
											slog.Warn("Missing last commit metadata for one or more repositories; forcing rebuild")
										}
									}
								} else {
									// Zero files previously; allow skip path only if commits and public dir checks passed (no content probe required).
									allHaveCommits := true
									for _, r := range reposAny {
										if sm.GetRepoLastCommit(r.URL) == "" {
											allHaveCommits = false
											break
										}
									}
									if allHaveCommits {
										report := &hugo.BuildReport{SchemaVersion: 1, Start: time.Now(), End: time.Now(), SkipReason: "no_changes", Outcome: hugo.OutcomeSuccess}
										if err := report.Persist(outDir); err != nil {
											slog.Warn("Failed to persist skip report", "error", err)
										} else if rb, rberr := os.ReadFile(prevPath); rberr == nil {
											hs := sha256.Sum256(rb)
											sm.SetLastReportChecksum(hex.EncodeToString(hs[:]))
										}
										slog.Info("Skipping build (unchanged) with zero prior files", "repos", len(reposAny))
										return report, nil
									}
								}
								// If we reached here, one of the guards failed and we will proceed with a full rebuild.
								// (No-op here; fall through to normal build path)
							}
						}
					}
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

	// Persist last repo heads & config hash if available and build succeeded (or skipped with no changes)
	if err == nil && report != nil {
		// Access optional state manager passed via metadata to avoid global coupling.
		if smAny, ok := job.Metadata["state_manager"]; ok {
			if sm, ok2 := smAny.(interface {
				SetRepoLastCommit(string, string, string, string)
				SetLastConfigHash(string)
				SetLastReportChecksum(string)
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
