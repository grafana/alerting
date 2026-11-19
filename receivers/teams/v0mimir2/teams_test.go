// Copyright 2024 Prometheus Team
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

package v0mimir2

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
	commoncfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	amcommoncfg "github.com/prometheus/alertmanager/config/common"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/msteamsv2"
	"github.com/prometheus/alertmanager/notify/test"
	"github.com/prometheus/alertmanager/types"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/receivers"
)

// This is a test URL that has been modified to not be valid.
var testWebhookURL, _ = url.Parse("https://example.westeurope.logic.azure.com:443/workflows/xxx/triggers/manual/paths/invoke?api-version=2016-06-01&sp=%2Ftriggers%2Fmanual%2Frun&sv=1.0&sig=xxx")

func TestMSTeamsV2Retry(t *testing.T) {
	notifier, err := New(
		&Config{
			WebhookURL: &receivers.SecretURL{URL: testWebhookURL},
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	for statusCode, expected := range test.RetryTests(test.DefaultRetryCodes()) {
		actual, _ := notifier.retrier.Check(statusCode, nil)
		require.Equal(t, expected, actual, "retry - error on status %d", statusCode)
	}
}

func TestNotifier_Notify_WithReason(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		responseContent string
		expectedReason  notify.Reason
		noError         bool
	}{
		{
			name:            "with a 2xx status code and response 1",
			statusCode:      http.StatusOK,
			responseContent: "1",
			noError:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := New(
				&Config{
					WebhookURL: &receivers.SecretURL{URL: testWebhookURL},
					HTTPConfig: &httpcfg.HTTPClientConfig{},
				},
				test.CreateTmpl(t),
				log.NewNopLogger(),
			)
			require.NoError(t, err)

			notifier.postJSONFunc = func(ctx context.Context, client *http.Client, url string, body io.Reader) (*http.Response, error) {
				resp := httptest.NewRecorder()
				_, _ = resp.WriteString(tt.responseContent)
				resp.WriteHeader(tt.statusCode)
				return resp.Result(), nil
			}
			ctx := context.Background()
			ctx = notify.WithGroupKey(ctx, "1")

			alert1 := &types.Alert{
				Alert: model.Alert{
					StartsAt: time.Now(),
					EndsAt:   time.Now().Add(time.Hour),
				},
			}
			_, err = notifier.Notify(ctx, alert1)
			if tt.noError {
				require.NoError(t, err)
			} else {
				var reasonError *notify.ErrorWithReason
				require.ErrorAs(t, err, &reasonError)
				require.Equal(t, tt.expectedReason, reasonError.Reason)
			}
		})
	}
}

func TestMSTeamsV2Templating(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dec := json.NewDecoder(r.Body)
		out := make(map[string]interface{})
		err := dec.Decode(&out)
		if err != nil {
			panic(err)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	for _, tc := range []struct {
		title string
		cfg   *Config

		retry  bool
		errMsg string
	}{
		{
			title: "full-blown message",
			cfg: &Config{
				Title: `{{ template "msteamsv2.default.title" . }}`,
				Text:  `{{ template "msteamsv2.default.text" . }}`,
			},
			retry: false,
		},
		{
			title: "title with templating errors",
			cfg: &Config{
				Title: "{{ ",
			},
			errMsg: "template: :1: unclosed action",
		},
		{
			title: "message with templating errors",
			cfg: &Config{
				Title: `{{ template "msteamsv2.default.title" . }}`,
				Text:  "{{ ",
			},
			errMsg: "template: :1: unclosed action",
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			tc.cfg.WebhookURL = &receivers.SecretURL{URL: u}
			tc.cfg.HTTPConfig = &httpcfg.HTTPClientConfig{}
			pd, err := New(tc.cfg, test.CreateTmpl(t), log.NewNopLogger())
			require.NoError(t, err)

			ctx := context.Background()
			ctx = notify.WithGroupKey(ctx, "1")

			ok, err := pd.Notify(ctx, []*types.Alert{
				{
					Alert: model.Alert{
						Labels: model.LabelSet{
							"lbl1": "val1",
						},
						StartsAt: time.Now(),
						EndsAt:   time.Now().Add(time.Hour),
					},
				},
			}...)
			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			}
			require.Equal(t, tc.retry, ok)
		})
	}
}

func TestMSTeamsV2PayloadParity(t *testing.T) {
	payloads := make(chan []byte, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		payloads <- body
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	const title = `Alert: {{ .Status }}`
	const text = "First line\nSecond line"
	tmpl := test.CreateTmpl(t)
	local, err := New(&Config{
		WebhookURL: &receivers.SecretURL{URL: u},
		HTTPConfig: &httpcfg.HTTPClientConfig{},
		Title:      title,
		Text:       text,
	}, tmpl, log.NewNopLogger())
	require.NoError(t, err)
	fork, err := msteamsv2.New(&msteamsv2.MSTeamsV2Config{
		WebhookURL: &amcommoncfg.SecretURL{URL: u},
		HTTPConfig: &commoncfg.HTTPClientConfig{},
		Title:      title,
		Text:       text,
	}, tmpl, slog.New(slog.DiscardHandler))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(notify.WithGroupKey(t.Context(), "1"), 5*time.Second)
	defer cancel()
	alert := &types.Alert{Alert: model.Alert{StartsAt: time.Now()}}
	_, err = local.Notify(ctx, alert)
	require.NoError(t, err)
	_, err = fork.Notify(ctx, alert)
	require.NoError(t, err)
	require.Len(t, payloads, 2)
	localPayload, forkPayload := <-payloads, <-payloads
	require.JSONEq(t, string(forkPayload), string(localPayload))

	var message struct {
		Attachments []struct {
			Content struct {
				Body []struct {
					Wrap   bool   `json:"wrap"`
					Weight string `json:"weight"`
				} `json:"body"`
			} `json:"content"`
		} `json:"attachments"`
	}
	require.NoError(t, json.Unmarshal(localPayload, &message))
	require.Len(t, message.Attachments, 1)
	blocks := message.Attachments[0].Content.Body
	require.Len(t, blocks, 2)
	require.Equal(t, "Bolder", blocks[0].Weight)
	require.True(t, blocks[0].Wrap)
	require.True(t, blocks[1].Wrap)
}

func TestMSTeamsV2RedactedURL(t *testing.T) {
	ctx, u, fn := test.GetContextWithCancelingURL()
	defer fn()

	secret := "secret"
	notifier, err := New(
		&Config{
			WebhookURL: &receivers.SecretURL{URL: u},
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, secret)
}
