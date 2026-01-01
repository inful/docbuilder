# DevContainer Features

This directory contains [Development Container Features](https://containers.dev/implementors/features/) for DocBuilder.

## Available Features

### docbuilder-preview

Installs DocBuilder with bundled Hugo and automatically starts a documentation preview server.

**Usage:**
```json
{
  "features": {
    "ghcr.io/inful/docbuilder-preview:1": {}
  }
}
```

See [docbuilder-preview/README.md](./docbuilder-preview/README.md) for detailed documentation.

## Publishing

Features are automatically published to GitHub Container Registry (GHCR) as OCI artifacts when:

1. **On Release**: When a new GitHub release is published, features are tagged with the release version (e.g., `1.0.0`)
2. **On Main Branch**: When changes to `features/**` or `.versions` are pushed to `main`, a snapshot is published with the commit SHA

### Publishing Workflow

The `.github/workflows/publish-devcontainer-features.yml` workflow handles publishing:

1. **Syncs Versions**: Runs `scripts/sync-feature-versions.sh` to ensure Hugo version is up-to-date
2. **Installs devcontainer CLI**: Uses `@devcontainers/cli` to build and publish
3. **Authenticates**: Logs into GHCR using GitHub token
4. **Publishes**: Publishes each feature in `features/` directory to `ghcr.io/inful/devcontainer-features/`

### Version Management

Hugo version synchronization is managed through:

- **`.versions`**: Central source of truth for Hugo and Go versions
- **`scripts/sync-feature-versions.sh`**: Syncs versions from `.versions` to feature files
- **Makefile targets**:
  - `make sync-feature-versions`: Manual sync
  - `make update-hugo-version`: Interactive version update
  - `make check-versions`: Verify versions are in sync

## Development

### Testing Locally

To test a feature locally before publishing:

1. **Reference local feature**:
   ```json
   {
     "features": {
       "./features/docbuilder-preview": {}
     }
   }
   ```

2. **Rebuild devcontainer**: Use VS Code's "Dev Containers: Rebuild Container" command

### Adding a New Feature

1. Create a new directory under `features/` (e.g., `features/my-feature/`)
2. Add `devcontainer-feature.json` manifest
3. Add `install.sh` installation script
4. Add `README.md` documentation
5. Commit and push to `main` branch
6. Feature will be automatically published on next push

### Template Variables

The `install.sh.template` file supports version placeholders:

- `{{HUGO_VERSION}}`: Replaced with version from `.versions` file

The sync script (`sync-feature-versions.sh`) processes templates and generates final `install.sh` files.

## OCI Artifact Structure

Published features follow the [devcontainer feature distribution spec](https://containers.dev/implementors/features-distribution/):

```
ghcr.io/inful/devcontainer-features/<feature-name>:<version>
```

**Example:**
```
ghcr.io/inful/devcontainer-features/docbuilder-preview:1.0.0
ghcr.io/inful/devcontainer-features/docbuilder-preview:latest
```

## Versioning Strategy

- **Major versions** (e.g., `:1`, `:2`): Recommended for stability
- **Specific versions** (e.g., `:1.0.0`): Pin to exact version
- **Latest**: Always points to most recent release (`:latest`)
- **Snapshot builds**: Tagged with commit SHA (`:sha-abc1234`) for testing

## References

- [Dev Container Features Specification](https://containers.dev/implementors/features/)
- [Publishing Features](https://containers.dev/implementors/features-distribution/)
- [devcontainer CLI](https://github.com/devcontainers/cli)
