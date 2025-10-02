package main

import (
	"context"
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/cli"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// ServiceContainer holds the service orchestrator and executor
type ServiceContainer struct {
	Orchestrator *services.ServiceOrchestrator
	Executor     *cli.DefaultCommandExecutor
}

// InitializeServices creates and starts the service container
func InitializeServices(ctx context.Context) (*ServiceContainer, error) {
	// Create service orchestrator
	orchestrator := services.NewServiceOrchestrator()

	// Create command executor (standalone service)
	executor := cli.NewCommandExecutor("cli_executor")

	// Note: Command executor operates independently for now
	// Future: Implement full ManagedService interface for orchestrator integration

	return &ServiceContainer{
		Orchestrator: orchestrator,
		Executor:     executor,
	}, nil
}

// Shutdown gracefully stops all services
func (sc *ServiceContainer) Shutdown(ctx context.Context) error {
	if err := sc.Orchestrator.StopAll(ctx); err != nil {
		return fmt.Errorf("stop services: %w", err)
	}
	return nil
}

// IntegratedMain demonstrates the complete service-oriented CLI
func IntegratedMain() error {
	ctx := context.Background()

	// Initialize services
	container, err := InitializeServices(ctx)
	if err != nil {
		return fmt.Errorf("initialize services: %w", err)
	}
	defer container.Shutdown(ctx)

	// Example build execution
	buildReq := cli.BuildRequest{
		ConfigPath:  "config.yaml",
		OutputDir:   "output",
		Incremental: false,
		RenderMode:  "hugo",
		Verbose:     true,
	}

	result := container.Executor.ExecuteBuild(ctx, buildReq)
	if result.IsErr() {
		return fmt.Errorf("build execution failed: %w", result.UnwrapErr())
	}

	response := result.Unwrap()
	fmt.Printf("Build completed: %s (%d files)\n",
		response.OutputPath, response.FilesBuilt)

	return nil
}
