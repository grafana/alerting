package templates

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/utils"
)

func TestNewFactory(t *testing.T) {
	logger := log.NewNopLogger()
	externalURL := "http://localhost:3000"
	tests := []struct {
		name        string
		templates   []TemplateDefinition
		expectError error

		expected map[Kind][]TemplateDefinition // Expected templates grouped by kind
	}{
		{
			name:      "valid templates, no duplicates",
			templates: []TemplateDefinition{{Name: "t1", Kind: GrafanaKind}, {Name: "t2", Kind: MimirKind}},
			expected: map[Kind][]TemplateDefinition{
				GrafanaKind: {{Name: "t1", Kind: GrafanaKind}},
				MimirKind:   {{Name: "t2", Kind: MimirKind}},
			},
		},
		{
			name:      "empty templates",
			templates: nil,
			expected:  map[Kind][]TemplateDefinition{},
		},
		{
			name:      "duplicate templates",
			templates: []TemplateDefinition{{Name: "t1", Kind: GrafanaKind, Template: "TEST1"}, {Name: "t1", Kind: GrafanaKind, Template: "TEST2"}},
			expected: map[Kind][]TemplateDefinition{
				GrafanaKind: {{Name: "t1", Kind: GrafanaKind, Template: "TEST1"}},
			},
		},
		{
			name:        "unknown kind defaults to GrafanaKind",
			templates:   []TemplateDefinition{{Name: "t1", Kind: 1234}},
			expectError: ErrInvalidKind,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewConfig("grafana", externalURL, DefaultLimits)
			require.NoError(t, err)
			factory, err := NewFactory(tc.templates, cfg, logger)
			if tc.expectError != nil {
				require.ErrorIs(t, err, ErrInvalidKind)
				return
			}
			require.NoError(t, err)
			// Validate the templates map
			for kind, expectedTemplates := range tc.expected {
				require.ElementsMatch(t, expectedTemplates, factory.templates[kind])
			}
		})
	}
}

func TestFactoryNewTemplate(t *testing.T) {
	as := []*types.Alert{
		{
			Alert: model.Alert{
				Labels:   model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
				StartsAt: timeNow().Add(-1 * time.Hour),
				EndsAt:   timeNow().Add(1 * time.Hour),
				Annotations: model.LabelSet{
					"description": "alert1 description",
					"summary":     "alert1 summary",
				},
			},
			UpdatedAt: timeNow(),
			Timeout:   false,
		},
		{
			Alert: model.Alert{
				Labels:   model.LabelSet{"__alert_rule_uid__": "rule uid", "alertname": "alert1", "lbl1": "val1"},
				StartsAt: timeNow().Add(-2 * time.Hour),
				EndsAt:   timeNow().Add(-1 * time.Hour),
				Annotations: model.LabelSet{
					"description": "alert1 description",
					"summary":     "alert1 summary",
				},
			},
			UpdatedAt: timeNow(),
			Timeout:   false,
		},
	}

	def := make([]TemplateDefinition, 0, len(validKinds))
	for kind := range validKinds {
		def = append(def, TemplateDefinition{
			Name:     "test",
			Kind:     kind,
			Template: fmt.Sprintf(`{{ define "factory_test" }}TEST %s KIND{{ end }}`, kind),
		})
	}
	cfg, err := NewConfig("grafana", "http://localhost", DefaultLimits)
	require.NoError(t, err)
	f, err := NewFactory(def, cfg, log.NewNopLogger())
	require.NoError(t, err)

	testCases := []struct {
		name     string
		kind     Kind
		template string
		expected string
		err      string
	}{
		{
			name:     "Grafana template for Grafana kind",
			kind:     GrafanaKind,
			template: `{{ template "default.title" . }}`,
			expected: "[FIRING:1, RESOLVED:1]  (alert1 val1)",
		},
		{
			name:     "Grafana template does not work for Prometheus kind",
			kind:     MimirKind,
			template: `{{ template "default.title" . }}`,
			err:      `template "default.title" not defined`,
		},
		{
			name:     "Promtheus template for Prometheus kind",
			kind:     MimirKind,
			template: `{{ template "__subject" . }}`,
			expected: `[FIRING:1]  (alert1 val1)`,
		},
	}
	seen := make(map[Kind]struct{}, len(validKinds))
	maps.Copy(seen, validKinds)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			templ, err := f.GetTemplate(tc.kind)
			assert.NoError(t, err)
			require.NotNil(t, templ)
			var tmplErr error
			tmpl, _ := TmplText(context.Background(), templ, as, utils.SlogFromGoKit(log.NewNopLogger()), &tmplErr)
			result := tmpl(tc.template)
			if tc.err != "" {
				assert.ErrorContains(t, tmplErr, tc.err)
			} else {
				assert.Equal(t, tc.expected, result)
				assert.NoError(t, tmplErr)
			}
		})
		delete(seen, tc.kind)
	}
	assert.Empty(t, seen)

	t.Run("should apply user-defined templates", func(t *testing.T) {
		for kind := range validKinds {
			templ, err := f.GetTemplate(kind)
			require.NoError(t, err)
			var tmplErr error
			tmpl, _ := TmplText(context.Background(), templ, as, utils.SlogFromGoKit(log.NewNopLogger()), &tmplErr)
			result := tmpl(`{{ template "factory_test" . }}`)
			require.NoError(t, tmplErr)
			require.Equal(t, fmt.Sprintf(`TEST %s KIND`, kind), result)
		}
	})

	t.Run("user-defined template only applies to the given kind", func(t *testing.T) {
		cfg, err := NewConfig("grafana", "http://localhost", DefaultLimits)
		require.NoError(t, err)
		f, err := NewFactory([]TemplateDefinition{
			{
				Name:     "test",
				Kind:     GrafanaKind,
				Template: fmt.Sprintf(`{{ define "factory_test" }}TEST %s KIND{{ end }}`, GrafanaKind),
			},
		}, cfg, log.NewNopLogger())
		require.NoError(t, err)
		templ, err := f.GetTemplate(GrafanaKind)
		require.NoError(t, err)
		var tmplErr error
		tmpl, _ := TmplText(context.Background(), templ, as, utils.SlogFromGoKit(log.NewNopLogger()), &tmplErr)
		result := tmpl(`{{ template "factory_test" . }}`)
		require.NoError(t, tmplErr)
		require.Equal(t, `TEST Grafana KIND`, result)
		templ, err = f.GetTemplate(MimirKind)
		require.NoError(t, err)
		require.NotNil(t, templ)
		tmpl, _ = TmplText(context.Background(), templ, as, utils.SlogFromGoKit(log.NewNopLogger()), &tmplErr)
		_ = tmpl(`{{ template "factory_test" . }}`)
		require.ErrorContains(t, tmplErr, `template "factory_test" not defined`)
	})
}

func TestLimitsValidate(t *testing.T) {
	tests := []struct {
		name        string
		limits      Limits
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid limits with positive MaxTemplateOutputSize",
			limits: Limits{
				MaxTemplateOutputSize: 1024,
			},
			expectError: false,
		},
		{
			name: "valid limits with zero MaxTemplateOutputSize",
			limits: Limits{
				MaxTemplateOutputSize: 0,
			},
			expectError: false,
		},
		{
			name:        "default limits are valid",
			limits:      DefaultLimits,
			expectError: false,
		},
		{
			name: "invalid limits with negative MaxTemplateOutputSize",
			limits: Limits{
				MaxTemplateOutputSize: -1,
			},
			expectError: true,
			errorMsg:    "maxTemplateOutputSize must be greater than or equal to 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.limits.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	u, _ := url.Parse("http://localhost:3000")
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with external URL and valid limits",
			config: Config{
				OrgID:       "grafana",
				ExternalURL: u,
				Limits:      DefaultLimits,
			},
			expectError: false,
		},
		{
			name: "invalid config with nil ExternalURL",
			config: Config{
				OrgID:       "grafana",
				ExternalURL: nil,
				Limits:      DefaultLimits,
			},
			expectError: true,
			errorMsg:    "externalURL must be set",
		},
		{
			name: "invalid config with invalid limits",
			config: Config{
				OrgID:       "grafana",
				ExternalURL: u,
				Limits: Limits{
					MaxTemplateOutputSize: -1,
				},
			},
			expectError: true,
			errorMsg:    "maxTemplateOutputSize must be greater than or equal to 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.config
			err := cfg.Validate()
			if tc.expectError {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFactoryWithTemplate(t *testing.T) {
	as := []*types.Alert{{}}
	kind := GrafanaKind
	initial := TemplateDefinition{Name: "test", Kind: kind, Template: `{{ define "factory_test" }}TEST{{ end }}`}
	cfg, err := NewConfig("grafana", "http://localhost", DefaultLimits)
	require.NoError(t, err)
	f, err := NewFactory([]TemplateDefinition{initial}, cfg, log.NewNopLogger())
	require.NoError(t, err)
	templ, err := f.GetTemplate(kind)
	require.NoError(t, err)
	var tmplErr error
	tmpl, _ := TmplText(context.Background(), templ, as, utils.SlogFromGoKit(log.NewNopLogger()), &tmplErr)
	result := tmpl(`{{ template "factory_test" . }}`)
	require.NoError(t, tmplErr)
	assert.Equal(t, `TEST`, result)

	t.Run("should add new template", func(t *testing.T) {
		f2, err := f.WithTemplate(TemplateDefinition{Name: "test2", Kind: kind, Template: `{{ define "factory_test2" }}TEST2{{ end }}`})
		require.NoError(t, err)
		templ, err := f2.GetTemplate(kind)
		require.NoError(t, err)
		var tmplErr error
		tmpl, _ := TmplText(context.Background(), templ, as, utils.SlogFromGoKit(log.NewNopLogger()), &tmplErr)
		result := tmpl(`{{ template "factory_test2" . }}`)
		require.NoError(t, tmplErr)
		require.Equal(t, `TEST2`, result)
	})

	t.Run("should replace existing template", func(t *testing.T) {
		f2, err := f.WithTemplate(TemplateDefinition{Name: "test", Kind: kind, Template: `{{ define "factory_test" }}TEST2{{ end }}`})
		require.NoError(t, err)
		templ, err := f2.GetTemplate(kind)
		require.NoError(t, err)
		var tmplErr error
		tmpl, _ := TmplText(context.Background(), templ, as, utils.SlogFromGoKit(log.NewNopLogger()), &tmplErr)
		result := tmpl(`{{ template "factory_test" . }}`)
		require.NoError(t, tmplErr)
		require.Equal(t, `TEST2`, result)
	})

	t.Run("should fail if kind is not known", func(t *testing.T) {
		_, err := f.WithTemplate(TemplateDefinition{Name: "test", Kind: 1234, Template: `{{ define "factory_test" }}TEST{{ end }}`})
		require.ErrorIs(t, err, ErrInvalidKind)
	})
}
