package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Every LiveReload client holds a goroutine and a channel for as long
// as it lives, and any page of the internet may open them. The number
// of them has a limit.
func TestTheLiveReloadHubRefusesMoreClientsThanItTakes(t *testing.T) {
	hub := NewLiveReloadHub(nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// A client is a long-lived stream: each one runs in its own
	// goroutine and stays until the test ends.
	for range maxLiveReloadClients {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/livereload", nil)
		go func() {
			hub.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		hub.mu.RLock()
		clients := len(hub.clients)
		hub.mu.RUnlock()
		if clients == maxLiveReloadClients {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d clients were registered", clients, maxLiveReloadClients)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The client beyond the limit is refused. The answer comes at once:
	// a client that is taken stays until its stream ends.
	answered := make(chan int, 1)
	go func() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/livereload", nil)
		hub.ServeHTTP(rec, req)
		answered <- rec.Code
	}()

	select {
	case code := <-answered:
		if code != http.StatusServiceUnavailable {
			t.Fatalf("the client beyond the limit was answered with %d, want %d", code, http.StatusServiceUnavailable)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the client beyond the limit was taken")
	}
}
