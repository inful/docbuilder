---
uid: fb886a4a-1a7f-4d2d-9789-791247e160a0
date: 2025-12-29
categories:
  - how-to
tags:
  - linting
  - validation
  - git-hooks
  - developer-experience
fingerprint: dbc0778554c87a8ca8b10587bc8eb390157c610bde41662f584b2d89e26ec7fa
---

# Setup Documentation Linting

This guide explains how to set up documentation linting in your repository to catch issues before commit and during CI/CD.

## Overview

DocBuilder's linting system validates documentation files against Hugo and DocBuilder best practices, catching common issues like:

- Invalid filenames (spaces, uppercase, special characters)
- Malformed frontmatter
- Broken internal links
- Invalid double extensions (except whitelisted `.drawio.png`, `.drawio.svg`)

Linting can run:
- **Manually**: `docbuilder lint` command
- **Pre-commit**: Git hooks (lefthook or traditional)
- **CI/CD**: GitHub Actions, GitLab CI, etc.

## Prerequisites

- DocBuilder installed (`go install` or download binary)
- Git repository with documentation (usually in `docs/` or `documentation/`)

## Manual Linting

### Basic Usage

Run linting from your repository root:

```bash
# Auto-detects docs/ or documentation/ directory
docbuilder lint

# Lint specific directory
docbuilder lint ./docs

# Lint current directory explicitly
docbuilder lint .
```

### Understanding Output

```
Linting documentation in: ./docs
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✗ docs/API Guide.md
  ERROR: Invalid filename
  └─ Contains uppercase letters and spaces
  
  Current:  docs/API Guide.md
  Suggested: docs/api-guide.md

✓ docs/getting-started.md
✓ docs/images/architecture.drawio.png (whitelisted)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Results:
  3 files scanned
  1 error (blocks build)
  0 warnings
```

### Exit Codes

- `0`: No issues (clean)
- `1`: Warnings present (should fix)
- `2`: Errors found (blocks build)
- `3`: Execution error (filesystem access, etc.)

### Auto-Fix

Automatically fix common issues:

```bash
# Dry-run mode (preview changes)
docbuilder lint --fix --dry-run

# Interactive mode (prompts for confirmation)
docbuilder lint --fix

# Non-interactive mode (CI/automation)
docbuilder lint --fix --yes

# Force overwrite existing files
docbuilder lint --fix --force
```

Auto-fix will:
- Rename files to lowercase with hyphens
- Update all internal links to renamed files
- Preserve Git history with `git mv`
- Show detailed report of changes

### Output Formats

```bash
# Human-readable (default)
docbuilder lint

# JSON for CI/CD integration
docbuilder lint --format=json > lint-report.json

# Quiet mode (errors only)
docbuilder lint --quiet

# Verbose mode (detailed output)
docbuilder lint --verbose
```

## Git Hooks Integration

### Option 1: Lefthook (Recommended)

Lefthook is a fast, modern Git hooks manager with better performance and easier configuration.

#### Installation

**macOS:**
```bash
brew install lefthook
```

**Linux:**
```bash
# Using Go
go install github.com/evilmartians/lefthook@latest

# Or download binary from releases
curl -1sLf 'https://dl.cloudsmith.io/public/evilmartians/lefthook/setup.deb.sh' | sudo -E bash
sudo apt install lefthook
```

**Windows:**
```powershell
scoop install lefthook
```

#### Configuration

Create `lefthook.yml` in your repository root:

```yaml
# lefthook.yml
pre-commit:
  parallel: true
  commands:
    lint-docs:
      glob: "*.{md,markdown,png,jpg,jpeg,gif,svg}"
      run: docbuilder lint {staged_files} --quiet
      stage_fixed: true  # Auto-stage files fixed with --fix
```

#### Setup in Repository

```bash
# Install hooks (one-time per clone)
lefthook install

# Verify installation
lefthook run pre-commit --verbose
```

#### With Auto-Fix

To automatically fix issues on commit:

```yaml
# lefthook.yml
pre-commit:
  parallel: true
  commands:
    lint-docs:
      glob: "*.{md,markdown,png,jpg,jpeg,gif,svg}"
      run: docbuilder lint {staged_files} --fix --yes --quiet
      stage_fixed: true
```

#### Workflow

```bash
# Make changes to documentation
vim docs/API Guide.md

# Commit triggers automatic linting
git add docs/
git commit -m "Add API documentation"

# Lefthook runs automatically
Lefthook > pre-commit > lint-docs:
✓ All staged files pass linting

[main a1b2c3d] Add API documentation
```

### Option 2: Traditional Pre-Commit Hook

For repositories that prefer traditional Git hooks without additional tools.

#### Installation

```bash
# Automated installation
docbuilder lint install-hook

# Manual installation
curl -o .git/hooks/pre-commit https://raw.githubusercontent.com/your-org/docbuilder/main/scripts/pre-commit-hook.sh
chmod +x .git/hooks/pre-commit
```

#### Manual Hook Setup

Create `.git/hooks/pre-commit`:

```bash
#!/bin/sh
# DocBuilder documentation linting pre-commit hook

# Only lint staged markdown and asset files
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep -E '\.(md|markdown|png|jpg|jpeg|gif|svg)$')

if [ -n "$STAGED_FILES" ]; then
  echo "Linting staged documentation files..."
  docbuilder lint $STAGED_FILES --quiet
  
  LINT_EXIT=$?
  
  if [ $LINT_EXIT -eq 2 ]; then
    echo ""
    echo "❌ Lint errors found. Commit blocked."
    echo "Fix errors or run: docbuilder lint --fix"
    exit 1
  elif [ $LINT_EXIT -eq 1 ]; then
    echo ""
    echo "⚠️  Lint warnings present. Consider fixing before commit."
    echo "To auto-fix: docbuilder lint --fix"
    # Allow commit but show warning
    exit 0
  fi
fi

exit 0
```

Make executable:
```bash
chmod +x .git/hooks/pre-commit
```

### Comparison: Lefthook vs Traditional Hooks

| Feature | Lefthook | Traditional Hook |
|---------|----------|------------------|
| **Installation** | Single command: `lefthook install` | Manual script creation |
| **Configuration** | Checked into repo (`lefthook.yml`) | Not in repo (`.git/hooks/`) |
| **Performance** | Parallel execution | Sequential |
| **Portability** | Works across team (committed config) | Each clone needs manual setup |
| **Auto-staging** | Built-in (`stage_fixed: true`) | Manual implementation |
| **Maintenance** | Easy to update (edit `lefthook.yml`) | Edit script in each clone |
| **Dependencies** | Requires lefthook binary | Only Git |

**Recommendation**: Use lefthook for better developer experience and maintainability.

## Skipping Hooks (When Needed)

Sometimes you need to commit without running hooks:

```bash
# Skip all hooks
git commit --no-verify -m "Emergency hotfix"

# With lefthook, skip specific commands
LEFTHOOK_EXCLUDE=lint-docs git commit -m "Skip linting"
```

⚠️ **Warning**: Skipping linting may cause CI failures. Use sparingly.

## Team Adoption

### Gradual Rollout

**Week 1: Introduce linting**
```bash
# Team members run manually
docbuilder lint
docbuilder lint --fix --dry-run  # Show what would change
docbuilder lint --fix            # Apply fixes
```

**Week 2: Add lefthook configuration**
```yaml
# Start with warnings only (non-blocking)
pre-commit:
  commands:
    lint-docs:
      run: docbuilder lint {staged_files} --quiet || true  # Don't block commits
```

**Week 3: Enable enforcement**
```yaml
# Remove || true to block commits with errors
pre-commit:
  commands:
    lint-docs:
      run: docbuilder lint {staged_files} --quiet
```

### Team Setup Guide

Share this with your team:

```markdown
## Documentation Linting Setup (One-Time)

1. Install lefthook:
   - macOS: `brew install lefthook`
   - Linux: `go install github.com/evilmartians/lefthook@latest`
   - Windows: `scoop install lefthook`

2. Install hooks in your clone:
   ```bash
   cd your-repo
   lefthook install
   ```

3. Verify it works:
   ```bash
   lefthook run pre-commit --verbose
   ```

4. Clean up existing docs (one-time):
   ```bash
   docbuilder lint --fix --dry-run  # Preview changes
   docbuilder lint --fix            # Apply fixes
   git add -A
   git commit -m "docs: normalize filenames for linting"
   ```

That's it! Linting now runs automatically on every commit.
```

## Troubleshooting

### Issue: "docbuilder: command not found"

**Solution**: Add DocBuilder to PATH:

```bash
# Add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:$(go env GOPATH)/bin"

# Or install globally
sudo cp $(which docbuilder) /usr/local/bin/
```

### Issue: Hook doesn't run on commit

**Lefthook:**
```bash
# Reinstall hooks
lefthook install

# Check status
lefthook run pre-commit --verbose
```

**Traditional hook:**
```bash
# Verify hook exists and is executable
ls -la .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

### Issue: Too many warnings/errors in existing repo

**Solution**: Bulk fix existing issues:

```bash
# Preview all fixes
docbuilder lint --fix --dry-run

# Review changes and apply
docbuilder lint --fix

# Commit cleanup
git add -A
git commit -m "docs: normalize filenames and fix linting issues"
```

### Issue: Lefthook slows down commits

**Solution**: Optimize configuration:

```yaml
pre-commit:
  parallel: true  # Run commands in parallel
  commands:
    lint-docs:
      glob: "*.{md,markdown}"  # Only markdown files
      run: docbuilder lint {staged_files} --quiet
      skip:
        - merge      # Skip during merge commits
        - rebase     # Skip during rebase
```

### Issue: Need to commit despite linting errors

```bash
# Emergency bypass (use sparingly)
git commit --no-verify -m "Hotfix: critical issue"

# Or with lefthook
LEFTHOOK=0 git commit -m "Skip all hooks"
```

## Next Steps

- See [Lint Rules Reference](../reference/lint-rules.md) for complete rule documentation
- See [CI/CD Linting Integration](./ci-cd-linting.md) for automated validation
- See [Migration Guide](./migrate-to-linting.md) for cleaning up existing repositories

## Additional Resources

- [Lefthook Documentation](https://github.com/evilmartians/lefthook)
- [Git Hooks Tutorial](https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks)
- [Hugo URL Management](https://gohugo.io/content-management/urls/)
- [ADR-005: Documentation Linting](../adr/adr-005-documentation-linting.md)
