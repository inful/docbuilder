---
aliases:
  - /_uid/57b21142-f4d9-4cb5-b0b8-f4bc55f520d8/
categories:
  - how-to
date: 2026-05-30T00:00:00Z
fingerprint: f699eafb5f00d7e50bccfeb11124818f5a8048aae92398384dec63df4f2ac193
lastmod: "2026-06-04"
tags:
  - documentation
  - images
  - markdown
  - preview
title: 'How To: Write Image Links'
uid: 57b21142-f4d9-4cb5-b0b8-f4bc55f520d8
---

# How to Write Image Links

Use this guide to write markdown image references that work consistently in both local preview and production builds.

## Asset Size Limits

DocBuilder enforces a default maximum asset size of **50MB** to prevent memory pressure during builds. Files exceeding this limit are skipped with a warning logged to the console.

To adjust the limit, set `max_asset_size` in your configuration:

```yaml
build:
  max_asset_size: 104857600  # 100MB in bytes
```

## Recommended Image Link Style

Use page-relative paths in markdown:

```markdown
![Architecture](images/architecture.png)
![Flow](../assets/flow.svg)
```

These are the safest defaults for documentation authoring.

## What DocBuilder Does

During content processing, DocBuilder rewrites image paths to match generated Hugo content paths. It identifies image assets by performing a filesystem walk of the configured documentation directories and copying files with supported extensions into the content tree.

In single-repository preview mode:

- Repository namespace is omitted from generated content paths.
- Relative image links resolve to section-rooted paths.
- Example from guides/intro.md:
  - Source: `![Diagram](images/diagram.png)`
  - Resolved URL: /guides/images/diagram.png

In multi-repository production builds:

- Repository namespace is included.
- Optional forge namespace may also be included.
- Example:
  - Source: `![Diagram](images/diagram.png)`
  - Resolved URL: /my-repo/guides/images/diagram.png
  - Or with forge namespace: /gitlab/my-repo/guides/images/diagram.png

## HTML Image Tags

DocBuilder also rewrites HTML img tags with relative sources:

```html
<img src="images/banner.jpg" alt="Banner" />
```

Absolute external URLs are left unchanged.

## Supported and Unchanged Cases

- External image URLs are unchanged:
  - ![Logo](https://example.com/logo.png)
- Root-relative image paths are normalized for case:
  - `![Hero](/Static/Hero.PNG)` becomes /static/hero.png
- Relative image paths are normalized to lowercase to match generated content paths.

## Authoring Best Practices

1. Keep images near the document that references them, usually in an images subdirectory.
2. Prefer relative links over hard-coding repository-prefixed URLs.
3. Use markdown image syntax unless HTML img is required.
4. Avoid spaces and mixed-case filenames in image assets.
5. Validate changes with local preview before merging.

## Troubleshooting

Image does not render in preview:

1. Verify file extension is supported (png, jpg, jpeg, gif, svg, webp, bmp, ico).
2. Confirm the file exists under a configured docs path.
3. Check path case on disk versus markdown reference.
4. Rebuild preview after adding new files.
5. Check if the file exceeds `max_asset_size` (see console warnings).

Image renders in preview but not production:

1. Confirm repository and forge namespacing expectations for your build.
2. Verify the image file is committed and discoverable in the source repository.
3. Check for path collisions caused by case-only filename differences.
