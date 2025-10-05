# syntax=docker/dockerfile:1.6

# Minimal multi-arch image for DocBuilder with Hugo
# Since go-git is pure Go (no external git needed), we only need:
# - DocBuilder binary (statically compiled)
# - Hugo binary (for site generation) 
# - CA certificates (for HTTPS Git repository access)
# - glibc (for Hugo extended binary)

############################
# Build DocBuilder
############################
# Note: Using Go 1.23 for Docker compatibility. 
# Local development may use Go 1.25 - temporarily edit go.mod during Docker builds.
FROM golang:1.23-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=arm64
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w" -o /out/docbuilder ./cmd/docbuilder

############################
# Get Hugo binary
############################
FROM alpine:3.18 AS hugo_downloader
ARG TARGETOS=linux
ARG TARGETARCH=arm64
SHELL ["/bin/ash", "-o", "pipefail", "-c"]
RUN apk add --no-cache curl
# Download Hugo extended for target platform
RUN HUGO_VERSION="0.151.0" && \
    HUGO_ARCH="${TARGETARCH}" && \
    if [ "${TARGETARCH}" = "amd64" ]; then HUGO_ARCH="amd64"; fi && \
    if [ "${TARGETARCH}" = "arm64" ]; then HUGO_ARCH="arm64"; fi && \
    curl -L "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_${TARGETOS}-${HUGO_ARCH}.tar.gz" \
    | tar -xz -C /tmp hugo && \
    mv /tmp/hugo /usr/local/bin/hugo && \
    chmod +x /usr/local/bin/hugo

############################
# Prepare runtime dependencies
############################
FROM alpine:3.18 AS runtime_deps
RUN apk add --no-cache ca-certificates gcompat libstdc++ libgcc && \
    adduser -D -u 10000 -g 10001 appuser && \
    mkdir -p /data /config && \
    chown 10000:10001 /data /config

############################
# Final runtime image
############################
FROM alpine:3.18
# Install minimal runtime dependencies including Go and git for Hugo modules
RUN apk add --no-cache ca-certificates gcompat libstdc++ libgcc go git && \
    adduser -D -u 10000 -g 10001 appuser && \
    mkdir -p /data /config && \
    chown 10000:10001 /data /config

# Copy binaries
COPY --from=builder /out/docbuilder /usr/local/bin/docbuilder
COPY --from=hugo_downloader /usr/local/bin/hugo /usr/local/bin/hugo

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
