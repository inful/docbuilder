---
title: "How To: Relearn Theme Configuration"
date: 2025-12-16
categories:
  - how-to
tags:
  - themes
  - hugo
  - relearn
---

# How To: Configure the Relearn Theme

DocBuilder is hardcoded to use the [Relearn theme](https://github.com/McShelby/hugo-theme-relearn) exclusively. The theme is automatically configured via Hugo Modules with sensible defaults.

## Theme Configuration

No theme selection is needed. DocBuilder automatically:
1. Applies Relearn-specific default parameters
2. Imports the theme module: `github.com/McShelby/hugo-theme-relearn`
3. Generates a `go.mod` in the output directory with the theme dependency

## Default Relearn Parameters

DocBuilder applies these defaults (can be overridden via config):

```yaml
params:
  themeVariant: "relearn-light"
  disableSearch: false
  disableLandingPageButton: true
  collapsibleMenu: true
  showVisitedLinks: true
```

## Customizing Parameters

Override Relearn parameters via configuration:

```yaml
hugo:
  title: "My Documentation"
  description: "Documentation site"
  base_url: "https://docs.example.com/"
  params:
    themeVariant: "relearn-dark"  # Override default
    customCSS:
      - "css/custom.css"
```

User-provided parameters take precedence over defaults via deep merge.

## Overriding Index Templates

Place template overrides before running the build:

```text
outputDir/
  templates/
    index/
      main.md.tmpl
      repository.md.tmpl
      section.md.tmpl
```

DocBuilder searches these patterns (first match wins):
1. `templates/index/<kind>.md.tmpl`
2. `templates/index/<kind>.tmpl`
3. `templates/<kind>_index.tmpl`

If none match, an embedded default is used.

## Controlling Front Matter

If a template body starts with a YAML front matter fence (`---`), DocBuilder will NOT inject its own. Otherwise it prepends computed front matter (title, repository, section, forge, dates, edit link, etc.).

## Theme-Specific Features

### Search
- FlexSearch enabled by default (`disableSearch: false`)
- Automatic search index generation
- Client-side full-text search

### Navigation
- Collapsible sidebar menu (`collapsibleMenu: true`)
- Visited link tracking (`showVisitedLinks: true`)
- Automatic breadcrumbs

### Customization
- Multiple theme variants available
- Custom CSS support
- Configurable landing page

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Theme assets missing | Hugo not run | Run `hugo` manually or rerun DocBuilder with `--render-mode always`. |
| Edit links absent | Repo metadata incomplete | Ensure repo URL + branch were configured. |
| Theme variant not applied | Typo in parameter | Check `params.themeVariant` spelling in config. |

## See Also

- [Use Relearn Theme Guide](use-relearn-theme.md) - Comprehensive Relearn feature guide
- [Configuration Reference](../reference/configuration.md) - All configuration options
- [Relearn Theme Documentation](https://mcshelby.github.io/hugo-theme-relearn/) - Official theme docs
| Wrong base URL | `hugo.base_url` mismatch | Update config and rebuild. |
