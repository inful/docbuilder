package state

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// deleteEntity provides a unified pattern for Delete operations in JSON stores.
// It validates the key, performs existence check, invokes deleter, persists if needed,
// and returns a standardized result.
func deleteEntity(
	key string,
	exists func() bool,
	deleter func(),
	notFoundName string,
	markDirty func(),
) foundation.Result[struct{}, error] {
	if key == "" {
		return foundation.Err[struct{}, error](
			errors.ValidationError("key cannot be empty").Build(),
		)
	}

	if !exists() {
		return foundation.Err[struct{}, error](
			errors.NotFoundError(notFoundName).
				WithContext("key", key).
				Build(),
		)
	}

	deleter()

	// The file is written when the state is saved.
	markDirty()

	return foundation.Ok[struct{}, error](struct{}{})
}
