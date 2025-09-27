package config

import "strings"

// LogLevel enumerates supported logging levels (subset; mapping to slog or zap later).
type LogLevel string

const (
    LogLevelDebug LogLevel = "debug"
    LogLevelInfo  LogLevel = "info"
    LogLevelWarn  LogLevel = "warn"
    LogLevelError LogLevel = "error"
)

func NormalizeLogLevel(raw string) LogLevel {
    switch strings.ToLower(strings.TrimSpace(raw)) {
    case string(LogLevelDebug):
        return LogLevelDebug
    case string(LogLevelInfo):
        return LogLevelInfo
    case string(LogLevelWarn):
        return LogLevelWarn
    case string(LogLevelError):
        return LogLevelError
    default:
        return ""
    }
}

// LogFormat enumerates supported log output formats.
type LogFormat string

const (
    LogFormatJSON LogFormat = "json"
    LogFormatText LogFormat = "text"
)

func NormalizeLogFormat(raw string) LogFormat {
    switch strings.ToLower(strings.TrimSpace(raw)) {
    case string(LogFormatJSON):
        return LogFormatJSON
    case string(LogFormatText):
        return LogFormatText
    default:
        return ""
    }
}
