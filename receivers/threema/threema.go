package threema

import (
	"context"
	"fmt"
	"net/url"
	"path"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	template2 "github.com/grafana/alerting/templates"
)

var (
	ThreemaGwBaseURL = "https://msgapi.threema.ch/send_simple"
)

// ThreemaNotifier is responsible for sending
// alert notifications to Threema.
type ThreemaNotifier struct {
	*receivers.Base
	log      logging.Logger
	images   images.ImageStore
	ns       receivers.WebhookSender
	tmpl     *template.Template
	settings ThreemaConfig
}

func ThreemaFactory(fc receivers.FactoryConfig) (receivers.NotificationChannel, error) {
	notifier, err := NewThreemaNotifier(fc)
	if err != nil {
		return nil, receivers.ReceiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return notifier, nil
}

func NewThreemaNotifier(fc receivers.FactoryConfig) (*ThreemaNotifier, error) {
	settings, err := BuildThreemaConfig(fc)
	if err != nil {
		return nil, err
	}
	return &ThreemaNotifier{
		Base:     receivers.NewBase(fc.Config),
		log:      fc.Logger,
		images:   fc.ImageStore,
		ns:       fc.NotificationService,
		tmpl:     fc.Template,
		settings: settings,
	}, nil
}

// Notify send an alert notification to Threema
func (tn *ThreemaNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	tn.log.Debug("sending threema alert notification", "from", tn.settings.GatewayID, "to", tn.settings.RecipientID)

	// Set up basic API request data
	data := url.Values{}
	data.Set("from", tn.settings.GatewayID)
	data.Set("to", tn.settings.RecipientID)
	data.Set("secret", tn.settings.APISecret)
	data.Set("text", tn.buildMessage(ctx, as...))

	cmd := &receivers.SendWebhookSettings{
		URL:        ThreemaGwBaseURL,
		Body:       data.Encode(),
		HTTPMethod: "POST",
		HTTPHeader: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
	}
	if err := tn.ns.Send(ctx, cmd); err != nil {
		tn.log.Error("Failed to send threema notification", "error", err, "webhook", tn.Name)
		return false, err
	}

	return true, nil
}

func (tn *ThreemaNotifier) SendResolved() bool {
	return !tn.GetDisableResolveMessage()
}

func (tn *ThreemaNotifier) buildMessage(ctx context.Context, as ...*types.Alert) string {
	var tmplErr error
	tmpl, _ := template2.TmplText(ctx, tn.tmpl, as, tn.log, &tmplErr)

	message := fmt.Sprintf("%s%s\n\n*Message:*\n%s\n*URL:* %s\n",
		selectEmoji(as...),
		tmpl(tn.settings.Title),
		tmpl(tn.settings.Description),
		path.Join(tn.tmpl.ExternalURL.String(), "/alerting/list"),
	)

	if tmplErr != nil {
		tn.log.Warn("failed to template Threema message", "error", tmplErr.Error())
	}

	_ = receivers.WithStoredImages(ctx, tn.log, tn.images,
		func(_ int, image images.Image) error {
			if image.URL != "" {
				message += fmt.Sprintf("*Image:* %s\n", image.URL)
			}
			return nil
		}, as...)

	return message
}

func selectEmoji(as ...*types.Alert) string {
	if types.Alerts(as...).Status() == model.AlertResolved {
		return "\u2705 " // Check Mark Button
	}
	return "\u26A0\uFE0F " // Warning sign
}
