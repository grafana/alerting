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
	"net/http"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/test"
	"github.com/prometheus/alertmanager/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
	"github.com/grafana/alerting/receivers"
)

func TestPushoverRetry(t *testing.T) {
	notifier, err := New(
		&Config{
			HTTPConfig: &httpcfg.HTTPClientConfig{},
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

func TestPushoverRedactedURL(t *testing.T) {
	ctx, u, fn := test.GetContextWithCancelingURL()
	defer fn()

	key, token := "user_key", "token"
	notifier, err := New(
		&Config{
			UserKey:    receivers.Secret(key),
			Token:      receivers.Secret(token),
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)
	notifier.apiURL = u.String()

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, key, token)
}

func TestPushoverMonospaceParameter(t *testing.T) {
	_, apiURL, fn := test.GetContextWithCancelingURL(func(w http.ResponseWriter, r *http.Request) {
		if !assert.NoError(t, r.ParseForm()) {
			return
		}
		assert.Equal(t, "1", r.FormValue("monospace"), `expected monospace parameter to be set to "1"`)
	})
	defer fn()

	notifier, err := New(
		&Config{
			UserKey:    "user_key",
			Token:      "token",
			Monospace:  true,
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)
	notifier.apiURL = apiURL.String()

	_, err = notifier.Notify(notify.WithGroupKey(t.Context(), "1"), &types.Alert{})
	require.NoError(t, err)
}
