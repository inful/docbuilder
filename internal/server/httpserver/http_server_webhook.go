package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func normalizeWebhookPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

func (s *Server) webhookMux() (*http.ServeMux, error) {
	mux := http.NewServeMux()

	// Forge-specific webhook endpoints (configured per forge instance)
	seen := map[string]string{}
	for _, forgeCfg := range s.cfg.Forges {
		if forgeCfg == nil || forgeCfg.Webhook == nil {
			continue
		}

		path := normalizeWebhookPath(forgeCfg.Webhook.Path)
		if path == "" {
			// Keep a predictable default for single-instance setups.
			path = "/webhooks/" + string(forgeCfg.Type)
		}

		if prev, ok := seen[path]; ok {
			return nil, fmt.Errorf("duplicate webhook path %q for forges %q and %q", path, prev, forgeCfg.Name)
		}
		seen[path] = forgeCfg.Name

		forgeName := forgeCfg.Name
		forgeType := forgeCfg.Type
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			s.webhookHandlers.HandleForgeWebhook(w, r, forgeName, forgeType)
		})
	}

	// Generic webhook endpoint (no signature validation, no build triggering)
	mux.HandleFunc("/webhook", s.webhookHandlers.HandleGenericWebhook)

	return mux, nil
}

func (s *Server) startWebhookServerWithListener(_ context.Context, ln net.Listener) error {
	mux, err := s.webhookMux()
	if err != nil {
		return err
	}

	s.webhookServer = &http.Server{Handler: s.mchain(mux), ReadTimeout: 30 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	return s.startServerWithListener("webhook", s.webhookServer, ln)
}
