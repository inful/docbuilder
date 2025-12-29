package commands

import (
	"fmt"
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/lint"
)

// LintCmd implements the 'lint' command.
type LintCmd struct {
	Format string `short:"f" enum:"text,json" default:"text" help:"Output format (text or json)"`
	Quiet  bool   `short:"q" help:"Quiet mode: only show errors, suppress warnings"`
	Fix    bool   `help:"Automatically fix issues where possible (requires confirmation)"`
	DryRun bool   `help:"Show what would be fixed without applying changes (requires --fix)"`
	Yes    bool   `short:"y" help:"Auto-confirm fixes without prompting (for CI/CD)"`

	Path        *LintPathCmd     `cmd:"" default:"withargs" help:"Lint a path (file or directory)"`
	InstallHook *InstallHookCmd  `cmd:"" help:"Install pre-commit hook for automatic linting"`
}

// LintPathCmd handles linting a specific path.
type LintPathCmd struct {
	Path string `arg:"" optional:"" help:"Path to lint (file or directory). Defaults to intelligent detection (docs/, documentation/, or .)"`
}

// Run executes the lint path command.
func (lp *LintPathCmd) Run(parent *LintCmd, _ *Global, root *CLI) error {
	// Validate flags
	if parent.DryRun && !parent.Fix {
		return fmt.Errorf("--dry-run requires --fix flag")
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
		fixer := lint.NewFixer(linter, parent.DryRun, false) // force=false for safety
		fixResult, err := fixer.Fix(path)
		if err != nil {
			return fmt.Errorf("fixing failed: %w", err)
		}

		// Display what was fixed
		if parent.DryRun {
			fmt.Println("DRY RUN: No changes will be applied")
			fmt.Println()
		}

		if len(fixResult.FilesRenamed) > 0 {
			fmt.Println("Files to be renamed:")
			for _, op := range fixResult.FilesRenamed {
				if op.Success {
					fmt.Printf("  %s → %s\n", op.OldPath, op.NewPath)
				} else if op.Error != nil {
					fmt.Printf("  %s → %s (ERROR: %v)\n", op.OldPath, op.NewPath, op.Error)
				}
			}
			fmt.Println()
		}

		if len(fixResult.LinksUpdated) > 0 {
			fmt.Printf("Links updated in %d files:\n", countUniqueFiles(fixResult.LinksUpdated))
			fileLinks := groupLinksByFile(fixResult.LinksUpdated)
			for file, links := range fileLinks {
				fmt.Printf("  %s (%d links)\n", file, len(links))
			}
			fmt.Println()
		}

		// Display fix summary
		fmt.Println(fixResult.Summary())

		// Exit with error if fixes failed
		if fixResult.HasErrors() {
			os.Exit(2)
		}

		if !parent.DryRun {
			fmt.Printf("\n✨ Successfully fixed %d errors\n", fixResult.ErrorsFixed)
		}
		return nil
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
