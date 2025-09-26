package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Repositories []Repository `yaml:"repositories"`
	Hugo         HugoConfig   `yaml:"hugo"`
	Output       OutputConfig `yaml:"output"`
}

// Repository represents a Git repository to process
type Repository struct {
	URL    string            `yaml:"url"`
	Name   string            `yaml:"name"`
	Branch string            `yaml:"branch,omitempty"`
	Auth   *AuthConfig       `yaml:"auth,omitempty"`
	Paths  []string          `yaml:"paths,omitempty"` // Specific paths to docs, defaults to ["docs"]
	Tags   map[string]string `yaml:"tags,omitempty"`  // Additional metadata
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type     string `yaml:"type"` // "ssh", "token", "basic"
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Token    string `yaml:"token,omitempty"`
	KeyPath  string `yaml:"key_path,omitempty"`
}

// HugoConfig represents Hugo-specific configuration
type HugoConfig struct {
	Theme       string            `yaml:"theme,omitempty"`
	BaseURL     string            `yaml:"base_url,omitempty"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description,omitempty"`
	Params      map[string]any    `yaml:"params,omitempty"`
	Menu        map[string][]Menu `yaml:"menu,omitempty"`
}

// Menu represents a Hugo menu item
type Menu struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight,omitempty"`
}

// OutputConfig represents output configuration
type OutputConfig struct {
	Directory string `yaml:"directory"`
	Clean     bool   `yaml:"clean"` // Clean output directory before build
}

// Load loads configuration from the specified file
func Load(configPath string) (*Config, error) {
	// Load .env file if it exists
	if err := loadEnvFile(); err != nil {
		// Don't fail if .env doesn't exist, just log it
		fmt.Fprintf(os.Stderr, "Note: .env file not found or couldn't be loaded: %v\n", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the YAML content
	expandedData := os.ExpandEnv(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply defaults
	if config.Hugo.Title == "" {
		config.Hugo.Title = "Documentation Site"
	}
	// Default theme to Hextra if not specified
	if config.Hugo.Theme == "" {
		config.Hugo.Theme = "hextra"
	}
	if config.Output.Directory == "" {
		config.Output.Directory = "./site"
	}
	// Default to clean output directory
	if config.Output.Directory != "" {
		// Only set default if Output was specified
		config.Output.Clean = true
	}

	// Set default paths for repositories
	for i := range config.Repositories {
		if len(config.Repositories[i].Paths) == 0 {
			config.Repositories[i].Paths = []string{"docs"}
		}
		if config.Repositories[i].Branch == "" {
			config.Repositories[i].Branch = "main"
		}
	}

	return &config, nil
}

// Init creates a new configuration file with example content
func Init(configPath string, force bool) error {
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists: %s (use --force to overwrite)", configPath)
	}

	exampleConfig := Config{
		Repositories: []Repository{
			{
				URL:    "https://github.com/example/repo1.git",
				Name:   "repo1",
				Branch: "main",
				Paths:  []string{"docs"},
			},
			{
				URL:    "https://github.com/example/repo2.git",
				Name:   "repo2",
				Branch: "main",
				Paths:  []string{"docs", "documentation"},
				Auth: &AuthConfig{
					Type:  "token",
					Token: "YOUR_GITHUB_TOKEN",
				},
			},
		},
		Hugo: HugoConfig{
			Title:       "My Documentation Site",
			Description: "Aggregated documentation from multiple repositories",
			BaseURL:     "https://example.com",
			Theme:       "hextra",
			Params: map[string]any{
				"search":  map[string]any{"enable": true, "type": "flexsearch"},
				"theme":   map[string]any{"default": "system", "displayToggle": true},
				"mermaid": map[string]any{},
			},
		},
		Output: OutputConfig{
			Directory: "./site",
			Clean:     true,
		},
	}

	data, err := yaml.Marshal(&exampleConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// loadEnvFile loads environment variables from .env file
func loadEnvFile() error {
	envPaths := []string{".env", ".env.local"}

	for _, envPath := range envPaths {
		if err := loadSingleEnvFile(envPath); err == nil {
			fmt.Fprintf(os.Stderr, "Loaded environment variables from %s\n", envPath)
			return nil
		}
	}

	return fmt.Errorf("no .env file found")
}

// loadSingleEnvFile loads environment variables from a single file
func loadSingleEnvFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return err
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Only set if not already set in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}
