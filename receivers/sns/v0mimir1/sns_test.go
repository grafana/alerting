// Copyright 2021 Prometheus Team
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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/notify/test"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	httpcfg "github.com/grafana/alerting/http/v0mimir"
)

// isolateAWSEnv keeps the developer's AWS environment and shared config files
// from leaking into the SDK's default config resolution.
func isolateAWSEnv(t *testing.T) {
	t.Helper()
	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv("AWS_CONFIG_FILE", missing)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", missing)
	for _, k := range []string{"AWS_PROFILE", "AWS_REGION", "AWS_DEFAULT_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_ENDPOINT_URL", "AWS_ENDPOINT_URL_SNS", "AWS_CA_BUNDLE"} {
		t.Setenv(k, "")
	}
}

func newTestNotifier(t *testing.T, apiURL string) *Notifier {
	t.Helper()
	cfg := DefaultConfig
	cfg.HTTPConfig = &httpcfg.HTTPClientConfig{}
	cfg.APIUrl = apiURL
	cfg.TopicARN = "arn:aws:sns:us-east-1:123456789012:alerts"
	cfg.Sigv4 = SigV4Config{Region: "us-east-1", AccessKey: "AKIDTEST", SecretKey: "secret"}
	n, err := New(&cfg, test.CreateTmpl(t), log.NewNopLogger())
	require.NoError(t, err)
	return n
}

func notifyTestAlert(t *testing.T, n *Notifier) (bool, error) {
	t.Helper()
	ctx := notify.WithGroupKey(context.Background(), "1")
	return n.Notify(ctx, &types.Alert{Alert: model.Alert{
		Labels:   model.LabelSet{"alertname": "test"},
		StartsAt: time.Now(),
		EndsAt:   time.Now().Add(time.Hour),
	}})
}

func TestNotify_Publish(t *testing.T) {
	isolateAWSEnv(t)
	var gotForm url.Values
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(body))
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/xml")
		_, _ = io.WriteString(w, `<PublishResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/"><PublishResult><MessageId>msg-1</MessageId></PublishResult><ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata></PublishResponse>`)
	}))
	defer srv.Close()

	retry, err := notifyTestAlert(t, newTestNotifier(t, srv.URL))
	require.NoError(t, err)
	require.False(t, retry)
	require.Equal(t, "Publish", gotForm.Get("Action"))
	require.Equal(t, "arn:aws:sns:us-east-1:123456789012:alerts", gotForm.Get("TopicArn"))
	require.Contains(t, gotAuth, "Credential=AKIDTEST/")
	require.Contains(t, gotAuth, "/us-east-1/sns/aws4_request")
}

func TestNotify_PublishClientError(t *testing.T) {
	isolateAWSEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `<ErrorResponse><Error><Type>Sender</Type><Code>InvalidParameter</Code><Message>Invalid parameter: TopicArn</Message></Error><RequestId>req-1</RequestId></ErrorResponse>`)
	}))
	defer srv.Close()

	retry, err := notifyTestAlert(t, newTestNotifier(t, srv.URL))
	require.False(t, retry)
	require.ErrorContains(t, err, "Invalid parameter: TopicArn")
	var reasonErr *notify.ErrorWithReason
	require.True(t, errors.As(err, &reasonErr))
	require.Equal(t, notify.ClientErrorReason, reasonErr.Reason)
}

func TestCreateSNSClient_RequiresRegion(t *testing.T) {
	isolateAWSEnv(t)
	n := newTestNotifier(t, "")
	n.conf.Sigv4.Region = ""
	_, err := n.createSNSClient(context.Background(), func(s string) string { return s })
	require.ErrorContains(t, err, "region not configured")
}

func TestWithDefaultScheme(t *testing.T) {
	require.Equal(t, "https://sns.us-east-1.amazonaws.com", withDefaultScheme("sns.us-east-1.amazonaws.com"))
	require.Equal(t, "http://localhost:4566", withDefaultScheme("http://localhost:4566"))
	require.True(t, strings.HasPrefix(withDefaultScheme("https://example.com"), "https://example.com"))
}

func TestValidateAndTruncateMessage(t *testing.T) {
	sBuff := make([]byte, 257*1024)
	for i := range sBuff {
		sBuff[i] = byte(33)
	}
	truncatedMessage, isTruncated, err := validateAndTruncateMessage(string(sBuff), 256*1024)
	require.True(t, isTruncated)
	require.NoError(t, err)
	require.NotEqual(t, sBuff, truncatedMessage)
	require.Len(t, truncatedMessage, 256*1024)

	sBuff = make([]byte, 100)
	for i := range sBuff {
		sBuff[i] = byte(33)
	}
	truncatedMessage, isTruncated, err = validateAndTruncateMessage(string(sBuff), 100)
	require.False(t, isTruncated)
	require.NoError(t, err)
	require.Equal(t, string(sBuff), truncatedMessage)

	invalidUtf8String := "\xc3\x28"
	_, _, err = validateAndTruncateMessage(invalidUtf8String, 100)
	require.Error(t, err)
}
