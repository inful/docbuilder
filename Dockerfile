# syntax=docker/dockerfile:1.6

# Optimized multi-stage Dockerfile for pre-built binaries
# Uses GoReleaser-built binaries to avoid rebuilding in Docker

############################
# Tool Download Stage
############################
FROM ubuntu:22.04 AS downloader
ARG TARGETOS=linux
ARG TARGETARCH
ARG HUGO_VERSION="0.152.2"
ENV DEBIAN_FRONTEND=noninteractive

# Install minimal dependencies
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl

# Download Hugo Extended with caching
RUN --mount=type=cache,target=/tmp/downloads \
    HUGO_ARCH="${TARGETARCH}" && \
    echo "Downloading Hugo ${HUGO_VERSION} for ${TARGETOS}-${HUGO_ARCH}" && \
    HUGO_FILE="hugo_extended_${HUGO_VERSION}_${TARGETOS}-${HUGO_ARCH}.tar.gz" && \
    if [ ! -f "/tmp/downloads/${HUGO_FILE}" ]; then \
      curl -fsSL "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/${HUGO_FILE}" \
      -o "/tmp/downloads/${HUGO_FILE}"; \
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
RUN BINARY_PATH="dist/docbuilder_${TARGETOS}_${TARGETARCH}*/docbuilder" && \
    if ls $BINARY_PATH 1> /dev/null 2>&1; then \
      echo "✅ Found pre-built binary: $BINARY_PATH" && \
      mkdir -p /out && \
      cp $BINARY_PATH /out/docbuilder && \
      chmod +x /out/docbuilder; \
    else \
      echo "❌ ERROR: No pre-built binary found at $BINARY_PATH" && \
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
FROM gcr.io/distroless/cc-debian12:nonroot AS runtime-minimal
ARG HUGO_VERSION="0.152.2"

USER nonroot:nonroot
WORKDIR /data

# Copy pre-built binaries
COPY --from=binary /out/docbuilder /usr/local/bin/docbuilder
COPY --from=downloader /usr/local/bin/hugo /usr/local/bin/hugo
COPY --from=downloader /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
ENV HUGO_ENV=production

ENTRYPOINT ["/usr/local/bin/docbuilder"]
CMD ["daemon", "--config", "/config/config.yaml"]
EXPOSE 1313 8080 9090
