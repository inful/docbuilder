package hugo

import (
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

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
	for k, v := range in.Existing { // shallow copy
		fm[k] = v
	}

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
	if in.File.Section != "" {
		fm["section"] = in.File.Section
	}
	// Metadata passthrough
	for k, v := range in.File.Metadata {
		if fm[k] == nil {
			fm[k] = v
		}
	}

	// Per-page edit URL (Hextra only) if not already present
	if _, exists := fm["editURL"]; !exists {
		// use resolver (cheap to allocate if generator not available)
		if edit := generatePerPageEditURL(in.Config, in.File); edit != "" {
			fm["editURL"] = edit
		}
	}

	return fm
}

// parseExistingFrontMatter removed (unused)

// (Future) Additional front matter transformations can compose here.
