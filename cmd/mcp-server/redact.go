package main

import (
	"reflect"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// redactedMarker is the placeholder used to replace secret values anywhere
// they might leak through an MCP tool result. We never want the LLM to
// receive a raw token, password, or SSH key path — even if the host didn't
// redact.
const redactedMarker = "***"

// secretFields names the YAML/struct keys whose values must always be masked
// before they leave the server. Keep this list in sync with internal/config
// auth-related structs.
var secretFields = map[string]struct{}{
	"token":    {},
	"password": {},
	"key_path": {},
	"keypath":  {},
	"ssh_key":  {},
	"api_key":  {},
}

// redactConfig returns a deep copy of cfg with every value under a secret
// field replaced by "***". Auth blocks at any nesting depth are covered.
//
// A nil input is treated as an empty config so callers always get a
// non-nil pointer back — it's much friendlier for the LLM than a literal
// `null`.
//
// The returned config is safe to marshal to JSON for an MCP tool result
// and guaranteed not to mutate the input.
func redactConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return &config.Config{}
	}
	clone := *cfg
	// Allocate fresh slices so we can mutate Auth pointers without bleeding
	// changes back to the caller's Repositories / Forges backing array.
	if len(cfg.Repositories) > 0 {
		clone.Repositories = make([]config.Repository, len(cfg.Repositories))
		copy(clone.Repositories, cfg.Repositories)
	}
	if len(cfg.Forges) > 0 {
		clone.Forges = make([]*config.ForgeConfig, len(cfg.Forges))
		copy(clone.Forges, cfg.Forges)
	}
	redactAuthSlice(clone.Repositories)
	redactAuthSlice(clone.Forges)
	return &clone
}

// redactAuthSlice walks a slice of structs that contain an Auth field and
// masks the secret-shaped entries. The two relevant types are
// config.Repository and config.ForgeConfig; both expose an `Auth` of type
// config.AuthConfig, so we reflect through that rather than type-asserting
// twice.
//
// reflect-based scrubbing is intentional here: it stays correct when new
// auth fields are added to AuthConfig without forcing this file to be
// updated in lockstep. The trade-off is one-time reflection per response,
// which is negligible compared to network round-trips.
func redactAuthSlice(slice any) {
	v := reflect.ValueOf(slice)
	if v.Kind() != reflect.Slice {
		return
	}
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		if elem.Kind() != reflect.Struct {
			continue
		}
		auth := elem.FieldByName("Auth")
		if !auth.IsValid() || !auth.CanSet() {
			continue
		}
		// Pull the underlying AuthConfig value out (Auth may be a value or a
		// pointer depending on how the surrounding type declares it).
		authVal := auth
		if authVal.Kind() == reflect.Pointer {
			if authVal.IsNil() {
				continue
			}
			// Deep-copy so the original config is never mutated. Without
			// this, the cloned Config and the input would share the same
			// *AuthConfig and redaction would bleed back to the caller.
			orig := authVal.Elem()
			copyVal := reflect.New(orig.Type())
			copyVal.Elem().Set(orig)
			auth.Set(copyVal)
			authVal = copyVal.Elem()
		}
		if authVal.Kind() != reflect.Struct {
			continue
		}
		redactStructFields(authVal)
	}
}

// redactStructFields zeroes every string field whose name is in secretFields.
// Other fields (Username, Type) are left intact so the LLM can still reason
// about *which* auth is configured, just not *what* it is.
func redactStructFields(v reflect.Value) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		switch f.Kind() {
		case reflect.String:
			name := jsonTagOrName(t.Field(i))
			if _, ok := secretFields[name]; ok {
				if f.String() != "" {
					f.SetString(redactedMarker)
				}
			}
		case reflect.Struct:
			// Recurse into nested structs (e.g. for future AuthConfig nesting).
			redactStructFields(f)
		case reflect.Pointer:
			if !f.IsNil() && f.Elem().Kind() == reflect.Struct {
				redactStructFields(f.Elem())
			}
		}
	}
}

// jsonTagOrName prefers the JSON tag if present (so we match what callers
// actually see), falling back to the Go field name.
func jsonTagOrName(sf reflect.StructField) string {
	if tag, ok := sf.Tag.Lookup("json"); ok && tag != "" && tag != "-" {
		// Trim ",omitempty" etc.
		for i, c := range tag {
			if c == ',' {
				return tag[:i]
			}
		}
		return tag
	}
	if tag, ok := sf.Tag.Lookup("yaml"); ok && tag != "" && tag != "-" {
		for i, c := range tag {
			if c == ',' {
				return tag[:i]
			}
		}
		return tag
	}
	return sf.Name
}
