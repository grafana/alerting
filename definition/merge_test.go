package definition

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/dispatch"
	"github.com/prometheus/alertmanager/pkg/labels"
	commoncfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMergeOpts_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		opts        MergeOpts
		expectedErr error
	}{
		{
			name: "error if subtree matchers are empty",
			opts: MergeOpts{
				SubtreeMatchers: config.Matchers{},
			},
			expectedErr: ErrNoMatchers,
		},
		{
			name: "error if subtree matchers are not equal",
			opts: MergeOpts{
				SubtreeMatchers: config.Matchers{
					{
						Type:  labels.MatchNotEqual,
						Name:  "label",
						Value: "test",
					},
				},
			},
			expectedErr: ErrInvalidMatchers,
		},
		{
			name: "error if subtree matchers are regex",
			opts: MergeOpts{
				SubtreeMatchers: config.Matchers{
					{
						Type:  labels.MatchRegexp,
						Name:  "label",
						Value: "test",
					},
				},
			},
			expectedErr: ErrInvalidMatchers,
		},
		{
			name: "error if subtree matchers are not-regex",
			opts: MergeOpts{
				SubtreeMatchers: config.Matchers{
					{
						Type:  labels.MatchNotRegexp,
						Name:  "label",
						Value: "test",
					},
				},
			},
			expectedErr: ErrInvalidMatchers,
		},
		{
			name: "error if duplicates",
			opts: MergeOpts{
				SubtreeMatchers: config.Matchers{
					{
						Type:  labels.MatchEqual,
						Name:  "label",
						Value: "test",
					},
					{
						Type:  labels.MatchEqual,
						Name:  "label",
						Value: "test",
					},
				},
			},
			expectedErr: ErrDuplicateMatchers,
		},
		{
			name: "valid if no duplicates and only equal matchers",
			opts: MergeOpts{
				SubtreeMatchers: config.Matchers{
					{
						Type:  labels.MatchEqual,
						Name:  "al",
						Value: "test",
					},
					{
						Type:  labels.MatchEqual,
						Name:  "bl",
						Value: "test",
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.opts.Validate()
			if tc.expectedErr != nil {
				assert.ErrorIs(t, actual, tc.expectedErr)
				return
			}
			assert.NoError(t, actual)
		})
	}
}

func TestMerge(t *testing.T) {
	opts := MergeOpts{
		DedupSuffix: "_mimir-12345",
		SubtreeMatchers: config.Matchers{
			{
				Type:  labels.MatchEqual,
				Name:  "__datasource_uid__",
				Value: "12345",
			},
			{
				Type:  labels.MatchEqual,
				Name:  "__mimir__",
				Value: "true",
			},
		},
	}

	testCases := []struct {
		name        string
		grafana     *PostableApiAlertingConfig
		mimir       *PostableApiAlertingConfig
		expected    MergeResult
		expectedErr error
	}{
		{
			name:    "should merge all resources, no renames",
			grafana: load(t, fullGrafanaConfig),
			mimir:   load(t, fullMimirConfig),
			expected: MergeResult{
				Config: *load(t, fullMergedConfig),
			},
		},
		{
			name:    "should populate intervals by defaults",
			grafana: load(t, fullGrafanaConfig),
			mimir: load(t, fullMimirConfig, func(p *PostableApiAlertingConfig) {
				p.Route.GroupWait = nil
				p.Route.GroupInterval = nil
				p.Route.RepeatInterval = nil
			}),
			expected: MergeResult{
				Config: *load(t, fullMergedConfig, func(p *PostableApiAlertingConfig) {
					gw := model.Duration(dispatch.DefaultRouteOpts.GroupWait)
					gi := model.Duration(dispatch.DefaultRouteOpts.GroupInterval)
					ri := model.Duration(dispatch.DefaultRouteOpts.RepeatInterval)
					p.Route.Routes[0].GroupWait = &gw
					p.Route.Routes[0].GroupInterval = &gi
					p.Route.Routes[0].RepeatInterval = &ri
				}),
			},
		},
		{
			name:    "should rename receivers and refactor usages",
			grafana: load(t, fullGrafanaConfig),
			mimir: load(t, fullMimirConfig, func(p *PostableApiAlertingConfig) {
				p.Receivers = append(p.Receivers, &PostableApiReceiver{
					Receiver: config.Receiver{
						Name: "grafana-default-email",
					},
				})
				p.Route.Routes = append(p.Route.Routes, &Route{
					Receiver: "grafana-default-email",
					Matchers: config.Matchers{
						{
							Type:  labels.MatchEqual,
							Name:  "label",
							Value: "test",
						},
					},
				})
			}),
			expected: MergeResult{
				Config: *load(t, fullMergedConfig, func(p *PostableApiAlertingConfig) {
					p.Route.Routes[0].Routes = append(p.Route.Routes[0].Routes, &Route{
						Receiver: "grafana-default-email_mimir-12345",
						Matchers: config.Matchers{
							{
								Type:  labels.MatchEqual,
								Name:  "label",
								Value: "test",
							},
						},
					})
					p.Receivers = append(p.Receivers, &PostableApiReceiver{
						Receiver: config.Receiver{
							Name: "grafana-default-email_mimir-12345",
						},
					})

				}),
				RenamedReceivers: map[string]string{
					"grafana-default-email": "grafana-default-email_mimir-12345",
				},
			},
		},
		{
			name: "should append index suffix if rename still collides",
			grafana: load(t, fullGrafanaConfig, func(p *PostableApiAlertingConfig) {
				p.Receivers = append(p.Receivers, &PostableApiReceiver{
					Receiver: config.Receiver{
						Name: "grafana-default-email_mimir-12345",
					},
				})
			}),
			mimir: load(t, fullMimirConfig, func(p *PostableApiAlertingConfig) {
				p.Receivers = append(p.Receivers, &PostableApiReceiver{
					Receiver: config.Receiver{
						Name: "grafana-default-email",
					},
				})
			}),
			expected: MergeResult{
				Config: *load(t, fullMergedConfig, func(p *PostableApiAlertingConfig) {
					p.Receivers = append(p.Receivers,
						&PostableApiReceiver{
							Receiver: config.Receiver{
								Name: "grafana-default-email_mimir-12345",
							},
						},
						&PostableApiReceiver{
							Receiver: config.Receiver{
								Name: "grafana-default-email_mimir-12345_01",
							},
						},
					)
				}),
				RenamedReceivers: map[string]string{
					"grafana-default-email": "grafana-default-email_mimir-12345_01",
				},
			},
		},
		{
			name:    "should rename time intervals and refactor usages",
			grafana: load(t, fullGrafanaConfig),
			mimir: load(t, fullMimirConfig, func(p *PostableApiAlertingConfig) {
				// intentionally swap intervals here, just make sure the uniqueness is enforced across both fields
				p.TimeIntervals = append(p.TimeIntervals, config.TimeInterval{
					Name: "mti-1",
				})
				p.MuteTimeIntervals = append(p.MuteTimeIntervals, config.MuteTimeInterval{
					Name: "ti-1",
				})
				p.Route.Routes = append(p.Route.Routes, &Route{
					Matchers: config.Matchers{
						{
							Type:  labels.MatchEqual,
							Name:  "label",
							Value: "test",
						},
					},
					MuteTimeIntervals:   []string{"ti-1"},
					ActiveTimeIntervals: []string{"mti-1"},
				})
			}),
			expected: MergeResult{
				Config: *load(t, fullMergedConfig, func(p *PostableApiAlertingConfig) {
					p.TimeIntervals = append(p.TimeIntervals, config.TimeInterval{
						Name: "mti-1_mimir-12345",
					})
					p.MuteTimeIntervals = append(p.MuteTimeIntervals, config.MuteTimeInterval{
						Name: "ti-1_mimir-12345",
					})
					p.Route.Routes[0].Routes = append(p.Route.Routes[0].Routes, &Route{
						Matchers: config.Matchers{
							{
								Type:  labels.MatchEqual,
								Name:  "label",
								Value: "test",
							},
						},
						MuteTimeIntervals:   []string{"ti-1_mimir-12345"},
						ActiveTimeIntervals: []string{"mti-1_mimir-12345"},
					})
				}),
				RenamedTimeIntervals: map[string]string{
					"ti-1":  "ti-1_mimir-12345",
					"mti-1": "mti-1_mimir-12345",
				},
			},
		},
		{
			name: "should fail if merging matchers conflict with Grafana, exact match",
			grafana: load(t, fullGrafanaConfig, func(p *PostableApiAlertingConfig) {
				p.Route.Routes = append(p.Route.Routes, &Route{
					Matchers: opts.SubtreeMatchers,
				})
			}),
			mimir:       load(t, fullMimirConfig),
			expectedErr: ErrSubtreeMatchersConflict,
		},
		{
			name: "should fail if merging matchers conflict with Grafana, subset match",
			grafana: load(t, fullGrafanaConfig, func(p *PostableApiAlertingConfig) {
				m, err := labels.NewMatcher(labels.MatchEqual, "label", "test")
				require.NoError(t, err)
				p.Route.Routes = append(p.Route.Routes, &Route{
					Matchers: append(opts.SubtreeMatchers, m),
				})
			}),
			mimir:       load(t, fullMimirConfig),
			expectedErr: ErrSubtreeMatchersConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Merge(*tc.grafana, *tc.mimir, opts)
			if tc.expectedErr != nil {
				if err == nil {
					data, err := yaml.Marshal(result.Config)
					require.NoError(t, err)
					t.Fatalf("Expected error but got result. YAML:\n%v", string(data))
				}
				assert.ErrorIs(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			tc.expected.Config.Global = nil

			diff := cmp.Diff(tc.expected, result,
				cmpopts.IgnoreUnexported(commoncfg.ProxyConfig{}, labels.Matcher{}),
				cmpopts.SortSlices(func(a, b *labels.Matcher) bool {
					return a.Name < b.Name
				}),
				cmpopts.SortSlices(func(a, b *PostableApiReceiver) bool {
					return a.Name < b.Name
				}),
				cmpopts.EquateEmpty(),
			)
			if !assert.Empty(t, diff) {
				data, err := yaml.Marshal(result.Config)
				require.NoError(t, err)
				t.Fatalf("YAML:\n%v", string(data))
			}
		})
	}
}

func TestCheckIfMatchersUsed(t *testing.T) {
	m := config.Matchers{
		{
			Type:  labels.MatchEqual,
			Name:  "al",
			Value: "av",
		},
		{
			Type:  labels.MatchEqual,
			Name:  "bl",
			Value: "bv",
		},
	}

	mustMatcher := func(mt labels.MatchType, n, v string) *labels.Matcher {
		m, err := labels.NewMatcher(mt, n, v)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}

	testCases := []struct {
		name     string
		route    *Route
		expected bool
	}{
		{
			name: "true if the same matchers",
			route: &Route{
				Matchers: m,
			},
			expected: true,
		},
		{
			name: "true if sub set of matchers",
			route: &Route{
				Matchers: config.Matchers{
					{
						Type:  labels.MatchEqual,
						Name:  "al",
						Value: "av",
					},
				},
			},
			expected: true,
		},
		{
			name: "true if regex that matches",
			route: &Route{
				Matchers: append(m, mustMatcher(labels.MatchRegexp, "al", ".*")),
			},
			expected: true,
		},
		{
			name: "true if superset of matchers",
			route: &Route{
				Matchers: append(m, &labels.Matcher{
					Type:  labels.MatchEqual,
					Name:  "cl",
					Value: "cv",
				}),
			},
			expected: true,
		},
		{
			name: "false if different matchers",
			route: &Route{
				Matchers: config.Matchers{
					{
						Type:  labels.MatchEqual,
						Name:  "al",
						Value: "test",
					},
					{
						Type:  labels.MatchEqual,
						Name:  "bl",
						Value: "bv",
					},
				},
			},
			expected: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := checkIfMatchersUsed(m, []*Route{tc.route})
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func load(t *testing.T, yaml string, mutate ...func(p *PostableApiAlertingConfig)) *PostableApiAlertingConfig {
	t.Helper()
	p, err := LoadCompat([]byte(yaml))
	require.NoError(t, err)
	for _, m := range mutate {
		m(p)
	}
	return p
}

const fullGrafanaConfig = `
mute_time_intervals:
  - name: mti-1
    time_intervals:
    - times:
      - start_time: 00:00
        end_time: 12:00
time_intervals:
  - name: ti-1
    time_intervals:
    - weekdays:
      - saturday
      - sunday
inhibit_rules:
    - source_matchers:
        - alertname="test"
        - cluster="test1"
      target_matchers:
        - alertname="test2"
        - cluster="test1"
      equal:
        - namespace
route:
  receiver: grafana-default-email
  group_by: [test, test2]
  group_wait: 1m
  group_interval: 1m
  repeat_interval: 1m
  routes:
  - receiver: test-webhook
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
    - mti-1
    active_time_intervals:
    - ti-1
receivers:
  - name: grafana-default-email
    grafana_managed_receiver_configs:
      - uid: uxwfZvtnz
        type: email
        disableResolveMessage: false
        settings:
          addresses: "<example@email.com>"
        secureFields: {}
  - name: test-webhook
    grafana_managed_receiver_configs:
      - uid: 12345
        type: webhook
        disableResolveMessage: false
        settings:
          url: "http://localhost/api/v1/alerts"
        secureFields: {}
`

const fullMimirConfig = `
mute_time_intervals:
  - name: mti-2
    time_intervals:
      - times:
          - start_time: 00:00
            end_time: 12:00
time_intervals:
  - name: ti-2
    time_intervals:
    - weekdays:
      - monday
      - tuesday
      - wednesday
      - thursday
      - friday
inhibit_rules:
    - source_matchers:
        - alertname="test"
      target_matchers:
        - servicename="test2"
      equal:
        - namespace
route:
  receiver: recv
  group_by:
    - alertname
    - groupby
  group_wait: 65s
  group_interval: 20m
  repeat_interval: 10h   
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
        - mti-2
      active_time_intervals:
        - ti-2
receivers:
  - name: recv
    email_configs:
      - to: recv
        smarthost: smtp.example.org:587
        from: email@example.com
  - name: recv2
    webhook_configs:
      - url: http://localhost
`

const fullMergedConfig = `
route:
    receiver: grafana-default-email
    group_by:
        - test
        - test2
    group_wait: 1m
    group_interval: 1m
    repeat_interval: 1m
    routes:
        - receiver: recv
          group_by:
            - alertname
            - groupby
          group_wait: 65s
          group_interval: 20m
          repeat_interval: 10h        
          matchers:
            - __mimir__="true"
            - __datasource_uid__="12345"
          routes:
            - receiver: recv2
              group_by:
                - teste
                - test2f
              object_matchers:
                - - team
                  - =
                  - teamC
              mute_time_intervals:
                - mti-2
              active_time_intervals:
                - ti-2
              group_wait: 0s
              group_interval: 1m
              repeat_interval: 1m
        - receiver: test-webhook
          group_by:
            - teste
            - test2f
          object_matchers:
            - - team
              - =
              - teamC
          mute_time_intervals:
            - mti-1
          active_time_intervals:
            - ti-1
          group_wait: 0s
          group_interval: 1m
          repeat_interval: 1m
mute_time_intervals:
    - name: mti-1
      time_intervals:
        - times:
            - start_time: "00:00"
              end_time: "12:00"
    - name: mti-2
      time_intervals:
        - times:
            - start_time: "00:00"
              end_time: "12:00"
time_intervals:
    - name: ti-1
      time_intervals:
        - weekdays: 
          - saturday
          - sunday
    - name: ti-2
      time_intervals:
        - weekdays:
          - monday
          - tuesday
          - wednesday
          - thursday
          - friday
receivers:
    - name: grafana-default-email
      grafana_managed_receiver_configs:
        - uid: uxwfZvtnz
          type: email
          disableResolveMessage: false
          settings:
            addresses: <example@email.com>
    - name: test-webhook
      grafana_managed_receiver_configs:
        - uid: "12345"
          type: webhook
          disableResolveMessage: false
          settings:
            url: http://localhost/api/v1/alerts
    - name: recv
      email_configs:
        - to: recv
          from: email@example.com
          smarthost: smtp.example.org:587
    - name: recv2
      webhook_configs:
        - url: http://localhost
inhibit_rules:
    - source_matchers:
        - alertname="test"
        - cluster="test1"
      target_matchers:
        - alertname="test2"
        - cluster="test1"
      equal:
        - namespace
    - source_matchers:
        - alertname="test"
        - __datasource_uid__="12345"
        - __mimir__="true"
      target_matchers:
        - servicename="test2"
        - __datasource_uid__="12345"
        - __mimir__="true"
      equal:
        - namespace

`
