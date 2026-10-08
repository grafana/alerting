package v1

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"
)

// configPlain has Config's fields without its serialization methods.
type configPlain Config

// UnmarshalJSON adapts only fields whose wire representation differs from Config.
func (c *Config) UnmarshalJSON(data []byte) error {
	var decoded configPlain
	wire := struct {
		*configPlain
		URL            string `json:"api_url"`
		ReopenDuration string `json:"reopen_duration"`
	}{configPlain: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.URL != "" {
		u, err := url.Parse(wire.URL)
		if err != nil {
			return fmt.Errorf("field api_url is not a valid URL: %w", err)
		}
		decoded.URL = u
	}
	if wire.ReopenDuration != "" {
		d, err := model.ParseDuration(wire.ReopenDuration)
		if err != nil {
			return fmt.Errorf("field reopen_duration is not a valid duration: %w", err)
		}
		decoded.ReopenDuration = d
	}
	*c = Config(decoded)
	return nil
}

// MarshalJSON writes URLs as strings; model.Duration already marshals as a string.
func (c Config) MarshalJSON() ([]byte, error) {
	var apiURL string
	if c.URL != nil {
		apiURL = c.URL.String()
	}
	return json.Marshal(struct {
		configPlain
		URL string `json:"api_url,omitempty"`
	}{configPlain: configPlain(c), URL: apiURL})
}

// UnmarshalYAML uses the JSON representation so both formats apply the same
// URL/duration parsing and represent arbitrary custom fields consistently.
func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	var fields map[string]any
	if err := node.Decode(&fields); err != nil {
		return err
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return c.UnmarshalJSON(data)
}

func (c Config) MarshalYAML() (any, error) {
	data, err := c.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}
