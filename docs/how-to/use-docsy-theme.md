---
aliases:
  - /_uid/aad82557-a8ca-417e-9da3-e5cec27f80df/
  - /how-to/use-relearn-theme/
categories:
  - how-to
date: 2025-12-15T00:00:00Z
description: The Docsy theme, and what the build does for you
fingerprint: ae5554abdb2a23a37adcf8c52bd2f6920c24b76d75230f98c3b424792ad59bbd
lastmod: "2026-11-22"
tags:
  - themes
  - docsy
  - hugo
title: 'How To: Use the Docsy Theme'
uid: aad82557-a8ca-417e-9da3-e5cec27f80df
---

DocBuilder builds the site with the [Hugo Docsy theme](https://github.com/docsy/docsy),
a documentation-focused theme with the navigation, the search and the
page furniture a technical documentation site needs.

There is nothing to configure to get it: the build imports the theme,
writes the layout overrides, and ships the stylesheet.

## What the build does for you

- **The theme**: imported as a Hugo Module (`github.com/docsy/docsy/theme`,
  version `v0.18.1`), so there is nothing to install by hand.
- **The stylesheet**: shipped prebuilt. Hugo 0.170 has no Sass support
  of its own, so DocBuilder vendors the compiled stylesheet, the
  Bootstrap JavaScript and the two web fonts the stylesheet references.
  `scripts/vendor-docsy-assets.sh` rebuilds them; the build needs
  neither Sass nor Node.
- **Search**: the theme's own offline index, with no JSON output format
  and no external service.
- **Edit links**: the "Edit this page" link points at the page's own
  forge - GitHub, GitLab or Gitea - computed from the repository the
  document came from.
- **Math and Mermaid**: KaTeX and Mermaid, with the versions the theme
  pins.
- **The light/dark switch** and a sidebar whose sections fold.

## The navigation

The sidebar is the content tree: the sections and pages of the
repositories, in the order the documents declare.

The `categories:` front matter of a document is a **taxonomy**, not a
sidebar block. Every category gets a landing page under
`/categories/<name>/` that lists its documents with their
descriptions - across every repository the build aggregates.

To order the sidebar, use the `weight` front matter of the section
index pages.

## Configuration

### The navigation chrome

```yaml
hugo:
  chrome: shared
```

| Mode | What it does |
|------|--------------|
| `full` (the default) | The navigation is rendered on every page. It is there without JavaScript. |
| `shared` | The navigation is rendered on one page and restored in the browser. The published site is much smaller, and the navigation needs JavaScript. |

The theme calls its shared mode experimental and recommends `full` for
production. The default here is `full` for that reason. The saving is
real on a large site: the navigation is carried by every page, so a
site with a thousand pages carries it a thousand times.

### Custom CSS

```yaml
hugo:
  custom_css: |
    .td-sidebar { font-size: 0.95rem; }
```

The CSS is inlined in the theme's head hook, after the stylesheet, so
it wins on specificity ties. A whitespace-only value suppresses the
block entirely.

### View transitions

```yaml
hugo:
  enable_page_transitions: true
```

The View Transitions API animates the move between pages. The
stylesheet and the link are written by the build.

### Taxonomies

```yaml
hugo:
  taxonomies:
    tag: tags
    category: categories
```

The default set is `tag`, `category` and `project`. Each one gets a
landing page under `/<plural>/` and a term page per value.

## The categories of a document

```yaml
---
categories: [how-to]
tags: [linting]
---
```

A document may carry several categories; it appears in each of their
landing pages. A document with no category still appears in the site -
it is simply in no category listing.

## The site does not ask the network

The theme loads several assets from a CDN: the mermaid script, the KaTeX
stylesheet and its fonts, the Bootstrap JavaScript, the Font Awesome
fonts and the search library. A build without a network died on it, and
a reader without a network saw a site without its scripts.

DocBuilder serves its own copies of all of them, the way it already
serves the compiled stylesheet: the assets are vendored in the
repository and written into the Hugo project with every build.

- `scripts/vendor-docsy-assets.sh` rebuilds the stylesheet, the
  Bootstrap JavaScript and the fonts.
- `scripts/vendor-docsy-cdn-assets.sh` fetches the KaTeX stylesheet and
  its fonts.

The mermaid script is the exception: the browser imports it as a module
from the CDN, and the build does not check that the version exists any
more, because the check was the only thing that needed the network.

## What the theme does not do

- **No per-category sidebar.** The sidebar is the content tree.
- **No Lunr search index.** The theme's offline index replaces it.
- **No theme switching.** The theme is Docsy; there is no registry of
  themes to choose from.
