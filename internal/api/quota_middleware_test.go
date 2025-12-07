package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/quota"
	"git.home.luguber.info/inful/docbuilder/internal/tenant"
)

func TestQuotaMiddlewareNoTenant(t *testing.T) {
	manager := quota.NewManager()
	middleware := QuotaMiddleware(manager)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/builds", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestQuotaMiddlewareNonBuildRequest(t *testing.T) {
	manager := quota.NewManager()
	manager.SetQuota("tenant-1", quota.PlanQuotas["pro"])

	middleware := QuotaMiddleware(manager)

	handlerCalled := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	tnt := &tenant.Tenant{
		ID:   "tenant-1",
		Name: "Test",
	}
	ctx := tenant.WithTenant(context.Background(), tnt)

	req := httptest.NewRequest("GET", "/health", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called for non-build request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestQuotaMiddlewareCreateBuildAllowed(t *testing.T) {
	manager := quota.NewManager()
	manager.SetQuota("tenant-1", quota.PlanQuotas["pro"])

	middleware := QuotaMiddleware(manager)

	handlerCalled := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	}))

	tnt := &tenant.Tenant{
		ID:   "tenant-1",
		Name: "Test",
	}
	ctx := tenant.WithTenant(context.Background(), tnt)

	req := httptest.NewRequest("POST", "/builds", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("expected handler to be called when quota allows")
	}

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	// Verify quota was incremented
	usage := manager.GetUsage("tenant-1")
	if usage.ConcurrentBuilds != 1 {
		t.Errorf("expected 1 concurrent build, got %d", usage.ConcurrentBuilds)
	}
}

func TestQuotaMiddlewareCreateBuildDenied(t *testing.T) {
	manager := quota.NewManager()
	freePlan := quota.PlanQuotas["free"]
	manager.SetQuota("tenant-1", freePlan)

	// Use up the concurrent build limit
	manager.IncrementBuild("tenant-1")

	middleware := QuotaMiddleware(manager)

	handlerCalled := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	}))

	tnt := &tenant.Tenant{
		ID:   "tenant-1",
		Name: "Test",
	}
	ctx := tenant.WithTenant(context.Background(), tnt)

	req := httptest.NewRequest("POST", "/builds", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if handlerCalled {
		t.Error("expected handler not to be called when quota exceeded")
	}

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
}

func TestQuotaStatusHandler(t *testing.T) {
	manager := quota.NewManager()
	manager.SetQuota("tenant-1", quota.PlanQuotas["pro"])
	manager.IncrementBuild("tenant-1")

	handler := QuotaStatusHandler(manager)

	tnt := &tenant.Tenant{
		ID:   "tenant-1",
		Name: "Test",
	}
	ctx := tenant.WithTenant(context.Background(), tnt)

	req := httptest.NewRequest("GET", "/quota/status", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success=true")
	}

	if resp.Data == nil {
		t.Error("expected data in response")
	}
}

func TestQuotaStatusHandlerNoTenant(t *testing.T) {
	manager := quota.NewManager()
	handler := QuotaStatusHandler(manager)

	req := httptest.NewRequest("GET", "/quota/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestQuotaStatusHandlerNoQuota(t *testing.T) {
	manager := quota.NewManager()
	handler := QuotaStatusHandler(manager)

	tnt := &tenant.Tenant{
		ID:   "tenant-2",
		Name: "Test",
	}
	ctx := tenant.WithTenant(context.Background(), tnt)

	req := httptest.NewRequest("GET", "/quota/status", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}
