package v1

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/grafana/alerting/receivers"
)

type configPlain Config

const alertsPath = "/api/v2/alerts"

// UnmarshalJSON converts the comma-separated base URLs to notification endpoints.
func (c *Config) UnmarshalJSON(data []byte) error {
	var decoded configPlain
	wire := struct {
		*configPlain
		URLs receivers.CommaSeparatedStrings `json:"url"`
	}{configPlain: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	urls, err := parseURLs(wire.URLs)
	if err != nil {
		return err
	}
	decoded.URLs = urls
	*c = Config(decoded)
	return nil
}

func parseURLs(values []string) ([]*url.URL, error) {
	urls := make([]*url.URL, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		u, err := url.Parse(strings.TrimSuffix(value, "/") + alertsPath)
		if err != nil {
			return nil, fmt.Errorf("invalid url property in settings: %w", err)
		}
		urls = append(urls, u)
	}
	return urls, nil
}

// MarshalJSON reverses the endpoint suffix added during decoding.
func (c Config) MarshalJSON() ([]byte, error) {
	urls, err := c.baseURLs()
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		configPlain
		URLs string `json:"url,omitempty"`
	}{configPlain: configPlain(c), URLs: urls})
}

func (c Config) baseURLs() (string, error) {
	values := make([]string, 0, len(c.URLs))
	for _, u := range c.URLs {
		if u == nil {
			return "", fmt.Errorf("cannot encode nil URL")
		}
		value := u.String()
		if strings.HasSuffix(value, alertsPath) {
			// Restore one trailing slash so decoding removes exactly that slash,
			// preserving repeated slashes and the root-relative endpoint.
			value = strings.TrimSuffix(value, alertsPath) + "/"
		}
		values = append(values, value)
	}
	return strings.Join(values, ","), nil
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	var fields map[string]yaml.Node
	if err := node.Decode(&fields); err != nil {
		return err
	}
	var rawURLs string
	if value, ok := fields["url"]; ok {
		if err := value.Decode(&rawURLs); err != nil {
			return err
		}
		delete(fields, "url")
	}
	var mapping yaml.Node
	if err := mapping.Encode(fields); err != nil {
		return err
	}
	var decoded configPlain
	if err := mapping.Decode(&decoded); err != nil {
		return err
	}
	urls, err := parseURLs(strings.Split(rawURLs, ","))
	if err != nil {
		return err
	}
	decoded.URLs = urls
	*c = Config(decoded)
	return nil
}

func (c Config) MarshalYAML() (any, error) {
	urls, err := c.baseURLs()
	if err != nil {
		return nil, err
	}
	var node yaml.Node
	if err := node.Encode(configPlain(c)); err != nil {
		return nil, err
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == "url" {
			node.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: urls}
		}
	}
	return &node, nil
}
