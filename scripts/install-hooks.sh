#!/usr/bin/env bash
# Install pre-commit hook for DocBuilder documentation linting
set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Determine git hooks directory
if [ -d ".git" ]; then
    HOOKS_DIR=".git/hooks"
elif [ -f ".git" ]; then
    # Worktree or submodule - parse gitdir from .git file
    GIT_DIR=$(grep "gitdir:" .git | cut -d' ' -f2)
    HOOKS_DIR="${GIT_DIR}/hooks"
else
    echo -e "${RED}Error: Not in a Git repository${NC}"
    exit 1
fi

HOOK_PATH="${HOOKS_DIR}/pre-commit"
BACKUP_PATH="${HOOK_PATH}.backup-$(date +%Y%m%d-%H%M%S)"

# Create hooks directory if it doesn't exist
mkdir -p "${HOOKS_DIR}"

# Backup existing hook if present
if [ -f "${HOOK_PATH}" ]; then
    echo -e "${YELLOW}Existing pre-commit hook found, backing up to:${NC}"
    echo "  ${BACKUP_PATH}"
    cp "${HOOK_PATH}" "${BACKUP_PATH}"
fi

# Create the pre-commit hook
cat > "${HOOK_PATH}" << 'EOF'
#!/usr/bin/env bash
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
EOF

# Make hook executable
chmod +x "${HOOK_PATH}"

echo -e "${GREEN}✅ Pre-commit hook installed successfully${NC}"
echo ""
echo "The hook will:"
echo "  • Run automatically on 'git commit'"
echo "  • Lint only staged documentation files"
echo "  • Prevent commits with linting errors"
echo ""
echo "To uninstall:"
echo "  rm ${HOOK_PATH}"
if [ -f "${BACKUP_PATH}" ]; then
    echo "  mv ${BACKUP_PATH} ${HOOK_PATH}  # Restore previous hook"
fi
echo ""
echo "To bypass the hook (not recommended):"
echo "  git commit --no-verify"
