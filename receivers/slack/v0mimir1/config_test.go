// Copyright 2018 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v0mimir1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
)

func TestLoadSlackConfiguration(t *testing.T) {
	tests := []struct {
		in       string
		expected Config
	}{
		{
			in: `
color: green
username: mark
channel: engineering
title_link: http://example.com/
image_url: https://example.com/logo.png
`,
			expected: Config{
				Color: "green", Username: "mark", Channel: "engineering",
				TitleLink: "http://example.com/",
				ImageURL:  "https://example.com/logo.png",
			},
		},
		{
			in: `
color: green
username: mark
channel: alerts
title_link: http://example.com/alert1
mrkdwn_in:
- pretext
- text
`,
			expected: Config{
				Color: "green", Username: "mark", Channel: "alerts",
				MrkdwnIn: []string{"pretext", "text"}, TitleLink: "http://example.com/alert1",
			},
		},
	}
	for _, rt := range tests {
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(rt.in), &cfg)
		if err != nil {
			t.Fatalf("\nerror returned when none expected, error:\n%v", err)
		}
		if rt.expected.Color != cfg.Color {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", rt.expected.Color, cfg.Color)
		}
		if rt.expected.Username != cfg.Username {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", rt.expected.Username, cfg.Username)
		}
		if rt.expected.Channel != cfg.Channel {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", rt.expected.Channel, cfg.Channel)
		}
		if rt.expected.ThumbURL != cfg.ThumbURL {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", rt.expected.ThumbURL, cfg.ThumbURL)
		}
		if rt.expected.TitleLink != cfg.TitleLink {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", rt.expected.TitleLink, cfg.TitleLink)
		}
		if rt.expected.ImageURL != cfg.ImageURL {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", rt.expected.ImageURL, cfg.ImageURL)
		}
		if len(rt.expected.MrkdwnIn) != len(cfg.MrkdwnIn) {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", rt.expected.MrkdwnIn, cfg.MrkdwnIn)
		}
		for i := range cfg.MrkdwnIn {
			if rt.expected.MrkdwnIn[i] != cfg.MrkdwnIn[i] {
				t.Errorf("\nexpected:\n%v\ngot:\n%v\nat index %v", rt.expected.MrkdwnIn[i], cfg.MrkdwnIn[i], i)
			}
		}
	}
}

func TestSlackFieldConfigValidation(t *testing.T) {
	tests := []struct {
		in       string
		expected string
	}{
		{
			in: `
fields:
- title: first
  value: hello
- title: second
`,
			expected: "missing value in Slack field configuration",
		},
		{
			in: `
fields:
- title: first
  value: hello
  short: true
- value: world
  short: true
`,
			expected: "missing title in Slack field configuration",
		},
		{
			in: `
fields:
- title: first
  value: hello
  short: true
- title: second
  value: world
`,
			expected: "",
		},
	}

	for _, rt := range tests {
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(rt.in), &cfg)

		// Check if an error occurred when it was NOT expected to.
		if rt.expected == "" && err != nil {
			t.Fatalf("\nerror returned when none expected, error:\n%v", err)
		}
		// Check that an error occurred if one was expected to.
		if rt.expected != "" && err == nil {
			t.Fatalf("\nno error returned, expected:\n%v", rt.expected)
		}
		// Check that the error that occurred was what was expected.
		if err != nil && err.Error() != rt.expected {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", rt.expected, err.Error())
		}
	}
}

func TestSlackFieldConfigUnmarshaling(t *testing.T) {
	in := `
fields:
- title: first
  value: hello
  short: true
- title: second
  value: world
- title: third
  value: slack field test
  short: false
`
	expected := []*SlackField{
		{
			Title: "first",
			Value: "hello",
			Short: newBoolPointer(true),
		},
		{
			Title: "second",
			Value: "world",
			Short: nil,
		},
		{
			Title: "third",
			Value: "slack field test",
			Short: newBoolPointer(false),
		},
	}

	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)
	if err != nil {
		t.Fatalf("\nerror returned when none expected, error:\n%v", err)
	}

	for index, field := range cfg.Fields {
		exp := expected[index]
		if field.Title != exp.Title {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.Title, field.Title)
		}
		if field.Value != exp.Value {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.Value, field.Value)
		}
		if exp.Short == nil && field.Short != nil {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.Short, *field.Short)
		}
		if exp.Short != nil && field.Short == nil {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", *exp.Short, field.Short)
		}
		if exp.Short != nil && *exp.Short != *field.Short {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", *exp.Short, *field.Short)
		}
	}
}

func TestSlackActionsValidation(t *testing.T) {
	in := `
actions:
- type: button
  text: hello
  url: https://localhost
  style: danger
- type: button
  text: hello
  name: something
  style: default
  confirm:
    title: please confirm
    text: are you sure?
    ok_text: yes
    dismiss_text: no
`
	expected := []*SlackAction{
		{
			Type:  "button",
			Text:  "hello",
			URL:   "https://localhost",
			Style: "danger",
		},
		{
			Type:  "button",
			Text:  "hello",
			Name:  "something",
			Style: "default",
			ConfirmField: &SlackConfirmationField{
				Title:       "please confirm",
				Text:        "are you sure?",
				OkText:      "yes",
				DismissText: "no",
			},
		},
	}

	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)
	if err != nil {
		t.Fatalf("\nerror returned when none expected, error:\n%v", err)
	}

	for index, action := range cfg.Actions {
		exp := expected[index]
		if action.Type != exp.Type {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.Type, action.Type)
		}
		if action.Text != exp.Text {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.Text, action.Text)
		}
		if action.URL != exp.URL {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.URL, action.URL)
		}
		if action.Style != exp.Style {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.Style, action.Style)
		}
		if action.Name != exp.Name {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.Name, action.Name)
		}
		if action.Value != exp.Value {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.Value, action.Value)
		}
		if action.ConfirmField != nil && exp.ConfirmField == nil || action.ConfirmField == nil && exp.ConfirmField != nil {
			t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.ConfirmField, action.ConfirmField)
		} else if action.ConfirmField != nil && exp.ConfirmField != nil {
			if action.ConfirmField.Title != exp.ConfirmField.Title {
				t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.ConfirmField.Title, action.ConfirmField.Title)
			}
			if action.ConfirmField.Text != exp.ConfirmField.Text {
				t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.ConfirmField.Text, action.ConfirmField.Text)
			}
			if action.ConfirmField.OkText != exp.ConfirmField.OkText {
				t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.ConfirmField.OkText, action.ConfirmField.OkText)
			}
			if action.ConfirmField.DismissText != exp.ConfirmField.DismissText {
				t.Errorf("\nexpected:\n%v\ngot:\n%v", exp.ConfirmField.DismissText, action.ConfirmField.DismissText)
			}
		}
	}
}

func newBoolPointer(b bool) *bool {
	return &b
}

func TestValidate(t *testing.T) {
	t.Run("FullValidConfigForTesting is valid", func(t *testing.T) {
		var cfg Config
		err := yaml.UnmarshalStrict([]byte(FullValidConfigForTesting), &cfg)
		require.NoError(t, err)
		require.NoError(t, cfg.Validate())
	})
	cases := []struct {
		name        string
		mutate      func(cfg *Config)
		expectedErr string
	}{
		{
			name:   "GetFullValidConfig is valid",
			mutate: func(cfg *Config) {},
		},
		{
			name:        "Missing api_url",
			mutate:      func(cfg *Config) { cfg.APIURL = nil },
			expectedErr: "missing api_url",
		},
		{
			name: "Invalid field - missing title",
			mutate: func(cfg *Config) {
				cfg.Fields = []*SlackField{{Value: "val"}}
			},
			expectedErr: "invalid fields[0]: missing title",
		},
		{
			name: "Invalid field - missing value",
			mutate: func(cfg *Config) {
				cfg.Fields = []*SlackField{{Title: "title"}}
			},
			expectedErr: "invalid fields[0]: missing value",
		},
		{
			name: "Invalid action - missing type",
			mutate: func(cfg *Config) {
				cfg.Actions = []*SlackAction{{Text: "text", URL: "http://localhost"}}
			},
			expectedErr: "invalid actions[0]: missing type",
		},
		{
			name: "Invalid action - missing text",
			mutate: func(cfg *Config) {
				cfg.Actions = []*SlackAction{{Type: "button", URL: "http://localhost"}}
			},
			expectedErr: "invalid actions[0]: missing text",
		},
		{
			name: "Invalid action - missing name or url",
			mutate: func(cfg *Config) {
				cfg.Actions = []*SlackAction{{Type: "button", Text: "text"}}
			},
			expectedErr: "invalid actions[0]: missing name or url",
		},
		{
			name: "Invalid action confirm - missing text",
			mutate: func(cfg *Config) {
				cfg.Actions = []*SlackAction{{
					Type: "button", Text: "text", Name: "name",
					ConfirmField: &SlackConfirmationField{Title: "title"},
				}}
			},
			expectedErr: "invalid actions[0]: invalid confirm: missing text",
		},
		{
			name: "Invalid http_config",
			mutate: func(cfg *Config) {
				cfg.HTTPConfig = &httpcfg.HTTPClientConfig{
					BasicAuth: &httpcfg.BasicAuth{},
					OAuth2:    &httpcfg.OAuth2{},
				}
			},
			expectedErr: "invalid http_config",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := GetFullValidConfig()
			c.mutate(&cfg)
			err := cfg.Validate()

			if c.expectedErr != "" {
				require.ErrorContains(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewConfig(t *testing.T) {
	defaultHTTPConfig := httpcfg.DefaultHTTPClientConfig

	cases := []struct {
		name              string
		settings          string
		secrets           map[string][]byte
		expectedConfig    Config
		expectedInitError string
	}{
		{
			name:              "Error if empty",
			settings:          "",
			expectedInitError: "failed to unmarshal settings",
		},
		{
			name:              "Error if missing api_url",
			settings:          `{"channel": "#test"}`,
			expectedInitError: "missing api_url",
		},
		{
			name: "Minimal valid configuration",
			settings: `{
				"api_url": "http://slack.example.com/webhook"
			}`,
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: false},
				HTTPConfig:     &defaultHTTPConfig,
				APIURL:         receivers.MustParseSecretURL("http://slack.example.com/webhook"),
				Color:          DefaultConfig.Color,
				Username:       DefaultConfig.Username,
				Title:          DefaultConfig.Title,
				TitleLink:      DefaultConfig.TitleLink,
				IconEmoji:      DefaultConfig.IconEmoji,
				IconURL:        DefaultConfig.IconURL,
				Pretext:        DefaultConfig.Pretext,
				Text:           DefaultConfig.Text,
				Fallback:       DefaultConfig.Fallback,
				CallbackID:     DefaultConfig.CallbackID,
				Footer:         DefaultConfig.Footer,
			},
		},
		{
			name:     "Secret api_url from decrypt",
			settings: `{}`,
			secrets: map[string][]byte{
				"api_url": []byte("http://slack.example.com/secret-webhook"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: false},
				HTTPConfig:     &defaultHTTPConfig,
				APIURL:         receivers.MustParseSecretURL("http://slack.example.com/secret-webhook"),
				Color:          DefaultConfig.Color,
				Username:       DefaultConfig.Username,
				Title:          DefaultConfig.Title,
				TitleLink:      DefaultConfig.TitleLink,
				IconEmoji:      DefaultConfig.IconEmoji,
				IconURL:        DefaultConfig.IconURL,
				Pretext:        DefaultConfig.Pretext,
				Text:           DefaultConfig.Text,
				Fallback:       DefaultConfig.Fallback,
				CallbackID:     DefaultConfig.CallbackID,
				Footer:         DefaultConfig.Footer,
			},
		},
		{
			name: "Secret overrides setting",
			settings: `{
				"api_url": "http://original.example.com/webhook"
			}`,
			secrets: map[string][]byte{
				"api_url": []byte("http://override.example.com/webhook"),
			},
			expectedConfig: Config{
				NotifierConfig: receivers.NotifierConfig{VSendResolved: false},
				HTTPConfig:     &defaultHTTPConfig,
				APIURL:         receivers.MustParseSecretURL("http://override.example.com/webhook"),
				Color:          DefaultConfig.Color,
				Username:       DefaultConfig.Username,
				Title:          DefaultConfig.Title,
				TitleLink:      DefaultConfig.TitleLink,
				IconEmoji:      DefaultConfig.IconEmoji,
				IconURL:        DefaultConfig.IconURL,
				Pretext:        DefaultConfig.Pretext,
				Text:           DefaultConfig.Text,
				Fallback:       DefaultConfig.Fallback,
				CallbackID:     DefaultConfig.CallbackID,
				Footer:         DefaultConfig.Footer,
			},
		},
		{
			name:     "FullValidConfigForTesting is valid",
			settings: FullValidConfigForTesting,
			expectedConfig: func() Config {
				cfg := DefaultConfig
				_ = json.Unmarshal([]byte(FullValidConfigForTesting), &cfg)
				httpCfg := httpcfg.DefaultHTTPClientConfig
				cfg.HTTPConfig = &httpCfg
				// Validate() calls action.validate() which clears Name/Value/ConfirmField when URL is set
				_ = cfg.Validate()
				return cfg
			}(),
		},
		{
			name:     "GetFullValidConfig round-trips through JSON",
			settings: func() string { b, _ := json.Marshal(GetFullValidConfig()); return string(b) }(),
			secrets: map[string][]byte{
				"api_url": []byte(GetFullValidConfig().APIURL.String()),
			},
			expectedConfig: func() Config {
				cfg := GetFullValidConfig()
				cfg.HTTPConfig = &defaultHTTPConfig
				return cfg
			}(),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(c.secrets))

			if c.expectedInitError != "" {
				require.ErrorContains(t, err, c.expectedInitError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.expectedConfig, actual)
		})
	}
}
