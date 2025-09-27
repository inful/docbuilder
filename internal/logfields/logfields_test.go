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
		attr    interface{}
	}{
		{"JobID", KeyJobID, "123", JobID("123")},
		{"JobType", KeyJobType, "build", JobType("build")},
		{"JobStatus", KeyJobStatus, "queued", JobStatus("queued")},
		{"ScheduleID", KeyScheduleID, "sch1", ScheduleID("sch1")},
		{"ScheduleName", KeySchedule, "nightly", ScheduleName("nightly")},
		{"Repository", KeyRepo, "repo1", Repository("repo1")},
		{"Section", KeySection, "sec", Section("sec")},
		{"Path", KeyPath, "/tmp/x", Path("/tmp/x")},
		{"File", KeyFile, "file.md", File("file.md")},
		{"Worker", KeyWorker, "w1", Worker("w1")},
		{"Method", KeyMethod, "GET", Method("GET")},
		{"UserAgent", KeyUserAgent, "ua", UserAgent("ua")},
		{"RemoteAddr", KeyRemoteAddr, "1.2.3.4", RemoteAddr("1.2.3.4")},
		{"RequestID", KeyRequestID, "rid", RequestID("rid")},
		{"ForgeType", KeyForgeType, "github", ForgeType("github")},
		{"Name", KeyName, "n", Name("n")},
		{"URL", KeyURL, "http://example", URL("http://example")},
	}

	for _, tc := range cases {
		a := tc.attr.(slog.Attr)
		if a.Key != tc.attrKey {
			// Key drift would break log ingestion schemas.
			 t.Fatalf("%s: expected key %s, got %s", tc.name, tc.attrKey, a.Key)
		}
		if got := a.Value.String(); got != tc.attrVal { // Value is slog.Value
			 t.Fatalf("%s: expected value %s, got %v", tc.name, tc.attrVal, got)
		}
	}
}

// TestNumericHelpers verifies keys for numeric & float helpers.
func TestNumericHelpers(t *testing.T) {
	if v := JobPriority(5); v.Key != KeyJobPriority { t.Fatalf("JobPriority key mismatch: %s", v.Key) }
	if v := Status(200); v.Key != KeyStatus { t.Fatalf("Status key mismatch: %s", v.Key) }
	if v := ResponseSize(42); v.Key != KeyResponseSz { t.Fatalf("ResponseSize key mismatch: %s", v.Key) }
	if v := DurationMS(12.5); v.Key != KeyDurationMS { t.Fatalf("DurationMS key mismatch: %s", v.Key) }
	if v := ContentLength(1234); v.Key != KeyContentLen { t.Fatalf("ContentLength key mismatch: %s", v.Key) }
}

// TestErrorHelper ensures Error() handles nil and non-nil errors predictably.
func TestErrorHelper(t *testing.T) {
	attr := Error(nil)
	if attr.Key != KeyError { t.Fatalf("Error key mismatch: %s", attr.Key) }
	if attr.Value.String() != "" { t.Fatalf("Expected empty error string, got %s", attr.Value.String()) }
	attr = Error(errTest{})
	if attr.Value.String() != "err-test" { t.Fatalf("Expected 'err-test', got %s", attr.Value.String()) }
}

type errTest struct{}
func (e errTest) Error() string { return "err-test" }
