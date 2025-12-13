# Enable Page Transitions

This guide explains how to enable smooth page transitions using the View Transitions API in your Hextra-themed documentation site.

## Overview

Page transitions provide a smooth, animated navigation experience between pages in your documentation site. DocBuilder supports the View Transitions API for Hextra themes, creating fluid animations when users navigate between documentation pages.

## Prerequisites

- Hugo theme: `hextra` (View Transitions are currently only supported for Hextra)
- Modern browser with View Transitions API support (Chrome 111+, Edge 111+)

## Configuration

Add the following to your `config.yaml` under the `hugo` section:

```yaml
hugo:
  title: "My Documentation Site"
  theme: "hextra"
  
  # Enable page transitions
  enable_page_transitions: true
  
  # Optional: Configure transition duration (in milliseconds)
  # Default is 300ms if not specified
  page_transition_duration: 300
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enable_page_transitions` | boolean | `false` | Enable/disable View Transitions API |
| `page_transition_duration` | integer | `300` | Transition duration in milliseconds (100-1000) |

## Example Configuration

### Basic Setup

```yaml
hugo:
  theme: "hextra"
  enable_page_transitions: true
```

### Custom Duration

For faster transitions:

```yaml
hugo:
  theme: "hextra"
  enable_page_transitions: true
  page_transition_duration: 200
```

For slower, more dramatic transitions:

```yaml
hugo:
  theme: "hextra"
  enable_page_transitions: true
  page_transition_duration: 500
```

## How It Works

When enabled, DocBuilder:

1. Injects View Transitions API CSS and JavaScript into your Hugo site
2. Adds the necessary CSS classes for smooth page transitions
3. Configures the transition duration based on your settings
4. Automatically applies transitions to all page navigations

## Browser Compatibility

The View Transitions API is supported in:

- ✅ Chrome 111+
- ✅ Edge 111+
- ✅ Opera 97+
- ⚠️ Safari (experimental, behind flag)
- ⚠️ Firefox (in development)

For browsers without View Transitions support, the site will function normally without animations (graceful degradation).

## Verifying Transitions

After enabling transitions and rebuilding your site:

1. Navigate to your documentation site
2. Click between different pages
3. You should see smooth fade/slide animations between pages
4. Check browser console for any errors

### Troubleshooting

**Transitions not working:**
- Verify `theme: "hextra"` is set (other themes not yet supported)
- Check browser compatibility (use Chrome 111+ for testing)
- Ensure you rebuilt the site after changing configuration
- In daemon mode, the configuration reload should trigger an automatic rebuild

**Transitions too fast/slow:**
- Adjust `page_transition_duration` (recommended range: 200-500ms)
- Values below 100ms may be imperceptible
- Values above 1000ms may feel sluggish

## Related Configuration

View Transitions work well with other Hextra features:

```yaml
hugo:
  theme: "hextra"
  enable_page_transitions: true
  params:
    search:
      enable: true
      type: flexsearch
    theme:
      default: system
      displayToggle: true
```

## Disabling Transitions

To disable transitions, set `enable_page_transitions: false` or remove the option entirely:

```yaml
hugo:
  theme: "hextra"
  # enable_page_transitions: false  # Explicitly disabled
```

## Performance Considerations

- Transitions add minimal overhead (~2KB of CSS/JS)
- Static assets are embedded at build time
- No runtime performance impact on browsers without View Transitions support
- Transitions do not affect SEO or accessibility

## See Also

- [Hextra Theme Configuration](add-theme-support.md)
- [Hugo Configuration Reference](../reference/configuration.md)
- [View Transitions API Documentation](https://developer.mozilla.org/en-US/docs/Web/API/View_Transitions_API)
