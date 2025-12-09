package transforms

import (
	"fmt"
	"strings"
)

// VisualizationFormat represents the output format for pipeline visualization.
type VisualizationFormat string

const (
	FormatText    VisualizationFormat = "text"
	FormatMermaid VisualizationFormat = "mermaid"
	FormatDOT     VisualizationFormat = "dot"
	FormatJSON    VisualizationFormat = "json"
)

// VisualizePipeline generates a visual representation of the transform pipeline.
func VisualizePipeline(format VisualizationFormat) (string, error) {
	transforms, err := List()
	if err != nil {
		return "", err
	}

	switch format {
	case FormatText:
		return visualizeText(transforms)
	case FormatMermaid:
		return visualizeMermaid(transforms)
	case FormatDOT:
		return visualizeDOT(transforms)
	case FormatJSON:
		return visualizeJSON(transforms)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// visualizeText creates a text-based visualization with ASCII art.
func visualizeText(transforms []Transformer) (string, error) {
	var sb strings.Builder
	
	sb.WriteString("Transform Pipeline Visualization\n")
	sb.WriteString("=================================\n\n")
	
	// Group by stage
	byStage := make(map[TransformStage][]Transformer)
	for _, t := range transforms {
		stage := t.Stage()
		byStage[stage] = append(byStage[stage], t)
	}
	
	stages := []TransformStage{StageParse, StageBuild, StageEnrich, StageMerge, StageTransform, StageFinalize, StageSerialize}
	
	for i, stage := range stages {
		stageTransforms := byStage[stage]
		if len(stageTransforms) == 0 {
			continue
		}
		
		// Stage header
		sb.WriteString(fmt.Sprintf("┌─ Stage %d: %s\n", i+1, stage))
		sb.WriteString("│\n")
		
		// Transforms in this stage
		for j, t := range stageTransforms {
			deps := t.Dependencies()
			isLast := j == len(stageTransforms)-1
			
			prefix := "├──"
			if isLast {
				prefix = "└──"
			}
			
			sb.WriteString(fmt.Sprintf("│ %s [%s]\n", prefix, t.Name()))
			
			// Show dependencies
			if len(deps.MustRunAfter) > 0 {
				connector := "│   "
				if isLast {
					connector = "    "
				}
				sb.WriteString(fmt.Sprintf("│ %s   ⤷ depends on: %s\n", connector, strings.Join(deps.MustRunAfter, ", ")))
			}
			
			if len(deps.MustRunBefore) > 0 {
				connector := "│   "
				if isLast {
					connector = "    "
				}
				sb.WriteString(fmt.Sprintf("│ %s   ⤶ required before: %s\n", connector, strings.Join(deps.MustRunBefore, ", ")))
			}
		}
		
		sb.WriteString("│\n")
		if i < len(stages)-1 {
			sb.WriteString("↓\n")
		}
	}
	
	sb.WriteString(fmt.Sprintf("\nTotal: %d transforms across %d stages\n", len(transforms), len(byStage)))
	
	return sb.String(), nil
}

// visualizeMermaid creates a Mermaid diagram.
func visualizeMermaid(transforms []Transformer) (string, error) {
	var sb strings.Builder
	
	sb.WriteString("```mermaid\n")
	sb.WriteString("graph TD\n")
	
	// Group by stage for subgraphs
	byStage := make(map[TransformStage][]Transformer)
	for _, t := range transforms {
		stage := t.Stage()
		byStage[stage] = append(byStage[stage], t)
	}
	
	stages := []TransformStage{StageParse, StageBuild, StageEnrich, StageMerge, StageTransform, StageFinalize, StageSerialize}
	
	// Create subgraphs for each stage
	for _, stage := range stages {
		stageTransforms := byStage[stage]
		if len(stageTransforms) == 0 {
			continue
		}
		
		sb.WriteString(fmt.Sprintf("    subgraph %s[\"Stage: %s\"]\n", stage, stage))
		
		for _, t := range stageTransforms {
			// Sanitize name for Mermaid (replace underscores and hyphens)
			nodeName := strings.ReplaceAll(t.Name(), "_", "")
			nodeName = strings.ReplaceAll(nodeName, "-", "")
			
			sb.WriteString(fmt.Sprintf("        %s[\"%s\"]\n", nodeName, t.Name()))
		}
		
		sb.WriteString("    end\n")
	}
	
	sb.WriteString("\n")
	
	// Add dependency edges
	for _, t := range transforms {
		deps := t.Dependencies()
		tName := strings.ReplaceAll(t.Name(), "_", "")
		tName = strings.ReplaceAll(tName, "-", "")
		
		for _, dep := range deps.MustRunAfter {
			depName := strings.ReplaceAll(dep, "_", "")
			depName = strings.ReplaceAll(depName, "-", "")
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", depName, tName))
		}
		
		for _, after := range deps.MustRunBefore {
			afterName := strings.ReplaceAll(after, "_", "")
			afterName = strings.ReplaceAll(afterName, "-", "")
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", tName, afterName))
		}
	}
	
	sb.WriteString("```\n")
	
	return sb.String(), nil
}

// visualizeDOT creates a Graphviz DOT diagram.
func visualizeDOT(transforms []Transformer) (string, error) {
	var sb strings.Builder
	
	sb.WriteString("digraph TransformPipeline {\n")
	sb.WriteString("    rankdir=TB;\n")
	sb.WriteString("    node [shape=box, style=rounded];\n\n")
	
	// Group by stage for clusters
	byStage := make(map[TransformStage][]Transformer)
	for _, t := range transforms {
		stage := t.Stage()
		byStage[stage] = append(byStage[stage], t)
	}
	
	stages := []TransformStage{StageParse, StageBuild, StageEnrich, StageMerge, StageTransform, StageFinalize, StageSerialize}
	
	// Create clusters for each stage
	for i, stage := range stages {
		stageTransforms := byStage[stage]
		if len(stageTransforms) == 0 {
			continue
		}
		
		sb.WriteString(fmt.Sprintf("    subgraph cluster_%d {\n", i))
		sb.WriteString(fmt.Sprintf("        label=\"Stage: %s\";\n", stage))
		sb.WriteString("        style=filled;\n")
		sb.WriteString("        color=lightgrey;\n\n")
		
		for _, t := range stageTransforms {
			// Quote names for DOT format
			sb.WriteString(fmt.Sprintf("        \"%s\";\n", t.Name()))
		}
		
		sb.WriteString("    }\n\n")
	}
	
	// Add dependency edges
	for _, t := range transforms {
		deps := t.Dependencies()
		
		for _, dep := range deps.MustRunAfter {
			sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", dep, t.Name()))
		}
		
		for _, after := range deps.MustRunBefore {
			sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", t.Name(), after))
		}
	}
	
	sb.WriteString("}\n")
	
	return sb.String(), nil
}

// visualizeJSON creates a JSON representation of the pipeline.
func visualizeJSON(transforms []Transformer) (string, error) {
	var sb strings.Builder
	
	sb.WriteString("{\n")
	sb.WriteString("  \"transforms\": [\n")
	
	for i, t := range transforms {
		deps := t.Dependencies()
		
		sb.WriteString("    {\n")
		sb.WriteString(fmt.Sprintf("      \"name\": %q,\n", t.Name()))
		sb.WriteString(fmt.Sprintf("      \"stage\": %q,\n", t.Stage()))
		sb.WriteString(fmt.Sprintf("      \"order\": %d,\n", i+1))
		
		// Dependencies
		sb.WriteString("      \"dependencies\": {\n")
		
		// MustRunAfter
		sb.WriteString("        \"mustRunAfter\": [")
		if len(deps.MustRunAfter) > 0 {
			sb.WriteString("\n")
			for j, dep := range deps.MustRunAfter {
				sb.WriteString(fmt.Sprintf("          %q", dep))
				if j < len(deps.MustRunAfter)-1 {
					sb.WriteString(",")
				}
				sb.WriteString("\n")
			}
			sb.WriteString("        ")
		}
		sb.WriteString("],\n")
		
		// MustRunBefore
		sb.WriteString("        \"mustRunBefore\": [")
		if len(deps.MustRunBefore) > 0 {
			sb.WriteString("\n")
			for j, after := range deps.MustRunBefore {
				sb.WriteString(fmt.Sprintf("          %q", after))
				if j < len(deps.MustRunBefore)-1 {
					sb.WriteString(",")
				}
				sb.WriteString("\n")
			}
			sb.WriteString("        ")
		}
		sb.WriteString("]\n")
		
		sb.WriteString("      }\n")
		sb.WriteString("    }")
		
		if i < len(transforms)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	
	sb.WriteString("  ],\n")
	sb.WriteString(fmt.Sprintf("  \"totalTransforms\": %d,\n", len(transforms)))
	
	// Count stages
	stageSet := make(map[TransformStage]bool)
	for _, t := range transforms {
		stageSet[t.Stage()] = true
	}
	sb.WriteString(fmt.Sprintf("  \"totalStages\": %d\n", len(stageSet)))
	
	sb.WriteString("}\n")
	
	return sb.String(), nil
}

// GetSupportedFormats returns a list of supported visualization formats.
func GetSupportedFormats() []VisualizationFormat {
	return []VisualizationFormat{FormatText, FormatMermaid, FormatDOT, FormatJSON}
}

// GetFormatDescription returns a description of a visualization format.
func GetFormatDescription(format VisualizationFormat) string {
	descriptions := map[VisualizationFormat]string{
		FormatText:    "Human-readable text with ASCII art",
		FormatMermaid: "Mermaid diagram (for GitHub, GitLab, etc.)",
		FormatDOT:     "Graphviz DOT format (render with `dot -Tpng pipeline.dot -o pipeline.png`)",
		FormatJSON:    "Structured JSON representation",
	}
	return descriptions[format]
}
