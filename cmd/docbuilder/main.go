package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
	"github.com/alecthomas/kong"
)

var CLI struct {
	Config  string `short:"c" help:"Configuration file path" default:"config.yaml"`
	Verbose bool   `short:"v" help:"Enable verbose logging"`

	Build struct {
		Output      string `short:"o" help:"Output directory for generated site" default:"./site"`
		Incremental bool   `short:"i" help:"Use incremental updates instead of fresh clone"`
	} `cmd:"" help:"Build documentation site from configured repositories"`

	Init struct {
		Force bool `help:"Overwrite existing configuration file"`
	} `cmd:"" help:"Initialize a new configuration file"`

	Discover struct {
		Repository string `short:"r" help:"Specific repository to discover (optional)"`
	} `cmd:"" help:"Discover documentation files without building"`

	Daemon struct {
		DataDir string `short:"d" help:"Data directory for daemon state" default:"./daemon-data"`
	} `cmd:"" help:"Start the daemon mode for continuous documentation updates"`
}

func main() {
	ctx := kong.Parse(&CLI)

	// Set up logging
	logLevel := slog.LevelInfo
	if CLI.Verbose {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Execute command
	switch ctx.Command() {
	case "build":
		// Support v2 config with auto-discovery. If file is v2 and repositories empty, perform forge discovery.
		isV2, _ := config.IsV2Config(CLI.Config)
		if isV2 {
			v2cfg, err := config.LoadV2(CLI.Config)
			if err != nil {
				slog.Error("Failed to load V2 configuration", "error", err)
				os.Exit(1)
			}
			legacyCfg := &config.Config{Hugo: v2cfg.Hugo, Output: v2cfg.Output}
			if len(legacyCfg.Repositories) == 0 && len(v2cfg.Forges) > 0 {
				if repos, err := autoDiscoverRepositories(context.Background(), v2cfg); err == nil {
					legacyCfg.Repositories = repos
				} else {
					slog.Error("Auto-discovery failed", "error", err)
					os.Exit(1)
				}
			}
			if err := runBuild(legacyCfg, CLI.Build.Output, CLI.Build.Incremental, CLI.Verbose); err != nil {
				slog.Error("Build failed", "error", err)
				os.Exit(1)
			}
		} else {
			cfg, err := config.Load(CLI.Config)
			if err != nil {
				slog.Error("Failed to load configuration", "error", err)
				os.Exit(1)
			}
			if err := runBuild(cfg, CLI.Build.Output, CLI.Build.Incremental, CLI.Verbose); err != nil {
				slog.Error("Build failed", "error", err)
				os.Exit(1)
			}
		}
	case "init":
		if err := runInit(CLI.Config, CLI.Init.Force); err != nil {
			slog.Error("Init failed", "error", err)
			os.Exit(1)
		}
	case "discover":
		isV2, _ := config.IsV2Config(CLI.Config)
		if isV2 {
			v2cfg, err := config.LoadV2(CLI.Config)
			if err != nil {
				slog.Error("Failed to load V2 configuration", "error", err)
				os.Exit(1)
			}
			legacyCfg := &config.Config{Hugo: v2cfg.Hugo, Output: v2cfg.Output}
			if len(legacyCfg.Repositories) == 0 && len(v2cfg.Forges) > 0 {
				if repos, err := autoDiscoverRepositories(context.Background(), v2cfg); err == nil {
					legacyCfg.Repositories = repos
				} else {
					slog.Error("Auto-discovery failed", "error", err)
					os.Exit(1)
				}
			}
			if err := runDiscover(legacyCfg, CLI.Discover.Repository); err != nil {
				slog.Error("Discover failed", "error", err)
				os.Exit(1)
			}
		} else {
			cfg, err := config.Load(CLI.Config)
			if err != nil {
				slog.Error("Failed to load configuration", "error", err)
				os.Exit(1)
			}
			if err := runDiscover(cfg, CLI.Discover.Repository); err != nil {
				slog.Error("Discover failed", "error", err)
				os.Exit(1)
			}
		}
	case "daemon":
		// Load V2 configuration for daemon command
		cfg, err := config.LoadV2(CLI.Config)
		if err != nil {
			slog.Error("Failed to load V2 configuration", "error", err)
			os.Exit(1)
		}
		if err := runDaemon(cfg, CLI.Daemon.DataDir); err != nil {
			slog.Error("Daemon failed", "error", err)
			os.Exit(1)
		}
	}
}

func runBuild(cfg *config.Config, outputDir string, incremental bool, verbose bool) error {
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
	discovery := docs.NewDiscovery(cfg.Repositories)

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
	return nil
}

func runInit(configPath string, force bool) error {
	slog.Info("Initializing configuration", "path", configPath, "force", force)
	return config.Init(configPath, force)
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
	discovery := docs.NewDiscovery(reposToProcess)
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

func runDaemon(cfg *config.V2Config, dataDir string) error {
	slog.Info("Starting daemon mode", "data_dir", dataDir)

	// Create main context for the daemon
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create and start the daemon with config file watching
	d, err := daemon.NewDaemonWithConfigFile(cfg, CLI.Config)
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
func autoDiscoverRepositories(ctx context.Context, v2cfg *config.V2Config) ([]config.Repository, error) {
	manager := forge.NewForgeManager()

	// Instantiate forge clients
	for _, f := range v2cfg.Forges {
		var client forge.ForgeClient
		var err error
		switch f.Type {
		case string(forge.ForgeTypeForgejo):
			client, err = forge.NewForgejoClient(f)
		// Future: add github/gitlab here
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
