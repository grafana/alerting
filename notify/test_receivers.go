package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/nfstatus"
)

func (am *GrafanaAlertmanager) TestIntegration(
	ctx context.Context,
	receiverName string,
	integrationConfig models.IntegrationConfig,
	alert models.TestReceiversConfigAlertParams,
) (models.IntegrationStatus, error) {
	am.reloadConfigMtx.RLock()
	templates := am.templates
	am.reloadConfigMtx.RUnlock()

	return TestIntegration(ctx, receiverName, integrationConfig, alert, am.buildReceiverIntegrations, templates)
}

func TestIntegration(ctx context.Context,
	receiverName string,
	integrationConfig models.IntegrationConfig,
	testAlert models.TestReceiversConfigAlertParams,
	buildIntegrationsFunc func(models.ReceiverConfig, TemplatesProvider) ([]*nfstatus.Integration, error),
	tmplProvider TemplatesProvider,
) (models.IntegrationStatus, error) {
	nf, err := buildIntegrationsFunc(models.ReceiverConfig{
		Name:         receiverName,
		Integrations: []*models.IntegrationConfig{&integrationConfig},
	}, tmplProvider)
	if err != nil || len(nf) == 0 {
		return models.IntegrationStatus{}, err
	}
	now := time.Now()
	alert := newTestAlert(&testAlert, now, now)
	err = TestNotifier(ctx, nf[0], []*types.Alert{&alert}, now)
	result := models.IntegrationStatus{
		LastNotifyAttempt:         strfmt.DateTime(now),
		LastNotifyAttemptDuration: model.Duration(time.Since(now)).String(),
		Name:                      nf[0].Name(),
		SendResolved:              nf[0].SendResolved(),
	}
	if err != nil {
		result.LastNotifyAttemptError = err.Error()
	}
	return result, nil
}

// TestNotifier sends a test notification with one or more alerts.
func TestNotifier(ctx context.Context, notifier *nfstatus.Integration, alerts []*types.Alert, now time.Time) error {
	if len(alerts) == 0 {
		return fmt.Errorf("no alerts to notify")
	}
	ctx = context.WithValue(ctx, nfstatus.TestNotificationKey, true)
	ctx = notify.WithGroupKey(ctx, fmt.Sprintf("%s-%s-%d", notifier.Name(), alerts[0].Labels.Fingerprint(), now.Unix()))
	ctx = notify.WithGroupLabels(ctx, alerts[0].Labels)
	ctx = notify.WithReceiverName(ctx, notifier.Name())
	if _, err := notifier.Notify(ctx, alerts...); err != nil {
		return err
	}
	return nil
}
