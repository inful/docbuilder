---
title: "How To: Run Incremental Builds"
date: 2025-12-15
categories:
  - how-to
tags:
  - performance
  - incremental
  - builds
---

# How To: Run Incremental Builds

Incremental builds avoid recloning repositories and only fetch updates, saving time and bandwidth.

## Choose a Clone Strategy

Set in `build.clone_strategy`:

- `fresh`: Always reclone (slowest, cleanest).
- `update`: Always attempt fast-forward/hard reset existing clones.
- `auto`: (Recommended) Use `update` if the directory exists else `fresh`.

Example:

```yaml
build:
  clone_strategy: auto
  shallow_depth: 10
```

## Persisting Workspaces

To persist clones across runs, either:

1. Leave `workspace_dir` unset and keep `clone_strategy` at `auto` (default resolves a stable path), or
2. Explicitly set `build.workspace_dir` to a persistent location.

## Handling Divergence

If local branches diverge (e.g., manual changes), enable hard reset:

```yaml
build:
  hard_reset_on_diverge: true
```

Without this flag divergence becomes a reported issue (`REMOTE_DIVERGED`).

## Cleaning Untracked Files

Enable to remove stray generated or stale files after updates:

```yaml
build:
  clean_untracked: true
```

## Shallow Clones

Use `shallow_depth` to limit history and speed up fetches:

```yaml
build:
  shallow_depth: 5
```

## Detecting No-Op Builds

The build log will emit:

- `No repository head changes detected` (all repos unchanged)
- `Documentation files unchanged` (doc file set identical)

You can also compare `doc_files_hash` in successive `build-report.json` files.

## CI Optimization Pattern

Pseudocode:

```bash
prev=$(cat prev-report.json | jq -r .doc_files_hash 2>/dev/null)
./bin/docbuilder build -c config.yaml -v
current=$(cat site/build-report.json | jq -r .doc_files_hash)
if test "$prev" = "$current"; then
  echo "Docs unchanged; skipping search index regeneration"
fi
```

## Retry Behavior

Transient clone/update failures (network flake, intermittent Hugo issues) can be retried with backoff:

```yaml
build:
  max_retries: 3
  retry_backoff: exponential
  retry_initial_delay: 1s
  retry_max_delay: 30s
```

Permanent failures (auth, repo not found, unsupported protocol) short‑circuit retries and surface granular issue codes.

## Summary

Use `clone_strategy: auto`, shallow depth, and hash comparison to keep builds fast and conditional.
