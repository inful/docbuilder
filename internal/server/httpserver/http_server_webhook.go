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

		// A path with braces is a wildcard of the HTTP router. It never
		// matches the path the forge calls, and a malformed one takes
		// the process down when the router is built. Refuse it here,
		// like the duplicate path.
		if strings.ContainsAny(path, "{}") {
			return nil, fmt.Errorf("invalid webhook path %q for forge %q: a path may not hold a wildcard", path, forgeCfg.Name)
		}

		if prev, ok := seen[path]; ok {
			return nil, fmt.Errorf("duplicate webhook path %q for forges %q and %q", path, prev, forgeCfg.Name)
		}
		seen[path] = forgeCfg.Name

		// A webhook without a secret accepts any POST and starts a build.
		// An unset secret is the usual reason: the config template points
		// at an environment variable that may not be set, and the empty
		// expansion left the endpoint open. Refuse to start rather than
		// serve an open build trigger.
		if forgeCfg.Webhook.Secret == "" {
			return nil, fmt.Errorf(
				"forge %q has a webhook endpoint (%s) but no secret: set forges[].webhook.secret, or remove forges[].webhook.path to disable the endpoint",
				forgeCfg.Name, path)
		}

		forgeName := forgeCfg.Name
		forgeType := forgeCfg.Type
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			s.webhookHandlers.HandleForgeWebhook(w, r, forgeName, forgeType)
		})
	}

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
