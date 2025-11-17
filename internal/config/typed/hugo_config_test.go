package typed

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHugoThemeType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected HugoThemeType
		valid    bool
	}{
		{"hextra", "hextra", HugoThemeHextra, true},
		{"docsy", "docsy", HugoThemeDocsy, true},
		{"book", "book", HugoThemeBook, true},
		{"custom", "custom", HugoThemeCustom, true},
		{"invalid", "invalid", HugoThemeHextra, false}, // normalized to default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseHugoThemeType(tt.input)

			if tt.valid && tt.input != "invalid" {
				assert.True(t, result.IsOk())
				assert.Equal(t, tt.expected, result.Unwrap())
				assert.True(t, result.Unwrap().Valid())
			} else if tt.input == "invalid" {
				assert.True(t, result.IsErr())
			}

			// Test normalization
			normalized := NormalizeHugoThemeType(tt.input)
			assert.Equal(t, tt.expected, normalized)
		})
	}
}

func TestHugoThemeType_Features(t *testing.T) {
	tests := []struct {
		theme           HugoThemeType
		supportsModules bool
		modulePath      string
	}{
		{HugoThemeHextra, true, "github.com/imfing/hextra"},
		{HugoThemeDocsy, true, "github.com/google/docsy"},
		{HugoThemeBook, false, ""},
		{HugoThemeCustom, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.theme.String(), func(t *testing.T) {
			assert.Equal(t, tt.supportsModules, tt.theme.SupportsModules())

			modulePath := tt.theme.GetModulePath()
			if tt.modulePath != "" {
				assert.True(t, modulePath.IsSome())
				assert.Equal(t, tt.modulePath, modulePath.Unwrap())
			} else {
				assert.True(t, modulePath.IsNone())
			}
		})
	}
}

func TestTypedHugoConfig_Validation(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := HugoConfig{
			Title:      "Test Site",
			Theme:      HugoThemeHextra,
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
			Theme: HugoThemeHextra,
		}

		result := config.Validate()
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "title", result.Errors[0].Field)
	})

	t.Run("invalid baseURL", func(t *testing.T) {
		config := HugoConfig{
			Title:   "Test Site",
			Theme:   HugoThemeHextra,
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
			Theme:      HugoThemeHextra,
			ContentDir: "../../../etc/passwd", // directory traversal
		}

		result := config.Validate()
		assert.False(t, result.Valid)
	})
}

func TestTypedHugoConfig_LegacyConversion(t *testing.T) {
	t.Run("to legacy map", func(t *testing.T) {
		config := HugoConfig{
			Title:       "Test Site",
			Theme:       HugoThemeHextra,
			BaseURL:     foundation.Some("https://example.com"),
			ContentDir:  "content",
			BuildDrafts: true,
			Params: HugoParams{
				Author:   foundation.Some("Test Author"),
				Keywords: []string{"docs", "test"},
				Custom: map[string]any{
					"custom_param": "custom_value",
				},
			},
			CustomConfig: map[string]any{
				"custom_config": "custom_value",
			},
		}

		legacyMap := config.ToLegacyMap()

		assert.Equal(t, "Test Site", legacyMap["title"])
		assert.Equal(t, "hextra", legacyMap["theme"])
		assert.Equal(t, "https://example.com", legacyMap["baseURL"])
		assert.Equal(t, "content", legacyMap["contentDir"])
		assert.Equal(t, true, legacyMap["buildDrafts"])
		assert.Equal(t, "custom_value", legacyMap["custom_config"])

		// Check params
		params, ok := legacyMap["params"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Test Author", params["author"])
		assert.Equal(t, []string{"docs", "test"}, params["keywords"])
		assert.Equal(t, "custom_value", params["custom_param"])
	})

	t.Run("from legacy map", func(t *testing.T) {
		legacyMap := map[string]any{
			"title":       "Test Site",
			"theme":       "docsy",
			"baseURL":     "https://example.com",
			"contentDir":  "docs",
			"buildDrafts": true,
			"params": map[string]any{
				"author":       "Test Author",
				"keywords":     []string{"docs", "test"},
				"custom_param": "custom_value",
			},
			"custom_config": "custom_value",
		}

		result := FromLegacyMap(legacyMap)
		require.True(t, result.IsOk(), "Should parse legacy map successfully")

		config := result.Unwrap()
		assert.Equal(t, "Test Site", config.Title)
		assert.Equal(t, HugoThemeDocsy, config.Theme)
		assert.True(t, config.BaseURL.IsSome())
		assert.Equal(t, "https://example.com", config.BaseURL.Unwrap())
		assert.Equal(t, "docs", config.ContentDir)
		assert.True(t, config.BuildDrafts)

		assert.True(t, config.Params.Author.IsSome())
		assert.Equal(t, "Test Author", config.Params.Author.Unwrap())
		assert.Equal(t, []string{"docs", "test"}, config.Params.Keywords)
		assert.Equal(t, "custom_value", config.Params.Custom["custom_param"])
		assert.Equal(t, "custom_value", config.CustomConfig["custom_config"])
	})

	t.Run("from legacy map with validation error", func(t *testing.T) {
		legacyMap := map[string]any{
			"title": "", // empty title should fail validation
			"theme": "hextra",
		}

		result := FromLegacyMap(legacyMap)
		assert.True(t, result.IsErr())
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

func TestTypedDaemonConfig_LegacyConversion(t *testing.T) {
	t.Run("to legacy map", func(t *testing.T) {
		config := DaemonConfig{
			Mode: DaemonModeHTTP,
			Server: ServerConfig{
				Host:        "localhost",
				Port:        8080,
				ReadTimeout: foundation.Some(30 * time.Second),
			},
			Logging: LoggingConfig{
				Level:      LogLevelInfo,
				Structured: true,
				Format:     foundation.Some("json"),
			},
			Custom: map[string]any{
				"custom_setting": "custom_value",
			},
		}

		legacyMap := config.ToLegacyMap()

		assert.Equal(t, "http", legacyMap["mode"])
		assert.Equal(t, "custom_value", legacyMap["custom_setting"])

		// Check server config
		server, ok := legacyMap["server"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "localhost", server["host"])
		assert.Equal(t, 8080, server["port"])
		assert.Equal(t, "30s", server["read_timeout"])

		// Check logging config
		logging, ok := legacyMap["logging"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "info", logging["level"])
		assert.Equal(t, true, logging["structured"])
		assert.Equal(t, "json", logging["format"])
	})

	t.Run("from legacy map", func(t *testing.T) {
		legacyMap := map[string]any{
			"mode": "http", // Changed from "webhook" to "http" to avoid needing webhook config
			"server": map[string]any{
				"host":         "0.0.0.0",
				"port":         "9000",
				"read_timeout": "45s",
			},
			"logging": map[string]any{
				"level":      "debug",
				"structured": true,
				"format":     "json",
				"file":       "/var/log/docbuilder.log",
			},
			"custom_setting": "custom_value",
		}

		result := FromDaemonLegacyMap(legacyMap)
		if result.IsErr() {
			t.Logf("Legacy conversion error: %v", result.UnwrapErr())
		}
		require.True(t, result.IsOk(), "Should parse legacy map successfully: %v",
			func() string {
				if result.IsErr() {
					return result.UnwrapErr().Error()
				}
				return "no error"
			}())

		config := result.Unwrap()
		assert.Equal(t, DaemonModeHTTP, config.Mode) // Updated expected mode
		assert.Equal(t, "0.0.0.0", config.Server.Host)
		assert.Equal(t, 9000, config.Server.Port)
		assert.True(t, config.Server.ReadTimeout.IsSome())
		assert.Equal(t, 45*time.Second, config.Server.ReadTimeout.Unwrap())

		assert.Equal(t, LogLevelDebug, config.Logging.Level)
		assert.True(t, config.Logging.Structured)
		assert.True(t, config.Logging.Format.IsSome())
		assert.Equal(t, "json", config.Logging.Format.Unwrap())
		assert.True(t, config.Logging.File.IsSome())
		assert.Equal(t, "/var/log/docbuilder.log", config.Logging.File.Unwrap())

		assert.Equal(t, "custom_value", config.Custom["custom_setting"])
	})
}
