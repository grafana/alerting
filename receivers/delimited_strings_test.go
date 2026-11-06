package receivers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDelimitedStrings(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  DelimitedStrings
	}{
		{"empty", "", nil},
		{"single", "one", DelimitedStrings{"one"}},
		{"mixed delimiters", "one,two;three\nfour", DelimitedStrings{"one", "two", "three", "four"}},
		{"empty segments", ",;one;;\ntwo,", DelimitedStrings{"one", "two"}},
		{"delimiters only", ",;\n", DelimitedStrings{}},
		{"whitespace around values", " one ;\ttwo\r\n ", DelimitedStrings{" one ", "\ttwo\r"}},
		{"whitespace only", " \t\r\n\u00a0\u2003", DelimitedStrings{}},
		{"blank entries", ", ;one;\t,\r\n\u00a0;two;\u2003,", DelimitedStrings{"one", "two"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, codec := range []struct {
				name      string
				marshal   func(any) ([]byte, error)
				unmarshal func([]byte, any) error
			}{
				{"JSON", json.Marshal, json.Unmarshal},
				{"YAML", yaml.Marshal, yaml.Unmarshal},
			} {
				t.Run(codec.name, func(t *testing.T) {
					data, err := codec.marshal(tc.input)
					require.NoError(t, err)
					got := DelimitedStrings{"previous"}
					require.NoError(t, codec.unmarshal(data, &got))
					require.Equal(t, tc.want, got)
					data, err = codec.marshal(got)
					require.NoError(t, err)
					var roundTrip DelimitedStrings
					require.NoError(t, codec.unmarshal(data, &roundTrip))
					require.Equal(t, got, roundTrip)
				})
			}
		})
	}
}

func TestDelimitedStringsInvalidJSON(t *testing.T) {
	for _, input := range []string{`[]`, `{}`, `123`, `true`, `"unterminated`} {
		t.Run(input, func(t *testing.T) {
			values := DelimitedStrings{"previous"}
			require.Error(t, json.Unmarshal([]byte(input), &values))
			require.Equal(t, DelimitedStrings{"previous"}, values)
		})
	}
}

func TestDelimitedStringsNullJSON(t *testing.T) {
	values := DelimitedStrings{"previous"}
	require.NoError(t, json.Unmarshal([]byte(`null`), &values))
	require.Nil(t, values)
}
