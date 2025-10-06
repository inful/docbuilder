# syntax=docker/dockerfile:1.6

# Multi-stage Dockerfile that handles entire build pipeline:
# 1. Download tools (Hugo, golangci-lint) with architecture detection
# 2. Run tests, formatting checks, and linting
# 3. Build the final optimized runtime image

############################
# Download build tools
############################
FROM debian:12-slim AS tools_downloader
ARG TARGETOS=linux
ARG TARGETARCH=arm64
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Download Hugo Extended
RUN HUGO_VERSION="0.151.0" && \
    HUGO_ARCH="${TARGETARCH}" && \
    if [ "${TARGETARCH}" = "amd64" ]; then HUGO_ARCH="amd64"; fi && \
    if [ "${TARGETARCH}" = "arm64" ]; then HUGO_ARCH="arm64"; fi && \
    echo "Downloading Hugo ${HUGO_VERSION} for ${TARGETOS}-${HUGO_ARCH}" && \
    curl -L "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_${TARGETOS}-${HUGO_ARCH}.tar.gz" \
    | tar -xz -C /tmp hugo && \
    mv /tmp/hugo /usr/local/bin/hugo && \
    chmod +x /usr/local/bin/hugo

# Download golangci-lint
RUN GOLANGCI_VERSION="1.64.8" && \
    GOLANGCI_ARCH="${TARGETARCH}" && \
    if [ "${TARGETARCH}" = "amd64" ]; then GOLANGCI_ARCH="amd64"; fi && \
    if [ "${TARGETARCH}" = "arm64" ]; then GOLANGCI_ARCH="arm64"; fi && \
    echo "Downloading golangci-lint ${GOLANGCI_VERSION} for ${TARGETOS}-${GOLANGCI_ARCH}" && \
    curl -L "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_VERSION}/golangci-lint-${GOLANGCI_VERSION}-${TARGETOS}-${GOLANGCI_ARCH}.tar.gz" \
    | tar -xz -C /tmp && \
    mv "/tmp/golangci-lint-${GOLANGCI_VERSION}-${TARGETOS}-${GOLANGCI_ARCH}/golangci-lint" /usr/local/bin/golangci-lint && \
    chmod +x /usr/local/bin/golangci-lint

############################
# Build and Test Stage
############################
FROM golang:1.24-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=arm64

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

# Copy tools from downloader stage
COPY --from=tools_downloader /usr/local/bin/hugo /usr/local/bin/hugo
COPY --from=tools_downloader /usr/local/bin/golangci-lint /usr/local/bin/golangci-lint

# Verify tools are working
RUN hugo version && golangci-lint version

WORKDIR /src

# Copy go module files for dependency caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Run format check
RUN echo "=== Running format check ===" && \
    gofmt -l . | tee /tmp/fmt-issues && \
    if [ -s /tmp/fmt-issues ]; then \
        echo "Code is not formatted. Run 'go fmt ./...' to fix:" && \
        cat /tmp/fmt-issues && \
        exit 1; \
    fi && \
    echo "✅ Format check passed"

# Run linting
RUN echo "=== Running golangci-lint ===" && \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.cache/golangci-lint \
    golangci-lint run --timeout=10m && \
    echo "✅ Linting passed"

# Run tests with coverage
RUN echo "=== Running tests ===" && \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test -v -race -coverprofile=coverage.out ./... && \
    echo "✅ Tests passed" && \
    echo "=== Coverage Summary ===" && \
    go tool cover -func=coverage.out | tail -n 1

# Build the binary
RUN echo "=== Building binary ===" && \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w" -o /out/docbuilder ./cmd/docbuilder && \
    echo "✅ Binary built successfully"

# Test binary execution
RUN echo "=== Testing binary ===" && \
    /out/docbuilder --version && \
    /out/docbuilder --help >/dev/null && \
    echo "✅ Binary execution test passed"

############################
# Final runtime image
############################
FROM debian:12-slim
# Install minimal runtime dependencies including Go and git for Hugo modules
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates golang-go git \
    && rm -rf /var/lib/apt/lists/* \
    && adduser --disabled-password --uid 10000 --gid 10001 appuser \
    && mkdir -p /data /config \
    && chown 10000:10001 /data /config

# Copy binaries from builder stage
COPY --from=builder /out/docbuilder /usr/local/bin/docbuilder
COPY --from=tools_downloader /usr/local/bin/hugo /usr/local/bin/hugo

# Environment
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV HUGO_ENV=production
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org

# Run as non-root user
USER 10000:10001
WORKDIR /data

# Default command
ENTRYPOINT ["/usr/local/bin/docbuilder"]
CMD ["daemon", "--config", "/config/config.yaml"]

# Standard ports for docs and admin
EXPOSE 1313 8080 9090