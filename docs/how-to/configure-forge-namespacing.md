---
aliases:
  - /_uid/a8161eb4-7b61-46e5-81c8-cfa763e8d26e/
categories:
  - how-to
date: 2025-12-15T00:00:00Z
fingerprint: 3adea9e83bbb8a570efaf476d302c27dd653b4237ed14be0b3a1119a47d48b66
lastmod: "2026-06-02"
tags:
  - forge
  - namespacing
  - configuration
title: 'How To: Configure Forge Namespacing'
uid: a8161eb4-7b61-46e5-81c8-cfa763e8d26e
---

# How To: Configure Forge Namespacing

Forge namespacing helps avoid repository name collisions when aggregating multiple hosting platforms (GitHub, GitLab, Forgejo, etc.).

## Modes

Configured via `build.namespace_forges`:

- `auto` (default): Add `<forge>/` prefix only if more than one distinct forge type is present.
- `always`: Always prefix with the forge type when known.
- `never`: Never add the prefix (legacy layout).

## Explicit Namespace Alias

By default, DocBuilder detects the forge type from repository metadata. You can override this by setting the `namespace` field in the repository configuration:

```yaml
repositories:
  - name: my-docs
    url: https://github.com/example/docs
    namespace: github  # Explicit forge namespace
```

The `namespace` field takes precedence over automatic detection. This is useful when:
- Automatic detection fails (e.g., self-hosted GitLab without explicit forge metadata)
- You want to use a custom namespace for grouping

## Group Collision Resolution

When multiple repositories share the same name across different GitLab/GitHub groups, use the `group` field to disambiguate:

```yaml
repositories:
  - name: docs
    url: https://github.com/acme/docs
    namespace: github
    group: engineering
  - name: docs
    url: https://github.com/initech/docs
    namespace: github
    group: product
```

This creates paths like:

```
content/
  github/
    engineering/
      docs/...
    product/
      docs/...
```

**Note**: Repository names must still be unique when combined with their namespace and group. If you have two repos with the same name in the same group, you should differentiate them (e.g., `group-a/common-name` vs `group-b/common-name`).

## Example Layouts

Multiple forges (auto or always):

```
content/
  github/
    service-a/...
  gitlab/
    service-b/...
```

With groups:

```
content/
  github/
    engineering/
      service-a/...
    product/
      service-a/...
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
