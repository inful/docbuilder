package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The recorder is only there to see whether the page is missing. A big
// response must not be held in the memory of the server: one request
// with the LiveReload cookie held the whole file.
func TestTheResponseRecorderDoesNotHoldABigResponse(t *testing.T) {
	client := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: client, statusCode: http.StatusOK}

	big := bytes.Repeat([]byte("a"), maxRecorderBody+4096)
	written, err := rec.Write(big)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if written != len(big) {
		t.Fatalf("the recorder wrote %d bytes, want %d", written, len(big))
	}
	if len(rec.body) > maxRecorderBody {
		t.Fatalf("the recorder holds %d bytes, want at most %d", len(rec.body), maxRecorderBody)
	}

	rec.Flush()

	// The client got the whole file.
	if got := client.Body.Len(); got != len(big) {
		t.Fatalf("the client got %d bytes, want %d", got, len(big))
	}
}
