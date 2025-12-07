package quota

import (
	"testing"
	"time"
)

func TestPlanQuotasExist(t *testing.T) {
	for _, plan := range []string{"free", "pro", "enterprise"} {
		if _, ok := PlanQuotas[plan]; !ok {
			t.Errorf("plan %s not found in PlanQuotas", plan)
		}
	}
}

func TestFreeQuotasAreRestrictive(t *testing.T) {
	free := PlanQuotas["free"]
	pro := PlanQuotas["pro"]

	if free.MaxConcurrentBuilds >= pro.MaxConcurrentBuilds {
		t.Error("free tier should have fewer concurrent builds than pro")
	}

	if free.MaxBuildsPerDay >= pro.MaxBuildsPerDay {
		t.Error("free tier should have fewer builds per day than pro")
	}

	if free.MaxStorageBytes >= pro.MaxStorageBytes {
		t.Error("free tier should have less storage than pro")
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager, got nil")
	}
	if len(m.quotas) != 0 {
		t.Error("expected empty quotas")
	}
	if len(m.usage) != 0 {
		t.Error("expected empty usage")
	}
}

func TestSetQuota(t *testing.T) {
	m := NewManager()
	quotas := PlanQuotas["pro"]

	m.SetQuota("tenant-1", quotas)

	retrieved, ok := m.GetQuota("tenant-1")
	if !ok {
		t.Fatal("quota not found")
	}

	if retrieved.MaxConcurrentBuilds != quotas.MaxConcurrentBuilds {
		t.Errorf("expected %d, got %d", quotas.MaxConcurrentBuilds, retrieved.MaxConcurrentBuilds)
	}
}

func TestCanCreateBuildSuccess(t *testing.T) {
	m := NewManager()
	m.SetQuota("tenant-1", PlanQuotas["pro"])

	err := m.CanCreateBuild("tenant-1")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCanCreateBuildConcurrentLimit(t *testing.T) {
	m := NewManager()
	quotas := PlanQuotas["free"] // free has max 1 concurrent build
	m.SetQuota("tenant-1", quotas)

	// Create first build
	if err := m.IncrementBuild("tenant-1"); err != nil {
		t.Fatalf("failed to create first build: %v", err)
	}

	// Try to create second build (should fail)
	err := m.CanCreateBuild("tenant-1")
	if err == nil {
		t.Error("expected error for exceeding concurrent build limit")
	}

	if quotaErr, ok := err.(*QuotaLimitError); ok {
		if quotaErr.Limit != "concurrent builds" {
			t.Errorf("expected 'concurrent builds' limit, got %s", quotaErr.Limit)
		}
	}
}

func TestIncrementBuild(t *testing.T) {
	m := NewManager()
	m.SetQuota("tenant-1", PlanQuotas["pro"])

	if err := m.IncrementBuild("tenant-1"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	usage := m.GetUsage("tenant-1")
	if usage.ConcurrentBuilds != 1 {
		t.Errorf("expected 1 concurrent build, got %d", usage.ConcurrentBuilds)
	}

	if usage.BuildsToday != 1 {
		t.Errorf("expected 1 build today, got %d", usage.BuildsToday)
	}
}

func TestDecrementBuild(t *testing.T) {
	m := NewManager()
	m.SetQuota("tenant-1", PlanQuotas["pro"])

	m.IncrementBuild("tenant-1")
	m.DecrementBuild("tenant-1")

	usage := m.GetUsage("tenant-1")
	if usage.ConcurrentBuilds != 0 {
		t.Errorf("expected 0 concurrent builds, got %d", usage.ConcurrentBuilds)
	}
}

func TestAddStorageSuccess(t *testing.T) {
	m := NewManager()
	m.SetQuota("tenant-1", PlanQuotas["pro"])

	err := m.AddStorage("tenant-1", 100)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	usage := m.GetUsage("tenant-1")
	if usage.StorageUsedBytes != 100 {
		t.Errorf("expected 100 bytes, got %d", usage.StorageUsedBytes)
	}
}

func TestAddStorageExceedsLimit(t *testing.T) {
	m := NewManager()
	quotas := ResourceQuotas{
		MaxConcurrentBuilds: 10,
		MaxBuildsPerDay:     100,
		MaxStorageBytes:     1000,
		MaxQueueSize:        10,
		RateLimitPerHour:    10,
	}
	m.SetQuota("tenant-1", quotas)

	// Add storage that exceeds limit
	err := m.AddStorage("tenant-1", 2000)
	if err == nil {
		t.Error("expected error for exceeding storage limit")
	}

	if quotaErr, ok := err.(*QuotaLimitError); ok {
		if quotaErr.Limit != "storage" {
			t.Errorf("expected 'storage' limit, got %s", quotaErr.Limit)
		}
	}
}

func TestGetUsageReturnsSnapshot(t *testing.T) {
	m := NewManager()
	m.SetQuota("tenant-1", PlanQuotas["pro"])
	m.IncrementBuild("tenant-1")

	usage := m.GetUsage("tenant-1")
	if usage == nil {
		t.Fatal("expected usage, got nil")
	}

	if usage.ConcurrentBuilds != 1 {
		t.Errorf("expected 1 concurrent build, got %d", usage.ConcurrentBuilds)
	}

	if usage.TenantID != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", usage.TenantID)
	}
}

func TestHourlyRateLimit(t *testing.T) {
	m := NewManager()
	quotas := ResourceQuotas{
		MaxConcurrentBuilds: 100,
		MaxBuildsPerDay:     1000,
		MaxStorageBytes:     10000,
		MaxQueueSize:        100,
		RateLimitPerHour:    2,
	}
	m.SetQuota("tenant-1", quotas)

	// Create two builds and verify the second succeeds
	for i := 0; i < 2; i++ {
		if err := m.IncrementBuild("tenant-1"); err != nil {
			t.Fatalf("build %d failed: %v", i, err)
		}
	}

	usage := m.GetUsage("tenant-1")
	if usage.BuildsThisHour != 2 {
		t.Errorf("expected 2 builds this hour, got %d", usage.BuildsThisHour)
	}
}

func TestQuotaLimitError(t *testing.T) {
	err := &QuotaLimitError{
		Limit:      "concurrent builds",
		Current:    5,
		Maximum:    3,
		RetryAfter: 1 * time.Minute,
	}

	errStr := err.Error()
	if errStr != "quota limit exceeded: concurrent builds" {
		t.Errorf("unexpected error string: %s", errStr)
	}

	if err.RetryAfter != 1*time.Minute {
		t.Errorf("expected 1 minute retry after, got %v", err.RetryAfter)
	}
}

func TestDeleteTenant(t *testing.T) {
	m := NewManager()
	m.SetQuota("tenant-1", PlanQuotas["pro"])

	m.DeleteTenant("tenant-1")

	usage := m.GetUsage("tenant-1")
	if usage != nil {
		t.Error("expected nil usage after delete")
	}

	_, ok := m.GetQuota("tenant-1")
	if ok {
		t.Error("expected no quota after delete")
	}
}

func TestMultipleTenants(t *testing.T) {
	m := NewManager()
	m.SetQuota("tenant-1", PlanQuotas["free"])
	m.SetQuota("tenant-2", PlanQuotas["pro"])

	m.IncrementBuild("tenant-1")
	m.IncrementBuild("tenant-2")
	m.IncrementBuild("tenant-2")

	usage1 := m.GetUsage("tenant-1")
	usage2 := m.GetUsage("tenant-2")

	if usage1.ConcurrentBuilds != 1 {
		t.Errorf("tenant-1: expected 1 concurrent build, got %d", usage1.ConcurrentBuilds)
	}

	if usage2.ConcurrentBuilds != 2 {
		t.Errorf("tenant-2: expected 2 concurrent builds, got %d", usage2.ConcurrentBuilds)
	}
}
