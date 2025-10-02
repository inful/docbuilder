package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/services"
)

// HTTPServerService wraps the existing HTTPServer as a managed service.
// This allows the monolithic HTTP server to be managed by the service orchestrator.
type HTTPServerService struct {
	name       string
	httpServer *HTTPServer
	daemon     *Daemon
	config     *config.Config
	isRunning  bool
}

// NewHTTPServerService creates a new HTTP server service adapter.
func NewHTTPServerService(name string, daemon *Daemon, config *config.Config) *HTTPServerService {
	return &HTTPServerService{
		name:   name,
		daemon: daemon,
		config: config,
	}
}

// Name returns the service name.
func (s *HTTPServerService) Name() string {
	return s.name
}

// Start initializes and starts the HTTP server.
func (s *HTTPServerService) Start(ctx context.Context) error {
	slog.Info("Starting HTTP server service", "name", s.name)

	// Create HTTP server if it doesn't exist
	if s.httpServer == nil {
		s.httpServer = NewHTTPServer(s.config, s.daemon)
	}

	// Start the HTTP server
	if err := s.httpServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	s.isRunning = true
	slog.Info("HTTP server service started", "name", s.name)
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *HTTPServerService) Stop(ctx context.Context) error {
	if !s.isRunning || s.httpServer == nil {
		return nil
	}

	slog.Info("Stopping HTTP server service", "name", s.name)

	if err := s.httpServer.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop HTTP server: %w", err)
	}

	s.isRunning = false
	slog.Info("HTTP server service stopped", "name", s.name)
	return nil
}

// Health returns the health status of the HTTP server.
func (s *HTTPServerService) Health() services.HealthStatus {
	if !s.isRunning || s.httpServer == nil {
		return services.HealthStatus{
			Status:  "unhealthy",
			Message: "HTTP server not running",
			CheckAt: time.Now(),
		}
	}

	// Check if HTTP server is responsive - use admin port for health check
	if s.config.Daemon.HTTP.AdminPort > 0 {
		// Simple health check - try to create a request to the status endpoint
		client := &http.Client{Timeout: 2 * time.Second}
		url := fmt.Sprintf("http://localhost:%d/status", s.config.Daemon.HTTP.AdminPort)

		resp, err := client.Get(url)
		if err != nil {
			return services.HealthStatus{
				Status:  "unhealthy",
				Message: fmt.Sprintf("HTTP server not responsive: %v", err),
				CheckAt: time.Now(),
			}
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return services.HealthStatus{
				Status:  "unhealthy",
				Message: fmt.Sprintf("HTTP server returned status %d", resp.StatusCode),
				CheckAt: time.Now(),
			}
		}
	}

	return services.HealthStatus{
		Status:  "healthy",
		Message: "HTTP server is responsive",
		CheckAt: time.Now(),
	}
}

// Dependencies returns the services this HTTP server depends on.
func (s *HTTPServerService) Dependencies() []string {
	// HTTP server typically depends on state management for serving status
	return []string{"state"}
}

// BuildQueueService wraps the existing BuildQueue as a managed service.
type BuildQueueService struct {
	name       string
	buildQueue *BuildQueue
	config     *config.Config
	isRunning  bool
}

// NewBuildQueueService creates a new build queue service adapter.
func NewBuildQueueService(name string, config *config.Config) *BuildQueueService {
	return &BuildQueueService{
		name:   name,
		config: config,
	}
}

// Name returns the service name.
func (s *BuildQueueService) Name() string {
	return s.name
}

// Start initializes and starts the build queue.
func (s *BuildQueueService) Start(ctx context.Context) error {
	slog.Info("Starting build queue service", "name", s.name)

	// Create build queue if it doesn't exist
	if s.buildQueue == nil {
		queueSize := s.config.Daemon.Sync.QueueSize
		if queueSize <= 0 {
			queueSize = 100 // Default queue size
		}
		workers := s.config.Daemon.Sync.ConcurrentBuilds
		if workers <= 0 {
			workers = 2 // Default worker count
		}
		s.buildQueue = NewBuildQueue(queueSize, workers)
	}

	// Start the build queue
	s.buildQueue.Start(ctx)

	s.isRunning = true
	slog.Info("Build queue service started", "name", s.name, "queue_size", s.config.Daemon.Sync.QueueSize)
	return nil
}

// Stop gracefully shuts down the build queue.
func (s *BuildQueueService) Stop(ctx context.Context) error {
	if !s.isRunning || s.buildQueue == nil {
		return nil
	}

	slog.Info("Stopping build queue service", "name", s.name)

	s.buildQueue.Stop(ctx)

	s.isRunning = false
	slog.Info("Build queue service stopped", "name", s.name)
	return nil
}

// Health returns the health status of the build queue.
func (s *BuildQueueService) Health() services.HealthStatus {
	if !s.isRunning || s.buildQueue == nil {
		return services.HealthStatus{
			Status:  "unhealthy",
			Message: "Build queue not running",
			CheckAt: time.Now(),
		}
	}

	// Check queue health - ensure it's not overwhelmed
	queueLength := s.buildQueue.Length()
	maxQueue := s.config.Daemon.Sync.QueueSize

	if queueLength >= maxQueue {
		return services.HealthStatus{
			Status:  "unhealthy",
			Message: fmt.Sprintf("Build queue is full (%d/%d)", queueLength, maxQueue),
			CheckAt: time.Now(),
		}
	}

	if queueLength > maxQueue*3/4 {
		return services.HealthStatus{
			Status:  "degraded",
			Message: fmt.Sprintf("Build queue is nearly full (%d/%d)", queueLength, maxQueue),
			CheckAt: time.Now(),
		}
	}

	return services.HealthStatus{
		Status:  "healthy",
		Message: fmt.Sprintf("Build queue operational (%d/%d)", queueLength, maxQueue),
		CheckAt: time.Now(),
	}
}

// Dependencies returns the services this build queue depends on.
func (s *BuildQueueService) Dependencies() []string {
	// Build queue depends on state management for tracking builds
	return []string{"state"}
}

// GetBuildQueue returns the underlying build queue for direct access.
func (s *BuildQueueService) GetBuildQueue() *BuildQueue {
	return s.buildQueue
}

// SchedulerService wraps the existing Scheduler as a managed service.
type SchedulerService struct {
	name      string
	scheduler *Scheduler
	daemon    *Daemon
	isRunning bool
}

// NewSchedulerService creates a new scheduler service adapter.
func NewSchedulerService(name string, daemon *Daemon, buildQueue *BuildQueue) *SchedulerService {
	return &SchedulerService{
		name:      name,
		daemon:    daemon,
		scheduler: NewScheduler(buildQueue),
	}
}

// Name returns the service name.
func (s *SchedulerService) Name() string {
	return s.name
}

// Start initializes and starts the scheduler.
func (s *SchedulerService) Start(ctx context.Context) error {
	slog.Info("Starting scheduler service", "name", s.name)

	// Set daemon reference to avoid import cycles
	s.scheduler.SetDaemon(s.daemon)

	// Start the scheduler
	s.scheduler.Start(ctx)

	s.isRunning = true
	slog.Info("Scheduler service started", "name", s.name)
	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *SchedulerService) Stop(ctx context.Context) error {
	if !s.isRunning || s.scheduler == nil {
		return nil
	}

	slog.Info("Stopping scheduler service", "name", s.name)

	s.scheduler.Stop(ctx)

	s.isRunning = false
	slog.Info("Scheduler service stopped", "name", s.name)
	return nil
}

// Health returns the health status of the scheduler.
func (s *SchedulerService) Health() services.HealthStatus {
	if !s.isRunning || s.scheduler == nil {
		return services.HealthStatus{
			Status:  "unhealthy",
			Message: "Scheduler not running",
			CheckAt: time.Now(),
		}
	}

	// Simple check - if we can acquire the lock, scheduler is likely healthy
	s.scheduler.mu.RLock()
	scheduleCount := len(s.scheduler.schedules)
	s.scheduler.mu.RUnlock()

	return services.HealthStatus{
		Status:  "healthy",
		Message: fmt.Sprintf("Scheduler operational (%d schedules)", scheduleCount),
		CheckAt: time.Now(),
	}
}

// Dependencies returns the services this scheduler depends on.
func (s *SchedulerService) Dependencies() []string {
	// Scheduler depends on build queue and state management
	return []string{"build-queue", "state"}
}

// GetScheduler returns the underlying scheduler for direct access.
func (s *SchedulerService) GetScheduler() *Scheduler {
	return s.scheduler
}

// ConfigWatcherService wraps the existing ConfigWatcher as a managed service.
type ConfigWatcherService struct {
	name          string
	configWatcher *ConfigWatcher
	configPath    string
	daemon        *Daemon
	isRunning     bool
}

// NewConfigWatcherService creates a new config watcher service adapter.
func NewConfigWatcherService(name string, daemon *Daemon, configPath string) *ConfigWatcherService {
	return &ConfigWatcherService{
		name:       name,
		daemon:     daemon,
		configPath: configPath,
	}
}

// Name returns the service name.
func (s *ConfigWatcherService) Name() string {
	return s.name
}

// Start initializes and starts the config watcher.
func (s *ConfigWatcherService) Start(ctx context.Context) error {
	if s.configPath == "" {
		slog.Info("Config watcher service skipped - no config file path provided")
		return nil
	}

	slog.Info("Starting config watcher service", "name", s.name, "path", s.configPath)

	// Create config watcher if it doesn't exist
	if s.configWatcher == nil {
		configWatcher, err := NewConfigWatcher(s.configPath, s.daemon)
		if err != nil {
			return fmt.Errorf("failed to create config watcher: %w", err)
		}
		s.configWatcher = configWatcher
	}

	// Start the config watcher
	if err := s.configWatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start config watcher: %w", err)
	}

	s.isRunning = true
	slog.Info("Config watcher service started", "name", s.name)
	return nil
}

// Stop gracefully shuts down the config watcher.
func (s *ConfigWatcherService) Stop(ctx context.Context) error {
	if !s.isRunning || s.configWatcher == nil {
		return nil
	}

	slog.Info("Stopping config watcher service", "name", s.name)

	if err := s.configWatcher.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop config watcher: %w", err)
	}

	s.isRunning = false
	slog.Info("Config watcher service stopped", "name", s.name)
	return nil
}

// Health returns the health status of the config watcher.
func (s *ConfigWatcherService) Health() services.HealthStatus {
	if s.configPath == "" {
		return services.HealthStatus{
			Status:  "healthy",
			Message: "Config watcher disabled (no config file)",
			CheckAt: time.Now(),
		}
	}

	if !s.isRunning || s.configWatcher == nil {
		return services.HealthStatus{
			Status:  "unhealthy",
			Message: "Config watcher not running",
			CheckAt: time.Now(),
		}
	}

	// Basic health check - config watcher is healthy if it exists and is running
	return services.HealthStatus{
		Status:  "healthy",
		Message: "Config watcher monitoring file changes",
		CheckAt: time.Now(),
	}
}

// Dependencies returns the services this config watcher depends on.
func (s *ConfigWatcherService) Dependencies() []string {
	// Config watcher has no dependencies - it's one of the first services to start
	return []string{}
}
