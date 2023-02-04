package receivers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeSecrets(t *testing.T) {
	type Data struct {
		TaggedSecret   Secret `json:"tagged_secret"`
		MultiTagged    Secret `json:"multi_tagged" yaml:"secret"`
		UntaggedSecret Secret
	}

	testcases := []struct {
		name     string
		input    string
		secrets  map[string][]byte
		expected Data
	}{
		{
			name:  "should populate from secrets",
			input: `{ }`,
			secrets: map[string][]byte{
				"tagged_secret":  []byte("secret-value"),
				"multi_tagged":   []byte("multi-secret-value"),
				"UntaggedSecret": []byte("untagged-secret-value"),
			},
			expected: Data{
				TaggedSecret:   "secret-value",
				MultiTagged:    "multi-secret-value",
				UntaggedSecret: "untagged-secret-value",
			},
		},
		{
			name:  "should override from secrets",
			input: `{ "tagged_secret": "test", "multi_tagged" : "test2", "UntaggedSecret": "test3"}`,
			secrets: map[string][]byte{
				"tagged_secret":  []byte("secret-value"),
				"multi_tagged":   []byte("multi-secret-value"),
				"UntaggedSecret": []byte("untagged-secret-value"),
			},
			expected: Data{
				TaggedSecret:   "secret-value",
				MultiTagged:    "multi-secret-value",
				UntaggedSecret: "untagged-secret-value",
			},
		},
		{
			name:    "should not change original if missing secret",
			input:   `{ "tagged_secret": "test", "multi_tagged" : "test2", "UntaggedSecret": "test3"}`,
			secrets: map[string][]byte{},
			expected: Data{
				TaggedSecret:   "test",
				MultiTagged:    "test2",
				UntaggedSecret: "test3",
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			json := CreateMarshallerWithSecretsDecrypt(func(_ context.Context, sjd map[string][]byte, key string, fallback string) string {
				v, ok := sjd[key]
				if !ok {
					return fallback
				}
				return string(v)
			}, testcase.secrets)
			var actual Data
			require.NoError(t, json.Unmarshal([]byte(testcase.input), &actual))

			require.Equal(t, testcase.expected, actual)
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	type Data struct {
		TaggedSecret   Secret `json:"tagged_secret"`
		MultiTagged    Secret `json:"multi_tagged" yaml:"secret"`
		UntaggedSecret Secret
	}

	decrypt := func(_ context.Context, sjd map[string][]byte, key string, fallback string) string {
		v, ok := sjd[key]
		if !ok {
			return fallback
		}
		return string(v)
	}

	input := `{ "tagged_secret": "test", "multi_tagged" : "test2", "UntaggedSecret": "test3"}`
	secrets := map[string][]byte{
		"tagged_secret":  []byte("secret-value"),
		"multi_tagged":   []byte("multi-secret-value"),
		"UntaggedSecret": []byte("untagged-secret-value"),
	}

	b.Run("Decoder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			json := CreateMarshallerWithSecretsDecrypt(decrypt, secrets)
			var actual Data
			_ = json.Unmarshal([]byte(input), &actual)
		}
		b.ReportAllocs()
	})
	b.Run("explicit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var actual Data
			_ = json.Unmarshal([]byte(input), &actual)
			actual.TaggedSecret = Secret(decrypt(context.Background(), secrets, "tagged_secret", string(actual.TaggedSecret)))
			actual.MultiTagged = Secret(decrypt(context.Background(), secrets, "multi_tagged", string(actual.MultiTagged)))
			actual.UntaggedSecret = Secret(decrypt(context.Background(), secrets, "UntaggedSecret", string(actual.UntaggedSecret)))
		}
		b.ReportAllocs()
	})
}
