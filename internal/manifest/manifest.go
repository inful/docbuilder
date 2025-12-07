package manifest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// PopulatePluginsFromRegistry is a helper to populate plugin information from a plugin registry.
// It accepts any registry-like object that supports listing plugins by type.
// This allows the manifest package to remain independent of the plugin package.
type PluginRegistry interface {
	// ListByType returns all plugins of a specific type.
	ListByType(pluginType string) []interface{}
}

// PopulatePlugins populates the Plugins struct from a plugin registry.
// The registry must support ListByType("theme"), ListByType("transform"), etc.
// Each plugin must have Name, Version, and Type() methods.
func (p *Plugins) PopulateFrom(registry PluginRegistry) error {
	if registry == nil {
		return nil // No registry provided, skip population
	}

	// Helper to extract plugin metadata from interface{}
	extractPlugin := func(obj interface{}) *PluginVersion {
		// Try to access Name and Version properties
		// This is a generic approach that avoids direct dependency on plugin package
		type versionedPlugin interface {
			Name() string
			Version() string
			PluginType() string
		}

		if vp, ok := obj.(versionedPlugin); ok {
			return &PluginVersion{
				Name:    vp.Name(),
				Version: vp.Version(),
				Type:    vp.PluginType(),
			}
		}

		// Fallback for plugins with different interface
		type simplePlugin interface {
			Metadata() map[string]string
		}

		if sp, ok := obj.(simplePlugin); ok {
			meta := sp.Metadata()
			if name, ok := meta["name"]; ok {
				return &PluginVersion{
					Name:    name,
					Version: meta["version"],
					Type:    meta["type"],
				}
			}
		}

		return nil
	}

	// Populate theme plugin
	if themes := registry.ListByType("theme"); len(themes) > 0 {
		if pv := extractPlugin(themes[0]); pv != nil {
			p.Theme = pv
		}
	}

	// Populate transforms
	if transforms := registry.ListByType("transform"); len(transforms) > 0 {
		p.Transforms = make([]PluginVersion, 0, len(transforms))
		for _, t := range transforms {
			if pv := extractPlugin(t); pv != nil {
				p.Transforms = append(p.Transforms, *pv)
			}
		}
	}

	// Populate forges
	if forges := registry.ListByType("forge"); len(forges) > 0 {
		p.Forges = make([]PluginVersion, 0, len(forges))
		for _, f := range forges {
			if pv := extractPlugin(f); pv != nil {
				p.Forges = append(p.Forges, *pv)
			}
		}
	}

	// Populate publishers
	if publishers := registry.ListByType("publisher"); len(publishers) > 0 {
		p.Publishers = make([]PluginVersion, 0, len(publishers))
		for _, pb := range publishers {
			if pv := extractPlugin(pb); pv != nil {
				p.Publishers = append(p.Publishers, *pv)
			}
		}
	}

	return nil
}

// BuildManifest represents a complete record of a build's inputs, plan, and outputs.
type BuildManifest struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Inputs     Inputs    `json:"inputs"`
	Plan       Plan      `json:"plan"`
	Plugins    Plugins   `json:"plugins"`
	Outputs    Outputs   `json:"outputs"`
	Status     string    `json:"status"`
	Duration   int64     `json:"duration_ms"`
	EventCount int       `json:"event_count"`
}

// Inputs captures all inputs to the build.
type Inputs struct {
	Repos      []RepoInput `json:"repos"`
	ConfigHash string      `json:"config_hash"`
}

// RepoInput represents a repository input.
type RepoInput struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	Hash   string `json:"hash,omitempty"` // content hash
}

// Plan captures the build execution plan.
type Plan struct {
	Theme         string                 `json:"theme"`
	ThemeFeatures map[string]interface{} `json:"theme_features,omitempty"`
	Transforms    []string               `json:"transforms"`
	Filters       []string               `json:"filters,omitempty"`
}

// PluginVersion represents a versioned plugin used during a build.
type PluginVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
}

// Plugins captures all plugins used during the build.
type Plugins struct {
	Theme      *PluginVersion  `json:"theme,omitempty"`
	Transforms []PluginVersion `json:"transforms,omitempty"`
	Forges     []PluginVersion `json:"forges,omitempty"`
	Publishers []PluginVersion `json:"publishers,omitempty"`
}

// Outputs captures all outputs from the build.
type Outputs struct {
	HugoConfigHash string            `json:"hugo_config_hash"`
	ContentHash    string            `json:"content_hash,omitempty"`
	ArtifactHashes map[string]string `json:"artifact_hashes,omitempty"`
}

// ToJSON serializes the manifest to JSON.
func (m *BuildManifest) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	return data, nil
}

// FromJSON deserializes a manifest from JSON.
func FromJSON(data []byte) (*BuildManifest, error) {
	var m BuildManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	return &m, nil
}

// Hash computes a deterministic hash of the manifest's inputs, plan, and plugins.
// This can be used to detect if a build with identical inputs and plugins has been run before.
func (m *BuildManifest) Hash() (string, error) {
	// Create a normalized representation for hashing
	hashInput := struct {
		Repos      []RepoInput `json:"repos"`
		ConfigHash string      `json:"config_hash"`
		Theme      string      `json:"theme"`
		Transforms []string    `json:"transforms"`
		Filters    []string    `json:"filters"`
		Plugins    Plugins     `json:"plugins"`
	}{
		Repos:      m.Inputs.Repos,
		ConfigHash: m.Inputs.ConfigHash,
		Theme:      m.Plan.Theme,
		Transforms: m.Plan.Transforms,
		Filters:    m.Plan.Filters,
		Plugins:    m.Plugins,
	}

	data, err := json.Marshal(hashInput)
	if err != nil {
		return "", fmt.Errorf("marshal for hash: %w", err)
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}
