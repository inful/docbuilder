package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	tr "git.home.luguber.info/inful/docbuilder/internal/hugo/transforms"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
	"github.com/alecthomas/kong"
)

// Set at build time with: -ldflags "-X main.version=1.0.0-rc1"
var version = "dev"

// Root CLI definition & global flags.
type CLI struct {
	Config  string           `short:"c" help:"Configuration file path" default:"config.yaml"`
	Verbose bool             `short:"v" help:"Enable verbose logging"`
	Version kong.VersionFlag `name:"version" help:"Show version and exit"`

	Build     BuildCmd     `cmd:"" help:"Build documentation site from configured repositories"`
	Init      InitCmd      `cmd:"" help:"Initialize a new configuration file"`
	Discover  DiscoverCmd  `cmd:"" help:"Discover documentation files without building"`
	Daemon    DaemonCmd    `cmd:"" help:"Start daemon mode for continuous documentation updates"`
	Preview   PreviewCmd   `cmd:"" help:"Preview local docs with live reload (no git polling)"`
	Generate  GenerateCmd  `cmd:"" help:"Generate static site from local docs directory (for CI/CD)"`
	Visualize VisualizeCmd `cmd:"" help:"Visualize the transform pipeline (text, mermaid, dot, json)"`
}

// Common context passed to subcommands if we need to share global state later.
type Global struct {
	Logger *slog.Logger
}

// BuildCmd implements the 'build' command.
type BuildCmd struct {
	Output      string `short:"o" help:"Output directory for generated site" default:"./site"`
	Incremental bool   `short:"i" help:"Use incremental updates instead of fresh clone"`
	RenderMode  string `name:"render-mode" help:"Override build.render_mode (auto|always|never). Precedence: --render-mode > env vars (skip/run) > config."`
}

// InitCmd implements the 'init' command.
type InitCmd struct {
	Force  bool   `help:"Overwrite existing configuration file"`
	Output string `help:"Output directory for generated config file" name:"output" short:"o"`
}

// DiscoverCmd implements the 'discover' command.
type DiscoverCmd struct {
	Repository string `short:"r" help:"Specific repository to discover (optional)"`
}

// DaemonCmd implements the 'daemon' command.
type DaemonCmd struct {
	DataDir string `short:"d" help:"Data directory for daemon state" default:"./daemon-data"`
}

// GenerateCmd implements the 'generate' command for CI/CD pipelines.
type GenerateCmd struct {
	DocsDir string `name:"docs-dir" short:"d" help:"Path to local docs directory" default:"./docs"`
	Output  string `short:"o" help:"Output directory for generated site" default:"./public"`
	Theme   string `name:"theme" help:"Hugo theme to use (hextra or docsy)" default:"hextra"`
	Title   string `name:"title" help:"Site title" default:"Documentation"`
	BaseURL string `name:"base-url" help:"Base URL for the site" default:"/"`
	Render  bool   `name:"render" help:"Run Hugo to render the site" default:"true"`
}

// VisualizeCmd implements the 'visualize' command.
type VisualizeCmd struct {
	Format string `short:"f" help:"Output format: text, mermaid, dot, json" default:"text" enum:"text,mermaid,dot,json"`
	Output string `short:"o" help:"Output file path (optional, prints to stdout if not specified)"`
	List   bool   `short:"l" help:"List available formats and exit"`
}

// AfterApply runs after flag parsing; setup logging once.
// nolint:unparam // AfterApply currently never returns an error.
func (c *CLI) AfterApply() error {
	level := slog.LevelInfo
	if c.Verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return nil
}

// resolveOutputDir determines the final output directory based on CLI flag, config, and base_directory.
// Priority: CLI flag > config base_directory + directory > config directory
func resolveOutputDir(cliOutput string, cfg *config.Config) string {
	// If CLI flag is provided and not the default, use it directly
	if cliOutput != "" && cliOutput != "./site" {
		return cliOutput
	}

	// If base_directory is configured, combine it with directory
	if cfg.Output.BaseDirectory != "" {
		return filepath.Join(cfg.Output.BaseDirectory, cfg.Output.Directory)
	}

	// Otherwise use configured directory (or CLI default)
	if cfg.Output.Directory != "" {
		return cfg.Output.Directory
	}

	return cliOutput // fallback to CLI default
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
	if len(cfg.Repositories) == 0 && len(cfg.Forges) > 0 {
		if repos, err := autoDiscoverRepositories(context.Background(), cfg); err == nil {
			cfg.Repositories = repos
		} else {
			return fmt.Errorf("auto-discovery failed: %w", err)
		}
	}

	// Resolve output directory with base_directory support
	outputDir := resolveOutputDir(b.Output, cfg)
	return runBuild(cfg, outputDir, b.Incremental, root.Verbose)
}

func (i *InitCmd) Run(_ *Global, root *CLI) error {
	// If the user specified an output directory, place the config there as "docbuilder.yaml".
	if i.Output != "" {
		cfgPath := filepath.Join(i.Output, "docbuilder.yaml")
		return runInit(cfgPath, i.Force)
	}
	return runInit(root.Config, i.Force)
}

func (d *DiscoverCmd) Run(_ *Global, root *CLI) error {
	cfg, err := config.Load(root.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if len(cfg.Repositories) == 0 && len(cfg.Forges) > 0 {
		if repos, err := autoDiscoverRepositories(context.Background(), cfg); err == nil {
			cfg.Repositories = repos
		} else {
			return fmt.Errorf("auto-discovery failed: %w", err)
		}
	}
	return runDiscover(cfg, d.Repository)
}

func (d *DaemonCmd) Run(_ *Global, root *CLI) error {
	cfg, err := config.Load(root.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return runDaemon(cfg, d.DataDir, root.Config)
}

func main() {
	cli := &CLI{}
	parser := kong.Parse(cli,
		kong.Description("DocBuilder: aggregate multi-repo documentation into a Hugo site."),
		kong.Vars{"version": version},
	)

	// Set up structured error handling
	logger := slog.Default()
	errorAdapter := errors.NewCLIErrorAdapter(cli.Verbose, logger)

	// Prepare globals (currently just logger already installed in AfterApply)
	globals := &Global{Logger: logger}

	// Run command and handle errors uniformly
	if err := parser.Run(globals, cli); err != nil {
		errorAdapter.HandleError(err)
	}
}

// PreviewCmd starts a local server watching a docs directory without forge polling.
type PreviewCmd struct {
	DocsDir   string `name:"docs-dir" short:"d" help:"Path to local docs directory to watch." default:"./docs"`
	OutputDir string `name:"output" short:"o" help:"Output directory for the generated site (defaults to temp)." default:""`
	Theme     string `name:"theme" help:"Hugo theme to use (hextra or docsy)." default:"hextra"`
	Title     string `name:"title" help:"Site title." default:"Local Preview"`
	BaseURL   string `name:"base-url" help:"Base URL used in Hugo config." default:"http://localhost:1316"`
	Port      int    `name:"port" help:"Docs server port." default:"1316"`
}

func (p *PreviewCmd) Run(_ *Global, _ *CLI) error {
	// Setup signal-based context for graceful shutdown
	sigctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	// Build a minimal in-memory config
	cfg := &config.Config{}
	cfg.Version = "2.0"

	// Initialize monitoring config with defaults
	cfg.Monitoring = &config.MonitoringConfig{
		Health: config.MonitoringHealth{
			Path: "/health",
		},
		Metrics: config.MonitoringMetrics{
			Enabled: false,
			Path:    "/metrics",
		},
	}

	// Initialize daemon config
	cfg.Daemon = &config.DaemonConfig{
		HTTP: config.HTTPConfig{
			DocsPort:    p.Port,
			WebhookPort: p.Port + 1,
			AdminPort:   p.Port + 2,
		},
	}

	// If no output provided, create a temporary directory
	outDir := p.OutputDir
	tempOut := ""
	if outDir == "" {
		tmp, err := os.MkdirTemp("", "docbuilder-preview-*")
		if err != nil {
			return fmt.Errorf("create temp output: %w", err)
		}
		outDir = tmp
		tempOut = tmp
		slog.Info("Using temporary output directory for preview", "output", outDir)
		fmt.Println("Preview output directory:", outDir)
	}
	cfg.Output.Directory = outDir
	cfg.Output.Clean = true
	cfg.Hugo.Title = p.Title
	cfg.Hugo.Description = "DocBuilder local preview"
	cfg.Hugo.BaseURL = p.BaseURL
	cfg.Hugo.Theme = p.Theme
	cfg.Build.RenderMode = config.RenderModeAlways
	cfg.Build.LiveReload = true

	// Single local repository entry pointing to DocsDir
	cfg.Repositories = []config.Repository{{
		URL:    p.DocsDir,
		Name:   "local",
		Branch: "",
		Paths:  []string{"."},
	}}

	return daemon.StartLocalPreview(sigctx, cfg, p.Port, tempOut)
}

func (g *GenerateCmd) Run(_ *Global, _ *CLI) error {
	fmt.Println("Starting static site generation")

	// Validate docs directory exists
	if _, err := os.Stat(g.DocsDir); os.IsNotExist(err) {
		return fmt.Errorf("docs directory does not exist: %s", g.DocsDir)
	}

	slog.Info("Generating static site from local docs",
		"docs_dir", g.DocsDir,
		"output", g.Output,
		"theme", g.Theme,
		"render", g.Render)

	// Use a temporary directory for the Hugo project if rendering
	hugoProjectDir := g.Output
	var tempDir string

	if g.Render {
		// Create temp directory for Hugo project
		tmp, err := os.MkdirTemp("", "docbuilder-generate-*")
		if err != nil {
			return fmt.Errorf("create temp directory: %w", err)
		}
		tempDir = tmp
		hugoProjectDir = tmp
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				slog.Warn("Failed to cleanup temp directory", "error", err)
			}
		}()
		slog.Debug("Using temporary Hugo project directory", "path", hugoProjectDir)
	}

	// Build minimal config for local generation
	cfg := &config.Config{}
	cfg.Version = "2.0"
	cfg.Output.Directory = hugoProjectDir
	cfg.Output.Clean = true
	cfg.Hugo.Title = g.Title
	cfg.Hugo.Description = "Generated documentation"
	cfg.Hugo.BaseURL = g.BaseURL
	cfg.Hugo.Theme = g.Theme

	if g.Render {
		cfg.Build.RenderMode = config.RenderModeAlways
		// Disable GitInfo for CI/CD generation (no git repo in temp dir)
		cfg.Build.LiveReload = true // This disables GitInfo
	} else {
		cfg.Build.RenderMode = config.RenderModeNever
	}

	// Single local repository entry
	cfg.Repositories = []config.Repository{{
		URL:    g.DocsDir,
		Name:   "docs",
		Branch: "",
		Paths:  []string{"."},
	}}

	// Create workspace for processing
	wsDir := cfg.Build.WorkspaceDir
	if wsDir == "" {
		wsDir = "" // Will use temp dir
	}
	wsManager := workspace.NewManager(wsDir)
	if err := wsManager.Create(); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	defer func() {
		if err := wsManager.Cleanup(); err != nil {
			slog.Warn("Failed to cleanup workspace", "error", err)
		}
	}()

	// Create Git client
	gitClient := git.NewClient(wsManager.GetPath())
	if err := gitClient.EnsureWorkspace(); err != nil {
		return fmt.Errorf("ensure workspace: %w", err)
	}

	// Process the local docs directory
	slog.Info("Processing local documentation", "path", g.DocsDir)

	// For local directories, we can just use the path directly
	absDocsDir, err := filepath.Abs(g.DocsDir)
	if err != nil {
		return fmt.Errorf("resolve docs directory: %w", err)
	}

	repoPaths := map[string]string{
		"docs": absDocsDir,
	}

	// Discover documentation files
	slog.Info("Discovering documentation files")
	discovery := docs.NewDiscovery(cfg.Repositories, &cfg.Build)
	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return fmt.Errorf("discover docs: %w", err)
	}

	if len(docFiles) == 0 {
		slog.Warn("No documentation files found", "path", g.DocsDir)
		fmt.Println("Warning: No documentation files found")
		return nil
	}

	slog.Info("Documentation files discovered", "count", len(docFiles))

	// Generate Hugo site
	slog.Info("Generating Hugo site", "output", hugoProjectDir)
	generator := hugo.NewGenerator(cfg, hugoProjectDir)

	if err := generator.GenerateSite(docFiles); err != nil {
		return fmt.Errorf("generate site: %w", err)
	}

	// If rendering, move the public directory to the final output location
	if g.Render {
		publicDir := filepath.Join(hugoProjectDir, "public")

		// Verify the public directory exists
		if _, err := os.Stat(publicDir); os.IsNotExist(err) {
			return fmt.Errorf("hugo did not generate public directory")
		}

		// Clean the output directory if it exists
		if err := os.RemoveAll(g.Output); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clean output directory: %w", err)
		}

		// Create parent directory if needed
		if err := os.MkdirAll(filepath.Dir(g.Output), 0o755); err != nil {
			return fmt.Errorf("failed to create output parent directory: %w", err)
		}

		// Copy public directory to final output using recursive copy
		slog.Info("Copying rendered site to output", "from", publicDir, "to", g.Output)
		if err := copyDir(publicDir, g.Output); err != nil {
			return fmt.Errorf("failed to copy public directory: %w", err)
		}

		fmt.Printf("Static site generated successfully at: %s\n", g.Output)
	} else {
		fmt.Printf("Hugo project generated at: %s\n", g.Output)
		fmt.Println("Note: Run hugo in this directory to build the site")
	}

	return nil
}

func runBuild(cfg *config.Config, outputDir string, incremental, verbose bool) error {
	// Provide friendly user-facing messages on stdout for CLI integration tests.
	fmt.Println("Starting DocBuilder build")

	// Set logging level
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	slog.Info("Starting documentation build",
		"output", outputDir,
		"repositories", len(cfg.Repositories),
		"incremental", incremental)

	// Create workspace manager
	wsDir := cfg.Build.WorkspaceDir
	if wsDir == "" {
		wsDir = "" // Will use temp dir
	}
	wsManager := workspace.NewManager(wsDir)
	if err := wsManager.Create(); err != nil {
		return err
	}
	defer func() {
		if err := wsManager.Cleanup(); err != nil {
			slog.Warn("Failed to cleanup workspace", "error", err)
		}
	}()

	// Create Git client
	gitClient := git.NewClient(wsManager.GetPath())
	if err := gitClient.EnsureWorkspace(); err != nil {
		return err
	}

	// Clone/update all repositories
	repoPaths := make(map[string]string)
	repositoriesSkipped := 0
	for _, repo := range cfg.Repositories {
		slog.Info("Processing repository", "name", repo.Name, "url", repo.URL)

		var repoPath string
		var err error

		if incremental {
			repoPath, err = gitClient.UpdateRepository(repo)
		} else {
			repoPath, err = gitClient.CloneRepository(repo)
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
			"total", len(cfg.Repositories))
	}

	if len(repoPaths) == 0 {
		return fmt.Errorf("no repositories could be cloned successfully")
	}

	slog.Info("All repositories processed", "successful", len(repoPaths), "skipped", repositoriesSkipped)

	// Discover documentation files
	slog.Info("Starting documentation discovery")
	discovery := docs.NewDiscovery(cfg.Repositories, &cfg.Build)

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

func runInit(configPath string, force bool) error {
	// Provide friendly user-facing messages on stdout for CLI integration tests.
	fmt.Println("Initializing DocBuilder project")
	fmt.Printf("Writing configuration to %s\n", configPath)
	if err := config.Init(configPath, force); err != nil {
		fmt.Println("Initialization failed")
		return err
	}
	fmt.Println("initialized successfully")
	slog.Info("Initializing configuration", "path", configPath, "force", force)
	return nil
}

func runDiscover(cfg *config.Config, specificRepo string) error {
	slog.Info("Starting documentation discovery", "repositories", len(cfg.Repositories))

	// Create workspace manager
	wsDir := cfg.Build.WorkspaceDir
	if wsDir == "" {
		wsDir = "" // Will use temp dir
	}
	wsManager := workspace.NewManager(wsDir)
	if err := wsManager.Create(); err != nil {
		return err
	}
	defer func() {
		if err := wsManager.Cleanup(); err != nil {
			slog.Warn("Failed to cleanup workspace", "error", err)
		}
	}()

	// Create Git client
	gitClient := git.NewClient(wsManager.GetPath())
	if err := gitClient.EnsureWorkspace(); err != nil {
		return err
	}

	// Filter repositories if specific one requested
	var reposToProcess []config.Repository
	if specificRepo != "" {
		for _, repo := range cfg.Repositories {
			if repo.Name == specificRepo {
				reposToProcess = []config.Repository{repo}
				break
			}
		}
		if len(reposToProcess) == 0 {
			return fmt.Errorf("repository '%s' not found in configuration", specificRepo)
		}
	} else {
		reposToProcess = cfg.Repositories
	}

	// Clone repositories
	repoPaths := make(map[string]string)
	for _, repo := range reposToProcess {
		slog.Info("Cloning repository", "name", repo.Name, "url", repo.URL)

		repoPath, err := gitClient.CloneRepository(repo)
		if err != nil {
			slog.Error("Failed to clone repository", "name", repo.Name, "error", err)
			return err
		}

		repoPaths[repo.Name] = repoPath
	}

	// Discover documentation files
	discovery := docs.NewDiscovery(reposToProcess, &cfg.Build)
	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return err
	}

	// Print discovery results
	slog.Info("Discovery completed", "total_files", len(docFiles))

	filesByRepo := discovery.GetDocFilesByRepository()
	for repoName, files := range filesByRepo {
		slog.Info("Repository files", "repository", repoName, "count", len(files))
		for _, file := range files {
			slog.Info("  File discovered",
				"path", file.RelativePath,
				"section", file.Section,
				"hugo_path", file.GetHugoPath())
		}
	}

	return nil
}

func runDaemon(cfg *config.Config, dataDir, configPath string) error {
	slog.Info("Starting daemon mode", "data_dir", dataDir)

	// Create main context for the daemon
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create and start the daemon with config file watching
	d, err := daemon.NewDaemonWithConfigFile(cfg, configPath)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	// Start daemon in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(ctx)
	}()

	slog.Info("Daemon started, waiting for shutdown signal...")

	// Wait for either error or shutdown signal
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("daemon error: %w", err)
		}
	case <-ctx.Done():
		slog.Info("Shutdown signal received, stopping daemon...")
	}

	// Stop daemon gracefully
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()

	if err := d.Stop(stopCtx); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	slog.Info("Daemon stopped successfully")
	return nil
}

// autoDiscoverRepositories builds a forge manager from v2 config and returns converted repositories.
func autoDiscoverRepositories(ctx context.Context, v2cfg *config.Config) ([]config.Repository, error) {
	manager := forge.NewForgeManager()

	// Instantiate forge clients
	for _, f := range v2cfg.Forges {
		var client forge.Client
		var err error
		switch f.Type {
		case config.ForgeForgejo:
			client, err = forge.NewForgejoClient(f)
		case config.ForgeGitHub:
			client, err = forge.NewGitHubClient(f)
		case config.ForgeGitLab:
			client, err = forge.NewGitLabClient(f)
		default:
			slog.Warn("Unsupported forge type for auto-discovery (skipping)", "type", f.Type, "name", f.Name)
			continue
		}
		if err != nil {
			slog.Error("Failed to create forge client", "forge", f.Name, "error", err)
			continue
		}
		manager.AddForge(f, client)
	}

	filtering := v2cfg.Filtering
	if filtering == nil {
		filtering = &config.FilteringConfig{}
	}

	service := forge.NewDiscoveryService(manager, filtering)
	result, err := service.DiscoverAll(ctx)
	if err != nil {
		return nil, err
	}
	repos := service.ConvertToConfigRepositories(result.Repositories, manager)
	slog.Info("Auto-discovery completed", "repositories", len(repos))
	return repos, nil
}

// copyDir recursively copies a directory tree, handling cross-device scenarios
func copyDir(src, dst string) error {
	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read all directory contents
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = srcFile.Close()
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = dstFile.Close()
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Preserve file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// Run executes the visualize command.
func (cmd *VisualizeCmd) Run(_ *Global, _ *CLI) error {
	// Import defaults package to register transforms
	_ = hugo.NewGenerator(&config.Config{}, "")
	
	// Handle --list flag
	if cmd.List {
		fmt.Println("Available visualization formats:")
		fmt.Println()
		for _, format := range tr.GetSupportedFormats() {
			desc := tr.GetFormatDescription(format)
			fmt.Printf("  %-10s %s\n", format, desc)
		}
		fmt.Println()
		fmt.Println("Usage examples:")
		fmt.Println("  docbuilder visualize                    # Text format to stdout")
		fmt.Println("  docbuilder visualize -f mermaid         # Mermaid diagram to stdout")
		fmt.Println("  docbuilder visualize -f dot -o pipe.dot # DOT format to file")
		return nil
	}
	
	// Generate visualization
	output, err := tr.VisualizePipeline(tr.VisualizationFormat(cmd.Format))
	if err != nil {
		return fmt.Errorf("failed to visualize pipeline: %w", err)
	}
	
	// Write to file or stdout
	if cmd.Output != "" {
		if err := os.WriteFile(cmd.Output, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		slog.Info("Pipeline visualization written", "file", cmd.Output, "format", cmd.Format)
	} else {
		fmt.Print(output)
	}
	
	return nil
}
