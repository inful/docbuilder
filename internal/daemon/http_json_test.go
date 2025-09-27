package daemon

import (
	"net/http/httptest"
	"testing"
)

func TestWriteJSONBasic(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"status": "ok"}
	if err := writeJSON(rec, 202, payload); err != nil {
		t.Fatalf("writeJSON error: %v", err)
	}
	if rec.Code != 202 {
		t.Fatalf("expected status 202 got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content-type: %s", ct)
	}
	if body := rec.Body.String(); body == "" || body[0] != '{' {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestWriteJSONPretty(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/x?pretty=1", nil)
	payload := map[string]string{"hello": "world"}
	if err := writeJSONPretty(rec, r, 200, payload); err != nil {
		t.Fatalf("writeJSONPretty error: %v", err)
	}
	if rec.Code != 200 {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	if lines := len(splitLines(rec.Body.String())); lines < 2 {
		t.Fatalf("expected pretty (multi-line) output, got %d lines: %q", lines, rec.Body.String())
	}
}

// splitLines is a tiny helper to avoid importing strings just for tests.
func splitLines(s string) []string {
	var out []string
	cur := 0
	for i, c := range s {
		if c == '\n' {
			out = append(out, s[cur:i])
			cur = i + 1
		}
	}
	if cur < len(s) {
		out = append(out, s[cur:])
	}
	return out
}
