package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/alerting/log"
	"github.com/grafana/alerting/alerting/notifier/config"
	"github.com/grafana/alerting/alerting/notifier/images"
	"github.com/grafana/alerting/alerting/notifier/sender"
	template2 "github.com/grafana/alerting/alerting/notifier/template"
)

const (
	// https://developer.pagerduty.com/docs/ZG9jOjExMDI5NTgx-send-an-alert-event - 1024 characters or runes.
	pagerDutyMaxV2SummaryLenRunes = 1024
)

const (
	pagerDutyEventTrigger = "trigger"
	pagerDutyEventResolve = "resolve"
)

var (
	knownSeverity        = map[string]struct{}{config.DefaultPagerDutySeverity: {}, "error": {}, "warning": {}, "info": {}}
	PagerdutyEventAPIURL = "https://events.pagerduty.com/v2/enqueue"
)

// PagerdutyNotifier is responsible for sending
// alert notifications to pagerduty
type PagerdutyNotifier struct {
	*Base
	tmpl     *template.Template
	log      log.Logger
	ns       sender.WebhookSender
	images   images.ImageStore
	settings *config.PagerdutyConfig
}

func PagerdutyFactory(fc config.FactoryConfig) (NotificationChannel, error) {
	pdn, err := newPagerdutyNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return pdn, nil
}

// NewPagerdutyNotifier is the constructor for the PagerDuty notifier
func newPagerdutyNotifier(fc config.FactoryConfig) (*PagerdutyNotifier, error) {
	settings, err := config.BuildPagerdutyConfig(fc)
	if err != nil {
		return nil, err
	}

	return &PagerdutyNotifier{
		Base:     NewBase(fc.Config),
		tmpl:     fc.Template,
		log:      fc.Logger,
		ns:       fc.NotificationService,
		images:   fc.ImageStore,
		settings: settings,
	}, nil
}

// Notify sends an alert notification to PagerDuty
func (pn *PagerdutyNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	alerts := types.Alerts(as...)
	if alerts.Status() == model.AlertResolved && !pn.SendResolved() {
		pn.log.Debug("not sending a trigger to Pagerduty", "status", alerts.Status(), "auto resolve", pn.SendResolved())
		return true, nil
	}

	msg, eventType, err := pn.buildPagerdutyMessage(ctx, alerts, as)
	if err != nil {
		return false, fmt.Errorf("build pagerduty message: %w", err)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return false, fmt.Errorf("marshal json: %w", err)
	}

	pn.log.Info("notifying Pagerduty", "event_type", eventType)
	cmd := &sender.SendWebhookSettings{
		URL:        PagerdutyEventAPIURL,
		Body:       string(body),
		HTTPMethod: "POST",
		HTTPHeader: map[string]string{
			"Content-Type": "application/json",
		},
	}
	if err := pn.ns.SendWebhook(ctx, cmd); err != nil {
		return false, fmt.Errorf("send notification to Pagerduty: %w", err)
	}

	return true, nil
}

func (pn *PagerdutyNotifier) buildPagerdutyMessage(ctx context.Context, alerts model.Alerts, as []*types.Alert) (*pagerDutyMessage, string, error) {
	key, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return nil, "", err
	}

	eventType := pagerDutyEventTrigger
	if alerts.Status() == model.AlertResolved {
		eventType = pagerDutyEventResolve
	}

	var tmplErr error
	tmpl, data := template2.TmplText(ctx, pn.tmpl, as, pn.log, &tmplErr)

	details := make(map[string]string, len(pn.settings.CustomDetails))
	for k, v := range pn.settings.CustomDetails {
		detail, err := pn.tmpl.ExecuteTextString(v, data)
		if err != nil {
			return nil, "", fmt.Errorf("%q: failed to template %q: %w", k, v, err)
		}
		details[k] = detail
	}

	severity := strings.ToLower(tmpl(pn.settings.Severity))
	if _, ok := knownSeverity[severity]; !ok {
		pn.log.Warn("Severity is not in the list of known values - using default severity", "actualSeverity", severity, "defaultSeverity", config.DefaultPagerDutySeverity)
		severity = config.DefaultPagerDutySeverity
	}

	msg := &pagerDutyMessage{
		Client:      tmpl(pn.settings.Client),
		ClientURL:   tmpl(pn.settings.ClientURL),
		RoutingKey:  pn.settings.Key,
		EventAction: eventType,
		DedupKey:    key.Hash(),
		Links: []pagerDutyLink{{
			HRef: pn.tmpl.ExternalURL.String(),
			Text: "External URL",
		}},
		Payload: pagerDutyPayload{
			Source:        tmpl(pn.settings.Source),
			Component:     tmpl(pn.settings.Component),
			Summary:       tmpl(pn.settings.Summary),
			Severity:      severity,
			CustomDetails: details,
			Class:         tmpl(pn.settings.Class),
			Group:         tmpl(pn.settings.Group),
		},
	}

	_ = withStoredImages(ctx, pn.log, pn.images,
		func(_ int, image images.Image) error {
			if len(image.URL) != 0 {
				msg.Images = append(msg.Images, pagerDutyImage{Src: image.URL})
			}

			return nil
		},
		as...)

	summary, truncated := TruncateInRunes(msg.Payload.Summary, pagerDutyMaxV2SummaryLenRunes)
	if truncated {
		pn.log.Warn("Truncated summary", "key", key, "runes", pagerDutyMaxV2SummaryLenRunes)
	}
	msg.Payload.Summary = summary

	if tmplErr != nil {
		pn.log.Warn("failed to template PagerDuty message", "error", tmplErr.Error())
	}

	return msg, eventType, nil
}

func (pn *PagerdutyNotifier) SendResolved() bool {
	return !pn.GetDisableResolveMessage()
}

type pagerDutyMessage struct {
	RoutingKey  string           `json:"routing_key,omitempty"`
	ServiceKey  string           `json:"service_key,omitempty"`
	DedupKey    string           `json:"dedup_key,omitempty"`
	EventAction string           `json:"event_action"`
	Payload     pagerDutyPayload `json:"payload"`
	Client      string           `json:"client,omitempty"`
	ClientURL   string           `json:"client_url,omitempty"`
	Links       []pagerDutyLink  `json:"links,omitempty"`
	Images      []pagerDutyImage `json:"images,omitempty"`
}

type pagerDutyLink struct {
	HRef string `json:"href"`
	Text string `json:"text"`
}

type pagerDutyImage struct {
	Src string `json:"src"`
}

type pagerDutyPayload struct {
	Summary       string            `json:"summary"`
	Source        string            `json:"source"`
	Severity      string            `json:"severity"`
	Class         string            `json:"class,omitempty"`
	Component     string            `json:"component,omitempty"`
	Group         string            `json:"group,omitempty"`
	CustomDetails map[string]string `json:"custom_details,omitempty"`
}
