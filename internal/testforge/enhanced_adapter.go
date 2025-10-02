package testforge

import (
	"context"
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

// EnhancedTestForge provides backward-compatible API while using enhanced mock capabilities
type EnhancedTestForge struct {
	enhancedClient forge.ForgeClient
	originalName   string
	originalType   config.ForgeType
}

// NewEnhancedTestForge creates a new enhanced test forge that maintains API compatibility
// with the existing TestForge while providing advanced mock capabilities
func NewEnhancedTestForge(name string, forgeType config.ForgeType) *EnhancedTestForge {
	var client forge.ForgeClient

	switch forgeType {
	case config.ForgeGitHub:
		client = forge.NewEnhancedGitHubMock(name)
	case config.ForgeGitLab:
		client = forge.NewEnhancedGitLabMock(name)
	case config.ForgeForgejo:
		client = forge.NewEnhancedForgejoMock(name)
	default:
		// Fallback to generic enhanced mock
		client = forge.NewEnhancedMockForgeClient(name, forge.ForgeType(forgeType))
	}

	return &EnhancedTestForge{
		enhancedClient: client,
		originalName:   name,
		originalType:   forgeType,
	}
}

// AddRepository adds a repository using the TestRepository format but converts to enhanced mock format
func (e *EnhancedTestForge) AddRepository(repo TestRepository) {
	// Convert TestRepository to forge.Repository
	forgeRepo := &forge.Repository{
		ID:            repo.ID,
		Name:          repo.Name,
		FullName:      repo.FullName,
		CloneURL:      repo.CloneURL,
		SSHURL:        repo.SSHURL,
		DefaultBranch: repo.DefaultBranch,
		Description:   repo.Description,
		Private:       repo.Private,
		Archived:      repo.Archived,
		HasDocs:       repo.HasDocs,
		HasDocIgnore:  repo.HasDocIgnore,
		LastUpdated:   repo.LastUpdated,
		Topics:        repo.Topics,
		Language:      repo.Language,
		Metadata:      repo.Metadata,
	}

	// Add to enhanced client if it supports direct repository addition
	if enhancedClient, ok := e.enhancedClient.(interface {
		AddRepository(*forge.Repository)
	}); ok {
		enhancedClient.AddRepository(forgeRepo)
	}
}

// AddOrganization adds an organization (maintains compatibility)
func (e *EnhancedTestForge) AddOrganization(name string) {
	org := &forge.Organization{
		ID:          name,
		Name:        name,
		DisplayName: name,
		Type:        "organization",
	}

	// Add to enhanced client if it supports direct organization addition
	if enhancedClient, ok := e.enhancedClient.(interface {
		AddOrganization(*forge.Organization)
	}); ok {
		enhancedClient.AddOrganization(org)
	}
}

// ClearRepositories clears all repositories (maintains compatibility)
func (e *EnhancedTestForge) ClearRepositories() {
	if enhancedClient, ok := e.enhancedClient.(interface {
		ClearRepositories()
	}); ok {
		enhancedClient.ClearRepositories()
	}
}

// ClearOrganizations clears all organizations (maintains compatibility)
func (e *EnhancedTestForge) ClearOrganizations() {
	if enhancedClient, ok := e.enhancedClient.(interface {
		ClearOrganizations()
	}); ok {
		enhancedClient.ClearOrganizations()
	}
}

// Enhanced capabilities - these are new methods that leverage the enhanced mock system

// WithAuthFailure enables authentication failure simulation
func (e *EnhancedTestForge) WithAuthFailure() *EnhancedTestForge {
	if enhancedClient, ok := e.enhancedClient.(interface {
		WithAuthFailure() forge.ForgeClient
	}); ok {
		e.enhancedClient = enhancedClient.WithAuthFailure()
	}
	return e
}

// WithRateLimit enables rate limiting simulation
func (e *EnhancedTestForge) WithRateLimit(requestsPerHour int, resetDuration interface{}) *EnhancedTestForge {
	if enhancedClient, ok := e.enhancedClient.(interface {
		WithRateLimit(int, interface{}) forge.ForgeClient
	}); ok {
		e.enhancedClient = enhancedClient.WithRateLimit(requestsPerHour, resetDuration)
	}
	return e
}

// WithNetworkTimeout enables network timeout simulation
func (e *EnhancedTestForge) WithNetworkTimeout(timeoutDuration interface{}) *EnhancedTestForge {
	if enhancedClient, ok := e.enhancedClient.(interface {
		WithNetworkTimeout(interface{}) forge.ForgeClient
	}); ok {
		e.enhancedClient = enhancedClient.WithNetworkTimeout(timeoutDuration)
	}
	return e
}

// WithDelay enables response delay simulation
func (e *EnhancedTestForge) WithDelay(delayDuration interface{}) *EnhancedTestForge {
	if enhancedClient, ok := e.enhancedClient.(interface {
		WithDelay(interface{}) forge.ForgeClient
	}); ok {
		e.enhancedClient = enhancedClient.WithDelay(delayDuration)
	}
	return e
}

// ClearFailures clears all failure conditions
func (e *EnhancedTestForge) ClearFailures() {
	if enhancedClient, ok := e.enhancedClient.(interface {
		ClearFailures()
	}); ok {
		enhancedClient.ClearFailures()
	}
}

// GenerateForgeConfig generates a complete forge configuration
func (e *EnhancedTestForge) GenerateForgeConfig() *config.ForgeConfig {
	if enhancedClient, ok := e.enhancedClient.(interface {
		GenerateForgeConfig() *config.ForgeConfig
	}); ok {
		return enhancedClient.GenerateForgeConfig()
	}

	// Fallback to basic config generation
	return &config.ForgeConfig{
		Name:    e.originalName,
		Type:    e.originalType,
		APIURL:  fmt.Sprintf("https://api.%s.test", e.originalName),
		BaseURL: fmt.Sprintf("https://%s.test", e.originalName),
		Auth: &config.AuthConfig{
			Type:  config.AuthTypeToken,
			Token: "test-token",
		},
	}
}

// ForgeClient returns the underlying forge client for direct access to enhanced features
func (e *EnhancedTestForge) ForgeClient() forge.ForgeClient {
	return e.enhancedClient
}

// Original TestForge API compatibility methods

// GetUserOrganizations maintains API compatibility
func (e *EnhancedTestForge) GetUserOrganizations(ctx context.Context) ([]TestOrganization, error) {
	orgs, err := e.enhancedClient.ListOrganizations(ctx)
	if err != nil {
		return nil, err
	}

	testOrgs := make([]TestOrganization, len(orgs))
	for i, org := range orgs {
		testOrgs[i] = TestOrganization{
			ID:          org.ID,
			Name:        org.Name,
			DisplayName: org.DisplayName,
			Description: org.Description,
			Type:        org.Type,
		}
	}

	return testOrgs, nil
}

// GetRepositoriesForOrganization maintains API compatibility
func (e *EnhancedTestForge) GetRepositoriesForOrganization(ctx context.Context, orgName string) ([]TestRepository, error) {
	repos, err := e.enhancedClient.ListRepositories(ctx, []string{orgName})
	if err != nil {
		return nil, err
	}

	testRepos := make([]TestRepository, len(repos))
	for i, repo := range repos {
		testRepos[i] = TestRepository{
			ID:            repo.ID,
			Name:          repo.Name,
			FullName:      repo.FullName,
			CloneURL:      repo.CloneURL,
			SSHURL:        repo.SSHURL,
			DefaultBranch: repo.DefaultBranch,
			Description:   repo.Description,
			Private:       repo.Private,
			Archived:      repo.Archived,
			HasDocs:       repo.HasDocs,
			HasDocIgnore:  repo.HasDocIgnore,
			LastUpdated:   repo.LastUpdated,
			Topics:        repo.Topics,
			Language:      repo.Language,
			Metadata:      repo.Metadata,
		}
	}

	return testRepos, nil
}

// GetRepository maintains API compatibility
func (e *EnhancedTestForge) GetRepository(ctx context.Context, owner, repo string) (*TestRepository, error) {
	forgeRepo, err := e.enhancedClient.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	return &TestRepository{
		ID:            forgeRepo.ID,
		Name:          forgeRepo.Name,
		FullName:      forgeRepo.FullName,
		CloneURL:      forgeRepo.CloneURL,
		SSHURL:        forgeRepo.SSHURL,
		DefaultBranch: forgeRepo.DefaultBranch,
		Description:   forgeRepo.Description,
		Private:       forgeRepo.Private,
		Archived:      forgeRepo.Archived,
		HasDocs:       forgeRepo.HasDocs,
		HasDocIgnore:  forgeRepo.HasDocIgnore,
		LastUpdated:   forgeRepo.LastUpdated,
		Topics:        forgeRepo.Topics,
		Language:      forgeRepo.Language,
		Metadata:      forgeRepo.Metadata,
	}, nil
}

// ValidateWebhook provides webhook validation capabilities
func (e *EnhancedTestForge) ValidateWebhook(payload []byte, signature string, secret string) bool {
	return e.enhancedClient.ValidateWebhook(payload, signature, secret)
}

// ParseWebhookEvent provides webhook event parsing
func (e *EnhancedTestForge) ParseWebhookEvent(payload []byte, eventType string) (*forge.WebhookEvent, error) {
	return e.enhancedClient.ParseWebhookEvent(payload, eventType)
}

// GetEditURL provides edit URL generation
func (e *EnhancedTestForge) GetEditURL(repoOwner, repoName, filePath, branch string) string {
	repo := &forge.Repository{
		FullName: fmt.Sprintf("%s/%s", repoOwner, repoName),
	}
	return e.enhancedClient.GetEditURL(repo, filePath, branch)
}
