---
uid: 1bc938b7-2e2c-47d1-8192-06e2300d09aa
aliases:
  - /_uid/1bc938b7-2e2c-47d1-8192-06e2300d09aa/
title: "How To: Write Cross-Document Links"
date: 2025-12-15
categories:
  - how-to
tags:
  - documentation
  - links
  - markdown
fingerprint: 58297ba3725d544262e634420d2be6c48746393f73cdeee48bdcae462d7fa6a8
---

# How to Write Cross-Document Links

When writing markdown documentation that will be processed by DocBuilder, you have three options for linking between documents.

## Design Goal: Dual Compatibility

**DocBuilder's link transformation system is designed so your documentation works in both contexts:**

1. **In the source forge** (GitHub, GitLab, Forgejo) - Links work correctly when viewing files directly in the repository web interface
2. **In the generated Hugo site** - The same links are transformed to work with Hugo's URL structure

This dual compatibility means you can write standard relative markdown links (like `[guide](../guide.md)`) and they'll work correctly when:
- Developers browse documentation directly in GitHub/GitLab
- Users view the rendered documentation site
- Documentation is reviewed in pull requests/merge requests

The transform pipeline automatically rewrites links during the build process. You write links once, and they work everywhere.

## Link Types

### 1. Page-Relative Links (Recommended for Nearby Files)

Use relative paths from the current file's location. These work like standard filesystem paths.

```markdown
<!-- From tutorials/getting-started.md -->
[Other tutorial](advanced-usage.md)           → /repo/tutorials/advanced-usage/
[Up one level](../README.md)                  → /repo/
[Different section](../how-to/authentication.md) → /repo/how-to/authentication/
```

**Best for:** Links to files in the same directory or nearby directories.

### 2. Repository-Root-Relative Links (Recommended for Cross-Section Links)

Use paths starting with `/` to link from the repository root, regardless of where the current file is located.

```markdown
<!-- Works from any file in the repository -->
[API Reference](/api/reference.md)             → /repo/api/reference/
[How-To Guide](/how-to/authentication.md)      → /repo/how-to/authentication/
[Tutorial](/tutorials/getting-started.md)      → /repo/tutorials/getting-started/
```

**Best for:** Links between different major sections of documentation, deep directory structures, or when you want stable links that don't break if files move.

**Important:** Repository-root-relative links are relative to *your repository's root*, not the Hugo site root. DocBuilder automatically prefixes them with the repository (and forge, if applicable) during processing.

### 3. Absolute Hugo Links (For Advanced Users)

Use the full Hugo path including the repository name. Only necessary if linking between different repositories in a multi-repo documentation site.

```markdown
[Other repo docs](/other-repo/guide.md)        → /other-repo/guide/
[Forged repo](/github/org-repo/api.md)         → /github/org-repo/api/
```

**Best for:** Cross-repository links in multi-repo documentation sites.

## Link Syntax Rules

### Extension Handling

DocBuilder automatically removes `.md` and `.markdown` extensions and adds trailing slashes for Hugo's pretty URLs:

```markdown
[Link](guide.md)           → [Link](guide/)
[Link](tutorial.markdown)  → [Link](tutorial/)
```

### Anchor Support

Anchors (fragments) are preserved:

```markdown
[Section link](guide.md#installation)           → [Section link](guide/#installation)
[Repo-root anchor](/api/reference.md#errors)    → [Repo-root anchor](/repo/api/reference/#errors)
```

### Unchanged Links

These link types are never modified:

- External links: `https://example.com/page.md`
- Email links: `mailto:user@example.com`
- Anchor-only links: `#section-heading`
- Non-markdown links: `image.png`, `document.pdf`

## Common Patterns

### Linking from Tutorials to How-Tos

If your repository has this structure:
```
docs/
  tutorials/
    getting-started.md
  how-to/
    authentication.md
```

From `tutorials/getting-started.md`:

```markdown
<!-- Page-relative (requires ../): -->
See the [authentication guide](../how-to/authentication.md) for details.

<!-- Repository-root-relative (cleaner): -->
See the [authentication guide](/how-to/authentication.md) for details.
```

### Linking Within the Same Section

```markdown
<!-- From how-to/authentication.md to how-to/authorization.md -->
After authentication, see [authorization](authorization.md).
```

### Linking to Index Pages

```markdown
<!-- Link to a section's index page -->
[API Documentation](/api/README.md)    → /repo/api/
```

## Troubleshooting

### Links Break When Files Move

**Problem:** You used page-relative links, and moving files breaks the references.

**Solution:** Use repository-root-relative links (starting with `/`) for stability:

```markdown
<!-- Before (breaks if file moves): -->
[Guide](../how-to/authentication.md)

<!-- After (stable): -->
[Guide](/how-to/authentication.md)
```

### Wrong Path in Generated Site

**Problem:** Link resolves to `/repo/tutorials/how-to/authentication/` instead of `/repo/how-to/authentication/`

**Cause:** Using page-relative link from `tutorials/` directory without going up first.

**Solution:** Either use `../how-to/authentication.md` or use repository-root-relative `/how-to/authentication.md`.

### Link Works in Source but Not in Hugo Site

**Problem:** Repository-root-relative link works in source repo but not in DocBuilder output.

**Cause:** Links starting with `/` are treated as repository-root-relative and automatically prefixed.

**Solution:** This is expected behavior. DocBuilder handles the prefixing automatically.

## Best Practices

1. **Use repository-root-relative links** (`/section/file.md`) for major cross-section navigation
2. **Use page-relative links** (`file.md`, `../section/file.md`) for nearby files in the same section
3. **Always use `.md` extension** in source markdown - DocBuilder removes it automatically
4. **Don't worry about trailing slashes** - DocBuilder adds them automatically
5. **Test in preview** before deploying to ensure links work as expected

## Examples by Use Case

### Documentation Index

```markdown
<!-- From README.md linking to major sections -->
# Documentation

- [Getting Started](/tutorials/getting-started.md)
- [How-To Guides](/how-to/README.md)
- [API Reference](/api/reference.md)
```

### Tutorial Series

```markdown
<!-- From tutorials/part-2.md -->
← Previous: [Part 1](part-1.md)
→ Next: [Part 3](part-3.md)
```

### API Documentation with Cross-References

```markdown
<!-- From api/authentication.md -->
See [authorization](/api/authorization.md) and [error handling](/api/errors.md).
```

### How-To Guide with Related Content

```markdown
<!-- From how-to/deploy.md -->
Before deploying, complete the [configuration tutorial](/tutorials/configuration.md)
and review the [deployment API](/api/deployment.md).
```
