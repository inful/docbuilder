package hugo

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
)

// Renderer abstracts how the final static site rendering step is performed after
// Hugo project scaffolding. This allows swapping out the external hugo binary
// (BinaryRenderer) with alternative strategies (e.g., no-op for tests, remote
// render service, in-process library) without changing stage orchestration.
//
// Contract:
//
//	Execute(rootDir string) error  -> perform rendering inside provided directory.
//	Enabled(cfg *config.Config) bool -> determines if rendering should run (allows
//	  renderer-level gating beyond global build.render_mode semantics)
//
// Errors returned are surfaced as warnings (non-fatal) unless future policy changes.
type Renderer interface {
	Execute(rootDir string) error
}

// BinaryRenderer invokes the `hugo` binary present on PATH.
type BinaryRenderer struct{}

func (b *BinaryRenderer) Execute(rootDir string) error {
	if _, err := exec.LookPath("hugo"); err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrHugoBinaryNotFound, err)
	}

	// Check staging directory exists before Hugo runs
	if stat, err := os.Stat(rootDir); err != nil {
		slog.Error("Staging directory missing before Hugo execution", "dir", rootDir, "error", err)
		return fmt.Errorf("staging directory not found: %w", err)
	} else {
		slog.Debug("Staging directory confirmed before Hugo", "dir", rootDir, "is_dir", stat.IsDir())
	}

	// Increase log verbosity for better diagnostics
	cmd := exec.Command("hugo", "--logLevel", "debug")
	cmd.Dir = rootDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	slog.Debug("BinaryRenderer invoking hugo", "dir", rootDir)

	err := cmd.Run()

	// Always log Hugo output when non-empty to diagnose issues
	outStr := stdout.String()
	errStr := stderr.String()
	if outStr != "" {
		slog.Debug("hugo stdout", "output", outStr)
	}
	if errStr != "" {
		slog.Warn("hugo stderr", "error_output", errStr)
	}

	if err != nil {
		// Include both stdout and stderr in error message for better diagnostics
		// Hugo may output errors to either stream
		output := errStr
		if output == "" {
			output = outStr
		} else if outStr != "" {
			output = outStr + "\n" + errStr
		}

		if output != "" {
			return fmt.Errorf("%w: %w: %s", herrors.ErrHugoExecutionFailed, err, output)
		}
		return fmt.Errorf("%w: %w", herrors.ErrHugoExecutionFailed, err)
	}

	// Check staging directory still exists after Hugo runs
	if stat, err := os.Stat(rootDir); err != nil {
		slog.Error("Staging directory MISSING after Hugo execution", "dir", rootDir, "error", err)
		return fmt.Errorf("staging directory disappeared during Hugo execution: %w", err)
	} else {
		slog.Debug("Staging directory confirmed after Hugo", "dir", rootDir, "is_dir", stat.IsDir())
	}

	return nil
}

// NoopRenderer performs no rendering; useful in tests or when only scaffolding is desired.
type NoopRenderer struct{}

func (n *NoopRenderer) Execute(rootDir string) error {
	slog.Debug("NoopRenderer skipping render", "dir", rootDir)
	return nil
}

// WithRenderer allows tests or callers to inject a custom renderer.
func (g *Generator) WithRenderer(r Renderer) *Generator {
	if r != nil {
		g.renderer = r
	}
	return g
}
