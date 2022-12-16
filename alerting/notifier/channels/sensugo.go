package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/notifications"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
)

type SensuGoNotifier struct {
	*Base
	log  log.Logger
	ns   notifications.WebhookSender
	tmpl *template.Template

	URL       string
	Entity    string
	Check     string
	Namespace string
	Handler   string
	APIKey    string
	Message   string
}

type SensuGoConfig struct {
	*NotificationChannelConfig
	URL       string
	Entity    string
	Check     string
	Namespace string
	Handler   string
	APIKey    string
	Message   string
}

func SensuGoFactory(fc FactoryConfig) (NotificationChannel, error) {
	cfg, err := NewSensuGoConfig(fc.Config, fc.DecryptFunc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return NewSensuGoNotifier(cfg, fc.NotificationService, fc.Template), nil
}

func NewSensuGoConfig(config *NotificationChannelConfig, decryptFunc GetDecryptedValueFn) (*SensuGoConfig, error) {
	url := config.Settings.Get("url").MustString()
	if url == "" {
		return nil, errors.New("could not find URL property in settings")
	}
	apikey := decryptFunc(context.Background(), config.SecureSettings, "apikey", config.Settings.Get("apikey").MustString())
	if apikey == "" {
		return nil, errors.New("could not find the API key property in settings")
	}
	return &SensuGoConfig{
		NotificationChannelConfig: config,
		URL:                       url,
		Entity:                    config.Settings.Get("entity").MustString(),
		Check:                     config.Settings.Get("check").MustString(),
		Namespace:                 config.Settings.Get("namespace").MustString(),
		Handler:                   config.Settings.Get("handler").MustString(),
		APIKey:                    apikey,
		Message:                   config.Settings.Get("message").MustString(`{{ template "default.message" .}}`),
	}, nil
}

// NewSensuGoNotifier is the constructor for the SensuGo notifier
func NewSensuGoNotifier(config *SensuGoConfig, ns notifications.WebhookSender, t *template.Template) *SensuGoNotifier {
	return &SensuGoNotifier{
		Base: NewBase(&models.AlertNotification{
			Uid:                   config.UID,
			Name:                  config.Name,
			Type:                  config.Type,
			DisableResolveMessage: config.DisableResolveMessage,
			Settings:              config.Settings,
			SecureSettings:        config.SecureSettings,
		}),
		URL:       config.URL,
		Entity:    config.Entity,
		Check:     config.Check,
		Namespace: config.Namespace,
		Handler:   config.Handler,
		APIKey:    config.APIKey,
		Message:   config.Message,
		log:       log.New("alerting.notifier.sensugo"),
		ns:        ns,
		tmpl:      t,
	}
}

// Notify sends an alert notification to Sensu Go
func (sn *SensuGoNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	sn.log.Debug("Sending Sensu Go result")

	var tmplErr error
	tmpl, _ := TmplText(ctx, sn.tmpl, as, sn.log, &tmplErr)

	// Sensu Go alerts require an entity and a check. We set it to the user-specified
	// value (optional), else we fallback and use the grafana rule anme  and ruleID.
	entity := tmpl(sn.Entity)
	if entity == "" {
		entity = "default"
	}

	check := tmpl(sn.Check)
	if check == "" {
		check = "default"
	}

	alerts := types.Alerts(as...)
	status := 0
	if alerts.Status() == model.AlertFiring {
		// TODO figure out about NoData old state (we used to send status 1 in that case)
		status = 2
	}

	namespace := tmpl(sn.Namespace)
	if namespace == "" {
		namespace = "default"
	}

	var handlers []string
	if sn.Handler != "" {
		handlers = []string{tmpl(sn.Handler)}
	}

	ruleURL := joinUrlPath(sn.tmpl.ExternalURL.String(), "/alerting/list", sn.log)
	bodyMsgType := map[string]interface{}{
		"entity": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      entity,
				"namespace": namespace,
			},
		},
		"check": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": check,
				"labels": map[string]string{
					"ruleURL": ruleURL,
				},
			},
			"output":   tmpl(sn.Message),
			"issued":   timeNow().Unix(),
			"interval": 86400,
			"status":   status,
			"handlers": handlers,
		},
		"ruleUrl": ruleURL,
	}

	if tmplErr != nil {
		sn.log.Warn("failed to template sensugo message", "err", tmplErr.Error())
	}

	body, err := json.Marshal(bodyMsgType)
	if err != nil {
		return false, err
	}

	cmd := &models.SendWebhookSync{
		Url:        fmt.Sprintf("%s/api/core/v2/namespaces/%s/events", strings.TrimSuffix(sn.URL, "/"), namespace),
		Body:       string(body),
		HttpMethod: "POST",
		HttpHeader: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": fmt.Sprintf("Key %s", sn.APIKey),
		},
	}
	if err := sn.ns.SendWebhookSync(ctx, cmd); err != nil {
		sn.log.Error("Failed to send Sensu Go event", "error", err, "sensugo", sn.Name)
		return false, err
	}

	return true, nil
}

func (sn *SensuGoNotifier) SendResolved() bool {
	return !sn.GetDisableResolveMessage()
}
