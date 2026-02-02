---
aliases:
  - /_uid/dad2de36-18a1-42e4-b066-7bd353246c9b/
categories:
  - reference
date: 2025-12-15T00:00:00Z
fingerprint: 2308fc0201713954f78a0896498f97aac5c0cf300b78f5c362f443f80c345e91
lastmod: "2026-01-22"
tags:
  - cli
  - commands
  - usage
title: CLI Reference
uid: dad2de36-18a1-42e4-b066-7bd353246c9b
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
| `template` | Create new documentation pages from templates |
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

Note: `docbuilder lint --fix` may update markdown file content beyond renames/link rewrites, including regenerating frontmatter `fingerprint` values and setting `lastmod` (UTC `YYYY-MM-DD`) when a fingerprint changes.

## Template Command

Create new documentation pages from templates hosted in your documentation site.

```bash
docbuilder template <subcommand> [flags]
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List available templates from a documentation site |
| `new` | Create a new document from a selected template |

### Template List

List available templates from a documentation site.

```bash
docbuilder template list [flags]
```

#### Flags

| Flag | Description |
|------|-------------|
| `--base-url URL` | Base URL for template discovery (required if not in config/env) |

#### Examples

```bash
# List templates from explicit URL
docbuilder template list --base-url https://docs.example.com

# Using environment variable
export DOCBUILDER_TEMPLATE_BASE_URL=https://docs.example.com
docbuilder template list

# Using config file
docbuilder template list -c config.yaml  # Uses config.hugo.base_url
```

### Template New

Create a new document from a template.

```bash
docbuilder template new [flags]
```

#### Flags

| Flag | Description |
|------|-------------|
| `--base-url URL` | Base URL for template discovery |
| `--set KEY=VALUE` | Override template field (repeatable) |
| `--defaults` | Use template defaults and skip prompts |
| `-y, --yes` | Auto-confirm file creation without prompting |

#### Base URL Resolution

Resolved in order:
1. `--base-url` flag
2. `DOCBUILDER_TEMPLATE_BASE_URL` environment variable
3. `hugo.base_url` from config (if `-c/--config` provided)
4. Error if none found

#### Examples

```bash
# Interactive mode (prompts for all fields)
docbuilder template new --base-url https://docs.example.com

# With pre-filled values
docbuilder template new --base-url https://docs.example.com \
  --set Title="New Feature" \
  --set Slug="new-feature"

# Use defaults only
docbuilder template new --base-url https://docs.example.com \
  --set Title="Quick Start" \
  --defaults

# CI/CD mode (no prompts, auto-confirm)
docbuilder template new --base-url https://docs.example.com \
  --set Title="Release Notes" \
  --set Slug="release-1.0" \
  --yes
```

#### Generated File Processing

After creating a file, DocBuilder automatically:
1. Writes the file to `docs/` (or path specified by template)
2. Runs `docbuilder lint --fix` to ensure proper frontmatter

See [Using Templates](../how-to/use-templates.md) for detailed usage guide.

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
