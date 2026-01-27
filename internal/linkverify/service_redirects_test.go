package linkverify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerificationService_CheckExternalLink_AllowsExactMaxRedirects(t *testing.T) {
	// Create a server that redirects exactly 3 times and then returns 200.
	redirects := 0
	maxRedirects := 3
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			redirects++
			http.Redirect(w, r, "/r1", http.StatusFound)
		case "/r1":
			redirects++
			http.Redirect(w, r, "/r2", http.StatusFound)
		case "/r2":
			redirects++
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Match production semantics: allow exactly maxRedirects redirects.
		if len(via) > maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		return nil
	}

	vs := &VerificationService{httpClient: client}
	status, err := vs.checkExternalLink(context.Background(), srv.URL+"/start")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status=%d, want %d", status, http.StatusOK)
	}
	if redirects != 3 {
		t.Fatalf("redirects=%d, want %d", redirects, 3)
	}
}
