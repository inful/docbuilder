// Package handlers contains HTTP handlers for the DocBuilder HTTP API.
//
// This package provides handlers for:
//   - Status and health endpoints (monitoring)
//   - Build and discovery operations
//   - Webhook endpoints across different forge providers (GitHub, GitLab, Forgejo)
//   - Shared response helper functions
//
// All handlers follow a consistent pattern for error handling and response formatting,
// using the foundation/errors package for structured error handling and the
// server/responses package for standardized HTTP responses.
package handlers
