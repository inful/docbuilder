package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// docWithUIDAndFingerprint builds a minimal markdown document with YAML
// frontmatter containing the given uid and fingerprint values.
func docWithUIDAndFingerprint(uid, fp string) []byte {
	return []byte(fmt.Sprintf("---\nuid: %s\nfingerprint: %s\n---\n# Body\n\ntext\n", uid, fp))
}

// TestOutboundDispatcher_Preflight_SkipsOnFingerprintMatch verifies the happy
// path: preflight returns 200 with a fingerprint that matches the
// document's frontmatter, so the upload is skipped and the skip counter
// increments.
func TestOutboundDispatcher_Preflight_SkipsOnFingerprintMatch(t *testing.T) {
	const fp = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	var preflightHits, uploadHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/documents/uid-1/fingerprint":
			preflightHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"uid":         "uid-1",
				"fingerprint": fp,
			})
		case "POST /api/ingest/file":
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	if err != nil {
		t.Fatalf("NewOutboundDispatcher: %v", err)
	}
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint("uid-1", fp), "docs/x.md")

	// Wait for the preflight call to land.
	deadline := time.Now().Add(2 * time.Second)
	for preflightHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if preflightHits.Load() != 1 {
		t.Fatalf("preflight hits = %d, want 1", preflightHits.Load())
	}
	if uploadHits.Load() != 0 {
		t.Errorf("upload hits = %d, want 0 (preflight matched)", uploadHits.Load())
	}
	if d.PreflightSkipsTotal() != 1 {
		t.Errorf("PreflightSkipsTotal = %d, want 1", d.PreflightSkipsTotal())
	}
	if d.SendsTotal() != 0 {
		t.Errorf("SendsTotal = %d, want 0", d.SendsTotal())
	}
}

// TestOutboundDispatcher_Preflight_UploadsOnFingerprintMismatch verifies
// that when ragabast's stored fingerprint differs from the local one, the
// dispatcher falls through to the upload.
func TestOutboundDispatcher_Preflight_UploadsOnFingerprintMismatch(t *testing.T) {
	const localFP = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const storedFP = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	var uploadHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/documents/uid-1/fingerprint":
			_ = json.NewEncoder(w).Encode(map[string]string{"uid": "uid-1", "fingerprint": storedFP})
		case r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file":
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint("uid-1", localFP), "docs/x.md")

	deadline := time.Now().Add(2 * time.Second)
	for uploadHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if uploadHits.Load() != 1 {
		t.Errorf("upload hits = %d, want 1", uploadHits.Load())
	}
	if d.PreflightSkipsTotal() != 0 {
		t.Errorf("PreflightSkipsTotal = %d, want 0", d.PreflightSkipsTotal())
	}
	if d.SendsTotal() != 1 {
		t.Errorf("SendsTotal = %d, want 1", d.SendsTotal())
	}
}

// TestOutboundDispatcher_Preflight_UploadsOn404 verifies that a 404 from
// preflight (document not yet on the server) triggers an upload.
func TestOutboundDispatcher_Preflight_UploadsOn404(t *testing.T) {
	var uploadHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/documents/uid-1/fingerprint":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file":
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint("uid-1", "any-fp"), "docs/x.md")

	deadline := time.Now().Add(2 * time.Second)
	for uploadHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if uploadHits.Load() != 1 {
		t.Errorf("upload hits = %d, want 1", uploadHits.Load())
	}
	if d.PreflightSkipsTotal() != 0 {
		t.Errorf("PreflightSkipsTotal = %d, want 0", d.PreflightSkipsTotal())
	}
	if d.SendsTotal() != 1 {
		t.Errorf("SendsTotal = %d, want 1", d.SendsTotal())
	}
}

// TestOutboundDispatcher_Preflight_AuthFailureDoesNotUpload verifies that
// 401/403 from preflight is treated as a fail (not skipped and not
// uploaded) because the auth break would just repeat on the upload.
func TestOutboundDispatcher_Preflight_AuthFailureDoesNotUpload(t *testing.T) {
	var uploadHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/documents/uid-1/fingerprint":
			w.WriteHeader(http.StatusUnauthorized)
		case r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file":
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint("uid-1", "any-fp"), "docs/x.md")

	// Give the worker time to complete (or not).
	time.Sleep(200 * time.Millisecond)

	if uploadHits.Load() != 0 {
		t.Errorf("upload hits = %d, want 0 (auth failed)", uploadHits.Load())
	}
	if d.FailsTotal() != 1 {
		t.Errorf("FailsTotal = %d, want 1 (auth failure counted as fail)", d.FailsTotal())
	}
}

// TestOutboundDispatcher_Preflight_FailsOpenOnServerError verifies that a
// 5xx from preflight falls through to the upload (fail-open).
func TestOutboundDispatcher_Preflight_FailsOpenOnServerError(t *testing.T) {
	var uploadHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/documents/uid-1/fingerprint":
			w.WriteHeader(http.StatusInternalServerError)
		case r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file":
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint("uid-1", "any-fp"), "docs/x.md")

	deadline := time.Now().Add(2 * time.Second)
	for uploadHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if uploadHits.Load() != 1 {
		t.Errorf("upload hits = %d, want 1 (fail-open)", uploadHits.Load())
	}
	if d.PreflightErrorsTotal() != 1 {
		t.Errorf("PreflightErrorsTotal = %d, want 1", d.PreflightErrorsTotal())
	}
	if d.SendsTotal() != 1 {
		t.Errorf("SendsTotal = %d, want 1", d.SendsTotal())
	}
}

// TestOutboundDispatcher_Preflight_FailsOpenOnNetworkError verifies that a
// connection error (server unreachable) also fails open.
func TestOutboundDispatcher_Preflight_FailsOpenOnNetworkError(t *testing.T) {
	// Start and immediately close a server so its URL is dead.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	deadURL := srv.URL
	srv.Close()

	// Set up a real upload endpoint on a separate server. Preflight will
	// fail (connection refused), upload should still succeed.
	var uploadHits atomic.Int32
	uploadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file" {
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
			return
		}
		t.Errorf("unexpected request on upload server: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer uploadSrv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        uploadSrv.URL + "/api/ingest/file",
		PreflightBaseURL: deadURL,
		Workers:          1,
		QueueSize:        4,
		Timeout:          500 * time.Millisecond,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint("uid-1", "any-fp"), "docs/x.md")

	deadline := time.Now().Add(3 * time.Second)
	for uploadHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if uploadHits.Load() != 1 {
		t.Errorf("upload hits = %d, want 1 (fail-open on network error)", uploadHits.Load())
	}
	if d.PreflightErrorsTotal() != 1 {
		t.Errorf("PreflightErrorsTotal = %d, want 1", d.PreflightErrorsTotal())
	}
}

// TestOutboundDispatcher_Preflight_SkipsWhenNoFingerprintInContent verifies
// that documents without a fingerprint in their frontmatter bypass the
// preflight and go straight to upload.
func TestOutboundDispatcher_Preflight_SkipsWhenNoFingerprintInContent(t *testing.T) {
	var preflightHits, uploadHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			preflightHits.Add(1)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file":
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	d.Start(t.Context())
	defer d.Stop()

	// Doc has UID but no fingerprint.
	d.Enqueue([]byte("---\nuid: uid-1\n---\n# Body\n"), "docs/x.md")

	deadline := time.Now().Add(2 * time.Second)
	for uploadHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if preflightHits.Load() != 0 {
		t.Errorf("preflight hits = %d, want 0 (no fingerprint in content)", preflightHits.Load())
	}
	if uploadHits.Load() != 1 {
		t.Errorf("upload hits = %d, want 1", uploadHits.Load())
	}
}

// TestOutboundDispatcher_Preflight_SkipsWhenNoUIDInContent verifies that
// documents without a UID also bypass preflight.
func TestOutboundDispatcher_Preflight_SkipsWhenNoUIDInContent(t *testing.T) {
	var preflightHits, uploadHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			preflightHits.Add(1)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file":
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	d.Start(t.Context())
	defer d.Stop()

	// Doc has fingerprint but no UID.
	d.Enqueue([]byte("---\nfingerprint: aaaa\n---\n# Body\n"), "docs/x.md")

	deadline := time.Now().Add(2 * time.Second)
	for uploadHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if preflightHits.Load() != 0 {
		t.Errorf("preflight hits = %d, want 0 (no UID in content)", preflightHits.Load())
	}
	if uploadHits.Load() != 1 {
		t.Errorf("upload hits = %d, want 1", uploadHits.Load())
	}
}

// TestOutboundDispatcher_Preflight_DisabledByFlag verifies that setting
// PreflightEnabled to false skips preflight entirely even when a base URL
// would otherwise resolve.
func TestOutboundDispatcher_Preflight_DisabledByFlag(t *testing.T) {
	var preflightHits, uploadHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			preflightHits.Add(1)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file":
			uploadHits.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	disabled := false
	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL, // would otherwise be enabled
		PreflightEnabled: &disabled,
		Workers:          1,
		QueueSize:        4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint("uid-1", "any-fp"), "docs/x.md")

	deadline := time.Now().Add(2 * time.Second)
	for uploadHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if preflightHits.Load() != 0 {
		t.Errorf("preflight hits = %d, want 0 (disabled by flag)", preflightHits.Load())
	}
	if uploadHits.Load() != 1 {
		t.Errorf("upload hits = %d, want 1", uploadHits.Load())
	}
}

// TestOutboundDispatcher_Preflight_DerivesBaseURLFromIngestURL verifies that
// when PreflightBaseURL is unset, the dispatcher derives it from the
// IngestURL's scheme+host.
func TestOutboundDispatcher_Preflight_DerivesBaseURLFromIngestURL(t *testing.T) {
	var preflightHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/documents/uid-1/fingerprint" {
			preflightHits.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"uid":         "uid-1",
				"fingerprint": strings.Repeat("a", 64),
			})
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/ingest/file" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// No PreflightBaseURL — should derive from IngestURL.
	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL + "/api/ingest/file",
		Workers:   1,
		QueueSize: 4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint("uid-1", strings.Repeat("a", 64)), "docs/x.md")

	deadline := time.Now().Add(2 * time.Second)
	for preflightHits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if preflightHits.Load() != 1 {
		t.Errorf("preflight hits = %d, want 1 (base derived from IngestURL)", preflightHits.Load())
	}
	if d.PreflightSkipsTotal() != 1 {
		t.Errorf("PreflightSkipsTotal = %d, want 1", d.PreflightSkipsTotal())
	}
}

// TestOutboundDispatcher_UnparseableIngestURL_ReturnsError verifies
// that when the legacy IngestURL parses but has no usable scheme/host
// (e.g. "garbage"), the constructor returns an error rather than
// silently constructing a dispatcher with no preflight base. Operators
// see the misconfiguration at startup; we don't degrade silently.
func TestOutboundDispatcher_UnparseableIngestURL_ReturnsError(t *testing.T) {
	_, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: "garbage", // no scheme/host — fails deriveBaseURL
		Workers:   1,
		QueueSize: 4,
	})
	if err == nil {
		t.Fatal("expected error for unparseable IngestURL, got nil")
	}
}

// TestOutboundDispatcher_UnparseableBaseURL_ReturnsError verifies the
// same for the new preferred field.
func TestOutboundDispatcher_UnparseableBaseURL_ReturnsError(t *testing.T) {
	// Use a URL that fails url.Parse (contains a control character).
	_, err := NewOutboundDispatcher(DispatcherConfig{
		BaseURL: "http://[::1", // malformed IPv6 bracket — url.Parse rejects
		Workers: 1,
		QueueSize: 4,
	})
	if err == nil {
		t.Fatal("expected error for unparseable BaseURL, got nil")
	}
}

// TestParseUIDAndFingerprint covers the small parser used to extract the
// two fields the preflight contract needs.
func TestParseUIDAndFingerprint(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantUID     string
		wantFP      string
	}{
		{
			name:    "simple",
			content: "---\nuid: doc-1\nfingerprint: abc123\n---\n# body",
			wantUID: "doc-1",
			wantFP:  "abc123",
		},
		{
			name:    "quoted fingerprint",
			content: "---\nuid: doc-2\nfingerprint: \"abc123\"\n---\n# body",
			wantUID: "doc-2",
			wantFP:  "abc123",
		},
		{
			name:    "fingerprint before uid",
			content: "---\nfingerprint: abc123\nuid: doc-3\n---\n# body",
			wantUID: "doc-3",
			wantFP:  "abc123",
		},
		{
			name:    "no frontmatter",
			content: "# body only",
			wantUID: "",
			wantFP:  "",
		},
		{
			name:    "frontmatter without delimiter close",
			content: "---\nuid: doc-4\nfingerprint: abc\n# body never closed",
			wantUID: "",
			wantFP:  "",
		},
		{
			name:    "missing fingerprint",
			content: "---\nuid: doc-5\ntitle: foo\n---\n# body",
			wantUID: "doc-5",
			wantFP:  "",
		},
		{
			name:    "missing uid",
			content: "---\nfingerprint: abc\n---\n# body",
			wantUID: "",
			wantFP:  "abc",
		},
		{
			name:    "yaml list values preserved as string",
			content: "---\nfingerprint: abc\nuid: \"with spaces\"\n---\n# body",
			wantUID: "with spaces",
			wantFP:  "abc",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotUID, gotFP := parseUIDAndFingerprint([]byte(tc.content))
			if gotUID != tc.wantUID {
				t.Errorf("uid = %q, want %q", gotUID, tc.wantUID)
			}
			if gotFP != tc.wantFP {
				t.Errorf("fingerprint = %q, want %q", gotFP, tc.wantFP)
			}
		})
	}
}

// TestDeriveBaseURL covers the URL → base extraction used when the
// PreflightBaseURL is not set explicitly.
func TestDeriveBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"https with path", "https://ragabast.example.com/api/ingest/file", "https://ragabast.example.com", false},
		{"http with path", "http://localhost:8080/api/ingest/file", "http://localhost:8080", false},
		{"no path", "https://ragabast.example.com", "https://ragabast.example.com", false},
		{"with port", "https://ragabast.example.com:9443/api/ingest", "https://ragabast.example.com:9443", false},
		{"with query and fragment", "https://ragabast.example.com/api?x=1#frag", "https://ragabast.example.com", false},
		{"empty", "", "", true},
		{"no scheme", "ragabast.example.com/api", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := deriveBaseURL(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Errorf("err = nil, want error; got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
