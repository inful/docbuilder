package errors

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
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
		})
	}
}

// customHTTPError is a test helper for unclassified errors.
type customHTTPError struct {
	msg string
}

func (e *customHTTPError) Error() string {
	return e.msg
}
