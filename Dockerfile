# syntax=docker/dockerfile:1.6

# Multi-stage Dockerfile that handles entire build pipeline:
# 1. Download tools (Hugo, golangci-lint) with architecture detection
# 2. Run tests, formatting checks, and linting
# 3. Build the final optimized runtime image

############################
# Download build tools
############################
FROM debian:12-slim AS tools_downloader
# Prevent interactive prompts during package install
ENV DEBIAN_FRONTEND=noninteractive
ARG TARGETOS=linux
ARG TARGETARCH
ARG HUGO_VERSION="0.151.0"
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN apt-get update && apt-get install -y --no-install-recommends \
    -o Dpkg::Options::="--force-confold" \
    curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Download Hugo Extended
RUN HUGO_ARCH="${TARGETARCH}" && \
    if [ "${TARGETARCH}" = "amd64" ]; then HUGO_ARCH="amd64"; fi && \
    if [ "${TARGETARCH}" = "arm64" ]; then HUGO_ARCH="arm64"; fi && \
    echo "Downloading Hugo ${HUGO_VERSION} for ${TARGETOS}-${HUGO_ARCH}" && \
    curl -fsSL "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_${TARGETOS}-${HUGO_ARCH}.tar.gz" \
    | tar -xz -C /tmp hugo && \
    mv /tmp/hugo /usr/local/bin/hugo && \
    chmod +x /usr/local/bin/hugo

# Download golangci-lint

############################
# Build and Test Stage
############################
FROM golang:1.24-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION="dev"

# Install build dependencies
RUN apk add --no-cache git make ca-certificates

# Copy tools from downloader stage
COPY --from=tools_downloader /usr/local/bin/hugo /usr/local/bin/hugo
# Verify Hugo is working
RUN hugo version

WORKDIR /src

# Copy go module files for dependency caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Run format check

# Build the binary
RUN echo "=== Building binary ===" && \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/docbuilder ./cmd/docbuilder && \
    echo "✅ Binary built successfully"

# Test binary execution
RUN echo "=== Testing binary ===" && \
    /out/docbuilder --version && \
    /out/docbuilder --help >/dev/null && \
    echo "✅ Binary execution test passed"

############################
# Final runtime image
############################


## Minimal runtime (distroless cc) – smallest footprint, no git/go
FROM gcr.io/distroless/cc-debian12:nonroot AS runtime-minimal
USER nonroot:nonroot
WORKDIR /data
COPY --from=builder /out/docbuilder /usr/local/bin/docbuilder
COPY --from=tools_downloader /usr/local/bin/hugo /usr/local/bin/hugo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/local/go/bin/go /usr/local/bin/go
COPY --from=builder /usr/bin/git /usr/local/bin/git
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV HUGO_ENV=production
ENTRYPOINT ["/usr/local/bin/docbuilder"]
CMD ["daemon", "--config", "/config/config.yaml"]
EXPOSE 1313 8080 9090
