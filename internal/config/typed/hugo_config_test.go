package typed

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

func TestNormalizeHugoTheme(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"relearn", "relearn", RelearnTheme},
		{"hextra normalized", "hextra", RelearnTheme},   // Always normalize to relearn
		{"docsy normalized", "docsy", RelearnTheme},     // Always normalize to relearn
		{"invalid normalized", "invalid", RelearnTheme}, // Always normalize to relearn
		{"empty normalized", "", RelearnTheme},          // Always normalize to relearn
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeHugoTheme(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRelearnThemeConstants(t *testing.T) {
	assert.Equal(t, "relearn", RelearnTheme)
	assert.Equal(t, "github.com/McShelby/hugo-theme-relearn", RelearnModulePath)
}

func TestTypedHugoConfig_Validation(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := HugoConfig{
			Title:      "Test Site",
			BaseURL:    foundation.Some("https://example.com"),
			ContentDir: "content",
			PublishDir: "public",
			Params: HugoParams{
				Author: foundation.Some("Test Author"),
				EditLinks: EditLinksConfig{
					Enabled: true,
					PerPage: true,
				},
				Navigation: NavigationConfig{
					ShowTOC:     true,
					TOCMaxDepth: 3,
				},
			},
		}

		result := config.Validate()
		assert.True(t, result.Valid, "Config should be valid: %v", result.Errors)
	})

	t.Run("empty title", func(t *testing.T) {
		config := HugoConfig{
			Title: "",
		}

		result := config.Validate()
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "title", result.Errors[0].Field)
	})

	t.Run("invalid baseURL", func(t *testing.T) {
		config := HugoConfig{
			Title:   "Test Site",
			BaseURL: foundation.Some("://not-a-valid-url"), // More clearly invalid URL
		}

		result := config.Validate()
		assert.False(t, result.Valid, "Config should be invalid with malformed URL")
		hasBaseURLError := false
		for _, err := range result.Errors {
			if err.Field == "baseURL" {
				hasBaseURLError = true
				break
			}
		}
		assert.True(t, hasBaseURLError, "Should have baseURL validation error")
	})

	t.Run("invalid content directory", func(t *testing.T) {
		config := HugoConfig{
			Title:      "Test Site",
			ContentDir: "../../../etc/passwd", // directory traversal
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})
}

func TestDaemonModeType(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expected         DaemonModeType
		requiresHTTP     bool
		supportsWebhooks bool
	}{
		{"http", "http", DaemonModeHTTP, true, false},
		{"webhook", "webhook", DaemonModeWebhook, true, true},
		{"scheduled", "scheduled", DaemonModeScheduled, false, false},
		{"api", "api", DaemonModeAPI, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDaemonModeType(tt.input)
			assert.True(t, result.IsOk())

			mode := result.Unwrap()
			assert.Equal(t, tt.expected, mode)
			assert.True(t, mode.Valid())
			assert.Equal(t, tt.requiresHTTP, mode.RequiresHTTPServer())
			assert.Equal(t, tt.supportsWebhooks, mode.SupportsWebhooks())
		})
	}

	t.Run("invalid mode", func(t *testing.T) {
		result := ParseDaemonModeType("invalid")
		assert.True(t, result.IsErr())
	})
}

func TestLogLevelType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected LogLevelType
	}{
		{"debug", "debug", LogLevelDebug},
		{"info", "info", LogLevelInfo},
		{"warn", "warn", LogLevelWarn},
		{"error", "error", LogLevelError},
		{"fatal", "fatal", LogLevelFatal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLogLevelType(tt.input)
			assert.True(t, result.IsOk())

			level := result.Unwrap()
			assert.Equal(t, tt.expected, level)
			assert.True(t, level.Valid())
		})
	}

	t.Run("invalid level", func(t *testing.T) {
		result := ParseLogLevelType("invalid")
		assert.True(t, result.IsErr())
	})
}

func TestTypedDaemonConfig_Validation(t *testing.T) {
	t.Run("valid http config", func(t *testing.T) {
		config := DaemonConfig{
			Mode: DaemonModeHTTP,
			Server: ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
			Logging: LoggingConfig{
				Level:      LogLevelInfo,
				Structured: false,
			},
			Build:       BuildConfig{},
			Security:    SecurityConfig{},
			Performance: PerformanceConfig{},
			Storage:     StorageConfig{},
		}

		result := config.Validate()
		assert.True(t, result.Valid, "Config should be valid: %v", result.Errors)
	})

	t.Run("webhook mode requires webhook config", func(t *testing.T) {
		config := DaemonConfig{
			Mode: DaemonModeWebhook,
			Server: ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
			Logging: LoggingConfig{
				Level: LogLevelInfo,
			},
			Build:       BuildConfig{},
			Security:    SecurityConfig{},
			Performance: PerformanceConfig{},
			Storage:     StorageConfig{},
			// Missing webhook configuration
		}

		result := config.Validate()
		assert.False(t, result.Valid)

		hasWebhookError := false
		for _, err := range result.Errors {
			if err.Field == "webhook" {
				hasWebhookError = true
				break
			}
		}
		assert.True(t, hasWebhookError)
	})

	t.Run("scheduled mode requires schedule config", func(t *testing.T) {
		config := DaemonConfig{
			Mode: DaemonModeScheduled,
			Server: ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
			Logging: LoggingConfig{
				Level: LogLevelInfo,
			},
			Build:       BuildConfig{},
			Security:    SecurityConfig{},
			Performance: PerformanceConfig{},
			Storage:     StorageConfig{},
			// Missing schedule configuration
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})

	t.Run("invalid server port", func(t *testing.T) {
		config := DaemonConfig{
			Mode: DaemonModeHTTP,
			Server: ServerConfig{
				Host: "localhost",
				Port: 99999, // invalid port
			},
			Logging: LoggingConfig{
				Level: LogLevelInfo,
			},
			Build:       BuildConfig{},
			Security:    SecurityConfig{},
			Performance: PerformanceConfig{},
			Storage:     StorageConfig{},
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})

	t.Run("empty server host", func(t *testing.T) {
		config := DaemonConfig{
			Mode: DaemonModeHTTP,
			Server: ServerConfig{
				Host: "", // empty host
				Port: 8080,
			},
			Logging: LoggingConfig{
				Level: LogLevelInfo,
			},
			Build:       BuildConfig{},
			Security:    SecurityConfig{},
			Performance: PerformanceConfig{},
			Storage:     StorageConfig{},
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})
}

func TestTypedServerConfig_Validation(t *testing.T) {
	t.Run("valid server config", func(t *testing.T) {
		config := ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  foundation.Some(30 * time.Second),
			WriteTimeout: foundation.Some(30 * time.Second),
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("invalid host", func(t *testing.T) {
		config := ServerConfig{
			Host: "invalid..hostname",
			Port: 8080,
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})

	t.Run("invalid port range", func(t *testing.T) {
		configs := []ServerConfig{
			{Host: "localhost", Port: 0},
			{Host: "localhost", Port: -1},
			{Host: "localhost", Port: 65536},
		}

		for _, config := range configs {
			result := config.Validate()
			assert.False(t, result.Valid, "Port %d should be invalid", config.Port)
		}
	})
}

func TestTypedTLSConfig_Validation(t *testing.T) {
	t.Run("disabled TLS", func(t *testing.T) {
		config := TLSConfig{
			Enabled: false,
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("auto TLS", func(t *testing.T) {
		config := TLSConfig{
			Enabled: true,
			Auto:    true,
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("manual TLS with cert files", func(t *testing.T) {
		config := TLSConfig{
			Enabled:  true,
			Auto:     false,
			CertFile: foundation.Some("cert.pem"),
			KeyFile:  foundation.Some("key.pem"),
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("manual TLS missing cert files", func(t *testing.T) {
		config := TLSConfig{
			Enabled: true,
			Auto:    false,
			// Missing cert and key files
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})

	t.Run("manual TLS same cert and key file", func(t *testing.T) {
		config := TLSConfig{
			Enabled:  true,
			Auto:     false,
			CertFile: foundation.Some("same.pem"),
			KeyFile:  foundation.Some("same.pem"),
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})
}

func TestTypedWebhookConfig_Validation(t *testing.T) {
	t.Run("disabled webhook", func(t *testing.T) {
		config := WebhookConfig{
			Enabled: false,
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("valid webhook config", func(t *testing.T) {
		config := WebhookConfig{
			Enabled:        true,
			Path:           foundation.Some("/webhook"),
			MaxPayloadSize: foundation.Some(1024 * 1024), // 1MB
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("invalid webhook path", func(t *testing.T) {
		config := WebhookConfig{
			Enabled: true,
			Path:    foundation.Some("webhook"), // missing leading slash
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})

	t.Run("invalid payload size", func(t *testing.T) {
		configs := []WebhookConfig{
			{Enabled: true, MaxPayloadSize: foundation.Some(100)},               // too small
			{Enabled: true, MaxPayloadSize: foundation.Some(200 * 1024 * 1024)}, // too large
		}

		for _, config := range configs {
			result := config.Validate()
			assert.False(t, result.Valid)
		}
	})
}

func TestTypedScheduleConfig_Validation(t *testing.T) {
	t.Run("disabled schedule", func(t *testing.T) {
		config := ScheduleConfig{
			Enabled: false,
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("cron schedule", func(t *testing.T) {
		config := ScheduleConfig{
			Enabled: true,
			Cron:    foundation.Some("0 */6 * * *"),
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("interval schedule", func(t *testing.T) {
		config := ScheduleConfig{
			Enabled:  true,
			Interval: foundation.Some(1 * time.Hour),
		}

		result := config.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("missing cron and interval", func(t *testing.T) {
		config := ScheduleConfig{
			Enabled: true,
			// Neither cron nor interval specified
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})

	t.Run("both cron and interval", func(t *testing.T) {
		config := ScheduleConfig{
			Enabled:  true,
			Cron:     foundation.Some("0 */6 * * *"),
			Interval: foundation.Some(1 * time.Hour),
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})
}
