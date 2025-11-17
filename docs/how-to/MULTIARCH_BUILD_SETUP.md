# Multi-Architecture Build Setup Summary

## Files Created/Modified

### GitHub Actions Workflow
- **File**: `.github/workflows/buildx-multiarch.yml`
- **Purpose**: Automated multi-architecture Docker image builds using Docker Buildx
- **Features**:
  - Builds for linux/amd64 and linux/arm64
  - Creates both minimal and full runtime variants
  - Automatic tagging based on git tags and branches
  - Optional push to container registry
  - Manual workflow dispatch with configurable parameters

### Documentation
- **File**: `docs/how-to/setup-multiarch-builds.md`
- **Purpose**: Complete guide for using the multi-arch build workflow
- **Covers**:
  - Workflow triggers and configuration
  - Required secrets setup
  - Image tagging strategy
  - Usage examples (Docker Compose, Docker Run, Kubernetes)
  - Troubleshooting common issues

## Key Improvements Over the Fish Script

The GitHub Actions workflow provides several advantages:

1. **Automation**: Triggers automatically on version tags
2. **Secrets Management**: Secure handling of registry credentials
3. **Build Matrix**: Parallel building of minimal and full variants
4. **Conditional Push**: Can build without pushing when secrets are missing
5. **Build Summaries**: Rich GitHub UI summaries with usage instructions
6. **Consistency**: Reproducible builds in CI environment

## Quick Start

### 1. Configure Secrets

In your GitHub repository:

```
Settings → Secrets and variables → Actions → New repository secret
```

Add:
- `REGISTRY_USERNAME`: Your registry username
- `REGISTRY_PASSWORD`: Your registry password/token

### 2. Push a Version Tag

```bash
git tag v1.0.0
git push origin v1.0.0
```

The workflow will automatically:
- Build both image variants
- Tag them appropriately
- Push to your configured registry

### 3. Use the Images

```bash
# Pull and run minimal image
docker pull git.home.luguber.info/your-org/docbuilder:v1.0.0
docker run -v $(pwd)/config.yaml:/config/config.yaml \
  git.home.luguber.info/your-org/docbuilder:v1.0.0

# Pull and run full image (with Git/Go)
docker pull git.home.luguber.info/your-org/docbuilder:v1.0.0-full
docker run -v $(pwd)/config.yaml:/config/config.yaml \
  git.home.luguber.info/your-org/docbuilder:v1.0.0-full
```

## Compatibility with Existing Script

The Fish script (`scripts/buildx-multiarch.fish`) remains available for:
- Local testing
- Manual builds
- CI environments without GitHub Actions
- Custom build configurations

Both the workflow and script use the same Dockerfile and build targets, ensuring consistency.

## Testing the Workflow

### Without Pushing (No Secrets Required)

1. Go to Actions → "Build and Push DocBuilder Multi-Arch Images"
2. Click "Run workflow"
3. Set "Push images to registry" to `false`
4. Click "Run workflow"

This will build the images locally for validation.

### With Pushing (Requires Secrets)

1. Configure registry secrets (see above)
2. Create and push a test tag:
   ```bash
   git tag v0.0.1-test
   git push origin v0.0.1-test
   ```
3. Monitor the workflow in the Actions tab
4. Verify images in your registry

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  GitHub Actions Workflow (buildx-multiarch.yml)        │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  Trigger: Version tags or manual dispatch               │
│     ↓                                                     │
│  Setup Docker Buildx                                     │
│     ↓                                                     │
│  Build Matrix (Parallel):                               │
│     ├─→ runtime-minimal (linux/amd64, linux/arm64)     │
│     └─→ runtime-full    (linux/amd64, linux/arm64)     │
│     ↓                                                     │
│  Tag: version, branch, latest                            │
│     ↓                                                     │
│  Push to Registry (optional)                             │
│     ↓                                                     │
│  Generate Build Summary                                  │
└─────────────────────────────────────────────────────────┘
```

## Image Naming Convention

```
{REGISTRY}/{ORG}/{REPO}:{VERSION}{-SUFFIX}

Examples:
  git.home.luguber.info/inful/docbuilder:v1.2.3         # minimal, version tag
  git.home.luguber.info/inful/docbuilder:v1.2.3-full    # full, version tag
  git.home.luguber.info/inful/docbuilder:main           # minimal, main branch
  git.home.luguber.info/inful/docbuilder:main-full      # full, main branch
  git.home.luguber.info/inful/docbuilder:a1b2c3d        # minimal, commit sha
  git.home.luguber.info/inful/docbuilder:latest         # minimal, latest release
```

## Workflow Parameters (Manual Dispatch)

When triggering manually, you can customize:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `hugo_version` | `0.151.0` | Hugo version to bundle in images |
| `push_images` | `true` | Whether to push to registry |

## Next Steps

1. **Test the workflow**: Try a manual dispatch first
2. **Configure secrets**: Add registry credentials
3. **Create a test tag**: Verify end-to-end functionality
4. **Update documentation**: Document your specific registry setup
5. **Monitor builds**: Check Actions tab for build status

## Support

- **Documentation**: See `docs/how-to/setup-multiarch-builds.md`
- **Issues**: Check GitHub Actions logs for errors
- **Local Testing**: Use `scripts/buildx-multiarch.fish` for debugging
