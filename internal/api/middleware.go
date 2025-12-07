package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/tenant"
)

// TenantMiddleware adds tenant context extraction to requests.
// Looks for X-Tenant-ID header or tenant-id query parameter.
func TenantMiddleware(store tenant.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := extractTenantID(r)

			if tenantID == "" {
				slog.Warn("Missing tenant ID in request")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				resp := Response{
					Success: false,
					Error:   "missing tenant ID",
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			t, err := store.GetTenant(tenantID)
			if err != nil {
				slog.Warn("Tenant not found", "tenant_id", tenantID, "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				resp := Response{
					Success: false,
					Error:   "tenant not found",
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Add tenant to context
			ctx := tenant.WithTenant(r.Context(), t)
			slog.Info("Tenant context set", "tenant_id", t.ID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractTenantID extracts tenant ID from header or query parameter
func extractTenantID(r *http.Request) string {
	// Try X-Tenant-ID header first
	if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
		return strings.TrimSpace(tenantID)
	}

	// Try tenant-id query parameter
	if tenantID := r.URL.Query().Get("tenant-id"); tenantID != "" {
		return strings.TrimSpace(tenantID)
	}

	// Try x-tenant-id (lowercase) header
	if tenantID := r.Header.Get("x-tenant-id"); tenantID != "" {
		return strings.TrimSpace(tenantID)
	}

	return ""
}

// ValidateTenantInContext retrieves and validates tenant from request context
func ValidateTenantInContext(r *http.Request) (*tenant.Tenant, error) {
	t, err := tenant.FromContext(r.Context())
	if err != nil {
		return nil, err
	}
	return t, nil
}
