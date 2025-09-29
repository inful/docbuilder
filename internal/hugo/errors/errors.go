package errors

// Package errors provides sentinel errors for Hugo site generation stages.
// These enable consistent classification (expanded in Phase 4) while keeping
// user‑facing messages descriptive via wrapping.

import "errors"

var (
    // ErrHugoBinaryNotFound indicates the hugo executable was not detected on PATH.
    ErrHugoBinaryNotFound = errors.New("hugo binary not found")
    // ErrHugoExecutionFailed indicates the hugo command returned a non‑zero exit status.
    ErrHugoExecutionFailed = errors.New("hugo execution failed")
    // ErrConfigMarshalFailed indicates marshaling the generated Hugo configuration failed.
    ErrConfigMarshalFailed = errors.New("hugo config marshal failed")
    // ErrConfigWriteFailed indicates writing the hugo.yaml file failed.
    ErrConfigWriteFailed = errors.New("hugo config write failed")
)
