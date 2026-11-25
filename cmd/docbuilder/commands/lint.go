package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/lint"
)

// lintSettingsFromConfig returns the settings of the linter that the
// configuration holds: the files that are not linted and the category
// pairs a document may hold together. A configuration that is not
// there is no error: the linter works without one.
func lintSettingsFromConfig(root *CLI) ([]string, map[string]map[string]bool, error) {
	if root == nil || root.Config == "" {
		return nil, nil, nil
	}
	if _, err := os.Stat(root.Config); err != nil {
		// The linter works without a configuration file.
		//nolint:nilerr // a configuration that is not there is no error
		return nil, nil, nil
	}
	_, cfg, err := config.LoadWithResult(root.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	return cfg.LintExcludes(), cfg.LintAllowedCategoryPairs(), nil
}

// LintCmd implements the 'lint' command.
type LintCmd struct {
	Format string `short:"f" default:"text" help:"Output format (text or json)" enum:"text,json"`
	Quiet  bool   `short:"q" help:"Quiet mode: only show errors, suppress warnings"`
	Fix    bool   `help:"Automatically fix issues where possible (requires confirmation)"`
	DryRun bool   `help:"Show what would be fixed without applying changes (requires --fix)"`
	Yes    bool   `short:"y" help:"Auto-confirm fixes without prompting (for CI/CD)"`

	Path        *LintPathCmd    `cmd:"" default:"withargs" help:"Lint a path (file or directory)"`
	InstallHook *InstallHookCmd `cmd:"" help:"Install pre-commit hook for automatic linting"`
}

// LintPathCmd handles linting a specific path.
type LintPathCmd struct {
	// Paths are the files and directories to lint. The pre-commit hook
	// of the documentation passes the staged files, and there are often
	// more than one of them.
	Paths []string `help:"Paths to lint (files or directories). Defaults to intelligent detection (docs/, documentation/, or .)" arg:"" optional:""`
}

// Run executes the lint path command.
//

func (lp *LintPathCmd) Run(parent *LintCmd, _ *Global, root *CLI) error {
	// Validate flags
	if parent.DryRun && !parent.Fix {
		return errors.New("--dry-run requires --fix flag")
	}

	// Determine the paths to lint
	paths := lp.Paths
	wasAutoDetected := false

	if len(paths) == 0 {
		var found bool
		path, found := lint.DetectDefaultPath()
		wasAutoDetected = found
		paths = []string{path}

		if root.Verbose {
			if found {
				fmt.Fprintf(os.Stderr, "Detected documentation directory: %s\n", path)
			} else {
				fmt.Fprintf(os.Stderr, "No documentation directory detected (checked: docs/, documentation/)\n")
				fmt.Fprintf(os.Stderr, "Falling back to current directory: %s\n", path)
			}
		}
	}

	// Validate that every path exists
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", path)
		}
	}

	// The files that are not linted come from the configuration: the
	// documented lint.excludes was ignored by the command line.
	excludes, pairs, err := lintSettingsFromConfig(root)
	if err != nil {
		return err
	}

	// Create linter configuration
	cfg := &lint.Config{
		Quiet:    parent.Quiet,
		Format:   parent.Format,
		Fix:      parent.Fix,
		DryRun:   parent.DryRun,
		Yes:      parent.Yes,
		Excludes: excludes,

		AllowedCategoryPairs: pairs,
	}

	// Create linter
	linter := lint.NewLinter(cfg)

	// If fix mode is enabled, run fixer instead
	if parent.Fix {
		return runFixer(linter, paths, parent.DryRun, parent.Yes)
	}

	// Run linting
	result, err := linter.LintPaths(paths)
	if err != nil {
		return fmt.Errorf("linting failed: %w", err)
	}

	// Check if color output is supported
	useColor := isColorSupported()

	// Format and output results
	formatter := lint.NewFormatter(parent.Format, useColor)
	if err := formatter.Format(os.Stdout, result, strings.Join(paths, ", "), wasAutoDetected); err != nil {
		return fmt.Errorf("formatting output: %w", err)
	}

	// Determine exit code based on results
	if result.HasErrors() {
		os.Exit(2) // Errors found (blocks build)
	} else if result.HasWarnings() && !parent.Quiet {
		os.Exit(1) // Warnings present
	}

	return nil
}

// isColorSupported checks if the terminal supports color output.
func isColorSupported() bool {
	// Check if stdout is a terminal
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	// Check NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check TERM environment variable
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}

	return true
}

// runFixer executes the fixer and displays results.
//
// The fixer asks before it changes the files: the files of the operator
// are changed, and the operator has to approve what is changed. The
// confirmation is skipped when the answer is already known (--yes).
func runFixer(linter *lint.Linter, paths []string, dryRun, yes bool) error {
	fixer := lint.NewFixer(linter, dryRun, false).WithAutoConfirm(yes) // force=false for safety

	var fixResult *lint.FixResult
	var err error
	if yes || dryRun {
		fixResult, err = fixer.FixPaths(paths)
	} else {
		fixResult, err = fixer.FixPathsWithConfirmation(paths)
	}
	if err != nil {
		return fmt.Errorf("fixing failed: %w", err)
	}

	// Display what was fixed
	if dryRun {
		_, _ = fmt.Fprintf(os.Stdout, "DRY RUN: No changes will be applied\n")
		_, _ = fmt.Fprintf(os.Stdout, "\n")
	}

	if len(fixResult.FilesRenamed) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "Files to be renamed:\n")
		for _, op := range fixResult.FilesRenamed {
			if op.Success {
				_, _ = fmt.Fprintf(os.Stdout, "  %s → %s\n", op.OldPath, op.NewPath)
			} else if op.Error != nil {
				_, _ = fmt.Fprintf(os.Stdout, "  %s → %s (ERROR: %v)\n", op.OldPath, op.NewPath, op.Error)
			}
		}
		_, _ = fmt.Fprintf(os.Stdout, "\n")
	}

	if len(fixResult.LinksUpdated) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "Links updated in %d files:\n", countUniqueFiles(fixResult.LinksUpdated))
		fileLinks := groupLinksByFile(fixResult.LinksUpdated)
		for file, links := range fileLinks {
			_, _ = fmt.Fprintf(os.Stdout, "  %s (%d links)\n", file, len(links))
		}
		_, _ = fmt.Fprintf(os.Stdout, "\n")
	}

	if len(fixResult.Fingerprints) > 0 {
		if dryRun {
			// Nothing was written: the report says what would happen.
			_, _ = fmt.Fprintf(os.Stdout, "Frontmatter fingerprints that would be updated in %d files:\n", len(fixResult.Fingerprints))
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "Frontmatter fingerprints updated in %d files:\n", len(fixResult.Fingerprints))
		}
		for _, fp := range fixResult.Fingerprints {
			if fp.Success {
				_, _ = fmt.Fprintf(os.Stdout, "  %s\n", fp.FilePath)
			} else if fp.Error != nil {
				_, _ = fmt.Fprintf(os.Stdout, "  %s (ERROR: %v)\n", fp.FilePath, fp.Error)
			}
		}
		_, _ = fmt.Fprintf(os.Stdout, "\n")
	}

	// Display fix summary
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", fixResult.Summary())

	// Exit with error if fixes failed
	if fixResult.HasErrors() {
		os.Exit(2)
	}

	if !dryRun {
		_, _ = fmt.Fprintf(os.Stdout, "\n✨ Successfully fixed %d errors\n", fixResult.ErrorsFixed)
	}
	return nil
}

// countUniqueFiles counts the number of unique files in link updates.
func countUniqueFiles(links []lint.LinkUpdate) int {
	files := make(map[string]bool)
	for _, link := range links {
		files[link.SourceFile] = true
	}
	return len(files)
}

// groupLinksByFile groups link updates by source file.
func groupLinksByFile(links []lint.LinkUpdate) map[string][]lint.LinkUpdate {
	result := make(map[string][]lint.LinkUpdate)
	for _, link := range links {
		result[link.SourceFile] = append(result[link.SourceFile], link)
	}
	return result
}
