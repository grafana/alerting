// Copyright 2022 Prometheus Team
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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	commoncfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/test"
	"github.com/prometheus/alertmanager/types"
)

func TestTelegramRetry(t *testing.T) {
	// Fake url for testing purposes
	fakeURL := config.URL{
		URL: &url.URL{
			Scheme: "https",
			Host:   "FAKE_API",
		},
	}
	notifier, err := New(
		&Config{
			HTTPConfig: &commoncfg.HTTPClientConfig{},
			APIUrl:     &fakeURL,
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	for statusCode, expected := range test.RetryTests(test.DefaultRetryCodes()) {
		actual, _ := notifier.retrier.Check(statusCode, nil)
		require.Equal(t, expected, actual, "error on status %d", statusCode)
	}
}

func TestTelegramNotify(t *testing.T) {
	token := "secret"

	fileWithToken, err := os.CreateTemp("", "telegram-bot-token")
	require.NoError(t, err, "creating temp file failed")
	_, err = fileWithToken.WriteString(token)
	require.NoError(t, err, "writing to temp file failed")

	for _, tc := range []struct {
		name    string
		cfg     Config
		expText string
	}{
		{
			name: "No escaping by default",
			cfg: Config{
				Message:    "<code>x < y</code>",
				HTTPConfig: &commoncfg.HTTPClientConfig{},
				BotToken:   config.Secret(token),
			},
			expText: "<code>x < y</code>",
		},
		{
			name: "Characters escaped in HTML mode",
			cfg: Config{
				ParseMode:  "HTML",
				Message:    "<code>x < y</code>",
				HTTPConfig: &commoncfg.HTTPClientConfig{},
				BotToken:   config.Secret(token),
			},
			expText: "<code>x &lt; y</code>",
		},
		{
			name: "Bot token from file",
			cfg: Config{
				Message:      "test",
				HTTPConfig:   &commoncfg.HTTPClientConfig{},
				BotTokenFile: fileWithToken.Name(),
			},
			expText: "test",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/bot"+token+"/sendMessage", r.URL.Path)
				var err error
				out, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				w.Write([]byte(`{"ok":true,"result":{"chat":{}}}`))
			}))
			defer srv.Close()
			u, _ := url.Parse(srv.URL)

			tc.cfg.APIUrl = &config.URL{URL: u}

			notifier, err := New(&tc.cfg, test.CreateTmpl(t), log.NewNopLogger())
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctx = notify.WithGroupKey(ctx, "1")

			retry, err := notifier.Notify(ctx, []*types.Alert{
				{
					Alert: model.Alert{
						Labels: model.LabelSet{
							"lbl1": "val1",
							"lbl3": "val3",
						},
						StartsAt: time.Now(),
						EndsAt:   time.Now().Add(time.Hour),
					},
				},
			}...)

			require.False(t, retry)
			require.NoError(t, err)

			req := map[string]string{}
			err = json.Unmarshal(out, &req)
			require.NoError(t, err)
			require.Equal(t, tc.expText, req["text"])
		})
	}
}
