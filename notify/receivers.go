package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/prometheus/alertmanager/dispatch"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/definition"
	"github.com/grafana/alerting/http"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
)

const (
	maxTestReceiversWorkers = 10
)

var (
	ErrNoReceivers = errors.New("no receivers")
)

type TestReceiversResult struct {
	Alert     types.Alert          `json:"alert"`
	Receivers []TestReceiverResult `json:"receivers"`
	NotifedAt time.Time            `json:"notifiedAt"`
}

type TestReceiverResult struct {
	Name    string                        `json:"name"`
	Configs []TestIntegrationConfigResult `json:"configs"`
}

type TestIntegrationConfigResult struct {
	Name   string `json:"name"`
	UID    string `json:"uid"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

type ConfigReceiver = definition.Receiver

type TestReceiversConfigBodyParams struct {
	Alert     *models.TestReceiversConfigAlertParams `yaml:"alert,omitempty" json:"alert,omitempty"`
	Receivers []models.ReceiverConfig                `yaml:"receivers,omitempty" json:"receivers,omitempty"`
}

type IntegrationTimeoutError struct {
	Integration *models.IntegrationConfig
	Err         error
}

func (e IntegrationTimeoutError) Error() string {
	return fmt.Sprintf("the receiver timed out: %s", e.Err)
}

func (am *GrafanaAlertmanager) TestReceivers(ctx context.Context, c TestReceiversConfigBodyParams) (*TestReceiversResult, int, error) {
	am.reloadConfigMtx.RLock()
	templates := am.templates
	am.reloadConfigMtx.RUnlock()

	return TestReceivers(ctx, c, am.buildReceiverIntegrations, templates)
}

func newTestAlert(c *models.TestReceiversConfigAlertParams, startsAt, updatedAt time.Time) types.Alert {
	var (
		defaultAnnotations = model.LabelSet{
			"summary":          "Notification test",
			"__value_string__": "[ metric='foo' labels={instance=bar} value=10 ]",
		}
		defaultLabels = model.LabelSet{
			"alertname": "TestAlert",
			"instance":  "Grafana",
		}
	)

	alert := types.Alert{
		Alert: model.Alert{
			Labels:      defaultLabels,
			Annotations: defaultAnnotations,
			StartsAt:    startsAt,
		},
		UpdatedAt: updatedAt,
	}

	if c == nil {
		return alert
	}
	if c.Annotations != nil {
		for k, v := range c.Annotations {
			alert.Annotations[k] = v
		}
	}
	if c.Labels != nil {
		for k, v := range c.Labels {
			alert.Labels[k] = v
		}
	}
	return alert
}

func ProcessIntegrationError(config *models.IntegrationConfig, err error) error {
	if err == nil {
		return nil
	}

	var urlError *url.Error
	if errors.As(err, &urlError) {
		if urlError.Timeout() {
			return IntegrationTimeoutError{
				Integration: config,
				Err:         err,
			}
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return IntegrationTimeoutError{
			Integration: config,
			Err:         err,
		}
	}

	return err
}

// DecodeSecretsFn is a function used to decode a map of secrets before creating a receiver.
type DecodeSecretsFn func(secrets map[string]string) (map[string][]byte, error)

// DecodeSecretsFromBase64 is a DecodeSecretsFn that base64-decodes a map of secrets.
func DecodeSecretsFromBase64(secrets map[string]string) (map[string][]byte, error) {
	secureSettings := make(map[string][]byte, len(secrets))
	if secrets == nil {
		return secureSettings, nil
	}
	for k, v := range secrets {
		d, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("failed to decode secure settings key %s: %w", k, err)
		}
		secureSettings[k] = d
	}
	return secureSettings, nil
}

// NoopDecode is a DecodeSecretsFn that converts a map[string]string into a map[string][]byte without decoding it.
func NoopDecode(secrets map[string]string) (map[string][]byte, error) {
	secureSettings := make(map[string][]byte, len(secrets))
	if secrets == nil {
		return secureSettings, nil
	}

	for k, v := range secrets {
		secureSettings[k] = []byte(v)
	}
	return secureSettings, nil
}

// GetDecryptedValueFn is a function that returns the decrypted value of
// the given key. If the key is not present, then it returns the fallback value.
type GetDecryptedValueFn func(ctx context.Context, sjd map[string][]byte, key string, fallback string) string

// NoopDecrypt is a GetDecryptedValueFn that returns a value without decrypting it.
func NoopDecrypt(_ context.Context, sjd map[string][]byte, key string, fallback string) string {
	if v, ok := sjd[key]; ok {
		return string(v)
	}
	return fallback
}

func ValidateAPIReceiver(ctx context.Context, api models.ReceiverConfig, decode DecodeSecretsFn, decrypt GetDecryptedValueFn) error {
	var errs []error
	if api.Name == "" {
		errs = append(errs, fmt.Errorf("receiver name is required"))
	}
	for idx, integration := range api.Integrations {
		err := ValidateIntegrationConfig(ctx, integration, decode, decrypt)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid integration config at index %d: %w", idx, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func ValidateIntegrationConfig(ctx context.Context, cfg *models.IntegrationConfig, decode DecodeSecretsFn, decrypt GetDecryptedValueFn) error {
	if cfg.Type == "" {
		return fmt.Errorf("type should not be an empty string")
	}
	if cfg.Settings == nil {
		return fmt.Errorf("settings should not be empty")
	}
	if cfg.Version == "" {
		return fmt.Errorf("version should not be an empty string")
	}

	secureSettings, err := decode(cfg.SecureSettings)
	if err != nil {
		return err
	}

	decryptFn := func(key string, fallback string) (string, bool) {
		if _, ok := secureSettings[key]; !ok {
			return fallback, false
		}
		return decrypt(ctx, secureSettings, key, fallback), true
	}

	factory, ok := GetFactoryForIntegrationVersion(cfg.Type, cfg.Version)
	if !ok {
		return fmt.Errorf("invalid integration type or version: %s %s", cfg.Type, cfg.Version)
	}
	return factory.ValidateConfig(cfg.Settings, decryptFn)
}

// GetActiveReceiversMap returns all receivers that are in use by a route.
func GetActiveReceiversMap(r *dispatch.Route) map[string]struct{} {
	receiversMap := make(map[string]struct{})
	visitFunc := func(r *dispatch.Route) {
		receiversMap[r.RouteOpts.Receiver] = struct{}{}
	}
	r.Walk(visitFunc)

	return receiversMap
}

func parseHTTPConfig(integration *models.IntegrationConfig, decryptFn receivers.DecryptFunc) (*http.HTTPClientConfig, error) {
	if integration.Version != schema.V1 {
		return nil, nil
	}
	httpConfigSettings := struct {
		HTTPConfig *http.HTTPClientConfig `yaml:"http_config,omitempty" json:"http_config,omitempty"`
	}{}
	if err := json.Unmarshal(integration.Settings, &httpConfigSettings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal http_config settings: %w", err)
	}

	if httpConfigSettings.HTTPConfig == nil {
		return nil, nil
	}

	httpConfigSettings.HTTPConfig.Decrypt(decryptFn)
	if err := http.ValidateHTTPClientConfig(httpConfigSettings.HTTPConfig); err != nil {
		return nil, fmt.Errorf("invalid HTTP client configuration: %w", err)
	}
	return httpConfigSettings.HTTPConfig, nil
}

type IntegrationValidationError struct {
	Err         error
	Integration *models.IntegrationConfig
}

func (e IntegrationValidationError) Error() string {
	name := ""
	if e.Integration.Name != "" {
		name = fmt.Sprintf("%q ", e.Integration.Name)
	}
	s := fmt.Sprintf("failed to validate integration %s(UID %s) of type %q: %s", name, e.Integration.UID, e.Integration.Type, e.Err.Error())
	return s
}

func (e IntegrationValidationError) Unwrap() error { return e.Err }

type MimirIntegrationConfig struct {
	Schema schema.IntegrationSchemaVersion
	Config any
}

// ConfigJSON returns the JSON representation of the integration config with non-masked secrets.
func (c MimirIntegrationConfig) ConfigJSON() ([]byte, error) {
	return definition.MarshalJSONWithSecrets(c.Config)
}

func (c MimirIntegrationConfig) ConfigMap() (map[string]any, error) {
	data, err := c.ConfigJSON()
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
