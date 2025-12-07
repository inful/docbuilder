package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"git.home.luguber.info/inful/docbuilder/internal/quota"
	"git.home.luguber.info/inful/docbuilder/internal/tenant"
)

// QuotaMiddleware enforces quota limits for API requests
func QuotaMiddleware(manager *quota.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get tenant from context
			t, err := tenant.FromContext(r.Context())
			if err != nil {
				slog.Warn("No tenant in context for quota check")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				resp := Response{
					Success: false,
					Error:   "no tenant in context",
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// Check if this is a build creation request
			if r.Method == http.MethodPost && r.URL.Path == "/builds" {
				// Check quota
				if err := manager.CanCreateBuild(t.ID); err != nil {
					slog.Warn("Quota limit exceeded", "tenant_id", t.ID, "error", err)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)

					quotaErr := err.(*quota.QuotaLimitError)
					resp := Response{
						Success: false,
						Error:   quotaErr.Error(),
					}
					_ = json.NewEncoder(w).Encode(resp)
					return
				}

				// Increment quota counter
				if err := manager.IncrementBuild(t.ID); err != nil {
					slog.Error("Failed to increment build quota", "tenant_id", t.ID, "error", err)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					resp := Response{
						Success: false,
						Error:   "failed to create build",
					}
					_ = json.NewEncoder(w).Encode(resp)
					return
				}

				slog.Info("Build quota incremented", "tenant_id", t.ID)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// QuotaStatusHandler returns quota status for a tenant
func QuotaStatusHandler(manager *quota.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := tenant.FromContext(r.Context())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			resp := Response{
				Success: false,
				Error:   "no tenant in context",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		quotas, ok := manager.GetQuota(t.ID)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			resp := Response{
				Success: false,
				Error:   "no quotas found for tenant",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		usage := manager.GetUsage(t.ID)

		status := map[string]interface{}{
			"tenant_id": t.ID,
			"quotas": map[string]interface{}{
				"max_concurrent_builds": quotas.MaxConcurrentBuilds,
				"max_builds_per_day":    quotas.MaxBuildsPerDay,
				"max_storage_bytes":     quotas.MaxStorageBytes,
				"max_queue_size":        quotas.MaxQueueSize,
				"rate_limit_per_hour":   quotas.RateLimitPerHour,
			},
			"usage": map[string]interface{}{
				"concurrent_builds": usage.ConcurrentBuilds,
				"builds_today":      usage.BuildsToday,
				"storage_used":      usage.StorageUsedBytes,
				"queue_size":        usage.QueueSize,
				"builds_this_hour":  usage.BuildsThisHour,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := Response{
			Success: true,
			Data:    status,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
