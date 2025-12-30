package commands

import (
	"fmt"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// InitCmd implements the 'init' command.
type InitCmd struct {
	Force  bool   `help:"Overwrite existing configuration file"`
	Output string `short:"o" name:"output" help:"Output directory for generated config file"`
}

func (i *InitCmd) Run(_ *Global, root *CLI) error {
	// If the user specified an output directory, place the config there as "docbuilder.yaml".
	if i.Output != "" {
		cfgPath := filepath.Join(i.Output, "docbuilder.yaml")
		return RunInit(cfgPath, i.Force)
	}
	return RunInit(root.Config, i.Force)
}

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
