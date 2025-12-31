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
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')

# Install required tools if not present
if ! command -v wget &> /dev/null && ! command -v curl &> /dev/null; then
    echo "Installing wget..."
    apt-get update && apt-get install -y wget
fi

# Download DocBuilder
install_docbuilder() {
    if [ "$VERSION" = "latest" ]; then
        echo "Fetching latest DocBuilder version..."
        VERSION=$(curl -s https://api.github.com/repos/YOUR_ORG/docbuilder/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    fi
    
    DOCBUILDER_URL="https://github.com/YOUR_ORG/docbuilder/releases/download/${VERSION}/docbuilder-${OS}-${ARCH}"
    
    echo "Downloading DocBuilder ${VERSION} from: $DOCBUILDER_URL"
    
    if command -v wget &> /dev/null; then
        wget -q "$DOCBUILDER_URL" -O /usr/local/bin/docbuilder
    else
        curl -fsSL "$DOCBUILDER_URL" -o /usr/local/bin/docbuilder
    fi
    
    chmod +x /usr/local/bin/docbuilder
}

# Download Hugo Extended (must match Dockerfile version)
install_hugo() {
    HUGO_FILE="hugo_extended_${HUGO_VERSION}_${OS}-${ARCH}.tar.gz"
    HUGO_URL="https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/${HUGO_FILE}"
    
    echo "Downloading Hugo ${HUGO_VERSION} from: $HUGO_URL"
    
    if command -v wget &> /dev/null; then
        wget -q "$HUGO_URL" -O /tmp/hugo.tar.gz
    else
        curl -fsSL "$HUGO_URL" -o /tmp/hugo.tar.gz
    fi
    
    # Extract hugo binary
    tar -xzf /tmp/hugo.tar.gz -C /tmp hugo
    mv /tmp/hugo /usr/local/bin/hugo
    chmod +x /usr/local/bin/hugo
    rm /tmp/hugo.tar.gz
}

# Install both
install_docbuilder
install_hugo

# Verify installations
echo ""
echo "✓ DocBuilder installed successfully!"
docbuilder --version
echo ""
echo "✓ Hugo ${HUGO_VERSION} installed successfully!"
hugo version
echo ""
echo "Preview will start automatically on container creation"
echo "View logs with: tail -f /tmp/docbuilder-preview.log"
