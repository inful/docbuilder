package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/incremental"
	"git.home.luguber.info/inful/docbuilder/internal/manifest"
	"git.home.luguber.info/inful/docbuilder/internal/storage"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

// BuildCmd implements the 'build' command.
type BuildCmd struct {
	Output      string `short:"o" help:"Output directory for generated site" default:"./site"`
	Incremental bool   `short:"i" help:"Use incremental updates instead of fresh clone"`
	RenderMode  string `name:"render-mode" help:"Override build.render_mode (auto|always|never). Precedence: --render-mode > env vars (skip/run) > config."`
}

func (b *BuildCmd) Run(_ *Global, root *CLI) error {
	cfg, err := config.Load(root.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	// Apply CLI render mode override before any build operations (highest precedence besides explicit skip env)
	if b.RenderMode != "" {
		if rm := config.NormalizeRenderMode(b.RenderMode); rm != "" {
			cfg.Build.RenderMode = rm
			slog.Info("Render mode overridden via CLI flag", "mode", rm)
		} else {
			slog.Warn("Ignoring invalid --render-mode value", "value", b.RenderMode)
		}
	}
	if err := ApplyAutoDiscovery(context.Background(), cfg); err != nil {
		return err
	}

	// Resolve output directory with base_directory support
	outputDir := ResolveOutputDir(b.Output, cfg)
	return RunBuild(cfg, outputDir, b.Incremental, root.Verbose)
}

func RunBuild(cfg *config.Config, outputDir string, incrementalMode, verbose bool) error {
	// Provide friendly user-facing messages on stdout for CLI integration tests.
	fmt.Println("Starting DocBuilder build")

	// Set logging level (parseLogLevel handles both verbose flag and DOCBUILDER_LOG_LEVEL)
	level := parseLogLevel(verbose)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	slog.Info("Starting documentation build",
		"output", outputDir,
		"repositories", len(cfg.Repositories),
		"incremental", incrementalMode)

	// Initialize cache storage if incremental builds are enabled
	var buildCache *incremental.BuildCache
	var stageCache *incremental.StageCache
	var remoteHeadCache *git.RemoteHeadCache
	if cfg.Build.EnableIncremental {
		store, err := storage.NewFSStore(cfg.Build.CacheDir)
		if err != nil {
			return fmt.Errorf("failed to initialize cache storage: %w", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				slog.Warn("Failed to close storage", "error", err)
			}
		}()

		buildCache = incremental.NewBuildCache(store, cfg.Build.CacheDir)
		stageCache = incremental.NewStageCache(store)

		// Initialize remote HEAD cache
		remoteHeadCache, err = git.NewRemoteHeadCache(cfg.Build.CacheDir)
		if err != nil {
			slog.Warn("Failed to initialize remote HEAD cache", "error", err)
		} else {
			defer func() {
				if err := remoteHeadCache.Save(); err != nil {
					slog.Warn("Failed to save remote HEAD cache", "error", err)
				}
			}()
		}

		slog.Info("Incremental build cache initialized", "cache_dir", cfg.Build.CacheDir)
	}
	// StageCache reserved for future stage-level caching (Phase 1 steps 1.6-1.7)
	_ = stageCache

	// Create workspace manager
	wsManager, err := CreateWorkspace(cfg)
	if err != nil {
		return err
	}
	defer CleanupWorkspace(wsManager)

	// Create Git client with build config for auth support and remote HEAD cache
	gitClient, err := CreateGitClient(wsManager, cfg)
	if err != nil {
		return err
	}
	if remoteHeadCache != nil {
		gitClient.WithRemoteHeadCache(remoteHeadCache)
	}

	// Step 2.5: Expand repositories with versioning if enabled
	repositories := cfg.Repositories
	if cfg.Versioning != nil && !cfg.Versioning.DefaultBranchOnly {
		expandedRepos, err := versioning.ExpandRepositoriesWithVersions(gitClient, cfg)
		if err != nil {
			slog.Warn("Failed to expand repositories with versions, using original list", "error", err)
		} else {
			repositories = expandedRepos
			slog.Info("Using expanded repository list with versions", "count", len(repositories))
		}
	}

	// Clone/update all repositories
	repoPaths := make(map[string]string)
	repositoriesSkipped := 0
	for _, repo := range repositories {
		slog.Info("Processing repository", "name", repo.Name, "url", repo.URL)

		var repoPath string
		var err error

		if incrementalMode {
			repoPath, err = gitClient.UpdateRepo(repo)
		} else {
			repoPath, err = gitClient.CloneRepo(repo)
		}

		if err != nil {
			slog.Error("Failed to process repository", "name", repo.Name, "error", err)
			// Continue with remaining repositories instead of failing
			repositoriesSkipped++
			continue
		}

		repoPaths[repo.Name] = repoPath
		slog.Info("Repository processed", "name", repo.Name, "path", repoPath)
	}

	if repositoriesSkipped > 0 {
		slog.Warn("Some repositories were skipped due to errors",
			"skipped", repositoriesSkipped,
			"successful", len(repoPaths),
			"total", len(repositories))
	}

	if len(repoPaths) == 0 {
		return fmt.Errorf("no repositories could be cloned successfully")
	}

	slog.Info("All repositories processed", "successful", len(repoPaths), "skipped", repositoriesSkipped)

	// Step 1.5: Build-level cache check (if incremental enabled)
	if cfg.Build.EnableIncremental && buildCache != nil {
		// Compute repository hashes
		repoHashes, err := incremental.ComputeRepoHashes(repositories, repoPaths)
		if err != nil {
			slog.Warn("Failed to compute repo hashes, continuing without cache", "error", err)
		} else {
			// Compute build signature
			buildSig, err := incremental.ComputeSimpleBuildSignature(cfg, repoHashes)
			if err != nil {
				slog.Warn("Failed to compute build signature, continuing without cache", "error", err)
			} else {
				// Check if we can skip this build
				shouldSkip, cachedBuild, err := buildCache.ShouldSkipBuild(buildSig)
				if err != nil {
					slog.Warn("Failed to check build cache", "error", err)
				} else if shouldSkip {
					slog.Info("Build cache hit - site is up to date",
						"build_id", cachedBuild.BuildID,
						"cached_at", cachedBuild.Timestamp)
					fmt.Println("Build completed successfully (from cache)")
					return nil
				}
				slog.Info("Build cache miss - proceeding with full build", "signature", buildSig.BuildHash)
			}
		}
	}

	// Discover documentation files
	slog.Info("Starting documentation discovery")
	discovery := docs.NewDiscovery(repositories, &cfg.Build)

	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return err
	}

	if len(docFiles) == 0 {
		slog.Warn("No documentation files found in any repository")
		return nil
	}

	// Log discovery summary
	filesByRepo := discovery.GetDocFilesByRepository()
	for repoName, files := range filesByRepo {
		slog.Info("Documentation files by repository", "repository", repoName, "files", len(files))
	}

	// Generate Hugo site
	slog.Info("Generating Hugo site", "output", outputDir, "files", len(docFiles))
	generator := hugo.NewGenerator(cfg, outputDir)

	if err := generator.GenerateSite(docFiles); err != nil {
		slog.Error("Failed to generate Hugo site", "error", err)
		return err
	}

	slog.Info("Hugo site generated successfully", "output", outputDir)

	// Step 1.8: Save build manifest for future cache checks
	if cfg.Build.EnableIncremental && buildCache != nil {
		repoHashes, err := incremental.ComputeRepoHashes(repositories, repoPaths)
		if err == nil {
			buildSig, err := incremental.ComputeSimpleBuildSignature(cfg, repoHashes)
			if err == nil {
				// Create build manifest
				buildManifest := &manifest.BuildManifest{
					ID:        fmt.Sprintf("build-%d", time.Now().Unix()),
					Timestamp: time.Now(),
					Status:    "success",
					Inputs: manifest.Inputs{
						ConfigHash: buildSig.ConfigHash,
					},
					Plan: manifest.Plan{
						Theme:      cfg.Hugo.Theme,
						Transforms: []string{},
					},
				}

				// Convert repo hashes to manifest format
				for _, rh := range repoHashes {
					buildManifest.Inputs.Repos = append(buildManifest.Inputs.Repos, manifest.RepoInput{
						Name:   rh.Name,
						Commit: rh.Commit,
						Hash:   rh.Hash,
					})
				}

				// Save the build
				if err := buildCache.SaveBuild(buildSig, buildManifest, outputDir); err != nil {
					slog.Warn("Failed to save build to cache", "error", err)
				} else {
					slog.Info("Build manifest saved to cache", "build_id", buildManifest.ID)
				}
			}
		}
	}

	fmt.Println("Build completed successfully")
	return nil
}
