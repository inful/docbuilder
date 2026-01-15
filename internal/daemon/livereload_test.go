package daemon

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestLiveReload_InitialConnectReceivesNoReload ensures first event sets baseline but does not trigger reload logic client-side.
func TestLiveReload_InitialConnectReceivesNoReload(t *testing.T) {
	hub := NewLiveReloadHub(NewMetricsCollector())
	defer hub.Shutdown()

	// Seed state so initial event includes hash
	hub.Broadcast("abc123")

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(500 * time.Millisecond)
	found := false
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "abc123") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("did not find initial hash event")
	}
}

// TestLiveReload_BroadcastSendsEvent ensures a broadcast after connection emits an SSE message with new hash.
func TestLiveReload_BroadcastSendsEvent(t *testing.T) {
	hub := NewLiveReloadHub(NewMetricsCollector())
	defer hub.Shutdown()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	// Connect client
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	reader := bufio.NewReader(resp.Body)

	// Allow connection to establish
	time.Sleep(20 * time.Millisecond)

	// Broadcast new hash
	hub.Broadcast("newhash")

	// Read lines until we find the broadcast or timeout
	deadline := time.Now().Add(500 * time.Millisecond)
	found := false
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "newhash") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("did not observe broadcast hash in SSE stream")
	}
}

// TestLiveReload_DuplicateBroadcastIgnored ensures second broadcast with same hash not re-sent.
func TestLiveReload_DuplicateBroadcastIgnored(t *testing.T) {
	hub := NewLiveReloadHub(NewMetricsCollector())
	defer hub.Shutdown()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	reader := bufio.NewReader(resp.Body)

	// First broadcast
	hub.Broadcast("hash1")
	// Read until we encounter hash1
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read error: %v", err)
		}
		if strings.Contains(line, "hash1") {
			break
		}
	}

	// Count new lines after second broadcast (should not contain another hash1)
	hub.Broadcast("hash1")
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "hash1") {
			t.Fatalf("duplicate hash1 line received: %s", line)
		}
	}
}
