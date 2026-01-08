---
categories:
    - how-to
date: 2025-12-15T00:00:00Z
id: 54275365-2e98-4150-a31c-19de1f5bce37
tags:
    - forge
    - namespacing
    - configuration
title: 'How To: Configure Forge Namespacing'
---

# How To: Configure Forge Namespacing

Forge namespacing helps avoid repository name collisions when aggregating multiple hosting platforms (GitHub, GitLab, Forgejo, etc.).

## Modes

Configured via `build.namespace_forges`:

- `auto` (default): Add `<forge>/` prefix only if more than one distinct forge type is present.
- `always`: Always prefix with the forge type when known.
- `never`: Never add the prefix (legacy layout).

## Example Layouts

Multiple forges (auto or always):

```
content/
  github/
    service-a/...
  gitlab/
    service-b/...
```

Single forge (auto or never):

```
content/
  service-a/...
```

## Front Matter

Each generated page includes `forge` in its front matter when the value is known. This lets themes and custom templates branch per forge.

## Selecting a Mode

```yaml
build:
  namespace_forges: auto   # or always | never
```

## When To Use `always`

- You expect to add a second forge later and want stable URLs now.
- You prefer explicit clarity in paths regardless of ambiguity.

## When To Use `never`

- Migrating from an older installation that hard-coded non-namespaced paths in links.

## Verifying

Run a build with `-v` and observe resulting `content/` tree or inspect a page front matter for `forge:`.

## Troubleshooting

- Missing prefix when expected: ensure repositories actually declare forge metadata (tags / detection); confirm more than one forge type is present if using `auto`.
- Unexpected prefix: you probably have at least two repo forges; switch to `never` if undesired.
