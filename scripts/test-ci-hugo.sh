#!/bin/bash
set -euo pipefail

echo "=== Testing CI Hugo Setup ==="

# Test architecture detection
echo "Current architecture: $(uname -m)"

# Simulate CI binary installation logic
install_hugo() {
    HUGO_VERSION="0.151.0"
    # Detect architecture
    case "$(uname -m)" in
        x86_64) HUGO_ARCH="amd64" ;;
        aarch64|arm64) HUGO_ARCH="arm64" ;;
        *) echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac
    echo "Hugo architecture mapping: linux-${HUGO_ARCH}"
    
    # Just verify URL accessibility, don't actually download
    HUGO_URL="https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_linux-${HUGO_ARCH}.tar.gz"
    echo "Testing Hugo URL: ${HUGO_URL}"
    if curl -sSfI "${HUGO_URL}" >/dev/null 2>&1; then
        echo "✅ Hugo binary URL is accessible"
    else
        echo "❌ Hugo binary URL is NOT accessible"
        return 1
    fi
}

install_golangci_lint() {
    GOLANGCI_VERSION="1.59.1"
    # Detect architecture
    case "$(uname -m)" in
        x86_64) GOLANGCI_ARCH="amd64" ;;
        aarch64|arm64) GOLANGCI_ARCH="arm64" ;;
        *) echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac
    echo "golangci-lint architecture mapping: linux-${GOLANGCI_ARCH}"
    
    # Just verify URL accessibility, don't actually download
    GOLANGCI_URL="https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_VERSION}/golangci-lint-${GOLANGCI_VERSION}-linux-${GOLANGCI_ARCH}.tar.gz"
    echo "Testing golangci-lint URL: ${GOLANGCI_URL}"
    if curl -sSfI "${GOLANGCI_URL}" >/dev/null 2>&1; then
        echo "✅ golangci-lint binary URL is accessible"
    else
        echo "❌ golangci-lint binary URL is NOT accessible"
        return 1
    fi
}

# Test binary URL accessibility
echo "=== Testing Binary URL Accessibility ==="
install_hugo
install_golangci_lint

# Test Hugo availability
if command -v hugo >/dev/null 2>&1; then
    echo "✅ Hugo is available: $(hugo version)"
else
    echo "⚠️  Hugo is NOT available locally (this is expected for URL testing)"
fi

# Test golangci-lint availability  
if command -v golangci-lint >/dev/null 2>&1; then
    echo "✅ golangci-lint is available: $(golangci-lint version)"
else
    echo "⚠️  golangci-lint is NOT available locally (this is expected for URL testing)"
fi

# Run the specific test that was failing
echo "=== Running NoopRenderer test ==="
go test ./internal/hugo -run TestNoopRenderer -v

echo "=== CI Hugo setup test completed successfully ==="