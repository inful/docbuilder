---
title: "How To: Prune Workspace Size"
date: 2025-12-15
categories:
  - how-to
tags:
  - optimization
  - workspace
  - performance
---

# How To: Prune Workspace Size

Reduce disk usage by enabling top-level pruning of non‑documentation directories inside cloned repositories.

## Enable Pruning

```yaml
build:
  prune_non_doc_paths: true
```

## Control Allow / Deny Lists

```yaml
build:
  prune_allow: [LICENSE*, README.*]
  prune_deny: ["*.bak", test]
```

Precedence (highest wins):

1. `.git` (never removed)
2. Explicit deny (glob or exact)
3. Docs roots (derived from first segment of each configured docs path)
4. Explicit allow
5. Removal

## When To Use

- Large monorepos where only `docs/` subtree is needed.
- CI environments with tight ephemeral storage quotas.

## Risks

- Removing assets (images, includes) referenced by Markdown if they live outside allowed roots.

## Mitigation

- Add required top-level directories to `prune_allow`.
- Temporarily disable pruning to confirm root cause of missing references.

## Example

Repository tree:

```
README.md
build/
cmd/
docs/
scripts/
```

Config:

```yaml
build:
  prune_non_doc_paths: true
  prune_allow: [README.*]
```

Result keeps `.git`, `docs/`, `README.md`; prunes `build/`, `cmd/`, `scripts/`.

## Verification

Inspect repo directory after clone stage (verbose logging) or script a simple check:

```bash
find workspace/service-a -maxdepth 1 -mindepth 1 -type d
```
