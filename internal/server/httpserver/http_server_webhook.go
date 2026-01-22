package httpserver

import (
	"context"
	"net"
	"net/http"
	"time"
)

func (s *Server) startWebhookServerWithListener(_ context.Context, ln net.Listener) error {
	mux := http.NewServeMux()

	// Webhook endpoints for each forge type
	mux.HandleFunc("/webhooks/github", s.webhookHandlers.HandleGitHubWebhook)
	mux.HandleFunc("/webhooks/gitlab", s.webhookHandlers.HandleGitLabWebhook)
	mux.HandleFunc("/webhooks/forgejo", s.webhookHandlers.HandleForgejoWebhook)

	// Generic webhook endpoint (auto-detects forge type)
	mux.HandleFunc("/webhook", s.webhookHandlers.HandleGenericWebhook)

	s.webhookServer = &http.Server{Handler: s.mchain(mux), ReadTimeout: 30 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	return s.startServerWithListener("webhook", s.webhookServer, ln)
}
