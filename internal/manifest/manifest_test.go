package manifest

import (
	"encoding/json"
	"testing"
	"time"
)

func TestManifestSerialization(t *testing.T) {
	m := &BuildManifest{
		ID:        "build-123",
		TenantID:  "tenant-1",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Inputs: Inputs{
			Repos: []RepoInput{
				{Name: "repo1", URL: "https://github.com/org/repo1", Branch: "main", Commit: "abc123"},
				{Name: "repo2", URL: "https://github.com/org/repo2", Branch: "main", Commit: "def456"},
			},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{"frontmatter", "links"},
			Filters:    []string{"*.md"},
		},
		Outputs: Outputs{
			HugoConfigHash: "hugo-hash-456",
			ContentHash:    "content-hash-789",
			ArtifactHashes: map[string]string{
				"site.tar.gz": "artifact-hash-abc",
			},
		},
		Status:     "success",
		Duration:   5000,
		EventCount: 10,
	}

	// Test ToJSON
	jsonData, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("ToJSON returned empty data")
	}

	// Test FromJSON
	restored, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	// Verify key fields
	if restored.ID != m.ID {
		t.Errorf("expected ID %s, got %s", m.ID, restored.ID)
	}
	if restored.TenantID != m.TenantID {
		t.Errorf("expected TenantID %s, got %s", m.TenantID, restored.TenantID)
	}
	if len(restored.Inputs.Repos) != len(m.Inputs.Repos) {
		t.Errorf("expected %d repos, got %d", len(m.Inputs.Repos), len(restored.Inputs.Repos))
	}
	if restored.Plan.Theme != m.Plan.Theme {
		t.Errorf("expected theme %s, got %s", m.Plan.Theme, restored.Plan.Theme)
	}
	if restored.Status != m.Status {
		t.Errorf("expected status %s, got %s", m.Status, restored.Status)
	}
}

func TestManifestHash(t *testing.T) {
	m1 := &BuildManifest{
		ID:        "build-123",
		Timestamp: time.Now(),
		Inputs: Inputs{
			Repos: []RepoInput{
				{Name: "repo1", URL: "https://github.com/org/repo1", Branch: "main", Commit: "abc123"},
			},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{"frontmatter", "links"},
			Filters:    []string{"*.md"},
		},
	}

	m2 := &BuildManifest{
		ID:        "build-456",
		Timestamp: time.Now().Add(1 * time.Hour),
		Inputs: Inputs{
			Repos: []RepoInput{
				{Name: "repo1", URL: "https://github.com/org/repo1", Branch: "main", Commit: "abc123"},
			},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{"frontmatter", "links"},
			Filters:    []string{"*.md"},
		},
		Status:   "success",
		Duration: 1000,
	}

	hash1, err := m1.Hash()
	if err != nil {
		t.Fatalf("Hash failed for m1: %v", err)
	}

	hash2, err := m2.Hash()
	if err != nil {
		t.Fatalf("Hash failed for m2: %v", err)
	}

	// Same inputs and plan should produce same hash
	if hash1 != hash2 {
		t.Errorf("expected identical hashes for same inputs/plan, got %s and %s", hash1, hash2)
	}

	// Different inputs should produce different hash
	m3 := &BuildManifest{
		Inputs: Inputs{
			Repos: []RepoInput{
				{Name: "repo1", URL: "https://github.com/org/repo1", Branch: "main", Commit: "xyz789"},
			},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{"frontmatter", "links"},
			Filters:    []string{"*.md"},
		},
	}

	hash3, err := m3.Hash()
	if err != nil {
		t.Fatalf("Hash failed for m3: %v", err)
	}

	if hash1 == hash3 {
		t.Error("expected different hashes for different commits")
	}
}

func TestManifestHashConsistency(t *testing.T) {
	m := &BuildManifest{
		Inputs: Inputs{
			Repos: []RepoInput{
				{Name: "repo1", URL: "https://github.com/org/repo1", Branch: "main", Commit: "abc123"},
				{Name: "repo2", URL: "https://github.com/org/repo2", Branch: "main", Commit: "def456"},
			},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{"frontmatter", "links"},
			Filters:    []string{"*.md"},
		},
	}

	// Hash multiple times should produce same result
	hash1, _ := m.Hash()
	hash2, _ := m.Hash()
	hash3, _ := m.Hash()

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("hash not consistent: %s, %s, %s", hash1, hash2, hash3)
	}

	// Hash should be hex string
	if len(hash1) != 64 {
		t.Errorf("expected 64-char hex string, got %d chars: %s", len(hash1), hash1)
	}
}

func TestManifestJSONStructure(t *testing.T) {
	m := &BuildManifest{
		ID:        "build-123",
		TenantID:  "tenant-1",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Inputs: Inputs{
			Repos: []RepoInput{
				{Name: "repo1", URL: "https://github.com/org/repo1", Branch: "main", Commit: "abc123"},
			},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:         "hextra",
			ThemeFeatures: map[string]interface{}{"uses_modules": true},
			Transforms:    []string{"frontmatter"},
		},
		Outputs: Outputs{
			HugoConfigHash: "hugo-hash-456",
		},
		Status:     "success",
		Duration:   5000,
		EventCount: 10,
	}

	jsonData, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Parse JSON to verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Check required fields exist
	requiredFields := []string{"id", "timestamp", "inputs", "plan", "outputs", "status", "duration_ms", "event_count"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Check nested structures
	if inputs, ok := parsed["inputs"].(map[string]interface{}); ok {
		if _, ok := inputs["repos"]; !ok {
			t.Error("missing inputs.repos")
		}
		if _, ok := inputs["config_hash"]; !ok {
			t.Error("missing inputs.config_hash")
		}
	} else {
		t.Error("inputs is not an object")
	}

	if plan, ok := parsed["plan"].(map[string]interface{}); ok {
		if _, ok := plan["theme"]; !ok {
			t.Error("missing plan.theme")
		}
		if _, ok := plan["transforms"]; !ok {
			t.Error("missing plan.transforms")
		}
	} else {
		t.Error("plan is not an object")
	}
}

// TestManifestPluginVersions tests plugin version tracking in manifest.
func TestManifestPluginVersions(t *testing.T) {
	m := &BuildManifest{
		ID:        "build-456",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Inputs: Inputs{
			Repos: []RepoInput{
				{Name: "repo1", URL: "https://github.com/org/repo1", Branch: "main", Commit: "abc123"},
			},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{"frontmatter", "links"},
			Filters:    []string{"*.md"},
		},
		Plugins: Plugins{
			Theme: &PluginVersion{
				Name:    "hextra",
				Version: "v1.0.0",
				Type:    "theme",
			},
			Transforms: []PluginVersion{
				{
					Name:    "frontmatter",
					Version: "v1.0.0",
					Type:    "transform",
				},
				{
					Name:    "links",
					Version: "v1.0.1",
					Type:    "transform",
				},
			},
		},
		Outputs: Outputs{
			HugoConfigHash: "hugo-hash-456",
		},
		Status:     "success",
		Duration:   5000,
		EventCount: 10,
	}

	// Verify plugin versions are stored correctly
	if m.Plugins.Theme == nil {
		t.Error("Theme plugin should not be nil")
	} else {
		if m.Plugins.Theme.Name != "hextra" {
			t.Errorf("Theme plugin name should be 'hextra', got %s", m.Plugins.Theme.Name)
		}
		if m.Plugins.Theme.Version != "v1.0.0" {
			t.Errorf("Theme plugin version should be 'v1.0.0', got %s", m.Plugins.Theme.Version)
		}
	}

	if len(m.Plugins.Transforms) != 2 {
		t.Errorf("Should have 2 transforms, got %d", len(m.Plugins.Transforms))
	}

	// Test serialization with plugins
	jsonData, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Parse and verify plugins are in JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if _, ok := parsed["plugins"]; !ok {
		t.Error("missing plugins field in JSON")
	}

	// Test deserialization
	restored, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if restored.Plugins.Theme == nil {
		t.Error("Restored manifest should have theme plugin")
	} else if restored.Plugins.Theme.Version != "v1.0.0" {
		t.Errorf("Restored theme version should be 'v1.0.0', got %s", restored.Plugins.Theme.Version)
	}

	if len(restored.Plugins.Transforms) != 2 {
		t.Errorf("Restored manifest should have 2 transforms, got %d", len(restored.Plugins.Transforms))
	}
}

// TestManifestHashIncludesPlugins tests that plugin versions affect manifest hash.
func TestManifestHashIncludesPlugins(t *testing.T) {
	baseManifest := &BuildManifest{
		ID:        "build-789",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Inputs: Inputs{
			Repos: []RepoInput{
				{Name: "repo1", URL: "https://github.com/org/repo1", Branch: "main", Commit: "abc123"},
			},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{"frontmatter"},
		},
		Outputs: Outputs{
			HugoConfigHash: "hugo-hash-456",
		},
		Status:     "success",
		Duration:   5000,
		EventCount: 10,
	}

	// Calculate hash without plugins
	hash1, err := baseManifest.Hash()
	if err != nil {
		t.Fatalf("Hash() failed: %v", err)
	}

	// Add plugin versions
	baseManifest.Plugins = Plugins{
		Theme: &PluginVersion{
			Name:    "hextra",
			Version: "v1.0.0",
			Type:    "theme",
		},
	}

	hash2, err := baseManifest.Hash()
	if err != nil {
		t.Fatalf("Hash() failed after adding plugins: %v", err)
	}

	// Hashes should differ because plugins were added
	if hash1 == hash2 {
		t.Error("Hash should change when plugins are added")
	}

	// Change plugin version
	baseManifest.Plugins.Theme.Version = "v1.0.1"
	hash3, err := baseManifest.Hash()
	if err != nil {
		t.Fatalf("Hash() failed after changing plugin version: %v", err)
	}

	// Hash should differ for different plugin versions
	if hash2 == hash3 {
		t.Error("Hash should change when plugin version changes")
	}
}

// TestManifestEmptyPlugins tests manifest with no plugins specified.
func TestManifestEmptyPlugins(t *testing.T) {
	m := &BuildManifest{
		ID:        "build-empty-plugins",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Inputs: Inputs{
			Repos:      []RepoInput{},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{},
		},
		Plugins: Plugins{}, // Empty plugins
		Outputs: Outputs{
			HugoConfigHash: "hugo-hash-456",
		},
		Status:     "success",
		Duration:   1000,
		EventCount: 1,
	}

	// Should serialize successfully with empty plugins
	jsonData, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed with empty plugins: %v", err)
	}

	// Should deserialize successfully
	restored, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if restored.Plugins.Theme != nil {
		t.Error("Restored empty plugins should have nil theme")
	}

	if len(restored.Plugins.Transforms) != 0 {
		t.Error("Restored empty plugins should have no transforms")
	}
}

// TestManifestMultiplePluginTypes tests manifest with all plugin types.
func TestManifestMultiplePluginTypes(t *testing.T) {
	m := &BuildManifest{
		ID:        "build-all-plugins",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Inputs: Inputs{
			Repos:      []RepoInput{},
			ConfigHash: "config-hash-123",
		},
		Plan: Plan{
			Theme:      "hextra",
			Transforms: []string{"frontmatter"},
		},
		Plugins: Plugins{
			Theme: &PluginVersion{
				Name:    "hextra",
				Version: "v1.0.0",
				Type:    "theme",
			},
			Transforms: []PluginVersion{
				{Name: "frontmatter", Version: "v1.0.0", Type: "transform"},
			},
			Forges: []PluginVersion{
				{Name: "github", Version: "v1.0.0", Type: "forge"},
			},
			Publishers: []PluginVersion{
				{Name: "s3", Version: "v1.0.0", Type: "publisher"},
			},
		},
		Outputs: Outputs{
			HugoConfigHash: "hugo-hash-456",
		},
		Status:     "success",
		Duration:   1000,
		EventCount: 1,
	}

	// Verify all plugin types are stored
	if m.Plugins.Theme.Name != "hextra" {
		t.Error("Theme plugin missing")
	}
	if len(m.Plugins.Transforms) != 1 {
		t.Error("Transforms missing")
	}
	if len(m.Plugins.Forges) != 1 {
		t.Error("Forges missing")
	}
	if len(m.Plugins.Publishers) != 1 {
		t.Error("Publishers missing")
	}

	// Test serialization
	jsonData, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Verify JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	plugins, ok := parsed["plugins"].(map[string]interface{})
	if !ok {
		t.Fatal("plugins field is not an object")
	}

	// Verify each plugin type is in JSON
	if _, ok := plugins["theme"]; !ok {
		t.Error("missing theme in plugins JSON")
	}
	if _, ok := plugins["transforms"]; !ok {
		t.Error("missing transforms in plugins JSON")
	}
	if _, ok := plugins["forges"]; !ok {
		t.Error("missing forges in plugins JSON")
	}
	if _, ok := plugins["publishers"]; !ok {
		t.Error("missing publishers in plugins JSON")
	}
}

// TestPluginsPopulateFromRegistry tests populating plugins from a registry.
func TestPluginsPopulateFromRegistry(t *testing.T) {
	// Create a mock registry
	mockRegistry := &mockPluginRegistry{
		plugins: make(map[string][]interface{}),
	}

	// Add theme plugin
	mockRegistry.plugins["theme"] = []interface{}{
		&mockPlugin{
			name:    "hextra",
			version: "v1.0.0",
			ptype:   "theme",
		},
	}

	// Add transform plugins
	mockRegistry.plugins["transform"] = []interface{}{
		&mockPlugin{
			name:    "frontmatter",
			version: "v1.0.0",
			ptype:   "transform",
		},
		&mockPlugin{
			name:    "links",
			version: "v1.0.1",
			ptype:   "transform",
		},
	}

	// Populate from registry
	plugins := &Plugins{}
	err := plugins.PopulateFrom(mockRegistry)
	if err != nil {
		t.Fatalf("PopulateFrom failed: %v", err)
	}

	// Verify theme was populated
	if plugins.Theme == nil {
		t.Error("Theme plugin should be populated")
	} else if plugins.Theme.Name != "hextra" {
		t.Errorf("Theme name should be 'hextra', got %s", plugins.Theme.Name)
	}

	// Verify transforms were populated
	if len(plugins.Transforms) != 2 {
		t.Errorf("Should have 2 transforms, got %d", len(plugins.Transforms))
	}
	if len(plugins.Transforms) > 0 && plugins.Transforms[0].Name != "frontmatter" {
		t.Errorf("First transform should be 'frontmatter', got %s", plugins.Transforms[0].Name)
	}
}

// mockPlugin is a test plugin for registry tests
type mockPlugin struct {
	name    string
	version string
	ptype   string
}

func (m *mockPlugin) Metadata() map[string]string {
	return map[string]string{
		"name":    m.name,
		"version": m.version,
		"type":    m.ptype,
	}
}

// mockPluginRegistry is a test registry implementation
type mockPluginRegistry struct {
	plugins map[string][]interface{}
}

func (r *mockPluginRegistry) ListByType(pluginType string) []interface{} {
	return r.plugins[pluginType]
}
