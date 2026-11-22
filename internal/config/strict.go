package config

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// unmarshalStrict parses a config file and rejects keys the config schema
// does not know about.
//
// Without this, a key that was removed from the schema — or a typo — is
// silently dropped, and the operator is left believing the setting applies.
// The decoder names every unknown key and the line it is on.
//
// A key that the schema knows somewhere else gets a pointer to where it
// belongs: "outbound" is accepted at "daemon.outbound", not at the top
// level.
func unmarshalStrict(data []byte) (*Config, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var config Config
	if err := decoder.Decode(&config); err != nil {
		return nil, addKeyHints(err)
	}
	return &config, nil
}

// unknownFieldRE matches the message yaml.v3 produces for a key the
// schema does not know: "line 2: field outbound not found in type
// config.Config".
var unknownFieldRE = regexp.MustCompile(`field (\w+) not found in type [\w.]+`)

// keyPaths maps a yaml key to the paths in the schema that accept it,
// e.g. "outbound" to ["daemon.outbound"].
var keyPaths = buildKeyPaths()

// buildKeyPaths walks the config schema and records where every key is
// accepted.
func buildKeyPaths() map[string][]string {
	paths := map[string][]string{}
	collectKeyPaths(reflect.TypeFor[Config](), "", paths)
	return paths
}

func collectKeyPaths(t reflect.Type, prefix string, out map[string][]string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for _, f := range reflect.VisibleFields(t) {
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		out[name] = append(out[name], path)

		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			collectKeyPaths(ft, path, out)
		}
	}
}

// addKeyHints rewrites the yaml error so that a key the schema knows
// elsewhere says where it belongs.
func addKeyHints(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}
	msgs := make([]string, 0, len(typeErr.Errors))
	for _, msg := range typeErr.Errors {
		if m := unknownFieldRE.FindStringSubmatch(msg); m != nil {
			if paths := keyPaths[m[1]]; len(paths) > 0 {
				quoted := make([]string, 0, len(paths))
				for _, p := range paths {
					quoted = append(quoted, fmt.Sprintf("%q", p))
				}
				msg = fmt.Sprintf("%s; %q is accepted at %s", msg, m[1], strings.Join(quoted, ", "))
			}
		}
		msgs = append(msgs, msg)
	}
	if len(msgs) == 0 {
		return err
	}
	return &yaml.TypeError{Errors: msgs}
}
