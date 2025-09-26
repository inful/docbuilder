//go:build !prometheus

package daemon

import "net/http"

// prometheusOptionalHandler fallback when prometheus build tag not set.
func prometheusOptionalHandler() http.Handler { return nil }
