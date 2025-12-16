package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/versioning"
)

// BuildCmd implements the 'build' command.
type BuildCmd struct {
	Output      string `short:"o" help:"Output directory for generated site" default:"./site"`
	Incremental bool   `short:"i" help:"Use incremental updates instead of fresh clone"`
	RenderMode  string `name:"render-mode" help:"Override build.render_mode (auto|always|never). Precedence: --render-mode > env vars (skip/run) > config."`
	DocsDir     string `name:"docs-dir" short:"d" help:"Path to local docs directory (used when no config file provided)" default:"./docs"`
	Theme       string `name:"theme" help:"Hugo theme to use when no config provided (hextra, docsy, or relearn)" default:"relearn"`
	Title       string `name:"title" help:"Site title when no config provided" default:"Documentation"`
}

func (b *BuildCmd) Run(_ *Global, root *CLI) error {
	// If no config file is specified and doesn't exist, create a minimal config for local docs
	var cfg *config.Config
	var err error
	var useLocalMode bool
	
	if root.Config == "" || !fileExists(root.Config) {
		// No config file - use local docs directory mode
		cfg = b.createLocalConfig()
		useLocalMode = true
		slog.Info("No config file found, using local docs directory mode",
			"docs_dir", b.DocsDir,
			"output", b.Output,
			"theme", b.Theme)
	} else {
		cfg, err = config.Load(root.Config)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		useLocalMode = false
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
	
	// Resolve output directory with base_directory support
	outputDir := ResolveOutputDir(b.Output, cfg)
	
	// Use different build paths for local vs remote
	if useLocalMode {
		return b.runLocalBuild(cfg, outputDir, root.Verbose)
	}
	
	if err := ApplyAutoDiscovery(context.Background(), cfg); err != nil {
		return err
	}
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

	// Create workspace manager
	wsManager, err := CreateWorkspace(cfg)
	if err != nil {
		return err
	}
	defer CleanupWorkspace(wsManager)

	// Create Git client with build config for auth support
	gitClient, err := CreateGitClient(wsManager, cfg)
	if err != nil {
		return err
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

	fmt.Println("Build completed successfully")
	return nil
}

// runLocalBuild builds from a local docs directory without git cloning
func (b *BuildCmd) runLocalBuild(cfg *config.Config, outputDir string, verbose bool) error {
	fmt.Println("Starting DocBuilder local build")

	// Set logging level
	level := parseLogLevel(verbose)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Resolve absolute path to docs directory
	docsPath, err := filepath.Abs(b.DocsDir)
	if err != nil {
		return fmt.Errorf("resolve docs dir: %w", err)
	}
	
	// Verify docs directory exists
	if st, err := os.Stat(docsPath); err != nil || !st.IsDir() {
		return fmt.Errorf("docs dir not found or not a directory: %s (use -d to specify a different path)", docsPath)
	}

	slog.Info("Building from local directory",
		"docs_dir", docsPath,
		"output", outputDir,
		"theme", cfg.Hugo.Theme)

	// Prepare discovery
	repos := []config.Repository{{
		URL:    docsPath,
		Name:   "local",
		Branch: "",
		Paths:  []string{"."},
	}}
	discovery := docs.NewDiscovery(repos, &cfg.Build)
	repoPaths := map[string]string{"local": docsPath}
	
	// Discover docs
	slog.Info("Discovering documentation files")
	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}
	
	if len(docFiles) == 0 {
		slog.Warn("No documentation files found in directory", "dir", docsPath)
		return fmt.Errorf("no documentation files found in %s", docsPath)
	}
	
	slog.Info("Documentation discovered", "files", len(docFiles))

	// Generate Hugo site
	slog.Info("Generating Hugo site", "output", outputDir)
	generator := hugo.NewGenerator(cfg, outputDir)
	
	if err := generator.GenerateSite(docFiles); err != nil {
		return fmt.Errorf("site generation failed: %w", err)
	}

	slog.Info("Hugo site generated successfully", "output", outputDir)
	fmt.Println("Build completed successfully")
	return nil
}

// createLocalConfig creates a minimal configuration for building from a local docs directory
func (b *BuildCmd) createLocalConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Version = "2.0"
	
	cfg.Output.Directory = b.Output
	cfg.Output.Clean = true
	
	cfg.Hugo.Title = b.Title
	cfg.Hugo.Description = "Documentation built with DocBuilder"
	cfg.Hugo.BaseURL = "/"
	cfg.Hugo.Theme = b.Theme
	cfg.Hugo.EnableTransitions = true
	
	cfg.Build.RenderMode = config.RenderModeAlways
	
	// Single local repository entry pointing to DocsDir
	cfg.Repositories = []config.Repository{{
		URL:    b.DocsDir,
		Name:   "local",
		Branch: "",
		Paths:  []string{"."},
	}}
	
	return cfg
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
