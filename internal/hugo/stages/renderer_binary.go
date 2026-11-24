package stages

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/pipeline"
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

// hugoBuildTimeout is how long a build of Hugo may take. A site with
// thousands of pages takes minutes; a build that takes longer is stuck.
const hugoBuildTimeout = 30 * time.Minute

// BinaryRenderer invokes the `hugo` binary present on PATH.
type BinaryRenderer struct{}

// hugoLogLevelArg returns the log level to ask Hugo for.
//
// Hugo writes a lot at debug level - tens of megabytes for a big site
// - and the daemon drops those lines at the usual log level anyway.
// So it is only asked for them when the daemon logs at debug level.
func hugoLogLevelArg(ctx context.Context) string {
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		return "debug"
	}
	return "info"
}

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

// hugoLogLevel maps a Hugo log line to the level DocBuilder logs it at.
//
// Hugo writes its progress output to stderr, so a build's INFO lines (the
// per-step timings included) arrive on the same stream as its warnings.
// Reporting all of them as warnings buries the real ones and makes a
// successful build look like a stream of failures.
func hugoLogLevel(line string) slog.Level {
	switch {
	case strings.HasPrefix(line, "ERROR"), strings.HasPrefix(line, "Error"):
		return slog.LevelError
	case strings.HasPrefix(line, "WARN"):
		return slog.LevelWarn
	case strings.HasPrefix(line, "DEBUG"), strings.HasPrefix(line, "TRACE"):
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

func (b *BinaryRenderer) Execute(ctx context.Context, rootDir string) error {
	if _, err := exec.LookPath("hugo"); err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrHugoBinaryNotFound, err)
	}
	// The theme is pulled via Hugo Modules, which shells out to `go mod ...`.
	// If Go isn't available, fail fast with a clear message instead of Hugo's
	// often-opaque module download error.
	goPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("%w: %w", herrors.ErrGoBinaryNotFound, err)
	}
	goDir := filepath.Dir(goPath)

	// The Hugo of the operator has to be new enough for the theme.
	if version := models.DetectHugoVersion(ctx); hugoVersionTooOld(version) {
		return fmt.Errorf("%w: the hugo binary is v%s, and the Docsy theme %s needs v%s or newer: the theme uses the field .Site.Language.Direction, which the older Hugo does not have",
			herrors.ErrHugoExecutionFailed, version, pipeline.DocsyVersion, pipeline.MinHugoVersion)
	}

	// Check staging directory exists before Hugo runs
	stat, statErr := os.Stat(rootDir)
	if statErr != nil {
		slog.Error("Staging directory missing before Hugo execution", "dir", rootDir, "error", statErr)
		return fmt.Errorf("staging directory not found: %w", statErr)
	}
	slog.Debug("Staging directory confirmed before Hugo", "dir", rootDir, "is_dir", stat.IsDir())

	// Hugo writes a lot at debug level - tens of megabytes for a big
	// site - and the daemon drops those lines at the usual log level
	// anyway. Ask for them only when the daemon logs at debug level.
	// The build has a deadline. A Hugo that hangs - a module download
	// that never answers - held the build of the daemon forever, and
	// every later build queued behind it until the queue was full.
	renderCtx, cancelRender := context.WithTimeout(ctx, hugoBuildTimeout)
	defer cancelRender()

	//nolint:gosec // the log level is one of two constants this file picks
	cmd := exec.CommandContext(renderCtx, "hugo", "--logLevel", hugoLogLevelArg(ctx))
	cmd.Dir = rootDir
	// The pipes are closed when the deadline passes: a child of Hugo -
	// a module download - keeps them open, and the build would wait for
	// it past the deadline.
	cmd.WaitDelay = 30 * time.Second
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

	// The output of Hugo goes to the log as it arrives. Hugo writes
	// tens of megabytes for a big site: holding it all in memory until
	// the process ends is a lot for nothing.
	stdout := &hugoOutputWriter{ctx: ctx, attr: "hugo stdout", debug: slog.Default().Enabled(ctx, slog.LevelDebug)}
	stderr := &hugoOutputWriter{ctx: ctx, attr: "hugo stderr", level: hugoLogLevel}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	slog.Debug("BinaryRenderer invoking hugo", "dir", rootDir)

	err = cmd.Run()

	// The lines that are kept are the ones of the failure message.
	outStr := stdout.tail()
	errStr := stderr.tail()

	if err != nil {
		logHugoExecutionError(outStr, errStr)

		// The message of the failure carries what Hugo said: the
		// operator sees "exit status 1" otherwise, and the reason is
		// only in the log of the daemon.
		if summary := hugoFailureSummary(outStr, errStr); summary != "" {
			return fmt.Errorf("%w: %w: %s", herrors.ErrHugoExecutionFailed, err, summary)
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

// maxHugoOutputLines is the number of lines of the output of Hugo that
// are kept for the message of a failure.
const maxHugoOutputLines = 200

// maxHugoOutputLine is the length of a line of the output of Hugo that
// is held while it is read. A longer line is cut: Hugo writes no such
// line, and the buffer must not grow without a bound.
const maxHugoOutputLine = 8 << 10

// hugoOutputWriter logs the output of Hugo line by line as it arrives
// and keeps the last lines for the message of a failure.
type hugoOutputWriter struct {
	// The context of the build: the lines of Hugo belong to it.
	//nolint:containedctx // the writer lives as long as the build
	ctx   context.Context
	attr  string
	level func(line string) slog.Level
	debug bool

	mu    sync.Mutex
	buf   []byte
	lines []string
}

// Write takes a piece of the output and logs the lines that are
// complete.
func (w *hugoOutputWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
		w.logLine(line)
	}
	if len(w.buf) > maxHugoOutputLine {
		w.buf = w.buf[len(w.buf)-maxHugoOutputLine:]
	}
	return len(p), nil
}

// logLine logs one line and keeps it for the message of a failure.
func (w *hugoOutputWriter) logLine(line string) {
	if line == "" {
		return
	}
	w.lines = append(w.lines, line)
	if len(w.lines) > maxHugoOutputLines {
		w.lines = w.lines[len(w.lines)-maxHugoOutputLines:]
	}
	if w.level != nil {
		slog.Log(w.ctx, w.level(line), w.attr, "line", line)
		return
	}
	// The stdout of Hugo is only read when somebody reads it.
	if w.debug {
		slog.Debug(w.attr, "line", line)
	}
}

// tail returns the lines that were kept, and the line that was not
// finished when the process ended.
func (w *hugoOutputWriter) tail() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if rest := strings.TrimSpace(string(w.buf)); rest != "" {
		w.lines = append(w.lines, rest)
	}
	w.buf = nil
	return strings.Join(w.lines, "\n")
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
	// Maintain the invariant expected by the pipeline/reporting:
	// if rendering is considered successful, a publish directory exists.
	if err := os.MkdirAll(filepath.Join(rootDir, "public"), 0o750); err != nil {
		return fmt.Errorf("create public dir: %w", err)
	}
	return nil
}

// hugoFailureSummary returns what Hugo said about the failure.
//
// The errors of Hugo come first: they say what is wrong with the
// content. The other lines are the last ones of the output, so that a
// failure without an error line is not a bare "exit status 1".
func hugoFailureSummary(outStr, errStr string) string {
	lines := strings.Split(strings.TrimSpace(errStr+outStr), "\n")

	var errors []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "ERROR") || strings.HasPrefix(trimmed, "Error") ||
			strings.HasPrefix(trimmed, "error") {
			errors = append(errors, trimmed)
		}
	}

	if len(errors) > 0 {
		if len(errors) > 5 {
			errors = errors[:5]
		}
		return strings.Join(errors, "; ")
	}

	// No error line: the last lines of the output are the best guess.
	var last []string
	for i := len(lines) - 1; i >= 0 && len(last) < 5; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			last = append([]string{trimmed}, last...)
		}
	}
	return strings.Join(last, "; ")
}

// hugoVersionTooOld reports whether the Hugo of the operator is too old
// for the pinned theme.
//
// The version is empty when the binary does not answer: the render
// fails with a clear message in that case.
func hugoVersionTooOld(version string) bool {
	if version == "" {
		return false
	}
	return compareVersions(version, pipeline.MinHugoVersion) < 0
}

// compareVersions compares two semantic versions ("0.158.0").
//
//nolint:unparam // the second version is the minimum of the theme, the first one is the version of the operator
func compareVersions(a, b string) int {
	partsA := versionParts(a)
	partsB := versionParts(b)
	for i := range partsA {
		switch {
		case partsA[i] < partsB[i]:
			return -1
		case partsA[i] > partsB[i]:
			return 1
		}
	}
	return 0
}

// versionParts splits a semantic version into its numbers.
func versionParts(version string) [3]int {
	var parts [3]int
	for i, part := range strings.SplitN(version, ".", 3) {
		if i > 2 {
			break
		}
		number, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			break
		}
		parts[i] = number
	}
	return parts
}
