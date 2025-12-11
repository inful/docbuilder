package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/daemon"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/services"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// CommandExecutor provides a service-oriented interface for CLI command execution
type CommandExecutor interface {
	services.ManagedService
	ExecuteBuild(ctx context.Context, req BuildRequest) foundation.Result[BuildResponse, error]
	ExecuteInit(ctx context.Context, req InitRequest) foundation.Result[InitResponse, error]
	ExecuteDiscover(ctx context.Context, req DiscoverRequest) foundation.Result[DiscoverResponse, error]
	ExecuteDaemon(ctx context.Context, req DaemonRequest) foundation.Result[DaemonResponse, error]
}

// Request/Response types for each command

type BuildRequest struct {
	ConfigPath  string
	OutputDir   string
	Incremental bool
	RenderMode  string
	Verbose     bool
}

type BuildResponse struct {
	OutputPath    string
	FilesBuilt    int
	Repositories  int
	BuildDuration time.Duration
}

type InitRequest struct {
	ConfigPath string
	Force      bool
}

type InitResponse struct {
	ConfigPath string
	Created    bool
}

type DiscoverRequest struct {
	ConfigPath   string
	SpecificRepo string
}

type DiscoverResponse struct {
	TotalFiles       int
	Repositories     map[string]int // repo name -> file count
	DiscoveryResults []docs.DocFile
}

type DaemonRequest struct {
	ConfigPath string
	DataDir    string
}

type DaemonResponse struct {
	StartTime time.Time
	Stopped   bool
}

// DefaultCommandExecutor implements the CommandExecutor interface
type DefaultCommandExecutor struct {
	name         string
	buildService build.BuildService
}

// NewCommandExecutor creates a new command executor service
func NewCommandExecutor(name string) *DefaultCommandExecutor {
	return &DefaultCommandExecutor{
		name:         name,
		buildService: createDefaultBuildService(),
	}
}

// createDefaultBuildService creates a BuildService with the hugo generator factory.
func createDefaultBuildService() build.BuildService {
	return build.NewBuildService().
		WithHugoGeneratorFactory(func(cfg any, outputDir string) build.HugoGenerator {
			if c, ok := cfg.(*config.Config); ok {
				return hugo.NewGenerator(c, outputDir)
			}
			return nil
		})
}

// WithBuildService allows injecting a custom BuildService (for testing).
func (e *DefaultCommandExecutor) WithBuildService(svc build.BuildService) *DefaultCommandExecutor {
	e.buildService = svc
	return e
}

// Service interface implementation
func (e *DefaultCommandExecutor) Name() string {
	return e.name
}

func (e *DefaultCommandExecutor) Start(_ context.Context) error {
	slog.Debug("Command executor service started", "service", e.name)
	return nil
}

func (e *DefaultCommandExecutor) Stop(_ context.Context) error {
	slog.Debug("Command executor service stopped", "service", e.name)
	return nil
}

func (e *DefaultCommandExecutor) HealthCheck(_ context.Context) services.HealthStatus {
	return services.HealthStatus{
		Status:  "healthy",
		Message: "Command executor ready",
		CheckAt: time.Now(),
	}
}

// Command execution implementations

func (e *DefaultCommandExecutor) ExecuteBuild(ctx context.Context, req BuildRequest) foundation.Result[BuildResponse, error] {
	// Load configuration
	cfg, err := config.Load(req.ConfigPath)
	if err != nil {
		return foundation.Err[BuildResponse](fmt.Errorf("load config: %w", err))
	}

	// Apply CLI render mode override
	if req.RenderMode != "" {
		if rm := config.NormalizeRenderMode(req.RenderMode); rm != "" {
			cfg.Build.RenderMode = rm
			slog.Info("Render mode overridden via CLI flag", "mode", rm)
		} else {
			slog.Warn("Ignoring invalid --render-mode value", "value", req.RenderMode)
		}
	}

	// Auto-discover repositories if needed
	if len(cfg.Repositories) == 0 && len(cfg.Forges) > 0 {
		if repos, err := e.autoDiscoverRepositories(ctx, cfg); err == nil {
			cfg.Repositories = repos
		} else {
			return foundation.Err[BuildResponse](fmt.Errorf("auto-discovery failed: %w", err))
		}
	}

	// Set logging level
	level := slog.LevelInfo
	if req.Verbose {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// Delegate to BuildService
	buildReq := build.BuildRequest{
		Config:      cfg,
		OutputDir:   req.OutputDir,
		Incremental: req.Incremental,
		Options: build.BuildOptions{
			Verbose: req.Verbose,
		},
	}

	result, err := e.buildService.Run(ctx, buildReq)
	if err != nil {
		return foundation.Err[BuildResponse](err)
	}

	return foundation.Ok[BuildResponse, error](BuildResponse{
		OutputPath:    result.OutputPath,
		FilesBuilt:    result.FilesProcessed,
		Repositories:  result.Repositories,
		BuildDuration: result.Duration,
	})
}

func (e *DefaultCommandExecutor) ExecuteInit(_ context.Context, req InitRequest) foundation.Result[InitResponse, error] {
	slog.Info("Initializing configuration", "path", req.ConfigPath, "force", req.Force)

	err := config.Init(req.ConfigPath, req.Force)
	if err != nil {
		return foundation.Err[InitResponse](err)
	}

	return foundation.Ok[InitResponse, error](InitResponse{
		ConfigPath: req.ConfigPath,
		Created:    true,
	})
}

func (e *DefaultCommandExecutor) ExecuteDiscover(ctx context.Context, req DiscoverRequest) foundation.Result[DiscoverResponse, error] {
	// Load configuration
	cfg, err := config.Load(req.ConfigPath)
	if err != nil {
		return foundation.Err[DiscoverResponse](fmt.Errorf("load config: %w", err))
	}

	// Auto-discover repositories if needed
	if len(cfg.Repositories) == 0 && len(cfg.Forges) > 0 {
		if repos, err := e.autoDiscoverRepositories(ctx, cfg); err == nil {
			cfg.Repositories = repos
		} else {
			return foundation.Err[DiscoverResponse](fmt.Errorf("auto-discovery failed: %w", err))
		}
	}

	slog.Info("Starting documentation discovery", "repositories", len(cfg.Repositories))

	// Create workspace manager
	wsManager := workspace.NewManager("")
	if err := wsManager.Create(); err != nil {
		return foundation.Err[DiscoverResponse](err)
	}
	defer func() {
		if err := wsManager.Cleanup(); err != nil {
			slog.Warn("Failed to cleanup workspace", "error", err)
		}
	}()

	// Create Git client
	gitClient := git.NewClient(wsManager.GetPath())
	if err := gitClient.EnsureWorkspace(); err != nil {
		return foundation.Err[DiscoverResponse](err)
	}

	// Filter repositories if specific one requested
	var reposToProcess []config.Repository
	if req.SpecificRepo != "" {
		for _, repo := range cfg.Repositories {
			if repo.Name == req.SpecificRepo {
				reposToProcess = []config.Repository{repo}
				break
			}
		}
		if len(reposToProcess) == 0 {
			return foundation.Err[DiscoverResponse](fmt.Errorf("repository '%s' not found in configuration", req.SpecificRepo))
		}
	} else {
		reposToProcess = cfg.Repositories
	}

	// Clone repositories
	repoPaths := make(map[string]string)
	for _, repo := range reposToProcess {
		slog.Info("Cloning repository", "name", repo.Name, "url", repo.URL)

		repoPath, err := gitClient.CloneRepo(repo)
		if err != nil {
			slog.Error("Failed to clone repository", "name", repo.Name, "error", err)
			return foundation.Err[DiscoverResponse](err)
		}

		repoPaths[repo.Name] = repoPath
	}

	// Discover documentation files
	discovery := docs.NewDiscovery(reposToProcess, &cfg.Build)
	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		return foundation.Err[DiscoverResponse](err)
	}

	// Print discovery results
	slog.Info("Discovery completed", "total_files", len(docFiles))

	filesByRepo := discovery.GetDocFilesByRepository()
	repositories := make(map[string]int)
	for repoName, files := range filesByRepo {
		repositories[repoName] = len(files)
		slog.Info("Repository files", "repository", repoName, "count", len(files))
		for _, file := range files {
			slog.Info("  File discovered",
				"path", file.RelativePath,
				"section", file.Section,
				"hugo_path", file.GetHugoPath())
		}
	}

	return foundation.Ok[DiscoverResponse, error](DiscoverResponse{
		TotalFiles:       len(docFiles),
		Repositories:     repositories,
		DiscoveryResults: docFiles,
	})
}

func (e *DefaultCommandExecutor) ExecuteDaemon(ctx context.Context, req DaemonRequest) foundation.Result[DaemonResponse, error] {
	startTime := time.Now()
	slog.Info("Starting daemon mode", "data_dir", req.DataDir)

	// Load configuration
	cfg, err := config.Load(req.ConfigPath)
	if err != nil {
		return foundation.Err[DaemonResponse](fmt.Errorf("load config: %w", err))
	}

	// Create main context for the daemon
	daemonCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create and start the daemon with config file watching
	d, err := daemon.NewDaemonWithConfigFile(cfg, req.ConfigPath)
	if err != nil {
		return foundation.Err[DaemonResponse](fmt.Errorf("failed to create daemon: %w", err))
	}

	// Start daemon in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- d.Start(daemonCtx)
	}()

	slog.Info("Daemon started, waiting for shutdown signal...")

	// Wait for either error or shutdown signal
	select {
	case err := <-errChan:
		if err != nil {
			return foundation.Err[DaemonResponse](fmt.Errorf("daemon error: %w", err))
		}
	case <-daemonCtx.Done():
		slog.Info("Shutdown signal received, stopping daemon...")
	}

	// Stop daemon gracefully
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()

	if err := d.Stop(stopCtx); err != nil {
		return foundation.Err[DaemonResponse](fmt.Errorf("failed to stop daemon: %w", err))
	}

	slog.Info("Daemon stopped successfully")

	return foundation.Ok[DaemonResponse, error](DaemonResponse{
		StartTime: startTime,
		Stopped:   true,
	})
}

// Helper method for auto-discovery
func (e *DefaultCommandExecutor) autoDiscoverRepositories(ctx context.Context, v2cfg *config.Config) ([]config.Repository, error) {
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
