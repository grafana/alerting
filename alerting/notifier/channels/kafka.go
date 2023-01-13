package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type kafkaV3Record struct {
	Type string      `json:"type"`
	Data kafkaRecord `json:"data"`
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
	Endpoint       string `json:"kafkaRestProxy,omitempty" yaml:"kafkaRestProxy,omitempty"`
	Topic          string `json:"kafkaTopic,omitempty" yaml:"kafkaTopic,omitempty"`
	Description    string `json:"description,omitempty" yaml:"description,omitempty"`
	Details        string `json:"details,omitempty" yaml:"details,omitempty"`
	Username       string `json:"username,omitempty" yaml:"username,omitempty"`
	Password       string `json:"password,omitempty" yaml:"password,omitempty"`
	APIVersion     string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	KafkaClusterID string `json:"kafkaClusterId,omitempty" yaml:"kafkaClusterId,omitempty"`
}

// The user can choose which API version to use when sending
// messages to Kafka. The default is v2.
// Details on how these versions differ can be found here:
// https://docs.confluent.io/platform/current/kafka-rest/api.html
const (
	APIVersionV2 = "v2"
	APIVersionV3 = "v3"
)

func buildKafkaSettings(fc FactoryConfig) (*kafkaSettings, error) {
	var settings kafkaSettings
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.Endpoint == "" {
		return nil, errors.New("could not find kafka rest proxy endpoint property in settings")
	}
	settings.Endpoint = strings.TrimRight(settings.Endpoint, "/")

	if settings.Topic == "" {
		return nil, errors.New("could not find kafka topic property in settings")
	}
	if settings.Description == "" {
		settings.Description = DefaultMessageTitleEmbed
	}
	if settings.Details == "" {
		settings.Details = DefaultMessageEmbed
	}
	settings.Password = fc.DecryptFunc(context.Background(), fc.Config.SecureSettings, "password", settings.Password)

	if settings.APIVersion == "" {
		settings.APIVersion = APIVersionV2
	} else if settings.APIVersion == APIVersionV3 {
		if settings.KafkaClusterID == "" {
			return nil, errors.New("kafka cluster id must be provided when using api version 3")
		}
	} else if settings.APIVersion != APIVersionV2 && settings.APIVersion != APIVersionV3 {
		return nil, fmt.Errorf("unsupported api version: %s", settings.APIVersion)
	}
	return &settings, nil
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
	if kn.settings.APIVersion == APIVersionV3 {
		return kn.notifyWithAPIV3(ctx, as...)
	}
	return kn.notifyWithAPIV2(ctx, as...)
}

// Use the v2 API to send the alert notification.
func (kn *KafkaNotifier) notifyWithAPIV2(ctx context.Context, as ...*types.Alert) (bool, error) {
	var tmplErr error
	tmpl, _ := TmplText(ctx, kn.tmpl, as, kn.log, &tmplErr)

	topicURL := kn.settings.Endpoint + "/topics/" + tmpl(kn.settings.Topic)
	if tmplErr != nil {
		kn.log.Warn("failed to template Kafka url", "error", tmplErr.Error())
	}

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
		User:     kn.settings.Username,
		Password: kn.settings.Password,
	}

	if err := kn.ns.SendWebhook(ctx, cmd); err != nil {
		kn.log.Error("Failed to send notification to Kafka", "error", err, "body", body)
		return false, err
	}
	return true, nil
}

// Use the v3 API to send the alert notification.
func (kn *KafkaNotifier) notifyWithAPIV3(ctx context.Context, as ...*types.Alert) (bool, error) {
	var tmplErr error
	tmpl, _ := TmplText(ctx, kn.tmpl, as, kn.log, &tmplErr)

	// For v3 the Produce URL is like this,
	// <Endpoint>/v3/clusters/<KafkaClusterID>/topics/<Topic>/records
	topicURL := kn.settings.Endpoint + "/v3/clusters/" + tmpl(kn.settings.KafkaClusterID) + "/topics/" + tmpl(kn.settings.Topic) + "/records"
	if tmplErr != nil {
		kn.log.Warn("failed to template Kafka url", "error", tmplErr.Error())
	}

	body, err := kn.buildV3Body(ctx, tmpl, as...)
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
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		Validation: validateKafkaV3Response,
		User:       kn.settings.Username,
		Password:   kn.settings.Password,
	}

	// TODO: Convert to a stream - keep a single connection open and send records on it.
	// Can be implemented nicely using channels. The v3 API can be used in streaming mode
	// by setting “Transfer-Encoding: chunked” header.
	// For as long as the connection is kept open, the server will keep accepting records.
	if err := kn.ns.SendWebhook(ctx, cmd); err != nil {
		kn.log.Error("Failed to send notification to Kafka", "error", err, "body", body)
		return false, err
	}
	return true, nil
}

/*
A sample of V3 response looks like this,

	{
		"error_code": 200,
		"cluster_id": "lkc-abcd",
		"topic_name": "myTopic",
		"partition_id": 5,
		"offset": 0,
		"timestamp": "2023-01-08T11:21:48.031Z",
		"value": { "type" : "JSON", "size" : 14 }
	}
*/
type kafkaV3Response struct {
	ErrorCode int `json:"error_code"`
}

func validateKafkaV3Response(rawResponse []byte, statusCode int) error {
	if statusCode/100 != 2 {
		return fmt.Errorf("unexpected status code %d", statusCode)
	}
	// 200 status means the API was processed successfully.
	// The message publishing could still fail. This is verified by checking the error_code field in the response.
	var response kafkaV3Response
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		return err
	}
	if response.ErrorCode/100 != 2 {
		return fmt.Errorf("failed to publish message to Kafka. response: %s", string(rawResponse))
	}
	return nil
}

func (kn *KafkaNotifier) SendResolved() bool {
	return !kn.GetDisableResolveMessage()
}

func (kn *KafkaNotifier) buildBody(ctx context.Context, tmpl func(string) string, as ...*types.Alert) (string, error) {
	if kn.settings.APIVersion == APIVersionV3 {
		return kn.buildV3Body(ctx, tmpl, as...)
	}
	return kn.buildV2Body(ctx, tmpl, as...)
}

func (kn *KafkaNotifier) buildV2Body(ctx context.Context, tmpl func(string) string, as ...*types.Alert) (string, error) {
	var record kafkaRecord
	if err := kn.buildKafkaRecord(ctx, &record, tmpl, as...); err != nil {
		return "", err
	}
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

func (kn *KafkaNotifier) buildV3Body(ctx context.Context, tmpl func(string) string, as ...*types.Alert) (string, error) {
	var record kafkaRecord
	if err := kn.buildKafkaRecord(ctx, &record, tmpl, as...); err != nil {
		return "", err
	}
	records := map[string]kafkaV3Record{
		"value": {
			Type: "JSON",
			Data: record,
		},
	}
	body, err := json.Marshal(records)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (kn *KafkaNotifier) buildKafkaRecord(ctx context.Context, record *kafkaRecord, tmpl func(string) string, as ...*types.Alert) error {
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
		return err
	}
	record.IncidentKey = groupKey.Hash()
	return nil
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
