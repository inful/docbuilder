// Package handlers provides shared response helper functions for HTTP handlers.
package handlers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// writeJSON serializes the provided value to JSON and writes it with the given
// status code. It sets a consistent Content-Type header. Encoding is performed
// into an intermediate buffer so that we don't send partial responses if
// serialization fails. On failure it attempts to write a 500 error (unless
// headers/body already sent) and returns the encode error.
func writeJSON(w http.ResponseWriter, status int, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		// Do not write fallback responses here; let callers surface via their adapters.
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.Error("failed writing JSON response body", logfields.Error(err))
		return err
	}
	return nil
}

// writeJSONPretty optionally pretty prints when pretty=true via query parameter.
// It falls back to compact form if marshalling fails for any reason.
func writeJSONPretty(w http.ResponseWriter, r *http.Request, status int, v any) error {
	if r != nil {
		if p := r.URL.Query().Get("pretty"); p == "1" || p == "true" {
			b, err := json.MarshalIndent(v, "", "  ")
			if err == nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(status)
				if _, werr := w.Write(append(b, '\n')); werr != nil { // newline parity with Encoder
					slog.Error("failed writing pretty JSON", logfields.Error(werr))
					return werr
				}
				return nil
			}
			slog.Warn("pretty JSON marshal failed, falling back to standard encode", logfields.Error(err))
		}
	}
	return writeJSON(w, status, v)
}
