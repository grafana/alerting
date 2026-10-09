package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/templates"
)

func TestGrafanaAlertmanagerCallerlessLogger(t *testing.T) {
	var buf bytes.Buffer
	normal := log.With(log.NewLogfmtLogger(&buf), "caller", "normal.go:1")
	am := &GrafanaAlertmanager{
		opts: GrafanaAlertmanagerOpts{
			Logger: normal, CallerlessLogger: log.NewLogfmtLogger(&buf),
			TenantKey: "org", TenantID: 42,
		},
		logger: log.With(normal, "component", "alertmanager", "org", 42),
	}
	for _, scoped := range []bool{true, false} {
		buf.Reset()
		logger := am.forkLoggerRaw()
		if scoped {
			logger = am.forkLogger()
		}
		_, _, line, _ := runtime.Caller(0)
		logger.With("operation", "test").Info("fork")
		require.Contains(t, buf.String(), fmt.Sprintf("caller=callerless_logger_test.go:%d", line+1))
		require.Equal(t, 1, strings.Count(buf.String(), "caller="))
		require.NotContains(t, buf.String(), "normal.go")
		if scoped {
			require.Contains(t, buf.String(), "component=alertmanager org=42")
		} else {
			require.NotContains(t, buf.String(), "component=")
		}
	}
	am.opts.CallerlessLogger = nil
	require.Nil(t, am.receiverForkLogger())
	buf.Reset()
	am.forkLogger().Info("default")
	require.Contains(t, buf.String(), "caller=normal.go:1")
}

func TestGrafanaAlertmanagerPassesForkLoggerToReceivers(t *testing.T) {
	var buf bytes.Buffer
	normal := log.With(log.NewLogfmtLogger(&buf), "caller", "normal.go:1")
	am := &GrafanaAlertmanager{
		opts: GrafanaAlertmanagerOpts{
			Logger: normal, CallerlessLogger: log.NewLogfmtLogger(&buf),
			TenantKey: "org", TenantID: 42,
			EmailSender: receivers.MockNotificationService(), ImageProvider: images.NewFakeProvider(0),
			Decrypter: NoopDecrypt,
		},
		logger: log.With(normal, "component", "alertmanager", "org", 42),
	}
	cfg, err := templates.NewConfig("grafana", "http://localhost", "", templates.DefaultLimits)
	require.NoError(t, err)
	factory, err := templates.NewFactory(nil, cfg, log.NewNopLogger())
	require.NoError(t, err)
	channels, err := am.buildReceiverIntegrations(models.ReceiverConfig{
		Name: "team",
		Integrations: []*models.IntegrationConfig{{
			Name: "team", Type: schema.EmailType, Version: schema.V1,
			Settings: json.RawMessage(`{"addresses":"team@example.com","subject":"test"}`),
		}},
	}, factory)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	_, err = channels[0].Notify(context.Background())
	require.NoError(t, err)
	forkLines := 0
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "Missing receiver") || strings.Contains(line, "Missing group labels") {
			forkLines++
			require.Contains(t, line, "caller=util.go:")
			require.Contains(t, line, "component=alertmanager org=42")
			require.Contains(t, line, "receiver=team")
			require.NotContains(t, line, "normal.go")
		}
	}
	require.Equal(t, 2, forkLines)
}
