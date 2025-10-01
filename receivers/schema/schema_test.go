package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntegrationSchemaVersionGetSecretFieldsPaths(t *testing.T) {
	testCases := []struct {
		name     string
		version  IntegrationSchemaVersion
		expected []IntegrationFieldPath
	}{
		{
			name:     "no fields",
			version:  IntegrationSchemaVersion{},
			expected: nil,
		},
		{
			name: "no secure fields",
			version: IntegrationSchemaVersion{
				Options: []Field{
					{
						PropertyName: "test",
						Secure:       false,
						SubformOptions: []Field{
							{
								PropertyName: "test_nested",
							},
						},
					},
				},
			},
		},
		{
			name: "flat secure fields",
			version: IntegrationSchemaVersion{
				Options: []Field{
					{
						PropertyName: "test",
						Secure:       true,
					},
					{
						PropertyName: "test2",
						Secure:       false,
					},
					{
						PropertyName: "test3",
						Secure:       true,
					},
				},
			},
			expected: []IntegrationFieldPath{
				{"test"},
				{"test3"},
			},
		},
		{
			name: "nested secure fields",
			version: IntegrationSchemaVersion{
				Options: []Field{
					{
						PropertyName: "test",
						SubformOptions: []Field{
							{
								PropertyName: "child",
								SubformOptions: []Field{
									{
										PropertyName: "secured",
										Secure:       true,
									},
									{
										PropertyName: "secured2",
										Secure:       true,
									},
								},
							},
						},
					},
				},
			},
			expected: []IntegrationFieldPath{
				{"test", "child", "secured"},
				{"test", "child", "secured2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.EqualValues(t, tc.expected, tc.version.GetSecretFieldsPaths())
		})
	}
}

func TestIntegrationSchemaVersionIsSecureField(t *testing.T) {
	v := IntegrationSchemaVersion{
		Options: []Field{
			{
				PropertyName: "test",
				SubformOptions: []Field{
					{
						PropertyName: "child",
						Secure:       true,
					},
					{
						PropertyName: "child2",
						Secure:       false,
					},
				},
			},
			{
				PropertyName: "test2",
				Secure:       true,
			},
		},
	}

	testCases := []struct {
		name     string
		path     IntegrationFieldPath
		expected bool
	}{
		{
			name:     "nil path",
			path:     nil,
			expected: false,
		},
		{
			name:     "empty path",
			path:     IntegrationFieldPath{},
			expected: false,
		},
		{
			name:     "invalid nested path",
			path:     IntegrationFieldPath{"test", "child3"},
			expected: false,
		},
		{
			name:     "invalid path",
			path:     IntegrationFieldPath{"child2"},
			expected: false,
		},
		{
			name:     "existing nested path",
			path:     IntegrationFieldPath{"test", "child"},
			expected: true,
		},
		{
			name:     "existing path not secure",
			path:     IntegrationFieldPath{"test"},
			expected: false,
		},
		{
			name:     "existing path secure",
			path:     IntegrationFieldPath{"test2"},
			expected: true,
		},
		{
			name:     "upper case path",
			path:     IntegrationFieldPath{"TEST2"},
			expected: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, v.IsSecureField(testCase.path))
		})
	}
}

func TestIntegrationSchemaVersionGetField(t *testing.T) {
	v := IntegrationSchemaVersion{
		Options: []Field{
			{
				PropertyName: "test",
				SubformOptions: []Field{
					{
						PropertyName: "child",
						Secure:       true,
					},
					{
						PropertyName: "child2",
						Secure:       false,
					},
				},
			},
			{
				PropertyName: "test2",
				Secure:       true,
			},
		},
	}

	testCases := []struct {
		name     string
		path     IntegrationFieldPath
		expected Field
		missing  bool
	}{
		{
			name:    "nil path",
			path:    nil,
			missing: true,
		},
		{
			name:    "empty path",
			path:    IntegrationFieldPath{},
			missing: true,
		},
		{
			name:    "missing nested path",
			path:    IntegrationFieldPath{"test", "child3"},
			missing: true,
		},
		{
			name:    "missing path",
			path:    IntegrationFieldPath{"child2"},
			missing: true,
		},
		{
			name:     "existing nested path",
			path:     IntegrationFieldPath{"test", "child2"},
			expected: v.Options[0].SubformOptions[1],
		},
		{
			name:     "existing path",
			path:     IntegrationFieldPath{"test"},
			expected: v.Options[0],
		},
		{
			name:     "upper case path",
			path:     IntegrationFieldPath{"TEST", "CHiLD2"},
			expected: v.Options[0].SubformOptions[1],
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f, found := v.GetField(testCase.path)
			assert.Equal(t, testCase.missing, !found)
			assert.Equal(t, testCase.expected, f)
		})
	}
}
