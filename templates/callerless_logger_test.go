package templates

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
)

func TestTmplTextUsesForkLogger(t *testing.T) {
	tmpl, err := fromContent(defaultTemplatesPerKind(GrafanaKind), defaultOptionsPerKind(GrafanaKind, "grafana")...)
	require.NoError(t, err)
	var buf bytes.Buffer
	normal := log.With(log.NewLogfmtLogger(&buf), "caller", "normal.go:1")
	fork := logging.NewSlogLogger(log.NewLogfmtLogger(&buf), logging.WithCaller())
	var templateErr error
	tmpl.ExternalURL = &url.URL{}
	TmplText(context.Background(), &Template{Template: tmpl}, nil, normal, &templateErr, fork)
	forkLines := 0
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "Missing receiver") || strings.Contains(line, "Missing group labels") {
			forkLines++
			require.Contains(t, line, "caller=util.go:")
			require.NotContains(t, line, "normal.go")
			require.Equal(t, 1, strings.Count(line, "caller="))
		}
	}
	require.Equal(t, 2, forkLines)
}
