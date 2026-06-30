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
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
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
			return derrors.WrapError(err, derrors.CategoryConfig, "load config").Build()
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

// RunBuild executes the build pipeline using build.BuildService.
// This is the canonical CLI entry point; it routes through the same service
// the daemon uses (with a nil skip-evaluator and a noop recorder).
//
//nolint:forbidigo // fmt is used for user-facing messages
func RunBuild(cfg *config.Config, outputDir string, incrementalMode, verbose, keepWorkspace bool) error {
	// Provide friendly user-facing messages on stdout for CLI integration tests.
	fmt.Println("Starting DocBuilder build")

	// Set logging level (parseLogLevel handles both verbose flag and DOCBUILDER_LOG_LEVEL)
	level := parseLogLevel(verbose)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Map incremental flag to config
	if incrementalMode {
		cfg.Build.CloneStrategy = config.CloneStrategyUpdate
	}

	slog.Info("Starting documentation build",
		"output", outputDir,
		"repositories", len(cfg.Repositories),
		"incremental", incrementalMode,
		"keep_workspace", keepWorkspace)

	// Build the workspace factory. Persistent managers (KeepWorkspace=true)
	// skip cleanup on exit; ephemeral managers (KeepWorkspace=false) are
	// removed by BuildService's deferred cleanup.
	wsDir := cfg.Build.WorkspaceDir
	workspaceFactory := func() *workspace.Manager {
		if keepWorkspace {
			return workspace.NewPersistentManager(wsDir, "working")
		}
		return workspace.NewManager(wsDir)
	}

	svc := build.NewBuildService().
		WithWorkspaceFactory(workspaceFactory).
		WithHugoGeneratorFactory(func(c *config.Config, dir string) build.HugoGenerator {
			return hugo.NewGenerator(c, dir).WithKeepStaging(keepWorkspace)
		}).
		WithSkipEvaluatorFactory(func(string) build.SkipEvaluator {
			// Skip evaluation requires daemon state; the CLI never has it.
			return nil
		})

	req := build.BuildRequest{
		Config:      cfg,
		OutputDir:   outputDir,
		Incremental: incrementalMode,
		Options: build.BuildOptions{
			Verbose:         verbose,
			KeepWorkspace:   keepWorkspace,
			SkipIfUnchanged: false,
		},
	}

	result, err := svc.Run(context.Background(), req)
	if err != nil {
		slog.Error("Build pipeline failed", "error", err)
		if keepWorkspace {
			fmt.Printf("\nError occurred. Workspace preserved (check above log).\n")
			fmt.Printf("Hugo staging directory: %s_stage\n", outputDir)
		}
		return err
	}

	if result.Report != nil && result.Report.FailedRepositories > 0 {
		slog.Warn("Some repositories were skipped due to errors",
			"skipped", result.Report.FailedRepositories,
			"total", len(cfg.Repositories))
	}

	// Preserve the original log shape: "pages" = rendered HTML pages (not
	// markdown files). The CLI used to read report.RenderedPages directly;
	// BuildService exposes that via BuildResult.Report.RenderedPages.
	renderedPages := 0
	if result.Report != nil {
		renderedPages = result.Report.RenderedPages
	}
	slog.Info("Build completed successfully",
		"output", outputDir,
		"pages", renderedPages,
		"skipped_repos", result.RepositoriesSkipped)

	fmt.Println("Build completed successfully")
	return nil
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
//
//nolint:forbidigo // fmt is used for user-facing messages
func (b *BuildCmd) runLocalBuild(cfg *config.Config, outputDir string, verbose, keepWorkspace bool) error {
	fmt.Println("Starting DocBuilder local build")

	// Set logging level
	level := parseLogLevel(verbose)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if keepWorkspace {
		slog.Info("Workspace preservation enabled for debugging", "keep_workspace", true)
	}

	// Resolve absolute path to docs directory
	docsPath, err := filepath.Abs(b.DocsDir)
	if err != nil {
		return derrors.WrapError(err, derrors.CategoryFileSystem, "resolve docs dir").Build()
	}

	// Verify docs directory exists
	if st, statErr := os.Stat(docsPath); statErr != nil || !st.IsDir() {
		return derrors.NewError(derrors.CategoryNotFound, "docs dir not found or not a directory (use -d to specify a different path)").WithContext("path", docsPath).Build()
	}

	slog.Info("Building from local directory",
		"docs_dir", docsPath,
		"output", outputDir)

	// Prepare repository configuration for discovery
	repos, repoPath := b.prepareLocalRepoConfig(cfg, docsPath)
	discovery := docs.NewDiscovery(repos, &cfg.Build)
	repoPaths := map[string]string{"local": repoPath}

	// Discover docs
	slog.Info("Discovering documentation files")
	docFiles, discErr := discovery.DiscoverDocs(repoPaths)
	if discErr != nil {
		return derrors.WrapError(discErr, derrors.CategoryInternal, "discovery failed").Build()
	}

	if len(docFiles) == 0 {
		slog.Warn("No documentation files found in directory", "dir", docsPath)
		return derrors.NewError(derrors.CategoryNotFound, "no documentation files found").WithContext("path", docsPath).Build()
	}

	slog.Info("Documentation discovered", "files", len(docFiles))

	// Generate Hugo site
	slog.Info("Generating Hugo site", "output", outputDir)

	// Use newer site generation with report support
	generator := hugo.NewGenerator(cfg, outputDir).WithKeepStaging(keepWorkspace)

	report, err := generator.GenerateSiteWithReportContext(context.Background(), docFiles)
	if err != nil {
		// Show staging location on error for debugging
		if keepWorkspace {
			fmt.Printf("\nError occurred. Hugo staging directory: %s_stage\n", outputDir)
		}
		return derrors.WrapError(err, derrors.CategoryInternal, "site generation failed").Build()
	}

	slog.Info("Hugo site generated successfully",
		"output", outputDir,
		"pages", report.RenderedPages)

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
