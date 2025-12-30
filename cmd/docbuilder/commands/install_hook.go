package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// InstallHookCmd implements the 'lint install-hook' command.
type InstallHookCmd struct {
	Force bool `help:"Overwrite existing hook without backup"`
}

// Run executes the install-hook command.
//
//nolint:forbidigo // fmt is used for user-facing messages
func (cmd *InstallHookCmd) Run(_ *Global, _ *CLI) error {
	// Find git directory
	gitDir, err := findGitDir()
	if err != nil {
		return fmt.Errorf("not in a Git repository: %w", err)
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	hookPath := filepath.Join(hooksDir, "pre-commit")

	// Create hooks directory if it doesn't exist
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Backup existing hook unless --force
	if _, err := os.Stat(hookPath); err == nil && !cmd.Force {
		backupPath := fmt.Sprintf("%s.backup-%s", hookPath, time.Now().Format("20060102-150405"))
		fmt.Printf("📦 Backing up existing hook to: %s\n", backupPath)

		content, err := os.ReadFile(hookPath)
		if err != nil {
			return fmt.Errorf("failed to read existing hook: %w", err)
		}

		if err := os.WriteFile(backupPath, content, 0o755); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Create the pre-commit hook
	hookContent := `#!/usr/bin/env bash
# DocBuilder pre-commit hook - Lint staged documentation files
set -e

# Determine how to run docbuilder
DOCBUILDER_CMD=""
if command -v docbuilder &> /dev/null; then
    DOCBUILDER_CMD="docbuilder"
elif [ -f "go.mod" ] && grep -q "docbuilder" go.mod; then
    # In development mode - use go run
    DOCBUILDER_CMD="go run ./cmd/docbuilder"
else
    echo "⚠️  docbuilder not found in PATH"
    echo "   Install: go install git.home.luguber.info/inful/docbuilder/cmd/docbuilder@latest"
    echo "   Skipping documentation linting..."
    exit 0
fi

# Get list of staged markdown and image files
STAGED_DOCS=$(git diff --cached --name-only --diff-filter=ACM | grep -E '\.(md|markdown|png|jpg|jpeg|gif|svg)$' || true)

if [ -z "$STAGED_DOCS" ]; then
    # No documentation files staged, skip linting
    exit 0
fi

echo "🔍 Linting staged documentation files..."

# Create temporary directory for staged files
TEMP_DIR=$(mktemp -d)
trap "rm -rf ${TEMP_DIR}" EXIT

# Copy staged files to temporary directory preserving structure
for file in $STAGED_DOCS; do
    mkdir -p "${TEMP_DIR}/$(dirname "$file")"
    git show ":$file" > "${TEMP_DIR}/${file}"
done

# Run linter on temporary directory
if $DOCBUILDER_CMD lint "${TEMP_DIR}" --quiet; then
    echo "✅ Documentation linting passed"
    exit 0
else
    EXIT_CODE=$?
    echo ""
    echo "❌ Documentation linting failed"
    echo ""
    echo "To fix automatically:"
    echo "  $DOCBUILDER_CMD lint --fix"
    echo ""
    echo "To bypass this check (not recommended):"
    echo "  git commit --no-verify"
    echo ""
    exit $EXIT_CODE
fi
`

	if err := os.WriteFile(hookPath, []byte(hookContent), 0o755); err != nil {
		return fmt.Errorf("failed to write hook file: %w", err)
	}

	fmt.Println("✅ Pre-commit hook installed successfully")
	fmt.Println()
	fmt.Println("The hook will:")
	fmt.Println("  • Run automatically on 'git commit'")
	fmt.Println("  • Lint only staged documentation files")
	fmt.Println("  • Prevent commits with linting errors")
	fmt.Println()
	fmt.Println("To uninstall:")
	fmt.Printf("  rm %s\n", hookPath)
	fmt.Println()
	fmt.Println("To bypass the hook (not recommended):")
	fmt.Println("  git commit --no-verify")

	return nil
}

// findGitDir locates the .git directory.
func findGitDir() (string, error) {
	// Check if .git directory exists
	if info, err := os.Stat(".git"); err == nil && info.IsDir() {
		return ".git", nil
	}

	// Check if .git is a file (worktree/submodule)
	if info, err := os.Stat(".git"); err == nil && !info.IsDir() {
		content, err := os.ReadFile(".git")
		if err != nil {
			return "", err
		}

		// Parse gitdir from .git file
		line := string(content)
		if len(line) > 8 && line[:8] == "gitdir: " {
			return line[8:], nil
		}
	}

	// Try git command as fallback
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.New("not in a git repository")
	}

	return string(output), nil
}
