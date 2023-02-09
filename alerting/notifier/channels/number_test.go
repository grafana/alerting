package channels

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNumberDecode(t *testing.T) {
	type Data struct {
		Num OptionalNumber `json:"num"`
	}

	tests := []struct {
		name          string
		json          string
		expected      int64
		expectedError string
	}{
		{
			name:     "empty string =  0",
			json:     `{ "num" : ""} `,
			expected: 0,
		},
		{
			name:          "invalid string =  0",
			json:          `{ "num" : "test"} `,
			expected:      0,
			expectedError: `parsing "test": invalid syntax`,
		},
		{
			name:     "can parse number",
			json:     `{ "num" : 12345555555} `,
			expected: 12345555555,
		},
		{
			name:     "can parse number as string",
			json:     `{ "num" : "12345555555" } `,
			expected: 12345555555,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := Data{}
			require.NoError(t, json.Unmarshal([]byte(test.json), &actual))
			num, err := actual.Num.Int64()
			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expected, num)
		})
	}
}
