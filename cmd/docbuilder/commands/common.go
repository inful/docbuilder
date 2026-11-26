package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/git"
	"git.home.luguber.info/inful/docbuilder/internal/workspace"
)

// DefaultConfigFileName is the name of the configuration file every
// command reads.
const DefaultConfigFileName = "config.yaml"

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
	Template TemplateCmd `cmd:"" help:"Create documentation from templates"`
	Reingest ReingestCmd `cmd:"" help:"Send the whole corpus to ragabast again"`
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

// LoadEnvFile loads environment variables from the .env files.
// Returns nil if successful; returns error if no file exists but doesn't fail the command.
//
// The local file is loaded first, so it wins over the shared one, and
// the environment of the process wins over both: a value that is
// already set is not overwritten.
func LoadEnvFile() error {
	loaded := false
	for _, p := range []string{".env.local", ".env"} {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := godotenv.Load(p); err != nil {
			return fmt.Errorf("failed loading %s: %w", p, err)
		}
		loaded = true
	}
	if !loaded {
		return errors.New("no .env file found")
	}
	return nil
}

// ResolveOutputDir determines the final output directory based on CLI flag, config, and base_directory.
// Priority: CLI flag > config base_directory + directory > config directory.
func ResolveOutputDir(cliOutput *string, cfg *config.Config) string {
	// The flag wins: the operator typed it. A flag that was not given
	// is not the same as a flag that was given the default value.
	if cliOutput != nil && *cliOutput != "" {
		return *cliOutput
	}

	// The configuration decides, with the same rules the daemon and
	// the server use: the site is published in the same place whoever
	// looks for it.
	return cfg.ResolveOutputDir()
}

// outputFlag returns the value of the output flag, or the default when
// the flag was not given.
func outputFlag(output *string) string {
	if output != nil && *output != "" {
		return *output
	}
	return "./site"
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
	// #nosec G304 -- src/dst paths are validated by caller
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = srcFile.Close()
	}()

	// #nosec G304 -- dst path is validated by caller
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
	// The local forge has no remote API, so it takes no part in the
	// discovery.
	remote := make([]*forge.Config, 0, len(v2cfg.Forges))
	for _, f := range v2cfg.Forges {
		if f.Type == config.ForgeLocal {
			slog.Warn("Local forge type does not support auto-discovery (skipping)", "name", f.Name)
			continue
		}
		remote = append(remote, f)
	}

	// A forge that cannot be built is an error: a site without the forge
	// of the operator looks like a site without repositories.
	manager, err := forge.CreateForgeManager(remote)
	if err != nil {
		return nil, fmt.Errorf("build the forge manager: %w", err)
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
