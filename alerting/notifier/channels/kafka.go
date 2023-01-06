package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
)

type kafkaBody struct {
	Records []kafkaRecordEnvelope `json:"records"`
}

type kafkaRecordEnvelope struct {
	Value kafkaRecord `json:"value"`
}

type kafkaRecord struct {
	Description string         `json:"description"`
	Client      string         `json:"client,omitempty"`
	Details     string         `json:"details,omitempty"`
	AlertState  AlertStateType `json:"alert_state,omitempty"`
	ClientURL   string         `json:"client_url,omitempty"`
	Contexts    []kafkaContext `json:"contexts,omitempty"`
	IncidentKey string         `json:"incident_key,omitempty"`
}

type kafkaContext struct {
	Type   string `json:"type"`
	Source string `json:"src"`
}

// KafkaNotifier is responsible for sending
// alert notifications to Kafka.
type KafkaNotifier struct {
	*Base
	log      Logger
	images   ImageStore
	ns       WebhookSender
	tmpl     *template.Template
	settings *kafkaSettings
}

type kafkaSettings struct {
	Endpoint     string `json:"kafkaRestProxy,omitempty" yaml:"kafkaRestProxy,omitempty"`
	Topic        string `json:"kafkaTopic,omitempty" yaml:"kafkaTopic,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Details      string `json:"details,omitempty" yaml:"details,omitempty"`
	BasicAuth    bool   `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	PassFilePath string `json:"passwordFilePath,omitempty" yaml:"passwordFilePath,omitempty"`
	AuthUser     string `json:"user,omitempty" yaml:"user,omitempty"`
	AuthPass     string `json:"password,omitempty" yaml:"password,omitempty"`
}

func buildKafkaSettings(fc FactoryConfig) (*kafkaSettings, error) {
	var settings kafkaSettings
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.Endpoint == "" {
		return nil, errors.New("could not find kafka rest proxy endpoint property in settings")
	}
	if settings.Topic == "" {
		return nil, errors.New("could not find kafka topic property in settings")
	}
	if settings.Description == "" {
		settings.Description = DefaultMessageTitleEmbed
	}
	if settings.Details == "" {
		settings.Details = DefaultMessageEmbed
	}
	if settings.BasicAuth {
		if settings.AuthUser == "" {
			return nil, errors.New("if basic auth is enabled, user must be provided")
		}
		if settings.PassFilePath == "" {
			return nil, errors.New("if basic auth is enabled, password file path must be provided")
		}
		if err := settings.RefreshPassword(); err != nil {
			return nil, err
		}
	}
	return &settings, nil
}

func (ks *kafkaSettings) RefreshPassword() error {
	passwordBytes, err := os.ReadFile(ks.PassFilePath)
	if err != nil {
		return fmt.Errorf("failed to read password from file: %w", err)
	}
	ks.AuthPass = string(passwordBytes)
	return nil
}

func KafkaFactory(fc FactoryConfig) (NotificationChannel, error) {
	ch, err := newKafkaNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return ch, nil
}

// newKafkaNotifier is the constructor function for the Kafka notifier.
func newKafkaNotifier(fc FactoryConfig) (*KafkaNotifier, error) {
	settings, err := buildKafkaSettings(fc)
	if err != nil {
		return nil, err
	}

	return &KafkaNotifier{
		Base:     NewBase(fc.Config),
		log:      fc.Logger,
		images:   fc.ImageStore,
		ns:       fc.NotificationService,
		tmpl:     fc.Template,
		settings: settings,
	}, nil
}

// Notify sends the alert notification.
func (kn *KafkaNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	var tmplErr error
	tmpl, _ := TmplText(ctx, kn.tmpl, as, kn.log, &tmplErr)

	topicURL := strings.TrimRight(kn.settings.Endpoint, "/") + "/topics/" + tmpl(kn.settings.Topic)

	body, err := kn.buildBody(ctx, tmpl, as...)
	if err != nil {
		return false, err
	}

	if tmplErr != nil {
		kn.log.Warn("failed to template Kafka message", "error", tmplErr.Error())
	}

	cmd := &SendWebhookSettings{
		URL:        topicURL,
		Body:       body,
		HTTPMethod: "POST",
		HTTPHeader: map[string]string{
			"Content-Type": "application/vnd.kafka.json.v2+json",
			"Accept":       "application/vnd.kafka.v2+json",
		},
	}

	retryOnFailure := false
	if kn.settings.BasicAuth {
		cmd.User = kn.settings.AuthUser
		cmd.Password = kn.settings.AuthPass
		retryOnFailure = true
	}

	return kn.sendWebhookWithRetry(ctx, cmd, retryOnFailure)
}

func (kn *KafkaNotifier) sendWebhookWithRetry(ctx context.Context, cmd *SendWebhookSettings, retry bool) (bool, error) {
	if err := kn.ns.SendWebhook(ctx, cmd); err != nil {
		kn.log.Error("Failed to send notification to Kafka", "error", err, "body", cmd.Body)
		// No way to check if this was an auth error. Retry with a refreshed password
		if retry {
			kn.log.Debug("Retrying with a refreshed password")
			if err := kn.settings.RefreshPassword(); err != nil {
				return false, err
			}
			return kn.sendWebhookWithRetry(ctx, cmd, false)
		}
		return false, err
	}
	return true, nil
}

func (kn *KafkaNotifier) SendResolved() bool {
	return !kn.GetDisableResolveMessage()
}

func (kn *KafkaNotifier) buildBody(ctx context.Context, tmpl func(string) string, as ...*types.Alert) (string, error) {
	var record kafkaRecord
	record.Client = "Grafana"
	record.Description = tmpl(kn.settings.Description)
	record.Details = tmpl(kn.settings.Details)

	state := buildState(as...)
	kn.log.Debug("notifying Kafka", "alert_state", state)
	record.AlertState = state

	ruleURL := joinURLPath(kn.tmpl.ExternalURL.String(), "/alerting/list", kn.log)
	record.ClientURL = ruleURL

	contexts := buildContextImages(ctx, kn.log, kn.images, as...)
	if len(contexts) > 0 {
		record.Contexts = contexts
	}

	groupKey, err := notify.ExtractGroupKey(ctx)
	if err != nil {
		return "", err
	}
	record.IncidentKey = groupKey.Hash()

	records := kafkaBody{
		Records: []kafkaRecordEnvelope{
			{Value: record},
		},
	}

	body, err := json.Marshal(records)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func buildState(as ...*types.Alert) AlertStateType {
	// We are using the state from 7.x to not break kafka.
	// TODO: should we switch to the new ones?
	if types.Alerts(as...).Status() == model.AlertResolved {
		return AlertStateOK
	}
	return AlertStateAlerting
}

func buildContextImages(ctx context.Context, l Logger, imageStore ImageStore, as ...*types.Alert) []kafkaContext {
	var contexts []kafkaContext
	_ = withStoredImages(ctx, l, imageStore,
		func(_ int, image Image) error {
			if image.URL != "" {
				contexts = append(contexts, kafkaContext{
					Type:   "image",
					Source: image.URL,
				})
			}
			return nil
		}, as...)
	return contexts
}
