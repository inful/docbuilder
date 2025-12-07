package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/tenant"
)

func TestExtractTenantIDFromHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Tenant-ID", "tenant-123")

	tenantID := extractTenantID(req)
	if tenantID != "tenant-123" {
		t.Errorf("expected tenant-123, got %s", tenantID)
	}
}

func TestExtractTenantIDFromQueryParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/health?tenant-id=tenant-456", nil)

	tenantID := extractTenantID(req)
	if tenantID != "tenant-456" {
		t.Errorf("expected tenant-456, got %s", tenantID)
	}
}

func TestExtractTenantIDFromLowercaseHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("x-tenant-id", "tenant-789")

	tenantID := extractTenantID(req)
	if tenantID != "tenant-789" {
		t.Errorf("expected tenant-789, got %s", tenantID)
	}
}

func TestExtractTenantIDHeaderPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "/health?tenant-id=from-query", nil)
	req.Header.Set("X-Tenant-ID", "from-header")

	tenantID := extractTenantID(req)
	if tenantID != "from-header" {
		t.Errorf("expected from-header (header has priority), got %s", tenantID)
	}
}

func TestTenantMiddlewareMissingTenantID(t *testing.T) {
	store := tenant.NewMockStore()
	middleware := TenantMiddleware(store)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	// No tenant ID header or query param
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestTenantMiddlewareTenantNotFound(t *testing.T) {
	store := tenant.NewMockStore()
	middleware := TenantMiddleware(store)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Tenant-ID", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestTenantMiddlewareSuccess(t *testing.T) {
	store := tenant.NewMockStore()
	testTenant := &tenant.Tenant{
		ID:   "tenant-1",
		Name: "Test Tenant",
		Plan: "pro",
	}
	if err := store.CreateTenant(testTenant); err != nil {
		t.Fatalf("failed to create test tenant: %v", err)
	}

	middleware := TenantMiddleware(store)

	handlerCalled := false
	contextTenantID := ""

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		t, err := tenant.FromContext(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		contextTenantID = t.ID
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/builds", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if contextTenantID != "tenant-1" {
		t.Errorf("expected tenant-1 in context, got %s", contextTenantID)
	}
}

func TestValidateTenantInContext(t *testing.T) {
	ctx := tenant.WithTenant(context.Background(), &tenant.Tenant{
		ID:   "tenant-1",
		Name: "Test",
	})

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

	tnt, err := ValidateTenantInContext(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if tnt.ID != "tenant-1" {
		t.Errorf("expected ID tenant-1, got %s", tnt.ID)
	}
}
