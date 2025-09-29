package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/notify/notifytest"
	receiversTesting "github.com/grafana/alerting/receivers/testing"
	"github.com/grafana/alerting/templates"
)

func ptr[T any](v T) *T {
	return &v
}

func newFakeMaintanenceOptions(t *testing.T) *fakeMaintenanceOptions {
	t.Helper()

	return &fakeMaintenanceOptions{}
}

type fakeMaintenanceOptions struct {
}

func (f *fakeMaintenanceOptions) InitialState() string {
	return ""
}

func (f *fakeMaintenanceOptions) Retention() time.Duration {
	return 30 * time.Millisecond
}

func (f *fakeMaintenanceOptions) MaintenanceFrequency() time.Duration {
	return 15 * time.Millisecond
}

func (f *fakeMaintenanceOptions) MaintenanceFunc(_ State) (int64, error) {
	return 0, nil
}

type FakeConfig struct {
}

func (f *FakeConfig) DispatcherLimits() DispatcherLimits {
	panic("implement me")
}

func (f *FakeConfig) InhibitRules() []*InhibitRule {
	// TODO implement me
	panic("implement me")
}

func (f *FakeConfig) MuteTimeIntervals() []MuteTimeInterval {
	// TODO implement me
	panic("implement me")
}

func (f *FakeConfig) ReceiverIntegrations() (map[string][]Integration, error) {
	// TODO implement me
	panic("implement me")
}

func (f *FakeConfig) RoutingTree() *Route {
	// TODO implement me
	panic("implement me")
}

func (f *FakeConfig) Templates() *templates.Template {
	// TODO implement me
	panic("implement me")
}

func (f *FakeConfig) Hash() [16]byte {
	// TODO implement me
	panic("implement me")
}

func (f *FakeConfig) Raw() []byte {
	// TODO implement me
	panic("implement me")
}

type fakeNotifier struct{}

func (f *fakeNotifier) Notify(_ context.Context, _ ...*types.Alert) (bool, error) {
	return true, nil
}

func (f *fakeNotifier) SendResolved() bool {
	return true
}

func GetDecryptedValueFnForTesting(_ context.Context, sjd map[string][]byte, key string, fallback string) string {
	return receiversTesting.DecryptForTesting(sjd)(key, fallback)
}

func MergeSettings(a []byte, b []byte) ([]byte, error) {
	var origSettings map[string]any
	err := json.Unmarshal(a, &origSettings)
	if err != nil {
		return nil, err
	}
	var newSettings map[string]any
	err = json.Unmarshal(b, &newSettings)
	if err != nil {
		return nil, err
	}

	for key, value := range newSettings {
		origSettings[key] = value
	}

	return json.Marshal(origSettings)
}

func GetRawNotifierConfig(n notifytest.NotifierConfigTest, name string) *GrafanaIntegrationConfig {
	if name == "" {
		name = string(n.NotifierType)
	}
	secrets := make(map[string]string)
	if n.Secrets != "" {
		err := json.Unmarshal([]byte(n.Secrets), &secrets)
		if err != nil {
			panic(err)
		}
		for key, value := range secrets {
			secrets[key] = base64.StdEncoding.EncodeToString([]byte(value))
		}
	}

	config := []byte(n.Config)
	if !n.CommonHTTPConfigUnsupported {
		var err error
		config, err = MergeSettings([]byte(n.Config), []byte(notifytest.FullValidHTTPConfigForTesting))
		if err != nil {
			panic(err)
		}
	}

	return &GrafanaIntegrationConfig{
		UID:                   fmt.Sprintf("%s-uid", name),
		Name:                  name,
		Type:                  string(n.NotifierType),
		DisableResolveMessage: true,
		Settings:              config,
		SecureSettings:        secrets,
	}
}
