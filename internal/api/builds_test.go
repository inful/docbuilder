package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMockBuildStoreCreate(t *testing.T) {
	store := NewMockBuildStore()
	build := &BuildResponse{
		Name:   "test-build",
		Status: "pending",
	}

	if err := store.CreateBuild(build); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if build.ID == "" {
		t.Error("expected build ID to be set")
	}
}

func TestMockBuildStoreGet(t *testing.T) {
	store := NewMockBuildStore()
	build := &BuildResponse{
		ID:     "build-1",
		Name:   "test-build",
		Status: "pending",
	}

	if err := store.CreateBuild(build); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	retrieved, err := store.GetBuild("build-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if retrieved.ID != "build-1" {
		t.Errorf("expected ID build-1, got %s", retrieved.ID)
	}
}

func TestMockBuildStoreGetNotFound(t *testing.T) {
	store := NewMockBuildStore()

	_, err := store.GetBuild("nonexistent")
	if err != ErrBuildNotFound {
		t.Errorf("expected ErrBuildNotFound, got %v", err)
	}
}

func TestMockBuildStoreList(t *testing.T) {
	store := NewMockBuildStore()

	for i := 1; i <= 5; i++ {
		build := &BuildResponse{
			ID:     genBuildID(i),
			Name:   "test-build-" + string(rune(i)),
			Status: "pending",
		}
		if err := store.CreateBuild(build); err != nil {
			t.Fatalf("failed to create build: %v", err)
		}
	}

	builds, err := store.ListBuilds(10, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(builds) != 5 {
		t.Errorf("expected 5 builds, got %d", len(builds))
	}
}

func TestMockBuildStoreUpdate(t *testing.T) {
	store := NewMockBuildStore()
	build := &BuildResponse{
		ID:     "build-1",
		Name:   "test-build",
		Status: "pending",
	}

	if err := store.CreateBuild(build); err != nil {
		t.Fatalf("failed to create build: %v", err)
	}

	build.Status = "completed"
	if err := store.UpdateBuild(build); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	retrieved, _ := store.GetBuild("build-1")
	if retrieved.Status != "completed" {
		t.Errorf("expected status completed, got %s", retrieved.Status)
	}
}

func TestMockBuildStoreDelete(t *testing.T) {
	store := NewMockBuildStore()
	build := &BuildResponse{
		ID:     "build-1",
		Name:   "test-build",
		Status: "pending",
	}

	if err := store.CreateBuild(build); err != nil {
		t.Fatalf("failed to create build: %v", err)
	}

	if err := store.DeleteBuild("build-1"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err := store.GetBuild("build-1")
	if err != ErrBuildNotFound {
		t.Errorf("expected ErrBuildNotFound after delete, got %v", err)
	}
}

func TestCreateBuildRequest(t *testing.T) {
	srv := NewServer(":8080")
	req := BuildRequest{
		Name:        "my-build",
		Description: "A test build",
		Repos: []RepoRequest{
			{
				URL:    "https://github.com/example/repo.git",
				Branch: "main",
				Name:   "example",
			},
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/builds", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}
}

func TestListBuildsRequest(t *testing.T) {
	srv := NewServer(":8080")
	httpReq := httptest.NewRequest("GET", "/builds?limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}
}

func TestGetBuildRequest(t *testing.T) {
	srv := NewServer(":8080")
	httpReq := httptest.NewRequest("GET", "/builds/build-456", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}
}

func TestUpdateBuildStatusRequest(t *testing.T) {
	srv := NewServer(":8080")
	updateReq := map[string]string{
		"status": "completed",
	}

	body, _ := json.Marshal(updateReq)
	httpReq := httptest.NewRequest("PUT", "/builds/build-789/status", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got %v", resp.Success)
	}
}

func TestCreateBuildInvalidRequest(t *testing.T) {
	srv := NewServer(":8080")
	httpReq := httptest.NewRequest("POST", "/builds", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAPIError(t *testing.T) {
	err := NewAPIError(http.StatusNotFound, "not found")
	if err.Code != http.StatusNotFound {
		t.Errorf("expected code %d, got %d", http.StatusNotFound, err.Code)
	}
	if err.Message != "not found" {
		t.Errorf("expected message 'not found', got %s", err.Message)
	}
	if err.Error() != "not found" {
		t.Errorf("expected Error() to return 'not found', got %s", err.Error())
	}
}

// Helper function to generate build IDs
func genBuildID(i int) string {
	switch i {
	case 1:
		return "build-001"
	case 2:
		return "build-002"
	case 3:
		return "build-003"
	case 4:
		return "build-004"
	case 5:
		return "build-005"
	default:
		return "build-unknown"
	}
}
