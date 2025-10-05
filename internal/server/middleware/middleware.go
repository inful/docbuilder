// Package middleware provides HTTP middleware for logging and panic recovery for DocBuilder servers.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	derrors "git.home.luguber.info/inful/docbuilder/internal/errors"
	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// Chain returns a middleware wrapper that applies logging and panic recovery around a handler.
func Chain(logger *slog.Logger, adapter *derrors.HTTPErrorAdapter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return loggingMiddleware(logger, panicRecoveryMiddleware(logger, adapter, next))
	}
}

// loggingMiddleware logs method, path, status, duration, user agent, and remote addr.
func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start)
		logger.Info("HTTP request",
			logfields.Method(r.Method),
			logfields.Path(r.URL.Path),
			logfields.Status(wrapped.statusCode),
			slog.Duration("duration", duration),
			logfields.UserAgent(r.UserAgent()),
			logfields.RemoteAddr(r.RemoteAddr))
	})
}

// panicRecoveryMiddleware recovers from panics and writes a structured error response via the HTTPErrorAdapter.
func panicRecoveryMiddleware(logger *slog.Logger, adapter *derrors.HTTPErrorAdapter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("HTTP handler panic",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
					"remote_addr", r.RemoteAddr)

				panicErr := derrors.New(derrors.CategoryInternal, derrors.SeverityError, "internal server error").
					WithContext("path", r.URL.Path).
					WithContext("method", r.Method).
					Build()

				adapter.WriteErrorResponse(w, panicErr)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter captures status codes for logging.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
