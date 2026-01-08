package commands

import (
	"errors"
	"fmt"
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/lint"
)

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
	Path string `help:"Path to lint (file or directory). Defaults to intelligent detection (docs/, documentation/, or .)" arg:"" optional:""`
}

// Run executes the lint path command.
//

func (lp *LintPathCmd) Run(parent *LintCmd, _ *Global, root *CLI) error {
	// Validate flags
	if parent.DryRun && !parent.Fix {
		return errors.New("--dry-run requires --fix flag")
	}

	// Determine path to lint
	path := lp.Path
	wasAutoDetected := false

	if path == "" {
		var found bool
		path, found = lint.DetectDefaultPath()
		wasAutoDetected = found

		if root.Verbose {
			if found {
				fmt.Fprintf(os.Stderr, "Detected documentation directory: %s\n", path)
			} else {
				fmt.Fprintf(os.Stderr, "No documentation directory detected (checked: docs/, documentation/)\n")
				fmt.Fprintf(os.Stderr, "Falling back to current directory: %s\n", path)
			}
		}
	}

	// Validate path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	// Create linter configuration
	cfg := &lint.Config{
		Quiet:  parent.Quiet,
		Format: parent.Format,
		Fix:    parent.Fix,
		DryRun: parent.DryRun,
		Yes:    parent.Yes,
	}

	// Create linter
	linter := lint.NewLinter(cfg)

	// If fix mode is enabled, run fixer instead
	if parent.Fix {
		return runFixer(linter, path, parent.DryRun)
	}

	// Run linting
	result, err := linter.LintPath(path)
	if err != nil {
		return fmt.Errorf("linting failed: %w", err)
	}

	// Check if color output is supported
	useColor := isColorSupported()

	// Format and output results
	formatter := lint.NewFormatter(parent.Format, useColor)
	if err := formatter.Format(os.Stdout, result, path, wasAutoDetected); err != nil {
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
func runFixer(linter *lint.Linter, path string, dryRun bool) error {
	fixer := lint.NewFixer(linter, dryRun, false) // force=false for safety
	fixResult, err := fixer.Fix(path)
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

	// Display fix summary
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", fixResult.Summary())

	// Exit with error if fixes failed
	if fixResult.HasErrors() {
		os.Exit(2)
	}

	if !dryRun {
		switch {
		case fixResult.ErrorsFixed > 0 && fixResult.WarningsFixed > 0:
			_, _ = fmt.Fprintf(os.Stdout, "\n✨ Successfully fixed %d errors and %d warnings\n",
				fixResult.ErrorsFixed, fixResult.WarningsFixed)
		case fixResult.ErrorsFixed > 0:
			_, _ = fmt.Fprintf(os.Stdout, "\n✨ Successfully fixed %d errors\n", fixResult.ErrorsFixed)
		case fixResult.WarningsFixed > 0:
			_, _ = fmt.Fprintf(os.Stdout, "\n✨ Successfully fixed %d warnings\n", fixResult.WarningsFixed)
		}
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
