package forge

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

var (
	// ErrForgeUnsupported signals that the forge type is not supported.
	ErrForgeUnsupported = errors.ForgeError("unsupported forge type").Fatal().Build()

	// ErrRepositoryNotFound signals that a repository was not found.
	ErrRepositoryNotFound = errors.ForgeError("repository not found").WithSeverity(errors.SeverityWarning).Build()

	// ErrOrgNotFound signals that an organization was not found.
	ErrOrgNotFound = errors.ForgeError("organization not found").WithSeverity(errors.SeverityWarning).Build()

	// ErrAuthRequired signals that authentication is required for a forge operation.
	ErrAuthRequired = errors.AuthError("authentication required for forge client").Build()

	// ErrWebhookNotConfigured signals that a webhook is not configured for a forge.
	ErrWebhookNotConfigured = errors.ForgeError("webhook not configured").WithSeverity(errors.SeverityWarning).Build()

	// ErrWebhookSecretMissing signals that a webhook secret is missing in configuration.
	ErrWebhookSecretMissing = errors.ConfigError("webhook secret missing").Build()

	// ErrInvalidPayload signals that a webhook payload is invalid.
	ErrInvalidPayload = errors.ForgeError("invalid webhook payload").Build()

	// ErrUnsupportedEvent signals that a webhook event type is not supported.
	ErrUnsupportedEvent = errors.ForgeError("unsupported webhook event type").WithSeverity(errors.SeverityInfo).Build()
)
