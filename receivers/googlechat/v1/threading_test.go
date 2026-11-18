package v1

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/receivers"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

const (
	threadingTestGroupKey  = `{}/{__grafana_receiver__="gchat"}:{alertname="HighLatency"}`
	threadingTestWebhook   = "https://chat.example.com/v1/spaces/SPACE/messages?key=KEY&token=TOKEN"
	threadingTestThreadURL = "https://chat.example.com/v1/spaces/SPACE/messages?key=KEY&messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD&token=TOKEN"
)

var threadingTestIncidentStart = time.Date(2026, time.November, 2, 10, 0, 0, 0, time.UTC)

func TestNotifyThreadingDisabled(t *testing.T) {
	alert := firingAlert("instance-1", threadingTestIncidentStart)

	cases := []struct {
		name     string
		settings string
	}{
		{
			name:     "enable_threading omitted",
			settings: `{"url": "` + threadingTestWebhook + `"}`,
		},
		{
			name:     "enable_threading false",
			settings: `{"url": "` + threadingTestWebhook + `", "enable_threading": false}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			config, err := NewConfig(json.RawMessage(c.settings), receiversTesting.DecryptForTesting(nil))
			require.NoError(t, err)
			notifier, webhookSender := newThreadingTestNotifier(t, config)

			notifyGroup(t, notifier, threadingTestGroupKey, alert)

			// The request must stay exactly as it was before threading existed.
			require.Equal(t, threadingTestWebhook, webhookSender.Webhook.URL)
			var body map[string]any
			require.NoError(t, json.Unmarshal([]byte(webhookSender.Webhook.Body), &body))
			require.NotContains(t, body, "thread")
		})
	}
}

func TestNotifyThreading(t *testing.T) {
	incidentStart := threadingTestIncidentStart

	t.Run("posts the notification in the thread of its incident", func(t *testing.T) {
		notifier, webhookSender := newThreadingTestNotifier(t, threadingTestConfig(threadingTestWebhook))

		notifyGroup(t, notifier, threadingTestGroupKey, firingAlert("instance-1", incidentStart))

		require.Equal(t, threadingTestThreadURL, webhookSender.Webhook.URL)
		// sha256 of the group key, a NUL byte and the RFC 3339 StartsAt of the incident.
		// Pinned because changing it would move open incidents to new threads after an upgrade.
		require.Equal(t, &thread{ThreadKey: "1133daba3e585f52f8b004ad3d55458d0ccbfc31fa717ec138f36ef0c2a0a52f"}, sentThread(t, webhookSender))
	})

	t.Run("keeps the resolved notification in the thread of the firing one", func(t *testing.T) {
		firing := sendAndGetThreadKey(t, threadingTestGroupKey, firingAlert("instance-1", incidentStart))
		resolved := sendAndGetThreadKey(t, threadingTestGroupKey, resolvedAlert("instance-1", incidentStart, incidentStart.Add(10*time.Minute)))

		require.Equal(t, firing, resolved)
	})

	t.Run("keeps the thread when newer alerts join the group", func(t *testing.T) {
		first := sendAndGetThreadKey(t, threadingTestGroupKey, firingAlert("instance-1", incidentStart))
		grown := sendAndGetThreadKey(t, threadingTestGroupKey,
			firingAlert("instance-2", incidentStart.Add(2*time.Minute)),
			firingAlert("instance-1", incidentStart),
		)
		partiallyResolved := sendAndGetThreadKey(t, threadingTestGroupKey,
			firingAlert("instance-1", incidentStart),
			resolvedAlert("instance-2", incidentStart.Add(2*time.Minute), incidentStart.Add(5*time.Minute)),
		)

		require.Equal(t, first, grown)
		require.Equal(t, first, partiallyResolved)
	})

	t.Run("keeps the thread while alerts that fired together resolve one by one", func(t *testing.T) {
		// Alerts raised by the same evaluation share their StartsAt, so the group
		// keeps its thread whichever of them resolves first.
		notifications := [][]*types.Alert{
			{firingAlert("instance-1", incidentStart), firingAlert("instance-2", incidentStart), firingAlert("instance-3", incidentStart)},
			{resolvedAlert("instance-1", incidentStart, incidentStart.Add(5*time.Minute)), firingAlert("instance-2", incidentStart), firingAlert("instance-3", incidentStart)},
			{firingAlert("instance-2", incidentStart), firingAlert("instance-3", incidentStart)},
			{resolvedAlert("instance-2", incidentStart, incidentStart.Add(10*time.Minute)), firingAlert("instance-3", incidentStart)},
			{firingAlert("instance-3", incidentStart)},
			{resolvedAlert("instance-3", incidentStart, incidentStart.Add(15*time.Minute))},
		}

		first := sendAndGetThreadKey(t, threadingTestGroupKey, notifications[0]...)
		for _, alerts := range notifications[1:] {
			require.Equal(t, first, sendAndGetThreadKey(t, threadingTestGroupKey, alerts...))
		}
	})

	t.Run("starts a new thread when the group fires again after being resolved", func(t *testing.T) {
		firing := sendAndGetThreadKey(t, threadingTestGroupKey, firingAlert("instance-1", incidentStart))
		resolved := sendAndGetThreadKey(t, threadingTestGroupKey, resolvedAlert("instance-1", incidentStart, incidentStart.Add(10*time.Minute)))
		firingAgain := sendAndGetThreadKey(t, threadingTestGroupKey, firingAlert("instance-1", incidentStart.Add(time.Hour)))

		require.Equal(t, firing, resolved)
		require.NotEqual(t, firing, firingAgain)
	})

	t.Run("starts a new thread when another alert fires while resolved alerts of the previous incident are still in the group", func(t *testing.T) {
		// Grafana and Prometheus keep sending resolved alerts to the Alertmanager for a
		// while, so they stay in the group and show up in the next notifications.
		previous := sendAndGetThreadKey(t, threadingTestGroupKey,
			firingAlert("instance-2", incidentStart),
			firingAlert("instance-3", incidentStart),
		)
		next := incidentStart.Add(4 * time.Minute)
		firing := sendAndGetThreadKey(t, threadingTestGroupKey,
			firingAlert("instance-1", next),
			resolvedAlert("instance-2", incidentStart, incidentStart.Add(2*time.Minute)),
			resolvedAlert("instance-3", incidentStart, incidentStart.Add(3*time.Minute)),
		)
		resolved := sendAndGetThreadKey(t, threadingTestGroupKey,
			resolvedAlert("instance-1", next, next.Add(time.Minute)),
			resolvedAlert("instance-2", incidentStart, incidentStart.Add(2*time.Minute)),
			resolvedAlert("instance-3", incidentStart, incidentStart.Add(3*time.Minute)),
		)

		require.NotEqual(t, previous, firing)
		require.Equal(t, sendAndGetThreadKey(t, threadingTestGroupKey, firingAlert("instance-1", next)), firing)
		require.Equal(t, firing, resolved)
	})

	t.Run("keeps the thread when the alert that started the incident resolves before the others", func(t *testing.T) {
		firing := sendAndGetThreadKey(t, threadingTestGroupKey,
			firingAlert("instance-1", incidentStart),
			firingAlert("instance-2", incidentStart.Add(2*time.Minute)),
		)
		oldestResolved := sendAndGetThreadKey(t, threadingTestGroupKey,
			resolvedAlert("instance-1", incidentStart, incidentStart.Add(10*time.Minute)),
			firingAlert("instance-2", incidentStart.Add(2*time.Minute)),
		)
		// instance-3 starts after instance-1 resolved, but instance-2 kept the incident going.
		joinedLater := sendAndGetThreadKey(t, threadingTestGroupKey,
			resolvedAlert("instance-1", incidentStart, incidentStart.Add(10*time.Minute)),
			firingAlert("instance-2", incidentStart.Add(2*time.Minute)),
			firingAlert("instance-3", incidentStart.Add(15*time.Minute)),
		)

		require.Equal(t, firing, oldestResolved)
		require.Equal(t, firing, joinedLater)
	})

	t.Run("starts a new thread once the alert that started the incident is no longer in the group", func(t *testing.T) {
		// Known limitation of deriving the key from the notification alone: when the
		// sender stops sending the resolved alert, the next oldest one identifies the incident.
		firing := sendAndGetThreadKey(t, threadingTestGroupKey,
			firingAlert("instance-1", incidentStart),
			firingAlert("instance-2", incidentStart.Add(2*time.Minute)),
		)
		remaining := sendAndGetThreadKey(t, threadingTestGroupKey, firingAlert("instance-2", incidentStart.Add(2*time.Minute)))

		require.NotEqual(t, firing, remaining)
	})

	t.Run("posts a notification about new alerts of an open incident as a new message", func(t *testing.T) {
		notifier, webhookSender := newThreadingTestNotifier(t, threadingTestConfig(threadingTestWebhook))

		notifyGroupWithReason(t, notifier, notify.ReasonNewAlertsInGroup,
			firingAlert("instance-1", incidentStart),
			firingAlert("instance-2", incidentStart.Add(2*time.Minute)),
		)

		require.Equal(t, threadingTestWebhook, webhookSender.Webhook.URL)
		require.Nil(t, sentThread(t, webhookSender))
	})

	t.Run("posts the other notifications of an incident in its thread", func(t *testing.T) {
		alerts := []*types.Alert{firingAlert("instance-1", incidentStart)}
		expected := sendAndGetThreadKey(t, threadingTestGroupKey, alerts...)

		for _, reason := range []notify.NotifyReason{
			notify.ReasonFirstNotification,
			notify.ReasonRepeatIntervalElapsed,
			notify.ReasonNewResolvedAlerts,
			notify.ReasonAllAlertsResolved,
			notify.ReasonUnknown,
		} {
			notifier, webhookSender := newThreadingTestNotifier(t, threadingTestConfig(threadingTestWebhook))

			notifyGroupWithReason(t, notifier, reason, alerts...)

			require.Equal(t, threadingTestThreadURL, webhookSender.Webhook.URL, reason.String())
			require.Equal(t, &thread{ThreadKey: expected}, sentThread(t, webhookSender), reason.String())
		}
	})

	t.Run("uses a separate thread for each alert group", func(t *testing.T) {
		alert := firingAlert("instance-1", incidentStart)

		first := sendAndGetThreadKey(t, threadingTestGroupKey, alert)
		other := sendAndGetThreadKey(t, `{}/{__grafana_receiver__="gchat"}:{alertname="DiskFull"}`, alert)

		require.NotEqual(t, first, other)
	})

	t.Run("adds messageReplyOption to a webhook URL without query parameters", func(t *testing.T) {
		notifier, webhookSender := newThreadingTestNotifier(t, threadingTestConfig("http://localhost"))

		notifyGroup(t, notifier, threadingTestGroupKey, firingAlert("instance-1", incidentStart))

		require.Equal(t, "http://localhost?messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD", webhookSender.Webhook.URL)
	})

	t.Run("overrides a messageReplyOption already in the webhook URL", func(t *testing.T) {
		notifier, webhookSender := newThreadingTestNotifier(t, threadingTestConfig(threadingTestWebhook+"&messageReplyOption=REPLY_MESSAGE_OR_FAIL"))

		notifyGroup(t, notifier, threadingTestGroupKey, firingAlert("instance-1", incidentStart))

		require.Equal(t, threadingTestThreadURL, webhookSender.Webhook.URL)
	})

	t.Run("sends the notification without a thread when the group key is missing", func(t *testing.T) {
		notifier, webhookSender := newThreadingTestNotifier(t, threadingTestConfig(threadingTestWebhook))

		notifyGroup(t, notifier, "", firingAlert("instance-1", incidentStart))

		require.Equal(t, threadingTestWebhook, webhookSender.Webhook.URL)
		require.Nil(t, sentThread(t, webhookSender))
	})

	t.Run("sends the notification without a thread when the webhook URL query cannot be parsed", func(t *testing.T) {
		const webhookWithSemicolon = "https://chat.example.com/v1/spaces/SPACE/messages?key=KEY;token=TOKEN"
		notifier, webhookSender := newThreadingTestNotifier(t, threadingTestConfig(webhookWithSemicolon))

		notifyGroup(t, notifier, threadingTestGroupKey, firingAlert("instance-1", incidentStart))

		require.Equal(t, webhookWithSemicolon, webhookSender.Webhook.URL)
		require.Nil(t, sentThread(t, webhookSender))
	})
}

func TestThreadedWebhookURLErrorsDoNotLeakTheURL(t *testing.T) {
	for _, webhookURL := range []string{
		"https://chat.example.com/v1/spaces/SPACE/messages?key=KEY;token=SECRET",
		"https://chat.example.com/v1/spaces/SPACE/messages?key=KEY&token=SECRET%zz",
		"https://chat.example.com:port/v1/spaces/SPACE/messages?token=SECRET",
	} {
		_, err := threadedWebhookURL(webhookURL)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "SECRET")
	}
}

func threadingTestConfig(webhookURL string) Config {
	return Config{
		URL:             webhookURL,
		Title:           templates.DefaultMessageTitleEmbed,
		Message:         templates.DefaultMessageEmbed,
		EnableThreading: true,
	}
}

func newThreadingTestNotifier(t *testing.T, config Config) (*Notifier, *receivers.NotificationServiceMock) {
	t.Helper()

	template := templates.ForTests(t)
	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	template.ExternalURL = externalURL

	webhookSender := receivers.MockNotificationService()
	notifier := New(config, receivers.Metadata{}, template, webhookSender, &images.UnavailableProvider{}, log.NewNopLogger(), "1.0.0")
	return notifier, webhookSender
}

// notifyGroup sends the alerts as one notification of the group. An empty
// group key leaves the group key out of the context.
func notifyGroup(t *testing.T, notifier *Notifier, groupKey string, alerts ...*types.Alert) {
	t.Helper()

	ctx := context.Background()
	if groupKey != "" {
		ctx = notify.WithGroupKey(ctx, groupKey)
	}
	ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": "HighLatency"})

	ok, err := notifier.Notify(ctx, alerts...)
	require.NoError(t, err)
	require.True(t, ok)
}

// notifyGroupWithReason sends the alerts as one notification of the group, with
// the reason the Alertmanager gives for sending it.
func notifyGroupWithReason(t *testing.T, notifier *Notifier, reason notify.NotifyReason, alerts ...*types.Alert) {
	t.Helper()

	ctx := notify.WithGroupKey(context.Background(), threadingTestGroupKey)
	ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": "HighLatency"})
	ctx = notify.WithNotificationReason(ctx, reason)

	ok, err := notifier.Notify(ctx, alerts...)
	require.NoError(t, err)
	require.True(t, ok)
}

// sendAndGetThreadKey sends the alerts as one notification of the group with a
// fresh notifier, mirroring how each notification can be sent by a different
// Alertmanager replica, and returns the thread key of the sent message.
func sendAndGetThreadKey(t *testing.T, groupKey string, alerts ...*types.Alert) string {
	t.Helper()

	notifier, webhookSender := newThreadingTestNotifier(t, threadingTestConfig(threadingTestWebhook))
	notifyGroup(t, notifier, groupKey, alerts...)

	sent := sentThread(t, webhookSender)
	require.NotNil(t, sent)
	require.NotEmpty(t, sent.ThreadKey)
	return sent.ThreadKey
}

func sentThread(t *testing.T, webhookSender *receivers.NotificationServiceMock) *thread {
	t.Helper()

	var message outerStruct
	require.NoError(t, json.Unmarshal([]byte(webhookSender.Webhook.Body), &message))
	return message.Thread
}

func firingAlert(instance string, startsAt time.Time) *types.Alert {
	return &types.Alert{
		Alert: model.Alert{
			Labels:   model.LabelSet{"alertname": "HighLatency", "instance": model.LabelValue(instance)},
			StartsAt: startsAt,
		},
	}
}

func resolvedAlert(instance string, startsAt, endsAt time.Time) *types.Alert {
	alert := firingAlert(instance, startsAt)
	alert.EndsAt = endsAt
	return alert
}
