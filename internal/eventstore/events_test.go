package eventstore

import (
	"encoding/json"
	"testing"
	"time"
)

const testBuildID = "build-123"

func TestEventSerialization(t *testing.T) {
	buildID := testBuildID

	tests := []struct {
		name      string
		createFn  func() (Event, error)
		eventType string
	}{
		{
			name: "BuildStarted",
			createFn: func() (Event, error) {
				return NewBuildStarted(buildID, BuildStartedMeta{TenantID: "tenant-1", Type: "manual"})
			},
			eventType: "BuildStarted",
		},
		{
			name: "RepositoryCloned",
			createFn: func() (Event, error) {
				return NewRepositoryCloned(buildID, "repo-1", "abc123", "/path/to/repo", 100*time.Millisecond)
			},
			eventType: "RepositoryCloned",
		},
		{
			name: "DocumentsDiscovered",
			createFn: func() (Event, error) {
				return NewDocumentsDiscovered(buildID, "repo-1", []string{"file1.md", "file2.md"})
			},
			eventType: "DocumentsDiscovered",
		},
		{
			name: "TransformApplied",
			createFn: func() (Event, error) {
				return NewTransformApplied(buildID, "frontmatter", 10, 50*time.Millisecond)
			},
			eventType: "TransformApplied",
		},
		{
			name: "HugoConfigGenerated",
			createFn: func() (Event, error) {
				return NewHugoConfigGenerated(buildID, "hash123", map[string]any{"theme": "relearn"})
			},
			eventType: "HugoConfigGenerated",
		},
		{
			name: "SiteGenerated",
			createFn: func() (Event, error) {
				return NewSiteGenerated(buildID, "/output/path", 100, 2*time.Second)
			},
			eventType: "SiteGenerated",
		},
		{
			name: "BuildCompleted",
			createFn: func() (Event, error) {
				return NewBuildCompleted(buildID, "success", 5*time.Second, map[string]string{"site": "/output"})
			},
			eventType: "BuildCompleted",
		},
		{
			name: "BuildFailed",
			createFn: func() (Event, error) {
				return NewBuildFailed(buildID, "generate", "failed to generate site")
			},
			eventType: "BuildFailed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create event
			event, err := tt.createFn()
			if err != nil {
				t.Fatalf("failed to create event: %v", err)
			}

			// Verify required fields
			if event.BuildID() != buildID {
				t.Errorf("expected build_id %s, got %s", buildID, event.BuildID())
			}
			if event.Type() != tt.eventType {
				t.Errorf("expected event_type %s, got %s", tt.eventType, event.Type())
			}
			if event.Timestamp().IsZero() {
				t.Error("timestamp should not be zero")
			}

			// Verify payload is valid JSON
			payload := event.Payload()
			if len(payload) == 0 {
				t.Error("payload should not be empty")
			}

			var data map[string]any
			if err := json.Unmarshal(payload, &data); err != nil {
				t.Errorf("failed to unmarshal payload: %v", err)
			}
		})
	}
}

func TestBuildStartedFields(t *testing.T) {
	buildID := testBuildID
	meta := BuildStartedMeta{
		TenantID: "tenant-1",
		Type:     "manual",
		Priority: 5,
		WorkerID: "worker-1",
	}

	event, err := NewBuildStarted(buildID, meta)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	if event.TenantID != meta.TenantID {
		t.Errorf("expected tenant_id %s, got %s", meta.TenantID, event.TenantID)
	}
	if event.Config.Type != "manual" {
		t.Errorf("expected config type=manual, got %s", event.Config.Type)
	}
	if event.Config.Priority != 5 {
		t.Errorf("expected config priority=5, got %d", event.Config.Priority)
	}
}

func TestRepositoryClonedFields(t *testing.T) {
	buildID := testBuildID
	repoName := "repo-1"
	commit := "abc123"
	path := "/path/to/repo"
	duration := 100 * time.Millisecond

	event, err := NewRepositoryCloned(buildID, repoName, commit, path, duration)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	if event.RepoName != repoName {
		t.Errorf("expected repo_name %s, got %s", repoName, event.RepoName)
	}
	if event.Commit != commit {
		t.Errorf("expected commit %s, got %s", commit, event.Commit)
	}
	if event.Path != path {
		t.Errorf("expected path %s, got %s", path, event.Path)
	}
	if event.Duration != duration {
		t.Errorf("expected duration %v, got %v", duration, event.Duration)
	}
}

func TestDocumentsDiscoveredFields(t *testing.T) {
	buildID := "build-123"
	repoName := "repo-1"
	files := []string{"file1.md", "file2.md", "file3.md"}

	event, err := NewDocumentsDiscovered(buildID, repoName, files)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	if event.RepoName != repoName {
		t.Errorf("expected repo_name %s, got %s", repoName, event.RepoName)
	}
	if event.FileCount != len(files) {
		t.Errorf("expected file_count %d, got %d", len(files), event.FileCount)
	}
	if len(event.Files) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(event.Files))
	}
}

func TestBuildFailedFields(t *testing.T) {
	buildID := "build-123"
	stage := "generate"
	errorMsg := "failed to generate site"

	event, err := NewBuildFailed(buildID, stage, errorMsg)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	if event.Stage != stage {
		t.Errorf("expected stage %s, got %s", stage, event.Stage)
	}
	if event.Error != errorMsg {
		t.Errorf("expected error %s, got %s", errorMsg, event.Error)
	}
}
