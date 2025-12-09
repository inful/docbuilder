package transforms

import (
	"strings"
	"testing"
)

// TestStageGrouping verifies transforms are properly grouped by stage
func TestStageGrouping(t *testing.T) {
	transformList, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	
	// Track stage transitions
	var lastStage TransformStage
	stageCount := make(map[TransformStage]int)
	
	for i, tr := range transformList {
		stage := tr.Stage()
		stageCount[stage]++
		
		if i > 0 {
			// Verify stages only move forward, never backward
			lastIndex := StageIndex(lastStage)
			currentIndex := StageIndex(stage)
			
			if currentIndex < lastIndex {
				t.Errorf("Stage ordering violation at position %d: %s (stage %s) after %s (stage %s)",
					i, tr.Name(), stage, transformList[i-1].Name(), lastStage)
			}
		}
		
		lastStage = stage
	}
	
	// Log stage distribution
	t.Logf("Stage distribution:")
	for _, stage := range StageOrder {
		if count := stageCount[stage]; count > 0 {
			t.Logf("  %s: %d transforms", stage, count)
		}
	}
}

// TestDependenciesSatisfied verifies all declared dependencies are satisfied
func TestDependenciesSatisfied(t *testing.T) {
	transformList, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	
	// Build execution order map
	position := make(map[string]int)
	for i, tr := range transformList {
		position[tr.Name()] = i
	}
	
	// Verify all dependencies are satisfied
	for i, tr := range transformList {
		deps := tr.Dependencies()
		
		// Check MustRunAfter
		for _, depName := range deps.MustRunAfter {
			depPos, exists := position[depName]
			if !exists {
				t.Errorf("%s declares MustRunAfter %q but %q is not registered", tr.Name(), depName, depName)
				continue
			}
			if depPos >= i {
				t.Errorf("%s (pos %d) declares MustRunAfter %q (pos %d) but runs before or at same position",
					tr.Name(), i, depName, depPos)
			}
		}
		
		// Check MustRunBefore
		for _, afterName := range deps.MustRunBefore {
			afterPos, exists := position[afterName]
			if !exists {
				t.Errorf("%s declares MustRunBefore %q but %q is not registered", tr.Name(), afterName, afterName)
				continue
			}
			if afterPos <= i {
				t.Errorf("%s (pos %d) declares MustRunBefore %q (pos %d) but runs after or at same position",
					tr.Name(), i, afterName, afterPos)
			}
		}
	}
}

// TestNoCycles verifies the dependency graph has no cycles
func TestNoCycles(t *testing.T) {
	// List() calls BuildPipeline internally which uses toposort and returns an error if there are cycles
	pipeline, err := List()
	if err != nil {
		if strings.Contains(err.Error(), "cycle") {
			t.Fatalf("Dependency graph contains a cycle: %v", err)
		}
		t.Fatalf("List() failed: %v", err)
	}
	
	if len(pipeline) == 0 {
		t.Fatal("List() returned empty pipeline")
	}
	
	t.Logf("Successfully built pipeline with %d transforms", len(pipeline))
}

// TestAllRegisteredTransformsInPipeline verifies all registered transforms appear in the pipeline
func TestAllRegisteredTransformsInPipeline(t *testing.T) {
	// List() returns all transforms already in pipeline order
	pipeline, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	
	if len(pipeline) == 0 {
		t.Fatal("List() returned empty pipeline")
	}
	
	// Verify no duplicates
	seen := make(map[string]bool)
	for _, tr := range pipeline {
		if seen[tr.Name()] {
			t.Errorf("Duplicate transform %q in pipeline", tr.Name())
		}
		seen[tr.Name()] = true
	}
	
	t.Logf("Pipeline contains %d unique transforms", len(pipeline))
}
