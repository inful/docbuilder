package forge

import "git.home.luguber.info/inful/docbuilder/internal/config"

// ForgeCapabilities describes feature flags per forge implementation.
// Extended cautiously; adding a field requires updating capability golden test (to be added).
type ForgeCapabilities struct {
	SupportsEditLinks bool
	SupportsWebhooks  bool
	SupportsTopics    bool
}

// caps holds the canonical capability map keyed by normalized forge type.
var caps = map[config.ForgeType]ForgeCapabilities{
	config.ForgeGitHub:  {SupportsEditLinks: true, SupportsWebhooks: true, SupportsTopics: true},
	config.ForgeGitLab:  {SupportsEditLinks: true, SupportsWebhooks: true, SupportsTopics: true},
	config.ForgeForgejo: {SupportsEditLinks: true, SupportsWebhooks: true},
	// Additional forges added here.
}

// Capabilities returns capability flags for the given forge type.
func Capabilities(t config.ForgeType) ForgeCapabilities { return caps[t] }
