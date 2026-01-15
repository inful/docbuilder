---
uid: d7da54f5-3864-4e53-b004-d8d3ae551f98
aliases:
  - /_uid/d7da54f5-3864-4e53-b004-d8d3ae551f98/
title: "CI/CD Setup"
date: 2025-12-15
categories:
  - ci-cd
tags:
  - continuous-integration
  - docker
fingerprint: e2145298145f73b9adff03b579235d6c836be5ec20d5eebf53eeaffae474b815
---

# CI/CD Setup

CI/CD pipeline configuration for DocBuilder using GitHub Actions and Forgejo Actions.

## GitHub Actions

### Multi-Architecture Docker Build

Workflow: `.github/workflows/docker-multiarch.yml`

**Triggers:**
- Push to `main` branch
- Git tags `v*.*.*`
- Manual dispatch

**Features:**
- Multi-arch builds (linux/amd64, linux/arm64)
- Automatic semver tagging (`v1.2.3`, `v1.2`, `v1`, `latest`)
- GitHub Container Registry (ghcr.io)
- Build caching

**Usage:**

```bash
# Pull latest
docker pull ghcr.io/inful/docbuilder:latest

# Pull specific version
docker pull ghcr.io/inful/docbuilder:v1.0.0

# Pull branch build
docker pull ghcr.io/inful/docbuilder:main
```

## Forgejo Actions

### Pipeline Jobs

Workflow: `.forgejo/workflows/ci.yml`

**Triggers:**
- Push to `main`, `master`, `develop`
- Pull requests
- Release publications

**Jobs:**
1. **Test** - Tests, linting, formatting
2. **Build** - Binary compilation, artifacts
3. **Docker Build** - Container images
4. **Integration Test** - Full pipeline test
5. **Security Scan** - Vulnerability scanning
6. **Deploy Staging** - Staging deployment
7. **Deploy Production** - Production deployment

### Maintenance Pipeline

Workflow: `.forgejo/workflows/maintenance.yml`

**Triggers:**
- Weekly schedule (Sundays 2 AM UTC)
- Manual dispatch

**Jobs:**
- Dependency updates
- Hugo version updates
- Artifact cleanup

## Configuration

### Secrets Required

```
REGISTRY_TOKEN - Container registry authentication
```

### Environments

- `staging` - Automatic deployment
- `production` - Manual approval required

## Local Development

### Docker Compose

```bash
# Build application
docker-compose build

# Run DocBuilder
docker-compose up docbuilder

# Development with Hugo server
docker-compose --profile dev up

# Monitoring stack
docker-compose --profile monitoring up
```

### Manual Testing

```bash
# Build binary
make build
./bin/docbuilder --version

# Run tests
make test
make test-coverage

# Test Docker image
docker build -t docbuilder:test .
docker run --rm docbuilder:test --version
```

## Pipeline Features

### Caching
- Go module cache
- Docker layer cache

### Multi-Platform Support
- Docker Buildx with QEMU (GitHub Actions)
- Native multi-arch (Forgejo Actions)
- Automatic platform detection

### Testing
- Unit tests with coverage
- Integration tests
- Docker functionality tests
- Security scanning (Trivy)

### Quality Gates
- Code formatting (gofmt)
- Linting (golangci-lint)
- Test coverage
- Security scanning

## Deployment

### Staging
- Automatic on `main` branch
- Latest successful build
- Environment: `staging`

### Production
- Manual via GitHub releases
- Requires approval
- Versioned tags
- Environment: `production`

## Customization

### Adding Tests
Add `*_test.go` files. Pipeline discovers them automatically.

### Build Targets
Modify Makefile targets:
- `make deps` - Dependencies
- `make fmt` - Formatting
- `make lint` - Linting
- `make build` - Binary build
- `make test-coverage` - Tests

### Environment Variables
- `GO_VERSION` - Go version
- `REGISTRY` - Container registry URL
- `IMAGE_NAME` - Docker image name

## Troubleshooting

### Build Failures
- Check Go version compatibility
- Verify dependency availability
- Review test output

### Docker Build Issues
- Verify Dockerfile syntax
- Check build context includes required files
- Confirm base images available

### Registry Authentication
- Ensure `REGISTRY_TOKEN` configured
- Verify token has push permissions
- Check registry URL

### Debug Commands

```bash
# Local test
make dev

# Docker build test
docker build --no-cache -t docbuilder:debug .

# Integration test
docker run --rm -v $(pwd)/test-config.yaml:/config.yaml \
  docbuilder:debug build -c /config.yaml
```

## Monitoring

Pipeline includes observability:

- **Metrics** - Prometheus collection
- **Logs** - Structured logging (slog)
- **Health Checks** - Docker health checks
- **Performance** - Build time tracking