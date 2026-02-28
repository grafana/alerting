package receivers

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestJSONMarshalSecret(t *testing.T) {
	test := struct {
		S Secret
	}{
		S: Secret("test"),
	}

	c, err := json.Marshal(test)
	require.NoError(t, err)
	require.JSONEq(t, `{"S":"<secret>"}`, string(c), "Secret not properly elided.")
}

func TestJSONMarshalHideSecretURL(t *testing.T) {
	urlp, err := url.Parse("http://example.com/")
	require.NoError(t, err)
	u := &SecretURL{urlp}

	c, err := json.Marshal(u)
	require.NoError(t, err)
	// u003c -> "<"
	// u003e -> ">"
	require.Equal(t, "\"\\u003csecret\\u003e\"", string(c), "SecretURL not properly elided in JSON.")
	// Check that the marshaled data can be unmarshaled again.
	out := &SecretURL{}
	err = json.Unmarshal(c, out)
	require.NoError(t, err)

	c, err = yaml.Marshal(u)
	require.NoError(t, err)
	require.Equal(t, "<secret>\n", string(c), "SecretURL not properly elided in YAML.")
	// Check that the marshaled data can be unmarshaled again.
	out = &SecretURL{}
	err = yaml.Unmarshal(c, &out)
	require.NoError(t, err)
}

func TestUnmarshalSecretURL(t *testing.T) {
	b := []byte(`"http://example.com/se cret"`)
	var u SecretURL

	err := json.Unmarshal(b, &u)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/se%20cret", u.String(), "SecretURL not properly unmarshaled in JSON.")

	err = yaml.Unmarshal(b, &u)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/se%20cret", u.String(), "SecretURL not properly unmarshaled in YAML.")
}

func TestMarshalURL(t *testing.T) {
	for name, tc := range map[string]struct {
		input        *URL
		expectedJSON string
		expectedYAML string
	}{
		"url": {
			input:        MustParseURL("http://example.com/"),
			expectedJSON: "\"http://example.com/\"",
			expectedYAML: "http://example.com/\n",
		},

		"wrapped nil value": {
			input:        &URL{},
			expectedJSON: "null",
			expectedYAML: "null\n",
		},

		"wrapped empty URL": {
			input:        &URL{&url.URL{}},
			expectedJSON: "\"\"",
			expectedYAML: "\"\"\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			j, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expectedJSON, string(j), "URL not properly marshaled into JSON.")

			y, err := yaml.Marshal(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expectedYAML, string(y), "URL not properly marshaled into YAML.")
		})
	}
}

func TestUnmarshalNilURL(t *testing.T) {
	b := []byte(`null`)

	{
		var u URL
		err := json.Unmarshal(b, &u)
		require.Error(t, err, "unsupported scheme \"\" for URL")
	}

	{
		var u URL
		err := yaml.Unmarshal(b, &u)
		require.NoError(t, err)
	}
}

func TestUnmarshalEmptyURL(t *testing.T) {
	b := []byte(`""`)

	{
		var u URL
		err := json.Unmarshal(b, &u)
		require.Error(t, err, "unsupported scheme \"\" for URL")
		require.Equal(t, (*url.URL)(nil), u.URL)
	}

	{
		var u URL
		err := yaml.Unmarshal(b, &u)
		require.Error(t, err, "unsupported scheme \"\" for URL")
		require.Equal(t, (*url.URL)(nil), u.URL)
	}
}

func TestUnmarshalURL(t *testing.T) {
	b := []byte(`"http://example.com/a b"`)
	var u URL

	err := json.Unmarshal(b, &u)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/a%20b", u.String(), "URL not properly unmarshaled in JSON.")

	err = yaml.Unmarshal(b, &u)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/a%20b", u.String(), "URL not properly unmarshaled in YAML.")
}

func TestUnmarshalInvalidURL(t *testing.T) {
	for _, b := range [][]byte{
		[]byte(`"://example.com"`),
		[]byte(`"http:example.com"`),
		[]byte(`"telnet://example.com"`),
	} {
		var u URL

		err := json.Unmarshal(b, &u)
		require.Error(t, err, "Expected an error unmarshaling %q from JSON", string(b))

		err = yaml.Unmarshal(b, &u)
		require.Error(t, err, "Expected an error unmarshaling %q from YAML", string(b))
	}
}

func TestUnmarshalRelativeURL(t *testing.T) {
	b := []byte(`"/home"`)
	var u URL

	err := json.Unmarshal(b, &u)
	require.Error(t, err, "Expected an error parsing relative URL from JSON")

	err = yaml.Unmarshal(b, &u)
	require.Error(t, err, "Expected an error parsing relative URL from YAML")
}

func TestUnmarshalHostPort(t *testing.T) {
	for _, tc := range []struct {
		in string

		exp     HostPort
		jsonOut string
		yamlOut string
		err     bool
	}{
		{
			in:  `""`,
			exp: HostPort{},
			yamlOut: `""
`,
			jsonOut: `""`,
		},
		{
			in:  `"localhost:25"`,
			exp: HostPort{Host: "localhost", Port: "25"},
			yamlOut: `localhost:25
`,
			jsonOut: `"localhost:25"`,
		},
		{
			in:  `":25"`,
			exp: HostPort{Host: "", Port: "25"},
			yamlOut: `:25
`,
			jsonOut: `":25"`,
		},
		{
			in:  `"localhost"`,
			err: true,
		},
		{
			in:  `"localhost:"`,
			err: true,
		},
	} {
		t.Run(tc.in, func(t *testing.T) {
			hp := HostPort{}
			err := yaml.Unmarshal([]byte(tc.in), &hp)
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.exp, hp)

			b, err := yaml.Marshal(&hp)
			require.NoError(t, err)
			require.Equal(t, tc.yamlOut, string(b))

			b, err = json.Marshal(&hp)
			require.NoError(t, err)
			require.Equal(t, tc.jsonOut, string(b))
		})
	}
}
