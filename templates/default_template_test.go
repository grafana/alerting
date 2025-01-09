package templates

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/logging"
)

func TestDefaultTemplateString(t *testing.T) {
	alerts := []*types.Alert{
		{ // Firing with dashboard and panel ID.
			Alert: model.Alert{
				Labels: model.LabelSet{
					"alertname": "alert1",
					"lbl1":      "val1",
				},
				Annotations: model.LabelSet{
					"ann1":             "annv1",
					"__orgId__":        "1",
					"__dashboardUid__": "dbuid123",
					"__panelId__":      "puid123",
					"__values__":       "{\"A\": 1234}",
					"__value_string__": "1234",
				},
				StartsAt:     time.Now(),
				EndsAt:       time.Now().Add(1 * time.Hour),
				GeneratorURL: "http://localhost/alert1?orgId=1",
			},
		}, { // Firing without dashboard and panel ID.
			Alert: model.Alert{
				Labels: model.LabelSet{
					"alertname": "alert1",
					"lbl1":      "val2",
				},
				Annotations: model.LabelSet{
					"ann1":             "annv2",
					"__values__":       "{\"A\": 1234, \"B\": 5678, \"C\": 9}",
					"__value_string__": "1234",
				},
				StartsAt:     time.Now(),
				EndsAt:       time.Now().Add(2 * time.Hour),
				GeneratorURL: "http://localhost/alert2",
			},
		}, { // Firing with OrgID and without dashboard and panel ID.
			Alert: model.Alert{
				Labels: model.LabelSet{
					"alertname": "alert1",
					"lbl1":      "val3",
				},
				Annotations: model.LabelSet{
					"ann1":             "annv3",
					"__orgId__":        "1",
					"__values__":       "{\"A\": 1234}",
					"__value_string__": "1234",
				},
				StartsAt:     time.Now(),
				EndsAt:       time.Now().Add(2 * time.Hour),
				GeneratorURL: "http://localhost/alert3?orgId=1",
			},
		}, { // Resolved with dashboard and panel ID.
			Alert: model.Alert{
				Labels: model.LabelSet{
					"alertname": "alert1",
					"lbl1":      "val4",
				},
				Annotations: model.LabelSet{
					"ann1":             "annv4",
					"__orgId__":        "1",
					"__dashboardUid__": "dbuid456",
					"__panelId__":      "puid456",
					"__values__":       "{\"A\": 1234}",
					"__value_string__": "1234",
				},
				StartsAt:     time.Now().Add(-1 * time.Hour),
				EndsAt:       time.Now().Add(-30 * time.Minute),
				GeneratorURL: "http://localhost/alert4?orgId=1",
			},
		}, { // Resolved without dashboard and panel ID.
			Alert: model.Alert{
				Labels: model.LabelSet{
					"alertname": "alert1",
					"lbl1":      "val5",
				},
				Annotations: model.LabelSet{
					"ann1":             "annv5",
					"__values__":       "{\"A\": 1234, \"B\": 5678, \"C\": 9}",
					"__value_string__": "1234",
				},
				StartsAt:     time.Now().Add(-2 * time.Hour),
				EndsAt:       time.Now().Add(-3 * time.Hour),
				GeneratorURL: "http://localhost/alert5",
			},
		}, { // Resolved with OrgID and without dashboard and panel ID.
			Alert: model.Alert{
				Labels: model.LabelSet{
					"alertname": "alert1",
					"lbl1":      "val6",
				},
				Annotations: model.LabelSet{
					"ann1":             "annv6",
					"__orgId__":        "1",
					"__values__":       "{\"A\": 1234}",
					"__value_string__": "1234",
				},
				StartsAt:     time.Now().Add(-2 * time.Hour),
				EndsAt:       time.Now().Add(-3 * time.Hour),
				GeneratorURL: "http://localhost/alert6?orgId=1",
			},
		},
	}

	tmpl, err := FromContent(nil)
	require.NoError(t, err)

	externalURL, err := url.Parse("http://localhost/grafana")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	var tmplErr error
	l := &logging.FakeLogger{}
	expand, _ := TmplText(context.Background(), tmpl, alerts, l, &tmplErr)

	tmplDef, err := DefaultTemplate()
	require.NoError(t, err)

	tmplFromDefinition, err := newTemplate()
	require.NoError(t, err)
	// Parse default template string.
	err = tmplFromDefinition.Parse(strings.NewReader(tmplDef.Template))
	require.NoError(t, err)
	tmplFromDefinition.ExternalURL = externalURL

	var tmplDefErr error
	expandFromDefinition, _ := TmplText(context.Background(), tmplFromDefinition, alerts, l, &tmplDefErr)

	cases := []struct {
		templateString string
		expected       string
	}{
		{
			templateString: DefaultMessageTitleEmbed,
			expected:       `[FIRING:3, RESOLVED:3]  (alert1)`,
		},
		{
			templateString: DefaultMessageEmbed,
			expected: `**Firing**

Value: A=1234
Labels:
 - alertname = alert1
 - lbl1 = val1
Annotations:
 - ann1 = annv1
Source: http://localhost/alert1?orgId=1
Silence: http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1
Dashboard: http://localhost/grafana/d/dbuid123?orgId=1
Panel: http://localhost/grafana/d/dbuid123?orgId=1&viewPanel=puid123

Value: A=1234, B=5678, C=9
Labels:
 - alertname = alert1
 - lbl1 = val2
Annotations:
 - ann1 = annv2
Source: http://localhost/alert2
Silence: http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2

Value: A=1234
Labels:
 - alertname = alert1
 - lbl1 = val3
Annotations:
 - ann1 = annv3
Source: http://localhost/alert3?orgId=1
Silence: http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval3&orgId=1


**Resolved**

Value: A=1234
Labels:
 - alertname = alert1
 - lbl1 = val4
Annotations:
 - ann1 = annv4
Source: http://localhost/alert4?orgId=1
Silence: http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval4&orgId=1
Dashboard: http://localhost/grafana/d/dbuid456?orgId=1
Panel: http://localhost/grafana/d/dbuid456?orgId=1&viewPanel=puid456

Value: A=1234, B=5678, C=9
Labels:
 - alertname = alert1
 - lbl1 = val5
Annotations:
 - ann1 = annv5
Source: http://localhost/alert5
Silence: http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval5

Value: A=1234
Labels:
 - alertname = alert1
 - lbl1 = val6
Annotations:
 - ann1 = annv6
Source: http://localhost/alert6?orgId=1
Silence: http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval6&orgId=1
`,
		},
		{
			templateString: `{{ template "teams.default.message" .}}`,
			expected: `**Firing**

Value: A=1234
Labels:
 - alertname = alert1
 - lbl1 = val1

Annotations:
 - ann1 = annv1

Source: [http://localhost/alert1?orgId=1](http://localhost/alert1?orgId=1)

Silence: [http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1](http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1&orgId=1)

Dashboard: [http://localhost/grafana/d/dbuid123?orgId=1](http://localhost/grafana/d/dbuid123?orgId=1)

Panel: [http://localhost/grafana/d/dbuid123?orgId=1&viewPanel=puid123](http://localhost/grafana/d/dbuid123?orgId=1&viewPanel=puid123)



Value: A=1234, B=5678, C=9
Labels:
 - alertname = alert1
 - lbl1 = val2

Annotations:
 - ann1 = annv2

Source: [http://localhost/alert2](http://localhost/alert2)

Silence: [http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2](http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2)



Value: A=1234
Labels:
 - alertname = alert1
 - lbl1 = val3

Annotations:
 - ann1 = annv3

Source: [http://localhost/alert3?orgId=1](http://localhost/alert3?orgId=1)

Silence: [http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval3&orgId=1](http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval3&orgId=1)




**Resolved**

Value: A=1234
Labels:
 - alertname = alert1
 - lbl1 = val4

Annotations:
 - ann1 = annv4

Source: [http://localhost/alert4?orgId=1](http://localhost/alert4?orgId=1)

Silence: [http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval4&orgId=1](http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval4&orgId=1)

Dashboard: [http://localhost/grafana/d/dbuid456?orgId=1](http://localhost/grafana/d/dbuid456?orgId=1)

Panel: [http://localhost/grafana/d/dbuid456?orgId=1&viewPanel=puid456](http://localhost/grafana/d/dbuid456?orgId=1&viewPanel=puid456)



Value: A=1234, B=5678, C=9
Labels:
 - alertname = alert1
 - lbl1 = val5

Annotations:
 - ann1 = annv5

Source: [http://localhost/alert5](http://localhost/alert5)

Silence: [http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval5](http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval5)



Value: A=1234
Labels:
 - alertname = alert1
 - lbl1 = val6

Annotations:
 - ann1 = annv6

Source: [http://localhost/alert6?orgId=1](http://localhost/alert6?orgId=1)

Silence: [http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval6&orgId=1](http://localhost/grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval6&orgId=1)


`,
		},
	}

	for _, c := range cases {
		t.Run(c.templateString, func(t *testing.T) {
			t.Run("FromContent", func(t *testing.T) {
				act := expand(c.templateString)
				require.NoError(t, tmplErr)
				require.Equal(t, c.expected, act)
			})

			t.Run("DefaultTemplate", func(t *testing.T) {
				act := expandFromDefinition(c.templateString)
				require.NoError(t, tmplDefErr)
				require.Equal(t, c.expected, act)
			})
		})
	}
	require.NoError(t, tmplErr)
	require.NoError(t, tmplDefErr)
}
