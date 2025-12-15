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

Relearn supports multiple built-in color schemes:

- `auto` - Automatically switches between light and dark based on system preferences (default)
- `relearn-light` - Official light theme
- `relearn-dark` - Official dark theme
- `learn` - Original Learn theme colors
- `neon` - Bright, high-contrast theme
- `blue`, `green`, `red` - Colored variants

Set via:
```yaml
hugo:
  params:
    themeVariant: "relearn-dark"
```

## Default Settings

DocBuilder automatically configures these defaults for Relearn:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `disableSearch` | `false` | Enable Lunr.js search |
| `themeVariant` | `"auto"` | Auto light/dark mode |
| `showVisitedLinks` | `true` | Mark visited pages |
| `collapsibleMenu` | `true` | Collapsible sidebar sections |
| `disableBreadcrumb` | `false` | Show breadcrumb navigation |
| `disableLandingPageButton` | `true` | Hide landing page button |
| `mermaid.enable` | `true` | Enable Mermaid diagrams |
| `math.enable` | `true` | Enable MathJax support |
| `editURL.enable` | `true` | Show "Edit this page" links |

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
