package opsgenie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

const (
	// https://docs.opsgenie.com/docs/alert-api - 130 characters meaning runes.
	opsGenieMaxMessageLenRunes = 130
)

var (
	ValidPriorities = map[string]bool{"P1": true, "P2": true, "P3": true, "P4": true, "P5": true}
)

// Notifier is responsible for sending alert notifications to Opsgenie.
type Notifier struct {
	*receivers.Base
	tmpl     *template.Template
	log      logging.Logger
	ns       receivers.WebhookSender
	images   images.ImageStore
	settings Config
}

// New is the constructor for the Opsgenie notifier
func New(fc receivers.FactoryConfig) (*Notifier, error) {
	settings, err := NewConfig(fc.Config.Settings, fc.Decrypt)
	if err != nil {
		return nil, err
	}
	return &Notifier{
		Base:     receivers.NewBase(fc.Config),
		tmpl:     fc.Template,
		log:      fc.Logger,
		ns:       fc.NotificationService,
		images:   fc.ImageStore,
		settings: settings,
	}, nil
}

// Notify sends an alert notification to Opsgenie
func (on *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	on.log.Debug("executing Opsgenie notification", "notification", on.Name)

	alerts := types.Alerts(as...)
	if alerts.Status() == model.AlertResolved && !on.SendResolved() {
		on.log.Debug("not sending a trigger to Opsgenie", "status", alerts.Status(), "auto resolve", on.SendResolved())
		return true, nil
	}

	body, url, err := on.buildOpsgenieMessage(ctx, alerts, as)
	if err != nil {
		return false, fmt.Errorf("build Opsgenie message: %w", err)
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
			"Content-Type":  "application/json",
			"Authorization": fmt.Sprintf("GenieKey %s", on.settings.APIKey),
		},
	}

	if err := on.ns.SendWebhook(ctx, cmd); err != nil {
		return false, fmt.Errorf("send notification to Opsgenie: %w", err)
	}

	return true, nil
}

func (on *Notifier) buildOpsgenieMessage(ctx context.Context, alerts model.Alerts, as []*types.Alert) (payload []byte, apiURL string, err error) {
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
		msg := opsGenieCloseMessage{
			Source: "Grafana",
		}
		data, err := json.Marshal(msg)
		apiURL = fmt.Sprintf("%s/%s/close?identifierType=alias", on.settings.APIUrl, key.Hash())
		return data, apiURL, err
	}

	ruleURL := receivers.JoinURLPath(on.tmpl.ExternalURL.String(), "/alerting/list", on.log)

	var tmplErr error
	tmpl, data := templates.TmplText(ctx, on.tmpl, as, on.log, &tmplErr)

	message, truncated := receivers.TruncateInRunes(tmpl(on.settings.Message), opsGenieMaxMessageLenRunes)
	if truncated {
		on.log.Warn("Truncated message", "alert", key, "max_runes", opsGenieMaxMessageLenRunes)
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
		if k == "og_priority" && on.settings.OverridePriority {
			if ValidPriorities[v] {
				priority = v
			}
		}
	}

	// Check for templating errors
	if tmplErr != nil {
		on.log.Warn("failed to template Opsgenie message", "error", tmplErr.Error())
		tmplErr = nil
	}

	details := make(map[string]interface{})
	details["url"] = ruleURL
	if on.sendDetails() {
		for k, v := range lbls {
			details[k] = v
		}
		var imageUrls []string
		_ = images.WithStoredImages(ctx, on.log, on.images,
			func(_ int, image images.Image) error {
				if len(image.URL) == 0 {
					return nil
				}
				imageUrls = append(imageUrls, image.URL)
				return nil
			},
			as...)

		if len(imageUrls) != 0 {
			details["image_urls"] = imageUrls
		}
	}

	tags := make([]string, 0, len(lbls))
	if on.sendTags() {
		for k, v := range lbls {
			tags = append(tags, fmt.Sprintf("%s:%s", k, v))
		}
	}
	sort.Strings(tags)

	result := opsGenieCreateMessage{
		Alias:       key.Hash(),
		Description: description,
		Tags:        tags,
		Source:      "Grafana",
		Message:     message,
		Details:     details,
		Priority:    priority,
	}

	apiURL = tmpl(on.settings.APIUrl)
	if tmplErr != nil {
		on.log.Warn("failed to template Opsgenie URL", "error", tmplErr.Error(), "fallback", on.settings.APIUrl)
		apiURL = on.settings.APIUrl
	}

	b, err := json.Marshal(result)
	return b, apiURL, err
}

func (on *Notifier) SendResolved() bool {
	return !on.GetDisableResolveMessage()
}

func (on *Notifier) sendDetails() bool {
	return on.settings.SendTagsAs == SendDetails || on.settings.SendTagsAs == SendBoth
}

func (on *Notifier) sendTags() bool {
	return on.settings.SendTagsAs == SendTags || on.settings.SendTagsAs == SendBoth
}

type opsGenieCreateMessage struct {
	Alias       string                           `json:"alias"`
	Message     string                           `json:"message"`
	Description string                           `json:"description,omitempty"`
	Details     map[string]interface{}           `json:"details"`
	Source      string                           `json:"source"`
	Responders  []opsGenieCreateMessageResponder `json:"responders,omitempty"`
	Tags        []string                         `json:"tags"`
	Note        string                           `json:"note,omitempty"`
	Priority    string                           `json:"priority,omitempty"`
	Entity      string                           `json:"entity,omitempty"`
	Actions     []string                         `json:"actions,omitempty"`
}

type opsGenieCreateMessageResponder struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Type     string `json:"type"` // team, user, escalation, schedule etc.
}

type opsGenieCloseMessage struct {
	Source string `json:"source"`
}
