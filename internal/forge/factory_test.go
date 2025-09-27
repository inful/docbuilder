package forge

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestNewForgeClient(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.ForgeConfig
		wantType  ForgeType
		wantError bool
	}{
		{
			name: "GitHub client creation",
			config: &config.ForgeConfig{
				Name:          "test-github",
				Type:          "github",
				APIURL:        "https://api.github.com",
				Organizations: []string{"test-org"},
				Auth: &config.AuthConfig{
					Type:  "token",
					Token: "test-token",
				},
			},
			wantType:  ForgeTypeGitHub,
			wantError: false,
		},
		{
			name: "GitLab client creation",
			config: &config.ForgeConfig{
				Name:   "test-gitlab",
				Type:   "gitlab",
				APIURL: "https://gitlab.example.com/api/v4",
				Groups: []string{"test-group"},
				Auth: &config.AuthConfig{
					Type:  "token",
					Token: "test-token",
				},
			},
			wantType:  ForgeTypeGitLab,
			wantError: false,
		},
		{
			name: "Forgejo client creation",
			config: &config.ForgeConfig{
				Name:          "test-forgejo",
				Type:          "forgejo",
				APIURL:        "https://forge.example.com/api/v1",
				Organizations: []string{"test-org"},
				Auth: &config.AuthConfig{
					Type:  "token",
					Token: "test-token",
				},
			},
			wantType:  ForgeTypeForgejo,
			wantError: false,
		},
		{
			name: "Unsupported forge type",
			config: &config.ForgeConfig{
				Name: "test-unsupported",
				Type: "bitbucket",
				Auth: &config.AuthConfig{
					Type:  "token",
					Token: "test-token",
				},
			},
			wantError: true,
		},
		{
			name: "Missing authentication",
			config: &config.ForgeConfig{
				Name:          "test-no-auth",
				Type:          "github",
				Organizations: []string{"test-org"},
				// No Auth field
			},
			wantError: true,
		},
		{
			name: "Wrong authentication type",
			config: &config.ForgeConfig{
				Name:          "test-wrong-auth",
				Type:          "github",
				Organizations: []string{"test-org"},
				Auth: &config.AuthConfig{
					Type: "ssh", // GitHub client expects token
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewForgeClient(tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewForgeClient() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewForgeClient() unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Fatal("NewForgeClient() returned nil client")
			}

			if client.GetType() != tt.wantType {
				t.Errorf("NewForgeClient() type = %v, want %v", client.GetType(), tt.wantType)
			}

			if client.GetName() != tt.config.Name {
				t.Errorf("NewForgeClient() name = %v, want %v", client.GetName(), tt.config.Name)
			}
		})
	}
}

func TestCreateForgeManager(t *testing.T) {
	tests := []struct {
		name      string
		configs   []*config.ForgeConfig
		wantCount int
		wantError bool
	}{
		{
			name: "Multiple forge configurations",
			configs: []*config.ForgeConfig{
				{
					Name:          "github-main",
					Type:          "github",
					Organizations: []string{"test-org"},
					Auth: &config.AuthConfig{
						Type:  "token",
						Token: "github-token",
					},
				},
				{
					Name:   "gitlab-internal",
					Type:   "gitlab",
					Groups: []string{"internal"},
					Auth: &config.AuthConfig{
						Type:  "token",
						Token: "gitlab-token",
					},
				},
			},
			wantCount: 2,
			wantError: false,
		},
		{
			name:      "Empty configurations",
			configs:   []*config.ForgeConfig{},
			wantCount: 0,
			wantError: false,
		},
		{
			name: "Invalid configuration",
			configs: []*config.ForgeConfig{
				{
					Name: "invalid",
					Type: "invalid-type",
					Auth: &config.AuthConfig{
						Type:  "token",
						Token: "token",
					},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := CreateForgeManager(tt.configs)

			if tt.wantError {
				if err == nil {
					t.Errorf("CreateForgeManager() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CreateForgeManager() unexpected error: %v", err)
				return
			}

			if manager == nil {
				t.Fatal("CreateForgeManager() returned nil manager")
			}

			forges := manager.GetAllForges()
			if len(forges) != tt.wantCount {
				t.Errorf("CreateForgeManager() forge count = %v, want %v", len(forges), tt.wantCount)
			}

			configs := manager.GetForgeConfigs()
			if len(configs) != tt.wantCount {
				t.Errorf("CreateForgeManager() config count = %v, want %v", len(configs), tt.wantCount)
			}

			// Test individual forge retrieval
			for _, config := range tt.configs {
				forge := manager.GetForge(config.Name)
				if forge == nil {
					t.Errorf("GetForge(%s) returned nil", config.Name)
				} else if forge.GetName() != config.Name {
					t.Errorf("GetForge(%s) name = %v, want %v", config.Name, forge.GetName(), config.Name)
				}
			}
		})
	}
}

func TestForgeManager(t *testing.T) {
	manager := NewForgeManager()

	// Test empty manager
	if len(manager.GetAllForges()) != 0 {
		t.Error("New manager should have no forges")
	}

	if len(manager.GetForgeConfigs()) != 0 {
		t.Error("New manager should have no configs")
	}

	// Create mock client and config
	mockClient := NewMockForgeClient("test-mock", ForgeTypeGitHub)
	mockConfig := CreateMockForgeConfig("test-mock", "github", []string{"test-org"}, nil)

	// Add forge
	manager.AddForge(mockConfig, mockClient)

	// Test retrieval
	if len(manager.GetAllForges()) != 1 {
		t.Error("Manager should have one forge after adding")
	}

	retrievedForge := manager.GetForge("test-mock")
	if retrievedForge == nil {
		t.Error("Should be able to retrieve added forge")
	}

	if retrievedForge.GetName() != "test-mock" {
		t.Errorf("Retrieved forge name = %v, want test-mock", retrievedForge.GetName())
	}

	// Test non-existent forge
	nonExistent := manager.GetForge("non-existent")
	if nonExistent != nil {
		t.Error("Non-existent forge should return nil")
	}
}

func TestRepositoryToConfigRepository(t *testing.T) {
	repo := CreateMockGitHubRepo("test-org", "test-repo", true, false, false, false)
	auth := &config.AuthConfig{
		Type:  "token",
		Token: "test-token",
	}

	configRepo := repo.ToConfigRepository(auth)

	if configRepo.URL != repo.CloneURL {
		t.Errorf("ConfigRepository URL = %v, want %v", configRepo.URL, repo.CloneURL)
	}

	if configRepo.Name != repo.Name {
		t.Errorf("ConfigRepository Name = %v, want %v", configRepo.Name, repo.Name)
	}

	if configRepo.Branch != repo.DefaultBranch {
		t.Errorf("ConfigRepository Branch = %v, want %v", configRepo.Branch, repo.DefaultBranch)
	}

	if configRepo.Auth != auth {
		t.Error("ConfigRepository should have same auth reference")
	}

	// Test SSH URL preference
	repo.SSHURL = "git@github.com:test-org/test-repo.git"
	sshAuth := &config.AuthConfig{
		Type: "ssh",
	}

	sshConfigRepo := repo.ToConfigRepository(sshAuth)
	if sshConfigRepo.URL != repo.SSHURL {
		t.Errorf("ConfigRepository with SSH auth should use SSH URL, got %v", sshConfigRepo.URL)
	}

	// Test metadata tags
	if len(configRepo.Tags) == 0 {
		t.Error("ConfigRepository should have metadata tags")
	}

	expectedTags := []string{"forge_id", "full_name", "description", "private", "has_docs", "forge_type"}
	for _, tag := range expectedTags {
		if _, exists := configRepo.Tags[tag]; !exists {
			t.Errorf("ConfigRepository should have tag %s", tag)
		}
	}
}
