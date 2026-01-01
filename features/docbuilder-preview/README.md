# DocBuilder Preview Feature

Installs DocBuilder and Hugo, then automatically starts a documentation preview server.

## Usage

Add to your `.devcontainer/devcontainer.json`:

```json
{
  "features": {
    "ghcr.io/inful/docbuilder-preview:1": {
      "version": "latest",
      "previewPort": "1313"
    }
  },
  "forwardPorts": [1313],
  "portsAttributes": {
    "1313": {
      "label": "Documentation Preview",
      "onAutoForward": "openBrowser"
    }
  }
}
```

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `version` | string | `"latest"` | DocBuilder version to install |
| `previewPort` | string | `"1313"` | Port for preview server |

## What it does

1. Downloads DocBuilder binary from GitHub releases
2. Downloads Hugo Extended 0.152.2 (version matched to DocBuilder)
3. Starts preview server automatically on container creation
4. Logs to `/tmp/docbuilder-preview.log`

## View Logs

```bash
tail -f /tmp/docbuilder-preview.log
```

## Hugo Version

This feature installs Hugo Extended 0.152.2, which matches the version bundled in the DocBuilder container image. The Hugo version cannot be overridden to ensure compatibility.

## Commands

Once installed, you can use:

```bash
# View version
docbuilder --version

# Build documentation
docbuilder build

# Start preview server manually
docbuilder preview --port 1313

# View logs
tail -f /tmp/docbuilder-preview.log
```
