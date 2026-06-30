package logfields

import (
	"log/slog"
	"testing"
)

// TestHelperKeyNames verifies string-based helper key/value stability.
func TestHelperKeyNames(t *testing.T) {
	cases := []struct {
		name    string
		attrKey string
		attrVal string
		attr    any
	}{
		{"JobID", KeyJobID, "123", JobID("123")},
		{"Repository", KeyRepo, "repo1", Repository("repo1")},
		{"Section", KeySection, "sec", Section("sec")},
		{"Path", KeyPath, "/tmp/x", Path("/tmp/x")},
		{"File", KeyFile, "file.md", File("file.md")},
		{"Method", KeyMethod, "GET", Method("GET")},
		{"UserAgent", KeyUserAgent, "ua", UserAgent("ua")},
		{"RemoteAddr", KeyRemoteAddr, "1.2.3.4", RemoteAddr("1.2.3.4")},
		{"Status", KeyStatus, "200", Status(200)},
		{"Name", KeyName, "n", Name("n")},
		{"URL", KeyURL, "http://example", URL("http://example")},
	}

	for _, tc := range cases {
		a := tc.attr.(slog.Attr)
		if a.Key != tc.attrKey {
			// Key drift would break log ingestion schemas.
			t.Fatalf("%s: expected key %s, got %s", tc.name, tc.attrKey, a.Key)
		}
		if got := a.Value.String(); got != tc.attrVal {
			t.Fatalf("%s: expected value %s, got %v", tc.name, tc.attrVal, got)
		}
	}
}

// TestErrorHelper ensures Error() handles nil and non-nil errors predictably.
func TestErrorHelper(t *testing.T) {
	attr := Error(nil)
	if attr.Key != KeyError {
		t.Fatalf("Error key mismatch: %s", attr.Key)
	}
	if attr.Value.String() != "" {
		t.Fatalf("Expected empty error string, got %s", attr.Value.String())
	}
	attr = Error(testError{})
	if attr.Value.String() != "err-test" {
		t.Fatalf("Expected 'err-test', got %s", attr.Value.String())
	}
}

type testError struct{}

func (e testError) Error() string { return "err-test" }
