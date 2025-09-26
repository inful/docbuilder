//go:build prometheus

package metrics

import (
	"net/http"

	prom "github.com/prometheus/client_golang/prometheus"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPHandler returns an http.Handler that serves Prometheus metrics for the provided registry.
func HTTPHandler(reg *prom.Registry) http.Handler {
	if reg == nil {
		reg = prom.DefaultRegisterer.(*prom.Registry)
	}
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: true})
}
