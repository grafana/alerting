package templates

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/models"
)

func TestTmplText(t *testing.T) {
	constNow := time.Now()
	defer mockTimeNow(constNow)()

	alerts := []*types.Alert{
		{
			Alert: model.Alert{
				Labels: model.LabelSet{
					"alertname":             "TestAlert",
					"severity":              "critical",
					models.FolderTitleLabel: "test-folder",
					models.RuleUIDLabel:     "test-rule-uid",
				},
				Annotations: model.LabelSet{
					"summary":     "Test summary",
					"description": "Test description",
					"__orgId__":   "1",
				},
				StartsAt:     constNow,
				EndsAt:       constNow.Add(1 * time.Hour),
				GeneratorURL: "http://localhost/alert",
			},
		},
	}
	tm, err := fromContent(defaultTemplatesPerKind(GrafanaKind), defaultOptionsPerKind(GrafanaKind, "grafana")...)
	require.NoError(t, err)

	externalURL, err := url.Parse("http://localhost/grafana")
	require.NoError(t, err)
	tm.ExternalURL = externalURL
	l := log.NewNopLogger()

	tmpl := &Template{
		Template: tm,
		limits:   DefaultLimits,
	}

	t.Run("should execute simple template successfully", func(t *testing.T) {
		var tmplErr error
		expand, data := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		result := expand("{{ len .Alerts }}")
		assert.NoError(t, tmplErr)
		assert.Equal(t, "1", result)
		assert.NotNil(t, data)
		assert.Len(t, data.Alerts, 1)
	})

	t.Run("should execute multiple templates in sequence", func(t *testing.T) {
		var tmplErr error
		expand, _ := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		result1 := expand("{{ len .Alerts }}")
		assert.NoError(t, tmplErr)
		assert.Equal(t, "1", result1)

		result2 := expand("{{ .Status }}")
		assert.NoError(t, tmplErr)
		assert.Equal(t, "firing", result2)
	})

	t.Run("should propagate template parsing error", func(t *testing.T) {
		var tmplErr error
		expand, _ := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		// Invalid template syntax
		result := expand("{{ .InvalidField }")
		assert.Error(t, tmplErr)
		assert.Empty(t, result)
		// Just verify there's an error, don't check specific message
	})

	t.Run("should not execute subsequent templates after error", func(t *testing.T) {
		var tmplErr error
		expand, _ := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		// First template with error
		result1 := expand("{{ .InvalidField }")
		assert.Error(t, tmplErr)
		assert.Empty(t, result1)

		// Second template should not execute
		result2 := expand("{{ len .Alerts }}")
		assert.Error(t, tmplErr) // Error persists
		assert.Empty(t, result2) // Should return empty string
	})

	t.Run("should handle empty template string", func(t *testing.T) {
		var tmplErr error
		expand, _ := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		result := expand("")
		assert.NoError(t, tmplErr)
		assert.Equal(t, "", result)
	})

	t.Run("should include extended data fields", func(t *testing.T) {
		var tmplErr error
		_, data := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		assert.NotNil(t, data)
		assert.Equal(t, "http://localhost/grafana", data.ExternalURL)
		assert.Len(t, data.Alerts, 1)
		assert.Equal(t, "TestAlert", data.Alerts[0].Labels["alertname"])
	})

	t.Run("should extract group key from context", func(t *testing.T) {
		// Create context with group key
		ctx := context.Background()
		groupKey := "test-group-key"
		ctx = notify.WithGroupKey(ctx, groupKey)

		var tmplErr error
		_, data := TmplText(ctx, tmpl, alerts, l, &tmplErr)

		assert.NotNil(t, data)
		assert.Equal(t, groupKey, data.GroupKey)
	})

	t.Run("should handle context without group key", func(t *testing.T) {
		var tmplErr error
		_, data := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		assert.NotNil(t, data)
		assert.Equal(t, "", data.GroupKey) // Should be empty when not in context
	})

	t.Run("should allow template output under size limit", func(t *testing.T) {
		var tmplErr error
		expand, _ := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		// Small output should work
		result := expand("{{ range .Alerts }}{{ .Labels.alertname }}{{ end }}")
		assert.NoError(t, tmplErr)
		assert.Equal(t, "TestAlert", result)
	})

	t.Run("should reject template output exceeding size limit", func(t *testing.T) {
		tmpl := &Template{
			Template: tm,
			limits:   Limits{MaxTemplateOutputSize: 1024},
		}

		var tmplErr error
		expand, _ := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

		// Create a template that generates output larger than 1 KB by repeating a pattern
		largeTemplate := `{{ range .Alerts }}` + strings.Repeat("X", 2000) + `{{ end }}`

		result := expand(largeTemplate)
		assert.Error(t, tmplErr)
		assert.ErrorIs(t, tmplErr, ErrTemplateOutputTooLarge)
		assert.NotEmpty(t, result) // Should contain partial output up to limit
	})
}
