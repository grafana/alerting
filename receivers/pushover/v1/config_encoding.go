package v1

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/grafana/alerting/receivers"
)

type configPlain Config

// Only these fields differ between Config and its wire representation.
// NewConfig delays numeric conversion until after credential checks to preserve
// validation order and the partially populated configs returned on errors.
type configWire struct {
	*configPlain
	AlertingPriority receivers.OptionalNumber `json:"priority"`
	OKPriority       receivers.OptionalNumber `json:"okPriority"`
	Retry            receivers.OptionalNumber `json:"retry"`
	Expire           receivers.OptionalNumber `json:"expire"`
	Upload           *bool                    `json:"uploadImage"`
}

func (w configWire) applyNumbers(c *Config) error {
	var err error
	if w.AlertingPriority != "" {
		c.AlertingPriority, err = w.AlertingPriority.Int64()
		if err != nil {
			return fmt.Errorf("failed to convert alerting priority to integer: %w", err)
		}
	}
	if w.OKPriority != "" {
		c.OkPriority, err = w.OKPriority.Int64()
		if err != nil {
			return fmt.Errorf("failed to convert OK priority to integer: %w", err)
		}
	}
	c.Retry, _ = w.Retry.Int64()
	c.Expire, _ = w.Expire.Int64()
	return nil
}

func (w configWire) config() (Config, error) {
	c := Config(*w.configPlain)
	if err := w.applyNumbers(&c); err != nil {
		return Config{}, err
	}
	c.Upload = w.Upload == nil || *w.Upload
	return c, nil
}

func (c *Config) UnmarshalJSON(data []byte) error {
	wire := configWire{configPlain: &configPlain{}}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	decoded, err := wire.config()
	if err != nil {
		return err
	}
	*c = decoded
	return nil
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	var fields map[string]yaml.Node
	if err := node.Decode(&fields); err != nil {
		return err
	}
	wire := configWire{configPlain: &configPlain{}}
	for key, target := range map[string]*receivers.OptionalNumber{
		"priority": &wire.AlertingPriority, "okPriority": &wire.OKPriority,
		"retry": &wire.Retry, "expire": &wire.Expire,
	} {
		if value, ok := fields[key]; ok {
			for value.Kind == yaml.AliasNode {
				value = *value.Alias
			}
			if value.Kind == yaml.ScalarNode {
				if value.Tag != "!!null" {
					*target = receivers.OptionalNumber(value.Value)
				}
			} else {
				// As in JSON, nonnumeric inputs fail for priorities but are
				// ignored for retry/expire.
				*target = "invalid"
			}
			delete(fields, key)
		}
	}
	if value, ok := fields["uploadImage"]; ok {
		if err := value.Decode(&wire.Upload); err != nil {
			return err
		}
		delete(fields, "uploadImage")
	}
	var mapping yaml.Node
	if err := mapping.Encode(fields); err != nil {
		return err
	}
	if err := mapping.Decode(wire.configPlain); err != nil {
		return err
	}
	decoded, err := wire.config()
	if err != nil {
		return err
	}
	*c = decoded
	return nil
}
