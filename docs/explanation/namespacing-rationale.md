---
aliases:
  - /_uid/23db9b8b-6c79-4e9d-b8e6-6afe9251dc94/
categories:
  - explanation
date: 2025-12-15T00:00:00Z
fingerprint: ad3360abc55ded35015adf53f1fcdb4c8553a7b40490e29ce12e0380842108fe
lastmod: "2026-01-22"
tags:
  - architecture
  - namespacing
  - design-decisions
title: Namespacing Rationale
uid: 23db9b8b-6c79-4e9d-b8e6-6afe9251dc94
---

# Forge Namespacing Rationale

## Problem

Aggregating multiple repositories from different hosting forges (GitHub, GitLab, Forgejo, etc.) risks name collisions (`service-api` existing on two platforms) and makes future cross-forge expansion disruptive if paths are encoded without forge context.

## Options Considered

| Option | Pros | Cons |
|--------|------|------|
| Never prefix | Shorter paths | Collisions; retrofits painful |
| Always prefix | Stable & explicit | Longer paths for single-forge installs |
| Conditional (current) | Shorter single-forge, collision-safe multi-forge | Slight path shape change when second forge added |

## Chosen Strategy

`namespace_forges=auto` (default): Insert `<forge>/` only when >1 forge type detected among active repositories. Users wanting stability ahead of expansion can set `always`.

## Trade-Offs

- Pros: Minimal path length for the common single-forge scenario; zero ambiguity in multi-forge.
- Cons: Path structure changes the first time a second forge appears (mitigated by recommending `always` when future expansion is known early).

## Front Matter Inclusion

Including `forge` in generated page front matter allows theming & navigation to group or filter content by hosting platform.

## Future Possibilities

- Per-forge landing pages summarizing repositories & health.
- Analytics segmented by forge type (build counts, failure rates).
- Theme navigation grouping (sidebars by forge).

## Migration Guidance

If you start single-forge and later add a second forge, set `namespace_forges: always` for one build; adjust any hard-coded links; then leaving it on `auto` remains safe.
