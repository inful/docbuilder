package middleware

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/commands"
)

// Middleware represents a function that can wrap command execution.
// This implements the Decorator pattern for adding cross-cutting concerns.
type Middleware func(commands.StageCommand) commands.StageCommand

// Chain applies multiple middleware to a command in order.
func Chain(cmd commands.StageCommand, middlewares ...Middleware) commands.StageCommand {
	// Apply middleware in reverse order so they execute in the correct order
	for i := len(middlewares) - 1; i >= 0; i-- {
		cmd = middlewares[i](cmd)
	}
	return cmd
}

// Command wraps another command to provide middleware functionality.
type Command struct {
	wrapped commands.StageCommand
	execute func(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution
}

// NewCommand creates a new middleware command that wraps another command.
func NewCommand(wrapped commands.StageCommand, execute func(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution) *Command {
	return &Command{
		wrapped: wrapped,
		execute: execute,
	}
}

// Name returns the wrapped command's name.
func (m *Command) Name() hugo.StageName {
	return m.wrapped.Name()
}

// Description returns the wrapped command's description.
func (m *Command) Description() string {
	return m.wrapped.Description()
}

// Dependencies returns the wrapped command's dependencies.
func (m *Command) Dependencies() []hugo.StageName {
	return m.wrapped.Dependencies()
}

// Execute runs the middleware's custom execution logic.
func (m *Command) Execute(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution {
	return m.execute(ctx, bs)
}

// TimingMiddleware adds execution timing to commands.
// Note: This middleware depends on the metrics being recorded separately by the pipeline.
func TimingMiddleware() Middleware {
	return func(cmd commands.StageCommand) commands.StageCommand {
		return NewCommand(cmd, func(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution {
			start := time.Now()

			// Execute the command
			result := cmd.Execute(ctx, bs)

			// Timing is recorded by the pipeline infrastructure,
			// not directly by middleware to avoid accessing private fields
			_ = start // duration available for future direct recording

			return result
		})
	}
}

// ObservabilityMiddleware adds result observation to commands.
// Note: This middleware depends on the metrics being recorded separately by the pipeline.
func ObservabilityMiddleware() Middleware {
	return func(cmd commands.StageCommand) commands.StageCommand {
		return NewCommand(cmd, func(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution {
			result := cmd.Execute(ctx, bs)

			// Result observation is recorded by the pipeline infrastructure,
			// not directly by middleware to avoid accessing private fields

			return result
		})
	}
}

// LoggingMiddleware adds structured logging to commands.
func LoggingMiddleware() Middleware {
	return func(cmd commands.StageCommand) commands.StageCommand {
		return NewCommand(cmd, func(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution {
			// Log stage start if the command supports it
			if logger, ok := cmd.(interface{ LogStageStart() }); ok {
				logger.LogStageStart()
			}

			result := cmd.Execute(ctx, bs)

			// Log result if the command supports it
			if logger, ok := cmd.(interface {
				LogStageSuccess()
				LogStageFailure(error)
			}); ok {
				if result.IsSuccess() {
					logger.LogStageSuccess()
				} else {
					logger.LogStageFailure(result.Err)
				}
			}

			return result
		})
	}
}

// SkipMiddleware adds skip condition checking to commands.
func SkipMiddleware() Middleware {
	return func(cmd commands.StageCommand) commands.StageCommand {
		return NewCommand(cmd, func(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution {
			// Check if command should be skipped
			if skipper, ok := cmd.(interface{ ShouldSkip(*hugo.BuildState) bool }); ok {
				if skipper.ShouldSkip(bs) {
					// Log skip if the command supports it
					if logger, ok := cmd.(interface{ LogStageSkipped() }); ok {
						logger.LogStageSkipped()
					}
					return hugo.ExecutionSuccessWithSkip()
				}
			}

			return cmd.Execute(ctx, bs)
		})
	}
}

// ContextMiddleware adds context cancellation checking.
func ContextMiddleware() Middleware {
	return func(cmd commands.StageCommand) commands.StageCommand {
		return NewCommand(cmd, func(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution {
			select {
			case <-ctx.Done():
				return hugo.ExecutionFailure(ctx.Err())
			default:
				return cmd.Execute(ctx, bs)
			}
		})
	}
}

// ErrorHandlingMiddleware adds structured error handling to commands.
func ErrorHandlingMiddleware() Middleware {
	return func(cmd commands.StageCommand) commands.StageCommand {
		return NewCommand(cmd, func(ctx context.Context, bs *hugo.BuildState) hugo.StageExecution {
			result := cmd.Execute(ctx, bs)

			// Wrap errors with command context if not already wrapped
			if result.Err != nil {
				if _, ok := result.Err.(*commands.ExecutionError); !ok {
					result.Err = &commands.ExecutionError{
						Command: cmd.Name(),
						Cause:   result.Err,
					}
				}
			}

			return result
		})
	}
}

// DefaultMiddleware returns the standard middleware stack.
func DefaultMiddleware() []Middleware {
	return []Middleware{
		ContextMiddleware(),
		ErrorHandlingMiddleware(),
		LoggingMiddleware(),
		SkipMiddleware(),
		TimingMiddleware(),
		ObservabilityMiddleware(),
	}
}
