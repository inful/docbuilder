# Setting Up Multi-Architecture Builds with Docker Buildx

This guide explains how to use the GitHub Actions workflow for building and pushing multi-architecture Docker images of DocBuilder.

## Overview

The `buildx-multiarch.yml` workflow builds DocBuilder for multiple architectures (amd64 and arm64) using Docker Buildx. It produces two image variants:

- **Minimal** (`runtime-minimal`): Distroless base image with the smallest footprint
- **Full** (`runtime-full`): Debian-based image with Git and Go for Hugo Modules support

## Workflow Triggers

The workflow runs automatically on:

- **Version tags**: When you push a tag matching `v*.*.*` (e.g., `v1.0.0`)
- **Manual dispatch**: Can be triggered manually from GitHub Actions UI with optional parameters

## Required Secrets

To enable pushing images to your container registry, configure these repository secrets:

1. Go to your repository Settings → Secrets and variables → Actions
2. Add the following secrets:
   - `REGISTRY_USERNAME`: Your container registry username
   - `REGISTRY_PASSWORD`: Your container registry password or access token

**Without these secrets**, the workflow will still run and build images locally for testing, but won't push them to the registry.

## Manual Workflow Dispatch

You can manually trigger the workflow with custom parameters:

1. Go to Actions → "Build and Push DocBuilder Multi-Arch Images"
2. Click "Run workflow"
3. Configure optional parameters:
   - **Hugo version**: Specify a different Hugo version (default: 0.151.0)
   - **Push images**: Enable/disable pushing to registry (default: true)

## Workflow Configuration

The workflow is configured via environment variables at the top of `.github/workflows/buildx-multiarch.yml`:

```yaml
env:
  REGISTRY: git.home.luguber.info
  IMAGE_REPO: ${{ github.repository }}
  HUGO_VERSION: ${{ github.event.inputs.hugo_version || '0.151.0' }}
```

### Customizing the Registry

To use a different container registry:

1. Edit the `REGISTRY` environment variable in the workflow file
2. Update registry credentials in repository secrets

Supported registries include:

- Docker Hub (leave `REGISTRY` empty or use `docker.io`)
- GitHub Container Registry (`ghcr.io`)
- GitLab Container Registry (`registry.gitlab.com`)
- Self-hosted registries (use full domain)

## Image Tags

The workflow automatically tags images based on the trigger:

### On Version Tags (e.g., `v1.2.3`)

- Minimal: `registry/repo:1.2.3`, `registry/repo:latest`
- Full: `registry/repo:1.2.3-full`

### On Branch Commits

- Minimal: `registry/repo:{short-sha}`, `registry/repo:{branch-name}`
- Full: `registry/repo:{short-sha}-full`, `registry/repo:{branch-name}-full`

## Using the Built Images

### Docker Compose

```yaml
# docker-compose.yml
services:
  docbuilder:
    image: git.home.luguber.info/your-org/docbuilder:v1.2.3
    volumes:
      - ./config.yaml:/config/config.yaml
      - ./output:/data
    ports:
      - "8080:8080"
```

### Docker Run

```bash
# Minimal image
docker run -v $(pwd)/config.yaml:/config/config.yaml \
  -v $(pwd)/output:/data \
  git.home.luguber.info/your-org/docbuilder:v1.2.3

# Full image (with Git and Go)
docker run -v $(pwd)/config.yaml:/config/config.yaml \
  -v $(pwd)/output:/data \
  git.home.luguber.info/your-org/docbuilder:v1.2.3-full
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: docbuilder
spec:
  replicas: 1
  selector:
    matchLabels:
      app: docbuilder
  template:
    metadata:
      labels:
        app: docbuilder
    spec:
      containers:
      - name: docbuilder
        image: git.home.luguber.info/your-org/docbuilder:v1.2.3
        volumeMounts:
        - name: config
          mountPath: /config
        - name: data
          mountPath: /data
      volumes:
      - name: config
        configMap:
          name: docbuilder-config
      - name: data
        persistentVolumeClaim:
          claimName: docbuilder-data
```

## Choosing Between Minimal and Full Images

### Use Minimal Image When

- You don't need Hugo Modules
- You want the smallest possible image size
- Security is a top priority (distroless base)
- Your documentation doesn't use Go-based Hugo features

### Use Full Image When

- Your Hugo site uses Hugo Modules
- You need Git for repository operations
- You need Go toolchain for theme dependencies
- You're using themes that require `go mod` operations

## Troubleshooting

### Build Fails with "buildx not available"

Ensure your GitHub Actions runner has Docker Buildx installed. The workflow automatically sets up Buildx, but if you're running locally, install it:

```bash
# Install buildx plugin
docker buildx version

# Create and use a new builder
docker buildx create --name multiarch --use --bootstrap
```

### Images Not Pushing to Registry

1. Verify registry credentials are configured correctly
2. Check the workflow run logs for authentication errors
3. Ensure the registry URL is correct
4. Verify your account has push permissions to the repository

### "load" Only Supports Single Platform

When building without pushing (no credentials), Docker Buildx can only load one platform at a time. This is a Docker limitation.

**Automatic Behavior**:

- **With credentials**: Builds for `linux/amd64` and `linux/arm64`, pushes to registry
- **Without credentials**: Builds for native platform only (auto-detected), loads locally

To build for multiple platforms, you must push to a registry.

### Architecture-Specific Issues

If one architecture fails to build:

1. Check the Dockerfile for platform-specific issues
2. Verify tool downloads support both amd64 and arm64
3. Review the build logs for architecture-specific errors

## Local Testing

To test the multi-arch build locally before pushing:

```bash
# Create buildx builder
docker buildx create --name local-test --use

# Build both architectures (requires pushing to test registry)
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --target runtime-minimal \
  --tag localhost:5000/docbuilder:test \
  --push \
  .
```

Or use the Fish script for more control:

```bash
# Build both targets for both architectures
./scripts/buildx-multiarch.fish --platform linux/amd64,linux/arm64 --target both

# Build and push (requires registry credentials)
./scripts/buildx-multiarch.fish --push --registry git.home.luguber.info --repo your-org/docbuilder
```

## Related Documentation

- [CI/CD Setup](../ci-cd-setup.md) - Overall CI/CD architecture
- [Architecture Detection](../ci-architecture-detection.md) - How multi-arch builds work
- [Dockerfile Reference](../../Dockerfile) - Detailed build stage information
