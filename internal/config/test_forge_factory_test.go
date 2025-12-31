package config

import (
	"testing"
)

func TestCreateLocalForge(t *testing.T) {
	factory := NewTestForgeConfigFactory()

	forge := factory.CreateLocalForge("test")

	if forge == nil {
		t.Fatal("CreateLocalForge should not return nil")
		return
	}

	if forge.Type != ForgeLocal {
		t.Errorf("Expected forge type ForgeLocal, got %v", forge.Type)
	}

	if forge.Name == "" {
		t.Error("Expected non-empty forge name")
	}

	// Local forges don't have organizations, groups, or webhooks
	if len(forge.Organizations) != 0 {
		t.Errorf("Expected empty organizations for local forge, got %v", forge.Organizations)
	}
	if len(forge.Groups) != 0 {
		t.Errorf("Expected empty groups for local forge, got %v", forge.Groups)
	}
	if forge.Webhook != nil {
		t.Error("Expected nil webhook for local forge")
	}
}

func TestCreateForgeWithAutoDiscover_AllTypes(t *testing.T) {
	tests := []struct {
		name      string
		forgeType ForgeType
		wantType  ForgeType
	}{
		{
			name:      "GitHub forge",
			forgeType: ForgeGitHub,
			wantType:  ForgeGitHub,
		},
		{
			name:      "GitLab forge",
			forgeType: ForgeGitLab,
			wantType:  ForgeGitLab,
		},
		{
			name:      "Forgejo forge",
			forgeType: ForgeForgejo,
			wantType:  ForgeForgejo,
		},
		{
			name:      "Local forge",
			forgeType: ForgeLocal,
			wantType:  ForgeLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewTestForgeConfigFactory()
			forge := factory.CreateForgeWithAutoDiscover(tt.forgeType, "test")

			if forge == nil {
				t.Fatal("CreateForgeWithAutoDiscover should not return nil")
				return
			}

			if forge.Type != tt.wantType {
				t.Errorf("Expected forge type %v, got %v", tt.wantType, forge.Type)
			}

			// Local forges don't support auto-discovery
			if tt.forgeType == ForgeLocal {
				if forge.AutoDiscover {
					t.Error("Local forge should not have auto-discovery enabled")
				}
				return
			}

			// Remote forges should have auto-discovery enabled
			if !forge.AutoDiscover {
				t.Error("Expected auto-discovery to be enabled")
			}

			if len(forge.Organizations) != 0 {
				t.Errorf("Expected empty organizations with auto-discovery, got %v", forge.Organizations)
			}
			if len(forge.Groups) != 0 {
				t.Errorf("Expected empty groups with auto-discovery, got %v", forge.Groups)
			}
		})
	}
}

func TestCreateForgeWithOptions_AllTypes(t *testing.T) {
	tests := []struct {
		name      string
		forgeType ForgeType
		wantType  ForgeType
	}{
		{
			name:      "GitHub forge",
			forgeType: ForgeGitHub,
			wantType:  ForgeGitHub,
		},
		{
			name:      "GitLab forge",
			forgeType: ForgeGitLab,
			wantType:  ForgeGitLab,
		},
		{
			name:      "Forgejo forge",
			forgeType: ForgeForgejo,
			wantType:  ForgeForgejo,
		},
		{
			name:      "Local forge",
			forgeType: ForgeLocal,
			wantType:  ForgeLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewTestForgeConfigFactory()
			options := map[string]any{
				"test_option": "test_value",
			}
			forge := factory.CreateForgeWithOptions(tt.forgeType, "test", options)

			if forge == nil {
				t.Fatal("CreateForgeWithOptions should not return nil")
			}

			if forge.Type != tt.wantType {
				t.Errorf("Expected forge type %v, got %v", tt.wantType, forge.Type)
			}

			if forge.Options == nil {
				t.Fatal("Expected non-nil options")
			}

			if val, ok := forge.Options["test_option"]; !ok || val != "test_value" {
				t.Errorf("Expected options to contain test_option=test_value, got %v", forge.Options)
			}
		})
	}
}
