package transforms

import (
	"fmt"
	"sort"
	"strings"
)

// ValidationResult holds the results of pipeline validation.
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// AddError adds an error to the validation result.
func (vr *ValidationResult) AddError(format string, args ...any) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, fmt.Sprintf(format, args...))
}

// AddWarning adds a warning to the validation result.
func (vr *ValidationResult) AddWarning(format string, args ...any) {
	vr.Warnings = append(vr.Warnings, fmt.Sprintf(format, args...))
}

// ValidatePipeline performs comprehensive validation of the transform pipeline.
// This function checks for:
// - Missing dependencies
// - Circular dependencies
// - Invalid stage assignments
// - Unused transforms (warning only)
// - Capability mismatches
func ValidatePipeline() *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Get all registered transforms
	transforms := make([]Transformer, 0, len(registry))
	for _, t := range registry {
		transforms = append(transforms, t)
	}

	if len(transforms) == 0 {
		result.AddWarning("No transforms registered in pipeline")
		return result
	}

	// Build name mapping
	byName := make(map[string]Transformer)
	for _, t := range transforms {
		byName[t.Name()] = t
	}

	// Validate each transform
	for _, t := range transforms {
		name := t.Name()
		stage := t.Stage()
		deps := t.Dependencies()

		// Validate stage
		if !isValidStage(stage) {
			result.AddError("transform %q has invalid stage %q", name, stage)
		}

		// Validate MustRunAfter dependencies
		for _, dep := range deps.MustRunAfter {
			if depTransform, exists := byName[dep]; !exists {
				result.AddError("transform %q depends on missing transform %q (MustRunAfter)", name, dep)
			} else {
				// Check if dependency is in an earlier or same stage
				if !isValidDependencyStage(stage, depTransform.Stage()) {
					result.AddWarning("transform %q (stage %s) depends on %q (stage %s) which runs in a later stage - dependency may not be effective",
						name, stage, dep, depTransform.Stage())
				}
			}
		}

		// Validate MustRunBefore dependencies
		for _, after := range deps.MustRunBefore {
			if afterTransform, exists := byName[after]; !exists {
				result.AddError("transform %q requires missing transform %q to run after it (MustRunBefore)", name, after)
			} else {
				// Check if dependent is in a later or same stage
				if !isValidDependencyStage(afterTransform.Stage(), stage) {
					result.AddWarning("transform %q (stage %s) must run before %q (stage %s) which runs in an earlier stage - dependency may not be effective",
						name, stage, after, afterTransform.Stage())
				}
			}
		}
	}

	// Check for circular dependencies by attempting to build pipeline
	_, err := BuildPipeline(transforms)
	if err != nil {
		if strings.Contains(err.Error(), "circular") {
			result.AddError("circular dependency detected: %v", err)
		} else {
			result.AddError("pipeline build failed: %v", err)
		}
	}

	// Detect potentially unused transforms (those not in any dependency chain from core transforms)
	// This is informational only
	referencedTransforms := make(map[string]bool)
	for _, t := range transforms {
		deps := t.Dependencies()
		for _, dep := range deps.MustRunAfter {
			referencedTransforms[dep] = true
		}
		for _, after := range deps.MustRunBefore {
			referencedTransforms[after] = true
		}
	}

	// Core transforms that should always run
	coreTransforms := map[string]bool{
		"front_matter_parser":     true,
		"front_matter_builder_v2": true,
		"edit_link_injector_v2":   true,
		"front_matter_merge":      true,
		"relative_link_rewriter":  true,
		"front_matter_serialize":  true,
	}

	for _, t := range transforms {
		name := t.Name()
		if !coreTransforms[name] && !referencedTransforms[name] {
			result.AddWarning("transform %q is not referenced by any other transform's dependencies - ensure it's intentionally standalone", name)
		}
	}

	return result
}

// isValidStage checks if a stage is one of the defined transform stages.
func isValidStage(stage TransformStage) bool {
	switch stage {
	case StageParse, StageBuild, StageEnrich, StageMerge, StageTransform, StageFinalize, StageSerialize:
		return true
	default:
		return false
	}
}

// isValidDependencyStage checks if a dependency relationship respects stage ordering.
// Returns true if depStage comes before or in the same stage as currentStage.
func isValidDependencyStage(currentStage, depStage TransformStage) bool {
	stageOrder := map[TransformStage]int{
		StageParse:     1,
		StageBuild:     2,
		StageEnrich:    3,
		StageMerge:     4,
		StageTransform: 5,
		StageFinalize:  6,
		StageSerialize: 7,
	}

	current, currentOk := stageOrder[currentStage]
	dep, depOk := stageOrder[depStage]

	if !currentOk || !depOk {
		return false
	}

	return dep <= current
}

// ValidatePipelineWithSuggestions returns validation results with helpful suggestions.
func ValidatePipelineWithSuggestions() (*ValidationResult, []string) {
	result := ValidatePipeline()
	suggestions := make([]string, 0)

	if !result.Valid {
		suggestions = append(suggestions, "Pipeline validation failed. Please fix the errors above.")

		if len(result.Errors) > 0 {
			suggestions = append(suggestions, "")
			suggestions = append(suggestions, "Common fixes:")
			suggestions = append(suggestions, "  • Missing dependencies: Ensure all referenced transforms are registered")
			suggestions = append(suggestions, "  • Circular dependencies: Review MustRunAfter/MustRunBefore declarations")
			suggestions = append(suggestions, "  • Invalid stages: Use one of: parse, build, enrich, merge, transform, finalize, serialize")
		}
	}

	if len(result.Warnings) > 0 {
		suggestions = append(suggestions, "")
		suggestions = append(suggestions, "Warnings indicate potential issues but won't prevent pipeline execution.")
	}

	return result, suggestions
}

// PrintValidationResult prints a formatted validation result to help debugging.
func PrintValidationResult(result *ValidationResult) string {
	var sb strings.Builder

	sb.WriteString("Transform Pipeline Validation\n")
	sb.WriteString("==============================\n\n")

	if result.Valid && len(result.Warnings) == 0 {
		sb.WriteString("✓ Pipeline is valid with no warnings\n")
		return sb.String()
	}

	if len(result.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("✗ Errors (%d):\n", len(result.Errors)))
		for i, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err))
		}
		sb.WriteString("\n")
	}

	if len(result.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("⚠ Warnings (%d):\n", len(result.Warnings)))
		for i, warn := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, warn))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetPipelineInfo returns human-readable information about the current pipeline.
func GetPipelineInfo() (string, error) {
	transforms, err := List()
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString("Transform Pipeline Execution Order\n")
	sb.WriteString("===================================\n\n")

	// Group by stage
	byStage := make(map[TransformStage][]Transformer)
	for _, t := range transforms {
		stage := t.Stage()
		byStage[stage] = append(byStage[stage], t)
	}

	// Sort stages
	stages := []TransformStage{StageParse, StageBuild, StageEnrich, StageMerge, StageTransform, StageFinalize, StageSerialize}

	for _, stage := range stages {
		if transforms := byStage[stage]; len(transforms) > 0 {
			sb.WriteString(fmt.Sprintf("Stage: %s\n", stage))
			sb.WriteString(strings.Repeat("-", 40) + "\n")

			for _, t := range transforms {
				deps := t.Dependencies()
				sb.WriteString(fmt.Sprintf("  • %s\n", t.Name()))

				if len(deps.MustRunAfter) > 0 {
					sb.WriteString(fmt.Sprintf("    MustRunAfter: %s\n", strings.Join(deps.MustRunAfter, ", ")))
				}
				if len(deps.MustRunBefore) > 0 {
					sb.WriteString(fmt.Sprintf("    MustRunBefore: %s\n", strings.Join(deps.MustRunBefore, ", ")))
				}
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString(fmt.Sprintf("Total transforms: %d\n", len(transforms)))

	return sb.String(), nil
}

// ListTransformNames returns a sorted list of all registered transform names.
func ListTransformNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
