package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/nfstatus"
)

func (am *GrafanaAlertmanager) TestIntegration(ctx context.Context, receiverName string, integrationConfig models.IntegrationConfig, alert models.TestReceiversConfigAlertParams) error {
	am.reloadConfigMtx.RLock()
	templates := am.templates
	am.reloadConfigMtx.RUnlock()

	return TestIntegration(ctx, receiverName, integrationConfig, alert, am.buildReceiverIntegrations, templates)
}

func TestIntegration(ctx context.Context,
	receiverName string,
	integrationConfig models.IntegrationConfig,
	testAlert models.TestReceiversConfigAlertParams,
	buildIntegrationsFunc func(*APIReceiver, TemplatesProvider) ([]*nfstatus.Integration, error),
	tmplProvider TemplatesProvider,
) error {
	nf, err := buildIntegrationsFunc(&APIReceiver{
		ConfigReceiver: config.Receiver{
			Name: receiverName,
		},
		ReceiverConfig: models.ReceiverConfig{
			Integrations: []*models.IntegrationConfig{&integrationConfig},
		},
	}, tmplProvider)
	if err != nil || len(nf) == 0 {
		return err
	}
	now := time.Now()
	return TestNotifier(ctx, nf[0], newTestAlert(&testAlert, now, now), now)
}

func TestNotifier(ctx context.Context, notifier *nfstatus.Integration, testAlert types.Alert, now time.Time) error {
	ctx = notify.WithGroupKey(ctx, fmt.Sprintf("%s-%s-%d", notifier.Name(), testAlert.Labels.Fingerprint(), now.Unix()))
	ctx = notify.WithGroupLabels(ctx, testAlert.Labels)
	ctx = notify.WithReceiverName(ctx, notifier.Name())
	if _, err := notifier.Notify(ctx, &testAlert); err != nil {
		return err
	}
	return nil
}
