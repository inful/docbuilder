package commands

import (
	"context"
	"fmt"
	"os"

	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// PrepareOutputCommand implements the output preparation stage.
type PrepareOutputCommand struct {
	BaseCommand
}

// NewPrepareOutputCommand creates a new prepare output command.
func NewPrepareOutputCommand() *PrepareOutputCommand {
	return &PrepareOutputCommand{
		BaseCommand: NewBaseCommand(CommandMetadata{
			Name:         hugo.StagePrepareOutput,
			Description:  "Prepare output directory and workspace",
			Dependencies: []hugo.StageName{}, // No dependencies - first stage
		}),
	}
}

// Execute runs the prepare output stage.
func (c *PrepareOutputCommand) Execute(_ context.Context, bs *hugo.BuildState) hugo.StageExecution {
	c.LogStageStart()

	// This is a simplified implementation for the command pattern
	// In practice, this would prepare the output directory and workspace
	// For now, we just ensure the workspace directory exists

	if bs.Git.WorkspaceDir != "" {
		// Ensure workspace directory exists
		if err := os.MkdirAll(bs.Git.WorkspaceDir, 0o755); err != nil {
			err = fmt.Errorf("failed to create workspace directory %s: %w", bs.Git.WorkspaceDir, err)
			c.LogStageFailure(err)
			return hugo.ExecutionFailure(err)
		}
	}

	c.LogStageSuccess()
	return hugo.ExecutionSuccess()
}

func init() {
	// Register the prepare output command
	DefaultRegistry.Register(NewPrepareOutputCommand())
}
