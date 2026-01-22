---
aliases:
  - /_uid/56876591-4835-49a5-a63e-494590a557d5/
categories:
  - how-to
date: 2025-12-15T00:00:00Z
fingerprint: cde4319931551f9681d70d5cd64b149abc1e1c5c2b9c342bc4eaf6208de96d62
lastmod: "2026-01-22"
tags:
  - optimization
  - workspace
  - performance
title: 'How To: Prune Workspace Size'
uid: 56876591-4835-49a5-a63e-494590a557d5
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
