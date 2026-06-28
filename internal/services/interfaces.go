// Package services provides interfaces for service orchestration and lifecycle management.
package services

import (
	"context"
	"time"
)

// ManagedService defines the interface for services with lifecycle management.
type ManagedService interface {
	// Name returns the service name for logging and identification.
	Name() string

	// Start initializes and starts the service.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the service.
	Stop(ctx context.Context) error

	// Health returns the current health status of the service.
	Health() HealthStatus

	// Dependencies returns the names of services this service depends on.
	Dependencies() []string
}

// HealthStatus represents the health of a managed service, including status, message, and timestamp.
type HealthStatus struct {
	Status  string    `json:"status"`
	Message string    `json:"message,omitempty"`
	CheckAt time.Time `json:"check_at"`
}
