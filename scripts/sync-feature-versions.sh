#!/bin/bash
set -e

# Load versions from central file
if [ ! -f ".versions" ]; then
    echo "Error: .versions file not found"
    exit 1
fi

source .versions

echo "Syncing versions to devcontainer feature..."
echo "Hugo version: ${HUGO_VERSION}"

FEATURE_INSTALL="features/docbuilder-preview/install.sh"
FEATURE_TEMPLATE="features/docbuilder-preview/install.sh.template"

# Replace version placeholders in template
sed "s/{{HUGO_VERSION}}/${HUGO_VERSION}/g" "$FEATURE_TEMPLATE" > "$FEATURE_INSTALL"
chmod +x "$FEATURE_INSTALL"

echo "✓ Feature install.sh updated with Hugo ${HUGO_VERSION}"

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
