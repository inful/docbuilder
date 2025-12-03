package errors

import (
	"encoding/json"
	stdErrors "errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	dberrors "git.home.luguber.info/inful/docbuilder/internal/errors"
)

func TestHTTPErrorAdapter_StatusCodeFor(t *testing.T) {
	adapter := NewHTTPErrorAdapter(slog.Default())

	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: http.StatusOK,
		},
		{
			name: "classified validation error",
			err: NewError(CategoryValidation, "invalid input").
				WithSeverity(SeverityError).
				Build(),
			expected: http.StatusBadRequest,
		},
		{
			name: "classified auth error",
			err: NewError(CategoryAuth, "unauthorized").
				WithSeverity(SeverityError).
				Build(),
			expected: http.StatusUnauthorized,
		},
		{
			name:     "docbuilder network error",
			err:      dberrors.New(dberrors.CategoryNetwork, dberrors.SeverityFatal, "network failed"),
			expected: http.StatusBadGateway,
		},
		{
			name:     "docbuilder internal error",
			err:      dberrors.New(dberrors.CategoryInternal, dberrors.SeverityFatal, "internal error"),
			expected: http.StatusInternalServerError,
		},
		{
			name:     "unclassified error",
			err:      &customHTTPError{msg: "unknown error"},
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.StatusCodeFor(tt.err)
			if got != tt.expected {
				t.Errorf("StatusCodeFor() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHTTPErrorAdapter_WriteErrorResponse(t *testing.T) {
	adapter := NewHTTPErrorAdapter(slog.Default())

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		checkJSON      bool
	}{
		{
			name:           "nil error",
			err:            nil,
			expectedStatus: http.StatusOK,
			checkJSON:      false,
		},
		{
			name: "classified validation error",
			err: NewError(CategoryValidation, "invalid input").
				WithSeverity(SeverityError).
				Build(),
			expectedStatus: http.StatusBadRequest,
			checkJSON:      true,
		},
		{
			name:           "docbuilder config error",
			err:            dberrors.New(dberrors.CategoryConfig, dberrors.SeverityFatal, "bad config"),
			expectedStatus: http.StatusBadRequest,
			checkJSON:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			adapter.WriteErrorResponse(w, tt.err)

			if w.Code != tt.expectedStatus {
				t.Errorf("WriteErrorResponse() status = %v, want %v", w.Code, tt.expectedStatus)
			}

			if tt.checkJSON {
				// Verify we get valid JSON response
				var response HTTPErrorResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Errorf("WriteErrorResponse() invalid JSON: %v", err)
				}

				if response.Error == "" {
					t.Error("WriteErrorResponse() missing error message")
				}

				if response.Code == "" {
					t.Error("WriteErrorResponse() missing error code")
				}

				// Check content type
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("WriteErrorResponse() content-type = %v, want application/json", contentType)
				}
			}
		})
	}
}

func TestHTTPErrorAdapter_FormatErrorResponse(t *testing.T) {
	adapter := NewHTTPErrorAdapter(slog.Default())

	tests := []struct {
		name     string
		err      error
		checkMsg bool
	}{
		{
			name:     "nil error",
			err:      nil,
			checkMsg: false,
		},
		{
			name: "classified error with context",
			err: NewError(CategoryValidation, "invalid field").
				WithSeverity(SeverityError).
				WithContext("field", "username").
				Build(),
			checkMsg: true,
		},
		{
			name:     "docbuilder retryable error",
			err:      dberrors.Retryable(dberrors.CategoryNetwork, dberrors.SeverityWarning, "network timeout"),
			checkMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := adapter.FormatErrorResponse(tt.err)

			if !tt.checkMsg {
				return // Skip further checks for nil error
			}

			if response.Error == "" {
				t.Error("FormatErrorResponse() missing error message")
			}

			if response.Code == "" {
				t.Error("FormatErrorResponse() missing error code")
			}

			// Check retryable flag for retryable errors
			if tt.err != nil {
				var dbe *dberrors.DocBuilderError
				if stdErrors.As(tt.err, &dbe) && dbe.Retryable {
					if response.Details == nil || !response.Details["retryable"].(bool) {
						t.Error("FormatErrorResponse() missing retryable flag for retryable error")
					}
				}
			}
		})
	}
}

// customHTTPError is a test helper for unclassified errors
type customHTTPError struct {
	msg string
}

func (e *customHTTPError) Error() string {
	return e.msg
}
