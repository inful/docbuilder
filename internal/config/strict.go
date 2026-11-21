package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// unmarshalStrict parses a config file and rejects keys the config schema
// does not know about.
//
// Without this, a key that was removed from the schema — or a typo — is
// silently dropped, and the operator is left believing the setting applies.
// The decoder names every unknown key and the line it is on.
func unmarshalStrict(data []byte) (*Config, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var config Config
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return &config, nil
}
