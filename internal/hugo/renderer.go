package hugo

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	Execute(ctx context.Context, rootDir string) error
}

// BinaryRenderer invokes the `hugo` binary present on PATH.
type BinaryRenderer struct{}

// getEnvValue returns the value of the environment variable identified by key
// from the provided env slice, which contains entries in "KEY=VALUE" form.
// It returns the value and true if key is found, or an empty string and false otherwise.
func getEnvValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, prefix); ok {
			return v, true
		}
	}

	return "", false
}

// setEnvValue sets or replaces an environment variable in the provided env slice and returns the updated slice.
func setEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	newEnv := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			newEnv = append(newEnv, prefix+value)
			replaced = true
			continue
		}
		newEnv = append(newEnv, kv)
	}
	if !replaced {
		newEnv = append(newEnv, prefix+value)
	}

	return newEnv
}

// pathContainsDir reports whether dir appears as an element in a PATH-style
// string that is separated by os.PathListSeparator (colon on Unix, semicolon on Windows).
func pathContainsDir(pathValue, dir string) bool {
	for part := range strings.SplitSeq(pathValue, string(os.PathListSeparator)) {
		if part == dir {
			return true
		}
	}
	return false
}

// ensurePATHContainsDir ensures that dir is present in the PATH entry within
// the provided env slice, prepending it to PATH if it is not already included.
func ensurePATHContainsDir(env []string, dir string) []string {
	pathValue, ok := getEnvValue(env, "PATH")
	if !ok || pathValue == "" {
		return setEnvValue(env, "PATH", dir)
	}
	if pathContainsDir(pathValue, dir) {
		return env
	}

	return setEnvValue(env, "PATH", dir+string(os.PathListSeparator)+pathValue)
}

func (b *BinaryRenderer) Execute(ctx context.Context, rootDir string) error {
	if _, err := exec.LookPath("hugo"); err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrHugoBinaryNotFound, err)
	}
	// Relearn is pulled via Hugo Modules, which shells out to `go mod ...`.
	// If Go isn't available, fail fast with a clear message instead of Hugo's
	// often-opaque module download error.
	goPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrGoBinaryNotFound, err)
	}
	goDir := filepath.Dir(goPath)

	// Check staging directory exists before Hugo runs
	stat, statErr := os.Stat(rootDir)
	if statErr != nil {
		slog.Error("Staging directory missing before Hugo execution", "dir", rootDir, "error", statErr)
		return fmt.Errorf("staging directory not found: %w", statErr)
	}
	slog.Debug("Staging directory confirmed before Hugo", "dir", rootDir, "is_dir", stat.IsDir())

	// Increase log verbosity for better diagnostics
	cmd := exec.CommandContext(ctx, "hugo", "--logLevel", "debug")
	cmd.Dir = rootDir
	// Be explicit about environment inheritance. Also, ensure PATH contains the
	// resolved go binary directory so Hugo Modules can reliably execute `go`.
	env := ensurePATHContainsDir(os.Environ(), goDir)
	cmd.Env = env

	// Lightweight debug preflight: this should be safe (no secrets) and helps
	// diagnose environment discrepancies when Hugo Modules fails.
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		pre := exec.CommandContext(ctx, "go", "version")
		pre.Dir = rootDir
		pre.Env = env
		if out, preErr := pre.CombinedOutput(); preErr == nil {
			slog.Debug("go preflight ok", "go", goPath, "version", strings.TrimSpace(string(out)))
		} else {
			slog.Warn("go preflight failed", "go", goPath, "error", preErr.Error(), "output", strings.TrimSpace(string(out)))
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	slog.Debug("BinaryRenderer invoking hugo", "dir", rootDir)

	err = cmd.Run()

	// Always log Hugo output when non-empty to diagnose issues
	outStr := stdout.String()
	errStr := stderr.String()
	if outStr != "" {
		// Log each line separately to avoid escaped newlines
		for line := range strings.SplitSeq(strings.TrimSpace(outStr), "\n") {
			if line != "" {
				slog.Debug("hugo stdout", "line", line)
			}
		}
	}
	if errStr != "" {
		// Log each line separately to avoid escaped newlines
		for line := range strings.SplitSeq(strings.TrimSpace(errStr), "\n") {
			if line != "" {
				slog.Warn("hugo stderr", "line", line)
			}
		}
	}

	if err != nil {
		logHugoExecutionError(outStr, errStr)
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

// logHugoExecutionError logs Hugo execution error details line by line.
func logHugoExecutionError(outStr, errStr string) {
	// Log the combined error output line by line for readability
	output := errStr
	if output == "" {
		output = outStr
	} else if outStr != "" {
		output = outStr + "\n" + errStr
	}

	if output == "" {
		return
	}

	slog.Error("hugo execution failed - output details:")
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		// Parse Hugo ERROR lines to extract key information
		if strings.HasPrefix(strings.TrimSpace(line), "ERROR render of") {
			parseHugoRenderError(line)
		} else {
			slog.Error("  " + line)
		}
	}
}

// parseHugoRenderError extracts key information from Hugo's verbose ERROR lines
// and logs them in a more readable structured format.
func parseHugoRenderError(line string) {
	// Extract the file path (between quotes after "ERROR render of")
	// Example: ERROR render of "/path/to/file.md" failed: ...
	var filePath string
	if idx := strings.Index(line, `ERROR render of "`); idx >= 0 {
		start := idx + len(`ERROR render of "`)
		if end := strings.Index(line[start:], `"`); end >= 0 {
			filePath = line[start : start+end]
		}
	}

	// Extract the root cause (look for common error patterns)
	var rootCause string
	if idx := strings.LastIndex(line, "runtime error:"); idx >= 0 {
		rootCause = strings.TrimSpace(line[idx:])
	} else if idx := strings.LastIndex(line, "error calling"); idx >= 0 {
		// Get a reasonable snippet around the error
		snippet := line[idx:]
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		rootCause = snippet
	} else if _, after, ok := strings.Cut(line, "failed:"); ok {
		// Take a snippet after "failed:"
		snippet := strings.TrimSpace(after)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		rootCause = snippet
	}

	switch {
	case filePath != "" && rootCause != "":
		slog.Error("  Hugo render error",
			"file", filePath,
			"cause", rootCause)
	case filePath != "":
		slog.Error("  Hugo render error", "file", filePath)
	default:
		// Fallback: truncate very long lines
		if len(line) > 300 {
			slog.Error("  " + line[:300] + "...")
		} else {
			slog.Error("  " + line)
		}
	}
}

// NoopRenderer performs no rendering; useful in tests or when only scaffolding is desired.
type NoopRenderer struct{}

func (n *NoopRenderer) Execute(_ context.Context, rootDir string) error {
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
