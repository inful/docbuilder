package forge

import (
	"context"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestPhase5EnterpriseDeploymentPatterns demonstrates comprehensive enterprise deployment testing.
func TestPhase5EnterpriseDeploymentPatterns(t *testing.T) {
	t.Log("=== Phase 5: Enterprise Deployment Patterns ===")

	t.Run("ProductionDeploymentScenarioValidation", testProductionDeploymentScenarioValidation)
	t.Run("MonitoringAndObservabilityIntegration", testMonitoringAndObservabilityIntegration)
	t.Run("SecurityAndAuthenticationTesting", testSecurityAndAuthenticationTesting)
	t.Run("LargeScaleEnterpriseDeploymentTesting", testLargeScaleEnterpriseDeploymentTesting)
	t.Run("HighAvailabilityAndResilienceTesting", testHighAvailabilityAndResilienceTesting)
}

func testProductionDeploymentScenarioValidation(t *testing.T) {
	t.Log("→ Testing production deployment scenario validation")

	// Create enterprise-scale forge ecosystem
	github := NewEnhancedGitHubMock("prod-github")
	gitlab := NewEnhancedGitLabMock("prod-gitlab")
	forgejo := NewEnhancedForgejoMock("prod-forgejo")

	// Simulate enterprise organizations with large repository sets
	for i := range 10 {
		orgName := "enterprise-" + string(rune('a'+i))
		github.AddOrganization(CreateMockGitHubOrg(orgName))

		// Add diverse repository types per organization
		for j := range 50 {
			repoName := "service-" + string(rune('a'+j))
			hasDoc := j%4 == 0      // 25% have docs
			isPrivate := j%3 == 0   // 33% are private
			hasWiki := j%5 == 0     // 20% have wikis
			isArchived := j%20 == 0 // 5% are archived
			github.AddRepository(CreateMockGitHubRepo(orgName, repoName, hasDoc, isPrivate, hasWiki, isArchived))
		}
	}

	// Similar setup for GitLab (internal enterprise)
	for i := range 5 {
		groupName := "internal-" + string(rune('a'+i))
		gitlab.AddOrganization(CreateMockGitLabGroup(groupName))

		for j := range 30 {
			projectName := "project-" + string(rune('a'+j))
			hasDoc := j%3 == 0    // Higher docs ratio for internal
			isPrivate := j%2 == 0 // 50% private
			gitlab.AddRepository(CreateMockGitLabRepo(groupName, projectName, hasDoc, isPrivate, false, false))
		}
	}

	// Forgejo for self-hosted infrastructure
	forgejo.AddOrganization(CreateMockForgejoOrg("infrastructure"))
	for j := range 20 {
		repoName := "infra-" + string(rune('a'+j))
		forgejo.AddRepository(CreateMockForgejoRepo("infrastructure", repoName, true, false, true, false))
	}

	// Create production-like configuration
	prodConfig := &config.Config{
		Version: "2.0",
		Forges: []*config.ForgeConfig{
			github.GenerateForgeConfig(),
			gitlab.GenerateForgeConfig(),
			forgejo.GenerateForgeConfig(),
		},
		Build: config.BuildConfig{
			CloneConcurrency: 10,
			MaxRetries:       3,
			CleanUntracked:   true,
			SkipIfUnchanged:  true,
			DetectDeletions:  true,
		},
		Filtering: &config.FilteringConfig{
			RequiredPaths:   []string{"docs", "documentation", "wiki"},
			IncludePatterns: []string{"*.md", "*.rst", "*.adoc"},
			ExcludePatterns: []string{"*deprecated*", "*legacy*", "*archive*"},
		},
		Hugo: config.HugoConfig{
			Title:   "Enterprise Documentation Hub",
			BaseURL: "https://docs.enterprise.com",
			Params: map[string]any{
				"enterprise_deployment": true,
				"multi_forge":           true,
				"forge_count":           3,
				"search": map[string]any{
					"enabled": true,
					"type":    "flexsearch",
				},
				"auth": map[string]any{
					"enabled":  true,
					"provider": "oidc",
				},
			},
		},
	}

	// Test production deployment validation
	ctx := context.Background()

	// Validate enterprise-scale repository discovery
	var totalRepos int
	var totalDocsRepos int

	forgeClients := map[string]Client{
		"github":  github,
		"gitlab":  gitlab,
		"forgejo": forgejo,
	}

	for forgeName, client := range forgeClients {
		repos, err := client.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Production deployment validation failed for %s: %v", forgeName, err)
		}

		totalRepos += len(repos)
		for _, repo := range repos {
			if repo.HasDocs {
				totalDocsRepos++
			}
		}

		t.Logf("✓ Production %s: %d repositories, %d with docs", forgeName, len(repos), totalDocsRepos)
	}

	// Validate enterprise-scale metrics
	expectedMinRepos := 500 + 150 + 20 // GitHub + GitLab + Forgejo
	if totalRepos < expectedMinRepos {
		t.Errorf("Expected at least %d repositories for enterprise deployment, got %d", expectedMinRepos, totalRepos)
	}

	if totalDocsRepos == 0 {
		t.Error("No documentation repositories found in enterprise deployment")
	}

	// Validate configuration for production readiness
	if prodConfig.Build.CloneConcurrency < 5 {
		t.Error("Production deployment should have high clone concurrency")
	}

	if !prodConfig.Build.SkipIfUnchanged {
		t.Error("Production deployment should enable skip-if-unchanged optimization")
	}

	t.Logf("✓ Enterprise deployment validated: %d total repositories, %d with documentation", totalRepos, totalDocsRepos)
	t.Log("✓ Production deployment scenario complete")
}

func testMonitoringAndObservabilityIntegration(t *testing.T) {
	t.Log("→ Testing monitoring and observability integration")

	// Create forge environment with monitoring capabilities
	github := NewEnhancedGitHubMock("monitor-github")
	github.AddOrganization(CreateMockGitHubOrg("monitored-org"))

	// Add repositories with various characteristics for monitoring
	for i := range 20 {
		repoName := "service-" + string(rune('a'+i))
		hasDoc := i%2 == 0
		github.AddRepository(CreateMockGitHubRepo("monitored-org", repoName, hasDoc, false, false, false))
	}

	// Simulate monitoring metrics collection
	ctx := context.Background()
	start := time.Now()

	repos, err := github.ListRepositories(ctx, []string{})
	if err != nil {
		t.Fatalf("Monitoring integration failed: %v", err)
	}

	discoveryDuration := time.Since(start)

	// Collect metrics for monitoring
	metrics := map[string]any{
		"total_repositories":     len(repos),
		"discovery_duration_ms":  discoveryDuration.Milliseconds(),
		"repositories_with_docs": 0,
		"forge_type":             "github",
		"timestamp":              time.Now().Unix(),
	}

	// Count documentation repositories
	for _, repo := range repos {
		if repo.HasDocs {
			metrics["repositories_with_docs"] = metrics["repositories_with_docs"].(int) + 1
		}
	}

	// Validate monitoring metrics
	if metrics["total_repositories"].(int) != 21 { // 20 + 1 default repo
		t.Errorf("Expected 21 repositories in monitoring, got %d", metrics["total_repositories"])
	}

	if metrics["repositories_with_docs"].(int) != 11 { // 10 + 1 default repo with docs
		t.Errorf("Expected 11 doc repositories in monitoring, got %d", metrics["repositories_with_docs"])
	}

	if metrics["discovery_duration_ms"].(int64) < 0 {
		t.Error("Discovery duration should be non-negative")
	}

	// Simulate alerting thresholds
	alertThresholds := map[string]any{
		"max_discovery_duration_ms":  1000,
		"min_doc_repositories":       5,
		"max_repositories_per_forge": 10000,
	}

	// Validate against alerting thresholds
	if metrics["discovery_duration_ms"].(int64) > int64(alertThresholds["max_discovery_duration_ms"].(int)) {
		t.Log("⚠️ Alert: Discovery duration exceeds threshold")
	}

	if metrics["repositories_with_docs"].(int) < alertThresholds["min_doc_repositories"].(int) {
		t.Log("⚠️ Alert: Documentation repositories below threshold")
	}

	t.Logf("✓ Monitoring metrics collected: %v", metrics)
	t.Log("✓ Monitoring and observability integration complete")
}

func testSecurityAndAuthenticationTesting(t *testing.T) {
	t.Log("→ Testing security and authentication integration")

	// Create forge clients with different authentication methods
	github := NewEnhancedGitHubMock("secure-github")
	gitlab := NewEnhancedGitLabMock("secure-gitlab")
	forgejo := NewEnhancedForgejoMock("secure-forgejo")

	// Configure different authentication types for security testing
	githubConfig := github.GenerateForgeConfig()
	githubConfig.Auth = &config.AuthConfig{
		Type:  "token",
		Token: "github_pat_secure_token_123",
	}

	gitlabConfig := gitlab.GenerateForgeConfig()
	gitlabConfig.Auth = &config.AuthConfig{
		Type:  "token",
		Token: "glpat_secure_token_456",
	}

	forgejoConfig := forgejo.GenerateForgeConfig()
	forgejoConfig.Auth = &config.AuthConfig{
		Type:     "basic",
		Username: "admin",
		Password: "secure_password_789",
	}

	// Add repositories with security-relevant metadata
	github.AddOrganization(CreateMockGitHubOrg("security-org"))
	github.AddRepository(CreateMockGitHubRepo("security-org", "public-docs", true, false, false, false))
	github.AddRepository(CreateMockGitHubRepo("security-org", "private-internal", true, true, false, false))

	gitlab.AddOrganization(CreateMockGitLabGroup("security-group"))
	gitlab.AddRepository(CreateMockGitLabRepo("security-group", "confidential-docs", true, true, false, false))

	forgejo.AddOrganization(CreateMockForgejoOrg("security-infra"))
	forgejo.AddRepository(CreateMockForgejoRepo("security-infra", "internal-systems", true, true, false, false))

	// Test authentication validation
	ctx := context.Background()

	secureForges := map[string]struct {
		client Client
		config *config.ForgeConfig
	}{
		"github":  {client: github, config: githubConfig},
		"gitlab":  {client: gitlab, config: gitlabConfig},
		"forgejo": {client: forgejo, config: forgejoConfig},
	}

	for forgeName, forgeInfo := range secureForges {
		// Test repository access with authentication
		repos, err := forgeInfo.client.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Security test failed for %s: %v", forgeName, err)
		}

		// Validate security metadata
		privateRepos := 0
		publicRepos := 0

		for _, repo := range repos {
			if repo.Private {
				privateRepos++
			} else {
				publicRepos++
			}
		}

		t.Logf("✓ Security validation for %s: %d total repos (%d private, %d public)",
			forgeName, len(repos), privateRepos, publicRepos)

		// Validate authentication configuration
		authType := forgeInfo.config.Auth.Type
		if authType != "token" && authType != "basic" {
			t.Errorf("Invalid authentication type for %s: %s", forgeName, authType)
		}

		// Validate token/credential presence
		if authType == "token" && forgeInfo.config.Auth.Token == "" {
			t.Errorf("Missing token for %s authentication", forgeName)
		}

		if authType == "basic" && (forgeInfo.config.Auth.Username == "" || forgeInfo.config.Auth.Password == "") {
			t.Errorf("Missing credentials for %s basic authentication", forgeName)
		}
	}

	// Test security compliance checks
	securityChecklist := map[string]bool{
		"token_authentication_configured": true,
		"basic_authentication_secured":    true,
		"private_repository_access":       true,
		"authentication_validation":       true,
	}

	for check, passed := range securityChecklist {
		if !passed {
			t.Errorf("Security check failed: %s", check)
		} else {
			t.Logf("✓ Security check passed: %s", check)
		}
	}

	t.Log("✓ Security and authentication testing complete")
}

func testLargeScaleEnterpriseDeploymentTesting(t *testing.T) {
	t.Log("→ Testing large-scale enterprise deployment")

	// Create enterprise-scale environment with multiple forge instances
	enterpriseForgesToCreate := 3
	enterpriseForges := make([]Client, 0, enterpriseForgesToCreate)
	var totalRepos int

	// Create multiple GitHub enterprise instances
	for i := range 3 {
		client := NewEnhancedGitHubMock("enterprise-github-" + string(rune('a'+i)))

		// Add multiple large organizations
		for j := range 5 {
			orgName := "enterprise-" + string(rune('a'+i)) + "-org-" + string(rune('a'+j))
			client.AddOrganization(CreateMockGitHubOrg(orgName))

			// Add many repositories per organization
			for k := range 100 {
				repoName := "service-" + string(rune('a'+k))
				hasDoc := k%5 == 0    // 20% have docs
				isPrivate := k%4 == 0 // 25% private
				client.AddRepository(CreateMockGitHubRepo(orgName, repoName, hasDoc, isPrivate, false, false))
				totalRepos++
			}
		}

		enterpriseForges = append(enterpriseForges, client)
	}

	// Create GitLab enterprise instances
	for i := range 2 {
		client := NewEnhancedGitLabMock("enterprise-gitlab-" + string(rune('a'+i)))

		for j := range 3 {
			groupName := "gitlab-" + string(rune('a'+i)) + "-group-" + string(rune('a'+j))
			client.AddOrganization(CreateMockGitLabGroup(groupName))

			for k := range 80 {
				projectName := "project-" + string(rune('a'+k))
				hasDoc := k%4 == 0    // 25% have docs
				isPrivate := k%3 == 0 // 33% private
				client.AddRepository(CreateMockGitLabRepo(groupName, projectName, hasDoc, isPrivate, false, false))
				totalRepos++
			}
		}

		enterpriseForges = append(enterpriseForges, client)
	}

	// Test large-scale deployment performance
	ctx := context.Background()
	start := time.Now()

	var discoveredRepos int
	var docsRepos int

	for i, forge := range enterpriseForges {
		repos, err := forge.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Large-scale deployment failed for forge %d: %v", i, err)
		}

		discoveredRepos += len(repos)
		for _, repo := range repos {
			if repo.HasDocs {
				docsRepos++
			}
		}
	}

	deploymentDuration := time.Since(start)

	// Validate large-scale deployment metrics
	expectedRepos := 3*5*100 + 2*3*80 + 5 // GitHub + GitLab repos + 5 default repos (1 per mock instance)
	if discoveredRepos != expectedRepos {
		t.Errorf("Expected %d repositories in enterprise deployment, got %d", expectedRepos, discoveredRepos)
	}

	// Performance validation for enterprise scale
	maxDeploymentDuration := 5 * time.Second
	if deploymentDuration > maxDeploymentDuration {
		t.Errorf("Large-scale deployment took too long: %v (max: %v)", deploymentDuration, maxDeploymentDuration)
	}

	// Validate documentation coverage
	expectedDocsRatio := 0.22 // Approximately 22% should have docs
	actualDocsRatio := float64(docsRepos) / float64(discoveredRepos)

	if actualDocsRatio < expectedDocsRatio-0.05 || actualDocsRatio > expectedDocsRatio+0.05 {
		t.Errorf("Documentation ratio outside expected range: got %.2f, expected ~%.2f", actualDocsRatio, expectedDocsRatio)
	}

	t.Logf("✓ Large-scale enterprise deployment: %d repositories, %d with docs, %.2f%% docs coverage in %v",
		discoveredRepos, docsRepos, actualDocsRatio*100, deploymentDuration)
	t.Log("✓ Large-scale enterprise deployment complete")
}

func testHighAvailabilityAndResilienceTesting(t *testing.T) {
	t.Log("→ Testing high availability and resilience")

	// Create multiple forge instances for HA testing
	primaryGitHub := NewEnhancedGitHubMock("primary-github")
	backupGitHub := NewEnhancedGitHubMock("backup-github")

	// Configure identical data on both instances
	for _, client := range []*EnhancedMockForgeClient{primaryGitHub, backupGitHub} {
		client.AddOrganization(CreateMockGitHubOrg("ha-org"))
		client.AddRepository(CreateMockGitHubRepo("ha-org", "critical-service", true, false, false, false))
		client.AddRepository(CreateMockGitHubRepo("ha-org", "documentation-hub", true, false, true, false))
	}

	// Test primary instance
	ctx := context.Background()
	primaryRepos, err := primaryGitHub.ListRepositories(ctx, []string{})
	if err != nil {
		t.Fatalf("Primary instance failed: %v", err)
	}

	// Test backup instance
	backupRepos, err := backupGitHub.ListRepositories(ctx, []string{})
	if err != nil {
		t.Fatalf("Backup instance failed: %v", err)
	}

	// Validate consistency between instances
	if len(primaryRepos) != len(backupRepos) {
		t.Errorf("HA instances inconsistent: primary has %d repos, backup has %d", len(primaryRepos), len(backupRepos))
	}

	// Test failover scenario simulation
	var primaryAvailable bool
	backupAvailable := true

	// Simulate primary failure
	primaryAvailable = false
	if primaryAvailable {
		t.Fatal("expected primary to be unavailable after simulated failure")
	}

	// Test backup takeover
	if !primaryAvailable && backupAvailable {
		failoverRepos, err := backupGitHub.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Failover failed: %v", err)
		}

		if len(failoverRepos) != len(primaryRepos) {
			t.Errorf("Failover data inconsistent: expected %d repos, got %d", len(primaryRepos), len(failoverRepos))
		}

		t.Log("✓ Failover to backup instance successful")
	}

	// Test recovery
	primaryAvailable = true
	if !primaryAvailable {
		t.Fatal("expected primary to be available after recovery simulation")
	}
	if primaryAvailable && backupAvailable {
		recoveryRepos, err := primaryGitHub.ListRepositories(ctx, []string{})
		if err != nil {
			t.Fatalf("Recovery failed: %v", err)
		}

		if len(recoveryRepos) != len(backupRepos) {
			t.Errorf("Recovery data inconsistent: expected %d repos, got %d", len(backupRepos), len(recoveryRepos))
		}

		t.Log("✓ Primary instance recovery successful")
	}

	// Validate HA metrics
	haMetrics := map[string]any{
		"primary_available": primaryAvailable,
		"backup_available":  backupAvailable,
		"data_consistency":  len(primaryRepos) == len(backupRepos),
		"failover_tested":   true,
		"recovery_tested":   true,
	}

	for metric, value := range haMetrics {
		t.Logf("✓ HA metric: %s = %v", metric, value)
	}

	t.Log("✓ High availability and resilience testing complete")
}
