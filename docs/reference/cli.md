---
title: "CLI Reference"
date: 2025-12-15
categories:
  - reference
tags:
  - cli
  - commands
  - usage
---

# CLI Reference

DocBuilder provides a unified command-line interface for building documentation sites from Git repositories.

## Commands

| Command | Description |
|---------|-------------|
| `build` | Build documentation site from repositories or local directory |
| `init` | Create example configuration file |
| `discover` | List documentation files found in repositories (debugging) |
| `lint` | Check documentation for errors and style issues |
| `daemon` | Run continuous documentation server with webhooks |
| `preview` | Preview local documentation with live reload |

## Global Flags

| Flag | Description |
|------|-------------|
| `-c, --config PATH` | Configuration file (default: `config.yaml`) |
| `-v, --verbose` | Enable verbose logging |
| `--version` | Show version and exit |

## Build Command

Build documentation from configured repositories.

### Syntax

```bash
docbuilder build [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `-o, --output DIR` | Output directory (default: `./site`) |
| `-i, --incremental` | Use incremental updates (skip unchanged repos) |
| `--render-mode MODE` | Override Hugo rendering: `auto`, `always`, `never` |
| `-d, --docs-dir DIR` | Local docs directory when no config provided (default: `./docs`) |
| `--title TEXT` | Site title for local mode (default: `"Documentation"`) |
| `--base-url URL` | Override Hugo base_url |
| `--relocatable` | Generate fully relocatable site (relative links) |
| `--keep-workspace` | Keep workspace directories for debugging |

### Examples

```bash
# Build from config file
docbuilder build -c config.yaml

# Build with verbose output
docbuilder build -v

# Build without running Hugo
docbuilder build --render-mode=never

# Build local docs without config
docbuilder build -d ./docs -o ./site

# Incremental build (skip unchanged repos)
docbuilder build -i
```

## Init Command

Create example configuration file.

```bash
docbuilder init [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `-c, --config FILE` | Output filename (default: `config.yaml`) |

## Discover Command

List documentation files found in repositories (for debugging).

```bash
docbuilder discover [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `-r, --repository NAME` | Discover specific repository only |

## Lint Command

Check documentation for errors and style issues.

```bash
docbuilder lint [PATH] [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `-f, --format FORMAT` | Output format: `text` or `json` (default: `text`) |
| `-q, --quiet` | Show only errors, suppress warnings |
| `--fix` | Automatically fix issues (requires confirmation) |
| `--dry-run` | Show what would be fixed without applying changes |
| `-y, --yes` | Auto-confirm fixes (for CI/CD) |

### Examples

```bash
# Lint current directory
docbuilder lint

# Lint specific path
docbuilder lint docs/

# Fix issues automatically (with confirmation)
docbuilder lint --fix

# CI mode: fix without confirmation
docbuilder lint --fix -y

# Show JSON output for CI integration
docbuilder lint -f json
```

## Daemon Command

Run continuous documentation server with webhook support.

```bash
docbuilder daemon [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `-d, --data-dir DIR` | Data directory for daemon state (default: `./daemon-data`) |

## Preview Command

Preview local documentation with live reload.

```bash
docbuilder preview [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `-d, --docs-dir DIR` | Documentation directory to watch (default: `./docs`) |
| `-o, --output DIR` | Hugo site directory (default: `./site`) |
| `-p, --port PORT` | Server port (default: 1313) |
| `--no-livereload` | Disable live reload |

## Build Report

Generated in output directory after `build` command:

- `build-report.json` - Machine-readable build summary
- `build-report.txt` - Human-readable one-line summary

### JSON Fields

| Field | Description |
|-------|-------------|
| `repositories` | Number of repositories with documentation |
| `files` | Total documentation files discovered |
| `outcome` | Build result: `success`, `warning`, `failed`, `canceled` |
| `cloned_repositories` | Successfully cloned repositories |
| `failed_repositories` | Repositories that failed to clone |
| `rendered_pages` | Markdown files copied to Hugo content directory |
| `static_rendered` | True if Hugo rendering succeeded |
| `doc_files_hash` | SHA-256 fingerprint of documentation file set |
| `issues[]` | Structured issues (code, stage, severity, message) |

## Exit Codes

| Exit Code | Meaning |
|-----------|---------|
| 0 | Success |
| 2 | Configuration or validation error |
| 10 | Authentication error |
| 11 | Git operation error |
| 20 | Network error (retryable) |
| 1 | Other error |
