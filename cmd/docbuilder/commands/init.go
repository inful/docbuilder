package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// InitCmd implements the 'init' command.
type InitCmd struct {
	Force  bool   `help:"Overwrite existing configuration file"`
	Output string `short:"o" name:"output" help:"Output directory for generated config file"`
}

func (i *InitCmd) Run(_ *Global, root *CLI) error {
	// If the user specified an output directory, place the config
	// there. The directory is made: the operator named a directory that
	// does not exist yet.
	if i.Output != "" {
		if err := os.MkdirAll(i.Output, 0o750); err != nil {
			return fmt.Errorf("create the output directory: %w", err)
		}
		return RunInit(filepath.Join(i.Output, DefaultConfigFileName), i.Force)
	}
	return RunInit(root.Config, i.Force)
}

//nolint:forbidigo // fmt is used for user-facing messages
func RunInit(configPath string, force bool) error {
	// Provide friendly user-facing messages on stdout for CLI integration tests.
	fmt.Println("Initializing DocBuilder project")
	fmt.Printf("Writing configuration to %s\n", configPath)
	if err := config.Init(configPath, force); err != nil {
		fmt.Println("Initialization failed")
		return err
	}
	fmt.Println("initialized successfully")
	return nil
}
