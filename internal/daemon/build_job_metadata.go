package daemon

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// BuildJobMetadata represents typed metadata for build jobs
type BuildJobMetadata struct {
	// Core build configuration
	V2Config     *config.Config      `json:"v2_config,omitempty"`
	Repositories []config.Repository `json:"repositories,omitempty"`

	// State management
	StateManager services.StateManager `json:"-"` // Interface, don't serialize

	// Delta analysis
	DeltaRepoReasons map[string]string `json:"delta_repo_reasons,omitempty"`

	// Metrics and monitoring
	MetricsCollector *MetricsCollector `json:"-"` // Pointer to live collector

	// Live reload
	LiveReloadHub *LiveReloadHub `json:"-"` // Pointer to live hub

	// Build report (populated after completion)
	BuildReport *hugo.BuildReport `json:"build_report,omitempty"`
}

// ToMap converts the structured metadata back to a map for backward compatibility
func (m *BuildJobMetadata) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if m.V2Config != nil {
		result["v2_config"] = m.V2Config
	}
	if m.Repositories != nil {
		result["repositories"] = m.Repositories
	}
	if m.StateManager != nil {
		result["state_manager"] = m.StateManager
	}
	if m.DeltaRepoReasons != nil {
		result["delta_repo_reasons"] = m.DeltaRepoReasons
	}
	if m.MetricsCollector != nil {
		result["metrics_collector"] = m.MetricsCollector
	}
	if m.LiveReloadHub != nil {
		result["live_reload_hub"] = m.LiveReloadHub
	}
	if m.BuildReport != nil {
		result["build_report"] = m.BuildReport
	}

	return result
}

// FromMap populates the structured metadata from a map for backward compatibility
func (m *BuildJobMetadata) FromMap(data map[string]interface{}) {
	if v2Config, ok := data["v2_config"].(*config.Config); ok {
		m.V2Config = v2Config
	}
	if repositories, ok := data["repositories"].([]config.Repository); ok {
		m.Repositories = repositories
	}
	if stateManager, ok := data["state_manager"].(services.StateManager); ok {
		m.StateManager = stateManager
	}
	if deltaReasons, ok := data["delta_repo_reasons"].(map[string]string); ok {
		m.DeltaRepoReasons = deltaReasons
	}
	if metricsCollector, ok := data["metrics_collector"].(*MetricsCollector); ok {
		m.MetricsCollector = metricsCollector
	}
	if liveReloadHub, ok := data["live_reload_hub"].(*LiveReloadHub); ok {
		m.LiveReloadHub = liveReloadHub
	}
	if buildReport, ok := data["build_report"].(*hugo.BuildReport); ok {
		m.BuildReport = buildReport
	}
}
