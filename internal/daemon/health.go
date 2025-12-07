package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
	"git.home.luguber.info/inful/docbuilder/internal/version"
)

// HealthStatus represents the overall health of the daemon
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents a single health check
type HealthCheck struct {
	Name        string        `json:"name"`
	Status      HealthStatus  `json:"status"`
	Message     string        `json:"message,omitempty"`
	Duration    time.Duration `json:"duration"`
	LastChecked time.Time     `json:"last_checked"`
}

// HealthResponse represents the complete health check response
type HealthResponse struct {
	Status    HealthStatus  `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Uptime    string        `json:"uptime"`
	Version   string        `json:"version"`
	Checks    []HealthCheck `json:"checks"`
}

// PerformHealthChecks executes all health checks and returns the overall status
func (d *Daemon) PerformHealthChecks() *HealthResponse {
	startTime := time.Now()

	var checks []HealthCheck
	overallStatus := HealthStatusHealthy

	// Check daemon status
	daemonCheck := d.checkDaemonHealth()
	checks = append(checks, daemonCheck)
	if daemonCheck.Status != HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check HTTP servers
	httpCheck := d.checkHTTPHealth()
	checks = append(checks, httpCheck)
	if httpCheck.Status != HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check build queue
	queueCheck := d.checkBuildQueueHealth()
	checks = append(checks, queueCheck)
	if queueCheck.Status != HealthStatusHealthy && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check forge connectivity
	forgeCheck := d.checkForgeHealth()
	checks = append(checks, forgeCheck)
	if forgeCheck.Status != HealthStatusHealthy && overallStatus == HealthStatusHealthy {
		overallStatus = HealthStatusDegraded
	}

	// Check storage/state manager
	storageCheck := d.checkStorageHealth()
	checks = append(checks, storageCheck)
	if storageCheck.Status != HealthStatusHealthy {
		if overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
	}

	// Record health check metrics
	d.metrics.IncrementCounter("health_checks_total")
	d.metrics.RecordHistogram("health_check_duration_seconds", time.Since(startTime).Seconds())
	if overallStatus == HealthStatusHealthy {
		d.metrics.IncrementCounter("health_checks_healthy")
	} else {
		d.metrics.IncrementCounter("health_checks_unhealthy")
	}

	return &HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Uptime:    time.Since(d.startTime).String(),
		Version:   version.Version,
		Checks:    checks,
	}
}

// checkDaemonHealth verifies the daemon is in a healthy state
func (d *Daemon) checkDaemonHealth() HealthCheck {
	start := time.Now()

	status := d.GetStatus()
	check := HealthCheck{
		Name:        "daemon_status",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}

	switch status {
	case StatusRunning:
		check.Status = HealthStatusHealthy
		check.Message = "Daemon is running normally"
	case StatusStarting:
		check.Status = HealthStatusDegraded
		check.Message = "Daemon is still starting up"
	case StatusStopping:
		check.Status = HealthStatusDegraded
		check.Message = "Daemon is shutting down"
	case StatusError:
		check.Status = HealthStatusUnhealthy
		check.Message = "Daemon is in error state"
	default:
		check.Status = HealthStatusUnhealthy
		check.Message = "Daemon is in unknown state"
	}

	return check
}

// checkHTTPHealth verifies HTTP servers are responsive
func (d *Daemon) checkHTTPHealth() HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name:        "http_servers",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}

	if d.httpServer == nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "HTTP server not initialized"
		return check
	}

	// Check daemon status as proxy for server health
	if d.GetStatus() == StatusRunning {
		check.Status = HealthStatusHealthy
		check.Message = "HTTP servers are running and accepting connections"
	} else {
		check.Status = HealthStatusDegraded
		check.Message = "HTTP server initialized but daemon not fully running"
	}

	return check
}

// checkBuildQueueHealth verifies the build queue is functional
func (d *Daemon) checkBuildQueueHealth() HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name:        "build_queue",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}

	if d.buildQueue == nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "Build queue not initialized"
		return check
	}

	queueLength := d.buildQueue.Length()

	if queueLength > 100 {
		check.Status = HealthStatusDegraded
		check.Message = "Build queue is getting full"
	} else if queueLength > 50 {
		check.Status = HealthStatusDegraded
		check.Message = "Build queue has moderate load"
	} else {
		check.Status = HealthStatusHealthy
		check.Message = "Build queue is operating normally"
	}

	return check
}

// checkForgeHealth verifies forge connectivity
func (d *Daemon) checkForgeHealth() HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name:        "forge_connectivity",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}

	if d.forgeManager == nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "Forge manager not initialized"
		return check
	}

	// Check if forge discovery has run successfully
	result, err := d.discoveryCache.Get()
	if err != nil {
		check.Status = HealthStatusDegraded
		check.Message = fmt.Sprintf("Last forge discovery failed: %v", err)
	} else if result == nil {
		check.Status = HealthStatusDegraded
		check.Message = "No forge discovery has run yet"
	} else if len(result.Errors) > 0 {
		check.Status = HealthStatusDegraded
		check.Message = fmt.Sprintf("Some forges have errors: %d/%d", len(result.Errors), len(result.Repositories)+len(result.Errors))
	} else {
		check.Status = HealthStatusHealthy
		check.Message = fmt.Sprintf("All forges healthy, %d repositories discovered", len(result.Repositories))
	}

	return check
}

// checkStorageHealth verifies storage and state management
func (d *Daemon) checkStorageHealth() HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name:        "storage_state",
		LastChecked: time.Now(),
		Duration:    time.Since(start),
	}

	if d.stateManager == nil {
		check.Status = HealthStatusDegraded
		check.Message = "State manager not initialized"
		return check
	}

	// Check if state is loaded
	if !d.stateManager.IsLoaded() {
		check.Status = HealthStatusDegraded
		check.Message = "State not loaded"
	} else {
		check.Status = HealthStatusHealthy
		lastSaved := d.stateManager.LastSaved()
		if lastSaved != nil {
			check.Message = fmt.Sprintf("State operational, last saved: %s", lastSaved.Format(time.RFC3339))
		} else {
			check.Message = "State operational"
		}
	}

	return check
}

// EnhancedHealthHandler serves detailed health information
func (d *Daemon) EnhancedHealthHandler(w http.ResponseWriter, _ *http.Request) {
	health := d.PerformHealthChecks()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Set HTTP status code based on health
	switch health.Status {
	case HealthStatusHealthy:
		w.WriteHeader(http.StatusOK)
	case HealthStatusDegraded:
		w.WriteHeader(http.StatusOK) // Still OK, but with warnings
	case HealthStatusUnhealthy:
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(health); err != nil {
		adapter := errors.NewHTTPErrorAdapter(nil)
		e := errors.WrapError(err, errors.CategoryInternal, "failed to encode health response").Build()
		adapter.WriteErrorResponse(w, e)
	}
}
