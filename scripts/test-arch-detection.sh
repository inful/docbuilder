#!/bin/bash
set -euo pipefail

echo "=== Testing Architecture Detection Logic ==="

# Test the Hugo architecture detection
echo "Current architecture: $(uname -m)"

case "$(uname -m)" in
  x86_64) HUGO_ARCH="amd64" ;;
  aarch64|arm64) HUGO_ARCH="arm64" ;;
  *) echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
esac

echo "Hugo architecture mapping: linux-${HUGO_ARCH}"

# Test the golangci-lint architecture detection  
case "$(uname -m)" in
  x86_64) GOLANGCI_ARCH="amd64" ;;
  aarch64|arm64) GOLANGCI_ARCH="arm64" ;;
  *) echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
esac

echo "golangci-lint architecture mapping: linux-${GOLANGCI_ARCH}"

# Test Hugo URL construction
HUGO_VERSION="0.151.0"
HUGO_URL="https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-${HUGO_ARCH}.tar.gz"
echo "Hugo download URL: ${HUGO_URL}"

# Test golangci-lint URL construction
GOLANGCI_VERSION="1.59.1"
GOLANGCI_URL="https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_VERSION}/golangci-lint-${GOLANGCI_VERSION}-linux-${GOLANGCI_ARCH}.tar.gz"
echo "golangci-lint download URL: ${GOLANGCI_URL}"

# Test URL accessibility (just check if they exist, don't download)
echo "=== Testing URL accessibility ==="

if curl -sSfI "${HUGO_URL}" >/dev/null 2>&1; then
    echo "✅ Hugo URL is accessible"
else
    echo "❌ Hugo URL is NOT accessible: ${HUGO_URL}"
fi

if curl -sSfI "${GOLANGCI_URL}" >/dev/null 2>&1; then
    echo "✅ golangci-lint URL is accessible"
else
    echo "❌ golangci-lint URL is NOT accessible: ${GOLANGCI_URL}"
fi

echo "=== Architecture detection test completed ==="