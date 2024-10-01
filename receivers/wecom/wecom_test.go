package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify_GroupRobot(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      map[string]interface{}
		expMsgError error
	}{
		{
			name: "Default config with one alert",
			settings: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"markdown": map[string]interface{}{
					"content": "# [FIRING:1]  (val1)\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n\n",
				},
				"msgtype": "markdown",
			},
			expMsgError: nil,
		},
		{
			name: "Custom config with multiple alerts",
			settings: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     DefaultsgType,
				Message:     "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"markdown": map[string]interface{}{
					"content": "# [FIRING:2]  \n2 alerts are firing, 0 are resolved\n",
				},
				"msgtype": "markdown",
			},
			expMsgError: nil,
		},
		{
			name: "Custom title and message with multiple alerts",
			settings: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     DefaultsgType,
				Message:     "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
				Title:       "This notification is {{ .Status }}!",
				ToUser:      DefaultToUser,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"markdown": map[string]interface{}{
					"content": "# This notification is firing!\n2 alerts are firing, 0 are resolved\n",
				},
				"msgtype": "markdown",
			},
			expMsgError: nil,
		},
		{
			name: "Use default if optional fields are explicitly empty",
			settings: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"markdown": map[string]interface{}{
					"content": "# [FIRING:1]  (val1)\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n\n",
				},
				"msgtype": "markdown",
			},
			expMsgError: nil,
		},
		{
			name: "Use text are explicitly empty",
			settings: Config{
				Channel:     DefaultChannelType,
				EndpointURL: weComEndpoint,
				URL:         "http://localhost",
				AgentID:     "",
				CorpID:      "",
				Secret:      "",
				MsgType:     "text",
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"text": map[string]interface{}{
					"content": "[FIRING:1]  (val1)\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n\n",
				},
				"msgtype": "text",
			},
			expMsgError: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			webhookSender := receivers.MockNotificationService()

			pn := &Notifier{
				Base: &receivers.Base{
					Name:                  "",
					Type:                  "",
					UID:                   "",
					DisableResolveMessage: false,
				},
				log:      &logging.FakeLogger{},
				ns:       webhookSender,
				tmpl:     tmpl,
				settings: c.settings,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			ok, err := pn.Notify(ctx, c.alerts...)
			if c.expMsgError != nil {
				require.False(t, ok)
				require.Error(t, err)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			require.True(t, ok)

			expBody, err := json.Marshal(c.expMsg)
			require.NoError(t, err)

			require.JSONEq(t, string(expBody), webhookSender.Webhook.Body)
		})
	}
}

func TestNotify_ApiApp(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	tests := []struct {
		name         string
		settings     Config
		statusCode   int
		accessToken  string
		alerts       []*types.Alert
		expMsg       map[string]interface{}
		expInitError string
		expMsgError  error
	}{
		{
			name: "Default APIAPP config with one alert",
			settings: Config{
				Channel:     "apiapp",
				EndpointURL: weComEndpoint,
				URL:         "",
				AgentID:     "agent_id",
				CorpID:      "corp_id",
				Secret:      "secret",
				MsgType:     DefaultsgType,
				Message:     templates.DefaultMessageEmbed,
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
			accessToken: "access_token",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"markdown": map[string]interface{}{
					"content": "# [FIRING:1]  (val1)\n**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n\n",
				},
				"msgtype": "markdown",
				"agentid": "agent_id",
				"touser":  "@all",
			},
		},
		{
			name: "Custom message(markdown) with multiple alert",
			settings: Config{
				Channel:     "apiapp",
				EndpointURL: weComEndpoint,
				URL:         "",
				AgentID:     "agent_id",
				CorpID:      "corp_id",
				Secret:      "secret",
				MsgType:     DefaultsgType,
				Message:     "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
			accessToken:  "access_token",
			expInitError: "",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"markdown": map[string]interface{}{
					"content": "# [FIRING:2]  \n2 alerts are firing, 0 are resolved\n",
				},
				"msgtype": "markdown",
				"agentid": "agent_id",
				"touser":  "@all",
			},
			expMsgError: nil,
		},
		{
			name: "Custom message(Text) with multiple alert",
			settings: Config{
				Channel:     "apiapp",
				EndpointURL: weComEndpoint,
				URL:         "",
				AgentID:     "agent_id",
				CorpID:      "corp_id",
				Secret:      "secret",
				MsgType:     "text",
				Message:     "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
				Title:       templates.DefaultMessageTitleEmbed,
				ToUser:      DefaultToUser,
			},
			accessToken:  "access_token",
			expInitError: "",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				},
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"text": map[string]interface{}{
					"content": "[FIRING:2]  \n2 alerts are firing, 0 are resolved\n",
				},
				"msgtype": "text",
				"agentid": "agent_id",
				"touser":  "@all",
			},
			expMsgError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				accessToken := r.URL.Query().Get("access_token")
				if accessToken != tt.accessToken {
					t.Errorf("Expected access_token=%s got %s", tt.accessToken, accessToken)
					return
				}

				expBody, err := json.Marshal(tt.expMsg)
				require.NoError(t, err)

				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.JSONEq(t, string(expBody), string(b))
			}))
			defer server.Close()

			webhookSender := receivers.MockNotificationService()

			pn := &Notifier{
				Base: &receivers.Base{
					Name:                  "",
					Type:                  "",
					UID:                   "",
					DisableResolveMessage: false,
				},
				log:      &logging.FakeLogger{},
				ns:       webhookSender,
				tmpl:     tmpl,
				settings: tt.settings,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			// Avoid calling GetAccessToken interfaces
			pn.tokExpireAt = time.Now().Add(10 * time.Second)
			pn.tok = &accessToken{AccessToken: tt.accessToken}

			ok, err := pn.Notify(ctx, tt.alerts...)
			if tt.expMsgError != nil {
				require.False(t, ok)
				require.Error(t, err)
				require.Equal(t, tt.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			require.True(t, ok)

			expBody, err := json.Marshal(tt.expMsg)
			require.NoError(t, err)

			require.JSONEq(t, string(expBody), webhookSender.Webhook.Body)
		})
	}
}

func TestGetAccessToken(t *testing.T) {
	type fields struct {
		tok         *accessToken
		tokExpireAt time.Time
		corpid      string
		secret      string
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "no corpid",
			fields: fields{
				tok:         nil,
				tokExpireAt: time.Now().Add(-time.Minute),
			},
			want: "",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Error(t, err, i...)
			},
		},
		{
			name: "no corpsecret",
			fields: fields{
				tok:         nil,
				tokExpireAt: time.Now().Add(-time.Minute),
			},
			want: "",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Error(t, err, i...)
			},
		},
		{
			name: "get access token",
			fields: fields{
				corpid: "corpid",
				secret: "secret",
			},
			want:    "access_token",
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				corpid := r.URL.Query().Get("corpid")
				corpsecret := r.URL.Query().Get("corpsecret")

				assert.Equal(t, corpid, tt.fields.corpid, fmt.Sprintf("Expected corpid=%s got %s", tt.fields.corpid, corpid))
				if len(corpid) == 0 {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				assert.Equal(t, corpsecret, tt.fields.secret, fmt.Sprintf("Expected corpsecret=%s got %s", tt.fields.secret, corpsecret))
				if len(corpsecret) == 0 {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				b, err := json.Marshal(map[string]interface{}{
					"errcode":      0,
					"errmsg":       "ok",
					"access_token": tt.want,
					"expires_in":   7200,
				})
				assert.NoError(t, err)
				w.WriteHeader(http.StatusOK)
				_, err = w.Write(b)
				assert.NoError(t, err)
			}))
			defer server.Close()

			w := &Notifier{
				settings: Config{
					EndpointURL: server.URL,
					CorpID:      tt.fields.corpid,
					Secret:      tt.fields.secret,
				},
				tok:         tt.fields.tok,
				tokExpireAt: tt.fields.tokExpireAt,
			}
			got, err := w.GetAccessToken(context.Background())
			if !tt.wantErr(t, err, "GetAccessToken()") {
				return
			}
			assert.Equalf(t, tt.want, got, "GetAccessToken()")
		})
	}
}
