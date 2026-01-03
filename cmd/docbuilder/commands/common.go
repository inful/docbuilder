package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// Global context passed to subcommands if we need to share global state later.
type Global struct {
	Logger *slog.Logger
}

// CLI definition & global flags - used by commands that need access to root config.
type CLI struct {
	Config  string           `short:"c" default:"config.yaml" env:"DOCBUILDER_CONFIG" help:"Configuration file path"`
	Verbose bool             `short:"v" env:"DOCBUILDER_VERBOSE" help:"Enable verbose logging"`
	Version kong.VersionFlag `name:"version" help:"Show version and exit"`

	Build    BuildCmd    `cmd:"" help:"Build documentation site from configured repositories"`
	Init     InitCmd     `cmd:"" help:"Initialize a new configuration file"`
	Discover DiscoverCmd `cmd:"" help:"Discover documentation files without building"`
	Lint     LintCmd     `cmd:"" help:"Lint documentation files for errors and style issues"`
	Daemon   DaemonCmd   `cmd:"" help:"Start daemon mode for continuous documentation updates"`
	Preview  PreviewCmd  `cmd:"" help:"Preview local docs with live reload (no git polling)"`
}

// AfterApply runs after flag parsing; setup logging once.
func (c *CLI) AfterApply() error {
	level := parseLogLevel(c.Verbose)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return nil
}

// parseLogLevel determines the log level from DOCBUILDER_LOG_LEVEL env var or verbose flag.
// Precedence: --verbose flag > DOCBUILDER_LOG_LEVEL > default (info).
func parseLogLevel(verbose bool) slog.Level {
	// Verbose flag takes precedence for backwards compatibility
	if verbose {
		return slog.LevelDebug
	}

	// Check DOCBUILDER_LOG_LEVEL environment variable
	if envLevel := os.Getenv("DOCBUILDER_LOG_LEVEL"); envLevel != "" {
		switch strings.ToLower(envLevel) {
		case "debug":
			return slog.LevelDebug
		case "info":
			return slog.LevelInfo
		case "warn", "warning":
			return slog.LevelWarn
		case "error":
			return slog.LevelError
		default:
			// Invalid value, log warning and use default
			fmt.Fprintf(os.Stderr, "Warning: Invalid DOCBUILDER_LOG_LEVEL=%q, using 'info'. Valid values: debug, info, warn, error\n", envLevel)
		}
	}

	// Default to info level
	return slog.LevelInfo
}

// ResolveOutputDir determines the final output directory based on CLI flag, config, and base_directory.
// Priority: CLI flag > config base_directory + directory > config directory.
func ResolveOutputDir(cliOutput string, cfg *config.Config) string {
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

// CopyDir recursively copies a directory tree, handling cross-device scenarios.
func CopyDir(src, dst string) error {
	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
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
			if err := CopyDir(srcPath, dstPath); err != nil {
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

// copyFile copies a single file from src to dst.
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

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Preserve file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// CreateWorkspace creates a workspace manager and initializes it.
// The caller is responsible for calling CleanupWorkspace when done.
func CreateWorkspace(cfg *config.Config) (*workspace.Manager, error) {
	wsDir := cfg.Build.WorkspaceDir
	wsManager := workspace.NewManager(wsDir)
	if err := wsManager.Create(); err != nil {
		return nil, err
	}
	return wsManager, nil
}

// CleanupWorkspace cleans up a workspace manager, logging any errors.
func CleanupWorkspace(wsManager *workspace.Manager) {
	if err := wsManager.Cleanup(); err != nil {
		slog.Warn("Failed to cleanup workspace", "error", err)
	}
}

// CreateGitClient creates a git client with the given workspace and config.
func CreateGitClient(wsManager *workspace.Manager, cfg *config.Config) (*git.Client, error) {
	gitClient := git.NewClient(wsManager.GetPath()).WithBuildConfig(&cfg.Build)
	if err := gitClient.EnsureWorkspace(); err != nil {
		return nil, err
	}
	return gitClient, nil
}

// ApplyAutoDiscovery applies forge auto-discovery if repositories are empty and forges are configured.
func ApplyAutoDiscovery(ctx context.Context, cfg *config.Config) error {
	if len(cfg.Repositories) == 0 && len(cfg.Forges) > 0 {
		repos, err := AutoDiscoverRepositories(ctx, cfg)
		if err != nil {
			return fmt.Errorf("auto-discovery failed: %w", err)
		}
		cfg.Repositories = repos
	}
	return nil
}

// AutoDiscoverRepositories builds a forge manager from v2 config and returns converted repositories.
func AutoDiscoverRepositories(ctx context.Context, v2cfg *config.Config) ([]config.Repository, error) {
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
		case config.ForgeLocal:
			// Local forges don't support auto-discovery (no remote API)
			slog.Warn("Local forge type does not support auto-discovery (skipping)", "name", f.Name)
			continue
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
