package typed

import (
	"fmt"
	"net"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// DaemonModeType represents the daemon operation mode.
type DaemonModeType struct {
	value string
}

// Predefined daemon modes.
var (
	DaemonModeHTTP      = DaemonModeType{"http"}
	DaemonModeWebhook   = DaemonModeType{"webhook"}
	DaemonModeScheduled = DaemonModeType{"scheduled"}
	DaemonModeAPI       = DaemonModeType{"api"}

	// Normalizer for daemon modes.
	daemonModeNormalizer = foundation.NewNormalizer(map[string]DaemonModeType{
		"http":      DaemonModeHTTP,
		"webhook":   DaemonModeWebhook,
		"scheduled": DaemonModeScheduled,
		"api":       DaemonModeAPI,
	}, DaemonModeHTTP) // default to HTTP

	// Validator for daemon modes.
	daemonModeValidator = foundation.OneOf("daemon_mode", []DaemonModeType{
		DaemonModeHTTP, DaemonModeWebhook, DaemonModeScheduled, DaemonModeAPI,
	})
)

// String returns the string representation of the daemon mode.
func (dm DaemonModeType) String() string {
	return dm.value
}

// Valid checks if the daemon mode is valid.
func (dm DaemonModeType) Valid() bool {
	return daemonModeValidator(dm).Valid
}

// RequiresHTTPServer indicates if this mode requires an HTTP server.
func (dm DaemonModeType) RequiresHTTPServer() bool {
	switch dm {
	case DaemonModeHTTP, DaemonModeWebhook, DaemonModeAPI:
		return true
	default:
		return false
	}
}

// SupportsWebhooks indicates if this mode supports webhook processing.
func (dm DaemonModeType) SupportsWebhooks() bool {
	switch dm {
	case DaemonModeWebhook, DaemonModeAPI:
		return true
	default:
		return false
	}
}

// ParseDaemonModeType parses a string into a DaemonModeType.
func ParseDaemonModeType(s string) foundation.Result[DaemonModeType, error] {
	mode, err := daemonModeNormalizer.NormalizeWithError(s)
	if err != nil {
		return foundation.Err[DaemonModeType, error](
			errors.ValidationError(fmt.Sprintf("invalid daemon mode: %s", s)).
				WithContext("input", s).
				WithContext("valid_values", []string{"http", "webhook", "scheduled", "api"}).
				Build(),
		)
	}
	return foundation.Ok[DaemonModeType, error](mode)
}

// LogLevelType represents strongly-typed log levels.
type LogLevelType struct {
	value string
}

// Predefined log levels.
var (
	LogLevelDebug = LogLevelType{"debug"}
	LogLevelInfo  = LogLevelType{"info"}
	LogLevelWarn  = LogLevelType{"warn"}
	LogLevelError = LogLevelType{"error"}
	LogLevelFatal = LogLevelType{"fatal"}

	// Normalizer for log levels.
	logLevelNormalizer = foundation.NewNormalizer(map[string]LogLevelType{
		"debug": LogLevelDebug,
		"info":  LogLevelInfo,
		"warn":  LogLevelWarn,
		"error": LogLevelError,
		"fatal": LogLevelFatal,
	}, LogLevelInfo) // default to info

	// Validator for log levels.
	logLevelValidator = foundation.OneOf("log_level", []LogLevelType{
		LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, LogLevelFatal,
	})
)

// String returns the string representation of the log level.
func (ll LogLevelType) String() string {
	return ll.value
}

// Valid checks if the log level is valid.
func (ll LogLevelType) Valid() bool {
	return logLevelValidator(ll).Valid
}

// ParseLogLevelType parses a string into a LogLevelType.
func ParseLogLevelType(s string) foundation.Result[LogLevelType, error] {
	level, err := logLevelNormalizer.NormalizeWithError(s)
	if err != nil {
		return foundation.Err[LogLevelType, error](
			errors.ValidationError(fmt.Sprintf("invalid log level: %s", s)).
				WithContext("input", s).
				WithContext("valid_values", []string{"debug", "info", "warn", "error", "fatal"}).
				Build(),
		)
	}
	return foundation.Ok[LogLevelType, error](level)
}

// DaemonConfig represents a strongly-typed daemon configuration.
type DaemonConfig struct {
	// Operation mode
	Mode DaemonModeType `json:"mode" yaml:"mode"`

	// Server configuration
	Server ServerConfig `json:"server" yaml:"server"`

	// Logging configuration
	Logging LoggingConfig `json:"logging" yaml:"logging"`

	// Build configuration
	Build BuildConfig `json:"build" yaml:"build"`

	// Monitoring configuration
	Monitoring foundation.Option[MonitoringConfig] `json:"monitoring,omitempty" yaml:"monitoring,omitempty"`

	// Webhook configuration (if webhook mode)
	Webhook foundation.Option[WebhookConfig] `json:"webhook,omitempty" yaml:"webhook,omitempty"`

	// Scheduling configuration (if scheduled mode)
	Schedule foundation.Option[ScheduleConfig] `json:"schedule,omitempty" yaml:"schedule,omitempty"`

	// Security configuration
	Security SecurityConfig `json:"security" yaml:"security"`

	// Performance configuration
	Performance PerformanceConfig `json:"performance" yaml:"performance"`

	// Storage configuration
	Storage StorageConfig `json:"storage" yaml:"storage"`

	// Custom settings for extensibility
	Custom map[string]any `json:"custom,omitempty" yaml:"custom,omitempty"`
}

// ServerConfig represents HTTP server configuration.
type ServerConfig struct {
	Host         string                           `json:"host" yaml:"host"`
	Port         int                              `json:"port" yaml:"port"`
	ReadTimeout  foundation.Option[time.Duration] `json:"read_timeout,omitempty" yaml:"read_timeout,omitempty"`
	WriteTimeout foundation.Option[time.Duration] `json:"write_timeout,omitempty" yaml:"write_timeout,omitempty"`
	IdleTimeout  foundation.Option[time.Duration] `json:"idle_timeout,omitempty" yaml:"idle_timeout,omitempty"`
	TLS          foundation.Option[TLSConfig]     `json:"tls,omitempty" yaml:"tls,omitempty"`
	CORS         foundation.Option[CORSConfig]    `json:"cors,omitempty" yaml:"cors,omitempty"`
}

// TLSConfig represents TLS configuration.
type TLSConfig struct {
	Enabled  bool                      `json:"enabled" yaml:"enabled"`
	CertFile foundation.Option[string] `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`
	KeyFile  foundation.Option[string] `json:"key_file,omitempty" yaml:"key_file,omitempty"`
	Auto     bool                      `json:"auto" yaml:"auto"` // For automatic certificate generation
}

// CORSConfig represents CORS configuration.
type CORSConfig struct {
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	AllowedOrigins []string `json:"allowed_origins,omitempty" yaml:"allowed_origins,omitempty"`
	AllowedMethods []string `json:"allowed_methods,omitempty" yaml:"allowed_methods,omitempty"`
	AllowedHeaders []string `json:"allowed_headers,omitempty" yaml:"allowed_headers,omitempty"`
}

// LoggingConfig represents logging configuration.
type LoggingConfig struct {
	Level      LogLevelType              `json:"level" yaml:"level"`
	Format     foundation.Option[string] `json:"format,omitempty" yaml:"format,omitempty"`
	File       foundation.Option[string] `json:"file,omitempty" yaml:"file,omitempty"`
	MaxSize    foundation.Option[int]    `json:"max_size,omitempty" yaml:"max_size,omitempty"`
	Structured bool                      `json:"structured" yaml:"structured"`
}

// BuildConfig represents build execution configuration.
type BuildConfig struct {
	Timeout       foundation.Option[time.Duration] `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	MaxConcurrent foundation.Option[int]           `json:"max_concurrent,omitempty" yaml:"max_concurrent,omitempty"`
	RetryAttempts foundation.Option[int]           `json:"retry_attempts,omitempty" yaml:"retry_attempts,omitempty"`
	RetryDelay    foundation.Option[time.Duration] `json:"retry_delay,omitempty" yaml:"retry_delay,omitempty"`
	CleanupAfter  foundation.Option[time.Duration] `json:"cleanup_after,omitempty" yaml:"cleanup_after,omitempty"`
}

// MonitoringConfig represents monitoring and health check configuration.
type MonitoringConfig struct {
	Enabled     bool                             `json:"enabled" yaml:"enabled"`
	HealthCheck HealthCheckConfig                `json:"health_check" yaml:"health_check"`
	Metrics     foundation.Option[MetricsConfig] `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Tracing     foundation.Option[TracingConfig] `json:"tracing,omitempty" yaml:"tracing,omitempty"`
}

// HealthCheckConfig represents health check configuration.
type HealthCheckConfig struct {
	Enabled  bool                             `json:"enabled" yaml:"enabled"`
	Endpoint foundation.Option[string]        `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Interval foundation.Option[time.Duration] `json:"interval,omitempty" yaml:"interval,omitempty"`
}

// MetricsConfig represents metrics collection configuration.
type MetricsConfig struct {
	Enabled  bool                      `json:"enabled" yaml:"enabled"`
	Endpoint foundation.Option[string] `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Provider foundation.Option[string] `json:"provider,omitempty" yaml:"provider,omitempty"`
}

// TracingConfig represents distributed tracing configuration.
type TracingConfig struct {
	Enabled     bool                      `json:"enabled" yaml:"enabled"`
	ServiceName foundation.Option[string] `json:"service_name,omitempty" yaml:"service_name,omitempty"`
	Endpoint    foundation.Option[string] `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
}

// WebhookConfig represents webhook processing configuration.
type WebhookConfig struct {
	Enabled        bool                             `json:"enabled" yaml:"enabled"`
	Secret         foundation.Option[string]        `json:"secret,omitempty" yaml:"secret,omitempty"`
	Path           foundation.Option[string]        `json:"path,omitempty" yaml:"path,omitempty"`
	Timeout        foundation.Option[time.Duration] `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	MaxPayloadSize foundation.Option[int]           `json:"max_payload_size,omitempty" yaml:"max_payload_size,omitempty"`
	AuthRequired   bool                             `json:"auth_required" yaml:"auth_required"`
}

// ScheduleConfig represents scheduled build configuration.
type ScheduleConfig struct {
	Enabled  bool                             `json:"enabled" yaml:"enabled"`
	Cron     foundation.Option[string]        `json:"cron,omitempty" yaml:"cron,omitempty"`
	Interval foundation.Option[time.Duration] `json:"interval,omitempty" yaml:"interval,omitempty"`
	Timezone foundation.Option[string]        `json:"timezone,omitempty" yaml:"timezone,omitempty"`
}

// SecurityConfig represents security configuration.
type SecurityConfig struct {
	APIKey         foundation.Option[string]          `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	JWTSecret      foundation.Option[string]          `json:"jwt_secret,omitempty" yaml:"jwt_secret,omitempty"`
	RateLimit      foundation.Option[RateLimitConfig] `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty"`
	AllowedHosts   []string                           `json:"allowed_hosts,omitempty" yaml:"allowed_hosts,omitempty"`
	TrustedProxies []string                           `json:"trusted_proxies,omitempty" yaml:"trusted_proxies,omitempty"`
}

// RateLimitConfig represents rate limiting configuration.
type RateLimitConfig struct {
	Enabled           bool                             `json:"enabled" yaml:"enabled"`
	RequestsPerMinute foundation.Option[int]           `json:"requests_per_minute,omitempty" yaml:"requests_per_minute,omitempty"`
	BurstSize         foundation.Option[int]           `json:"burst_size,omitempty" yaml:"burst_size,omitempty"`
	WindowSize        foundation.Option[time.Duration] `json:"window_size,omitempty" yaml:"window_size,omitempty"`
}

// PerformanceConfig represents performance tuning configuration.
type PerformanceConfig struct {
	MaxWorkers     foundation.Option[int]           `json:"max_workers,omitempty" yaml:"max_workers,omitempty"`
	QueueSize      foundation.Option[int]           `json:"queue_size,omitempty" yaml:"queue_size,omitempty"`
	GCPercent      foundation.Option[int]           `json:"gc_percent,omitempty" yaml:"gc_percent,omitempty"`
	MemoryLimit    foundation.Option[string]        `json:"memory_limit,omitempty" yaml:"memory_limit,omitempty"`
	RequestTimeout foundation.Option[time.Duration] `json:"request_timeout,omitempty" yaml:"request_timeout,omitempty"`
}

// StorageConfig represents storage configuration.
type StorageConfig struct {
	WorkspaceDir foundation.Option[string]        `json:"workspace_dir,omitempty" yaml:"workspace_dir,omitempty"`
	StateFile    foundation.Option[string]        `json:"state_file,omitempty" yaml:"state_file,omitempty"`
	TempDir      foundation.Option[string]        `json:"temp_dir,omitempty" yaml:"temp_dir,omitempty"`
	CleanupAge   foundation.Option[time.Duration] `json:"cleanup_age,omitempty" yaml:"cleanup_age,omitempty"`
	MaxSize      foundation.Option[string]        `json:"max_size,omitempty" yaml:"max_size,omitempty"`
}

// Validation methods

// Validate performs comprehensive validation of the daemon configuration.
func (dc *DaemonConfig) Validate() foundation.ValidationResult {
	chain := foundation.NewValidatorChain(
		// Validate daemon mode
		func(config DaemonConfig) foundation.ValidationResult {
			return daemonModeValidator(config.Mode)
		},

		// Validate server configuration
		func(config DaemonConfig) foundation.ValidationResult {
			return config.Server.Validate()
		},

		// Validate logging configuration
		func(config DaemonConfig) foundation.ValidationResult {
			return config.Logging.Validate()
		},

		// Validate mode-specific configuration
		func(config DaemonConfig) foundation.ValidationResult {
			return config.validateModeSpecificConfig()
		},

		// Validate security configuration
		func(config DaemonConfig) foundation.ValidationResult {
			return config.Security.Validate()
		},
	)

	return chain.Validate(*dc)
}

// validateModeSpecificConfig validates configuration based on daemon mode.
func (dc *DaemonConfig) validateModeSpecificConfig() foundation.ValidationResult {
	switch dc.Mode {
	case DaemonModeWebhook:
		if dc.Webhook.IsNone() {
			return foundation.Invalid(
				foundation.NewValidationError("webhook", "required",
					"webhook configuration is required when mode is webhook"),
			)
		}
		webhookConfig := dc.Webhook.Unwrap()
		return webhookConfig.Validate()

	case DaemonModeScheduled:
		if dc.Schedule.IsNone() {
			return foundation.Invalid(
				foundation.NewValidationError("schedule", "required",
					"schedule configuration is required when mode is scheduled"),
			)
		}
		scheduleConfig := dc.Schedule.Unwrap()
		return scheduleConfig.Validate()

	case DaemonModeHTTP, DaemonModeAPI:
		// These modes have base server configuration which is already validated
		return foundation.Valid()

	default:
		return foundation.Invalid(
			foundation.NewValidationError("mode", "unknown",
				fmt.Sprintf("unknown daemon mode: %s", dc.Mode.String())),
		)
	}
}

// Validate validates server configuration.
func (sc *ServerConfig) Validate() foundation.ValidationResult {
	chain := foundation.NewValidatorChain(
		// Validate host
		func(config ServerConfig) foundation.ValidationResult {
			if config.Host == "" {
				return foundation.Invalid(
					foundation.NewValidationError("host", "not_empty", "host cannot be empty"),
				)
			}

			// Validate that host is a valid IP or hostname
			if ip := net.ParseIP(config.Host); ip == nil {
				// Not an IP, check if it's a valid hostname
				if !isValidHostname(config.Host) {
					return foundation.Invalid(
						foundation.NewValidationError("host", "valid_hostname",
							"host must be a valid IP address or hostname"),
					)
				}
			}
			return foundation.Valid()
		},

		// Validate port
		func(config ServerConfig) foundation.ValidationResult {
			if config.Port < 1 || config.Port > 65535 {
				return foundation.Invalid(
					foundation.NewValidationError("port", "valid_range",
						"port must be between 1 and 65535"),
				)
			}
			return foundation.Valid()
		},

		// Validate TLS configuration if present
		func(config ServerConfig) foundation.ValidationResult {
			if config.TLS.IsSome() {
				tlsConfig := config.TLS.Unwrap()
				return tlsConfig.Validate()
			}
			return foundation.Valid()
		},
	)

	return chain.Validate(*sc)
}

// Validate validates TLS configuration.
func (tc *TLSConfig) Validate() foundation.ValidationResult {
	if !tc.Enabled {
		return foundation.Valid()
	}

	if !tc.Auto {
		// Manual certificate configuration
		if tc.CertFile.IsNone() || tc.KeyFile.IsNone() {
			return foundation.Invalid(
				foundation.NewValidationError("tls", "cert_files_required",
					"cert_file and key_file are required when TLS is enabled and auto is false"),
			)
		}

		// Validate that cert and key files are different
		if tc.CertFile.Unwrap() == tc.KeyFile.Unwrap() {
			return foundation.Invalid(
				foundation.NewValidationError("tls", "cert_key_different",
					"cert_file and key_file must be different"),
			)
		}
	}

	return foundation.Valid()
}

// Validate validates logging configuration.
func (lc *LoggingConfig) Validate() foundation.ValidationResult {
	// Validate log level
	if !lc.Level.Valid() {
		return foundation.Invalid(
			foundation.NewValidationError("level", "valid_log_level",
				"invalid log level: "+lc.Level.String()),
		)
	}

	// Validate format if specified
	if lc.Format.IsSome() {
		format := lc.Format.Unwrap()
		validFormats := []string{"json", "text", "logfmt"}
		isValid := false
		for _, valid := range validFormats {
			if format == valid {
				isValid = true
				break
			}
		}
		if !isValid {
			return foundation.Invalid(
				foundation.NewValidationError("format", "valid_format",
					fmt.Sprintf("format must be one of: %v", validFormats)),
			)
		}
	}

	return foundation.Valid()
}

// Validate validates webhook configuration.
func (wc *WebhookConfig) Validate() foundation.ValidationResult {
	if !wc.Enabled {
		return foundation.Valid()
	}

	chain := foundation.NewValidatorChain(
		// Validate webhook path
		func(config WebhookConfig) foundation.ValidationResult {
			if config.Path.IsSome() {
				path := config.Path.Unwrap()
				if !strings.HasPrefix(path, "/") {
					return foundation.Invalid(
						foundation.NewValidationError("path", "starts_with_slash",
							"webhook path must start with /"),
					)
				}
			}
			return foundation.Valid()
		},

		// Validate max payload size
		func(config WebhookConfig) foundation.ValidationResult {
			if config.MaxPayloadSize.IsSome() {
				size := config.MaxPayloadSize.Unwrap()
				if size < 1024 || size > 100*1024*1024 { // 1KB to 100MB
					return foundation.Invalid(
						foundation.NewValidationError("max_payload_size", "valid_range",
							"max_payload_size must be between 1KB and 100MB"),
					)
				}
			}
			return foundation.Valid()
		},
	)

	return chain.Validate(*wc)
}

// Validate validates schedule configuration.
func (sc *ScheduleConfig) Validate() foundation.ValidationResult {
	if !sc.Enabled {
		return foundation.Valid()
	}

	// Must have either cron or interval, but not both
	hasCron := sc.Cron.IsSome()
	hasInterval := sc.Interval.IsSome()

	if !hasCron && !hasInterval {
		return foundation.Invalid(
			foundation.NewValidationError("schedule", "cron_or_interval",
				"either cron or interval must be specified"),
		)
	}

	if hasCron && hasInterval {
		return foundation.Invalid(
			foundation.NewValidationError("schedule", "cron_xor_interval",
				"cron and interval cannot both be specified"),
		)
	}

	return foundation.Valid()
}

// Validate validates security configuration.
func (sc *SecurityConfig) Validate() foundation.ValidationResult {
	// Validate rate limit configuration if present
	if sc.RateLimit.IsSome() {
		rateLimitConfig := sc.RateLimit.Unwrap()
		return rateLimitConfig.Validate()
	}
	return foundation.Valid()
}

// Validate validates rate limit configuration.
func (rlc *RateLimitConfig) Validate() foundation.ValidationResult {
	if !rlc.Enabled {
		return foundation.Valid()
	}

	chain := foundation.NewValidatorChain(
		// Validate requests per minute
		func(config RateLimitConfig) foundation.ValidationResult {
			if config.RequestsPerMinute.IsSome() {
				rpm := config.RequestsPerMinute.Unwrap()
				if rpm < 1 || rpm > 10000 {
					return foundation.Invalid(
						foundation.NewValidationError("requests_per_minute", "valid_range",
							"requests_per_minute must be between 1 and 10000"),
					)
				}
			}
			return foundation.Valid()
		},

		// Validate burst size
		func(config RateLimitConfig) foundation.ValidationResult {
			if config.BurstSize.IsSome() {
				burst := config.BurstSize.Unwrap()
				if burst < 1 || burst > 1000 {
					return foundation.Invalid(
						foundation.NewValidationError("burst_size", "valid_range",
							"burst_size must be between 1 and 1000"),
					)
				}
			}
			return foundation.Valid()
		},
	)

	return chain.Validate(*rlc)
}

// Helper functions

// isValidHostname checks if a string is a valid hostname.
func isValidHostname(hostname string) bool {
	if hostname == "" || len(hostname) > 253 {
		return false
	}

	// Check each label
	labels := strings.Split(hostname, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}

		// Basic character validation
		for i, c := range label {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') &&
				(c < '0' || c > '9') && c != '-' {
				return false
			}

			// Cannot start or end with hyphen
			if (i == 0 || i == len(label)-1) && c == '-' {
				return false
			}
		}
	}

	return true
}
