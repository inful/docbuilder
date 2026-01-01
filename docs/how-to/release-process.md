---
title: "Release Process"
date: 2026-01-01
categories:
  - development
  - ci-cd
tags:
  - releases
  - devcontainer
  - github-actions
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
3. **DevContainer Feature** (`.github/workflows/publish-devcontainer-features.yml`): 
   - Syncs versions using `scripts/sync-feature-versions.sh`
   - Publishes feature to `ghcr.io/inful/docbuilder-preview`

## Version Syncing

The `scripts/sync-feature-versions.sh` script automatically handles:

- **Hugo version**: Read from `.versions` file → embedded in `install.sh`
- **DocBuilder version**: Read from git tags → updated in `devcontainer-feature.json`

### How Version Detection Works

```bash
# Script reads version from git tags:
DOCBUILDER_VERSION=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')
# Example: v0.1.45 → 0.1.45

# Then updates devcontainer-feature.json using jq:
jq --arg ver "$DOCBUILDER_VERSION" '.version = $ver' devcontainer-feature.json
```

This ensures the DevContainer CLI publishes the correct version without manual JSON editing.

## Manual Version Override

If you need to publish a feature version that differs from the git tag:

```bash
DOCBUILDER_VERSION=0.2.0-beta ./scripts/sync-feature-versions.sh
```

This is useful for:
- Pre-release testing (`0.2.0-rc1`)
- Hotfixes on feature-only changes
- Development builds (`0.2.0-dev`)

## Troubleshooting

### Feature Not Publishing

**Symptom**: Workflow succeeds but users get old version

**Check**:
```bash
# 1. Verify version was updated in workflow logs
# Look for: "DocBuilder version: X.Y.Z"

# 2. Check if CLI skipped publishing
# Look for: "WARNING: Version X.Y.Z already exists, skipping"
```

**Cause**: The version in `devcontainer-feature.json` must be NEW. If it matches an existing published version, the CLI skips it.

**Fix**: The sync script now reads from git tags, so this should not happen. If it does:
- Verify git tag exists: `git tag -l "v0.1.*"`
- Check sync script ran: workflow logs show "✓ Feature devcontainer-feature.json updated"
- Ensure tag was pushed: `git ls-remote --tags origin`

### Version Mismatch

**Historical Issue (v0.1.44)**: Tag was created but `devcontainer-feature.json` wasn't updated, causing the CLI to attempt publishing version 0.1.43 (which already existed).

**Resolution**: Automated via sync script (commit cdb7ca0). The version field is now automatically derived from git tags during CI.

### Publishing to Wrong Registry Path

The DevContainer CLI may publish to different paths based on configuration:
- ✅ `ghcr.io/inful/docbuilder-preview` (correct)
- ❌ `ghcr.io/inful/devcontainer-features/docbuilder-preview` (wrong namespace)

The workflow explicitly sets `-r ghcr.io -n inful` to ensure correct registry and namespace.

## Release Checklist

Before tagging a release:

- [ ] All tests pass: `go test ./...`
- [ ] Linter passes: `golangci-lint run`
- [ ] Hugo version updated in `.versions` (if upgrading Hugo)
- [ ] Changelog/release notes prepared
- [ ] Version number follows semver (MAJOR.MINOR.PATCH)

After pushing tag:

- [ ] Check GitHub Actions: all 3 workflows complete successfully
- [ ] Verify binaries published: https://github.com/inful/docbuilder/releases
- [ ] Verify Docker images: https://github.com/inful/docbuilder/pkgs/container/docbuilder
- [ ] Verify DevContainer feature: https://github.com/inful/docbuilder/pkgs/container/docbuilder-preview
- [ ] Test feature installation in fresh devcontainer

## Version Number Scheme

DocBuilder follows semantic versioning:

- **MAJOR** (0.X.x): Breaking API changes, major architecture changes
- **MINOR** (0.x.X): New features, theme additions, backwards-compatible changes
- **PATCH** (0.1.x): Bug fixes, documentation, minor improvements

Currently in **0.1.x** series (pre-1.0 development).

## Related Documentation

- CI/CD Setup (see repository `.github/workflows/` directory)
- DevContainer Feature Configuration (see `features/docbuilder-preview/`)
- Version Sync Script (see `scripts/sync-feature-versions.sh`)
