package v1

import (
	"encoding/json"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/grafana/alerting/receivers"
)

// configPlain retains Config's fields without its decoding methods.
type configPlain Config

func (c *Config) UnmarshalJSON(data []byte) error {
	var decoded configPlain
	wire := struct {
		*configPlain
		MaxAlerts receivers.OptionalNumber `json:"maxAlerts"`
	}{configPlain: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	// Preserve the legacy conversion, including zero on invalid input and
	// strconv.Atoi's saturated value on overflow.
	decoded.MaxAlerts, _ = strconv.Atoi(wire.MaxAlerts.String())
	*c = Config(decoded)
	return nil
}

// UnmarshalYAML preserves YAML's native decoding for all fields except maxAlerts.
func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	var decoded configPlain
	// Decode nodes rather than interface values to preserve scalar text (e.g.
	// 1e2), while letting YAML resolve merge keys and reject duplicate keys.
	var fields map[string]yaml.Node
	if err := node.Decode(&fields); err != nil {
		return err
	}
	var maxAlerts string
	if value, ok := fields["maxAlerts"]; ok {
		for value.Kind == yaml.AliasNode {
			value = *value.Alias
		}
		if value.Kind == yaml.ScalarNode && value.Tag != "!!null" {
			maxAlerts = value.Value
		}
		fields["maxAlerts"] = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "0"}
	}
	var mapping yaml.Node
	if err := mapping.Encode(fields); err != nil {
		return err
	}
	if err := mapping.Decode(&decoded); err != nil {
		return err
	}
	decoded.MaxAlerts, _ = strconv.Atoi(maxAlerts)
	*c = Config(decoded)
	return nil
}
