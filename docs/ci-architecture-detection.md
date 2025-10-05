# CI Architecture Detection

This document explains how the CI workflow handles different runner architectures for binary downloads.

## Architecture Support

The CI workflow automatically detects the runner architecture and downloads the appropriate binaries for:

- **Hugo Extended**: Required for Hugo modules and SCSS processing
- **golangci-lint**: Code quality linting

### Supported Architectures

| `uname -m` output | Mapped to | Description |
|------------------|-----------|-------------|
| `x86_64` | `amd64` | Intel/AMD 64-bit |
| `aarch64` | `arm64` | ARM 64-bit |
| `arm64` | `arm64` | ARM 64-bit (macOS) |

### Detection Logic

```bash
case "$(uname -m)" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
esac
```

## Download URLs

### Hugo Extended

- **x86_64**: `https://github.com/gohugoio/hugo/releases/download/v{VERSION}/hugo_extended_{VERSION}_linux-amd64.tar.gz`
- **ARM64**: `https://github.com/gohugoio/hugo/releases/download/v{VERSION}/hugo_extended_{VERSION}_linux-arm64.tar.gz`

### golangci-lint

- **x86_64**: `https://github.com/golangci/golangci-lint/releases/download/v{VERSION}/golangci-lint-{VERSION}-linux-amd64.tar.gz`
- **ARM64**: `https://github.com/golangci/golangci-lint/releases/download/v{VERSION}/golangci-lint-{VERSION}-linux-arm64.tar.gz`

## Version Configuration

Binary versions are configured at the top of each installation step:

```yaml
- name: Install Hugo
  run: |
    HUGO_VERSION="0.151.0"
    # ... architecture detection and download
    
- name: Download golangci-lint
  run: |
    GOLANGCI_VERSION="1.59.1"
    # ... architecture detection and download
```

## Testing

Use the provided test scripts to verify architecture detection:

```bash
# Test architecture detection and URL accessibility
./scripts/test-arch-detection.sh

# Test complete CI setup (including Hugo/linter availability)
./scripts/test-ci-hugo.sh
```

## Benefits

1. **Multi-architecture support**: Works on both x86_64 and ARM64 runners
2. **Faster builds**: Downloads prebuilt binaries instead of compiling from source
3. **Reliable**: Fails fast with clear error messages for unsupported architectures
4. **Maintainable**: Architecture detection logic is centralized and consistent
