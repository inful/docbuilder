package config

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/normalization"
)

// LogLevel enumerates supported logging levels (subset; mapping to slog or zap later).
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

var logLevelNormalizer = normalization.NewNormalizer(map[string]LogLevel{
	"debug": LogLevelDebug,
	"info":  LogLevelInfo,
	"warn":  LogLevelWarn,
	"error": LogLevelError,
}, LogLevelInfo)

func NormalizeLogLevel(raw string) LogLevel {
	return logLevelNormalizer.Normalize(raw)
}

// LogFormat enumerates supported log output formats.
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

var logFormatNormalizer = normalization.NewNormalizer(map[string]LogFormat{
	"json": LogFormatJSON,
	"text": LogFormatText,
}, LogFormatText)

func NormalizeLogFormat(raw string) LogFormat {
	return logFormatNormalizer.Normalize(raw)
}
