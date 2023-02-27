package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"

	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/alertmanager"
	"github.com/grafana/alerting/receivers/dinding"
	"github.com/grafana/alerting/receivers/discord"
	"github.com/grafana/alerting/receivers/email"
	"github.com/grafana/alerting/receivers/googlechat"
	"github.com/grafana/alerting/receivers/kafka"
	"github.com/grafana/alerting/receivers/line"
	"github.com/grafana/alerting/receivers/opsgenie"
	"github.com/grafana/alerting/receivers/pagerduty"
	"github.com/grafana/alerting/receivers/pushover"
	"github.com/grafana/alerting/receivers/sensugo"
	"github.com/grafana/alerting/receivers/slack"
	"github.com/grafana/alerting/receivers/teams"
	"github.com/grafana/alerting/receivers/telegram"
	"github.com/grafana/alerting/receivers/threema"
	"github.com/grafana/alerting/receivers/victorops"
	"github.com/grafana/alerting/receivers/webex"
	"github.com/grafana/alerting/receivers/webhook"
	"github.com/grafana/alerting/receivers/wecom"
)

const (
	maxTestReceiversWorkers = 10
)

var (
	ErrNoReceivers = errors.New("no receivers")
)

type TestReceiversResult struct {
	Alert     types.Alert
	Receivers []TestReceiverResult
	NotifedAt time.Time
}

type TestReceiverResult struct {
	Name    string
	Configs []TestReceiverConfigResult
}

type TestReceiverConfigResult struct {
	Name   string
	UID    string
	Status string
	Error  error
}

type InvalidReceiverError struct {
	Receiver *GrafanaReceiver
	Err      error
}

type GrafanaReceiver struct {
	UID                   string            `json:"uid"`
	Name                  string            `json:"name"`
	Type                  string            `json:"type"`
	DisableResolveMessage bool              `json:"disableResolveMessage"`
	Settings              json.RawMessage   `json:"settings"`
	SecureSettings        map[string]string `json:"secureSettings"`
}

type ConfigReceiver = config.Receiver

type APIReceiver struct {
	ConfigReceiver   `yaml:",inline"`
	GrafanaReceivers `yaml:",inline"`
}

type GrafanaReceivers struct {
	Receivers []*GrafanaReceiver `yaml:"grafana_managed_receiver_configs,omitempty" json:"grafana_managed_receiver_configs,omitempty"`
}

type TestReceiversConfigBodyParams struct {
	Alert     *TestReceiversConfigAlertParams `yaml:"alert,omitempty" json:"alert,omitempty"`
	Receivers []*APIReceiver                  `yaml:"receivers,omitempty" json:"receivers,omitempty"`
}

type TestReceiversConfigAlertParams struct {
	Annotations model.LabelSet `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Labels      model.LabelSet `yaml:"labels,omitempty" json:"labels,omitempty"`
}

func (e InvalidReceiverError) Error() string {
	return fmt.Sprintf("the receiver is invalid: %s", e.Err)
}

type ReceiverTimeoutError struct {
	Receiver *GrafanaReceiver
	Err      error
}

func (e ReceiverTimeoutError) Error() string {
	return fmt.Sprintf("the receiver timed out: %s", e.Err)
}

func (am *GrafanaAlertmanager) TestReceivers(ctx context.Context, c TestReceiversConfigBodyParams) (*TestReceiversResult, error) {
	// now represents the start time of the test
	now := time.Now()
	testAlert := newTestAlert(c, now, now)

	// we must set a group key that is unique per test as some receivers use this key to deduplicate alerts
	ctx = notify.WithGroupKey(ctx, testAlert.Labels.String()+now.String())

	tmpl, err := am.getTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// job contains all metadata required to test a receiver
	type job struct {
		Config       *GrafanaReceiver
		ReceiverName string
		Notifier     notify.Notifier
	}

	// result contains the receiver that was tested and an error that is non-nil if the test failed
	type result struct {
		Config       *GrafanaReceiver
		ReceiverName string
		Error        error
	}

	newTestReceiversResult := func(alert types.Alert, results []result, notifiedAt time.Time) *TestReceiversResult {
		m := make(map[string]TestReceiverResult)
		for _, receiver := range c.Receivers {
			// set up the result for this receiver
			m[receiver.Name] = TestReceiverResult{
				Name: receiver.Name,
				// A Grafana receiver can have multiple nested receivers
				Configs: make([]TestReceiverConfigResult, 0, len(receiver.Receivers)),
			}
		}
		for _, next := range results {
			tmp := m[next.ReceiverName]
			status := "ok"
			if next.Error != nil {
				status = "failed"
			}
			tmp.Configs = append(tmp.Configs, TestReceiverConfigResult{
				Name:   next.Config.Name,
				UID:    next.Config.UID,
				Status: status,
				Error:  ProcessNotifierError(next.Config, next.Error),
			})
			m[next.ReceiverName] = tmp
		}
		v := new(TestReceiversResult)
		v.Alert = alert
		v.Receivers = make([]TestReceiverResult, 0, len(c.Receivers))
		v.NotifedAt = notifiedAt
		for _, next := range m {
			v.Receivers = append(v.Receivers, next)
		}

		// Make sure the return order is deterministic.
		sort.Slice(v.Receivers, func(i, j int) bool {
			return v.Receivers[i].Name < v.Receivers[j].Name
		})

		return v
	}

	// invalid keeps track of all invalid receiver configurations
	invalid := make([]result, 0, len(c.Receivers))
	// jobs keeps track of all receivers that need to be sent test notifications
	jobs := make([]job, 0, len(c.Receivers))

	for _, receiver := range c.Receivers {
		for _, next := range receiver.Receivers {
			n, err := am.buildReceiverIntegration(next, tmpl)
			if err != nil {
				invalid = append(invalid, result{
					Config:       next,
					ReceiverName: next.Name,
					Error:        err,
				})
			} else {
				jobs = append(jobs, job{
					Config:       next,
					ReceiverName: receiver.Name,
					Notifier:     n,
				})
			}
		}
	}

	if len(invalid)+len(jobs) == 0 {
		return nil, ErrNoReceivers
	}

	if len(jobs) == 0 {
		return newTestReceiversResult(testAlert, invalid, now), nil
	}

	numWorkers := maxTestReceiversWorkers
	if numWorkers > len(jobs) {
		numWorkers = len(jobs)
	}

	resultCh := make(chan result, len(jobs))
	workCh := make(chan job, len(jobs))
	for _, job := range jobs {
		workCh <- job
	}
	close(workCh)

	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < numWorkers; i++ {
		g.Go(func() error {
			for next := range workCh {
				v := result{
					Config:       next.Config,
					ReceiverName: next.ReceiverName,
				}
				if _, err := next.Notifier.Notify(ctx, &testAlert); err != nil {
					v.Error = err
				}
				resultCh <- v
			}
			return nil
		})
	}
	err = g.Wait() // nolint
	close(resultCh)

	if err != nil {
		return nil, err
	}

	results := make([]result, 0, len(jobs))
	for next := range resultCh {
		results = append(results, next)
	}

	return newTestReceiversResult(testAlert, append(invalid, results...), now), nil
}

func newTestAlert(c TestReceiversConfigBodyParams, startsAt, updatedAt time.Time) types.Alert {
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

	if c.Alert != nil {
		if c.Alert.Annotations != nil {
			for k, v := range c.Alert.Annotations {
				alert.Annotations[k] = v
			}
		}
		if c.Alert.Labels != nil {
			for k, v := range c.Alert.Labels {
				alert.Labels[k] = v
			}
		}
	}

	return alert
}

func ProcessNotifierError(config *GrafanaReceiver, err error) error {
	if err == nil {
		return nil
	}

	var urlError *url.Error
	if errors.As(err, &urlError) {
		if urlError.Timeout() {
			return ReceiverTimeoutError{
				Receiver: config,
				Err:      err,
			}
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ReceiverTimeoutError{
			Receiver: config,
			Err:      err,
		}
	}

	return err
}

// GrafanaReceiverTyped represents a parsed and validated APIReceiver
type GrafanaReceiverTyped struct {
	Name                string
	AlertmanagerConfigs []*NotifierConfig[alertmanager.Config]
	DingdingConfigs     []*NotifierConfig[dinding.Config]
	DiscordConfigs      []*NotifierConfig[discord.Config]
	EmailConfigs        []*NotifierConfig[email.Config]
	GooglechatConfigs   []*NotifierConfig[googlechat.Config]
	KafkaConfigs        []*NotifierConfig[kafka.Config]
	LineConfigs         []*NotifierConfig[line.Config]
	OpsgenieConfigs     []*NotifierConfig[opsgenie.Config]
	PagerdutyConfigs    []*NotifierConfig[pagerduty.Config]
	PushoverConfigs     []*NotifierConfig[pushover.Config]
	SensugoConfigs      []*NotifierConfig[sensugo.Config]
	SlackConfigs        []*NotifierConfig[slack.Config]
	TeamsConfigs        []*NotifierConfig[teams.Config]
	TelegramConfigs     []*NotifierConfig[telegram.Config]
	ThreemaConfigs      []*NotifierConfig[threema.Config]
	VictoropsConfigs    []*NotifierConfig[victorops.Config]
	WebhookConfigs      []*NotifierConfig[webhook.Config]
	WecomConfigs        []*NotifierConfig[wecom.Config]
	WebexConfigs        []*NotifierConfig[webex.Config]
}

// NotifierConfig represents parsed GrafanaReceiver.
type NotifierConfig[T interface{}] struct {
	receivers.Metadata
	Settings T
}

// ValidateAPIReceiver parses, decrypts and validates the APIReceiver. GrafanaReceiverTyped that contains configurations of all notifiers configurations for this receiver
func ValidateAPIReceiver(ctx context.Context, api *APIReceiver, decrypt receivers.GetDecryptedValueFn) (GrafanaReceiverTyped, error) {
	result := GrafanaReceiverTyped{
		Name: api.Name,
	}
	parseConfig := func(receiver *GrafanaReceiver) error {
		// secure settings are already encrypted at this point
		secureSettings := make(map[string][]byte, len(receiver.SecureSettings))

		if receiver.SecureSettings != nil {
			for k, v := range receiver.SecureSettings {
				d, err := base64.StdEncoding.DecodeString(v)
				if err != nil {
					return InvalidReceiverError{
						Receiver: receiver,
						Err:      errors.New("failed to decode secure setting"),
					}
				}
				secureSettings[k] = d
			}
		}

		var decryptFn receivers.DecryptFunc = func(key string, fallback string) string {
			return decrypt(ctx, secureSettings, key, fallback)
		}

		switch strings.ToLower(receiver.Type) {
		case "prometheus-alertmanager":
			return createReceiver[alertmanager.Config](receiver)(alertmanager.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[alertmanager.Config]) {
				result.AlertmanagerConfigs = append(result.AlertmanagerConfigs, f)
			})
		case "dingding":
			return createReceiver[dinding.Config](receiver)(dinding.NewConfig(receiver.Settings))(func(f *NotifierConfig[dinding.Config]) {
				result.DingdingConfigs = append(result.DingdingConfigs, f)
			})
		case "discord":
			return createReceiver[discord.Config](receiver)(discord.NewConfig(receiver.Settings))(func(f *NotifierConfig[discord.Config]) {
				result.DiscordConfigs = append(result.DiscordConfigs, f)
			})
		case "email":
			return createReceiver[email.Config](receiver)(email.NewConfig(receiver.Settings))(func(f *NotifierConfig[email.Config]) {
				result.EmailConfigs = append(result.EmailConfigs, f)
			})
		case "googlechat":
			return createReceiver[googlechat.Config](receiver)(googlechat.NewConfig(receiver.Settings))(func(f *NotifierConfig[googlechat.Config]) {
				result.GooglechatConfigs = append(result.GooglechatConfigs, f)
			})
		case "kafka":
			return createReceiver[kafka.Config](receiver)(kafka.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[kafka.Config]) {
				result.KafkaConfigs = append(result.KafkaConfigs, f)
			})
		case "line":
			return createReceiver[line.Config](receiver)(line.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[line.Config]) {
				result.LineConfigs = append(result.LineConfigs, f)
			})
		case "opsgenie":
			return createReceiver[opsgenie.Config](receiver)(opsgenie.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[opsgenie.Config]) {
				result.OpsgenieConfigs = append(result.OpsgenieConfigs, f)
			})
		case "pagerduty":
			return createReceiver[pagerduty.Config](receiver)(pagerduty.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[pagerduty.Config]) {
				result.PagerdutyConfigs = append(result.PagerdutyConfigs, f)
			})
		case "pushover":
			return createReceiver[pushover.Config](receiver)(pushover.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[pushover.Config]) {
				result.PushoverConfigs = append(result.PushoverConfigs, f)
			})
		case "sensugo":
			return createReceiver[sensugo.Config](receiver)(sensugo.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[sensugo.Config]) {
				result.SensugoConfigs = append(result.SensugoConfigs, f)
			})
		case "slack":
			return createReceiver[slack.Config](receiver)(slack.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[slack.Config]) {
				result.SlackConfigs = append(result.SlackConfigs, f)
			})
		case "teams":
			return createReceiver[teams.Config](receiver)(teams.NewConfig(receiver.Settings))(func(f *NotifierConfig[teams.Config]) {
				result.TeamsConfigs = append(result.TeamsConfigs, f)
			})
		case "telegram":
			return createReceiver[telegram.Config](receiver)(telegram.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[telegram.Config]) {
				result.TelegramConfigs = append(result.TelegramConfigs, f)
			})
		case "threema":
			return createReceiver[threema.Config](receiver)(threema.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[threema.Config]) {
				result.ThreemaConfigs = append(result.ThreemaConfigs, f)
			})
		case "victorops":
			return createReceiver[victorops.Config](receiver)(victorops.NewConfig(receiver.Settings))(func(f *NotifierConfig[victorops.Config]) {
				result.VictoropsConfigs = append(result.VictoropsConfigs, f)
			})
		case "webhook":
			return createReceiver[webhook.Config](receiver)(webhook.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[webhook.Config]) {
				result.WebhookConfigs = append(result.WebhookConfigs, f)
			})
		case "wecom":
			return createReceiver[wecom.Config](receiver)(wecom.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[wecom.Config]) {
				result.WecomConfigs = append(result.WecomConfigs, f)
			})
		case "webex":
			return createReceiver[webex.Config](receiver)(webex.NewConfig(receiver.Settings, decryptFn))(func(f *NotifierConfig[webex.Config]) {
				result.WebexConfigs = append(result.WebexConfigs, f)
			})
		default:
			return fmt.Errorf("notifier %s is not supported", receiver.Type)
		}
	}
	for _, receiver := range api.Receivers {
		err := parseConfig(receiver)
		if err != nil {
			return GrafanaReceiverTyped{}, &ReceiverValidationError{
				Cfg: receiver,
				Err: err,
			}
		}
	}
	return result, nil
}

func createReceiver[T interface{}](receiver *GrafanaReceiver) func(cfg T, err error) func(f func(f *NotifierConfig[T])) error {
	return func(settings T, err error) func(f func(f *NotifierConfig[T])) error {
		return func(f func(f *NotifierConfig[T])) error {
			if err != nil {
				return err
			}
			r := &NotifierConfig[T]{
				NotifierInfo: receivers.NotifierInfo{
					UID:                   receiver.UID,
					Name:                  receiver.Name,
					Type:                  receiver.Type,
					DisableResolveMessage: receiver.DisableResolveMessage,
				},
				Settings: settings,
			}
			f(r)
			return nil
		}
	}
}

type ReceiverValidationError struct {
	Err error
	Cfg *GrafanaReceiver
}

func (e ReceiverValidationError) Error() string {
	name := ""
	if e.Cfg.Name != "" {
		name = fmt.Sprintf("%q ", e.Cfg.Name)
	}
	s := fmt.Sprintf("failed to validate receiver %sof type %q: %s", name, e.Cfg.Type, e.Err.Error())
	return s
}

func (e ReceiverValidationError) Unwrap() error { return e.Err }
