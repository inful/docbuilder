package typed

import (
	"fmt"
	"net"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// DaemonModeType represents the daemon operation mode
type DaemonModeType struct {
	value string
}

// Predefined daemon modes
var (
	DaemonModeHTTP      = DaemonModeType{"http"}
	DaemonModeWebhook   = DaemonModeType{"webhook"}
	DaemonModeScheduled = DaemonModeType{"scheduled"}
	DaemonModeAPI       = DaemonModeType{"api"}

	// Normalizer for daemon modes
	daemonModeNormalizer = foundation.NewNormalizer(map[string]DaemonModeType{
		"http":      DaemonModeHTTP,
		"webhook":   DaemonModeWebhook,
		"scheduled": DaemonModeScheduled,
		"api":       DaemonModeAPI,
	}, DaemonModeHTTP) // default to HTTP

	// Validator for daemon modes
	daemonModeValidator = foundation.OneOf("daemon_mode", []DaemonModeType{
		DaemonModeHTTP, DaemonModeWebhook, DaemonModeScheduled, DaemonModeAPI,
	})
)

// String returns the string representation of the daemon mode
func (dm DaemonModeType) String() string {
	return dm.value
}

// Valid checks if the daemon mode is valid
func (dm DaemonModeType) Valid() bool {
	return daemonModeValidator(dm).Valid
}

// RequiresHTTPServer indicates if this mode requires an HTTP server
func (dm DaemonModeType) RequiresHTTPServer() bool {
	switch dm {
	case DaemonModeHTTP, DaemonModeWebhook, DaemonModeAPI:
		return true
	default:
		return false
	}
}

// SupportsWebhooks indicates if this mode supports webhook processing
func (dm DaemonModeType) SupportsWebhooks() bool {
	switch dm {
	case DaemonModeWebhook, DaemonModeAPI:
		return true
	default:
		return false
	}
}

// ParseDaemonModeType parses a string into a DaemonModeType
func ParseDaemonModeType(s string) foundation.Result[DaemonModeType, error] {
	mode, err := daemonModeNormalizer.NormalizeWithError(s)
	if err != nil {
		return foundation.Err[DaemonModeType, error](
			foundation.ValidationError(fmt.Sprintf("invalid daemon mode: %s", s)).
				WithContext(foundation.Fields{
					"input":        s,
					"valid_values": []string{"http", "webhook", "scheduled", "api"},
				}).
				Build(),
		)
	}
	return foundation.Ok[DaemonModeType, error](mode)
}

// LogLevelType represents strongly-typed log levels
type LogLevelType struct {
	value string
}

// Predefined log levels
var (
	LogLevelDebug = LogLevelType{"debug"}
	LogLevelInfo  = LogLevelType{"info"}
	LogLevelWarn  = LogLevelType{"warn"}
	LogLevelError = LogLevelType{"error"}
	LogLevelFatal = LogLevelType{"fatal"}

	// Normalizer for log levels
	logLevelNormalizer = foundation.NewNormalizer(map[string]LogLevelType{
		"debug": LogLevelDebug,
		"info":  LogLevelInfo,
		"warn":  LogLevelWarn,
		"error": LogLevelError,
		"fatal": LogLevelFatal,
	}, LogLevelInfo) // default to info

	// Validator for log levels
	logLevelValidator = foundation.OneOf("log_level", []LogLevelType{
		LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, LogLevelFatal,
	})
)

// String returns the string representation of the log level
func (ll LogLevelType) String() string {
	return ll.value
}

// Valid checks if the log level is valid
func (ll LogLevelType) Valid() bool {
	return logLevelValidator(ll).Valid
}

// ParseLogLevelType parses a string into a LogLevelType
func ParseLogLevelType(s string) foundation.Result[LogLevelType, error] {
	level, err := logLevelNormalizer.NormalizeWithError(s)
	if err != nil {
		return foundation.Err[LogLevelType, error](
			foundation.ValidationError(fmt.Sprintf("invalid log level: %s", s)).
				WithContext(foundation.Fields{
					"input":        s,
					"valid_values": []string{"debug", "info", "warn", "error", "fatal"},
				}).
				Build(),
		)
	}
	return foundation.Ok[LogLevelType, error](level)
}

// DaemonConfig represents a strongly-typed daemon configuration
type DaemonConfig struct {
	// Operation mode
	Mode DaemonModeType `yaml:"mode" json:"mode"`

	// Server configuration
	Server ServerConfig `yaml:"server" json:"server"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`

	// Build configuration
	Build BuildConfig `yaml:"build" json:"build"`

	// Monitoring configuration
	Monitoring foundation.Option[MonitoringConfig] `yaml:"monitoring,omitempty" json:"monitoring,omitempty"`

	// Webhook configuration (if webhook mode)
	Webhook foundation.Option[WebhookConfig] `yaml:"webhook,omitempty" json:"webhook,omitempty"`

	// Scheduling configuration (if scheduled mode)
	Schedule foundation.Option[ScheduleConfig] `yaml:"schedule,omitempty" json:"schedule,omitempty"`

	// Security configuration
	Security SecurityConfig `yaml:"security" json:"security"`

	// Performance configuration
	Performance PerformanceConfig `yaml:"performance" json:"performance"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage" json:"storage"`

	// Custom settings for extensibility
	Custom map[string]any `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host         string                           `yaml:"host" json:"host"`
	Port         int                              `yaml:"port" json:"port"`
	ReadTimeout  foundation.Option[time.Duration] `yaml:"read_timeout,omitempty" json:"read_timeout,omitempty"`
	WriteTimeout foundation.Option[time.Duration] `yaml:"write_timeout,omitempty" json:"write_timeout,omitempty"`
	IdleTimeout  foundation.Option[time.Duration] `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
	TLS          foundation.Option[TLSConfig]     `yaml:"tls,omitempty" json:"tls,omitempty"`
	CORS         foundation.Option[CORSConfig]    `yaml:"cors,omitempty" json:"cors,omitempty"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled  bool                      `yaml:"enabled" json:"enabled"`
	CertFile foundation.Option[string] `yaml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile  foundation.Option[string] `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	Auto     bool                      `yaml:"auto" json:"auto"` // For automatic certificate generation
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins,omitempty" json:"allowed_origins,omitempty"`
	AllowedMethods []string `yaml:"allowed_methods,omitempty" json:"allowed_methods,omitempty"`
	AllowedHeaders []string `yaml:"allowed_headers,omitempty" json:"allowed_headers,omitempty"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      LogLevelType              `yaml:"level" json:"level"`
	Format     foundation.Option[string] `yaml:"format,omitempty" json:"format,omitempty"`
	File       foundation.Option[string] `yaml:"file,omitempty" json:"file,omitempty"`
	MaxSize    foundation.Option[int]    `yaml:"max_size,omitempty" json:"max_size,omitempty"`
	Structured bool                      `yaml:"structured" json:"structured"`
}

// BuildConfig represents build execution configuration
type BuildConfig struct {
	Timeout       foundation.Option[time.Duration] `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxConcurrent foundation.Option[int]           `yaml:"max_concurrent,omitempty" json:"max_concurrent,omitempty"`
	RetryAttempts foundation.Option[int]           `yaml:"retry_attempts,omitempty" json:"retry_attempts,omitempty"`
	RetryDelay    foundation.Option[time.Duration] `yaml:"retry_delay,omitempty" json:"retry_delay,omitempty"`
	CleanupAfter  foundation.Option[time.Duration] `yaml:"cleanup_after,omitempty" json:"cleanup_after,omitempty"`
}

// MonitoringConfig represents monitoring and health check configuration
type MonitoringConfig struct {
	Enabled     bool                             `yaml:"enabled" json:"enabled"`
	HealthCheck HealthCheckConfig                `yaml:"health_check" json:"health_check"`
	Metrics     foundation.Option[MetricsConfig] `yaml:"metrics,omitempty" json:"metrics,omitempty"`
	Tracing     foundation.Option[TracingConfig] `yaml:"tracing,omitempty" json:"tracing,omitempty"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled  bool                             `yaml:"enabled" json:"enabled"`
	Endpoint foundation.Option[string]        `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Interval foundation.Option[time.Duration] `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// MetricsConfig represents metrics collection configuration
type MetricsConfig struct {
	Enabled  bool                      `yaml:"enabled" json:"enabled"`
	Endpoint foundation.Option[string] `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Provider foundation.Option[string] `yaml:"provider,omitempty" json:"provider,omitempty"`
}

// TracingConfig represents distributed tracing configuration
type TracingConfig struct {
	Enabled     bool                      `yaml:"enabled" json:"enabled"`
	ServiceName foundation.Option[string] `yaml:"service_name,omitempty" json:"service_name,omitempty"`
	Endpoint    foundation.Option[string] `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

// WebhookConfig represents webhook processing configuration
type WebhookConfig struct {
	Enabled        bool                             `yaml:"enabled" json:"enabled"`
	Secret         foundation.Option[string]        `yaml:"secret,omitempty" json:"secret,omitempty"`
	Path           foundation.Option[string]        `yaml:"path,omitempty" json:"path,omitempty"`
	Timeout        foundation.Option[time.Duration] `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxPayloadSize foundation.Option[int]           `yaml:"max_payload_size,omitempty" json:"max_payload_size,omitempty"`
	AuthRequired   bool                             `yaml:"auth_required" json:"auth_required"`
}

// ScheduleConfig represents scheduled build configuration
type ScheduleConfig struct {
	Enabled  bool                             `yaml:"enabled" json:"enabled"`
	Cron     foundation.Option[string]        `yaml:"cron,omitempty" json:"cron,omitempty"`
	Interval foundation.Option[time.Duration] `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timezone foundation.Option[string]        `yaml:"timezone,omitempty" json:"timezone,omitempty"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	APIKey         foundation.Option[string]          `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	JWTSecret      foundation.Option[string]          `yaml:"jwt_secret,omitempty" json:"jwt_secret,omitempty"`
	RateLimit      foundation.Option[RateLimitConfig] `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
	AllowedHosts   []string                           `yaml:"allowed_hosts,omitempty" json:"allowed_hosts,omitempty"`
	TrustedProxies []string                           `yaml:"trusted_proxies,omitempty" json:"trusted_proxies,omitempty"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool                             `yaml:"enabled" json:"enabled"`
	RequestsPerMinute foundation.Option[int]           `yaml:"requests_per_minute,omitempty" json:"requests_per_minute,omitempty"`
	BurstSize         foundation.Option[int]           `yaml:"burst_size,omitempty" json:"burst_size,omitempty"`
	WindowSize        foundation.Option[time.Duration] `yaml:"window_size,omitempty" json:"window_size,omitempty"`
}

// PerformanceConfig represents performance tuning configuration
type PerformanceConfig struct {
	MaxWorkers     foundation.Option[int]           `yaml:"max_workers,omitempty" json:"max_workers,omitempty"`
	QueueSize      foundation.Option[int]           `yaml:"queue_size,omitempty" json:"queue_size,omitempty"`
	GCPercent      foundation.Option[int]           `yaml:"gc_percent,omitempty" json:"gc_percent,omitempty"`
	MemoryLimit    foundation.Option[string]        `yaml:"memory_limit,omitempty" json:"memory_limit,omitempty"`
	RequestTimeout foundation.Option[time.Duration] `yaml:"request_timeout,omitempty" json:"request_timeout,omitempty"`
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	WorkspaceDir foundation.Option[string]        `yaml:"workspace_dir,omitempty" json:"workspace_dir,omitempty"`
	StateFile    foundation.Option[string]        `yaml:"state_file,omitempty" json:"state_file,omitempty"`
	TempDir      foundation.Option[string]        `yaml:"temp_dir,omitempty" json:"temp_dir,omitempty"`
	CleanupAge   foundation.Option[time.Duration] `yaml:"cleanup_age,omitempty" json:"cleanup_age,omitempty"`
	MaxSize      foundation.Option[string]        `yaml:"max_size,omitempty" json:"max_size,omitempty"`
}

// Validation methods

// Validate performs comprehensive validation of the daemon configuration
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

// validateModeSpecificConfig validates configuration based on daemon mode
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

// Validate validates server configuration
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

// Validate validates TLS configuration
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

// Validate validates logging configuration
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

// Validate validates webhook configuration
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

// Validate validates schedule configuration
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

// Validate validates security configuration
func (sc *SecurityConfig) Validate() foundation.ValidationResult {
	// Validate rate limit configuration if present
	if sc.RateLimit.IsSome() {
		rateLimitConfig := sc.RateLimit.Unwrap()
		return rateLimitConfig.Validate()
	}
	return foundation.Valid()
}

// Validate validates rate limit configuration
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

// isValidHostname checks if a string is a valid hostname
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
			if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') &&
				!(c >= '0' && c <= '9') && c != '-' {
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
