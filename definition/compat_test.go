package definition

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/alertmanager/pkg/labels"
)

func TestLoadCompat(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		expErr string
	}{
		{
			name:   "no configuration",
			input:  []byte(``),
			expErr: "empty input",
		},
		{
			name:   "no routes",
			input:  []byte(`{}`),
			expErr: "no routes provided",
		},
		{
			name:   "duplicated receivers",
			input:  []byte(testConfigDuplicatedReceivers),
			expErr: "notification config name \"test\" is not unique",
		},
		{
			name:  "no global config",
			input: []byte(testConfigWithoutGlobal),
		},
		{
			name:  "with global config",
			input: []byte(testConfigWithGlobal),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := LoadCompat(test.input)
			if test.expErr != "" {
				require.Error(t, err)
				require.Equal(t, test.expErr, err.Error())
				return
			}

			require.NoError(t, err)

			// It should add the default global config.
			require.NotNil(t, c.Global)
		})
	}
}

func TestAsAMRoute(t *testing.T) {
	// Ensure that AsAMRoute and AsGrafanaRoute are inverses of each other.
	cfg, err := LoadCompat([]byte(testConfigWithComplexRoutes))
	require.NoError(t, err)
	originalRoute := cfg.Route
	// For easier comparison move ObjectMatchers to Matchers.
	mergeMatchers(originalRoute)

	amRoute := originalRoute.AsAMRoute()
	grafanaRoute := AsGrafanaRoute(amRoute)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreUnexported(Route{}, labels.Matcher{}),
		cmpopts.EquateEmpty(),
	}
	if !cmp.Equal(grafanaRoute, originalRoute, cmpOpts...) {
		t.Errorf("Unexpected Diff: %v", cmp.Diff(grafanaRoute, originalRoute, cmpOpts...))
	}
}

func mergeMatchers(route *Route) {
	route.Matchers = append(route.Matchers, route.ObjectMatchers...)
	route.ObjectMatchers = nil
	for _, r := range route.Routes {
		mergeMatchers(r)
	}
}

const testConfigWithoutGlobal = `
route:
  receiver: test
  routes:
    - receiver: test
receivers:
  - name: test
`

const testConfigWithGlobal = `
global:
  smtp_smarthost: smtp.example.org:587
  smtp_from: testfrom@test.com
  resolve_timeout: 5m
  http_config:
    follow_redirects: false
    enable_http2: false
  smtp_hello: test
  smtp_require_tls: false
route:
  receiver: test
  routes:
    - receiver: test
receivers:
  - name: test
`

const testConfigDuplicatedReceivers = `
route:
  receiver: test
  routes:
    - receiver: test
receivers:
  - name: test
  - name: test
`

const testConfigWithComplexRoutes = `
mute_time_intervals:
  - name: test1
    time_intervals:
      - times:
          - start_time: 00:00
            end_time: 12:00
time_intervals:
  - name: weekends
    time_intervals:
    - weekdays:
      - saturday
      - sunday
  - name: weekdays
    time_intervals:
    - weekdays:
      - monday
      - tuesday
      - wednesday
      - thursday
      - friday
route:
  receiver: recv
  group_by:
    - test
    - test2
  group_wait: 1m
  group_interval: 1m
  repeat_interval: 1m
  routes:
    - receiver: recv2
      object_matchers:
        - - team
          - =
          - teamC
      group_by:
        - teste
        - test2f
      group_wait: 0s
      group_interval: 1m
      repeat_interval: 1m
      mute_time_intervals:
        - test1
      active_time_intervals:
        - weekdays
      routes:
        - receiver: recv
          group_by:
            - testc
            - test2d
          group_interval: 10m
          repeat_interval: 1h
          mute_time_intervals:
            - weekends
          active_time_intervals:
            - weekdays
          routes:
            - receiver: recv2
              group_by:
                - testa
                - test2b
              group_wait: 30s
              group_interval: 1m
              repeat_interval: 1m
              active_time_intervals:
                - weekdays
                - test1
receivers:
  - name: recv
  - name: recv2
`
