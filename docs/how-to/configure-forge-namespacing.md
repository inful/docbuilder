---
aliases:
  - /_uid/a8161eb4-7b61-46e5-81c8-cfa763e8d26e/
categories:
  - how-to
date: 2025-12-15T00:00:00Z
fingerprint: f5cc4f7320810e14b51caba57f0d636ca908eb9a333f3d8e86c9f4cf74951a2f
lastmod: "2026-11-19"
tags:
  - forge
  - namespacing
  - configuration
title: 'How To: Configure Forge Namespacing'
uid: a8161eb4-7b61-46e5-81c8-cfa763e8d26e
---

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

## Repository Names Are Unique

Every repository needs its own name. Two repositories with one name would
share a clone folder, a state entry and a content folder, so one of them
would silently vanish from the site. A configuration with two repositories
of the same name is refused when it is loaded.

Give each repository a name that says where it comes from:

```yaml
repositories:
  - name: acme-docs
    url: https://github.com/acme/docs
    namespace: github
  - name: initech-docs
    url: https://github.com/initech/docs
    namespace: github
```

This creates paths like:

```
content/
  github/
    acme-docs/...
    initech-docs/...
```

Repositories that are found on a forge do not need this: when two of them
share a name, the second one gets a name from its full path
(`acme/docs` becomes `acme-docs`).

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
