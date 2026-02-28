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
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify/test"

	httpcfg "github.com/grafana/alerting/http/v0mimir1"
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

func TestPushoverReadingUserKeyFromFile(t *testing.T) {
	ctx, apiURL, fn := test.GetContextWithCancelingURL()
	defer fn()

	const userKey = "user key"
	f, err := os.CreateTemp("", "pushover_user_key")
	require.NoError(t, err, "creating temp file failed")
	_, err = f.WriteString(userKey)
	require.NoError(t, err, "writing to temp file failed")

	notifier, err := New(
		&Config{
			UserKeyFile: f.Name(),
			Token:       receivers.Secret("token"),
			HTTPConfig:  &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	notifier.apiURL = apiURL.String()
	require.NoError(t, err)

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, userKey)
}

func TestPushoverReadingTokenFromFile(t *testing.T) {
	ctx, apiURL, fn := test.GetContextWithCancelingURL()
	defer fn()

	const token = "token"
	f, err := os.CreateTemp("", "pushover_token")
	require.NoError(t, err, "creating temp file failed")
	_, err = f.WriteString(token)
	require.NoError(t, err, "writing to temp file failed")

	notifier, err := New(
		&Config{
			UserKey:    receivers.Secret("user key"),
			TokenFile:  f.Name(),
			HTTPConfig: &httpcfg.HTTPClientConfig{},
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	notifier.apiURL = apiURL.String()
	require.NoError(t, err)

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, token)
}
