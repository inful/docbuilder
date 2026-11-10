---
aliases:
  - /_uid/591c7ad3-3af8-47f8-9d01-531da3233a5d/
categories:
  - how-to
date: 2026-01-01T00:00:00Z
fingerprint: dbde72b6b71559f4a7ec71242f89313b1db9c1196865d91d6c2b8eb6dee5aac1
lastmod: "2026-11-10"
tags:
  - releases
  - devcontainer
  - github-actions
title: Release Process
uid: 591c7ad3-3af8-47f8-9d01-531da3233a5d
---
# Release Process

This document describes how to create a new release of DocBuilder.

## Quick Release (Automated)

The entire release process is automated. You only need to:

```bash
# 1. Commit your changes
git add .
git commit -m "feat: your changes"

# 2. Tag the version
git tag -a v0.X.Y -m "Release v0.X.Y - description"

# 3. Push (triggers all pipelines)
git push origin main --tags
```

That's it! The following happens automatically:

1. **GoReleaser** (`.github/workflows/release.yml`): Creates binaries for all platforms
2. **Docker** (`.github/workflows/release.yml`, `docker-multiarch` job): Builds and publishes multi-arch images to GitHub Container Registry

The published image tags are the version **without** a `v` prefix (`0.20.0`, `0.20`, `0`) plus `latest`; a `v`-prefixed alias (`v0.20.0`) is published as well.

> **Note**: DevContainer features have been moved to a separate repository ([docbuilder-devcontainer-features](https://github.com/inful/docbuilder-devcontainer-features)) for independent versioning and maintenance.

## Release Checklist

Before tagging a release:

- [ ] All tests pass: `go test ./...`
- [ ] Linter passes: `golangci-lint run`
- [ ] Changelog/release notes prepared
- [ ] Version number follows semver (MAJOR.MINOR.PATCH)

After pushing tag:

- [ ] Check GitHub Actions: workflows complete successfully
- [ ] Verify binaries published: https://github.com/inful/docbuilder/releases
- [ ] Verify Docker images: https://github.com/inful/docbuilder/pkgs/container/docbuilder

## Version Number Scheme

DocBuilder follows semantic versioning:

- **MAJOR** (0.X.x): Breaking API changes, major architecture changes
- **MINOR** (0.x.X): New features, theme additions, backwards-compatible changes
- **PATCH** (0.1.x): Bug fixes, documentation, minor improvements

Currently in the **0.x** series (pre-1.0 development).

## Related Documentation

- CI/CD Setup (see repository `.github/workflows/` directory)
- DevContainer Features: See [docbuilder-devcontainer-features](https://github.com/inful/docbuilder-devcontainer-features) repository
