---
title: "How To: Use Relearn Theme"
date: 2025-12-15
categories:
  - how-to
tags:
  - themes
  - relearn
  - hugo
---

# Hugo Relearn Theme Support

DocBuilder includes built-in support for the [Hugo Relearn theme](https://github.com/McShelby/hugo-theme-relearn), a documentation-focused theme with extensive features for technical documentation.

## Features

The Relearn theme integration provides:

- **Hugo Modules**: Automatic theme installation via Hugo Modules (no manual theme setup required)
- **Math Support**: Built-in MathJax integration for mathematical and chemical formulae
- **Mermaid Diagrams**: Native support for Mermaid diagram rendering
- **Search**: Lunr.js-powered offline search
- **Edit Links**: Automatic "Edit this page" links to source repositories
- **Customizable Appearance**: Multiple color variants and dark mode support
- **Responsive Design**: Mobile-friendly layout
- **Multilingual**: Support for 20+ languages with RTL support

## Quick Start

### Basic Configuration

```yaml
version: "2.0"

repositories:
  - name: my-docs
    url: https://github.com/example/docs.git
    branch: main
    paths:
      - docs

hugo:
  title: "Documentation"
  theme: "relearn"
  base_url: "https://docs.example.com"

output:
  directory: "./site"
```

### Advanced Configuration

```yaml
hugo:
  title: "Technical Documentation"
  theme: "relearn"
  base_url: "https://docs.example.com"
  
  params:
    # Theme appearance
    themeVariant: "auto"  # auto, relearn-light, relearn-dark, learn, neon, blue, green, red
    
    # Navigation
    showVisitedLinks: true
    collapsibleMenu: true
    alwaysopen: false
    disableBreadcrumb: false
    
    # Search
    disableSearch: false
    
    # Features
    disableLandingPageButton: true
    disableShortcutsTitle: false
    disableTagHiddenPages: false
    
    # Footer
    disableGeneratorVersion: false
    
    # Edit links (automatically configured by DocBuilder)
    editURL:
      enable: true
    
    # Mermaid diagrams
    mermaid:
      enable: true
    
    # Math support
    math:
      enable: true
```

## Theme Variants

Relearn includes multiple built-in color schemes that can be configured in simple or advanced modes.

### Shipped Variants

The theme ships with the following color variants:

**Relearn Family:**
- `relearn-light` - Classic Relearn default with signature green, dark sidebar and light content
- `relearn-dark` - Dark variant with signature green, dark sidebar and dark content
- `relearn-bright` - Alternative with signature green, green sidebar and light content

**Zen Family:**
- `zen-light` - Relaxed white/grey variant with blue accents, light sidebar and light content
- `zen-dark` - Dark variant with blue accents, dark sidebar and dark content

**Experimental:**
- `neon` - Glowing dark theme with gradient sidebar

**Retro (Learn Theme):**
- `learn` - Original Learn theme with light purple, dark sidebar and light content
- `blue` - Blue-tinted Learn theme
- `green` - Green-tinted Learn theme
- `red` - Red-tinted Learn theme

### Simple Configuration

#### Single Variant

Use a single variant for your entire site:

```yaml
hugo:
  params:
    themeVariant: "relearn-dark"
```

#### Multiple Variants with Selector

Let users choose between variants via a switcher in the menu:

```yaml
hugo:
  params:
    themeVariant:
      - "relearn-light"
      - "relearn-dark"
      - "neon"
```

The first variant is the default. A variant selector appears automatically when multiple variants are configured.

#### Auto Mode (OS Light/Dark Detection)

Use `auto` to match the operating system's light/dark preference:

```yaml
hugo:
  params:
    themeVariant:
      - "auto"
      - "relearn-light"
      - "relearn-dark"
```

By default, `auto` uses `relearn-light` for light mode and `relearn-dark` for dark mode. You can customize this:

```yaml
hugo:
  params:
    themeVariant:
      - "auto"
      - "zen-light"
      - "neon"
    themeVariantAuto:
      - "zen-light"  # Light mode variant
      - "neon"       # Dark mode variant
```

### Advanced Configuration

For more control over variant names, auto-mode behavior, and logos, use the advanced array format:

```yaml
hugo:
  params:
    themeVariant:
      # Auto mode with custom name
      - identifier: "relearn-auto"
        name: "Relearn Light/Dark"
        auto:
          - "relearn-light"
          - "relearn-dark"
      
      # Standard variants
      - identifier: "relearn-light"
      
      - identifier: "relearn-dark"
      
      - identifier: "neon"
        name: "Neon Glow"
      
      # Zen auto mode
      - identifier: "zen-auto"
        name: "Zen Light/Dark"
        auto:
          - "zen-light"
          - "zen-dark"
      
      - identifier: "zen-light"
      
      - identifier: "zen-dark"
```

**Advanced Parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `identifier` | Yes | Name of the color variant (must match `theme-<identifier>.css`) |
| `name` | No | Display name in variant selector (defaults to identifier in human-readable form) |
| `auto` | No | Array of two variants: [light-mode, dark-mode] for OS detection |
| `logo` | No | Override the default logo for this variant |

### Custom Variants

You can create custom variants by:

1. **Copy and modify**: Copy a shipped variant from `themes/hugo-theme-relearn/assets/css/theme-*.css` to your site's `assets/css/` directory
2. **Import and extend**: Create a new CSS file that imports a base variant and overrides specific variables

Example custom variant (`assets/css/theme-my-brand.css`):

```css
@import "theme-relearn-light.css";

:root {
  --PRIMARY-color: rgba(96, 125, 139, 1);        /* Your brand color */
  --SECONDARY-color: rgba(236, 239, 241, 1);     /* Accent color */
  --CODE-theme: neon;                             /* Code highlighting */
  --CODE-BLOCK-color: rgba(226, 228, 229, 1);
  --CODE-BLOCK-BG-color: rgba(40, 42, 54, 1);
}
```

Then use it in your config:

```yaml
hugo:
  params:
    themeVariant: "my-brand"
```

See the [Relearn Color Documentation](https://mcshelby.github.io/hugo-theme-relearn/configuration/branding/colors/) and [Stylesheet Generator](https://mcshelby.github.io/hugo-theme-relearn/configuration/branding/generator/) for more customization options.

## Default Settings

DocBuilder automatically configures these defaults for Relearn:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `themeVariant` | `["auto", "zen-light", "zen-dark"]` | Auto light/dark mode with variant selector |
| `themeVariantAuto` | `["zen-light", "zen-dark"]` | OS light/dark mode fallbacks |
| `showVisitedLinks` | `true` | Mark visited pages |
| `collapsibleMenu` | `true` | Collapsible sidebar sections |
| `alwaysopen` | `false` | Don't force menu sections open |
| `disableBreadcrumb` | `false` | Show breadcrumb navigation |
| `disableLandingPageButton` | `true` | Hide landing page button |
| `disableShortcutsTitle` | `false` | Show shortcuts menu in sidebar |
| `disableTagHiddenPages` | `false` | Tag hidden pages |
| `disableGeneratorVersion` | `false` | Show generator version in footer |
| `mermaid.enable` | `true` | Enable Mermaid diagrams |
| `math.enable` | `true` | Enable MathJax support |

**Note:** `editURL` is not set by default. Configure it manually if you want "Edit this page" links.

All defaults can be overridden in your configuration's `hugo.params` section.

## Hugo Module Configuration

DocBuilder uses Hugo Modules to automatically install Relearn. The theme is configured as:

- Module Path: `github.com/McShelby/hugo-theme-relearn`
- Version: `v8.3.0`

No manual theme installation required - Hugo will download the theme on first build.

## Content Structure

Relearn builds navigation from your content structure. Place an `_index.md` in each directory to create sections:

```
content/
├── _index.md              # Home page
├── getting-started/
│   ├── _index.md          # Section index
│   └── installation.md
└── advanced/
    ├── _index.md
    └── configuration.md
```

## Shortcodes

Relearn includes many built-in shortcodes for rich content:

- `notice` - Styled notice boxes (info, warning, tip, note)
- `expand` - Expandable content sections
- `tabs` and `tab` - Tabbed content
- `button` - Styled buttons
- `mermaid` - Mermaid diagrams
- `math` - Mathematical formulae

See [Relearn Shortcodes Documentation](https://mcshelby.github.io/hugo-theme-relearn/shortcodes/notice) for full list.

## Troubleshooting

### Theme Not Loading

Ensure Hugo is installed and run:
```bash
cd site
hugo mod get -u
hugo server
```

### Search Not Working

Relearn search requires JavaScript. Ensure you're viewing the built site (not raw markdown):
```bash
cd site
hugo server
```

### Edit Links Not Appearing

Verify your repository configuration includes the forge URL:
```yaml
repositories:
  - url: https://github.com/example/docs.git  # Must be a valid forge URL
    branch: main
```

## Resources

- [Relearn Theme Documentation](https://mcshelby.github.io/hugo-theme-relearn/)
- [Relearn GitHub Repository](https://github.com/McShelby/hugo-theme-relearn)
- [Hugo Modules Documentation](https://gohugo.io/hugo-modules/)
- [DocBuilder Theme Guide](../how-to/add-theme-support.md)

## Comparison with Other Themes

| Feature | Relearn | Hextra | Docsy |
|---------|---------|--------|-------|
| Search | Lunr.js | FlexSearch | Algolia/Local |
| Math | MathJax | KaTeX | Limited |
| Mermaid | Built-in | Built-in | Plugin |
| Multilingual | 20+ languages | Basic | Full i18n |
| Mobile | Excellent | Excellent | Good |
| Customization | High | Medium | High |
| Learning Curve | Low | Low | Medium |

Choose Relearn if you need:
- Rich documentation features out-of-the-box
- Strong multilingual support
- Extensive built-in shortcodes
- Math and diagram support
