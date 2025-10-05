package errors

import (
	"fmt"
	"log/slog"
	"os"

	dberrors "git.home.luguber.info/inful/docbuilder/internal/errors"
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

	// Check for ClassifiedError from foundation package
	if classified, ok := AsClassified(err); ok {
		return a.exitCodeFromClassified(classified)
	}

	// Check for DocBuilderError from errors package
	if dbe, ok := err.(*dberrors.DocBuilderError); ok {
		return a.exitCodeFromDocBuilder(dbe)
	}

	// Fallback for unclassified errors
	return 1
}

// exitCodeFromClassified maps ClassifiedError to exit codes.
func (a *CLIErrorAdapter) exitCodeFromClassified(err *ClassifiedError) int {
	switch err.Category() {
	case CategoryValidation:
		return 2 // Invalid usage
	case CategoryConfig:
		return 7 // Configuration error
	case CategoryAuth:
		return 5 // Permission/auth error
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

// exitCodeFromDocBuilder maps DocBuilderError to exit codes.
func (a *CLIErrorAdapter) exitCodeFromDocBuilder(err *dberrors.DocBuilderError) int {
	switch err.Category {
	case dberrors.CategoryValidation:
		return 2 // Invalid usage
	case dberrors.CategoryConfig:
		return 7 // Configuration error
	case dberrors.CategoryAuth:
		return 5 // Auth error
	case dberrors.CategoryNetwork, dberrors.CategoryGit, dberrors.CategoryForge:
		return 8 // External system error
	case dberrors.CategoryBuild, dberrors.CategoryHugo, dberrors.CategoryFileSystem:
		return 11 // Build error
	case dberrors.CategoryDaemon, dberrors.CategoryRuntime:
		return 12 // Runtime error
	case dberrors.CategoryInternal:
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

	// Check for ClassifiedError
	if classified, ok := AsClassified(err); ok {
		return a.formatClassified(classified)
	}

	// Check for DocBuilderError
	if dbe, ok := err.(*dberrors.DocBuilderError); ok {
		return a.formatDocBuilder(dbe)
	}

	// Fallback for unclassified errors
	return fmt.Sprintf("Error: %v", err)
}

// formatClassified formats a ClassifiedError for display.
func (a *CLIErrorAdapter) formatClassified(err *ClassifiedError) string {
	// For now, treat all foundation errors as internal since we don't have user-facing flags
	if a.verbose {
		return err.Error()
	}

	// Non-verbose mode for internal errors
	return fmt.Sprintf("Internal error occurred (use -v for details)")
}

// formatDocBuilder formats a DocBuilderError for display.
func (a *CLIErrorAdapter) formatDocBuilder(err *dberrors.DocBuilderError) string {
	if a.verbose {
		return err.Error()
	}

	// Clean format for common user-facing categories
	switch err.Category {
	case dberrors.CategoryConfig, dberrors.CategoryValidation, dberrors.CategoryAuth:
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

	// Log the error with appropriate level
	if a.shouldLog(err) {
		a.logError(err)
	}

	// Print user-facing message to stderr
	fmt.Fprintf(os.Stderr, "%s\n", message)
	os.Exit(exitCode)
}

// shouldLog determines if an error should be logged.
func (a *CLIErrorAdapter) shouldLog(err error) bool {
	// Always log in verbose mode
	if a.verbose {
		return true
	}

	// Check ClassifiedError
	if classified, ok := AsClassified(err); ok {
		// Log fatal severity
		return classified.Severity() == SeverityFatal
	}

	// Check DocBuilderError
	if dbe, ok := err.(*dberrors.DocBuilderError); ok {
		// Log internal/runtime errors or fatal severity
		return dbe.Category == dberrors.CategoryInternal ||
			dbe.Category == dberrors.CategoryRuntime ||
			dbe.Severity == dberrors.SeverityFatal
	}

	// Log unclassified errors
	return true
}

// logError logs an error with appropriate level and context.
func (a *CLIErrorAdapter) logError(err error) {
	// Check ClassifiedError
	if classified, ok := AsClassified(err); ok {
		level := a.slogLevelFromSeverity(classified.Severity())
		attrs := []slog.Attr{
			slog.String("category", string(classified.Category())),
		}
		if classified.CanRetry() {
			attrs = append(attrs, slog.Bool("retryable", true))
		}

		a.logger.LogAttrs(nil, level, classified.Message(), attrs...)
		return
	}

	// Check DocBuilderError
	if dbe, ok := err.(*dberrors.DocBuilderError); ok {
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

	// Fallback for unclassified errors
	a.logger.Error("Unclassified error", "error", err)
}

// slogLevelFromSeverity converts ClassifiedError severity to slog level.
func (a *CLIErrorAdapter) slogLevelFromSeverity(severity ErrorSeverity) slog.Level {
	switch severity {
	case SeverityInfo:
		return slog.LevelInfo
	case SeverityWarning:
		return slog.LevelWarn
	case SeverityError, SeverityFatal:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}

// slogLevelFromDocBuilderSeverity converts DocBuilderError severity to slog level.
func (a *CLIErrorAdapter) slogLevelFromDocBuilderSeverity(severity dberrors.ErrorSeverity) slog.Level {
	switch severity {
	case dberrors.SeverityInfo:
		return slog.LevelInfo
	case dberrors.SeverityWarning:
		return slog.LevelWarn
	case dberrors.SeverityFatal:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}
