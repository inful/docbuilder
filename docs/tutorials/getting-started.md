---
title: "Getting Started Tutorial"
date: 2025-12-15
categories:
  - tutorials
tags:
  - getting-started
  - quickstart
  - introduction
---

# Getting Started with DocBuilder

This tutorial walks you through producing a multi‑repository Hugo documentation site in minutes.

## Prerequisites

- Go toolchain (>=1.21) installed.
- Hugo installed (optional unless you want automatic static rendering).
- Git access tokens / SSH keys for the repositories you want to aggregate.

## 1. Install / Build

```bash
go build -o ./bin/docbuilder ./cmd/docbuilder
# Or run directly without building:
go run ./cmd/docbuilder <command>
```

## 2. Initialize Configuration

```bash
./bin/docbuilder init -c config.yaml
```

This creates a starter `config.yaml` you can customize.

## 3. Add Repositories

Edit `config.yaml` and list repositories:

```yaml
repositories:
  - url: https://git.example.com/org/service-a.git
    name: service-a
    branch: main
    paths: [docs]
    auth:
      type: token
      token: "${GIT_ACCESS_TOKEN}"
  - url: https://git.example.com/org/monorepo.git
    name: monorepo
    branch: main
    paths: [docs, documentation/guides]
    auth:
      type: token
      token: "${GIT_ACCESS_TOKEN}"

hugo:
  title: "My Documentation Site"
  description: "Aggregated documentation from multiple repositories"
  base_url: "https://docs.example.com"
  theme: "hextra"
  params:
    search:
      enable: true
      type: flexsearch

output:
  directory: ./site
  clean: true
```

## 4. Run a Build

```bash
./bin/docbuilder build -c config.yaml -v
```

On success you’ll have:

- `site/hugo.yaml` (generated Hugo config)
- `site/content/<repo>/...` (or `content/<forge>/<repo>/...` if multi‑forge)
- Optional `site/public/` if Hugo rendering enabled
- `site/build-report.json` with metrics (`doc_files_hash` fingerprint)

## 5. Serve (Optional)

If you enabled Hugo rendering (for example with `--render-mode always`), serve the generated site directly:

```bash
hugo server -s site
```

Or run Hugo manually afterwards:

```bash
(cd site && hugo)
```

## 6. Incremental Workflow

Enable workspace persistence to reuse existing clones:

```yaml
build:
  workspace_dir: ./site/_workspace  # Persist clones between builds
```

On subsequent runs, DocBuilder will:
- Reuse existing clones when possible
- Only fetch updates (git pull) instead of full clones
- Significantly speed up builds

## 7. Continuous Updates (Daemon Mode)

For live documentation updates, use daemon mode:

```bash
./bin/docbuilder daemon -c config.yaml -v
```

This will:
- Poll repositories for changes
- Automatically rebuild when changes detected
- Serve documentation with live reload

Or for local development without git polling:

```bash
./bin/docbuilder preview --docs-dir ./docs
```

## 8. Next Steps

- Customize landing pages with `templates/index/*.tmpl`.
- Pick a supported theme (`hextra`, `docsy`, or `relearn`).
- Integrate with CI: compare `doc_files_hash` between runs to skip downstream jobs.

## Additional Commands

### Linting Documentation

Validate documentation follows best practices:

```bash
./bin/docbuilder lint docs/

# Automatically fix issues
./bin/docbuilder lint docs/ --fix

# Preview fixes without applying
./bin/docbuilder lint docs/ --fix --dry-run
```

### Generate from Local Directory

Generate a Hugo site from local documentation (no git required):

```bash
./bin/docbuilder generate --docs-dir ./docs --output ./site
```

---
**You are ready to explore How‑To guides for specific tasks.**
