package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// generateID creates a simple unique ID
func generateID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil { // extremely unlikely; fallback to timestamp string
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// ScheduleType represents different types of schedules
type ScheduleType string

const (
	ScheduleTypeCron     ScheduleType = "cron"
	ScheduleTypeInterval ScheduleType = "interval"
	ScheduleTypeOnce     ScheduleType = "once"
)

// Schedule represents a scheduled task
type Schedule struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       ScheduleType           `json:"type"`
	Expression string                 `json:"expression"` // Cron expression or interval
	Enabled    bool                   `json:"enabled"`
	LastRun    *time.Time             `json:"last_run,omitempty"`
	NextRun    *time.Time             `json:"next_run,omitempty"`
	RunCount   int64                  `json:"run_count"`
	ErrorCount int64                  `json:"error_count"`
	LastError  string                 `json:"last_error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// NOTE: A full cron expression struct/parser was previously stubbed here but removed
// due to being unused. The simplified validator + common pattern matcher remain.

// Scheduler manages scheduled tasks and executes them at the appropriate times
type Scheduler struct {
	schedules  map[string]*Schedule
	mu         sync.RWMutex
	ticker     *time.Ticker
	stopChan   chan struct{}
	wg         sync.WaitGroup
	buildQueue *BuildQueue
}

// NewScheduler creates a new scheduler instance
func NewScheduler(buildQueue *BuildQueue) *Scheduler {
	return &Scheduler{
		schedules:  make(map[string]*Schedule),
		buildQueue: buildQueue,
		stopChan:   make(chan struct{}),
	}
}

// Start begins the scheduler's main loop
func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("Starting scheduler")

	// Check schedules every minute
	s.ticker = time.NewTicker(time.Minute)

	s.wg.Add(1)
	go s.schedulerLoop(ctx)
}

// Stop gracefully shuts down the scheduler
func (s *Scheduler) Stop(ctx context.Context) {
	slog.Info("Stopping scheduler")

	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.stopChan)
	s.wg.Wait()

	slog.Info("Scheduler stopped")
}

// AddSchedule adds a new schedule
func (s *Scheduler) AddSchedule(schedule *Schedule) error {
	if schedule == nil {
		return fmt.Errorf("schedule cannot be nil")
	}

	if schedule.ID == "" {
		// Generate a simple UUID-like string
		schedule.ID = generateID()
	}

	if schedule.Name == "" {
		return fmt.Errorf("schedule name is required")
	}

	// Validate the schedule expression
	if err := s.validateSchedule(schedule); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	// Calculate next run time
	if err := s.calculateNextRun(schedule); err != nil {
		return fmt.Errorf("failed to calculate next run: %w", err)
	}

	s.mu.Lock()
	s.schedules[schedule.ID] = schedule
	s.mu.Unlock()

	slog.Info("Schedule added", logfields.ScheduleID(schedule.ID), logfields.ScheduleName(schedule.Name), slog.Any("next_run", schedule.NextRun))
	return nil
}

// RemoveSchedule removes a schedule by ID
func (s *Scheduler) RemoveSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.schedules[id]; !exists {
		return fmt.Errorf("schedule not found: %s", id)
	}

	delete(s.schedules, id)
	slog.Info("Schedule removed", logfields.ScheduleID(id))
	return nil
}

// GetSchedule returns a schedule by ID
func (s *Scheduler) GetSchedule(id string) (*Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, exists := s.schedules[id]
	if !exists {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	return schedule, nil
}

// ListSchedules returns all schedules
func (s *Scheduler) ListSchedules() []*Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedules := make([]*Schedule, 0, len(s.schedules))
	for _, schedule := range s.schedules {
		schedules = append(schedules, schedule)
	}

	return schedules
}

// EnableSchedule enables a schedule
func (s *Scheduler) EnableSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule, exists := s.schedules[id]
	if !exists {
		return fmt.Errorf("schedule not found: %s", id)
	}

	schedule.Enabled = true
	if err := s.calculateNextRun(schedule); err != nil {
		schedule.Enabled = false // disable if we cannot compute next run
		slog.Error("Failed to enable schedule (next run calc)", logfields.ScheduleID(id), logfields.Error(err))
		return fmt.Errorf("failed to calculate next run: %w", err)
	}

	slog.Info("Schedule enabled", logfields.ScheduleID(id), slog.Any("next_run", schedule.NextRun))
	return nil
}

// DisableSchedule disables a schedule
func (s *Scheduler) DisableSchedule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule, exists := s.schedules[id]
	if !exists {
		return fmt.Errorf("schedule not found: %s", id)
	}

	schedule.Enabled = false
	schedule.NextRun = nil

	slog.Info("Schedule disabled", logfields.ScheduleID(id))
	return nil
}

// schedulerLoop is the main scheduler loop that checks for due schedules
func (s *Scheduler) schedulerLoop(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			slog.Debug("Scheduler loop stopped by context")
			return
		case <-s.stopChan:
			slog.Debug("Scheduler loop stopped by stop signal")
			return
		case <-s.ticker.C:
			s.checkSchedules()
		}
	}
}

// checkSchedules examines all schedules and executes those that are due
func (s *Scheduler) checkSchedules() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, schedule := range s.schedules {
		if !schedule.Enabled || schedule.NextRun == nil {
			continue
		}

		if schedule.NextRun.Before(now) || schedule.NextRun.Equal(now) {
			s.executeSchedule(schedule, now)
		}
	}
}

// executeSchedule executes a scheduled task
func (s *Scheduler) executeSchedule(schedule *Schedule, now time.Time) {
	slog.Info("Executing scheduled task", logfields.ScheduleID(schedule.ID), logfields.ScheduleName(schedule.Name))

	// Update last run time
	schedule.LastRun = &now
	schedule.RunCount++

	// Create a build job for this scheduled task
	job := &BuildJob{
		ID:       fmt.Sprintf("schedule-%s-%d", schedule.ID, now.Unix()),
		Type:     BuildTypeScheduled,
		Priority: PriorityNormal,
		Metadata: map[string]interface{}{
			"schedule_id":   schedule.ID,
			"schedule_name": schedule.Name,
		},
	}

	// Enqueue the build job
	if err := s.buildQueue.Enqueue(job); err != nil {
		schedule.ErrorCount++
		schedule.LastError = fmt.Sprintf("Failed to enqueue build job: %v", err)
			slog.Error("Failed to enqueue scheduled build", logfields.ScheduleID(schedule.ID), logfields.Error(err))
	}

	// Calculate next run time
	if err := s.calculateNextRun(schedule); err != nil {
		schedule.ErrorCount++
		schedule.LastError = fmt.Sprintf("Failed to calculate next run: %v", err)
			slog.Error("Failed to calculate next run for schedule", logfields.ScheduleID(schedule.ID), logfields.Error(err))
		// Disable the schedule if we can't calculate the next run
		schedule.Enabled = false
	}
}

// validateSchedule validates a schedule's configuration
func (s *Scheduler) validateSchedule(schedule *Schedule) error {
	switch schedule.Type {
	case ScheduleTypeCron:
		return s.validateCronExpression(schedule.Expression)
	case ScheduleTypeInterval:
		return s.validateIntervalExpression(schedule.Expression)
	case ScheduleTypeOnce:
		return s.validateOnceExpression(schedule.Expression)
	default:
		return fmt.Errorf("unsupported schedule type: %s", schedule.Type)
	}
}

// validateCronExpression validates a cron expression (simplified version)
func (s *Scheduler) validateCronExpression(expression string) error {
	parts := strings.Fields(expression)
	if len(parts) != 5 {
		return fmt.Errorf("cron expression must have 5 parts (minute hour day month weekday), got %d", len(parts))
	}

	// Basic validation - could be more sophisticated
	for i, part := range parts {
		if part == "*" {
			continue
		}

		// Validate ranges for each field
		switch i {
		case 0: // minute (0-59)
			if err := s.validateCronField(part, 0, 59, "minute"); err != nil {
				return err
			}
		case 1: // hour (0-23)
			if err := s.validateCronField(part, 0, 23, "hour"); err != nil {
				return err
			}
		case 2: // day (1-31)
			if err := s.validateCronField(part, 1, 31, "day"); err != nil {
				return err
			}
		case 3: // month (1-12)
			if err := s.validateCronField(part, 1, 12, "month"); err != nil {
				return err
			}
		case 4: // weekday (0-6)
			if err := s.validateCronField(part, 0, 6, "weekday"); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateCronField validates a single field in a cron expression
func (s *Scheduler) validateCronField(field string, min, max int, fieldName string) error {
	// This is a simplified validation - could support ranges, lists, steps
	if strings.Contains(field, ",") || strings.Contains(field, "-") || strings.Contains(field, "/") {
		// Complex expressions - would need more sophisticated parsing
		return nil
	}

	// Single number validation would go here
	return nil
}

// validateIntervalExpression validates an interval expression (e.g., "5m", "1h", "30s")
func (s *Scheduler) validateIntervalExpression(expression string) error {
	_, err := time.ParseDuration(expression)
	if err != nil {
		return fmt.Errorf("invalid interval duration: %v", err)
	}
	return nil
}

// validateOnceExpression validates a one-time schedule expression (RFC3339 timestamp)
func (s *Scheduler) validateOnceExpression(expression string) error {
	_, err := time.Parse(time.RFC3339, expression)
	if err != nil {
		return fmt.Errorf("invalid timestamp format, use RFC3339: %v", err)
	}
	return nil
}

// calculateNextRun calculates the next run time for a schedule
func (s *Scheduler) calculateNextRun(schedule *Schedule) error {
	now := time.Now()

	switch schedule.Type {
	case ScheduleTypeCron:
		nextRun, err := s.calculateNextCronRun(schedule.Expression, now)
		if err != nil {
			return err
		}
		schedule.NextRun = &nextRun

	case ScheduleTypeInterval:
		duration, err := time.ParseDuration(schedule.Expression)
		if err != nil {
			return fmt.Errorf("invalid interval: %v", err)
		}

		var nextRun time.Time
		if schedule.LastRun != nil {
			nextRun = schedule.LastRun.Add(duration)
		} else {
			nextRun = now.Add(duration)
		}
		schedule.NextRun = &nextRun

	case ScheduleTypeOnce:
		targetTime, err := time.Parse(time.RFC3339, schedule.Expression)
		if err != nil {
			return fmt.Errorf("invalid timestamp: %v", err)
		}

		if targetTime.Before(now) {
			// One-time schedule in the past - disable it
			schedule.Enabled = false
			schedule.NextRun = nil
		} else {
			schedule.NextRun = &targetTime
		}

	default:
		return fmt.Errorf("unsupported schedule type: %s", schedule.Type)
	}

	return nil
}

// calculateNextCronRun calculates the next run time for a cron expression (simplified)
func (s *Scheduler) calculateNextCronRun(expression string, from time.Time) (time.Time, error) {
	// This is a simplified cron implementation
	// A production system would use a proper cron parsing library

	parts := strings.Fields(expression)
	if len(parts) != 5 {
		return time.Time{}, fmt.Errorf("invalid cron expression")
	}

	// For simplicity, handle some common patterns
	switch expression {
	case "0 0 * * *": // Daily at midnight
		next := time.Date(from.Year(), from.Month(), from.Day()+1, 0, 0, 0, 0, from.Location())
		return next, nil
	case "0 * * * *": // Every hour
		next := from.Add(time.Hour).Truncate(time.Hour)
		return next, nil
	case "*/5 * * * *": // Every 5 minutes
		next := from.Add(5 * time.Minute).Truncate(5 * time.Minute)
		return next, nil
	case "*/15 * * * *": // Every 15 minutes
		next := from.Add(15 * time.Minute).Truncate(15 * time.Minute)
		return next, nil
	case "0 0 * * 0": // Weekly on Sunday at midnight
		daysUntilSunday := (7 - int(from.Weekday())) % 7
		if daysUntilSunday == 0 && (from.Hour() > 0 || from.Minute() > 0 || from.Second() > 0) {
			daysUntilSunday = 7
		}
		next := time.Date(from.Year(), from.Month(), from.Day()+daysUntilSunday, 0, 0, 0, 0, from.Location())
		return next, nil
	default:
		// For unknown patterns, default to 1 hour from now
		// In a production system, you'd implement full cron parsing
		return from.Add(time.Hour), nil
	}
}
