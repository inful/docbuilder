package commands

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/alecthomas/kong"
)

// Global context passed to subcommands if we need to share global state later.
type Global struct {
	Logger *slog.Logger
}

// CLI definition & global flags - used by commands that need access to root config.
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

// ResolveOutputDir determines the final output directory based on CLI flag, config, and base_directory.
// Priority: CLI flag > config base_directory + directory > config directory
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

// CopyDir recursively copies a directory tree, handling cross-device scenarios
func CopyDir(src, dst string) error {
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
