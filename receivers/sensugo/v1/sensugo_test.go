package v1

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/go-kit/log"

	images2 "github.com/grafana/alerting/images"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	constNow := time.Now()
	defer mockTimeNow(constNow)()

	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	images := images2.NewFakeProvider(2)

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
				URL:       "http://sensu-api.local:8080",
				Entity:    "",
				Check:     "",
				Namespace: "",
				Handler:   "",
				APIKey:    "<apikey>",
				Message:   templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"entity": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "default",
						"namespace": "default",
					},
				},
				"check": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "default",
						"labels": map[string]string{
							"imageURL": "https://www.example.com/test-image-1.jpg",
							"ruleURL":  "http://localhost/alerting/list",
						},
					},
					"output":   "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=__alert_rule_uid__%3Drule+uid&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
					"issued":   timeNow().Unix(),
					"interval": 86400,
					"status":   2,
					"handlers": nil,
				},
				"ruleUrl": "http://localhost/alerting/list",
			},
			expMsgError: nil,
		}, {
			name: "Custom config with multiple alerts",
			settings: Config{
				URL:       "http://sensu-api.local:8080",
				Entity:    "grafana_instance_01",
				Check:     "grafana_rule_0",
				Namespace: "namespace",
				Handler:   "myhandler",
				APIKey:    "<apikey>",
				Message:   "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__alertImageToken__": "test-image-1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2", "__alertImageToken__": "test-image-2"},
					},
				},
			},
			expMsg: map[string]interface{}{
				"entity": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "grafana_instance_01",
						"namespace": "namespace",
					},
				},
				"check": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "grafana_rule_0",
						"labels": map[string]string{
							"imageURL": "https://www.example.com/test-image-1.jpg",
							"ruleURL":  "http://localhost/alerting/list",
						},
					},
					"output":   "2 alerts are firing, 0 are resolved",
					"issued":   timeNow().Unix(),
					"interval": 86400,
					"status":   2,
					"handlers": []string{"myhandler"},
				},
				"ruleUrl": "http://localhost/alerting/list",
			},
			expMsgError: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			webhookSender := receivers.MockNotificationService()

			sn := &Notifier{
				Base: receivers.NewBase(receivers.Metadata{}, log.NewNopLogger()),

				ns:       webhookSender,
				tmpl:     tmpl,
				settings: c.settings,
				images:   images,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ok, err := sn.Notify(ctx, c.alerts...)
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

func TestNotify_ExtraData(t *testing.T) {
	tmpl := templates.ForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	settings := Config{
		URL:     "http://sensu-api.local:8080",
		APIKey:  "<apikey>",
		Message: `{{ range $i, $a := .Alerts }}Alert {{ $i }}: {{ printf "%s" $a.ExtraData }} {{ end }}`,
	}

	// Create test alerts
	alerts := []*types.Alert{
		{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1"},
			},
		},
		{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert2", "lbl1": "val2"},
				Annotations: model.LabelSet{"ann1": "annv2"},
			},
		},
	}

	// Create extra data that will be passed via context
	extraData1 := json.RawMessage(`{"customField": "customValue1", "priority": "high"}`)
	extraData2 := json.RawMessage(`{"customField": "customValue2", "priority": "medium"}`)
	extraDataSlice := []json.RawMessage{extraData1, extraData2}

	webhookSender := receivers.MockNotificationService()

	sn := &Notifier{
		Base:     receivers.NewBase(receivers.Metadata{}, log.NewNopLogger()),
		ns:       webhookSender,
		tmpl:     tmpl,
		settings: settings,
		images:   &images2.UnavailableProvider{},
	}

	// Create context with extra data
	ctx := notify.WithGroupKey(context.Background(), "alertname")
	ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
	ctx = context.WithValue(ctx, receivers.ExtraDataKey, extraDataSlice)

	// Call Notify
	ok, err := sn.Notify(ctx, alerts...)
	require.NoError(t, err)
	require.True(t, ok)

	// Verify that extra data is present in the request body (output field)
	require.Contains(t, webhookSender.Webhook.Body, "customField")
	require.Contains(t, webhookSender.Webhook.Body, "customValue1")
	require.Contains(t, webhookSender.Webhook.Body, "customValue2")
}

// resetTimeNow resets the global variable timeNow to the default value, which is time.Now
func resetTimeNow() {
	timeNow = time.Now
}

func mockTimeNow(constTime time.Time) func() {
	timeNow = func() time.Time {
		return constTime
	}
	return resetTimeNow
}
