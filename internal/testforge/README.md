# Test Forge Documentation

The `internal/testforge` package provides a comprehensive test implementation of forge interfaces for testing DocBuilder without external API dependencies.

## Overview

The test forge system allows you to:
- Mock GitHub, GitLab, and Forgejo forges
- Simulate various failure modes (auth errors, rate limits, network issues)
- Create predictable test scenarios with known repositories and organizations
- Test configuration handling without real forge credentials

## Basic Usage

### Creating a Test Forge

```go
import "git.home.luguber.info/inful/docbuilder/internal/testforge"
import "git.home.luguber.info/inful/docbuilder/internal/config"

// Create a basic test forge
forge := testforge.NewTestForge("my-test-forge", config.ForgeGitHub)

// Clear default test data if you want a clean slate
forge.ClearRepositories()
forge.SetOrganizations([]string{}) // Clear organizations

// Add your own test data
testRepo := testforge.TestRepository{
    Name:        "docs-repo",
    FullName:    "myorg/docs-repo", 
    CloneURL:    "https://github.com/myorg/docs-repo.git",
    Description: "Test documentation",
    Topics:      []string{"docs", "testing"},
    Language:    "Markdown",
    Private:     false,
    Archived:    false,
}

forge.AddRepository(testRepo)
forge.AddOrganization("myorg")
```

### Using the Factory Pattern

```go
factory := testforge.NewTestForgeFactory()

// Create pre-configured forges for different platforms
githubForge := factory.CreateGitHubTestForge("github-test")
gitlabForge := factory.CreateGitLabTestForge("gitlab-test")  
forgejoForge := factory.CreateForgejoTestForge("forgejo-test")
```

### Configuration Generation

```go
// Generate test forge configuration
forgeConfig := testforge.CreateTestForgeConfig(
    "test-github",
    config.ForgeGitHub,
    []string{"test-org", "docs-org"},
)

// Use in your tests
config := &config.Config{
    Forges: []*config.ForgeConfig{&forgeConfig},
    // ... other config
}
```

## Testing Different Scenarios

### Failure Mode Testing

```go
forge := testforge.NewTestForge("failing-forge", config.ForgeGitHub)

// Test authentication failures
forge.SetFailMode(testforge.FailModeAuth)
_, err := forge.GetUserOrganizations(ctx)
// err will be an authentication error

// Test rate limiting
forge.SetFailMode(testforge.FailModeRateLimit)
_, err = forge.GetUserOrganizations(ctx)
// err will be a rate limit error

// Test network issues
forge.SetFailMode(testforge.FailModeNetwork)

// Disable failures to test recovery
forge.SetFailMode(testforge.FailModeNone)
```

### Predefined Test Scenarios

```go
scenarios := testforge.CreateTestScenarios()

for _, scenario := range scenarios {
    t.Run(scenario.Name, func(t *testing.T) {
        // Each scenario provides:
        // - scenario.Forges: configured test forges
        // - scenario.Expected: expected results
        // - scenario.Description: what this tests
        
        forge := scenario.Forges[0]
        orgs, err := forge.GetUserOrganizations(ctx)
        
        if len(orgs) != scenario.Expected.TotalRepositories {
            t.Errorf("Expected %d orgs, got %d", 
                scenario.Expected.TotalRepositories, len(orgs))
        }
    })
}
```

## Integration with Existing Tests

### CLI Tests

Replace external forge dependencies in CLI tests:

```go
func TestCLICommand(t *testing.T) {
    // Instead of real GitHub API
    forge := testforge.NewTestForge("test", config.ForgeGitHub)
    
    // Create test config that uses this forge
    config := testforge.CreateTestForgeConfig("test", config.ForgeGitHub, []string{"test-org"})
    
    // Use in CLI executor
    executor := cli.NewCommandExecutor(/* ... */)
    result := executor.Build(config)
    
    // Test results without external dependencies
}
```

### Service Tests

Use test forges in service integration tests:

```go
func TestServiceDiscovery(t *testing.T) {
    forge := testforge.NewTestForge("service-test", config.ForgeGitHub)
    
    // Add test repositories with specific characteristics
    forge.AddRepository(testforge.TestRepository{
        Name: "has-docs",
        Topics: []string{"documentation"},
        // ...
    })
    
    // Inject test forge into service
    service := discovery.NewService(forge)
    repos, err := service.DiscoverRepositories(ctx)
    
    // Assert expected behavior
}
```

## Available Failure Modes

| Mode | Description | Simulates |
|------|-------------|-----------|
| `FailModeNone` | Normal operation | Success cases |
| `FailModeAuth` | Authentication errors | Invalid tokens, expired credentials |
| `FailModeNetwork` | Network connectivity issues | Timeouts, connection refused |
| `FailModeRateLimit` | API rate limiting | Exceeded request quotas |
| `FailModeNotFound` | Resource not found | Missing repos, orgs |

## Test Repository Fields

The `TestRepository` struct supports all forge repository attributes:

```go
type TestRepository struct {
    Name        string            // Repository name
    FullName    string            // Full name (org/repo)
    CloneURL    string            // HTTPS clone URL
    Description string            // Repository description  
    Topics      []string          // Repository topics/tags
    Language    string            // Primary language
    Private     bool              // Private repository flag
    Archived    bool              // Archived repository flag
    Fork        bool              // Forked repository flag
    CreatedAt   time.Time         // Creation timestamp
    UpdatedAt   time.Time         // Last update timestamp
}
```

The test forge automatically converts these to the standard `forge.Repository` format.

## Best Practices

1. **Clear Defaults**: Always call `ClearRepositories()` and clear organizations if you want predictable test data
2. **Realistic Data**: Use realistic repository names, URLs, and metadata for better test coverage
3. **Test Failures**: Include failure mode testing to ensure error handling works correctly
4. **Scenario Reuse**: Use predefined scenarios for common test cases, create custom ones for specific needs
5. **Isolation**: Each test should use its own test forge instance to avoid interference

## Migration Plan for Existing Tests

### 🎯 **Primary Integration Opportunities**

#### 1. **Replace Existing MockForgeClient** (`internal/forge/mock_test.go`) - **HIGH PRIORITY**
**Current State**: Basic mock with limited functionality  
**Enhancement**: Replace with comprehensive test forge

**Benefits**:
- **Advanced failure simulation**: Sophisticated error scenarios (auth failures, rate limiting, network timeouts)
- **Factory patterns**: Easy creation of pre-configured test forges for different platforms
- **Configuration generation**: Automatic creation of valid forge configurations for tests
- **Enhanced data management**: Better organization and repository management

**Migration Strategy**:
```go
// Before: Basic mock
client := NewMockForgeClient("test-github", ForgeTypeGitHub)

// After: Rich test forge
forge := testforge.NewGitHubTestForge("test-github")
// OR with failure simulation
forge := testforge.NewTestForge("test-github", config.ForgeGitHub).
    WithAuthFailure().
    WithRateLimit(100, time.Hour)
```

#### 2. **Enhance Integration Summary Tests** (`internal/forge/integration_summary_test.go`) - **MEDIUM PRIORITY**
**Current Usage**: Tests basic forge manager functionality  
**Enhancement**: Add comprehensive failure mode testing

**Specific Improvements**:
- Test forge discovery with auth failures
- Test repository filtering with network timeouts  
- Test webhook functionality with various error conditions
- Test multi-forge scenarios with different failure states

#### 3. **Upgrade Discovery Tests** (`internal/docs/discovery_test.go`) - **MEDIUM PRIORITY**
**Current Tests**: `TestForgeNamespacingModes`, `TestForgeNamespacingAutoSingleForge`  
**Enhancement**: Add real forge discovery scenarios

**Integration Points**:
```go
// Enhanced forge namespacing tests with realistic data
forge := testforge.NewGitHubTestForge("test-github").
    WithOrganization("acme-corp", "ACME Corporation").
    WithRepository("acme-corp", "docs-site", true).
    WithRepository("acme-corp", "api-docs", true)

config := forge.GenerateForgeConfig()
// Test discovery with real-world forge structure
```

#### 4. **Improve CLI Integration Tests** (`internal/cli/command_executor_test.go`) - **LOW PRIORITY**
**Current State**: Uses minimal static config  
**Enhancement**: Test CLI with dynamic forge configurations

#### 5. **Enhance Daemon Integration Tests** (`internal/daemon/integration_test.go`) - **LOW PRIORITY**
**Current State**: Basic daemon lifecycle testing  
**Enhancement**: Add forge-aware daemon testing

#### 6. **Upgrade Webhook Tests** (`internal/forge/webhook_test.go`) - **MEDIUM PRIORITY**
**Current State**: Basic webhook validation  
**Enhancement**: Complete webhook lifecycle testing

### 📈 **Value Proposition**

The test forge provides **immediate value** by:

1. **Replacing 200+ lines** of basic mock code with **400+ lines** of comprehensive testing infrastructure
2. **Adding failure mode testing** that currently doesn't exist
3. **Enabling realistic multi-platform scenarios** for better integration testing
4. **Providing factory patterns** that reduce test setup boilerplate by ~60%
5. **Supporting configuration generation** for consistent test environments

## Example: Complete Test Suite

```go
func TestDocBuilderWithTestForge(t *testing.T) {
    // Create test forge with realistic data
    forge := testforge.NewTestForge("integration-test", config.ForgeGitHub)
    forge.ClearRepositories()
    
    // Add realistic test repositories
    forge.AddRepository(testforge.TestRepository{
        Name:        "api-docs",
        FullName:    "acme-corp/api-docs",
        CloneURL:    "https://github.com/acme-corp/api-docs.git",
        Description: "API documentation for Acme Corp",
        Topics:      []string{"api", "documentation", "openapi"},
        Language:    "Markdown",
        Private:     false,
        Archived:    false,
    })
    
    forge.AddOrganization("acme-corp")
    
    // Create configuration
    config := &config.Config{
        Forges: []*config.ForgeConfig{
            testforge.CreateTestForgeConfig(
                "test-github", 
                config.ForgeGitHub, 
                []string{"acme-corp"},
            ),
        },
        Output: config.OutputConfig{
            Directory: t.TempDir(),
        },
    }
    
    // Test full pipeline
    executor := cli.NewCommandExecutor(/* dependencies */)
    result := executor.Build(*config)
    
    if !result.Success {
        t.Errorf("Build failed: %v", result.Error)
    }
    
    // Verify outputs without external API calls
}
```

This test forge implementation provides comprehensive testing capabilities while eliminating external dependencies and ensuring reproducible test results.