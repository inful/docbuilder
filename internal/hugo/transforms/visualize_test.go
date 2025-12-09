package transforms

import (
	"strings"
	"testing"
)

func TestVisualizePipeline_Text(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)
	
	// Clear and populate registry
	registry = make(map[string]Transformer)
	
	Register(validTestTransform{
		name:  "parser",
		stage: StageParse,
		deps:  TransformDependencies{},
	})
	
	Register(validTestTransform{
		name:  "builder",
		stage: StageBuild,
		deps:  TransformDependencies{MustRunAfter: []string{"parser"}},
	})
	
	output, err := VisualizePipeline(FormatText)
	if err != nil {
		t.Fatalf("VisualizePipeline(FormatText) error = %v", err)
	}
	
	// Check for key elements
	expectedStrings := []string{
		"Transform Pipeline Visualization",
		"Stage",
		"parser",
		"builder",
		"depends on",
		"Total:",
	}
	
	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("VisualizePipeline(FormatText) output missing %q", expected)
		}
	}
}

func TestVisualizePipeline_Mermaid(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)
	
	// Clear and populate registry
	registry = make(map[string]Transformer)
	
	Register(validTestTransform{
		name:  "transform_a",
		stage: StageTransform,
		deps:  TransformDependencies{},
	})
	
	Register(validTestTransform{
		name:  "transform_b",
		stage: StageTransform,
		deps:  TransformDependencies{MustRunAfter: []string{"transform_a"}},
	})
	
	output, err := VisualizePipeline(FormatMermaid)
	if err != nil {
		t.Fatalf("VisualizePipeline(FormatMermaid) error = %v", err)
	}
	
	// Check for Mermaid syntax
	expectedStrings := []string{
		"```mermaid",
		"graph TD",
		"subgraph",
		"transform",
		"-->",
		"```",
	}
	
	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("VisualizePipeline(FormatMermaid) output missing %q", expected)
		}
	}
}

func TestVisualizePipeline_DOT(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)
	
	// Clear and populate registry
	registry = make(map[string]Transformer)
	
	Register(validTestTransform{
		name:  "enricher",
		stage: StageEnrich,
		deps:  TransformDependencies{},
	})
	
	output, err := VisualizePipeline(FormatDOT)
	if err != nil {
		t.Fatalf("VisualizePipeline(FormatDOT) error = %v", err)
	}
	
	// Check for DOT syntax
	expectedStrings := []string{
		"digraph TransformPipeline",
		"rankdir=TB",
		"node [shape=box",
		"subgraph cluster_",
		"label=",
		"enricher",
	}
	
	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("VisualizePipeline(FormatDOT) output missing %q", expected)
		}
	}
}

func TestVisualizePipeline_JSON(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)
	
	// Clear and populate registry
	registry = make(map[string]Transformer)
	
	Register(validTestTransform{
		name:  "serializer",
		stage: StageSerialize,
		deps:  TransformDependencies{MustRunBefore: []string{"finalizer"}},
	})
	
	Register(validTestTransform{
		name:  "finalizer",
		stage: StageFinalize,
		deps:  TransformDependencies{},
	})
	
	output, err := VisualizePipeline(FormatJSON)
	if err != nil {
		t.Fatalf("VisualizePipeline(FormatJSON) error = %v", err)
	}
	
	// Check for JSON structure
	expectedStrings := []string{
		`"transforms"`,
		`"name"`,
		`"stage"`,
		`"order"`,
		`"dependencies"`,
		`"mustRunAfter"`,
		`"mustRunBefore"`,
		`"totalTransforms"`,
		`"totalStages"`,
		"serializer",
		"finalizer",
	}
	
	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("VisualizePipeline(FormatJSON) output missing %q", expected)
		}
	}
}

func TestVisualizePipeline_UnsupportedFormat(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)
	
	registry = make(map[string]Transformer)
	Register(validTestTransform{name: "test", stage: StageParse})
	
	_, err := VisualizePipeline(VisualizationFormat("unsupported"))
	if err == nil {
		t.Error("VisualizePipeline() should error for unsupported format")
	}
	
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("VisualizePipeline() error = %v, want unsupported format error", err)
	}
}

func TestVisualizePipeline_EmptyRegistry(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)
	
	// Clear registry
	registry = make(map[string]Transformer)
	
	output, err := VisualizePipeline(FormatText)
	if err != nil {
		t.Fatalf("VisualizePipeline(FormatText) with empty registry error = %v", err)
	}
	
	if !strings.Contains(output, "Total: 0") {
		t.Error("VisualizePipeline() should handle empty registry")
	}
}

func TestVisualizePipeline_ComplexDependencies(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)
	
	// Create a more complex pipeline
	registry = make(map[string]Transformer)
	
	Register(validTestTransform{
		name:  "a",
		stage: StageParse,
		deps:  TransformDependencies{},
	})
	
	Register(validTestTransform{
		name:  "b",
		stage: StageBuild,
		deps:  TransformDependencies{MustRunAfter: []string{"a"}},
	})
	
	Register(validTestTransform{
		name:  "c",
		stage: StageBuild,
		deps:  TransformDependencies{MustRunAfter: []string{"a"}},
	})
	
	Register(validTestTransform{
		name:  "d",
		stage: StageEnrich,
		deps:  TransformDependencies{MustRunAfter: []string{"b", "c"}},
	})
	
	// Test all formats work with complex dependencies
	formats := []VisualizationFormat{FormatText, FormatMermaid, FormatDOT, FormatJSON}
	
	for _, format := range formats {
		t.Run(string(format), func(t *testing.T) {
			output, err := VisualizePipeline(format)
			if err != nil {
				t.Fatalf("VisualizePipeline(%s) error = %v", format, err)
			}
			
			// All formats should mention all transforms
			for _, name := range []string{"a", "b", "c", "d"} {
				if !strings.Contains(output, name) {
					t.Errorf("VisualizePipeline(%s) missing transform %q", format, name)
				}
			}
		})
	}
}

func TestGetSupportedFormats(t *testing.T) {
	formats := GetSupportedFormats()
	
	if len(formats) != 4 {
		t.Errorf("GetSupportedFormats() returned %d formats, want 4", len(formats))
	}
	
	expectedFormats := map[VisualizationFormat]bool{
		FormatText:    true,
		FormatMermaid: true,
		FormatDOT:     true,
		FormatJSON:    true,
	}
	
	for _, format := range formats {
		if !expectedFormats[format] {
			t.Errorf("GetSupportedFormats() returned unexpected format: %s", format)
		}
	}
}

func TestGetFormatDescription(t *testing.T) {
	tests := []struct {
		format VisualizationFormat
		want   string
	}{
		{FormatText, "Human-readable"},
		{FormatMermaid, "Mermaid"},
		{FormatDOT, "Graphviz"},
		{FormatJSON, "JSON"},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			desc := GetFormatDescription(tt.format)
			if desc == "" {
				t.Error("GetFormatDescription() returned empty string")
			}
			if !strings.Contains(desc, tt.want) {
				t.Errorf("GetFormatDescription(%s) = %q, want to contain %q", tt.format, desc, tt.want)
			}
		})
	}
}

func TestVisualizePipeline_MustRunBefore(t *testing.T) {
	// Test that MustRunBefore dependencies are properly visualized
	saved := snapshotRegistry()
	defer restoreRegistry(saved)
	
	registry = make(map[string]Transformer)
	
	Register(validTestTransform{
		name:  "early",
		stage: StageParse,
		deps:  TransformDependencies{MustRunBefore: []string{"late"}},
	})
	
	Register(validTestTransform{
		name:  "late",
		stage: StageBuild,
		deps:  TransformDependencies{},
	})
	
	// Test text format shows MustRunBefore
	textOutput, err := VisualizePipeline(FormatText)
	if err != nil {
		t.Fatalf("VisualizePipeline(FormatText) error = %v", err)
	}
	
	if !strings.Contains(textOutput, "required before") {
		t.Error("Text visualization should show 'required before' for MustRunBefore dependencies")
	}
	
	// Test DOT format includes edge
	dotOutput, err := VisualizePipeline(FormatDOT)
	if err != nil {
		t.Fatalf("VisualizePipeline(FormatDOT) error = %v", err)
	}
	
	if !strings.Contains(dotOutput, "early") || !strings.Contains(dotOutput, "late") {
		t.Error("DOT visualization should include both transforms in MustRunBefore relationship")
	}
}
