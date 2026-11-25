---
aliases:
  - /_uid/dad2de36-18a1-42e4-b066-7bd353246c9b/
categories:
  - reference
date: 2025-12-15T00:00:00Z
fingerprint: 5459750d98760809346cc768ab2c67abfdf39a9bd841b14ed8feda859defb9c9
lastmod: "2026-11-25"
tags:
  - cli
  - commands
  - usage
title: CLI Reference
uid: dad2de36-18a1-42e4-b066-7bd353246c9b
---

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

## `docbuilder-mcp` — MCP server for LLM integration

A separate binary (`cmd/mcp-server/`) speaks the
[Model Context Protocol](https://modelcontextprotocol.io/) over stdio so LLM
hosts (Claude Desktop, Cursor, VS Code, etc.) can drive doc-maintenance
tasks directly. It is built and released alongside `docbuilder` (same
tag, same archive). For full tool reference, configuration, and host
setup, see the **MCP Server (LLM Integration)** section in the root
README. Quick reference:

```bash
# Build
go build -o bin/docbuilder-mcp ./cmd/mcp-server

# Run (stdio JSON-RPC; meant to be spawned by an MCP host)
docbuilder-mcp --config config.yaml --docs-dir ./docs
```

| Flag | Description |
|------|-------------|
| `-c, --config PATH` | Docbuilder config file. Tools degrade gracefully if missing. |
| `--base-url URL` | Override the template discovery base URL. |
| `--docs-dir DIR` | Default docs root for `read_doc` / `create_doc` / `update_doc`. Default `./docs`. |
| `-v, --verbose` | Enable debug logging to stderr. |
| `--version` | Print version and exit. |

The template base URL resolves in this order: `--base-url` →
`DOCBUILDER_TEMPLATE_BASE_URL` → `hugo.base_url` from `--config`.

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
| `--edit-url-base URL` | Base URL of the edit links (the forge the pages are edited in) |
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
| `-c, --config FILE` | Configuration file that is written (default: `config.yaml`) |
| `-o, --output DIR` | Directory the configuration file is written to (default: the directory of the configuration file) |
| `--force` | Overwrite an existing configuration file |

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

### Exit Codes

| Exit Code | Meaning |
|-----------|---------|
| 0 | No issues found |
| 1 | Warnings found, no errors |
| 2 | Errors found (the build is blocked) |

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
| `-d, --data-dir DIR` | Data directory for daemon state (default: the `state_dir` of the configuration, `./daemon-data`) |

## Preview Command

Preview local documentation with live reload.

```bash
docbuilder preview [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `-d, --docs-dir DIR` | Documentation directory to watch (default: `./docs`) |
| `-o, --output DIR` | Output directory for the generated site (default: a temporary directory) |
| `--port PORT` | Docs server port (default: 1316) |
| `--livereload-port PORT` | LiveReload server port (default: the docs port + 3) |
| `--no-live-reload` | Disable the LiveReload stream and the script that opens it |
| `--title TITLE` | Site title (default: the name of the parent directory) |
| `--base-url URL` | Base URL of the site (default: `http://localhost:1316`) |
| `--vscode` | Enable the VS Code edit links (the files open in the editor) |

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
| 1 | Other error (the command failed) |
| 2 | The lint found errors (the `lint` command only) |
| 5 | Authentication or permission error |
| 7 | Configuration error |
| 8 | Network, git or forge error |
| 10 | Internal error |
| 11 | Build error (the site was not built) |
| 12 | Runtime or daemon error |
| 80 | Invalid usage (a flag or an argument is wrong) |
