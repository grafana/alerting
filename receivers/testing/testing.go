package testing

import (
	"context"
	"net/url"
	"testing"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func NewFactoryConfigForValidateConfigTesting(t *testing.T, m *receivers.NotificationChannelConfig) (receivers.FactoryConfig, error) {
	tmpl := templates.ForTests(t)
	tmpl.ExternalURL = ParseURLUnsafe("http://localhost")
	return receivers.NewFactoryConfig(m, nil, DecryptForTesting, tmpl, &images.UnavailableImageStore{}, func(ctx ...interface{}) logging.Logger {
		return &logging.FakeLogger{}
	}, "1.2.3")
}

func ParseURLUnsafe(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func DecryptForTesting(_ context.Context, sjd map[string][]byte, key string, fallback string) string {
	v, ok := sjd[key]
	if !ok {
		return fallback
	}
	return string(v)
}
