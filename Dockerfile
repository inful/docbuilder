# syntax=docker/dockerfile:1.6

# Multi-stage Dockerfile that handles entire build pipeline:
# 1. Download tools (Hugo, golangci-lint) with architecture detection
# 2. Run tests, formatting checks, and linting
# 3. Build the final optimized runtime image


############################
# Build and Test Stage
############################
FROM ubuntu:22.04 AS builder
ARG TARGETOS=linux
ARG TARGETARCH
ARG HUGO_VERSION="0.152.2"
ARG GO_VERSION="1.24.0"
ARG VERSION="dev"
ENV DEBIAN_FRONTEND=noninteractive
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Install build dependencies in a single layer
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    git make ca-certificates curl wget xz-utils

# Download Go with caching
RUN --mount=type=cache,target=/tmp/downloads \
    GO_ARCH="${TARGETARCH}" && \
    if [ "${TARGETARCH}" = "amd64" ]; then GO_ARCH="amd64"; fi && \
    if [ "${TARGETARCH}" = "arm64" ]; then GO_ARCH="arm64"; fi && \
    echo "Downloading Go ${GO_VERSION} for ${TARGETOS}-${GO_ARCH}" && \
    GO_FILE="go${GO_VERSION}.${TARGETOS}-${GO_ARCH}.tar.gz" && \
    if [ ! -f "/tmp/downloads/${GO_FILE}" ]; then \
      wget -q "https://go.dev/dl/${GO_FILE}" -O "/tmp/downloads/${GO_FILE}"; \
    fi && \
    tar -C /usr/local -xzf "/tmp/downloads/${GO_FILE}"

ENV PATH="/usr/local/go/bin:$PATH"

# Download Hugo Extended with caching
RUN --mount=type=cache,target=/tmp/downloads \
    HUGO_ARCH="${TARGETARCH}" && \
    if [ "${TARGETARCH}" = "amd64" ]; then HUGO_ARCH="amd64"; fi && \
    if [ "${TARGETARCH}" = "arm64" ]; then HUGO_ARCH="arm64"; fi && \
    echo "Downloading Hugo ${HUGO_VERSION} for ${TARGETOS}-${HUGO_ARCH}" && \
    HUGO_FILE="hugo_extended_${HUGO_VERSION}_${TARGETOS}-${HUGO_ARCH}.tar.gz" && \
    if [ ! -f "/tmp/downloads/${HUGO_FILE}" ]; then \
      curl -fsSL "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/${HUGO_FILE}" \
      -o "/tmp/downloads/${HUGO_FILE}"; \
    fi && \
    tar -xz -C /tmp -f "/tmp/downloads/${HUGO_FILE}" hugo && \
    mv /tmp/hugo /usr/local/bin/hugo && \
    chmod +x /usr/local/bin/hugo
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION="dev"

# Verify Hugo is working
RUN hugo version && go version && git --version

WORKDIR /src

# Copy go module files for dependency caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy pre-built binary from GoReleaser (if available) or build from source
# This allows us to use pre-built binaries in CI for faster Docker builds
COPY dist/ /dist/ 2>/dev/null || true
COPY . .

# Use pre-built binary if available, otherwise build from source
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    BINARY_PATH="/dist/docbuilder_${TARGETOS}_${TARGETARCH}*/docbuilder" && \
    if ls $BINARY_PATH 1> /dev/null 2>&1; then \
      echo "=== Using pre-built binary ==="  && \
      mkdir -p /out && \
      cp $BINARY_PATH /out/docbuilder && \
      chmod +x /out/docbuilder && \
      echo "✅ Using pre-built binary from GoReleaser"; \
    else \
      echo "=== Building binary from source ===" && \
      CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/docbuilder ./cmd/docbuilder && \
      echo "✅ Binary built from source"; \
    fi

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
COPY --from=builder /usr/local/bin/hugo /usr/local/bin/hugo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
## Provide full Go toolchain for Hugo Modules without requiring system git
COPY --from=builder /usr/local/go /usr/local/go
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV HUGO_ENV=production
ENV GOROOT=/usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
## Prefer proxy-based module fetching to avoid system git dependency at runtime
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org
# HUGO module proxy (fallbacks to GOPROXY if unset)
ENV HUGO_MODULE_PROXY=https://proxy.golang.org,direct
ENTRYPOINT ["/usr/local/bin/docbuilder"]
CMD ["daemon", "--config", "/config/config.yaml"]
EXPOSE 1313 8080 9090
