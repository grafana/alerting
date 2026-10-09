package receivers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/go-kit/log"
	amnotify "github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers/schema"
)

type forkLoggingNotifier struct {
	ForkLogger
	logger log.Logger
}

func (n *forkLoggingNotifier) Notify(ctx context.Context, alerts ...*types.Alert) (bool, error) {
	tmpl := &template.Template{ExternalURL: &url.URL{}}
	amnotify.GetTemplateData(ctx, tmpl, alerts, n.GetSlogLogger(n.logger))
	return false, nil
}

func (n *forkLoggingNotifier) SendResolved() bool { return true }

func TestFactoryPreservesForkCaller(t *testing.T) {
	var buf bytes.Buffer
	normal := log.With(log.NewLogfmtLogger(&buf), "caller", "normal.go:1")
	companion := logging.NewSlogLogger(log.NewLogfmtLogger(&buf), logging.WithCaller())
	factory := NewIntegrationVersionFactory("test", schema.V1,
		func(json.RawMessage, DecryptFunc) (struct{}, error) { return struct{}{}, nil },
		func(_ struct{}, _ Metadata, opts NotifierOpts) (NotificationChannel, error) {
			return &forkLoggingNotifier{logger: opts.Logger}, nil
		},
	)
	for _, supplied := range []bool{false, true} {
		buf.Reset()
		opts := NotifierOpts{Logger: normal}
		if supplied {
			opts.SlogLogger = companion
		}
		notifier, err := factory.NewNotifier(nil, nil, Metadata{}, opts)
		require.NoError(t, err)
		_, err = notifier.Notify(context.Background())
		require.NoError(t, err)
		if supplied {
			require.Contains(t, buf.String(), "caller=util.go:")
			require.NotContains(t, buf.String(), "normal.go")
		} else {
			require.Contains(t, buf.String(), "caller=normal.go:1")
		}
	}
}

func TestBaseScopesForkLogger(t *testing.T) {
	var normalBuf, forkBuf bytes.Buffer
	base := NewBase(Metadata{Name: "team", Type: "test", Version: schema.V1, Index: 2}, log.NewLogfmtLogger(&normalBuf))
	require.Nil(t, base.GetSlogLogger(context.Background()))
	base.SetSlogLogger(logging.NewSlogLogger(log.NewLogfmtLogger(&forkBuf), logging.WithCaller()))
	base.GetSlogLogger(context.Background()).Info("fork")
	require.Empty(t, normalBuf.String())
	require.Contains(t, forkBuf.String(), "receiver=team")
	require.Contains(t, forkBuf.String(), "integration=test[2]")
	require.Contains(t, forkBuf.String(), "version=v1")
	base.SetSlogLogger(nil)
	require.Nil(t, base.GetSlogLogger(context.Background()))
}
