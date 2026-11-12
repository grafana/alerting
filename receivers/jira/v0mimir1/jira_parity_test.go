package v0mimir1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/test"
	"github.com/prometheus/alertmanager/types"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/receivers"
)

func TestPrepareIssueRequestBodyOmitsEmptyDescription(t *testing.T) {
	for _, path := range []string{"/rest/api/2", "/rest/api/3/"} {
		t.Run(path, func(t *testing.T) {
			cfg := GetFullValidConfig()
			cfg.APIURL = &receivers.URL{URL: &url.URL{Scheme: "https", Host: "example.atlassian.net", Path: path}}
			cfg.Description = JiraFieldConfig{}
			n, err := New(&cfg, test.CreateTmpl(t), promslog.NewNopLogger())
			require.NoError(t, err)
			body, err := n.prepareIssueRequestBody(t.Context(), n.logger, "group", func(s string) (string, error) { return s, nil })
			require.NoError(t, err)
			encoded, err := json.Marshal(body)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), `"description"`)
		})
	}
}

func TestPrepareIssueRequestBodyTemplatesStructuredCustomFields(t *testing.T) {
	cfg := GetFullValidConfig()
	cfg.Fields = map[string]any{"number": "{{ . }}", "list": "[a, b]"}
	n, err := New(&cfg, test.CreateTmpl(t), promslog.NewNopLogger())
	require.NoError(t, err)
	body, err := n.prepareIssueRequestBody(t.Context(), n.logger, "group", func(s string) (string, error) {
		if s == "{{ . }}" {
			return "123", nil
		}
		return s, nil
	})
	require.NoError(t, err)
	require.Equal(t, 123, body.Fields.Fields["number"])
	require.Equal(t, []any{"a", "b"}, body.Fields.Fields["list"])
}

func TestPrepareIssueRequestBodyAPIv3TrailingSlashDescriptionValidation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		description string
		wantErr     bool
	}{
		{
			name:        "valid JSON description",
			description: `{"type":"doc","version":1,"content":[]}`,
		},
		{
			name:        "invalid JSON description",
			description: "not-json",
			wantErr:     true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := GetFullValidConfig()
			cfg.APIURL = &receivers.URL{URL: &url.URL{
				Scheme: "https",
				Host:   "example.atlassian.net",
				Path:   "/rest/api/3/",
			}}
			cfg.Description = JiraFieldConfig{Template: tc.description}
			n, err := New(&cfg, test.CreateTmpl(t), promslog.NewNopLogger())
			require.NoError(t, err)

			body, err := n.prepareIssueRequestBody(t.Context(), n.logger, "group", func(s string) (string, error) {
				return s, nil
			})
			if tc.wantErr {
				require.EqualError(t, err, "description template: invalid JSON for API v3")
				return
			}

			require.NoError(t, err)
			require.NotNil(t, body.Fields.Description)
			require.JSONEq(t, tc.description, string(body.Fields.Description.RawJSONDescription))
		})
	}
}

func TestTransitionIssueEmptyNameDoesNotRequestTransitions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		firing    bool
		statusKey string
	}{
		{name: "empty resolve transition", statusKey: "new"},
		{name: "empty reopen transition", firing: true, statusKey: "done"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)

			apiURL, err := url.Parse(server.URL + "/rest/api/2/")
			require.NoError(t, err)
			cfg := GetFullValidConfig()
			cfg.APIURL = &receivers.URL{URL: apiURL}
			cfg.ResolveTransition = ""
			cfg.ReopenTransition = ""
			n, err := New(&cfg, test.CreateTmpl(t), promslog.NewNopLogger())
			require.NoError(t, err)

			status := &issueStatus{}
			status.StatusCategory.Key = tc.statusKey
			retry, err := n.transitionIssue(t.Context(), n.logger, &issue{
				Key: "PROJ-1",
				Fields: &issueFields{
					Status: status,
				},
			}, tc.firing)

			require.NoError(t, err)
			require.False(t, retry)
			require.Zero(t, requests.Load())
		})
	}
}

func TestNotifierNilAPIURLReturnsError(t *testing.T) {
	cfg := Config{HTTPConfig: &httpcfg.HTTPClientConfig{}, APIType: "auto", Project: "PROJ", IssueType: "Bug"}
	n, err := New(&cfg, test.CreateTmpl(t), promslog.NewNopLogger())
	require.NoError(t, err)
	ctx := notify.WithGroupKey(t.Context(), "group")
	_, err = n.Notify(ctx, &types.Alert{Alert: model.Alert{Labels: model.LabelSet{"alertname": "test"}}})
	require.EqualError(t, err, "api_url is not set")
	_, _, err = n.doAPIRequest(ctx, http.MethodPost, "issue", nil)
	require.EqualError(t, err, "api_url is not set")
}
