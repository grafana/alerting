package logzio_opsgenie

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/grafana/alerting/models"
	"net/http"
	"strings"

	"github.com/prometheus/alertmanager/notify"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

// LOGZ.IO GRAFANA CHANGE :: DEV-46341 - Add support for logzio opsgenie integration
const (
	// https://docs.opsgenie.com/docs/alert-api - 130 characters meaning runes.
	logzioOpsGenieMaxMessageLenRunes = 130
)

var (
	ValidPriorities = map[string]bool{"P1": true, "P2": true, "P3": true, "P4": true, "P5": true}
)

// Notifier is responsible for sending alert notifications to Opsgenie.
type Notifier struct {
	*receivers.Base
	tmpl     *templates.Template
	log      logging.Logger
	ns       receivers.WebhookSender
	images   images.Provider
	settings Config
}

func New(cfg Config, meta receivers.Metadata, template *templates.Template, sender receivers.WebhookSender, images images.Provider, logger logging.Logger) *Notifier {
	return &Notifier{
		Base:     receivers.NewBase(meta),
		log:      logger,
		ns:       sender,
		images:   images,
		tmpl:     template,
		settings: cfg,
	}
}

// Notify sends an alert notification to Logzio Opsgenie
func (on *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	on.log.Debug("executing Logzio Opsgenie notification", "notification", on.Name)

	alerts := types.Alerts(as...)
	if alerts.Status() == model.AlertResolved && !on.SendResolved() {
		on.log.Debug("not sending a trigger to Logzio Opsgenie", "status", alerts.Status(), "auto resolve", on.SendResolved())
		return true, nil
	}

	body, url, err := on.buildLogzioOpsgenieMessage(ctx, alerts, as)
	if err != nil {
		return false, fmt.Errorf("build Logzio Opsgenie message: %w", err)
	}

	if url == "" {
		// Resolved alert with no auto close.
		// Hence skip sending anything.
		return true, nil
	}

	cmd := &receivers.SendWebhookSettings{
		URL:        url,
		Body:       string(body),
		HTTPMethod: http.MethodPost,
		HTTPHeader: map[string]string{
			"Content-Type": "application/json",
		},
	}

	if err := on.ns.SendWebhook(ctx, cmd); err != nil {
		return false, fmt.Errorf("send notification to Logzio Opsgenie: %w", err)
	}

	return true, nil
}

func (on *Notifier) buildLogzioOpsgenieMessage(ctx context.Context, alerts model.Alerts, as []*types.Alert) (payload []byte, apiURL string, err error) {
	key, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return nil, "", err
	}

	if alerts.Status() == model.AlertResolved {
		// For resolved notification, we only need the source.
		// Don't need to run other templates.
		if !on.settings.AutoClose { // TODO This should be handled by DisableResolveMessage?
			return nil, "", nil
		}
		msg := logzioOpsGenieCloseMessage{
			Source:         "LogzIO",
			AlertEventType: "close",
			Alias:          key.Hash(),
		}
		data, err := json.Marshal(msg)
		apiURL = fmt.Sprintf("%s?apiKey=%s", on.settings.APIUrl, on.settings.APIKey)
		return data, apiURL, err
	}

	// LOGZ.IO GRAFANA CHANGE :: DEV-43657 - Set logzio APP URLs for the URLs inside alert notifications
	basePath := receivers.ToBasePathWithAccountRedirect(on.tmpl.ExternalURL, as)
	ruleURL := receivers.ToLogzioAppPath(receivers.JoinURLPath(basePath, "/alerting/list", on.log))
	// LOGZ.IO GRAFANA CHANGE :: end

	var tmplErr error
	tmpl, data := templates.TmplText(ctx, on.tmpl, as, on.log, &tmplErr)

	message, truncated := receivers.TruncateInRunes(tmpl(on.settings.Message), logzioOpsGenieMaxMessageLenRunes)
	if truncated {
		on.log.Warn("Truncated message", "alert", key, "max_runes", logzioOpsGenieMaxMessageLenRunes)
	}

	description := tmpl(on.settings.Description)
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf(
			"%s\n%s\n\n%s",
			tmpl(templates.DefaultMessageTitleEmbed),
			ruleURL,
			tmpl(templates.DefaultMessageEmbed),
		)
	}

	var priority string

	// In the new notify system we've moved away from the grafana-tags. Instead, annotations on the rule itself should be used.
	lbls := make(map[string]string, len(data.CommonLabels))
	for k, v := range data.CommonLabels {
		lbls[k] = tmpl(v)

		// Though we disabled the override priority option in ui, we keep this code, so we are able to send alert priority.
		if k == "og_priority" && on.settings.OverridePriority {
			if ValidPriorities[v] {
				priority = v
			}
		}
	}

	// Check for templating errors
	if tmplErr != nil {
		on.log.Warn("failed to template Logzio Opsgenie message", "error", tmplErr.Error())
		tmplErr = nil
	}

	details := make(map[string]interface{})
	for k, v := range lbls {
		details[k] = v
	}

	var alertEventSamples string
	if len(as) == 1 {
		alertEventSamples = string(as[0].Annotations[models.ValueStringAnnotation])
	}
	details["url"] = ruleURL

	result := logzioOpsGenieCreateMessage{
		Alias:             key.Hash(),
		Description:       description,
		Source:            "Grafana",
		Message:           message,
		Details:           details,
		Priority:          priority,
		AlertEventSamples: alertEventSamples,
		AlertEventType:    "create",
		AlertViewLink:     ruleURL,
	}

	apiURL = tmpl(on.settings.APIUrl)
	if tmplErr != nil {
		on.log.Warn("failed to template Logzio Opsgenie URL", "error", tmplErr.Error(), "fallback", on.settings.APIUrl)
		apiURL = on.settings.APIUrl
	}

	apiURL = fmt.Sprintf("%s?apiKey=%s", on.settings.APIUrl, on.settings.APIKey)

	b, err := json.Marshal(result)
	return b, apiURL, err
}

func (on *Notifier) SendResolved() bool {
	return !on.GetDisableResolveMessage()
}

type logzioOpsGenieCreateMessage struct {
	Alias             string                 `json:"alert_alias"`
	Message           string                 `json:"alert_title"`
	Description       string                 `json:"alert_description,omitempty"`
	Details           map[string]interface{} `json:"alert_details"`
	Source            string                 `json:"source"`
	Priority          string                 `json:"priority,omitempty"`
	AlertEventSamples string                 `json:"alert_event_samples,omitempty"`
	AlertEventType    string                 `json:"alert_event_type,omitempty"`
	AlertViewLink     string                 `json:"alert_view_link,omitempty"`
}

type logzioOpsGenieCloseMessage struct {
	Source         string `json:"source"`
	AlertEventType string `json:"alert_event_type"`
	Alias          string `json:"alert_alias"`
}

// LOGZ.IO GRAFANA CHANGE :: end
