# syntax=docker/dockerfile:1.6

# Optimized multi-stage Dockerfile for pre-built binaries
# Uses GoReleaser-built binaries to avoid rebuilding in Docker

############################
# Tool Download Stage
############################
FROM ubuntu:22.04 AS downloader
ARG TARGETOS=linux
ARG TARGETARCH
ARG HUGO_VERSION="0.170.1"
ARG GO_VERSION="1.26.11"
ENV DEBIAN_FRONTEND=noninteractive

# Install minimal dependencies
#
# Pinning apt versions against a rolling base image breaks the build
# the next time Debian ships a security update.
# hadolint ignore=DL3008
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wget

# Download Go with caching (needed for Hugo modules)
RUN --mount=type=cache,target=/tmp/downloads \
    GO_ARCH="${TARGETARCH}" && \
    echo "Downloading Go ${GO_VERSION} for ${TARGETOS}-${GO_ARCH}" && \
    GO_FILE="go${GO_VERSION}.${TARGETOS}-${GO_ARCH}.tar.gz" && \
    if [ ! -f "/tmp/downloads/${GO_FILE}" ]; then \
      wget -q "https://go.dev/dl/${GO_FILE}" -O "/tmp/downloads/${GO_FILE}"; \
    fi && \
    tar -C /usr/local -xzf "/tmp/downloads/${GO_FILE}" && \
    /usr/local/go/bin/go version

# Download Hugo with caching
#
# Hugo 0.170.0 ended the extended edition: one binary with every
# feature but LibSass, and Dart Sass is the only transpiler. The
# extended archives are now aliases, so the build takes the standard
# one.
RUN --mount=type=cache,target=/tmp/downloads \
    HUGO_ARCH="${TARGETARCH}" && \
    echo "Downloading Hugo ${HUGO_VERSION} for ${TARGETOS}-${HUGO_ARCH}" && \
    HUGO_FILE="hugo_${HUGO_VERSION}_${TARGETOS}-${HUGO_ARCH}.tar.gz" && \
    if [ ! -f "/tmp/downloads/${HUGO_FILE}" ]; then \
      wget -q "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/${HUGO_FILE}" \
      -O "/tmp/downloads/${HUGO_FILE}"; \
    fi && \
    tar -xz -C /tmp -f "/tmp/downloads/${HUGO_FILE}" hugo && \
    mv /tmp/hugo /usr/local/bin/hugo && \
    chmod +x /usr/local/bin/hugo && \
    hugo version

############################
# Binary Extraction Stage
############################
FROM ubuntu:22.04 AS binary
ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src
COPY dist/ dist/

# Extract pre-built binary from GoReleaser artifacts
#
# The path holds a * that stands for the GoReleaser build directory.
# find matches it, so the path can stay quoted.
RUN BINARY_PATTERN="dist/docbuilder_${TARGETOS}_${TARGETARCH}*/docbuilder" && \
    BINARY_PATH="$(find dist/ -type f -path "$BINARY_PATTERN" -print -quit 2>/dev/null)" && \
    if [ -n "$BINARY_PATH" ]; then \
      echo "✅ Found pre-built binary: $BINARY_PATH" && \
      mkdir -p /out && \
      cp "$BINARY_PATH" /out/docbuilder && \
      chmod +x /out/docbuilder; \
    else \
      echo "❌ ERROR: No pre-built binary found at $BINARY_PATTERN" && \
      echo "Available files:" && \
      find dist/ -type f && \
      exit 1; \
    fi

# Verify binary works
RUN /out/docbuilder --version && \
    /out/docbuilder --help >/dev/null

############################
# Final runtime image
############################
FROM debian:bookworm-slim AS runtime-minimal
ARG HUGO_VERSION="0.170.1"

WORKDIR /data

# Install minimal runtime dependencies (ca-certificates for HTTPS, bash for script execution)
#
# Pinning apt versions against a rolling base image breaks the build
# the next time Debian ships a security update.
# hadolint ignore=DL3008
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create nonroot user
RUN useradd -m -u 65532 nonroot

# Copy pre-built binaries
#
# --chown sets the ownership in the same layer as the content. A
# "chown -R" afterwards would write the ownership of every file into
# a layer of its own - 25 MB of the image for metadata alone.
COPY --from=binary --chown=nonroot:nonroot /out/docbuilder /usr/local/bin/docbuilder
COPY --from=downloader --chown=nonroot:nonroot /usr/local/bin/hugo /usr/local/bin/hugo
COPY --from=downloader /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy Go runtime (required for Hugo modules)
COPY --from=downloader --chown=nonroot:nonroot /usr/local/go /usr/local/go

# The work directory belongs to the user that runs the daemon. It is
# empty, so this layer holds one directory entry.
RUN chown nonroot:nonroot /data

# The numeric id, so the image runs on a host that does not know the
# name, and the home directory the name gave us - the Go module cache
# for Hugo Modules lives there.
USER 65532:65532

ENV HOME=/home/nonroot
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV HUGO_ENV=production
ENV GOROOT=/usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
# Prefer proxy-based module fetching to avoid system git dependency
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org
ENV HUGO_MODULE_PROXY=https://proxy.golang.org,direct

ENTRYPOINT ["/usr/local/bin/docbuilder"]
CMD ["daemon", "--config", "/config/config.yaml"]
# The daemon's default ports: 8080 docs, 8081 webhook, 8082 admin.
# 8083 (livereload) is only used when build.livereload is enabled.
EXPOSE 8080 8081 8082
