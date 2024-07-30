package mqtt

import (
	"context"
	"net/url"
	"testing"

	mqttLib "github.com/eclipse/paho.mqtt.golang"
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

type mockMQTTClient struct {
	mock.Mock
	publishedMessages []*publishedMessage
}

type publishedMessage struct {
	topic   string
	message string
}

func (m *mockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqttLib.Token {
	args := m.Called(topic, qos, retained, payload)

	m.publishedMessages = append(m.publishedMessages, &publishedMessage{
		topic:   topic,
		message: payload.(string),
	})

	return args.Get(0).(mqttLib.Token)
}

func (m *mockMQTTClient) Connect() mqttLib.Token {
	args := m.Called()
	return args.Get(0).(mqttLib.Token)
}

// revive:disable:unused-parameter
func (m *mockMQTTClient) Disconnect(quiesce uint) {}

// revive:enable:unused-parameter

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)
	require.NotNil(t, tmpl)

	externalURL, err := url.Parse("http://localhost/base")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name       string
		settings   Config
		alerts     []*types.Alert
		expMessage *publishedMessage
		expError   error
	}{
		{
			name: "A single alert with default template",
			settings: Config{
				Topic:   "alert1",
				Message: templates.DefaultMessageEmbed,
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
			expMessage: &publishedMessage{
				topic:   "alert1",
				message: "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: a URL\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/base/d/abcd\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\n",
			},
			expError: nil,
		},
		{
			name: "Multiple alerts with default template",
			settings: Config{
				Topic:   "grafana/alerts",
				Message: templates.DefaultMessageEmbed,
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
			expMessage: &publishedMessage{
				topic:   "grafana/alerts",
				message: "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: URL 1\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/base/d/abcd\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\n\nValue: [no value]\nLabels:\n - alertname = alert2\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: URL 2\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert2&matcher=lbl1%3Dval2\nDashboard: http://localhost/base/d/abcd\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\n",
			},
			expError: nil,
		},
		{
			name: "Multiple alerts with custom template",
			settings: Config{
				Topic:   "grafana/alerts",
				Message: `count: {{len .Alerts.Firing}}, firing: {{ template "__text_alert_list" .Alerts.Firing }}}`,
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
			expMessage: &publishedMessage{
				topic:   "grafana/alerts",
				message: "count: 2, firing: \nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: URL 1\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/base/d/abcd\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\n\nValue: [no value]\nLabels:\n - alertname = alert2\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: URL 2\nSilence: http://localhost/base/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert2&matcher=lbl1%3Dval2\nDashboard: http://localhost/base/d/abcd\nPanel: http://localhost/base/d/abcd?viewPanel=efgh\n}",
			},
			expError: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockMQTTClient := new(mockMQTTClient)
			mockMQTTClient.On("Connect").Return(&mqttLib.DummyToken{})
			mockMQTTClient.On("Publish", mock.Anything, uint8(0), false, mock.Anything).Return(&mqttLib.DummyToken{})

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

			if c.expMessage != nil {
				require.Equal(t, 1, len(mockMQTTClient.publishedMessages))
				require.Equal(t, c.expMessage, mockMQTTClient.publishedMessages[0])
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
				Topic:              "alerts",
				Message:            templates.DefaultMessageEmbed,
				Username:           "user",
				Password:           "pass",
				BrokerURL:          "tcp://127.0.0.1:1883",
				ClientID:           "test-grafana",
				InsecureSkipVerify: true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calledWithOpts mqttLib.ClientOptions
			mockClientFactory := func(opts *mqttLib.ClientOptions) Client {
				calledWithOpts = *opts
				return &mockMQTTClient{}
			}

			n := New(
				tc.cfg,
				receivers.Metadata{},
				tmpl,
				&logging.FakeLogger{},
				mockClientFactory,
			)

			require.NotNil(t, n)
			require.NotNil(t, n.Base)
			require.NotNil(t, n.log)
			require.NotNil(t, n.settings)
			require.NotNil(t, n.client)
			require.Equal(t, tmpl, n.tmpl)

			require.NotNil(t, calledWithOpts)
			require.Equal(t, tc.cfg.Username, calledWithOpts.Username)
			require.Equal(t, tc.cfg.Password, calledWithOpts.Password)
			require.Equal(t, tc.cfg.BrokerURL, calledWithOpts.Servers[0].String())
			require.Equal(t, tc.cfg.ClientID, calledWithOpts.ClientID)

			if tc.cfg.InsecureSkipVerify {
				require.True(t, calledWithOpts.TLSConfig.InsecureSkipVerify)
			} else {
				require.Nil(t, calledWithOpts.TLSConfig)
			}
		})
	}
}
