package metrics

import (
	"net/http"

	prom "github.com/prometheus/client_golang/prometheus"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPHandler returns an http.Handler that serves Prometheus metrics for the provided registry.
func HTTPHandler(reg *prom.Registry) http.Handler {
	if reg == nil {
		if typed, ok := prom.DefaultRegisterer.(*prom.Registry); ok { // check assertion to satisfy errcheck
			reg = typed
		}
	}
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: true})
}
