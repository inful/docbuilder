package main

import (
	"context"
	"fmt"
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

	Build    BuildCmd    `cmd:"" help:"Build documentation site from configured repositories"`
	Init     InitCmd     `cmd:"" help:"Initialize a new configuration file"`
	Discover DiscoverCmd `cmd:"" help:"Discover documentation files without building"`
	Daemon   DaemonCmd   `cmd:"" help:"Start daemon mode for continuous documentation updates"`
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

// AfterApply runs after flag parsing; setup logging once.
func (c *CLI) AfterApply() error {
	level := slog.LevelInfo
	if c.Verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return nil
}

func (b *BuildCmd) Run(globals *Global, root *CLI) error {
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
	return runBuild(cfg, b.Output, b.Incremental, root.Verbose)
}

func (i *InitCmd) Run(globals *Global, root *CLI) error {
	// If the user specified an output directory, place the config there as "docbuilder.yaml".
	if i.Output != "" {
		cfgPath := filepath.Join(i.Output, "docbuilder.yaml")
		return runInit(cfgPath, i.Force)
	}
	return runInit(root.Config, i.Force)
}

func (d *DiscoverCmd) Run(globals *Global, root *CLI) error {
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

func (d *DaemonCmd) Run(globals *Global, root *CLI) error {
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

func runBuild(cfg *config.Config, outputDir string, incremental bool, verbose bool) error {
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
	wsManager := workspace.NewManager("")
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
			return err
		}

		repoPaths[repo.Name] = repoPath
		slog.Info("Repository processed", "name", repo.Name, "path", repoPath)
	}

	slog.Info("All repositories processed successfully", "count", len(repoPaths))

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
	wsManager := workspace.NewManager("")
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

func runDaemon(cfg *config.Config, dataDir string, configPath string) error {
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
		var client forge.ForgeClient
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
