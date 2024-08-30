package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/url"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

// Test certificates from https://github.com/golang/go/blob/4f852b9734249c063928b34a02dd689e03a8ab2c/src/crypto/tls/tls_test.go#L34
const (
	testRsaCertPem = `-----BEGIN CERTIFICATE-----
MIIB0zCCAX2gAwIBAgIJAI/M7BYjwB+uMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTIwOTEyMjE1MjAyWhcNMTUwOTEyMjE1MjAyWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANLJ
hPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wok/4xIA+ui35/MmNa
rtNuC+BdZ1tMuVCPFZcCAwEAAaNQME4wHQYDVR0OBBYEFJvKs8RfJaXTH08W+SGv
zQyKn0H8MB8GA1UdIwQYMBaAFJvKs8RfJaXTH08W+SGvzQyKn0H8MAwGA1UdEwQF
MAMBAf8wDQYJKoZIhvcNAQEFBQADQQBJlffJHybjDGxRMqaRmDhX0+6v02TUKZsW
r5QuVbpQhH6u+0UgcW0jp9QwpxoPTLTWGXEWBBBurxFwiCBhkQ+V
-----END CERTIFICATE-----`

	testRsaKeyPem = `-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBANLJhPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wo
k/4xIA+ui35/MmNartNuC+BdZ1tMuVCPFZcCAwEAAQJAEJ2N+zsR0Xn8/Q6twa4G
6OB1M1WO+k+ztnX/1SvNeWu8D6GImtupLTYgjZcHufykj09jiHmjHx8u8ZZB/o1N
MQIhAPW+eyZo7ay3lMz1V01WVjNKK9QSn1MJlb06h/LuYv9FAiEA25WPedKgVyCW
SmUwbPw8fnTcpqDWE3yTO3vKcebqMSsCIBF3UmVue8YU3jybC3NxuXq3wNm34R8T
xVLHwDXh/6NJAiEAl2oHGGLz64BuAfjKrqwz7qMYr9HCLIe/YsoWq/olzScCIQDi
D2lWusoe2/nEqfDVVWGWlyJ7yOmqaVm/iNUN9B2N2g==
-----END RSA PRIVATE KEY-----`
)

type mockMQTTClient struct {
	mock.Mock
	publishedMessages []message
	clientID          string
	brokerURL         string
	username          string
	password          string
	tlsCfg            *tls.Config
}

func (m *mockMQTTClient) Publish(ctx context.Context, message message) error {
	args := m.Called(ctx, message)

	m.publishedMessages = append(m.publishedMessages, message)

	return args.Error(0)
}

func (m *mockMQTTClient) Connect(ctx context.Context, brokerURL, clientID, username, password string, tlsCfg *tls.Config) error {
	args := m.Called(ctx, brokerURL, clientID, username, password, tlsCfg)

	m.clientID = clientID
	m.brokerURL = brokerURL
	m.username = username
	m.password = password
	m.tlsCfg = tlsCfg

	return args.Error(0)
}

// revive:disable:unused-parameter
func (m *mockMQTTClient) Disconnect(ctx context.Context) error {
	m.Called(ctx)

	return nil
}

// revive:enable:unused-parameter

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)
	require.NotNil(t, tmpl)

	externalURL, err := url.Parse("http://localhost/base")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMessage  message
		expUsername string
		expClientID string
		expPassword string
		expError    error
	}{
		{
			name: "A single alert with the default template in JSON",
			settings: Config{
				Topic:         "alert1",
				Message:       templates.DefaultMessageEmbed,
				MessageFormat: MessageFormatJSON,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "a URL",
					},
				},
			},
			expMessage: message{
				topic:   "alert1",
				payload: []byte("{\"receiver\":\"\",\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"annotations\":{\"ann1\":\"annv1\"},\"startsAt\":\"0001-01-01T00:00:00Z\",\"endsAt\":\"0001-01-01T00:00:00Z\",\"generatorURL\":\"a URL\",\"fingerprint\":\"fac0861a85de433a\",\"silenceURL\":\"http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\",\"dashboardURL\":\"http://localhost/base/d/abcd\",\"panelURL\":\"http://localhost/base/d/abcd?viewPanel=efgh\",\"values\":null,\"valueString\":\"\"}],\"groupLabels\":{\"alertname\":\"\"},\"commonLabels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"commonAnnotations\":{\"ann1\":\"annv1\"},\"externalURL\":\"http://localhost/base\",\"version\":\"1\",\"groupKey\":\"alertname\",\"message\":\"**Firing**\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val1\\nAnnotations:\\n - ann1 = annv1\\nSource: a URL\\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\\nDashboard: http://localhost/base/d/abcd\\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\\n\"}"),
				retain:  false,
				qos:     0,
			},
			expError: nil,
		},
		{
			name: "A single alert with the default template in JSON with retain and QoS",
			settings: Config{
				Topic:         "alert1",
				Message:       templates.DefaultMessageEmbed,
				MessageFormat: MessageFormatJSON,
				Retain:        true,
				QoS:           "1",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "a URL",
					},
				},
			},
			expMessage: message{
				topic:   "alert1",
				payload: []byte("{\"receiver\":\"\",\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"annotations\":{\"ann1\":\"annv1\"},\"startsAt\":\"0001-01-01T00:00:00Z\",\"endsAt\":\"0001-01-01T00:00:00Z\",\"generatorURL\":\"a URL\",\"fingerprint\":\"fac0861a85de433a\",\"silenceURL\":\"http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\",\"dashboardURL\":\"http://localhost/base/d/abcd\",\"panelURL\":\"http://localhost/base/d/abcd?viewPanel=efgh\",\"values\":null,\"valueString\":\"\"}],\"groupLabels\":{\"alertname\":\"\"},\"commonLabels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"commonAnnotations\":{\"ann1\":\"annv1\"},\"externalURL\":\"http://localhost/base\",\"version\":\"1\",\"groupKey\":\"alertname\",\"message\":\"**Firing**\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val1\\nAnnotations:\\n - ann1 = annv1\\nSource: a URL\\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\\nDashboard: http://localhost/base/d/abcd\\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\\n\"}"),
				retain:  true,
				qos:     1,
			},
			expError: nil,
		},
		{
			name: "A single alert with default template in plain text",
			settings: Config{
				Topic:         "alert1",
				Message:       templates.DefaultMessageEmbed,
				MessageFormat: MessageFormatText,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "a URL",
					},
				},
			},
			expMessage: message{
				topic:   "alert1",
				payload: []byte("**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/base/d/abcd\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\n"),
			},
			expError: nil,
		},
		{
			name: "Multiple alerts with default template",
			settings: Config{
				Topic:         "grafana/alerts",
				Message:       templates.DefaultMessageEmbed,
				MessageFormat: MessageFormatJSON,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "URL 1",
					},
				},
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert2", "lbl1": "val2"},
						Annotations:  model.LabelSet{"ann1": "annv2", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "URL 2",
					},
				},
			},
			expMessage: message{
				topic:   "grafana/alerts",
				payload: []byte("{\"receiver\":\"\",\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"annotations\":{\"ann1\":\"annv1\"},\"startsAt\":\"0001-01-01T00:00:00Z\",\"endsAt\":\"0001-01-01T00:00:00Z\",\"generatorURL\":\"URL 1\",\"fingerprint\":\"fac0861a85de433a\",\"silenceURL\":\"http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\",\"dashboardURL\":\"http://localhost/base/d/abcd\",\"panelURL\":\"http://localhost/base/d/abcd?viewPanel=efgh\",\"values\":null,\"valueString\":\"\"},{\"status\":\"firing\",\"labels\":{\"alertname\":\"alert2\",\"lbl1\":\"val2\"},\"annotations\":{\"ann1\":\"annv2\"},\"startsAt\":\"0001-01-01T00:00:00Z\",\"endsAt\":\"0001-01-01T00:00:00Z\",\"generatorURL\":\"URL 2\",\"fingerprint\":\"f6cbec330d95e626\",\"silenceURL\":\"http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert2\\u0026matcher=lbl1%3Dval2\",\"dashboardURL\":\"http://localhost/base/d/abcd\",\"panelURL\":\"http://localhost/base/d/abcd?viewPanel=efgh\",\"values\":null,\"valueString\":\"\"}],\"groupLabels\":{\"alertname\":\"\"},\"commonLabels\":{},\"commonAnnotations\":{},\"externalURL\":\"http://localhost/base\",\"version\":\"1\",\"groupKey\":\"alertname\",\"message\":\"**Firing**\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val1\\nAnnotations:\\n - ann1 = annv1\\nSource: URL 1\\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\\nDashboard: http://localhost/base/d/abcd\\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert2\\n - lbl1 = val2\\nAnnotations:\\n - ann1 = annv2\\nSource: URL 2\\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert2\\u0026matcher=lbl1%3Dval2\\nDashboard: http://localhost/base/d/abcd\\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\\n\"}"),
			},
			expError: nil,
		},
		{
			name: "Multiple alerts with custom template in plain text",
			settings: Config{
				Topic:         "grafana/alerts",
				Message:       `count: {{len .Alerts.Firing}}, firing: {{ template "__text_alert_list" .Alerts.Firing }}}`,
				MessageFormat: MessageFormatText,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "URL 1",
					},
				},
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert2", "lbl1": "val2"},
						Annotations:  model.LabelSet{"ann1": "annv2", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "URL 2",
					},
				},
			},
			expMessage: message{
				topic:   "grafana/alerts",
				payload: []byte("count: 2, firing: \nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: URL 1\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/base/d/abcd\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\n\nValue: [no value]\nLabels:\n - alertname = alert2\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: URL 2\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert2&matcher=lbl1%3Dval2\nDashboard: http://localhost/base/d/abcd\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\n}"),
			},
			expError: nil,
		},
		{
			name: "Multiple alerts with custom template in JSON",
			settings: Config{
				Topic:         "grafana/alerts",
				Message:       `count: {{len .Alerts.Firing}}, firing: {{ template "__text_alert_list" .Alerts.Firing }}}`,
				MessageFormat: MessageFormatJSON,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "URL 1",
					},
				},
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert2", "lbl1": "val2"},
						Annotations:  model.LabelSet{"ann1": "annv2", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "URL 2",
					},
				},
			},
			expMessage: message{
				topic:   "grafana/alerts",
				payload: []byte("{\"receiver\":\"\",\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"annotations\":{\"ann1\":\"annv1\"},\"startsAt\":\"0001-01-01T00:00:00Z\",\"endsAt\":\"0001-01-01T00:00:00Z\",\"generatorURL\":\"URL 1\",\"fingerprint\":\"fac0861a85de433a\",\"silenceURL\":\"http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\",\"dashboardURL\":\"http://localhost/base/d/abcd\",\"panelURL\":\"http://localhost/base/d/abcd?viewPanel=efgh\",\"values\":null,\"valueString\":\"\"},{\"status\":\"firing\",\"labels\":{\"alertname\":\"alert2\",\"lbl1\":\"val2\"},\"annotations\":{\"ann1\":\"annv2\"},\"startsAt\":\"0001-01-01T00:00:00Z\",\"endsAt\":\"0001-01-01T00:00:00Z\",\"generatorURL\":\"URL 2\",\"fingerprint\":\"f6cbec330d95e626\",\"silenceURL\":\"http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert2\\u0026matcher=lbl1%3Dval2\",\"dashboardURL\":\"http://localhost/base/d/abcd\",\"panelURL\":\"http://localhost/base/d/abcd?viewPanel=efgh\",\"values\":null,\"valueString\":\"\"}],\"groupLabels\":{\"alertname\":\"\"},\"commonLabels\":{},\"commonAnnotations\":{},\"externalURL\":\"http://localhost/base\",\"version\":\"1\",\"groupKey\":\"alertname\",\"message\":\"count: 2, firing: \\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val1\\nAnnotations:\\n - ann1 = annv1\\nSource: URL 1\\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\\nDashboard: http://localhost/base/d/abcd\\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert2\\n - lbl1 = val2\\nAnnotations:\\n - ann1 = annv2\\nSource: URL 2\\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert2\\u0026matcher=lbl1%3Dval2\\nDashboard: http://localhost/base/d/abcd\\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\\n}\"}"),
			},
			expError: nil,
		},
		{
			name: "With username and password",
			settings: Config{
				Topic:         "alert1",
				Message:       templates.DefaultMessageEmbed,
				MessageFormat: MessageFormatJSON,
				Username:      "user",
				Password:      "pass",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "a URL",
					},
				},
			},
			expMessage: message{
				topic:   "alert1",
				payload: []byte("{\"receiver\":\"\",\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"annotations\":{\"ann1\":\"annv1\"},\"startsAt\":\"0001-01-01T00:00:00Z\",\"endsAt\":\"0001-01-01T00:00:00Z\",\"generatorURL\":\"a URL\",\"fingerprint\":\"fac0861a85de433a\",\"silenceURL\":\"http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\",\"dashboardURL\":\"http://localhost/base/d/abcd\",\"panelURL\":\"http://localhost/base/d/abcd?viewPanel=efgh\",\"values\":null,\"valueString\":\"\"}],\"groupLabels\":{\"alertname\":\"\"},\"commonLabels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"commonAnnotations\":{\"ann1\":\"annv1\"},\"externalURL\":\"http://localhost/base\",\"version\":\"1\",\"groupKey\":\"alertname\",\"message\":\"**Firing**\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val1\\nAnnotations:\\n - ann1 = annv1\\nSource: a URL\\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\\nDashboard: http://localhost/base/d/abcd\\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\\n\"}"),
			},
			expUsername: "user",
			expPassword: "pass",
			expError:    nil,
		},
		{
			name: "With TLS config",
			settings: Config{
				Topic:         "alert1",
				Message:       templates.DefaultMessageEmbed,
				MessageFormat: MessageFormatJSON,
				TLSConfig: &receivers.TLSConfig{
					InsecureSkipVerify: true,
					CACertificate:      testRsaCertPem,
					ClientCertificate:  testRsaCertPem,
					ClientKey:          testRsaKeyPem,
				},
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
						GeneratorURL: "a URL",
					},
				},
			},
			expMessage: message{
				topic:   "alert1",
				payload: []byte("{\"receiver\":\"\",\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"annotations\":{\"ann1\":\"annv1\"},\"startsAt\":\"0001-01-01T00:00:00Z\",\"endsAt\":\"0001-01-01T00:00:00Z\",\"generatorURL\":\"a URL\",\"fingerprint\":\"fac0861a85de433a\",\"silenceURL\":\"http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\",\"dashboardURL\":\"http://localhost/base/d/abcd\",\"panelURL\":\"http://localhost/base/d/abcd?viewPanel=efgh\",\"values\":null,\"valueString\":\"\"}],\"groupLabels\":{\"alertname\":\"\"},\"commonLabels\":{\"alertname\":\"alert1\",\"lbl1\":\"val1\"},\"commonAnnotations\":{\"ann1\":\"annv1\"},\"externalURL\":\"http://localhost/base\",\"version\":\"1\",\"groupKey\":\"alertname\",\"message\":\"**Firing**\\n\\nValue: [no value]\\nLabels:\\n - alertname = alert1\\n - lbl1 = val1\\nAnnotations:\\n - ann1 = annv1\\nSource: a URL\\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana\\u0026matcher=alertname%3Dalert1\\u0026matcher=lbl1%3Dval1\\nDashboard: http://localhost/base/d/abcd\\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\\n\"}"),
			},
			expError: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockMQTTClient := new(mockMQTTClient)
			mockMQTTClient.On(
				"Connect",
				mock.Anything,
				c.settings.BrokerURL,
				c.settings.ClientID,
				c.settings.Username,
				c.settings.Password,
				mock.Anything,
			).Return(nil)
			mockMQTTClient.On("Disconnect", mock.Anything).Return(nil)
			mockMQTTClient.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			n := &Notifier{
				Base: &receivers.Base{
					Name:                  "",
					Type:                  "",
					UID:                   "",
					DisableResolveMessage: false,
				},
				log:      &logging.FakeLogger{},
				tmpl:     tmpl,
				settings: c.settings,
				client:   mockMQTTClient,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			recoverableErr, err := n.Notify(ctx, c.alerts...)
			if c.expError != nil {
				assert.False(t, recoverableErr)
				require.Error(t, err)
				require.Equal(t, c.expError.Error(), err.Error())
				return
			}
			require.NoError(t, err)

			require.Equal(t, 1, len(mockMQTTClient.publishedMessages))
			require.Equal(t, c.expMessage, mockMQTTClient.publishedMessages[0])
			require.Equal(t, c.expUsername, mockMQTTClient.username)
			require.Equal(t, c.expPassword, mockMQTTClient.password)
			require.Equal(t, c.settings.ClientID, mockMQTTClient.clientID)
			require.Equal(t, c.settings.BrokerURL, mockMQTTClient.brokerURL)

			if c.settings.TLSConfig == nil {
				require.Nil(t, mockMQTTClient.tlsCfg)
			} else {
				require.NotNil(t, mockMQTTClient.tlsCfg)
				require.Equal(t, mockMQTTClient.tlsCfg.InsecureSkipVerify, c.settings.TLSConfig.InsecureSkipVerify)

				// Check if the client certificate and key are set correctly.
				if c.settings.TLSConfig.ClientCertificate != "" && c.settings.TLSConfig.ClientKey != "" {
					clientCert, err := tls.X509KeyPair([]byte(c.settings.TLSConfig.ClientCertificate), []byte(c.settings.TLSConfig.ClientKey))
					require.NoError(t, err)
					require.Equal(t, clientCert, mockMQTTClient.tlsCfg.Certificates[0])
				}

				// Check if the CA certificate is set correctly.
				if c.settings.TLSConfig.CACertificate != "" {
					expectedRootCAs := x509.NewCertPool()
					expectedRootCAs.AppendCertsFromPEM([]byte(c.settings.TLSConfig.CACertificate))
					require.True(t, mockMQTTClient.tlsCfg.RootCAs.Equal(expectedRootCAs))
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	tmpl := templates.ForTests(t)
	require.NotNil(t, tmpl)

	cases := []struct {
		name string
		cfg  Config
	}{
		{
			name: "Simple configuration",
			cfg: Config{
				Topic:     "alerts",
				Message:   templates.DefaultMessageEmbed,
				Username:  "user",
				Password:  "pass",
				BrokerURL: "tcp://127.0.0.1:1883",
				ClientID:  "test-grafana",
			},
		},
		{
			name: "Configuration with insecureSkipVerify",
			cfg: Config{
				Topic:     "alerts",
				Message:   templates.DefaultMessageEmbed,
				Username:  "user",
				Password:  "pass",
				BrokerURL: "tcp://127.0.0.1:1883",
				ClientID:  "test-grafana",
				TLSConfig: &receivers.TLSConfig{
					InsecureSkipVerify: true,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(mockMQTTClient)
			logger := &logging.FakeLogger{}

			n := New(
				tc.cfg,
				receivers.Metadata{},
				tmpl,
				logger,
				mockClient,
			)

			require.NotNil(t, n)
			require.NotNil(t, n.Base)
			require.Equal(t, n.log, logger)
			require.Equal(t, n.settings, tc.cfg)
			require.Equal(t, n.client, mockClient)
			require.Equal(t, tmpl, n.tmpl)
		})
	}
}
