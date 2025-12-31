package hugo

import (
	"maps"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

const documentationType = "docs"

// FrontMatterInput bundles inputs required to build or augment front matter.
type FrontMatterInput struct {
	File     docs.DocFile
	Existing map[string]any // parsed existing front matter (may be empty)
	Config   *config.Config
	Now      time.Time
}

// BuildFrontMatter merges existing front matter with generated defaults and theme-specific additions.
// Behavior mirrors the inlined logic previously in processMarkdownFile to preserve output parity.
func BuildFrontMatter(in FrontMatterInput) map[string]any {
	fm := map[string]any{}
	// shallow copy
	maps.Copy(fm, in.Existing)

	// Title
	if fm["title"] == nil && in.File.Name != "index" {
		// Convert kebab or snake to Title Case: getting-started -> Getting Started
		base := in.File.Name
		base = strings.ReplaceAll(base, "_", "-")
		parts := strings.Split(base, "-")
		for i, part := range parts {
			if part == "" {
				continue
			}
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
		fm["title"] = strings.Join(parts, " ")
	}
	// Date
	if fm["date"] == nil {
		fm["date"] = in.Now.Format("2006-01-02T15:04:05-07:00")
	}
	// Repository & Section
	fm["repository"] = in.File.Repository
	if in.File.Forge != "" {
		fm["forge"] = in.File.Forge
	}
	if in.File.Section != "" {
		fm["section"] = in.File.Section
	}
	// Metadata passthrough
	for k, v := range in.File.Metadata {
		if fm[k] == nil {
			fm[k] = v
		}
	}

	// Ensure type: docs for all themes (must come after metadata to override tags)
	if in.Config != nil {
		fm["type"] = documentationType
	}

	// Per-page edit URL if not already present – tests expect BuildFrontMatter to set it.
	if _, exists := fm["editURL"]; !exists {
		if in.Config != nil {
			resolver := NewEditLinkResolver(in.Config)
			if edit := resolver.Resolve(in.File); edit != "" {
				fm["editURL"] = edit
			}
		}
	}

	return fm
}

// parseExistingFrontMatter removed (unused)

// (Future) Additional front matter transformations can compose here.
