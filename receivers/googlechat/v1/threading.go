package v1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
)

// https://developers.google.com/workspace/chat/api/reference/rest/v1/spaces.messages/create#messagereplyoption
const (
	messageReplyOptionParameter     = "messageReplyOption"
	replyMessageFallbackToNewThread = "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD"
)

// reportsNewAlerts reports whether the notification was sent because new alerts
// joined an open incident. Those are posted as new messages, because replies are
// collapsed in the space and may not notify its members.
func reportsNewAlerts(ctx context.Context) bool {
	reason, ok := notify.NotificationReason(ctx)
	return ok && reason == notify.ReasonNewAlertsInGroup
}

// applyThreading places the message in the thread of its incident and returns the
// webhook URL to post it to. The message is left untouched on error.
func applyThreading(ctx context.Context, webhookURL string, message *outerStruct, alerts []*types.Alert) (string, error) {
	groupKey, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return "", err
	}

	threadedURL, err := threadedWebhookURL(webhookURL)
	if err != nil {
		return "", err
	}

	message.Thread = &thread{ThreadKey: threadKey(groupKey, alerts)}
	return threadedURL, nil
}

// threadKey is derived from the notification alone, so all HA replicas agree on it.
// Changing the derivation moves open incidents to new threads after an upgrade.
func threadKey(groupKey notify.Key, alerts []*types.Alert) string {
	incident := groupKey.String() + "\x00" + incidentStart(alerts).UTC().Format(time.RFC3339Nano)
	hash := sha256.Sum256([]byte(incident))
	return hex.EncodeToString(hash[:])
}

// incidentStart returns the earliest StartsAt among the alerts whose activity
// overlaps, directly or through other alerts, with the latest activity in the
// notification. Grafana and Prometheus keep sending resolved alerts for a while,
// so alerts that ended before the current incident started must be left out.
func incidentStart(alerts []*types.Alert) time.Time {
	byStart := slices.Clone(alerts)
	slices.SortFunc(byStart, func(first, second *types.Alert) int {
		return first.StartsAt.Compare(second.StartsAt)
	})

	var start, end time.Time
	ongoing := false
	for index, alert := range byStart {
		if index == 0 || (!ongoing && alert.StartsAt.After(end)) {
			start, end, ongoing = alert.StartsAt, alert.EndsAt, !alert.Resolved()
			continue
		}
		if !alert.Resolved() {
			ongoing = true
		} else if alert.EndsAt.After(end) {
			end = alert.EndsAt
		}
	}
	return start
}

// threadedWebhookURL never includes the URL in its errors, because it carries the
// webhook secret.
func threadedWebhookURL(webhookURL string) (string, error) {
	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return "", errors.New("the webhook URL cannot be parsed")
	}

	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", errors.New("the query string of the webhook URL cannot be parsed")
	}

	query.Set(messageReplyOptionParameter, replyMessageFallbackToNewThread)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
