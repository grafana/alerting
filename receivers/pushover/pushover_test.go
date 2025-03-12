package pushover

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	images2 "github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)

	images := images2.NewFakeProviderWithFile(t, 2)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      map[string]string
		expMsgError error
	}{
		{
			name: "Correct config with single alert",
			settings: Config{
				UserKey:          "<userKey>",
				APIToken:         "<apiToken>",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg: map[string]string{
				"user":       "<userKey>",
				"token":      "<apiToken>",
				"priority":   "0",
				"sound":      "",
				"title":      "[FIRING:1]  (val1)",
				"url":        "http://localhost/alerting/list",
				"url_title":  "Show alert rule",
				"message":    "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=__alert_rule_uid__%3Drule+uid&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh",
				"attachment": "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\b\x04\x00\x00\x00\xb5\x1c\f\x02\x00\x00\x00\vIDATx\xdacd`\x00\x00\x00\x06\x00\x020\x81\xd0/\x00\x00\x00\x00IEND\xaeB`\x82",
				"html":       "1",
			},
			expMsgError: nil,
		},
		{
			name: "Upload is false",
			settings: Config{
				UserKey:          "<userKey>",
				APIToken:         "<apiToken>",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           false,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg: map[string]string{
				"user":      "<userKey>",
				"token":     "<apiToken>",
				"priority":  "0",
				"sound":     "",
				"title":     "[FIRING:1]  (val1)",
				"url":       "http://localhost/alerting/list",
				"url_title": "Show alert rule",
				"message":   "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=__alert_rule_uid__%3Drule+uid&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh",
				"html":      "1",
			},
			expMsgError: nil,
		},
		{
			name: "Custom title",
			settings: Config{
				UserKey:          "<userKey>",
				APIToken:         "<apiToken>",
				AlertingPriority: 0,
				OkPriority:       0,
				Retry:            0,
				Expire:           0,
				Device:           "",
				AlertingSound:    "",
				OkSound:          "",
				Upload:           true,
				Title:            "Alerts firing: {{ len .Alerts.Firing }}",
				Message:          templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expMsg: map[string]string{
				"user":       "<userKey>",
				"token":      "<apiToken>",
				"priority":   "0",
				"sound":      "",
				"title":      "Alerts firing: 1",
				"url":        "http://localhost/alerting/list",
				"url_title":  "Show alert rule",
				"message":    "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=__alert_rule_uid__%3Drule+uid&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh",
				"attachment": "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\b\x04\x00\x00\x00\xb5\x1c\f\x02\x00\x00\x00\vIDATx\xdacd`\x00\x00\x00\x06\x00\x020\x81\xd0/\x00\x00\x00\x00IEND\xaeB`\x82",
				"html":       "1",
			},
			expMsgError: nil,
		},
		{
			name: "Custom config with multiple alerts",
			settings: Config{
				UserKey:          "<userKey>",
				APIToken:         "<apiToken>",
				AlertingPriority: 2,
				OkPriority:       0,
				Retry:            30,
				Expire:           86400,
				Device:           "device",
				AlertingSound:    "echo",
				OkSound:          "magic",
				Upload:           true,
				Title:            templates.DefaultMessageTitleEmbed,
				Message:          "{{ len .Alerts.Firing }} alerts are firing, {{ len .Alerts.Resolved }} are resolved",
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
			expMsg: map[string]string{
				"user":       "<userKey>",
				"token":      "<apiToken>",
				"priority":   "2",
				"sound":      "echo",
				"title":      "[FIRING:2]  ",
				"url":        "http://localhost/alerting/list",
				"url_title":  "Show alert rule",
				"message":    "2 alerts are firing, 0 are resolved",
				"attachment": "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\b\x04\x00\x00\x00\xb5\x1c\f\x02\x00\x00\x00\vIDATx\xdacd`\x00\x00\x00\x06\x00\x020\x81\xd0/\x00\x00\x00\x00IEND\xaeB`\x82",
				"html":       "1",
				"retry":      "30",
				"expire":     "86400",
				"device":     "device",
			},
			expMsgError: nil,
		},
	}

	for _, c := range cases {
		origGetBoundary := receivers.GetBoundary
		boundary := "abcd"
		receivers.GetBoundary = func() string {
			return boundary
		}
		t.Cleanup(func() {
			receivers.GetBoundary = origGetBoundary
		})

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
				images:   images,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ok, err := pn.Notify(ctx, c.alerts...)
			if c.expMsgError != nil {
				require.Error(t, err)
				require.False(t, ok)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			require.True(t, ok)

			bodyReader := multipart.NewReader(strings.NewReader(webhookSender.Webhook.Body), boundary)
			for {
				part, err := bodyReader.NextPart()
				if part == nil || errors.Is(err, io.EOF) {
					assert.Empty(t, c.expMsg, fmt.Sprintf("expected fields %v", c.expMsg))
					break
				}
				formField := part.FormName()
				expected, ok := c.expMsg[formField]
				assert.True(t, ok, fmt.Sprintf("unexpected field %s", formField))
				actual := []byte("")
				if expected != "" {
					buf := new(bytes.Buffer)
					_, err := buf.ReadFrom(part)
					require.NoError(t, err)
					actual = buf.Bytes()
				}
				assert.Equal(t, expected, string(actual))
				delete(c.expMsg, formField)
			}
		})
	}
}
