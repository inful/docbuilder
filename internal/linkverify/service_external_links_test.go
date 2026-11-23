package linkverify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestVerificationService_CheckExternalLink_FallsBackToGETWhenHeadIsNotFound(t *testing.T) {
	var headCalls atomic.Int64
	var getCalls atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			headCalls.Add(1)
			w.WriteHeader(http.StatusNotFound)
			return
		case http.MethodGet:
			getCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	}))
	t.Cleanup(srv.Close)

	svc := &VerificationService{httpClient: srv.Client()}

	status, err := svc.checkExternalLink(context.Background(), srv.URL+"/path#fragment")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if headCalls.Load() != 1 {
		t.Fatalf("expected 1 HEAD call, got %d", headCalls.Load())
	}
	if getCalls.Load() != 1 {
		t.Fatalf("expected 1 GET call, got %d", getCalls.Load())
	}
}

func TestVerificationService_CheckExternalLink_Treats429AsValid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	svc := &VerificationService{httpClient: srv.Client()}

	status, err := svc.checkExternalLink(context.Background(), srv.URL+"/rate-limited")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, status)
	}
}

func TestVerificationService_CheckExternalLink_ReportsServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	svc := &VerificationService{httpClient: srv.Client()}

	status, err := svc.checkExternalLink(context.Background(), srv.URL+"/boom")
	if err == nil {
		t.Fatalf("expected error, got nil (status %d)", status)
	}
	if status != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
}

// A link that did not answer is not known to be broken and it is not
// known to be good. It is reported as an error, so it is checked again
// later instead of being cached as valid for the whole time to live.
func TestVerificationService_CheckExternalLink_ReportsATimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Timeout = 10 * time.Millisecond

	svc := &VerificationService{httpClient: client}

	status, err := svc.checkExternalLink(context.Background(), srv.URL+"/slow")
	if err == nil {
		t.Fatalf("expected an error for the link that timed out (status %d)", status)
	}
	if !isTimeoutError(err) {
		t.Fatalf("expected a timeout, got %v", err)
	}
}
