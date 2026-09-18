package receivers

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

// DelimitedStrings encodes a slice of strings as a delimiter-separated string.
// Commas, semicolons, and newlines separate values. Empty segments are
// discarded, but whitespace within segments is preserved.
// An empty string decodes to nil; a nonempty string containing only delimiters
// decodes to a non-nil empty slice.
type DelimitedStrings []string

func parseDelimitedStrings(value string) DelimitedStrings {
	if value == "" {
		return nil
	}
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
}

func (a *DelimitedStrings) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*a = parseDelimitedStrings(value)
	return nil
}

func (a DelimitedStrings) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.string())
}

func (a *DelimitedStrings) UnmarshalYAML(node *yaml.Node) error {
	var value string
	if err := node.Decode(&value); err != nil {
		return err
	}
	*a = parseDelimitedStrings(value)
	return nil
}

func (a DelimitedStrings) MarshalYAML() (any, error) {
	return a.string(), nil
}

func (a DelimitedStrings) string() string {
	if a != nil && len(a) == 0 {
		// Preserve the distinction between empty and delimiter-only input.
		return ","
	}
	return strings.Join(a, ",")
}
