package services

import (
	"context"
	"fmt"
	"time"
)

// HTTPServerService adapts an HTTP server to the ManagedService interface.
type HTTPServerService struct {
	server HTTPServer
	name   string
}

// HTTPServer defines the interface expected by HTTPServerService.
type HTTPServer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool
}

// NewHTTPServerService creates a new HTTP server service adapter.
func NewHTTPServerService(name string, server HTTPServer) *HTTPServerService {
	return &HTTPServerService{
		server: server,
		name:   name,
	}
}

func (h *HTTPServerService) Name() string {
	return h.name
}

func (h *HTTPServerService) Start(ctx context.Context) error {
	return h.server.Start(ctx)
}

func (h *HTTPServerService) Stop(ctx context.Context) error {
	return h.server.Stop(ctx)
}

func (h *HTTPServerService) Health() HealthStatus {
	if h.server.IsRunning() {
		return HealthStatusHealthy
	}
	return HealthStatusUnhealthy("server not running")
}

func (h *HTTPServerService) Dependencies() []string {
	return []string{} // HTTP server typically has no dependencies
}

// BuildQueueService adapts a build queue to the ManagedService interface.
type BuildQueueService struct {
	queue BuildQueue
	name  string
}

// BuildQueue defines the interface expected by BuildQueueService.
type BuildQueue interface {
	Start(ctx context.Context)
	Stop(ctx context.Context) error
	IsActive() bool
	QueueLength() int
}

// NewBuildQueueService creates a new build queue service adapter.
func NewBuildQueueService(name string, queue BuildQueue) *BuildQueueService {
	return &BuildQueueService{
		queue: queue,
		name:  name,
	}
}

func (b *BuildQueueService) Name() string {
	return b.name
}

func (b *BuildQueueService) Start(ctx context.Context) error {
	b.queue.Start(ctx)
	return nil // Build queue Start method doesn't return error in current implementation
}

func (b *BuildQueueService) Stop(ctx context.Context) error {
	return b.queue.Stop(ctx)
}

func (b *BuildQueueService) Health() HealthStatus {
	if b.queue.IsActive() {
		queueLen := b.queue.QueueLength()
		if queueLen > 100 { // Arbitrary threshold
			return HealthStatusUnhealthy(fmt.Sprintf("queue overloaded: %d items", queueLen))
		}
		return HealthStatusHealthy
	}
	return HealthStatusUnhealthy("queue not active")
}

func (b *BuildQueueService) Dependencies() []string {
	return []string{} // Build queue typically has no service dependencies
}

// SchedulerService adapts a scheduler to the ManagedService interface.
type SchedulerService struct {
	scheduler Scheduler
	name      string
}

// Scheduler defines the interface expected by SchedulerService.
type Scheduler interface {
	Start(ctx context.Context)
	Stop(ctx context.Context) error
	IsRunning() bool
}

// NewSchedulerService creates a new scheduler service adapter.
func NewSchedulerService(name string, scheduler Scheduler) *SchedulerService {
	return &SchedulerService{
		scheduler: scheduler,
		name:      name,
	}
}

func (s *SchedulerService) Name() string {
	return s.name
}

func (s *SchedulerService) Start(ctx context.Context) error {
	s.scheduler.Start(ctx)
	return nil
}

func (s *SchedulerService) Stop(ctx context.Context) error {
	return s.scheduler.Stop(ctx)
}

func (s *SchedulerService) Health() HealthStatus {
	if s.scheduler.IsRunning() {
		return HealthStatusHealthy
	}
	return HealthStatusUnhealthy("scheduler not running")
}

func (s *SchedulerService) Dependencies() []string {
	return []string{"build-queue"} // Scheduler depends on build queue
}

// ConfigWatcherService adapts a config watcher to the ManagedService interface.
type ConfigWatcherService struct {
	watcher ConfigWatcher
	name    string
}

// ConfigWatcher defines the interface expected by ConfigWatcherService.
type ConfigWatcher interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsWatching() bool
}

// NewConfigWatcherService creates a new config watcher service adapter.
func NewConfigWatcherService(name string, watcher ConfigWatcher) *ConfigWatcherService {
	return &ConfigWatcherService{
		watcher: watcher,
		name:    name,
	}
}

func (c *ConfigWatcherService) Name() string {
	return c.name
}

func (c *ConfigWatcherService) Start(ctx context.Context) error {
	return c.watcher.Start(ctx)
}

func (c *ConfigWatcherService) Stop(ctx context.Context) error {
	return c.watcher.Stop(ctx)
}

func (c *ConfigWatcherService) Health() HealthStatus {
	if c.watcher.IsWatching() {
		return HealthStatusHealthy
	}
	return HealthStatusUnhealthy("not watching config file")
}

func (c *ConfigWatcherService) Dependencies() []string {
	return []string{} // Config watcher typically has no dependencies
}

// StateManagerService adapts a state manager to the ManagedService interface.
type StateManagerService struct {
	manager StateManager
	name    string
}

// StateManager defines the interface expected by StateManagerService.
type StateManager interface {
	Load() error
	Save() error
	IsLoaded() bool
	LastSaved() *time.Time
}

// NewStateManagerService creates a new state manager service adapter.
func NewStateManagerService(name string, manager StateManager) *StateManagerService {
	return &StateManagerService{
		manager: manager,
		name:    name,
	}
}

func (s *StateManagerService) Name() string {
	return s.name
}

func (s *StateManagerService) Start(ctx context.Context) error {
	// State manager "start" means loading persistent state
	return s.manager.Load()
}

func (s *StateManagerService) Stop(ctx context.Context) error {
	// State manager "stop" means saving current state
	return s.manager.Save()
}

func (s *StateManagerService) Health() HealthStatus {
	if s.manager.IsLoaded() {
		lastSaved := s.manager.LastSaved()
		if lastSaved != nil && time.Since(*lastSaved) > 10*time.Minute {
			return HealthStatusUnhealthy("state not saved recently")
		}
		return HealthStatusHealthy
	}
	return HealthStatusUnhealthy("state not loaded")
}

func (s *StateManagerService) Dependencies() []string {
	return []string{} // State manager typically loads independently
}
