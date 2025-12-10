# Getting Started with DocBuilder

This tutorial walks you through producing a multi‑repository Hugo documentation site in minutes.

## Prerequisites

- Go toolchain (>=1.21) installed.
- Hugo installed (optional unless you want automatic static rendering).
- Git access tokens / SSH keys for the repositories you want to aggregate.

## 1. Install / Build

```bash
make build
# Result: ./bin/docbuilder
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
      token: ${GIT_ACCESS_TOKEN}
  - url: https://git.example.com/org/monorepo.git
    name: monorepo
    branch: main
    paths: [docs, documentation/guides]
    auth:
      type: token
      token: ${GIT_ACCESS_TOKEN}
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

Enable incremental builds to skip unchanged repositories:

```yaml
build:
  enable_incremental: true
  cache_dir: .docbuilder-cache
  clone_strategy: auto  # Reuse existing clones
```

On subsequent runs, DocBuilder will:
- Check cached build manifests
- Skip builds if inputs haven't changed
- Significantly speed up CI pipelines

## 7. Multi-Version Documentation (Optional)

To build documentation from multiple branches/tags:

```yaml
versioning:
  enabled: true
  strategy: branches_and_tags
  max_versions_per_repo: 5
  tag_patterns:
    - "v*"           # Semantic versions
    - "[0-9]*"       # Numeric tags
  branch_patterns:
    - "main"
    - "develop"
```

This will:
- Discover all matching branches/tags
- Clone each version separately
- Generate version-specific content paths
- Create Hugo config with version metadata

```yaml
build:
  clone_strategy: auto
  shallow_depth: 5
```

Re-run:

```bash
./bin/docbuilder build -c config.yaml -v
```

You’ll see logs about unchanged repository heads and (when applicable) an unchanged documentation set.

## 7. Next Steps

- Customize landing pages with `templates/index/*.tmpl`.
- Pick a supported theme (`hextra` or `docsy`).
- Integrate with CI: compare `doc_files_hash` between runs to skip downstream jobs.

---
**You are ready to explore How‑To guides for specific tasks.**
