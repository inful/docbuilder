package tenant

import (
	"context"
	"testing"
)

func TestWithTenant(t *testing.T) {
	ctx := context.Background()
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test Tenant",
	}

	ctx = WithTenant(ctx, tenant)
	retrieved, err := FromContext(ctx)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if retrieved.ID != "tenant-1" {
		t.Errorf("expected ID tenant-1, got %s", retrieved.ID)
	}
}

func TestFromContextNoTenant(t *testing.T) {
	ctx := context.Background()
	_, err := FromContext(ctx)

	if err != ErrNoTenant {
		t.Errorf("expected ErrNoTenant, got %v", err)
	}
}

func TestMockStoreCreate(t *testing.T) {
	store := NewMockStore()
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test Tenant",
		Plan: "pro",
	}

	if err := store.CreateTenant(tenant); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	retrieved, err := store.GetTenant("tenant-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if retrieved.Name != "Test Tenant" {
		t.Errorf("expected name Test Tenant, got %s", retrieved.Name)
	}
}

func TestMockStoreCreateInvalidID(t *testing.T) {
	store := NewMockStore()
	tenant := &Tenant{
		ID:   "",
		Name: "Invalid Tenant",
	}

	if err := store.CreateTenant(tenant); err != ErrInvalidTenant {
		t.Errorf("expected ErrInvalidTenant, got %v", err)
	}
}

func TestMockStoreCreateDuplicate(t *testing.T) {
	store := NewMockStore()
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test Tenant",
	}

	if err := store.CreateTenant(tenant); err != nil {
		t.Fatalf("failed to create first tenant: %v", err)
	}

	if err := store.CreateTenant(tenant); err == nil {
		t.Error("expected error creating duplicate tenant")
	}
}

func TestMockStoreGet(t *testing.T) {
	store := NewMockStore()
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test Tenant",
	}

	if err := store.CreateTenant(tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	retrieved, err := store.GetTenant("tenant-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if retrieved.ID != "tenant-1" {
		t.Errorf("expected ID tenant-1, got %s", retrieved.ID)
	}
}

func TestMockStoreGetNotFound(t *testing.T) {
	store := NewMockStore()
	_, err := store.GetTenant("nonexistent")

	if err != ErrNoTenant {
		t.Errorf("expected ErrNoTenant, got %v", err)
	}
}

func TestMockStoreUpdate(t *testing.T) {
	store := NewMockStore()
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test Tenant",
		Plan: "free",
	}

	if err := store.CreateTenant(tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	tenant.Plan = "pro"
	if err := store.UpdateTenant(tenant); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	retrieved, _ := store.GetTenant("tenant-1")
	if retrieved.Plan != "pro" {
		t.Errorf("expected plan pro, got %s", retrieved.Plan)
	}
}

func TestMockStoreDelete(t *testing.T) {
	store := NewMockStore()
	tenant := &Tenant{
		ID:   "tenant-1",
		Name: "Test Tenant",
	}

	if err := store.CreateTenant(tenant); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	if err := store.DeleteTenant("tenant-1"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err := store.GetTenant("tenant-1")
	if err != ErrNoTenant {
		t.Errorf("expected ErrNoTenant after delete, got %v", err)
	}
}

func TestMockStoreList(t *testing.T) {
	store := NewMockStore()

	for i := 1; i <= 3; i++ {
		tenant := &Tenant{
			ID:   genTenantID(i),
			Name: "Tenant " + string(rune(i)),
		}
		if err := store.CreateTenant(tenant); err != nil {
			t.Fatalf("failed to create tenant: %v", err)
		}
	}

	tenants, err := store.ListTenants()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tenants) != 3 {
		t.Errorf("expected 3 tenants, got %d", len(tenants))
	}
}

// Helper function
func genTenantID(i int) string {
	switch i {
	case 1:
		return "tenant-001"
	case 2:
		return "tenant-002"
	case 3:
		return "tenant-003"
	default:
		return "tenant-unknown"
	}
}
