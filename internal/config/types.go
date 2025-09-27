package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Repository represents a Git repository to process (shared between v2 config and generator logic)
type Repository struct {
	URL    string            `yaml:"url"`
	Name   string            `yaml:"name"`
	Branch string            `yaml:"branch,omitempty"`
	Auth   *AuthConfig       `yaml:"auth,omitempty"`
	Paths  []string          `yaml:"paths,omitempty"` // Specific paths to docs, defaults to ["docs"] (applied in defaults)
	Tags   map[string]string `yaml:"tags,omitempty"`  // Additional metadata
}

// AuthConfig represents authentication configuration
// AuthType enumerates supported authentication methods (stringly for YAML compatibility)
type AuthType string

const (
	AuthTypeNone  AuthType = "none"
	AuthTypeSSH   AuthType = "ssh"
	AuthTypeToken AuthType = "token"
	AuthTypeBasic AuthType = "basic"
)

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type     AuthType `yaml:"type"` // ssh|token|basic|none
	Username string   `yaml:"username,omitempty"`
	Password string   `yaml:"password,omitempty"`
	Token    string   `yaml:"token,omitempty"`
	KeyPath  string   `yaml:"key_path,omitempty"`
}

// IsZero reports whether no auth method specified.
func (a *AuthConfig) IsZero() bool { return a == nil || a.Type == "" || a.Type == AuthTypeNone }

// HugoConfig represents Hugo-specific configuration (embedded in V2Config)
type HugoConfig struct {
	Theme       string            `yaml:"theme,omitempty"`
	BaseURL     string            `yaml:"base_url,omitempty"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description,omitempty"`
	Params      map[string]any    `yaml:"params,omitempty"`
	Menu        map[string][]Menu `yaml:"menu,omitempty"`
}

// Theme constants to avoid magic strings across generator logic.
const (
	ThemeHextra = "hextra"
	ThemeDocsy  = "docsy"
)

// BuildConfig holds build performance tuning knobs.
// Additional fields (retry limits, timeouts, etc.) can be added iteratively without
// breaking existing configurations. All zero values trigger sensible defaults.
type BuildConfig struct {
	// CloneConcurrency caps the number of repositories cloned in parallel within a single build.
	// Defaults to 4; values <1 are coerced to 1; values larger than the repo count are bounded.
	CloneConcurrency int `yaml:"clone_concurrency,omitempty"`
	// Retry policy fields (apply to transient build failures at stage granularity)
	MaxRetries        int    `yaml:"max_retries,omitempty"`         // total retry attempts after first attempt (default 2)
	RetryBackoff      string `yaml:"retry_backoff,omitempty"`       // fixed|linear|exponential (default linear)
	RetryInitialDelay string `yaml:"retry_initial_delay,omitempty"` // duration string (default 1s)
	RetryMaxDelay     string `yaml:"retry_max_delay,omitempty"`     // cap for exponential (default 30s)
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

// loadEnvFile loads environment variables from .env/.env.local files (shared with v2 loader)
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
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
