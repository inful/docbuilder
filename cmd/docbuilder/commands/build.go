package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// BuildCmd implements the 'build' command.
type BuildCmd struct {
	Output        string `short:"o" default:"./site" help:"Output directory for generated site"`
	Incremental   bool   `short:"i" help:"Use incremental updates instead of fresh clone"`
	RenderMode    string `name:"render-mode" help:"Override build.render_mode (auto|always|never). Precedence: --render-mode > env vars (skip/run) > config."`
	DocsDir       string `short:"d" name:"docs-dir" default:"./docs" help:"Path to local docs directory (used when no config file provided)"`
	Title         string `name:"title" default:"Documentation" help:"Site title when no config provided"`
	BaseURL       string `name:"base-url" help:"Override hugo.base_url from config"`
	Relocatable   bool   `name:"relocatable" help:"Generate fully relocatable site with relative links (sets base_url to empty string)"`
	EditURLBase   string `name:"edit-url-base" help:"Base URL for generating edit links (e.g., https://github.com/org/repo). If not provided, edit links are only generated for cloned repos with forge URLs."`
	KeepWorkspace bool   `name:"keep-workspace" help:"Keep workspace and staging directories for debugging (do not clean up on exit)"`
}

func (b *BuildCmd) Run(_ *Global, root *CLI) error {
	// Load .env file if it exists (before config)
	if err := LoadEnvFile(); err == nil && root.Verbose {
		slog.Info("Loaded environment variables from .env file")
	}

	// If no config file is specified and doesn't exist, create a minimal config for local docs
	var cfg *config.Config
	var useLocalMode bool

	if root.Config == "" || !fileExists(root.Config) {
		// No config file - use local docs directory mode
		cfg = b.createLocalConfig()
		useLocalMode = true
		slog.Info("No config file found, using local docs directory mode",
			"docs_dir", b.DocsDir,
			"output", b.Output)
	} else {
		_, loadedCfg, err := config.LoadWithResult(root.Config)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		cfg = loadedCfg
		slog.Info("Loaded config from file", "config", root.Config)
	}

	// Apply CLI overrides to config
	if b.BaseURL != "" {
		cfg.Hugo.BaseURL = b.BaseURL
		slog.Info("Base URL overridden via CLI flag", "base_url", b.BaseURL)
	}
	if b.Relocatable {
		cfg.Hugo.BaseURL = ""
		slog.Info("Relocatable mode enabled (base_url set to empty)")
	}
	if b.RenderMode != "" {
		cfg.Build.RenderMode = config.NormalizeRenderMode(b.RenderMode)
		slog.Info("Render mode overridden via CLI flag", "render_mode", cfg.Build.RenderMode)
	}

	// Apply edit-url-base override if provided
	if b.EditURLBase != "" {
		cfg.Build.EditURLBase = b.EditURLBase
		slog.Info("Edit URL base overridden via CLI flag", "edit_url_base", b.EditURLBase)
	}

	// Resolve output directory with base_directory support
	outputDir := ResolveOutputDir(b.Output, cfg)

	// Use different build paths for local vs remote
	if useLocalMode {
		return b.runLocalBuild(cfg, outputDir, root.Verbose, b.KeepWorkspace)
	}

	if err := ApplyAutoDiscovery(context.Background(), cfg); err != nil {
		return err
	}
	return RunBuild(cfg, outputDir, b.Incremental, root.Verbose, b.KeepWorkspace)
}

// RunBuild executes the full build pipeline via the canonical BuildService.
// After the build-service-unification refactor this is a thin wrapper that
// constructs a BuildRequest and hands off to BuildService.Run.
//
//nolint:forbidigo // fmt is used for user-facing messages
func RunBuild(cfg *config.Config, outputDir string, incrementalMode, verbose, keepWorkspace bool) error {
	fmt.Println("Starting DocBuilder build")

	level := parseLogLevel(verbose)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	slog.Info("Starting documentation build",
		"output", outputDir,
		"repositories", len(cfg.Repositories),
		"incremental", incrementalMode,
		"keep_workspace", keepWorkspace)

	svc := newCLIService(cfg, keepWorkspace)
	result, err := svc.Run(context.Background(), build.BuildRequest{
		Config:        cfg,
		OutputDir:     outputDir,
		Incremental:   incrementalMode,
		KeepWorkspace: keepWorkspace,
		Options:       build.BuildOptions{SkipIfUnchanged: cfg.Build.SkipIfUnchanged},
	})
	if err != nil {
		slog.Error("Build pipeline failed", "error", err)
		if keepWorkspace {
			fmt.Printf("\nError occurred. Workspace preserved for debugging.\n")
		}
		return err
	}

	report := result.Report
	if report != nil && report.FailedRepositories > 0 {
		slog.Warn("Some repositories were skipped due to errors",
			"skipped", report.FailedRepositories,
			"total", len(cfg.Repositories))
	}

	slog.Info("Build completed successfully",
		"output", outputDir,
		"pages", reportRenderedPages(report),
		"skipped_repos", reportFailedRepos(report))

	fmt.Println("Build completed successfully")
	return nil
}

// newCLIService builds a BuildService configured for the CLI's workspace
// behavior. When keepWorkspace is true the workspace manager is persistent
// (its Cleanup is a no-op) so the cloned repos survive process exit.
func newCLIService(cfg *config.Config, keepWorkspace bool) *build.DefaultBuildService {
	wsDir := cfg.Build.WorkspaceDir
	return build.NewBuildService().
		WithWorkspaceFactory(func() *workspace.Manager {
			if keepWorkspace {
				return workspace.NewPersistentManager(wsDir, "working")
			}
			return workspace.NewManager(wsDir)
		})
}

// reportRenderedPages returns the rendered-pages count from a possibly-nil
// report. The BuildReport pointer is the concrete models type; we only
// read two fields so we accept a typed parameter.
func reportRenderedPages(report *models.BuildReport) int {
	if report == nil {
		return 0
	}
	return report.RenderedPages
}

// reportFailedRepos returns the failed-repo count from a possibly-nil report.
func reportFailedRepos(report *models.BuildReport) int {
	if report == nil {
		return 0
	}
	return report.FailedRepositories
}

// prepareLocalRepoConfig configures repository settings for local builds.
// Returns the repository config and the actual path to use for discovery.
func (b *BuildCmd) prepareLocalRepoConfig(cfg *config.Config, docsPath string) ([]config.Repository, string) {
	repos := cfg.Repositories

	if len(repos) == 0 {
		// Fallback if config is missing repositories
		return []config.Repository{{
			URL:    docsPath,
			Name:   "local",
			Branch: "",
			Paths:  []string{"."},
		}}, docsPath
	}

	// If paths is not just ["."], it means DocsDir is a subdirectory and we need
	// to use the parent directory as the repo root for discovery to work correctly
	if len(repos[0].Paths) == 1 && repos[0].Paths[0] != "." {
		// Use parent directory of docsPath as the repo root
		parentPath := filepath.Dir(docsPath)
		repos[0].URL = parentPath
		return repos, parentPath
	}

	// Standard case: paths is ["."], use docsPath directly
	repos[0].URL = docsPath
	return repos, docsPath
}

// runLocalBuild builds from a local docs directory without git cloning.
// Routes through BuildService.RunDirect so the CLI uses one canonical
// pipeline.
//
//nolint:forbidigo // fmt is used for user-facing messages
func (b *BuildCmd) runLocalBuild(cfg *config.Config, outputDir string, verbose, keepWorkspace bool) error {
	fmt.Println("Starting DocBuilder local build")

	level := parseLogLevel(verbose)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if keepWorkspace {
		slog.Info("Workspace preservation enabled for debugging", "keep_workspace", true)
	}

	// Resolve absolute path to docs directory
	docsPath, err := filepath.Abs(b.DocsDir)
	if err != nil {
		return fmt.Errorf("resolve docs dir: %w", err)
	}

	// Verify docs directory exists
	if st, statErr := os.Stat(docsPath); statErr != nil || !st.IsDir() {
		return fmt.Errorf("docs dir not found or not a directory: %s (use -d to specify a different path)", docsPath)
	}

	slog.Info("Building from local directory",
		"docs_dir", docsPath,
		"output", outputDir)

	// Prepare repository configuration for discovery
	repos, repoPath := b.prepareLocalRepoConfig(cfg, docsPath)
	discovery := docs.NewDiscovery(repos, &cfg.Build)
	repoPaths := map[string]string{"local": repoPath}

	slog.Info("Discovering documentation files")
	docFiles, discErr := discovery.DiscoverDocs(repoPaths)
	if discErr != nil {
		return fmt.Errorf("discovery failed: %w", discErr)
	}

	if len(docFiles) == 0 {
		slog.Warn("No documentation files found in directory", "dir", docsPath)
		return fmt.Errorf("no documentation files found in %s", docsPath)
	}

	slog.Info("Documentation discovered", "files", len(docFiles))
	slog.Info("Generating Hugo site", "output", outputDir)

	svc := build.NewBuildService()
	result, err := svc.RunDirect(context.Background(), build.DirectBuildRequest{
		Config:      cfg,
		OutputDir:   outputDir,
		DocFiles:    docFiles,
		KeepStaging: keepWorkspace,
	})
	if err != nil {
		if keepWorkspace {
			fmt.Printf("\nError occurred. Hugo staging directory may be preserved for debugging.\n")
		}
		return fmt.Errorf("site generation failed: %w", err)
	}

	slog.Info("Hugo site generated successfully",
		"output", outputDir,
		"pages", reportRenderedPages(result.Report))

	if keepWorkspace {
		fmt.Printf("Build output directory: %s\n", outputDir)
		fmt.Printf("(Staging directory was promoted to output on success)\n")
	}
	fmt.Println("Build completed successfully")
	return nil
}

// createLocalConfig creates a minimal configuration for building from a local docs directory.
func (b *BuildCmd) createLocalConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Version = "2.0"

	cfg.Output.Directory = b.Output
	cfg.Output.Clean = true

	cfg.Hugo.Title = b.Title
	cfg.Hugo.Description = "Documentation built with DocBuilder"
	cfg.Hugo.BaseURL = "http://localhost:1316/" // Match preview's default baseURL

	cfg.Build.RenderMode = config.RenderModeAlways

	// Determine repository URL and paths for edit URLs
	// Extract the base directory name to use as the docs path in edit URLs
	cleanDir := filepath.Clean(b.DocsDir)
	baseName := filepath.Base(cleanDir)

	repoURL := b.DocsDir
	paths := []string{"."}

	// If DocsDir is a subdirectory (not just "." or current directory),
	// use the parent as repo root and subdirectory name as the path
	// This ensures edit URLs include the correct subdirectory prefix
	if baseName != "." && baseName != ".." {
		parentDir := filepath.Dir(cleanDir)
		if parentDir == "." {
			parentDir = "./"
		}
		repoURL = parentDir
		paths = []string{baseName}
	}

	// Single local repository entry pointing to DocsDir
	cfg.Repositories = []config.Repository{{
		URL:    repoURL,
		Name:   "local",
		Branch: "",
		Paths:  paths,
	}}

	return cfg
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
