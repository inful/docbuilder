#!/bin/bash
set -e

# Load versions from central file
if [ ! -f ".versions" ]; then
    echo "Error: .versions file not found"
    exit 1
fi

source .versions

# Get DocBuilder version from git tag or environment variable
DOCBUILDER_VERSION="${DOCBUILDER_VERSION:-$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')}"
if [ -z "$DOCBUILDER_VERSION" ]; then
    echo "Error: Could not determine DocBuilder version (no git tags or DOCBUILDER_VERSION env var)"
    exit 1
fi

echo "Syncing versions to devcontainer feature..."
echo "Hugo version: ${HUGO_VERSION}"
echo "DocBuilder version: ${DOCBUILDER_VERSION}"

FEATURE_INSTALL="features/docbuilder-preview/install.sh"
FEATURE_TEMPLATE="features/docbuilder-preview/install.sh.template"
FEATURE_JSON="features/docbuilder-preview/devcontainer-feature.json"

# Replace version placeholders in template
sed "s/{{HUGO_VERSION}}/${HUGO_VERSION}/g" "$FEATURE_TEMPLATE" > "$FEATURE_INSTALL"
chmod +x "$FEATURE_INSTALL"

echo "✓ Feature install.sh updated with Hugo ${HUGO_VERSION}"

# Update version in devcontainer-feature.json
if [ -f "$FEATURE_JSON" ]; then
    # Use jq to update version field, preserving JSON formatting
    if command -v jq >/dev/null 2>&1; then
        tmp_file=$(mktemp)
        jq --arg ver "$DOCBUILDER_VERSION" '.version = $ver' "$FEATURE_JSON" > "$tmp_file"
        mv "$tmp_file" "$FEATURE_JSON"
        echo "✓ Feature devcontainer-feature.json updated to version ${DOCBUILDER_VERSION}"
    else
        echo "⚠️  Warning: jq not found, skipping devcontainer-feature.json update"
        echo "   Install jq or manually update version to ${DOCBUILDER_VERSION}"
    fi
fi

# Also update README if it mentions the version
FEATURE_README="features/docbuilder-preview/README.md"
if [ -f "$FEATURE_README" ]; then
    # Create backup
    cp "$FEATURE_README" "${FEATURE_README}.bak"
    
    # Update version mentions in README
    sed -i "s/Hugo Extended [0-9]\+\.[0-9]\+\.[0-9]\+/Hugo Extended ${HUGO_VERSION}/g" "$FEATURE_README"
    
    echo "✓ Feature README.md updated"
    rm "${FEATURE_README}.bak"
fi
