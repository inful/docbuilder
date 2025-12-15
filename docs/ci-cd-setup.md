---
title: "CI/CD Setup Guide"
date: 2025-12-15
categories:
  - ci-cd
tags:
  - continuous-integration
  - continuous-deployment
  - docker
  - gitea
---

# DocBuilder CI/CD

This document describes the CI/CD pipeline setup for DocBuilder using both GitHub Actions and Forgejo Actions.

## Pipeline Overview

DocBuilder supports CI/CD pipelines on multiple platforms:

### GitHub Actions

Multi-architecture Docker image builds are available through GitHub Actions workflow (`.github/workflows/docker-multiarch.yml`).

### Forgejo Actions

Complete CI/CD pipeline including testing, building, and deployment is available through Forgejo Actions.

## GitHub Actions Workflows

### 1. Multi-Arch Docker Build (`.github/workflows/docker-multiarch.yml`)

Builds and pushes multi-architecture Docker images to GitHub Container Registry (GHCR).

**Triggered on:**
- Push to `main` branch
- Git tags matching `v*.*.*` (e.g., v1.0.0)
- Manual workflow dispatch

**Features:**
- Multi-architecture builds (linux/amd64, linux/arm64)
- Automatic tagging strategy:
  - Semver tags: `v1.2.3`, `v1.2`, `v1`
  - Branch tags: `main`, `develop`
  - SHA tags: `main-abc1234`
  - `latest` tag for releases
- GitHub Container Registry (ghcr.io) integration
- Build caching with GitHub Actions cache
- Comprehensive build summaries

**Usage:**

The workflow runs automatically on tag pushes. To manually trigger:

1. Go to Actions → "Build and Push Multi-Arch Docker Images"
2. Click "Run workflow"
3. Optionally specify Hugo version
4. Choose whether to push images to registry

**Pulling images:**

```bash
# Latest release
docker pull ghcr.io/inful/docbuilder:latest

# Specific version
docker pull ghcr.io/inful/docbuilder:v1.0.0

# Branch build
docker pull ghcr.io/inful/docbuilder:main
```

**Registry Authentication:**

The workflow uses the built-in `GITHUB_TOKEN` for authentication. No additional secrets are required.

## Forgejo Actions Workflows

The CI/CD pipeline consists of several workflows:

### 1. Main CI/CD Pipeline (`.forgejo/workflows/ci.yml`)

Triggered on:
- Push to `main`, `master`, or `develop` branches
- Pull requests to these branches  
- Release publications

**Jobs:**

1. **Test** - Runs tests, linting, and formatting checks
2. **Build** - Compiles the binary and uploads artifacts
3. **Docker Build** - Builds and tests Docker images
4. **Integration Test** - Tests the complete DocBuilder + Hugo pipeline
5. **Security Scan** - Scans Docker images for vulnerabilities
6. **Deploy Staging** - Deploys to staging on main branch pushes
7. **Deploy Production** - Deploys to production on releases

### 2. Maintenance Pipeline (`.forgejo/workflows/maintenance.yml`)

Triggered:
- Weekly schedule (Sundays at 2 AM UTC)
- Manual dispatch

**Jobs:**
- Automated dependency updates
- Hugo version updates
- Artifact cleanup

## Setup Requirements

### 1. Forgejo Secrets

Configure these secrets in your Forgejo repository settings:

```
REGISTRY_TOKEN - Token for container registry authentication
```

### 2. Container Registry

The pipeline assumes you're using a container registry at `git.luguber.info`. Update the `REGISTRY` environment variable in the workflow if using a different registry.

### 3. Environments

Configure these environments in Forgejo:
- `staging` - For staging deployments
- `production` - For production deployments (with approval required)

## Local Development

### Using Docker Compose

1. **Build and test locally:**
   ```bash
   # Build the application
   docker-compose build
   
   # Run DocBuilder
   docker-compose up docbuilder
   ```

2. **Development with Hugo server:**
   ```bash
   # Run with development profile (includes Hugo server)
   docker-compose --profile dev up
   
   # Access at http://localhost:1313
   ```

3. **Monitoring stack:**
   ```bash
   # Run with monitoring profile
   docker-compose --profile monitoring up
   
   # Prometheus: http://localhost:9090
   # Grafana: http://localhost:3000 (admin/admin)
   ```

### Manual Testing

1. **Test binary build:**
   ```bash
   make build
   ./bin/docbuilder --version
   ```

2. **Run tests:**
   ```bash
   make test
   make test-coverage
   ```

3. **Test Docker image:**
   ```bash
   docker build -t docbuilder:test .
   docker run --rm docbuilder:test --version
   ```

## Pipeline Features

### Caching
- Go module cache for faster builds
- Docker layer cache for efficient image builds

### Multi-platform Support
- Docker images built for `linux/amd64` and `linux/arm64`
- GitHub Actions: Uses Docker Buildx with QEMU for cross-platform builds
- Forgejo Actions: Uses Docker Buildx with native multi-arch support
- Automatic platform detection and optimization

### Testing
- Unit tests with coverage reporting
- Integration tests with real Hugo builds
- Docker functionality tests
- Security vulnerability scanning

### Artifact Management
- Binary artifacts uploaded for each build
- Docker images tagged with branch, SHA, and semver
- Automatic cleanup of old artifacts

### Quality Gates
- Code formatting validation
- Linting with golangci-lint
- Test coverage reporting
- Security scanning with Trivy

## Deployment Strategy

### Staging
- Automatic deployment on `main` branch pushes
- Uses latest successful build
- Environment: `staging`

### Production  
- Manual deployment trigger via GitHub releases
- Requires environment approval
- Uses versioned tags
- Environment: `production`

## Customization

### Adding New Tests
Add test files following Go conventions (`*_test.go`). The pipeline will automatically discover and run them.

### Modifying Build Process
Update the `Makefile` targets. The CI pipeline uses these targets:
- `make deps` - Install dependencies
- `make fmt` - Format code
- `make lint` - Run linting
- `make build` - Build binary
- `make test-coverage` - Run tests with coverage

### Changing Docker Configuration
Modify the `Dockerfile` and `docker-compose.yml` as needed. The pipeline will use the updated configuration.

### Environment Variables
Key environment variables in the pipeline:
- `GO_VERSION` - Go version to use
- `REGISTRY` - Container registry URL
- `IMAGE_NAME` - Docker image name

## Troubleshooting

### Common Issues

1. **Build Failures**
   - Check Go version compatibility
   - Verify all dependencies are available
   - Review test output for specific failures

2. **Docker Build Issues**
   - Ensure Dockerfile syntax is correct
   - Check if all required files are included in build context
   - Verify base images are available

3. **Registry Authentication**
   - Ensure `REGISTRY_TOKEN` secret is configured
   - Verify token has push permissions to the registry
   - Check registry URL is correct

### Debug Commands

```bash
# Local test run
make dev

# Docker build test
docker build --no-cache -t docbuilder:debug .

# Integration test
docker run --rm -v $(pwd)/test-config.yaml:/config.yaml docbuilder:debug build -c /config.yaml
```

## Monitoring

The pipeline includes monitoring and observability:

- **Metrics**: Prometheus metrics collection
- **Logs**: Structured logging with slog
- **Health Checks**: Docker health checks
- **Performance**: Build time and resource usage tracking

Access monitoring dashboards:
- Prometheus: Available in monitoring profile
- Grafana: Pre-configured dashboards for DocBuilder metrics