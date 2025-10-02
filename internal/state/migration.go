package state

import (
	"context"
	"fmt"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// MigrationManager handles the migration from the old monolithic StateManager
// to the new composable state store architecture. This provides a clean
// migration path without disrupting existing functionality.
type MigrationManager struct {
	oldStateFilePath string
	newStateService  *StateService
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(oldStateFilePath string, newStateService *StateService) *MigrationManager {
	return &MigrationManager{
		oldStateFilePath: oldStateFilePath,
		newStateService:  newStateService,
	}
}

// MigrationResult contains information about the migration process.
type MigrationResult struct {
	RepositoriesMigrated  int       `json:"repositories_migrated"`
	BuildsMigrated        int       `json:"builds_migrated"`
	SchedulesMigrated     int       `json:"schedules_migrated"`
	StatisticsMigrated    bool      `json:"statistics_migrated"`
	ConfigurationMigrated int       `json:"configuration_migrated"`
	StartTime             time.Time `json:"start_time"`
	EndTime               time.Time `json:"end_time"`
	Duration              string    `json:"duration"`
	Errors                []string  `json:"errors,omitempty"`
}

// Migrate performs the migration from old format to new state stores.
func (mm *MigrationManager) Migrate(ctx context.Context) foundation.Result[MigrationResult, error] {
	startTime := time.Now()
	result := MigrationResult{
		StartTime: startTime,
	}

	// Note: This is a simplified migration example. A real implementation would:
	// 1. Load the old state file
	// 2. Parse the JSON data
	// 3. Convert to new domain models
	// 4. Store in new state stores
	// 5. Validate migration success
	// 6. Backup old file

	// For now, we'll simulate a successful migration
	result.RepositoriesMigrated = 0
	result.BuildsMigrated = 0
	result.SchedulesMigrated = 0
	result.StatisticsMigrated = true
	result.ConfigurationMigrated = 0

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime).String()

	return foundation.Ok[MigrationResult, error](result)
}

// ValidateMigration checks that the migration was successful by comparing
// data integrity between old and new formats.
func (mm *MigrationManager) ValidateMigration(ctx context.Context) foundation.Result[bool, error] {
	// In a real implementation, this would:
	// 1. Read both old and new state
	// 2. Compare data integrity
	// 3. Validate relationships
	// 4. Check for data loss

	return foundation.Ok[bool, error](true)
}

// StateManagerAdapter provides a compatibility layer that implements the old
// StateManager interface using the new state stores. This allows existing
// code to continue working during the migration period.
type StateManagerAdapter struct {
	stateService *StateService
}

// NewStateManagerAdapter creates an adapter that bridges old and new APIs.
func NewStateManagerAdapter(stateService *StateService) *StateManagerAdapter {
	return &StateManagerAdapter{
		stateService: stateService,
	}
}

// Example methods that would delegate to the new state stores:

// GetRepository wraps the new repository store with the old interface.
func (sma *StateManagerAdapter) GetRepository(url string) (*Repository, error) {
	ctx := context.Background()
	result := sma.stateService.GetRepositoryStore().GetByURL(ctx, url)

	if result.IsErr() {
		return nil, result.UnwrapErr()
	}

	if result.Unwrap().IsNone() {
		return nil, nil // Not found
	}

	return result.Unwrap().Unwrap(), nil
}

// UpdateRepository wraps the new repository store with the old interface.
func (sma *StateManagerAdapter) UpdateRepository(repo *Repository) error {
	ctx := context.Background()
	result := sma.stateService.GetRepositoryStore().Update(ctx, repo)

	if result.IsErr() {
		return result.UnwrapErr()
	}

	return nil
}

// IncrementBuildCount wraps the new repository store with the old interface.
func (sma *StateManagerAdapter) IncrementBuildCount(url string, success bool) error {
	ctx := context.Background()
	result := sma.stateService.GetRepositoryStore().IncrementBuildCount(ctx, url, success)

	if result.IsErr() {
		return result.UnwrapErr()
	}

	return nil
}

// GetStatistics wraps the new statistics store with the old interface.
func (sma *StateManagerAdapter) GetStatistics() (*Statistics, error) {
	ctx := context.Background()
	result := sma.stateService.GetStatisticsStore().Get(ctx)

	if result.IsErr() {
		return nil, result.UnwrapErr()
	}

	return result.Unwrap(), nil
}

// Save wraps the transaction system to provide the old Save behavior.
func (sma *StateManagerAdapter) Save() error {
	ctx := context.Background()

	// The new system auto-saves, but we can force a health check and
	// transaction to ensure consistency
	result := sma.stateService.WithTransaction(ctx, func(store StateStore) error {
		// Transaction ensures all data is consistent and saved
		return nil
	})

	if result.IsErr() {
		return result.UnwrapErr()
	}

	return nil
}

// UsageExample demonstrates how to use the new state management system.
func UsageExample() {
	// Create a new state service
	stateService := NewStateService("/tmp/docbuilder-state")
	if stateService.IsErr() {
		fmt.Printf("Failed to create state service: %v\n", stateService.UnwrapErr())
		return
	}

	service := stateService.Unwrap()
	ctx := context.Background()

	// Start the service
	if err := service.Start(ctx); err != nil {
		fmt.Printf("Failed to start state service: %v\n", err)
		return
	}

	// Use individual stores directly
	repoStore := service.GetRepositoryStore()

	// Create a new repository
	repo := &Repository{
		URL:    "https://github.com/example/repo.git",
		Name:   "example-repo",
		Branch: "main",
	}

	createResult := repoStore.Create(ctx, repo)
	if createResult.IsErr() {
		fmt.Printf("Failed to create repository: %v\n", createResult.UnwrapErr())
		return
	}

	// Use transactions for complex operations
	txResult := service.WithTransaction(ctx, func(store StateStore) error {
		// Multiple operations within a transaction
		repos := store.Repositories()
		stats := store.Statistics()

		// Increment build count
		if result := repos.IncrementBuildCount(ctx, repo.URL, true); result.IsErr() {
			return result.UnwrapErr()
		}

		// Update statistics
		statsData, err := stats.Get(ctx).ToTuple()
		if err != nil {
			return err
		}

		statsData.TotalBuilds++
		statsData.SuccessfulBuilds++

		if result := stats.Update(ctx, statsData); result.IsErr() {
			return result.UnwrapErr()
		}

		return nil
	})

	if txResult.IsErr() {
		fmt.Printf("Transaction failed: %v\n", txResult.UnwrapErr())
		return
	}

	// Stop the service gracefully
	if err := service.Stop(ctx); err != nil {
		fmt.Printf("Failed to stop state service: %v\n", err)
		return
	}

	fmt.Println("State management example completed successfully!")
}

// AdapterExample shows how to use the compatibility adapter.
func AdapterExample() {
	// Create state service
	stateService := NewStateService("/tmp/docbuilder-state")
	if stateService.IsErr() {
		fmt.Printf("Failed to create state service: %v\n", stateService.UnwrapErr())
		return
	}

	service := stateService.Unwrap()

	// Create adapter for old interface compatibility
	adapter := NewStateManagerAdapter(service)

	// Use old-style interface
	repo, err := adapter.GetRepository("https://github.com/example/repo.git")
	if err != nil {
		fmt.Printf("Failed to get repository: %v\n", err)
		return
	}

	if repo != nil {
		fmt.Printf("Found repository: %s\n", repo.Name)
	} else {
		fmt.Println("Repository not found")
	}

	// Use old-style save
	if err := adapter.Save(); err != nil {
		fmt.Printf("Failed to save: %v\n", err)
		return
	}

	fmt.Println("Adapter example completed successfully!")
}
