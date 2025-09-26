package daemon

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// LoggingMiddleware provides structured request logging with metrics
type LoggingMiddleware struct {
	metrics *MetricsCollector
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(metrics *MetricsCollector) *LoggingMiddleware {
	return &LoggingMiddleware{
		metrics: metrics,
	}
}

// logResponseWriter wraps http.ResponseWriter to capture status code and size for logging
type logResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

// WriteHeader captures the status code
func (rw *logResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures response size
func (rw *logResponseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Handler wraps an HTTP handler with structured logging and metrics
func (lm *LoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create request ID for tracing
		requestID := generateRequestID()
		type ctxKey string
		ctx := context.WithValue(r.Context(), ctxKey("request_id"), requestID)
		r = r.WithContext(ctx)

		// Wrap response writer to capture status and size
		rw := &logResponseWriter{
			ResponseWriter: w,
			status:         200, // Default status
		}

		// Add request ID to response headers
		rw.Header().Set("X-Request-ID", requestID)

		// Record request metrics
		lm.metrics.IncrementCounter("http_requests_total")
		lm.metrics.SetGauge("http_active_requests", 1) // TODO: Track concurrent requests properly

		// Log request start
		slog.Info("HTTP request started",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent())

		// Process request
		next.ServeHTTP(rw, r)

		// Calculate duration
		duration := time.Since(start)

		// Record response metrics
		lm.metrics.RecordHistogram("http_request_duration_seconds", duration.Seconds())
		lm.metrics.RecordHistogram("http_response_size_bytes", float64(rw.size))

		// Increment status code counters
		statusClass := rw.status / 100
		switch statusClass {
		case 2:
			lm.metrics.IncrementCounter("http_requests_2xx")
		case 3:
			lm.metrics.IncrementCounter("http_requests_3xx")
		case 4:
			lm.metrics.IncrementCounter("http_requests_4xx")
		case 5:
			lm.metrics.IncrementCounter("http_requests_5xx")
		}

		// Log request completion
		logLevel := slog.LevelInfo
		if rw.status >= 400 {
			logLevel = slog.LevelWarn
		}
		if rw.status >= 500 {
			logLevel = slog.LevelError
		}

		slog.Log(context.Background(), logLevel, "HTTP request completed",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", float64(duration.Nanoseconds())/1e6,
			"response_size", rw.size,
			"remote_addr", r.RemoteAddr)
	})
}

// generateRequestID creates a unique request identifier
func generateRequestID() string {
	// Simple implementation - in production, use proper UUID library
	return time.Now().Format("20060102-150405.000000")
}

// StructuredLogger provides enhanced logging capabilities for the daemon
type StructuredLogger struct {
	baseLogger *slog.Logger
	metrics    *MetricsCollector
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(metrics *MetricsCollector) *StructuredLogger {
	return &StructuredLogger{
		baseLogger: slog.Default(),
		metrics:    metrics,
	}
}

// LogWithMetrics logs a message and records it in metrics
func (sl *StructuredLogger) LogWithMetrics(level slog.Level, msg string, attrs ...slog.Attr) {
	// Record log metrics
	sl.metrics.IncrementCounter("log_messages_total")

	switch level {
	case slog.LevelDebug:
		sl.metrics.IncrementCounter("log_debug_total")
	case slog.LevelInfo:
		sl.metrics.IncrementCounter("log_info_total")
	case slog.LevelWarn:
		sl.metrics.IncrementCounter("log_warn_total")
	case slog.LevelError:
		sl.metrics.IncrementCounter("log_error_total")
	}

	// Log the message
	sl.baseLogger.LogAttrs(context.Background(), level, msg, attrs...)
}

// Info logs an info message with metrics
func (sl *StructuredLogger) Info(msg string, attrs ...slog.Attr) {
	sl.LogWithMetrics(slog.LevelInfo, msg, attrs...)
}

// Warn logs a warning message with metrics
func (sl *StructuredLogger) Warn(msg string, attrs ...slog.Attr) {
	sl.LogWithMetrics(slog.LevelWarn, msg, attrs...)
}

// Error logs an error message with metrics
func (sl *StructuredLogger) Error(msg string, attrs ...slog.Attr) {
	sl.LogWithMetrics(slog.LevelError, msg, attrs...)
}

// Debug logs a debug message with metrics
func (sl *StructuredLogger) Debug(msg string, attrs ...slog.Attr) {
	sl.LogWithMetrics(slog.LevelDebug, msg, attrs...)
}
