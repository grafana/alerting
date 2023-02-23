package webhook

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	template2 "github.com/grafana/alerting/templates"
)

// Notifier is responsible for sending
// alert notifications as webhooks.
type Notifier struct {
	*receivers.Base
	log      logging.Logger
	ns       receivers.WebhookSender
	images   images.ImageStore
	tmpl     *template.Template
	orgID    int64
	settings Config
}

// New is the constructor for
// the WebHook notifier.
func New(factoryConfig receivers.FactoryConfig) (*Notifier, error) {
	settings, err := NewConfig(factoryConfig.Config.Settings, factoryConfig.Decrypt)
	if err != nil {
		return nil, err
	}
	return &Notifier{
		Base:     receivers.NewBase(factoryConfig.Config),
		orgID:    factoryConfig.Config.OrgID,
		log:      factoryConfig.Logger,
		ns:       factoryConfig.NotificationService,
		images:   factoryConfig.ImageStore,
		tmpl:     factoryConfig.Template,
		settings: settings,
	}, nil
}

// webhookMessage defines the JSON object send to webhook endpoints.
type webhookMessage struct {
	*template2.ExtendedData

	// The protocol version.
	Version         string `json:"version"`
	GroupKey        string `json:"groupKey"`
	TruncatedAlerts int    `json:"truncatedAlerts"`
	OrgID           int64  `json:"orgId"`
	Title           string `json:"title"`
	State           string `json:"state"`
	Message         string `json:"message"`
}

// Notify implements the Notifier interface.
func (wn *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	groupKey, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return false, err
	}

	as, numTruncated := truncateAlerts(wn.settings.MaxAlerts, as)
	var tmplErr error
	tmpl, data := template2.TmplText(ctx, wn.tmpl, as, wn.log, &tmplErr)

	// Augment our Alert data with ImageURLs if available.
	_ = images.WithStoredImages(ctx, wn.log, wn.images,
		func(index int, image images.Image) error {
			if len(image.URL) != 0 {
				data.Alerts[index].ImageURL = image.URL
			}
			return nil
		},
		as...)

	msg := &webhookMessage{
		Version:         "1",
		ExtendedData:    data,
		GroupKey:        groupKey.String(),
		TruncatedAlerts: numTruncated,
		OrgID:           wn.orgID,
		Title:           tmpl(wn.settings.Title),
		Message:         tmpl(wn.settings.Message),
	}
	if types.Alerts(as...).Status() == model.AlertFiring {
		msg.State = string(receivers.AlertStateAlerting)
	} else {
		msg.State = string(receivers.AlertStateOK)
	}

	if tmplErr != nil {
		wn.log.Warn("failed to template webhook message", "error", tmplErr.Error())
		tmplErr = nil
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return false, err
	}

	headers := make(map[string]string)
	if wn.settings.AuthorizationScheme != "" && wn.settings.AuthorizationCredentials != "" {
		headers["Authorization"] = fmt.Sprintf("%s %s", wn.settings.AuthorizationScheme, wn.settings.AuthorizationCredentials)
	}

	parsedURL := tmpl(wn.settings.URL)
	if tmplErr != nil {
		return false, tmplErr
	}

	cmd := &receivers.SendWebhookSettings{
		URL:        parsedURL,
		User:       wn.settings.User,
		Password:   wn.settings.Password,
		Body:       string(body),
		HTTPMethod: wn.settings.HTTPMethod,
		HTTPHeader: headers,
	}

	if err := wn.ns.SendWebhook(ctx, cmd); err != nil {
		return false, err
	}

	return true, nil
}

func truncateAlerts(maxAlerts int, alerts []*types.Alert) ([]*types.Alert, int) {
	if maxAlerts > 0 && len(alerts) > maxAlerts {
		return alerts[:maxAlerts], len(alerts) - maxAlerts
	}

	return alerts, 0
}

func (wn *Notifier) SendResolved() bool {
	return !wn.GetDisableResolveMessage()
}
