package commands

import (
	"fmt"
	"log/slog"
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	tr "git.home.luguber.info/inful/docbuilder/internal/hugo/transforms"
)

// VisualizeCmd implements the 'visualize' command.
type VisualizeCmd struct {
	Format string `short:"f" help:"Output format: text, mermaid, dot, json" default:"text" enum:"text,mermaid,dot,json"`
	Output string `short:"o" help:"Output file path (optional, prints to stdout if not specified)"`
	List   bool   `short:"l" help:"List available formats and exit"`
}

// Run executes the visualize command.
func (cmd *VisualizeCmd) Run(_ *Global, _ *CLI) error {
	// Import defaults package to register transforms
	_ = hugo.NewGenerator(&config.Config{}, "")

	// Handle --list flag
	if cmd.List {
		fmt.Println("Available visualization formats:")
		fmt.Println()
		for _, format := range tr.GetSupportedFormats() {
			desc := tr.GetFormatDescription(format)
			fmt.Printf("  %-10s %s\n", format, desc)
		}
		fmt.Println()
		fmt.Println("Usage examples:")
		fmt.Println("  docbuilder visualize                    # Text format to stdout")
		fmt.Println("  docbuilder visualize -f mermaid         # Mermaid diagram to stdout")
		fmt.Println("  docbuilder visualize -f dot -o pipe.dot # DOT format to file")
		return nil
	}

	// Generate visualization
	output, err := tr.VisualizePipeline(tr.VisualizationFormat(cmd.Format))
	if err != nil {
		return fmt.Errorf("failed to visualize pipeline: %w", err)
	}

	// Write to file or stdout
	if cmd.Output != "" {
		if err := os.WriteFile(cmd.Output, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		slog.Info("Pipeline visualization written", "file", cmd.Output, "format", cmd.Format)
	} else {
		fmt.Print(output)
	}

	return nil
}
