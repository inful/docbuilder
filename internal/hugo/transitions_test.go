package hugo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestViewTransitions_Disabled verifies that transition assets are not copied when disabled
func TestViewTransitions_Disabled(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.EnableTransitions = false

	gen := NewGenerator(cfg, tmp)
	if err := gen.beginStaging(); err != nil {
		t.Fatalf("beginStaging: %v", err)
	}

	if err := gen.copyTransitionAssets(); err != nil {
		t.Fatalf("copyTransitionAssets: %v", err)
	}

	// Verify files were NOT created
	cssPath := filepath.Join(gen.buildRoot(), "static", "view-transitions.css")
	if _, err := os.Stat(cssPath); err == nil {
		t.Error("CSS file should not exist when transitions disabled")
	}

	jsPath := filepath.Join(gen.buildRoot(), "static", "view-transitions.js")
	if _, err := os.Stat(jsPath); err == nil {
		t.Error("JS file should not exist when transitions disabled")
	}
}

// TestViewTransitions_Enabled verifies that transition assets are copied when enabled
func TestViewTransitions_Enabled(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.EnableTransitions = true
	cfg.Hugo.TransitionDuration = "500ms"

	gen := NewGenerator(cfg, tmp)
	if err := gen.beginStaging(); err != nil {
		t.Fatalf("beginStaging: %v", err)
	}

	if err := gen.createHugoStructure(); err != nil {
		t.Fatalf("createHugoStructure: %v", err)
	}

	if err := gen.copyTransitionAssets(); err != nil {
		t.Fatalf("copyTransitionAssets: %v", err)
	}

	// Verify CSS file was created
	cssPath := filepath.Join(gen.buildRoot(), "static", "view-transitions.css")
	if _, err := os.Stat(cssPath); err != nil {
		t.Errorf("CSS file should exist: %v", err)
	}

	// Verify file contents are not empty
	cssData, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("read CSS: %v", err)
	}
	if len(cssData) == 0 {
		t.Error("CSS file is empty")
	}

	// Verify CSS contains @view-transition rule
	if !bytes.Contains(cssData, []byte("@view-transition")) {
		t.Error("CSS should contain @view-transition rule")
	}
}

// TestViewTransitions_ConfigParams verifies Hugo config params are set correctly
func TestViewTransitions_ConfigParams(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.Title = "Test Site"
	cfg.Hugo.EnableTransitions = true
	cfg.Hugo.TransitionDuration = "400ms"

	gen := NewGenerator(cfg, tmp)
	if err := gen.beginStaging(); err != nil {
		t.Fatalf("beginStaging: %v", err)
	}

	if err := gen.createHugoStructure(); err != nil {
		t.Fatalf("createHugoStructure: %v", err)
	}

	// Generate the Hugo config
	if err := gen.generateHugoConfig(); err != nil {
		t.Fatalf("generateHugoConfig: %v", err)
	}

	// Read and verify the config file
	configPath := filepath.Join(gen.buildRoot(), "hugo.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read hugo.yaml: %v", err)
	}

	configStr := string(data)

	// Check for enable_transitions param
	if !strings.Contains(configStr, "enable_transitions: true") {
		t.Error("Config should contain enable_transitions: true")
	}

	// Check for transition_duration param
	if !strings.Contains(configStr, "transition_duration: 400ms") {
		t.Error("Config should contain transition_duration: 400ms")
	}
}

// TestViewTransitions_Integration verifies the full pipeline with transitions
func TestViewTransitions_Integration(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.Title = "Transitions Test"
	cfg.Hugo.EnableTransitions = true
	cfg.Hugo.TransitionDuration = "350ms"

	gen := NewGenerator(cfg, tmp)

	// Create a simple doc file with proper structure
	docFiles := []docs.DocFile{
		{
			Path:         filepath.Join(tmp, "src", "test.md"),
			RelativePath: "test.md",
			Repository:   "test-repo",
			Name:         "test",
			Extension:    ".md",
			Content:      []byte("# Test\n\nTest content\n"),
		},
	}

	// Generate the site
	if err := gen.GenerateSite(docFiles); err != nil {
		t.Fatalf("GenerateSite: %v", err)
	}

	// Verify CSS exists in output
	cssPath := filepath.Join(tmp, "static", "view-transitions.css")
	if _, err := os.Stat(cssPath); err != nil {
		t.Errorf("CSS file should exist in output: %v", err)
	}

	// Verify the head partial exists (contains CSS link only, no JS)
	partialPath := filepath.Join(tmp, "layouts", "_partials", "custom", "head-end.html")
	if _, err := os.Stat(partialPath); err != nil {
		t.Errorf("Head partial should exist: %v", err)
	}

	// Verify JS does NOT exist (CSS-only approach)
	jsPath := filepath.Join(tmp, "static", "view-transitions.js")
	if _, err := os.Stat(jsPath); err == nil {
		t.Error("JS file should NOT exist (CSS-only approach)")
	}
}
