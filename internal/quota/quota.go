package quota

import (
	"errors"
	"sync"
	"time"
)

// QuotaLimitError indicates a quota limit has been exceeded
type QuotaLimitError struct {
	Limit      string
	Current    int64
	Maximum    int64
	RetryAfter time.Duration
}

// Error implements the error interface
func (e *QuotaLimitError) Error() string {
	return "quota limit exceeded: " + e.Limit
}

// ResourceQuotas defines resource limits for a tenant
type ResourceQuotas struct {
	MaxConcurrentBuilds int64     // Maximum concurrent builds allowed
	MaxBuildsPerDay     int64     // Maximum builds per 24 hours
	MaxStorageBytes     int64     // Maximum storage in bytes
	MaxQueueSize        int64     // Maximum items in queue
	RateLimitPerHour    int64     // Maximum builds per hour
	ResetTime           time.Time // When daily limits reset
	DefaultPlan         string    // free, pro, enterprise
}

// PlanQuotas provides preset quotas for different tiers
var PlanQuotas = map[string]ResourceQuotas{
	"free": {
		MaxConcurrentBuilds: 1,
		MaxBuildsPerDay:     5,
		MaxStorageBytes:     1024 * 1024 * 100, // 100 MB
		MaxQueueSize:        10,
		RateLimitPerHour:    2,
	},
	"pro": {
		MaxConcurrentBuilds: 5,
		MaxBuildsPerDay:     100,
		MaxStorageBytes:     1024 * 1024 * 1024, // 1 GB
		MaxQueueSize:        100,
		RateLimitPerHour:    20,
	},
	"enterprise": {
		MaxConcurrentBuilds: 50,
		MaxBuildsPerDay:     1000,
		MaxStorageBytes:     10 * 1024 * 1024 * 1024, // 10 GB
		MaxQueueSize:        1000,
		RateLimitPerHour:    200,
	},
}

// TenantUsage tracks current resource usage for a tenant
type TenantUsage struct {
	TenantID          string
	ConcurrentBuilds  int64
	BuildsToday       int64
	StorageUsedBytes  int64
	QueueSize         int64
	BuildsThisHour    int64
	LastResetTime     time.Time
	LastHourResetTime time.Time
	mu                sync.RWMutex
}

// Manager manages quotas and usage for all tenants
type Manager struct {
	quotas map[string]ResourceQuotas
	usage  map[string]*TenantUsage
	mu     sync.RWMutex
}

// NewManager creates a new quota manager
func NewManager() *Manager {
	return &Manager{
		quotas: make(map[string]ResourceQuotas),
		usage:  make(map[string]*TenantUsage),
	}
}

// SetQuota sets quotas for a tenant
func (m *Manager) SetQuota(tenantID string, quotas ResourceQuotas) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.quotas[tenantID] = quotas

	// Initialize usage if not present
	if _, ok := m.usage[tenantID]; !ok {
		m.usage[tenantID] = &TenantUsage{
			TenantID:          tenantID,
			LastResetTime:     time.Now(),
			LastHourResetTime: time.Now(),
		}
	}
}

// GetQuota retrieves quotas for a tenant
func (m *Manager) GetQuota(tenantID string) (ResourceQuotas, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quotas, ok := m.quotas[tenantID]
	return quotas, ok
}

// CanCreateBuild checks if a tenant can create a new build
func (m *Manager) CanCreateBuild(tenantID string) error {
	m.mu.RLock()
	quotas, ok := m.quotas[tenantID]
	usage, hasUsage := m.usage[tenantID]
	m.mu.RUnlock()

	if !ok {
		return errors.New("quota not found for tenant")
	}

	if !hasUsage {
		return errors.New("usage not found for tenant")
	}

	usage.mu.RLock()
	defer usage.mu.RUnlock()

	// Reset daily counters if needed
	if time.Since(usage.LastResetTime) > 24*time.Hour {
		usage.mu.RUnlock()
		m.resetDailyLimits(tenantID)
		usage.mu.RLock()
	}

	// Reset hourly counters if needed
	if time.Since(usage.LastHourResetTime) > time.Hour {
		usage.mu.RUnlock()
		m.resetHourlyLimits(tenantID)
		usage.mu.RLock()
	}

	// Check concurrent builds
	if usage.ConcurrentBuilds >= quotas.MaxConcurrentBuilds {
		return &QuotaLimitError{
			Limit:      "concurrent builds",
			Current:    usage.ConcurrentBuilds,
			Maximum:    quotas.MaxConcurrentBuilds,
			RetryAfter: 1 * time.Minute,
		}
	}

	// Check daily builds
	if usage.BuildsToday >= quotas.MaxBuildsPerDay {
		return &QuotaLimitError{
			Limit:      "builds per day",
			Current:    usage.BuildsToday,
			Maximum:    quotas.MaxBuildsPerDay,
			RetryAfter: time.Until(usage.LastResetTime.Add(24 * time.Hour)),
		}
	}

	// Check hourly rate limit
	if usage.BuildsThisHour >= quotas.RateLimitPerHour {
		return &QuotaLimitError{
			Limit:      "builds per hour",
			Current:    usage.BuildsThisHour,
			Maximum:    quotas.RateLimitPerHour,
			RetryAfter: time.Until(usage.LastHourResetTime.Add(time.Hour)),
		}
	}

	return nil
}

// IncrementBuild increments build counters
func (m *Manager) IncrementBuild(tenantID string) error {
	if err := m.CanCreateBuild(tenantID); err != nil {
		return err
	}

	m.mu.RLock()
	usage, ok := m.usage[tenantID]
	m.mu.RUnlock()

	if !ok {
		return errors.New("usage not found for tenant")
	}

	usage.mu.Lock()
	defer usage.mu.Unlock()

	usage.ConcurrentBuilds++
	usage.BuildsToday++
	usage.BuildsThisHour++

	return nil
}

// DecrementBuild decrements concurrent build count
func (m *Manager) DecrementBuild(tenantID string) {
	m.mu.RLock()
	usage, ok := m.usage[tenantID]
	m.mu.RUnlock()

	if !ok {
		return
	}

	usage.mu.Lock()
	defer usage.mu.Unlock()

	if usage.ConcurrentBuilds > 0 {
		usage.ConcurrentBuilds--
	}
}

// AddStorage adds to storage usage
func (m *Manager) AddStorage(tenantID string, bytes int64) error {
	m.mu.RLock()
	quotas, ok := m.quotas[tenantID]
	usage, hasUsage := m.usage[tenantID]
	m.mu.RUnlock()

	if !ok || !hasUsage {
		return errors.New("quota or usage not found")
	}

	usage.mu.Lock()
	defer usage.mu.Unlock()

	newTotal := usage.StorageUsedBytes + bytes
	if newTotal > quotas.MaxStorageBytes {
		return &QuotaLimitError{
			Limit:   "storage",
			Current: newTotal,
			Maximum: quotas.MaxStorageBytes,
		}
	}

	usage.StorageUsedBytes = newTotal
	return nil
}

// GetUsage retrieves current usage for a tenant
func (m *Manager) GetUsage(tenantID string) *TenantUsage {
	m.mu.RLock()
	usage, ok := m.usage[tenantID]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	usage.mu.RLock()
	// Return a copy to avoid race conditions
	copy := &TenantUsage{
		TenantID:          usage.TenantID,
		ConcurrentBuilds:  usage.ConcurrentBuilds,
		BuildsToday:       usage.BuildsToday,
		StorageUsedBytes:  usage.StorageUsedBytes,
		QueueSize:         usage.QueueSize,
		BuildsThisHour:    usage.BuildsThisHour,
		LastResetTime:     usage.LastResetTime,
		LastHourResetTime: usage.LastHourResetTime,
	}
	usage.mu.RUnlock()

	return copy
}

// resetDailyLimits resets daily counters for a tenant
func (m *Manager) resetDailyLimits(tenantID string) {
	m.mu.RLock()
	usage, ok := m.usage[tenantID]
	m.mu.RUnlock()

	if !ok {
		return
	}

	usage.mu.Lock()
	defer usage.mu.Unlock()

	usage.BuildsToday = 0
	usage.LastResetTime = time.Now()
}

// resetHourlyLimits resets hourly counters for a tenant
func (m *Manager) resetHourlyLimits(tenantID string) {
	m.mu.RLock()
	usage, ok := m.usage[tenantID]
	m.mu.RUnlock()

	if !ok {
		return
	}

	usage.mu.Lock()
	defer usage.mu.Unlock()

	usage.BuildsThisHour = 0
	usage.LastHourResetTime = time.Now()
}

// DeleteTenant removes usage tracking for a tenant
func (m *Manager) DeleteTenant(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.quotas, tenantID)
	delete(m.usage, tenantID)
}
