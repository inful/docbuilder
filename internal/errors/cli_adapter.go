package errors

import (
	"fmt"
	"log/slog"
	"os"
)

// CLIErrorAdapter handles error presentation and exit code determination for CLI applications.
type CLIErrorAdapter struct {
	verbose bool
	logger  *slog.Logger
}

// NewCLIErrorAdapter creates a new CLI error adapter.
func NewCLIErrorAdapter(verbose bool, logger *slog.Logger) *CLIErrorAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &CLIErrorAdapter{
		verbose: verbose,
		logger:  logger,
	}
}

// ExitCodeFor determines the appropriate exit code for an error.
func (a *CLIErrorAdapter) ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}

	if dbe, ok := err.(*DocBuilderError); ok {
		return a.exitCodeFromDocBuilder(dbe)
	}

	return 1
}

// exitCodeFromDocBuilder maps DocBuilderError to exit codes.
func (a *CLIErrorAdapter) exitCodeFromDocBuilder(err *DocBuilderError) int {
	switch err.Category {
	case CategoryValidation:
		return 2 // Invalid usage
	case CategoryConfig:
		return 7 // Configuration error
	case CategoryAuth:
		return 5 // Auth error
	case CategoryNetwork, CategoryGit, CategoryForge:
		return 8 // External system error
	case CategoryBuild, CategoryHugo, CategoryFileSystem:
		return 11 // Build error
	case CategoryDaemon, CategoryRuntime:
		return 12 // Runtime error
	case CategoryInternal:
		return 10 // Internal error
	default:
		return 1 // General error
	}
}

// FormatError formats an error for user-friendly display.
func (a *CLIErrorAdapter) FormatError(err error) string {
	if err == nil {
		return ""
	}

	if dbe, ok := err.(*DocBuilderError); ok {
		return a.formatDocBuilder(dbe)
	}

	return fmt.Sprintf("Error: %v", err)
}

// formatDocBuilder formats a DocBuilderError for display.
func (a *CLIErrorAdapter) formatDocBuilder(err *DocBuilderError) string {
	if a.verbose {
		return err.Error()
	}

	switch err.Category {
	case CategoryConfig, CategoryValidation, CategoryAuth:
		return err.Message
	default:
		return fmt.Sprintf("%s: %s", err.Category, err.Message)
	}
}

// HandleError processes an error and exits the program with appropriate code.
func (a *CLIErrorAdapter) HandleError(err error) {
	if err == nil {
		return
	}

	exitCode := a.ExitCodeFor(err)
	message := a.FormatError(err)

	if a.shouldLog(err) {
		a.logError(err)
	}

	fmt.Fprintf(os.Stderr, "%s\n", message)
	os.Exit(exitCode)
}

// shouldLog determines if an error should be logged.
func (a *CLIErrorAdapter) shouldLog(err error) bool {
	if a.verbose {
		return true
	}

	if dbe, ok := err.(*DocBuilderError); ok {
		return dbe.Category == CategoryInternal ||
			dbe.Category == CategoryRuntime ||
			dbe.Severity == SeverityFatal
	}

	return true
}

// logError logs an error with appropriate level and context.
func (a *CLIErrorAdapter) logError(err error) {
	if dbe, ok := err.(*DocBuilderError); ok {
		level := a.slogLevelFromDocBuilderSeverity(dbe.Severity)
		attrs := []slog.Attr{
			slog.String("category", string(dbe.Category)),
		}
		if dbe.Retryable {
			attrs = append(attrs, slog.Bool("retryable", true))
		}

		a.logger.LogAttrs(nil, level, dbe.Message, attrs...)
		return
	}

	a.logger.Error("Unclassified error", "error", err)
}

// slogLevelFromDocBuilderSeverity converts DocBuilderError severity to slog level.
func (a *CLIErrorAdapter) slogLevelFromDocBuilderSeverity(severity ErrorSeverity) slog.Level {
	switch severity {
	case SeverityInfo:
		return slog.LevelInfo
	case SeverityWarning:
		return slog.LevelWarn
	case SeverityFatal:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}
