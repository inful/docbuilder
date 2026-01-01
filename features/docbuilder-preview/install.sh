#!/bin/bash
set -e

VERSION="${VERSION:-latest}"
PREVIEW_PORT="${PREVIEWPORT:-1313}"

# Hugo version synced from repository (replaced during release)
HUGO_VERSION="0.152.2"

echo "Installing DocBuilder with Hugo ${HUGO_VERSION}..."

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) 
        echo "Error: Unsupported architecture: $ARCH"
        echo "Supported: x86_64 (amd64), aarch64/arm64"
        exit 1 
        ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
echo "Detected OS: ${OS}, Architecture: ${ARCH}"

# Install required tools if not present
if ! command -v wget &> /dev/null && ! command -v curl &> /dev/null; then
    echo "Installing curl..."
    apt-get update && apt-get install -y curl
fi

# Download DocBuilder
install_docbuilder() {
    if [ "$VERSION" = "latest" ]; then
        echo "Fetching latest DocBuilder version..."
        VERSION=$(curl -fsSL https://api.github.com/repos/inful/docbuilder/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")
        
        if [ -z "$VERSION" ]; then
            echo "Error: Failed to fetch latest release version"
            echo "GitHub API response might be rate-limited or unavailable"
            exit 1
        fi
        
        echo "Latest version: ${VERSION}"
    fi
    
    DOCBUILDER_FILE="docbuilder_${OS}_${ARCH}.tar.gz"
    DOCBUILDER_URL="https://github.com/inful/docbuilder/releases/download/${VERSION}/${DOCBUILDER_FILE}"
    
    echo "Downloading DocBuilder ${VERSION}..."
    echo "URL: $DOCBUILDER_URL"
    
    if command -v wget &> /dev/null; then
        if ! wget -q "$DOCBUILDER_URL" -O /tmp/docbuilder.tar.gz; then
            echo "Error: Failed to download DocBuilder from ${DOCBUILDER_URL}"
            echo "The release may not have binaries for ${OS}-${ARCH}"
            exit 1
        fi
    else
        if ! curl -fsSL "$DOCBUILDER_URL" -o /tmp/docbuilder.tar.gz; then
            echo "Error: Failed to download DocBuilder from ${DOCBUILDER_URL}"
            echo "The release may not have binaries for ${OS}-${ARCH}"
            exit 1
        fi
    fi
    
    # Extract docbuilder binary
    echo "Extracting DocBuilder..."
    tar -xzf /tmp/docbuilder.tar.gz -C /tmp docbuilder
    mv /tmp/docbuilder /usr/local/bin/docbuilder
    chmod +x /usr/local/bin/docbuilder
    rm /tmp/docbuilder.tar.gz
    echo "✓ DocBuilder installed successfully"
}

# Download Hugo Extended (must match Dockerfile version)
install_hugo() {
    HUGO_FILE="hugo_extended_${HUGO_VERSION}_${OS}-${ARCH}.tar.gz"
    HUGO_URL="https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/${HUGO_FILE}"
    
    echo "Downloading Hugo ${HUGO_VERSION}..."
    echo "URL: $HUGO_URL"
    
    if command -v wget &> /dev/null; then
        if ! wget -q "$HUGO_URL" -O /tmp/hugo.tar.gz; then
            echo "Error: Failed to download Hugo from ${HUGO_URL}"
            exit 1
        fi
    else
        if ! curl -fsSL "$HUGO_URL" -o /tmp/hugo.tar.gz; then
            echo "Error: Failed to download Hugo from ${HUGO_URL}"
            exit 1
        fi
    fi
    
    # Extract hugo binary
    echo "Extracting Hugo..."
    tar -xzf /tmp/hugo.tar.gz -C /tmp hugo
    mv /tmp/hugo /usr/local/bin/hugo
    chmod +x /usr/local/bin/hugo
    rm /tmp/hugo.tar.gz
    echo "✓ Hugo installed successfully"
}

# Install both
install_docbuilder
install_hugo

# Verify installations
echo ""
echo "==================================="
echo "Installation Summary"
echo "==================================="
docbuilder --version || echo "Warning: docbuilder --version failed"
hugo version || echo "Warning: hugo version failed"
echo ""
echo "✓ Installation complete!"
echo "Preview will start automatically on container creation"
echo "View logs with: tail -f /tmp/docbuilder-preview.log"
echo "==================================="
