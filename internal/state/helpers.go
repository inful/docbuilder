package state

import (
    "git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// deleteEntity provides a unified pattern for Delete operations in JSON stores.
// It validates the key, performs existence check, invokes deleter, persists if needed,
// and returns a standardized result.
func deleteEntity(
    key string,
    exists func() bool,
    deleter func(),
    save func() error,
    notFoundName string,
    saveErrorMessage string,
) foundation.Result[struct{}, error] {
    if key == "" {
        return foundation.Err[struct{}, error](
            foundation.ValidationError("key cannot be empty").Build(),
        )
    }

    if !exists() {
        return foundation.Err[struct{}, error](
            foundation.NotFoundError(notFoundName).
                WithContext(foundation.Fields{"key": key}).
                Build(),
        )
    }

    deleter()

    if err := save(); err != nil {
        return foundation.Err[struct{}, error](
            foundation.InternalError(saveErrorMessage).WithCause(err).Build(),
        )
    }

    return foundation.Ok[struct{}, error](struct{}{})
}
