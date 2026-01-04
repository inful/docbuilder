// Package metrics provides an observability framework for DocBuilder build metrics.
//
// # Design Philosophy
//
// This package implements the Null Object pattern to enable metrics collection
// without requiring explicit nil checks throughout the codebase. By default,
// all components use NoopRecorder which implements the Recorder interface with
// no-op methods that inline to nothing at compile time.
//
// # Architecture
//
// The metrics system has three components:
//
//  1. Recorder interface - Defines all metrics operations
//  2. NoopRecorder - Default implementation that does nothing (zero overhead)
//  3. Real implementations - Prometheus/OpenTelemetry adapters (activated when needed)
//
// # Usage Pattern
//
// Components receive a Recorder through dependency injection:
//
//	type BuildService struct {
//	    recorder metrics.Recorder
//	}
//
//	func NewBuildService() *BuildService {
//	    return &BuildService{
//	        recorder: metrics.NoopRecorder{}, // Default: no metrics
//	    }
//	}
//
// # Activation
//
// To enable metrics, swap NoopRecorder for a real implementation:
//
//	// When Prometheus is configured
//	recorder := metrics.NewPrometheusRecorder(registry)
//	service := NewBuildService().WithRecorder(recorder)
//
// This approach allows:
//   - Zero overhead when metrics are disabled (noop methods inline away)
//   - Metrics activation without code changes (just swap implementation)
//   - Clean testing (inject mock recorder for verification)
//   - Gradual rollout (enable metrics per-component)
//
// # Current State
//
// All production code currently uses NoopRecorder. Real implementations exist
// (prometheus_http.go) but are not yet activated in the build pipeline. When
// metrics are needed, simply inject the appropriate recorder implementation.
package metrics
