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
	"fmt"
	"net/http"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify/test"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"

	httpcfg "github.com/grafana/alerting/http/v0mimir1"
)

func TestWechatRedactedURLOnInitialAuthentication(t *testing.T) {
	ctx, u, fn := test.GetContextWithCancelingURL()
	defer fn()

	secret := "secret_key"
	notifier, err := New(
		&Config{
			APIURL:     &receivers.URL{URL: u},
			HTTPConfig: &httpcfg.HTTPClientConfig{},
			CorpID:     "corpid",
			APISecret:  receivers.Secret(secret),
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, secret)
}

func TestWechatRedactedURLOnNotify(t *testing.T) {
	secret, token := "secret", "token"
	ctx, u, fn := test.GetContextWithCancelingURL(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"access_token":"%s"}`, token)
	})
	defer fn()

	notifier, err := New(
		&Config{
			APIURL:     &receivers.URL{URL: u},
			HTTPConfig: &httpcfg.HTTPClientConfig{},
			CorpID:     "corpid",
			APISecret:  receivers.Secret(secret),
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, secret, token)
}

func TestWechatMessageTypeSelector(t *testing.T) {
	secret, token := "secret", "token"
	ctx, u, fn := test.GetContextWithCancelingURL(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"access_token":"%s"}`, token)
	})
	defer fn()

	notifier, err := New(
		&Config{
			APIURL:      &receivers.URL{URL: u},
			HTTPConfig:  &httpcfg.HTTPClientConfig{},
			CorpID:      "corpid",
			APISecret:   receivers.Secret(secret),
			MessageType: "markdown",
		},
		test.CreateTmpl(t),
		log.NewNopLogger(),
	)
	require.NoError(t, err)

	test.AssertNotifyLeaksNoSecret(ctx, t, notifier, secret, token)
}
