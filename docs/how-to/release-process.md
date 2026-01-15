---
uid: 591c7ad3-3af8-47f8-9d01-531da3233a5d
aliases:
  - /_uid/591c7ad3-3af8-47f8-9d01-531da3233a5d/
title: "Release Process"
date: 2026-01-01
categories:
  - development
  - ci-cd
tags:
  - releases
  - devcontainer
  - github-actions
fingerprint: d13e9f4dc79989f2727f0d7129449acddd0d7940e38c871e9dd89c02cb6d8c8e
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
2. **Docker** (`.github/workflows/ci.yml`): Builds and publishes multi-arch images

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

Currently in **0.1.x** series (pre-1.0 development).

## Related Documentation

- CI/CD Setup (see repository `.github/workflows/` directory)
- DevContainer Features: See [docbuilder-devcontainer-features](https://github.com/inful/docbuilder-devcontainer-features) repository
