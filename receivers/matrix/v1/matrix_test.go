package v1

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	tmpl := templates.ForTests(t)
	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name           string
		settings       Config
		alerts         []*types.Alert
		expectMsgType  string
		expectBodyHas  []string
		expectHTMLHas  []string
		expectHTMLMiss []string
	}{
		{
			name: "Firing alert, default template, labels are sorted",
			settings: Config{
				HomeserverURL: "https://matrix.example.com",
				AccessToken:   "secret",
				RoomID:        "!room:example.com",
				MessageType:   MessageTypeText,
				Title:         templates.DefaultMessageTitleEmbed,
				Message:       templates.DefaultMessageEmbed,
			},
			alerts: []*types.Alert{{
				Alert: model.Alert{
					Labels:       model.LabelSet{"alertname": "CPUHigh", "zone": "eu", "instance": "host-1", "severity": "critical"},
					Annotations:  model.LabelSet{"summary": "CPU is high"},
					GeneratorURL: "http://grafana/alerting/view",
				},
			}},
			expectMsgType: "m.text",
			expectBodyHas: []string{"CPUHigh"},
			expectHTMLHas: []string{"<h4>", "Firing", "CPUHigh", "CPU is high", "instance=host-1, severity=critical, zone=eu", `href="http://grafana/alerting/view"`},
		},
		{
			name: "Resolved and firing, m.notice",
			settings: Config{
				HomeserverURL: "https://matrix.example.com",
				AccessToken:   "secret",
				RoomID:        "!room:example.com",
				MessageType:   MessageTypeNotice,
				Title:         "alerts",
				Message:       "body text",
			},
			alerts: []*types.Alert{
				{Alert: model.Alert{Labels: model.LabelSet{"alertname": "A"}}},
				{Alert: model.Alert{Labels: model.LabelSet{"alertname": "B"}, EndsAt: time.Now().Add(-time.Minute)}},
			},
			expectMsgType: "m.notice",
			expectBodyHas: []string{"alerts", "body text"},
			expectHTMLHas: []string{"Firing", "A", "Resolved", "B"},
		},
		{
			name: "Title only",
			settings: Config{
				HomeserverURL: "https://matrix.example.com",
				AccessToken:   "secret",
				RoomID:        "!room:example.com",
				MessageType:   MessageTypeText,
				Title:         "only title",
				Message:       "",
			},
			alerts: []*types.Alert{{
				Alert: model.Alert{Labels: model.LabelSet{"alertname": "X"}},
			}},
			expectMsgType:  "m.text",
			expectBodyHas:  []string{"only title"},
			expectHTMLHas:  []string{"<h4>only title</h4>"},
			expectHTMLMiss: []string{"<a "},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sender := receivers.MockNotificationService()
			n := New(c.settings, receivers.Metadata{}, tmpl, sender, log.NewNopLogger())

			ctx := notify.WithGroupKey(context.Background(), "test")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			ok, err := n.Notify(ctx, c.alerts...)
			require.NoError(t, err)
			require.True(t, ok)

			sent := sender.Webhook
			require.Equal(t, "PUT", sent.HTTPMethod)
			require.Equal(t, "application/json", sent.ContentType)
			require.Equal(t, "Bearer secret", sent.HTTPHeader["Authorization"])

			require.True(t, strings.HasPrefix(sent.URL, "https://matrix.example.com/_matrix/client/v3/rooms/"))
			require.Contains(t, sent.URL, url.PathEscape("!room:example.com"))
			require.Contains(t, sent.URL, "/send/m.room.message/grafana-")

			var msg matrixMessage
			require.NoError(t, json.Unmarshal([]byte(sent.Body), &msg))
			require.Equal(t, c.expectMsgType, msg.MsgType)
			require.Equal(t, "org.matrix.custom.html", msg.Format)
			for _, want := range c.expectBodyHas {
				require.Contains(t, msg.Body, want)
			}
			for _, want := range c.expectHTMLHas {
				require.Contains(t, msg.FormattedBody, want)
			}
			for _, miss := range c.expectHTMLMiss {
				require.NotContains(t, msg.FormattedBody, miss)
			}

			require.NotNil(t, sent.Validation)
		})
	}
}

func TestValidateResponse(t *testing.T) {
	require.NoError(t, validateResponse([]byte(`{}`), 200))
	require.NoError(t, validateResponse([]byte(`{"event_id":"$abc"}`), 200))

	err := validateResponse([]byte(`{"errcode":"M_FORBIDDEN","error":"you shall not pass"}`), 403)
	require.Error(t, err)
	require.Contains(t, err.Error(), "matrix API responded")
	require.Contains(t, err.Error(), "M_FORBIDDEN")
	require.Contains(t, err.Error(), "you shall not pass")

	err = validateResponse([]byte(`not json`), 500)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}

func TestSendResolved(t *testing.T) {
	n := &Notifier{Base: receivers.NewBase(receivers.Metadata{DisableResolveMessage: false}, log.NewNopLogger())}
	require.True(t, n.SendResolved())
	n = &Notifier{Base: receivers.NewBase(receivers.Metadata{DisableResolveMessage: true}, log.NewNopLogger())}
	require.False(t, n.SendResolved())
}

func TestRenderHTMLTruncationIsUTF8Safe(t *testing.T) {
	// Build a firing alert whose summary is long enough to force truncation
	// AND contains multi-byte characters (CJK + emoji) so any byte-boundary
	// split would produce invalid UTF-8.
	longSummary := strings.Repeat("日本語テスト🚀 ", 4000)
	alerts := []*types.Alert{{
		Alert: model.Alert{
			Labels:      model.LabelSet{"alertname": "wide"},
			Annotations: model.LabelSet{"summary": model.LabelValue(longSummary)},
		},
	}}

	out := renderHTML("title", alerts)

	require.LessOrEqual(t, len(out), maxFormattedBodyBytes, "rendered HTML should be truncated within the byte budget")
	require.True(t, utf8.ValidString(out), "truncated HTML must remain valid UTF-8")
}
