package v1

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// configPlain retains Config's fields without its decoding methods.
type configPlain Config

func (c *Config) UnmarshalJSON(data []byte) error {
	decoded := configPlain{AutoClose: true, OverridePriority: true}
	wire := struct {
		*configPlain
		AutoClose        *bool `json:"autoClose"`
		OverridePriority *bool `json:"overridePriority"`
	}{configPlain: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	// Pointer fields preserve null clearing an earlier value in duplicate keys,
	// as well as distinguishing explicit false from missing/null values.
	if wire.AutoClose != nil {
		decoded.AutoClose = *wire.AutoClose
	}
	if wire.OverridePriority != nil {
		decoded.OverridePriority = *wire.OverridePriority
	}
	*c = Config(decoded)
	return nil
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	// YAML rejects duplicate keys and leaves scalar defaults unchanged on null.
	decoded := configPlain{AutoClose: true, OverridePriority: true}
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*c = Config(decoded)
	return nil
}
