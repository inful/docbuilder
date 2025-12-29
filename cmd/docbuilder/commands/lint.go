package commands

import (
	"fmt"
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/lint"
)

// LintCmd implements the 'lint' command.
type LintCmd struct {
	Path   string `arg:"" optional:"" help:"Path to lint (file or directory). Defaults to intelligent detection (docs/, documentation/, or .)"`
	Format string `short:"f" enum:"text,json" default:"text" help:"Output format (text or json)"`
	Quiet  bool   `short:"q" help:"Quiet mode: only show errors, suppress warnings"`
	Fix    bool   `help:"Automatically fix issues where possible (requires confirmation)"`
	DryRun bool   `help:"Show what would be fixed without applying changes (requires --fix)"`
	Yes    bool   `short:"y" help:"Auto-confirm fixes without prompting (for CI/CD)"`
}

// Run executes the lint command.
func (l *LintCmd) Run(_ *Global, root *CLI) error {
	// Validate flags
	if l.DryRun && !l.Fix {
		return fmt.Errorf("--dry-run requires --fix flag")
	}

	// Determine path to lint
	path := l.Path
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
		Quiet:  l.Quiet,
		Format: l.Format,
		Fix:    l.Fix,
		DryRun: l.DryRun,
		Yes:    l.Yes,
	}

	// Create linter
	linter := lint.NewLinter(cfg)

	// Run linting
	result, err := linter.LintPath(path)
	if err != nil {
		return fmt.Errorf("linting failed: %w", err)
	}

	// Check if color output is supported
	useColor := isColorSupported()

	// Format and output results
	formatter := lint.NewFormatter(l.Format, useColor)
	if err := formatter.Format(os.Stdout, result, path, wasAutoDetected); err != nil {
		return fmt.Errorf("formatting output: %w", err)
	}

	// Determine exit code based on results
	if result.HasErrors() {
		os.Exit(2) // Errors found (blocks build)
	} else if result.HasWarnings() && !l.Quiet {
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
