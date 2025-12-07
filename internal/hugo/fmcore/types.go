package fmcore

import (
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// MergeMode defines how a patch applies to existing front matter.
type MergeMode int

const (
	MergeDeep         MergeMode = iota // deep merge maps; arrays follow strategy
	MergeReplace                       // replace entire target keys
	MergeSetIfMissing                  // only set keys absent in base
)

// ArrayMergeStrategy controls how arrays are merged when both old and new are slices under Deep mode.
type ArrayMergeStrategy int

const (
	ArrayReplace ArrayMergeStrategy = iota
	ArrayUnion
	ArrayAppend
)

// FrontMatterPatch represents a unit of front matter changes from a transformer.
type FrontMatterPatch struct {
	Source        string
	Mode          MergeMode
	Priority      int                // higher applied later
	Data          map[string]any     // patch data
	ArrayStrategy ArrayMergeStrategy // optional override for all arrays in this patch (0 value = replace)
}

// ComputeBaseFrontMatter builds base front matter (title/date/repository/section/metadata) excluding edit link.
func ComputeBaseFrontMatter(name, repository, forge, section string, metadata, existing map[string]any, _ *config.Config, now time.Time) map[string]any {
	fm := map[string]any{}
	for k, v := range existing { // copy
		fm[k] = v
	}
	// Title (if missing and not index)
	if fm["title"] == nil && name != "index" {
		base := name
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
		fm["date"] = now.Format("2006-01-02T15:04:05-07:00")
	}
	// Repository & Section
	fm["repository"] = repository
	if forge != "" {
		fm["forge"] = forge
	}
	if section != "" {
		fm["section"] = section
	}
	// Metadata passthrough
	for k, v := range metadata {
		if fm[k] == nil {
			fm[k] = v
		}
	}
	return fm
}

// ComputeBaseFrontMatterTyped builds a typed FrontMatter struct (title/date/repository/section/metadata)
// excluding edit link. It preserves existing front matter values when present.
func ComputeBaseFrontMatterTyped(name, repository, forge, section string, metadata, existing map[string]any, _ *config.Config, now time.Time) *models.FrontMatter {
	// Seed from existing map to preserve any provided values
	fm, _ := models.FromMap(existing)

	// Title (if missing and not index)
	if fm.Title == "" && name != "index" {
		base := name
		base = strings.ReplaceAll(base, "_", "-")
		parts := strings.Split(base, "-")
		for i, part := range parts {
			if part == "" {
				continue
			}
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
		fm.Title = strings.Join(parts, " ")
	}

	// Date (inject if missing in existing)
	if _, hadDate := existing["date"]; !hadDate {
		fm.Date = now
	}

	// Repository & Section
	fm.Repository = repository
	if forge != "" {
		fm.Forge = forge
	}
	if section != "" {
		fm.Section = section
	}

	// Metadata passthrough (only when missing from existing)
	for k, v := range metadata {
		if _, had := existing[k]; !had {
			// Prefer known fields when applicable
			switch k {
			case "description":
				if s, ok := v.(string); ok {
					fm.Description = s
					continue
				}
			case "tags":
				if arr, ok := v.([]string); ok {
					fm.Tags = append(fm.Tags, arr...)
					continue
				}
			case "categories":
				if arr, ok := v.([]string); ok {
					fm.Categories = append(fm.Categories, arr...)
					continue
				}
			case "keywords":
				if arr, ok := v.([]string); ok {
					fm.Keywords = append(fm.Keywords, arr...)
					continue
				}
			}
			// Fallback to custom field storage
			fm.SetCustom(k, v)
		}
	}

	return fm
}

// ResolveEditLink reproduces per-page edit link generation (subset of original EditLinkResolver.Resolve)
// returning empty string when an edit link should not be generated.
// (Removed) ResolveEditLink – logic centralized in hugo.EditLinkResolver.
