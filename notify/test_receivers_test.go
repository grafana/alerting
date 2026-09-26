package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/nfstatus"
)

type capturingNotifier struct {
	alerts []*types.Alert
}

func (c *capturingNotifier) Notify(_ context.Context, alerts ...*types.Alert) (nfstatus.NotifyInfo, bool, error) {
	c.alerts = append([]*types.Alert(nil), alerts...)
	return nfstatus.NotifyInfo{}, false, nil
}

type alwaysSendResolved struct{}

func (alwaysSendResolved) SendResolved() bool { return true }

func newCapturingIntegration(t *testing.T) (*nfstatus.Integration, *capturingNotifier) {
	t.Helper()
	n := &capturingNotifier{}
	integration := nfstatus.NewIntegration(n, alwaysSendResolved{}, "test", 0, "test-receiver", nil, log.NewNopLogger())
	return integration, n
}

func TestNewTestAlert(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("firing by default leaves EndsAt zero", func(t *testing.T) {
		alert := newTestAlert(nil, now, now)
		require.True(t, alert.EndsAt.IsZero())
		require.False(t, alert.ResolvedAt(now))
		require.Equal(t, model.AlertFiring, alert.StatusAt(now))
		require.Equal(t, model.LabelValue("TestAlert"), alert.Labels["alertname"])
	})

	t.Run("explicit firing status leaves EndsAt zero", func(t *testing.T) {
		alert := newTestAlert(&models.TestReceiversConfigAlertParams{
			Status: model.AlertFiring,
			Labels: model.LabelSet{"severity": "critical"},
		}, now, now)
		require.True(t, alert.EndsAt.IsZero())
		require.False(t, alert.ResolvedAt(now))
		require.Equal(t, model.LabelValue("critical"), alert.Labels["severity"])
	})

	t.Run("resolved status sets EndsAt in the past", func(t *testing.T) {
		alert := newTestAlert(&models.TestReceiversConfigAlertParams{
			Status: model.AlertResolved,
		}, now, now)
		require.False(t, alert.EndsAt.IsZero())
		require.True(t, alert.ResolvedAt(now))
		require.Equal(t, model.AlertResolved, alert.StatusAt(now))
		require.Equal(t, now, alert.EndsAt)
	})
}

func TestNewTestAlerts(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("empty params yields one default firing alert", func(t *testing.T) {
		alerts := newTestAlerts(nil, now, now)
		require.Len(t, alerts, 1)
		require.False(t, alerts[0].ResolvedAt(now))
	})

	t.Run("multiple firing alerts", func(t *testing.T) {
		alerts := newTestAlerts([]models.TestReceiversConfigAlertParams{
			{Labels: model.LabelSet{"alertname": "A"}, Status: model.AlertFiring},
			{Labels: model.LabelSet{"alertname": "B"}, Status: model.AlertFiring},
		}, now, now)
		require.Len(t, alerts, 2)
		require.False(t, alerts[0].ResolvedAt(now))
		require.False(t, alerts[1].ResolvedAt(now))
		require.Equal(t, model.LabelValue("A"), alerts[0].Labels["alertname"])
		require.Equal(t, model.LabelValue("B"), alerts[1].Labels["alertname"])
	})

	t.Run("mixed firing and resolved alerts", func(t *testing.T) {
		alerts := newTestAlerts([]models.TestReceiversConfigAlertParams{
			{Labels: model.LabelSet{"alertname": "firing"}, Status: model.AlertFiring},
			{Labels: model.LabelSet{"alertname": "resolved"}, Status: model.AlertResolved},
		}, now, now)
		require.Len(t, alerts, 2)
		require.False(t, alerts[0].ResolvedAt(now))
		require.True(t, alerts[1].ResolvedAt(now))
	})
}

func TestResolveTestAlertParams(t *testing.T) {
	t.Run("Alerts takes precedence over Alert", func(t *testing.T) {
		params := resolveTestAlertParams(TestReceiversConfigBodyParams{
			Alert: &models.TestReceiversConfigAlertParams{Labels: model.LabelSet{"from": "alert"}},
			Alerts: []models.TestReceiversConfigAlertParams{
				{Labels: model.LabelSet{"from": "alerts"}},
			},
		})
		require.Len(t, params, 1)
		require.Equal(t, model.LabelValue("alerts"), params[0].Labels["from"])
	})

	t.Run("falls back to singular Alert", func(t *testing.T) {
		params := resolveTestAlertParams(TestReceiversConfigBodyParams{
			Alert: &models.TestReceiversConfigAlertParams{Labels: model.LabelSet{"from": "alert"}},
		})
		require.Len(t, params, 1)
		require.Equal(t, model.LabelValue("alert"), params[0].Labels["from"])
	})

	t.Run("both empty returns nil", func(t *testing.T) {
		require.Nil(t, resolveTestAlertParams(TestReceiversConfigBodyParams{}))
	})
}

func TestTestNotifier_MultiAlert(t *testing.T) {
	now := time.Now()
	integration, capture := newCapturingIntegration(t)

	firing := newTestAlert(&models.TestReceiversConfigAlertParams{
		Labels: model.LabelSet{"alertname": "firing"},
		Status: model.AlertFiring,
	}, now, now)
	resolved := newTestAlert(&models.TestReceiversConfigAlertParams{
		Labels: model.LabelSet{"alertname": "resolved"},
		Status: model.AlertResolved,
	}, now, now)

	err := TestNotifier(context.Background(), integration, []*types.Alert{&firing, &resolved}, now)
	require.NoError(t, err)
	require.Len(t, capture.alerts, 2)
	require.Equal(t, model.LabelValue("firing"), capture.alerts[0].Labels["alertname"])
	require.Equal(t, model.LabelValue("resolved"), capture.alerts[1].Labels["alertname"])
}

func TestTestReceivers_AlertFlows(t *testing.T) {
	build := func(_ models.ReceiverConfig, _ TemplatesProvider) ([]*nfstatus.Integration, error) {
		integration, _ := newCapturingIntegration(t)
		return []*nfstatus.Integration{integration}, nil
	}
	receiver := models.ReceiverConfig{
		Name: "test-receiver",
		Integrations: []*models.IntegrationConfig{{
			UID:      "uid-1",
			Name:     "test",
			Type:     "webhook",
			Settings: json.RawMessage(`{}`),
		}},
	}

	t.Run("one firing alert via singular Alert (regression)", func(t *testing.T) {
		res, status, err := TestReceivers(context.Background(), TestReceiversConfigBodyParams{
			Alert: &models.TestReceiversConfigAlertParams{
				Labels: model.LabelSet{"alertname": "one"},
				Status: model.AlertFiring,
			},
			Receivers: []models.ReceiverConfig{receiver},
		}, build, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
		require.Len(t, res.Alerts, 1)
		require.Equal(t, res.Alert, res.Alerts[0])
		require.False(t, res.Alerts[0].Resolved())
		require.Equal(t, model.LabelValue("one"), res.Alert.Labels["alertname"])
	})

	t.Run("one resolved alert", func(t *testing.T) {
		res, status, err := TestReceivers(context.Background(), TestReceiversConfigBodyParams{
			Alert: &models.TestReceiversConfigAlertParams{
				Status: model.AlertResolved,
			},
			Receivers: []models.ReceiverConfig{receiver},
		}, build, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
		require.Len(t, res.Alerts, 1)
		require.True(t, res.Alerts[0].Resolved())
	})

	t.Run("multiple firing alerts", func(t *testing.T) {
		res, status, err := TestReceivers(context.Background(), TestReceiversConfigBodyParams{
			Alerts: []models.TestReceiversConfigAlertParams{
				{Labels: model.LabelSet{"alertname": "a"}, Status: model.AlertFiring},
				{Labels: model.LabelSet{"alertname": "b"}, Status: model.AlertFiring},
			},
			Receivers: []models.ReceiverConfig{receiver},
		}, build, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
		require.Len(t, res.Alerts, 2)
		require.Equal(t, res.Alert, res.Alerts[0])
		require.False(t, res.Alerts[0].Resolved())
		require.False(t, res.Alerts[1].Resolved())
	})

	t.Run("mixed firing and resolved alerts", func(t *testing.T) {
		res, status, err := TestReceivers(context.Background(), TestReceiversConfigBodyParams{
			Alerts: []models.TestReceiversConfigAlertParams{
				{Labels: model.LabelSet{"alertname": "firing"}, Status: model.AlertFiring},
				{Labels: model.LabelSet{"alertname": "resolved"}, Status: model.AlertResolved},
			},
			Receivers: []models.ReceiverConfig{receiver},
		}, build, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
		require.Len(t, res.Alerts, 2)
		require.False(t, res.Alerts[0].Resolved())
		require.True(t, res.Alerts[1].Resolved())
	})

	t.Run("default alert when both Alert and Alerts empty", func(t *testing.T) {
		res, status, err := TestReceivers(context.Background(), TestReceiversConfigBodyParams{
			Receivers: []models.ReceiverConfig{receiver},
		}, build, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
		require.Len(t, res.Alerts, 1)
		require.False(t, res.Alerts[0].Resolved())
		require.Equal(t, model.LabelValue("TestAlert"), res.Alert.Labels["alertname"])
	})
}

func TestTestIntegration_ResolvedStatus(t *testing.T) {
	var capture *capturingNotifier
	build := func(_ models.ReceiverConfig, _ TemplatesProvider) ([]*nfstatus.Integration, error) {
		integration, n := newCapturingIntegration(t)
		capture = n
		return []*nfstatus.Integration{integration}, nil
	}

	status, err := TestIntegration(context.Background(), "recv", models.IntegrationConfig{
		UID:  "uid",
		Name: "test",
		Type: "webhook",
	}, models.TestReceiversConfigAlertParams{
		Status: model.AlertResolved,
	}, build, nil)
	require.NoError(t, err)
	require.Empty(t, status.LastNotifyAttemptError)
	require.Len(t, capture.alerts, 1)
	require.True(t, capture.alerts[0].Resolved())
}
