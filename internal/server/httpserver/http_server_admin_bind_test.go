package httpserver

import (
	"net"
	"strconv"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The admin API has no password: it can start a build and it shows the
// forge layout. It must therefore not be reachable from the network
// unless the operator asked for it.
func TestAdminServer_BindsToLoopbackByDefault(t *testing.T) {
	adminPort := freePort(t)

	cfg := &config.Config{
		Monitoring: &config.MonitoringConfig{
			Health: config.MonitoringHealth{Path: "/health"},
		},
		Daemon: &config.DaemonConfig{
			HTTP: config.HTTPConfig{
				DocsPort:    freePort(t),
				WebhookPort: freePort(t),
				AdminPort:   adminPort,
				AdminBind:   "127.0.0.1",
			},
		},
	}

	srv := New(cfg, &webhookRuntimeStub{}, Options{})
	if err := srv.Start(t.Context()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop(t.Context()) })

	// The admin port answers on the loopback interface.
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(adminPort)))
	if err != nil {
		t.Fatalf("admin port must accept connections on 127.0.0.1: %v", err)
	}
	_ = conn.Close()

	// It does not answer on the machine's other addresses.
	for _, ip := range nonLoopbackIPv4(t) {
		quick := &net.Dialer{Timeout: 500 * time.Millisecond}
		conn, err := quick.DialContext(t.Context(), "tcp", net.JoinHostPort(ip, strconv.Itoa(adminPort)))
		if err == nil {
			_ = conn.Close()
			t.Fatalf("admin port must not be reachable on %s", ip)
		}
	}
}

// freePort returns a port that is free right now.
func freePort(t *testing.T) int {
	t.Helper()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return n
}

// nonLoopbackIPv4 returns the machine's IPv4 addresses that are not
// loopback.
func nonLoopbackIPv4(t *testing.T) []string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatalf("interface addrs: %v", err)
	}
	var out []string
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil || ip.IsLoopback() {
			continue
		}
		out = append(out, ip.String())
	}
	return out
}
