package definition

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/prometheus/alertmanager/config"
)

// LoadCompat loads a PostableApiAlertingConfig from a YAML configuration and runs validations.
func LoadCompat(rawCfg []byte) (*PostableApiAlertingConfig, error) {
	if len(rawCfg) == 0 {
		return nil, errors.New("empty input")
	}

	var c PostableApiAlertingConfig
	if err := yaml.Unmarshal(rawCfg, &c); err != nil {
		return nil, err
	}

	// Having a nil global config causes panics in the Alertmanager codebase.
	if c.Global == nil {
		c.Global = &config.GlobalConfig{}
		*c.Global = config.DefaultGlobalConfig()
	}

	// Check that receiver names are unique.
	names := map[string]struct{}{}
	for _, rcv := range c.Receivers {
		if _, ok := names[rcv.Name]; ok {
			return nil, fmt.Errorf("notification config name %q is not unique", rcv.Name)
		}
		names[rcv.Name] = struct{}{}
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func TemplatesMapToPostableAPITemplates(templates map[string]string, kind TemplateKind) []PostableApiTemplate {
	// Ensure a consistent ordering. This is important for:
	// - Hash calculations for change detection.
	// - Consistent template output since template definitions can override.
	res := make([]PostableApiTemplate, 0, len(templates))
	for _, k := range slices.SortedFunc(maps.Keys(templates), func(a, b string) int {
		return strings.Compare(a, b)
	}) {
		res = append(res, PostableApiTemplate{
			Name:    k,
			Kind:    kind,
			Content: templates[k],
		})
	}
	return res
}
