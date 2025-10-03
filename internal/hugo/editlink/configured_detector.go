package editlink

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// ConfiguredDetector detects forge information from repository tags.
type ConfiguredDetector struct{}

// NewConfiguredDetector creates a new detector that uses repository tags.
func NewConfiguredDetector() *ConfiguredDetector {
	return &ConfiguredDetector{}
}

// Name returns the detector name.
func (d *ConfiguredDetector) Name() string {
	return "configured"
}

// Detect attempts to extract forge information from repository tags.
func (d *ConfiguredDetector) Detect(ctx DetectionContext) DetectionResult {
	if ctx.Repository == nil || ctx.Repository.Tags == nil {
		return DetectionResult{Found: false}
	}

	tags := ctx.Repository.Tags
	
	// Extract forge type from tags
	var forgeType config.ForgeType
	if t, ok := tags["forge_type"]; ok {
		forgeType = config.NormalizeForgeType(t)
	}

	// Extract full name from tags
	var fullName string
	if fn, ok := tags["full_name"]; ok && fn != "" {
		fullName = fn
	}

	// Extract base URL from forge configurations
	baseURL := ""
	if forgeType != "" && ctx.Config != nil {
		for _, forge := range ctx.Config.Forges {
			if forge != nil && forge.Type == forgeType {
				baseURL = forge.BaseURL
				break
			}
		}
	}

	// Only return success if we have both forge type and full name
	if forgeType != "" && fullName != "" {
		return DetectionResult{
			ForgeType: forgeType,
			BaseURL:   baseURL,
			FullName:  fullName,
			Found:     true,
		}
	}

	return DetectionResult{Found: false}
}