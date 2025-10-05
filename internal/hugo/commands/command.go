package commands

import (
	"context"
	"log/slog"

	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// StageCommand represents a single build stage that can be executed.
// This interface implements the Command pattern for hugo build stages.
type StageCommand interface {
	// Name returns the name of this stage command
	Name() hugo.StageName

	// Execute runs the stage command with the given build state
	Execute(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution

	// Description returns a human-readable description of what this stage does
	Description() string

	// Dependencies returns the names of stages that must complete successfully before this stage
	Dependencies() []hugo.StageName
}

// CommandMetadata provides additional information about a command.
type CommandMetadata struct {
	Name         hugo.StageName
	Description  string
	Dependencies []hugo.StageName
	Optional     bool                        // If true, failure doesn't stop the pipeline
	SkipIf       func(*hugo.BuildState) bool // Function to determine if stage should be skipped
}

// BaseCommand provides a common implementation for stage commands.
type BaseCommand struct {
	metadata CommandMetadata
}

// NewBaseCommand creates a new base command with the given metadata.
func NewBaseCommand(metadata CommandMetadata) BaseCommand {
	return BaseCommand{metadata: metadata}
}

// Name returns the stage name.
func (c BaseCommand) Name() hugo.StageName {
	return c.metadata.Name
}

// Description returns the stage description.
func (c BaseCommand) Description() string {
	return c.metadata.Description
}

// Dependencies returns the stage dependencies.
func (c BaseCommand) Dependencies() []hugo.StageName {
	return c.metadata.Dependencies
}

// IsOptional returns whether this stage is optional.
func (c BaseCommand) IsOptional() bool {
	return c.metadata.Optional
}

// ShouldSkip checks if this stage should be skipped based on build state.
func (c BaseCommand) ShouldSkip(bs *hugo.BuildState) bool {
	if c.metadata.SkipIf != nil {
		return c.metadata.SkipIf(bs)
	}
	return false
}

// LogStageStart logs the start of a stage execution.
func (c BaseCommand) LogStageStart() {
	slog.Info("Starting stage", slog.String("stage", string(c.Name())))
}

// LogStageSuccess logs successful completion of a stage.
func (c BaseCommand) LogStageSuccess() {
	slog.Info("Stage completed successfully", slog.String("stage", string(c.Name())))
}

// LogStageSkipped logs that a stage was skipped.
func (c BaseCommand) LogStageSkipped() {
	slog.Info("Stage skipped", slog.String("stage", string(c.Name())))
}

// LogStageFailure logs failure of a stage.
func (c BaseCommand) LogStageFailure(err error) {
	slog.Error("Stage failed", slog.String("stage", string(c.Name())), slog.Any("error", err))
}

// CommandRegistry manages registered stage commands.
type CommandRegistry struct {
	commands map[hugo.StageName]StageCommand
}

// NewCommandRegistry creates a new command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[hugo.StageName]StageCommand),
	}
}

// Register adds a command to the registry.
func (r *CommandRegistry) Register(cmd StageCommand) {
	r.commands[cmd.Name()] = cmd
}

// Get retrieves a command by name.
func (r *CommandRegistry) Get(name hugo.StageName) (StageCommand, bool) {
	cmd, exists := r.commands[name]
	return cmd, exists
}

// List returns all registered command names.
func (r *CommandRegistry) List() []hugo.StageName {
	names := make([]hugo.StageName, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}

// GetAll returns all registered commands.
func (r *CommandRegistry) GetAll() map[hugo.StageName]StageCommand {
	result := make(map[hugo.StageName]StageCommand, len(r.commands))
	for name, cmd := range r.commands {
		result[name] = cmd
	}
	return result
}

// ValidateDependencies checks that all command dependencies are satisfied.
func (r *CommandRegistry) ValidateDependencies() error {
	for _, cmd := range r.commands {
		for _, dep := range cmd.Dependencies() {
			if _, exists := r.commands[dep]; !exists {
				return &DependencyError{
					Command:    cmd.Name(),
					Dependency: dep,
				}
			}
		}
	}
	return nil
}

// DependencyError represents a missing dependency error.
type DependencyError struct {
	Command    hugo.StageName
	Dependency hugo.StageName
}

func (e *DependencyError) Error() string {
	return "command " + string(e.Command) + " depends on missing command " + string(e.Dependency)
}

// ExecutionError represents a command execution error.
type ExecutionError struct {
	Command hugo.StageName
	Cause   error
}

func (e *ExecutionError) Error() string {
	return "command " + string(e.Command) + " failed: " + e.Cause.Error()
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// DefaultRegistry is the default command registry used by the pipeline.
var DefaultRegistry = NewCommandRegistry()
