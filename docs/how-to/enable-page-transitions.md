---
title: "How To: Enable Page Transitions"
date: 2025-12-15
categories:
  - how-to
tags:
  - ui
  - transitions
  - hextra
  - docsy
  - relearn
---

# Enable Page Transitions

This guide explains how to enable smooth page transitions using the View Transitions API in your Hugo-themed documentation site.

## Overview

Page transitions provide a smooth, animated navigation experience between pages in your documentation site. DocBuilder supports the View Transitions API for all supported themes (Hextra, Docsy, and Relearn), creating fluid animations when users navigate between documentation pages.

The implementation uses browser-native CSS-only transitions with the `@view-transition { navigation: auto; }` rule, which means no JavaScript is required and all interactive elements (like search) continue to work correctly during and after transitions.

## Prerequisites

- Hugo theme: `hextra`, `docsy`, or `relearn`
- Modern browser with View Transitions API support:
  - Chrome 126+
  - Edge 126+
  - Safari 18.2+
  - Opera 112+
  - Firefox: In review

## Configuration

Add the following to your `config.yaml` under the `hugo` section:

```yaml
hugo:
  title: "My Documentation Site"
  theme: "hextra"  # or "docsy" or "relearn"
  
  # Enable page transitions
  enable_page_transitions: true
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enable_page_transitions` | boolean | `false` | Enable/disable View Transitions API |

## Example Configuration

### Basic Setup

```yaml
hugo:
  theme: "hextra"
  enable_page_transitions: true
```

## How It Works

When enabled, DocBuilder:

1. Injects View Transitions API CSS into your Hugo site
2. Adds the `@view-transition { navigation: auto; }` rule to enable browser-native transitions
3. Automatically applies transitions to all page navigations
4. Preserves all interactive elements (search, menus, etc.) without any DOM manipulation
5. Uses theme-appropriate head partial integration:
   - **Hextra**: `layouts/_partials/custom/head-end.html`
   - **Docsy**: `layouts/partials/hooks/head-end.html`
   - **Relearn**: `layouts/partials/custom-header.html`

## Browser Compatibility

The View Transitions API is supported in:

- ✅ Chrome 126+
- ✅ Edge 126+
- ✅ Safari 18.2+
- ✅ Opera 112+
- ⚠️ Firefox (in review)

For browsers without View Transitions support, the site will function normally without animations (graceful degradation).

## Verifying Transitions

After enabling transitions and rebuilding your site:

1. Navigate to your documentation site
2. Click between different pages
3. You should see smooth fade animations between pages
4. Verify interactive elements (search, menus) continue to work correctly
5. Check browser console for any errors

### Troubleshooting

**Transitions not working:**
- Check browser compatibility (use Chrome 126+ or Safari 18.2+ for testing)
- Ensure you rebuilt the site after changing configuration
- In daemon mode, restart the daemon to apply configuration changes
- Verify the CSS file exists at `/static/view-transitions.css`

**Interactive elements not working after transition:**
- This should not happen with the CSS-only implementation
- If you experience issues, please report a bug

## Theme-Specific Notes

### Hextra
Works seamlessly with FlexSearch and theme switcher.

### Docsy
Compatible with Algolia/local search and Bootstrap components.

### Relearn
Works with Lunr search, Mermaid diagrams, and math rendering.

## Related Configuration

View Transitions work well with other theme features:

```yaml
hugo:
  theme: "relearn"  # or "hextra" or "docsy"
  enable_page_transitions: true
  params:
    # Theme-specific parameters work alongside transitions
    search:
      enable: true
    mermaid:
      enable: true
```

## Disabling Transitions

To disable transitions, set `enable_page_transitions: false` or remove the option entirely:

```yaml
hugo:
  theme: "hextra"
  # enable_page_transitions: false  # Explicitly disabled
```

## Performance Considerations

- Transitions add minimal overhead (~1KB of CSS)
- Static assets are embedded at build time
- No runtime performance impact on browsers without View Transitions support
- Transitions do not affect SEO or accessibility

## Technical Details

The implementation uses the browser's native View Transitions API with a simple CSS rule:

```css
@view-transition {
  navigation: auto;
}
```

This tells the browser to automatically handle cross-document page transitions without any JavaScript intervention. The browser manages the DOM updates, preserving all event handlers and script contexts, which is why interactive elements continue to work correctly.

For more information, see:
- [Lincoln Loop Blog: View Transitions with Hugo](https://lincolnloop.com/blog/view-transitions-with-hugo/)
- [MDN: View Transitions API](https://developer.mozilla.org/en-US/docs/Web/API/View_Transitions_API)

## See Also

- [Hextra Theme Configuration](add-theme-support.md)
- [Hugo Configuration Reference](../reference/configuration.md)
