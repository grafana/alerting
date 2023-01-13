package channels

import (
	"context"
	"encoding/json"
	"strings"
	"time"

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

// https://help.victorops.com/knowledge-base/incident-fields-glossary/ - 20480 characters.
const victorOpsMaxMessageLenRunes = 20480

const (
	// victoropsAlertStateRecovery - VictorOps "RECOVERY" message type
	victoropsAlertStateRecovery = "RECOVERY"
)

func VictorOpsFactory(fc config.FactoryConfig) (NotificationChannel, error) {
	notifier, err := NewVictoropsNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return notifier, nil
}

// NewVictoropsNotifier creates an instance of VictoropsNotifier that
// handles posting notifications to Victorops REST API
func NewVictoropsNotifier(fc config.FactoryConfig) (*VictoropsNotifier, error) {
	settings, err := config.BuildVictorOpsConfig(fc)
	if err != nil {
		return nil, err
	}
	return &VictoropsNotifier{
		Base:       NewBase(fc.Config),
		log:        fc.Logger,
		images:     fc.ImageStore,
		ns:         fc.NotificationService,
		tmpl:       fc.Template,
		settings:   settings,
		appVersion: fc.GrafanaBuildVersion,
	}, nil
}

// VictoropsNotifier defines URL property for Victorops REST API
// and handles notification process by formatting POST body according to
// Victorops specifications (http://victorops.force.com/knowledgebase/articles/Integration/Alert-Ingestion-API-Documentation/)
type VictoropsNotifier struct {
	*Base
	log        log.Logger
	images     images.ImageStore
	ns         sender.WebhookSender
	tmpl       *template.Template
	settings   config.VictorOpsConfig
	appVersion string
}

// Notify sends notification to Victorops via POST to URL endpoint
func (vn *VictoropsNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	vn.log.Debug("sending notification", "notification", vn.Name)

	var tmplErr error
	tmpl, _ := template2.TmplText(ctx, vn.tmpl, as, vn.log, &tmplErr)

	messageType := buildMessageType(vn.log, tmpl, vn.settings.MessageType, as...)

	groupKey, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return false, err
	}

	stateMessage, truncated := TruncateInRunes(tmpl(vn.settings.Description), victorOpsMaxMessageLenRunes)
	if truncated {
		vn.log.Warn("Truncated stateMessage", "incident", groupKey, "max_runes", victorOpsMaxMessageLenRunes)
	}

	bodyJSON := map[string]interface{}{
		"message_type":        messageType,
		"entity_id":           groupKey.Hash(),
		"entity_display_name": tmpl(vn.settings.Title),
		"timestamp":           time.Now().Unix(),
		"state_message":       stateMessage,
		"monitoring_tool":     "Grafana v" + vn.appVersion,
	}

	if tmplErr != nil {
		vn.log.Warn("failed to expand message template. "+
			"", "error", tmplErr.Error())
		tmplErr = nil
	}

	_ = withStoredImages(ctx, vn.log, vn.images,
		func(index int, image images.Image) error {
			if image.URL != "" {
				bodyJSON["image_url"] = image.URL
				return images.ErrImagesDone
			}
			return nil
		}, as...)

	ruleURL := joinURLPath(vn.tmpl.ExternalURL.String(), "/alerting/list", vn.log)
	bodyJSON["alert_url"] = ruleURL

	u := tmpl(vn.settings.URL)
	if tmplErr != nil {
		vn.log.Info("failed to expand URL template", "error", tmplErr.Error(), "fallback", vn.settings.URL)
		u = vn.settings.URL
	}

	b, err := json.Marshal(bodyJSON)
	if err != nil {
		return false, err
	}
	cmd := &sender.SendWebhookSettings{
		URL:  u,
		Body: string(b),
	}

	if err := vn.ns.SendWebhook(ctx, cmd); err != nil {
		vn.log.Error("failed to send notification", "error", err, "webhook", vn.Name)
		return false, err
	}

	return true, nil
}

func (vn *VictoropsNotifier) SendResolved() bool {
	return !vn.GetDisableResolveMessage()
}

func buildMessageType(l log.Logger, tmpl func(string) string, msgType string, as ...*types.Alert) string {
	if types.Alerts(as...).Status() == model.AlertResolved {
		return victoropsAlertStateRecovery
	}
	if messageType := strings.ToUpper(tmpl(msgType)); messageType != "" {
		return messageType
	}
	l.Warn("expansion of message type template resulted in an empty string. Using fallback", "fallback", config.DefaultVictoropsMessageType, "template", msgType)
	return config.DefaultVictoropsMessageType
}
