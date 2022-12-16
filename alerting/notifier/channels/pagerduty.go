package channels

import (
	"context"
	"encoding/json"
	"net/url"
	"os"

	gokit_log "github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
	old_notifiers "github.com/grafana/grafana/pkg/services/alerting/notifiers"
)

const (
	pagerDutyEventTrigger = "trigger"
	pagerDutyEventResolve = "resolve"
)

var (
	pagerdutyEventAPIURL = "https://events.pagerduty.com/v2/enqueue"
)

// PagerdutyNotifier is responsible for sending
// alert notifications to pagerduty
type PagerdutyNotifier struct {
	old_notifiers.NotifierBase
	Key           string
	Severity      string
	AutoResolve   bool
	CustomDetails map[string]string
	Class         string
	Component     string
	Group         string
	Summary       string
	tmpl          *template.Template
	log           log.Logger
	externalUrl   *url.URL
}

// NewPagerdutyNotifier is the constructor for the PagerDuty notifier
func NewPagerdutyNotifier(model *models.AlertNotification, t *template.Template, externalUrl *url.URL) (*PagerdutyNotifier, error) {
	key := model.DecryptedValue("integrationKey", model.Settings.Get("integrationKey").MustString())
	if key == "" {
		return nil, alerting.ValidationError{Reason: "Could not find integration key property in settings"}
	}

	customDetails := model.Settings.Get("customDetails").MustMap(map[string]interface{}{
		"firing":       `{{ template "pagerduty.default.instances" .Alerts.Firing }}`,
		"resolved":     `{{ template "pagerduty.default.instances" .Alerts.Resolved }}`,
		"num_firing":   `{{ .Alerts.Firing | len }}`,
		"num_resolved": `{{ .Alerts.Resolved | len }}`,
	})

	details := make(map[string]string, len(customDetails))
	for k, v := range customDetails {
		if val, ok := v.(string); ok {
			details[k] = val
		}
	}

	return &PagerdutyNotifier{
		NotifierBase:  old_notifiers.NewNotifierBase(model),
		Key:           key,
		CustomDetails: details,
		Severity:      model.Settings.Get("severity").MustString("critical"),
		AutoResolve:   model.Settings.Get("autoResolve").MustBool(true),
		Class:         model.Settings.Get("class").MustString("todo_class"), // TODO
		Component:     model.Settings.Get("component").MustString("Grafana"),
		Group:         model.Settings.Get("group").MustString("todo_group"), // TODO
		Summary:       model.Settings.Get("summary").MustString(`{{ template "pagerduty.default.description" .}}`),
		tmpl:          t,
		externalUrl:   externalUrl,
		log:           log.New("alerting.notifier." + model.Name),
	}, nil
}

// Notify sends an alert notification to PagerDuty
func (pn *PagerdutyNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	alerts := types.Alerts(as...)
	if alerts.Status() == model.AlertResolved && !pn.AutoResolve {
		pn.log.Debug("Not sending a trigger to Pagerduty", "status", alerts.Status(), "auto resolve", pn.AutoResolve)
		return true, nil
	}

	msg, eventType, err := pn.buildPagerdutyMessage(ctx, alerts, as)
	if err != nil {
		return false, errors.Wrap(err, "build pagerduty message")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return false, errors.Wrap(err, "marshal json")
	}

	pn.log.Info("Notifying Pagerduty", "event_type", eventType)
	cmd := &models.SendWebhookSync{
		Url:        pagerdutyEventAPIURL,
		Body:       string(body),
		HttpMethod: "POST",
		HttpHeader: map[string]string{
			"Content-Type": "application/json",
		},
	}
	if err := bus.DispatchCtx(ctx, cmd); err != nil {
		return false, errors.Wrap(err, "send notification to Pagerduty")
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

	data := notify.GetTemplateData(ctx, &template.Template{ExternalURL: pn.externalUrl}, as, gokit_log.NewNopLogger())
	var tmplErr error
	tmpl := notify.TmplText(pn.tmpl, data, &tmplErr)

	details := make(map[string]string, len(pn.CustomDetails))
	for k, v := range pn.CustomDetails {
		detail, err := pn.tmpl.ExecuteTextString(v, data)
		if err != nil {
			return nil, "", errors.Wrapf(err, "%q: failed to template %q", k, v)
		}
		details[k] = detail
	}

	msg := &pagerDutyMessage{
		Client:      "Grafana",
		ClientURL:   pn.externalUrl.String(),
		RoutingKey:  pn.Key,
		EventAction: eventType,
		DedupKey:    key.Hash(),
		Links: []pagerDutyLink{{
			HRef: pn.externalUrl.String(),
			Text: "External URL",
		}},
		Description: getTitleFromTemplateData(data), // TODO: this can be configurable template.
		Payload: &pagerDutyPayload{
			Component:     tmpl(pn.Component),
			Summary:       tmpl(pn.Summary),
			Severity:      tmpl(pn.Severity),
			CustomDetails: details,
			Class:         tmpl(pn.Class),
			Group:         tmpl(pn.Group),
		},
	}

	if hostname, err := os.Hostname(); err == nil {
		// TODO: should this be configured like in Prometheus AM?
		msg.Payload.Source = hostname
	}

	if tmplErr != nil {
		return nil, "", errors.Wrap(tmplErr, "failed to template PagerDuty message")
	}

	return msg, eventType, nil
}

func (pn *PagerdutyNotifier) SendResolved() bool {
	return pn.AutoResolve
}

type pagerDutyMessage struct {
	RoutingKey  string            `json:"routing_key,omitempty"`
	ServiceKey  string            `json:"service_key,omitempty"`
	DedupKey    string            `json:"dedup_key,omitempty"`
	IncidentKey string            `json:"incident_key,omitempty"`
	EventType   string            `json:"event_type,omitempty"`
	Description string            `json:"description,omitempty"`
	EventAction string            `json:"event_action"`
	Payload     *pagerDutyPayload `json:"payload"`
	Client      string            `json:"client,omitempty"`
	ClientURL   string            `json:"client_url,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	Links       []pagerDutyLink   `json:"links,omitempty"`
}

type pagerDutyLink struct {
	HRef string `json:"href"`
	Text string `json:"text"`
}

type pagerDutyPayload struct {
	Summary       string            `json:"summary"`
	Source        string            `json:"source"`
	Severity      string            `json:"severity"`
	Timestamp     string            `json:"timestamp,omitempty"`
	Class         string            `json:"class,omitempty"`
	Component     string            `json:"component,omitempty"`
	Group         string            `json:"group,omitempty"`
	CustomDetails map[string]string `json:"custom_details,omitempty"`
}
