// Copyright 2019 Prometheus Team
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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/test"
	"github.com/prometheus/alertmanager/types"

	httpcfg "github.com/grafana/alerting/http/v0mimir1"
	"github.com/grafana/alerting/receivers"
)

func TestPagerDutyRetryV1(t *testing.T) {
	notifier, err := New(
		&Config{
			ServiceKey: receivers.Secret("01234567890123456789012345678901"),
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	retryCodes := append(test.DefaultRetryCodes(), http.StatusForbidden)
	for statusCode, expected := range test.RetryTests(retryCodes) {
		actual, _ := notifier.retrier.Check(statusCode, nil)
		require.Equal(t, expected, actual, "retryv1 - error on status %d", statusCode)
	}
}

func TestPagerDutyRetryV2(t *testing.T) {
	notifier, err := New(
		&Config{
			RoutingKey: receivers.Secret("01234567890123456789012345678901"),
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	retryCodes := append(test.DefaultRetryCodes(), http.StatusTooManyRequests)
	for statusCode, expected := range test.RetryTests(retryCodes) {
		actual, _ := notifier.retrier.Check(statusCode, nil)
		require.Equal(t, expected, actual, "retryv2 - error on status %d", statusCode)
	}
}

func TestPagerDutyRedactedURLV1(t *testing.T) {
	ctx, u, fn := test.GetContextWithCancelingURL()
	defer fn()

	key := "01234567890123456789012345678901"
	notifier, err := New(
		&Config{
			ServiceKey: receivers.Secret(key),
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)
	notifier.apiV1 = u.String()

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, key)
}

func TestPagerDutyRedactedURLV2(t *testing.T) {
	ctx, u, fn := test.GetContextWithCancelingURL()
	defer fn()

	key := "01234567890123456789012345678901"
	notifier, err := New(
		&Config{
			URL:        &receivers.URL{URL: u},
			RoutingKey: receivers.Secret(key),
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, key)
}

func TestPagerDutyV1ServiceKeyFromFile(t *testing.T) {
	key := "01234567890123456789012345678901"
	f, err := os.CreateTemp("", "pagerduty_test")
	require.NoError(t, err, "creating temp file failed")
	_, err = f.WriteString(key)
	require.NoError(t, err, "writing to temp file failed")

	ctx, u, fn := test.GetContextWithCancelingURL()
	defer fn()

	notifier, err := New(
		&Config{
			ServiceKeyFile: f.Name(),
			HTTPConfig:     &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)
	notifier.apiV1 = u.String()

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, key)
}

func TestPagerDutyV2RoutingKeyFromFile(t *testing.T) {
	key := "01234567890123456789012345678901"
	f, err := os.CreateTemp("", "pagerduty_test")
	require.NoError(t, err, "creating temp file failed")
	_, err = f.WriteString(key)
	require.NoError(t, err, "writing to temp file failed")

	ctx, u, fn := test.GetContextWithCancelingURL()
	defer fn()

	notifier, err := New(
		&Config{
			URL:            &receivers.URL{URL: u},
			RoutingKeyFile: f.Name(),
			HTTPConfig:     &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, key)
}

func TestPagerDutyTemplating(t *testing.T) {
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
				RoutingKey: receivers.Secret("01234567890123456789012345678901"),
				Images: []PagerdutyImage{
					{
						Src:  "{{ .Status }}",
						Alt:  "{{ .Status }}",
						Href: "{{ .Status }}",
					},
				},
				Links: []PagerdutyLink{
					{
						Href: "{{ .Status }}",
						Text: "{{ .Status }}",
					},
				},
				Details: map[string]string{
					"firing":       `{{ template "pagerduty.default.instances" .Alerts.Firing }}`,
					"resolved":     `{{ template "pagerduty.default.instances" .Alerts.Resolved }}`,
					"num_firing":   `{{ .Alerts.Firing | len }}`,
					"num_resolved": `{{ .Alerts.Resolved | len }}`,
				},
			},
		},
		{
			title: "details with templating errors",
			cfg: &Config{
				RoutingKey: receivers.Secret("01234567890123456789012345678901"),
				Details: map[string]string{
					"firing":       `{{ template "pagerduty.default.instances" .Alerts.Firing`,
					"resolved":     `{{ template "pagerduty.default.instances" .Alerts.Resolved }}`,
					"num_firing":   `{{ .Alerts.Firing | len }}`,
					"num_resolved": `{{ .Alerts.Resolved | len }}`,
				},
			},
			errMsg: "failed to template",
		},
		{
			title: "v2 message with templating errors",
			cfg: &Config{
				RoutingKey: receivers.Secret("01234567890123456789012345678901"),
				Severity:   "{{ ",
			},
			errMsg: "failed to template",
		},
		{
			title: "v1 message with templating errors",
			cfg: &Config{
				ServiceKey: receivers.Secret("01234567890123456789012345678901"),
				Client:     "{{ ",
			},
			errMsg: "failed to template",
		},
		{
			title: "routing key cannot be empty",
			cfg: &Config{
				RoutingKey: receivers.Secret(`{{ "" }}`),
			},
			errMsg: "routing key cannot be empty",
		},
		{
			title: "service_key cannot be empty",
			cfg: &Config{
				ServiceKey: receivers.Secret(`{{ "" }}`),
			},
			errMsg: "service key cannot be empty",
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			tc.cfg.URL = &receivers.URL{URL: u}
			tc.cfg.HTTPConfig = &httpcfg.HTTPClientConfig{}
			pd, err := New(tc.cfg, test.CreateTmpl(t), log.NewNopLogger())
			require.NoError(t, err)
			if pd.apiV1 != "" {
				pd.apiV1 = u.String()
			}

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

func TestErrDetails(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   io.Reader

		exp string
	}{
		{
			status: http.StatusBadRequest,
			body: bytes.NewBuffer([]byte(
				`{"status":"invalid event","message":"Event object is invalid","errors":["Length of 'routing_key' is incorrect (should be 32 characters)"]}`,
			)),

			exp: "Length of 'routing_key' is incorrect",
		},
		{
			status: http.StatusBadRequest,
			body:   bytes.NewBuffer([]byte(`{"status"}`)),

			exp: "",
		},
		{
			status: http.StatusBadRequest,

			exp: "",
		},
		{
			status: http.StatusTooManyRequests,

			exp: "",
		},
	} {
		tc := tc
		t.Run("", func(t *testing.T) {
			err := errDetails(tc.status, tc.body)
			require.Contains(t, err, tc.exp)
		})
	}
}

func TestEventSizeEnforcement(t *testing.T) {
	bigDetails := map[string]string{
		"firing": strings.Repeat("a", 513000),
	}

	// V1 Messages
	msgV1 := &pagerDutyMessage{
		ServiceKey: "01234567890123456789012345678901",
		EventType:  "trigger",
		Details:    bigDetails,
	}

	notifierV1, err := New(
		&Config{
			ServiceKey: receivers.Secret("01234567890123456789012345678901"),
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	encodedV1, err := notifierV1.encodeMessage(msgV1)
	require.NoError(t, err)
	require.Contains(t, encodedV1.String(), `"details":{"error":"Custom details have been removed because the original event exceeds the maximum size of 512KB"}`)

	// V2 Messages
	msgV2 := &pagerDutyMessage{
		RoutingKey:  "01234567890123456789012345678901",
		EventAction: "trigger",
		Payload: &pagerDutyPayload{
			CustomDetails: bigDetails,
		},
	}

	notifierV2, err := New(
		&Config{
			RoutingKey: receivers.Secret("01234567890123456789012345678901"),
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	encodedV2, err := notifierV2.encodeMessage(msgV2)
	require.NoError(t, err)
	require.Contains(t, encodedV2.String(), `"custom_details":{"error":"Custom details have been removed because the original event exceeds the maximum size of 512KB"}`)
}

func TestPagerDutyEmptySrcHref(t *testing.T) {
	type pagerDutyEvent struct {
		RoutingKey  string           `json:"routing_key"`
		EventAction string           `json:"event_action"`
		DedupKey    string           `json:"dedup_key"`
		Payload     pagerDutyPayload `json:"payload"`
		Images      []pagerDutyImage
		Links       []pagerDutyLink
	}

	images := []PagerdutyImage{
		{
			Src:  "",
			Alt:  "Empty src",
			Href: "https://example.com/",
		},
		{
			Src:  "https://example.com/cat.jpg",
			Alt:  "Empty href",
			Href: "",
		},
		{
			Src:  "https://example.com/cat.jpg",
			Alt:  "",
			Href: "https://example.com/",
		},
	}

	links := []PagerdutyLink{
		{
			Href: "",
			Text: "Empty href",
		},
		{
			Href: "https://example.com/",
			Text: "",
		},
	}

	expectedImages := make([]pagerDutyImage, 0, len(images))
	for _, image := range images {
		if image.Src == "" {
			continue
		}
		expectedImages = append(expectedImages, pagerDutyImage(image))
	}

	expectedLinks := make([]pagerDutyLink, 0, len(links))
	for _, link := range links {
		if link.Href == "" {
			continue
		}
		expectedLinks = append(expectedLinks, pagerDutyLink{
			HRef: link.Href,
			Text: link.Text,
		})
	}

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			decoder := json.NewDecoder(r.Body)
			var event pagerDutyEvent
			if err := decoder.Decode(&event); err != nil {
				panic(err)
			}

			if event.RoutingKey == "" || event.EventAction == "" {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			for _, image := range event.Images {
				if image.Src == "" {
					http.Error(w, "Event object is invalid: 'image src' is missing or blank", http.StatusBadRequest)
					return
				}
			}

			for _, link := range event.Links {
				if link.HRef == "" {
					http.Error(w, "Event object is invalid: 'link href' is missing or blank", http.StatusBadRequest)
					return
				}
			}

			require.Equal(t, expectedImages, event.Images)
			require.Equal(t, expectedLinks, event.Links)
		},
	))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	pagerDutyConfig := Config{
		HTTPConfig: &httpcfg.HTTPClientConfig{},
		RoutingKey: receivers.Secret("01234567890123456789012345678901"),
		URL:        &receivers.URL{URL: u},
		Images:     images,
		Links:      links,
	}

	pagerDuty, err := New(&pagerDutyConfig, test.CreateTmpl(t), log.NewNopLogger())
	require.NoError(t, err)

	ctx := context.Background()
	ctx = notify.WithGroupKey(ctx, "1")

	_, err = pagerDuty.Notify(ctx, []*types.Alert{
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
	require.NoError(t, err)
}
