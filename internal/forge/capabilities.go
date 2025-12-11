package forge

import "git.home.luguber.info/inful/docbuilder/internal/config"

// Capabilities describes feature flags per forge implementation.
// Extended cautiously; adding a field requires updating capability golden test (to be added).
type Capabilities struct {
	SupportsEditLinks bool
	SupportsWebhooks  bool
	SupportsTopics    bool
}

// capabilities holds the canonical capability map keyed by normalized forge type.
var capabilities = map[config.ForgeType]Capabilities{
	config.ForgeGitHub:  {SupportsEditLinks: true, SupportsWebhooks: true, SupportsTopics: true},
	config.ForgeGitLab:  {SupportsEditLinks: true, SupportsWebhooks: true, SupportsTopics: true},
	config.ForgeForgejo: {SupportsEditLinks: true, SupportsWebhooks: true},
	// Additional forges added here.
}

// GetCapabilities returns capability flags for the given forge type.
func GetCapabilities(t config.ForgeType) Capabilities { return capabilities[t] }
