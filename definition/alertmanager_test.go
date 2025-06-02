package definition

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/featurecontrol"
	"github.com/prometheus/alertmanager/matchers/compat"
	"github.com/prometheus/alertmanager/pkg/labels"
)

func Test_ApiReceiver_Marshaling(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		input PostableApiReceiver
		err   bool
	}{
		{
			desc: "success AM",
			input: PostableApiReceiver{
				Receiver: config.Receiver{
					Name: "foo",
					EmailConfigs: []*config.EmailConfig{{
						To:      "test@test.com",
						HTML:    config.DefaultEmailConfig.HTML,
						Headers: map[string]string{},
					}},
				},
			},
		},
		{
			desc: "success GM",
			input: PostableApiReceiver{
				Receiver: config.Receiver{
					Name: "foo",
				},
				PostableGrafanaReceivers: PostableGrafanaReceivers{
					GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
				},
			},
		},
		{
			desc: "failure mixed",
			input: PostableApiReceiver{
				Receiver: config.Receiver{
					Name: "foo",
					EmailConfigs: []*config.EmailConfig{{
						To:      "test@test.com",
						HTML:    config.DefaultEmailConfig.HTML,
						Headers: map[string]string{},
					}},
				},
				PostableGrafanaReceivers: PostableGrafanaReceivers{
					GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
				},
			},
			err: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			encoded, err := json.Marshal(tc.input)
			require.Nil(t, err)

			var out PostableApiReceiver
			err = json.Unmarshal(encoded, &out)

			if tc.err {
				require.Error(t, err)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.input, out)
			}
		})
	}
}

func Test_APIReceiverType(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		input    PostableApiReceiver
		expected ReceiverType
	}{
		{
			desc: "empty",
			input: PostableApiReceiver{
				Receiver: config.Receiver{
					Name: "foo",
				},
			},
			expected: EmptyReceiverType,
		},
		{
			desc: "am",
			input: PostableApiReceiver{
				Receiver: config.Receiver{
					Name: "foo",
					EmailConfigs: []*config.EmailConfig{{
						To:      "test@test.com",
						HTML:    config.DefaultEmailConfig.HTML,
						Headers: map[string]string{},
					}},
				},
			},
			expected: AlertmanagerReceiverType,
		},
		{
			desc: "graf",
			input: PostableApiReceiver{
				Receiver: config.Receiver{
					Name: "foo",
				},
				PostableGrafanaReceivers: PostableGrafanaReceivers{
					GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
				},
			},
			expected: GrafanaReceiverType,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.input.Type())
		})
	}
}

func Test_AllReceivers(t *testing.T) {
	input := &Route{
		Receiver: "foo",
		Routes: []*Route{
			{
				Receiver: "bar",
				Routes: []*Route{
					{
						Receiver: "bazz",
					},
				},
			},
			{
				Receiver: "buzz",
			},
		},
	}

	require.Equal(t, []string{"foo", "bar", "bazz", "buzz"}, AllReceivers(input.AsAMRoute()))

	// test empty
	var empty []string
	emptyRoute := &Route{}
	require.Equal(t, empty, AllReceivers(emptyRoute.AsAMRoute()))
}

func Test_ApiAlertingConfig_Marshaling(t *testing.T) {
	defaultGlobalConfig := config.DefaultGlobalConfig()
	for _, tc := range []struct {
		desc  string
		input PostableApiAlertingConfig
		err   bool
	}{
		{
			desc: "success am",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Receiver: "am",
						Routes: []*Route{
							{
								Receiver: "am",
							},
						},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "am",
							EmailConfigs: []*config.EmailConfig{{
								To:      "test@test.com",
								HTML:    config.DefaultEmailConfig.HTML,
								Headers: map[string]string{},
							}},
						},
					},
				},
			},
		},
		{
			desc: "success graf",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Receiver: "graf",
						Routes: []*Route{
							{
								Receiver: "graf",
							},
						},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "graf",
						},
						PostableGrafanaReceivers: PostableGrafanaReceivers{
							GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
						},
					},
				},
			},
		},
		{
			desc: "failure undefined am receiver",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Receiver: "am",
						Routes: []*Route{
							{
								Receiver: "unmentioned",
							},
						},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "am",
							EmailConfigs: []*config.EmailConfig{{
								To:      "test@test.com",
								HTML:    config.DefaultEmailConfig.HTML,
								Headers: map[string]string{},
							}},
						},
					},
				},
			},
			err: true,
		},
		{
			desc: "failure undefined graf receiver",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Receiver: "graf",
						Routes: []*Route{
							{
								Receiver: "unmentioned",
							},
						},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "graf",
						},
						PostableGrafanaReceivers: PostableGrafanaReceivers{
							GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
						},
					},
				},
			},
			err: true,
		},
		{
			desc: "failure graf no route",
			input: PostableApiAlertingConfig{
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "graf",
						},
						PostableGrafanaReceivers: PostableGrafanaReceivers{
							GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
						},
					},
				},
			},
			err: true,
		},
		{
			desc: "failure graf no default receiver",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Routes: []*Route{
							{
								Receiver: "graf",
							},
						},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "graf",
						},
						PostableGrafanaReceivers: PostableGrafanaReceivers{
							GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
						},
					},
				},
			},
			err: true,
		},
		{
			desc: "failure graf root route with matchers",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Receiver: "graf",
						Routes: []*Route{
							{
								Receiver: "graf",
							},
						},
						Match: map[string]string{"foo": "bar"},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "graf",
						},
						PostableGrafanaReceivers: PostableGrafanaReceivers{
							GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
						},
					},
				},
			},
			err: true,
		},
		{
			desc: "failure graf nested route duplicate group by labels",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Receiver: "graf",
						Routes: []*Route{
							{
								Receiver:   "graf",
								GroupByStr: []string{"foo", "bar", "foo"},
							},
						},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "graf",
						},
						PostableGrafanaReceivers: PostableGrafanaReceivers{
							GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
						},
					},
				},
			},
			err: true,
		},
		{
			desc: "success undefined am receiver in autogenerated route is ignored",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Receiver: "am",
						Routes: []*Route{
							{
								Matchers: config.Matchers{
									{
										Name:  autogeneratedRouteLabel,
										Type:  labels.MatchEqual,
										Value: "true",
									},
								},
								Routes: []*Route{
									{
										Receiver: "unmentioned",
									},
								},
							},
						},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "am",
							EmailConfigs: []*config.EmailConfig{{
								To:      "test@test.com",
								HTML:    config.DefaultEmailConfig.HTML,
								Headers: map[string]string{},
							}},
						},
					},
				},
			},
			err: false,
		},
		{
			desc: "success undefined graf receiver in autogenerated route is ignored",
			input: PostableApiAlertingConfig{
				Config: Config{
					Global: &defaultGlobalConfig,
					Route: &Route{
						Receiver: "graf",
						Routes: []*Route{
							{
								Matchers: config.Matchers{
									{
										Name:  autogeneratedRouteLabel,
										Type:  labels.MatchEqual,
										Value: "true",
									},
								},
								Routes: []*Route{
									{
										Receiver: "unmentioned",
									},
								},
							},
						},
					},
				},
				Receivers: []*PostableApiReceiver{
					{
						Receiver: config.Receiver{
							Name: "graf",
						},
						PostableGrafanaReceivers: PostableGrafanaReceivers{
							GrafanaManagedReceivers: []*PostableGrafanaReceiver{{}},
						},
					},
				},
			},
			err: false,
		},
	} {
		t.Run(tc.desc+" (json)", func(t *testing.T) {
			encoded, err := json.Marshal(tc.input)
			require.Nil(t, err)

			cfg, err := Load(encoded)
			if tc.err {
				require.Error(t, err)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.input, *cfg)
			}
		})

		t.Run(tc.desc+" (yaml)", func(t *testing.T) {
			encoded, err := yaml.Marshal(tc.input)
			require.Nil(t, err)

			cfg, err := Load(encoded)
			if tc.err {
				require.Error(t, err)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.input, *cfg)
			}
		})
	}
}

func Test_PostableApiReceiver_Unmarshaling_YAML(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		input string
		rtype ReceiverType
	}{
		{
			desc: "grafana receivers",
			input: `
name: grafana_managed
grafana_managed_receiver_configs:
  - uid: alertmanager UID
    name: an alert manager receiver
    type: prometheus-alertmanager
    sendreminder: false
    disableresolvemessage: false
    frequency: 5m
    isdefault: false
    settings: {}
    securesettings:
      basicAuthPassword: <basicAuthPassword>
  - uid: dingding UID
    name: a dingding receiver
    type: dingding
    sendreminder: false
    disableresolvemessage: false
    frequency: 5m
    isdefault: false`,
			rtype: GrafanaReceiverType,
		},
		{
			desc: "receiver",
			input: `
name: example-email
email_configs:
  - to: 'youraddress@example.org'`,
			rtype: AlertmanagerReceiverType,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var r PostableApiReceiver
			err := yaml.Unmarshal([]byte(tc.input), &r)
			require.Nil(t, err)
			assert.Equal(t, tc.rtype, r.Type())
		})
	}
}

func Test_ConfigUnmashaling(t *testing.T) {
	for _, tc := range []struct {
		desc, input string
		err         error
	}{
		{
			desc: "missing mute time interval name should error",
			err:  errors.New("missing name in mute time interval"),
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email"
				  },
				  "mute_time_intervals": [
					{
					  "name": "",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					}
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
		{
			desc: "missing time interval name should error",
			err:  errors.New("missing name in time interval"),
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email"
				  },
				  "time_intervals": [
					{
					  "name": "",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					}
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
		{
			desc: "duplicate mute time interval names should error",
			err:  errors.New("mute time interval \"test1\" is not unique"),
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email"
				  },
				  "mute_time_intervals": [
					{
					  "name": "test1",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					},
					{
						"name": "test1",
						"time_intervals": [
						  {
							"times": [
							  {
								"start_time": "00:00",
								"end_time": "12:00"
							  }
							]
						  }
						]
					  }
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
		{
			desc: "duplicate time interval names should error",
			err:  errors.New("time interval \"test1\" is not unique"),
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email"
				  },
				  "time_intervals": [
					{
					  "name": "test1",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					},
					{
						"name": "test1",
						"time_intervals": [
						  {
							"times": [
							  {
								"start_time": "00:00",
								"end_time": "12:00"
							  }
							]
						  }
						]
					  }
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
		{
			desc: "duplicate time and mute time interval names should error",
			err:  errors.New("time interval \"test1\" is not unique"),
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email"
				  },
				  "mute_time_intervals": [
					{
					  "name": "test1",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					}
				  ],
				  "time_intervals": [
					{
						"name": "test1",
						"time_intervals": [
						  {
							"times": [
							  {
								"start_time": "00:00",
								"end_time": "12:00"
							  }
							]
						  }
						]
				    }
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
		{
			desc: "mute time intervals on root route should error",
			err:  errors.New("root route must not have any mute time intervals"),
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email",
					"mute_time_intervals": ["test1"]
				  },
				  "mute_time_intervals": [
					{
					  "name": "test1",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					}
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
		{
			desc: "undefined mute time names in routes should error",
			err:  errors.New("undefined mute time interval \"test2\" used in route"),
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email",
					"routes": [
						{
						  "receiver": "grafana-default-email",
						  "object_matchers": [
							[
							  "a",
							  "=",
							  "b"
							]
						  ],
						  "mute_time_intervals": [
							"test2"
						  ]
						}
					  ]
				  },
				  "mute_time_intervals": [
					{
					  "name": "test1",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					}
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
		{
			desc: "undefined active time names in routes should error",
			err:  errors.New("undefined active time interval \"test2\" used in route"),
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email",
					"routes": [
						{
						  "receiver": "grafana-default-email",
						  "object_matchers": [
							[
							  "a",
							  "=",
							  "b"
							]
						  ],
						  "active_time_intervals": [
							"test2"
						  ]
						}
					  ]
				  },
				  "time_intervals": [
					{
					  "name": "test1",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					}
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
		{
			desc: "valid config should not error",
			input: `
				{
				  "route": {
					"receiver": "grafana-default-email",
					"routes": [
						{
						  "receiver": "grafana-default-email",
						  "object_matchers": [
							[
							  "a",
							  "=",
							  "b"
							]
						  ],
						  "mute_time_intervals": [
							"test1"
						  ]
						}
					  ]
				  },
				  "mute_time_intervals": [
					{
					  "name": "test1",
					  "time_intervals": [
						{
						  "times": [
							{
							  "start_time": "00:00",
							  "end_time": "12:00"
							}
						  ]
						}
					  ]
					}
				  ],
				  "templates": null,
				  "receivers": [
					{
					  "name": "grafana-default-email",
					  "grafana_managed_receiver_configs": [
						{
						  "uid": "uxwfZvtnz",
						  "name": "email receiver",
						  "type": "email",
						  "disableResolveMessage": false,
						  "settings": {
							"addresses": "<example@email.com>"
						  },
						  "secureFields": {}
						}
					  ]
					}
				  ]
				}
			`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var out Config
			err := json.Unmarshal([]byte(tc.input), &out)
			require.Equal(t, tc.err, err)
		})
	}
}

func Test_ReceiverCompatibility(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		a, b     ReceiverType
		expected bool
	}{
		{
			desc:     "grafana=grafana",
			a:        GrafanaReceiverType,
			b:        GrafanaReceiverType,
			expected: true,
		},
		{
			desc:     "am=am",
			a:        AlertmanagerReceiverType,
			b:        AlertmanagerReceiverType,
			expected: true,
		},
		{
			desc:     "empty=grafana",
			a:        EmptyReceiverType,
			b:        AlertmanagerReceiverType,
			expected: true,
		},
		{
			desc:     "empty=am",
			a:        EmptyReceiverType,
			b:        AlertmanagerReceiverType,
			expected: true,
		},
		{
			desc:     "empty=empty",
			a:        EmptyReceiverType,
			b:        EmptyReceiverType,
			expected: true,
		},
		{
			desc:     "graf!=am",
			a:        GrafanaReceiverType,
			b:        AlertmanagerReceiverType,
			expected: false,
		},
		{
			desc:     "am!=graf",
			a:        AlertmanagerReceiverType,
			b:        GrafanaReceiverType,
			expected: false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.a.Can(tc.b))
		})
	}
}

func Test_ReceiverMatchesBackend(t *testing.T) {
	for _, tc := range []struct {
		desc string
		rec  ReceiverType
		b    ReceiverType
		ok   bool
	}{
		{
			desc: "graf=graf",
			rec:  GrafanaReceiverType,
			b:    GrafanaReceiverType,
			ok:   true,
		},
		{
			desc: "empty=graf",
			rec:  EmptyReceiverType,
			b:    GrafanaReceiverType,
			ok:   true,
		},
		{
			desc: "am=am",
			rec:  AlertmanagerReceiverType,
			b:    AlertmanagerReceiverType,
			ok:   true,
		},
		{
			desc: "empty=am",
			rec:  EmptyReceiverType,
			b:    AlertmanagerReceiverType,
			ok:   true,
		},
		{
			desc: "graf!=am",
			rec:  GrafanaReceiverType,
			b:    AlertmanagerReceiverType,
			ok:   false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ok := tc.rec.Can(tc.b)
			require.Equal(t, tc.ok, ok)
		})
	}
}

func TestObjectMatchers_UnmarshalJSON(t *testing.T) {
	j := `{
		"receiver": "autogen-contact-point-default",
		"routes": [{
			"receiver": "autogen-contact-point-1",
			"object_matchers": [
				[
					"a",
					"=",
					"MFR3Gxrnk"
				],
				[
					"b",
					"=",
					"\"MFR3Gxrnk\""
				],
				[
					"c",
					"=~",
					"^[a-z0-9-]{1}[a-z0-9-]{0,30}$"
				],
				[
					"d",
					"=~",
					"\"^[a-z0-9-]{1}[a-z0-9-]{0,30}$\""
				]
			],
			"group_interval": "3s",
			"repeat_interval": "10s"
		}]
}`
	var r Route
	if err := json.Unmarshal([]byte(j), &r); err != nil {
		require.NoError(t, err)
	}

	matchers := r.Routes[0].ObjectMatchers

	// Without quotes.
	require.Equal(t, matchers[0].Name, "a")
	require.Equal(t, matchers[0].Value, "MFR3Gxrnk")

	// With double quotes.
	require.Equal(t, matchers[1].Name, "b")
	require.Equal(t, matchers[1].Value, "MFR3Gxrnk")

	// Regexp without quotes.
	require.Equal(t, matchers[2].Name, "c")
	require.Equal(t, matchers[2].Value, "^[a-z0-9-]{1}[a-z0-9-]{0,30}$")

	// Regexp with quotes.
	require.Equal(t, matchers[3].Name, "d")
	require.Equal(t, matchers[3].Value, "^[a-z0-9-]{1}[a-z0-9-]{0,30}$")
}

func TestObjectMatchers_UnmarshalYAML(t *testing.T) {
	y := `---
receiver: autogen-contact-point-default
routes:
- receiver: autogen-contact-point-1
  object_matchers:
  - - a
    - "="
    - MFR3Gxrnk
  - - b
    - "="
    - '"MFR3Gxrnk"'
  - - c
    - "=~"
    - "^[a-z0-9-]{1}[a-z0-9-]{0,30}$"
  - - d
    - "=~"
    - '"^[a-z0-9-]{1}[a-z0-9-]{0,30}$"'
  group_interval: 3s
  repeat_interval: 10s
`

	var r Route
	if err := yaml.Unmarshal([]byte(y), &r); err != nil {
		require.NoError(t, err)
	}

	matchers := r.Routes[0].ObjectMatchers

	// Without quotes.
	require.Equal(t, matchers[0].Name, "a")
	require.Equal(t, matchers[0].Value, "MFR3Gxrnk")

	// With double quotes.
	require.Equal(t, matchers[1].Name, "b")
	require.Equal(t, matchers[1].Value, "MFR3Gxrnk")

	// Regexp without quotes.
	require.Equal(t, matchers[2].Name, "c")
	require.Equal(t, matchers[2].Value, "^[a-z0-9-]{1}[a-z0-9-]{0,30}$")

	// Regexp with quotes.
	require.Equal(t, matchers[3].Name, "d")
	require.Equal(t, matchers[3].Value, "^[a-z0-9-]{1}[a-z0-9-]{0,30}$")
}

func Test_RawMessageMarshaling(t *testing.T) {
	type Data struct {
		Field RawMessage `json:"field" yaml:"field"`
	}

	t.Run("should unmarshal nil", func(t *testing.T) {
		v := Data{
			Field: nil,
		}
		data, err := json.Marshal(v)
		require.NoError(t, err)
		assert.JSONEq(t, `{ "field": null }`, string(data))

		var n Data
		require.NoError(t, json.Unmarshal(data, &n))
		assert.Equal(t, RawMessage("null"), n.Field)

		data, err = yaml.Marshal(&v)
		require.NoError(t, err)
		assert.Equal(t, "field: null\n", string(data))

		require.NoError(t, yaml.Unmarshal(data, &n))
		assert.Nil(t, n.Field)
	})

	t.Run("should unmarshal value", func(t *testing.T) {
		v := Data{
			Field: RawMessage(`{ "data": "test"}`),
		}
		data, err := json.Marshal(v)
		require.NoError(t, err)
		assert.JSONEq(t, `{"field":{"data":"test"}}`, string(data))

		var n Data
		require.NoError(t, json.Unmarshal(data, &n))
		assert.Equal(t, RawMessage(`{"data":"test"}`), n.Field)

		data, err = yaml.Marshal(&v)
		require.NoError(t, err)
		assert.Equal(t, "field:\n    data: test\n", string(data))

		require.NoError(t, yaml.Unmarshal(data, &n))
		assert.Equal(t, RawMessage(`{"data":"test"}`), n.Field)
	})
}

func TestDecryptSecureSettings(t *testing.T) {
	const testValue = "test-value-1"
	fakeDecryptFn := func(payload []byte) ([]byte, error) {
		if string(payload) == testValue {
			return []byte(testValue), nil
		}
		return nil, errors.New("key not found")
	}

	tests := []struct {
		name              string
		receiver          *PostableGrafanaReceiver
		expSecureSettings map[string]string
		expErr            string
	}{
		{
			name: "no secure settings",
			receiver: &PostableGrafanaReceiver{
				SecureSettings: map[string]string{},
			},
			expSecureSettings: map[string]string{},
			expErr:            "",
		},
		{
			name: "secure settings are not base64 encoded",
			receiver: &PostableGrafanaReceiver{
				SecureSettings: map[string]string{"test": "test"},
			},
			expSecureSettings: map[string]string{},
			expErr:            "failed to decrypt value for key 'test': key not found",
		},
		{
			name: "key not found",
			receiver: &PostableGrafanaReceiver{
				SecureSettings: map[string]string{"test": "test"},
			},
			expSecureSettings: map[string]string{},
			expErr:            "failed to decrypt value for key 'test': key not found",
		},
		{
			name: "illegal base64 value",
			receiver: &PostableGrafanaReceiver{
				SecureSettings: map[string]string{
					"test": "invalid value",
				},
			},
			expSecureSettings: map[string]string{
				"test": testValue,
			},
			expErr: "failed to decode value for key 'test': illegal base64 data at input byte 7",
		},
		{
			name: "second key not found",
			receiver: &PostableGrafanaReceiver{
				SecureSettings: map[string]string{
					"test1": base64.StdEncoding.EncodeToString([]byte(testValue)),
					"test2": "notfound",
				},
			},
			expSecureSettings: map[string]string{
				"test": testValue,
			},
			expErr: "failed to decrypt value for key 'test2': key not found",
		},
		{
			name: "success case",
			receiver: &PostableGrafanaReceiver{
				SecureSettings: map[string]string{
					"test": base64.StdEncoding.EncodeToString([]byte(testValue)),
				},
			},
			expSecureSettings: map[string]string{
				"test": testValue,
			},
			expErr: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := test.receiver.DecryptSecureSettings(fakeDecryptFn)
			if test.expErr != "" {
				require.NotNil(t, err)
				require.Equal(t, test.expErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, test.expSecureSettings, res)
		})
	}
}

func TestInhibitRule_Unmarshal_JSON(t *testing.T) {
	s := `{
 	"route": {
 		"receiver": "test"
 	},
 	"inhibit_rules": [{
 			"source_matchers": ["foo=bar"],
 			"target_matchers": ["bar=baz"],
 			"equal": ["qux", "corge"]
 	}]
 }`
	var c Config
	require.NoError(t, json.Unmarshal([]byte(s), &c))
	require.Len(t, c.InhibitRules, 1)
	r := c.InhibitRules[0]
	require.Equal(t, config.Matchers{{
		Name:  "foo",
		Type:  labels.MatchEqual,
		Value: "bar",
	}}, r.SourceMatchers)
	require.Equal(t, config.Matchers{{
		Name:  "bar",
		Type:  labels.MatchEqual,
		Value: "baz",
	}}, r.TargetMatchers)
	require.Equal(t, []string{"qux", "corge"}, r.Equal)
}

// This test asserts the same as [TestInhibitRule_Unmarshal_JSON],
// however it also checks that UTF-8 characters are allowed in the
// Equal field of an inhibition rule when UTF-8 strict mode is
// enabled.
func TestInhibitRule_UTF8_In_Equals_Unmarshal_JSON(t *testing.T) {
	s := `{
 	"route": {
 		"receiver": "test"
 	},
 	"inhibit_rules": [{
 			"source_matchers": ["foo=bar"],
 			"target_matchers": ["bar=baz"],
 			"equal": ["qux", "corge🙂"]
 	}]
 }`
	var c Config
	// Should return an error as UTF-8 mode is not enabled.
	require.EqualError(t, json.Unmarshal([]byte(s), &c), "invalid label name \"corge🙂\" in equal list")

	// Change the mode to UTF-8 mode.
	ff, err := featurecontrol.NewFlags(log.NewNopLogger(), featurecontrol.FeatureUTF8StrictMode)
	require.NoError(t, err)
	compat.InitFromFlags(log.NewNopLogger(), ff)

	// Restore the mode to classic at the end of the test.
	ff, err = featurecontrol.NewFlags(log.NewNopLogger(), featurecontrol.FeatureClassicMode)
	require.NoError(t, err)
	defer compat.InitFromFlags(log.NewNopLogger(), ff)

	require.NoError(t, json.Unmarshal([]byte(s), &c))
	require.Len(t, c.InhibitRules, 1)
	r := c.InhibitRules[0]
	require.Equal(t, config.Matchers{{
		Name:  "foo",
		Type:  labels.MatchEqual,
		Value: "bar",
	}}, r.SourceMatchers)
	require.Equal(t, config.Matchers{{
		Name:  "bar",
		Type:  labels.MatchEqual,
		Value: "baz",
	}}, r.TargetMatchers)
	require.Equal(t, []string{"qux", "corge🙂"}, r.Equal)
}

func TestInhibitRule_Marshal_JSON(t *testing.T) {
	r := config.InhibitRule{
		SourceMatchers: config.Matchers{{
			Name:  "foo",
			Type:  labels.MatchEqual,
			Value: "bar",
		}},
		TargetMatchers: config.Matchers{{
			Name:  "bar",
			Type:  labels.MatchEqual,
			Value: "baz",
		}},
		Equal: []string{"qux", "corge🙂"},
	}
	b, err := json.Marshal(r)
	require.NoError(t, err)
	require.Equal(t, `{"source_matchers":["foo=\"bar\""],"target_matchers":["bar=\"baz\""],"equal":["qux","corge🙂"]}`, string(b))
}

// This test asserts that the correct deserialization is applied when decoding
// a YAML configuration containing inhibition rules with equals labels.
func TestInhibitRule_Unmarshal_YAML(t *testing.T) {
	s := `
 route:
   receiver: test
 inhibit_rules:
 - source_matchers:
     - foo=bar
   target_matchers:
     - bar=baz
   equal:
     - qux
     - corge
 `
	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(s), &c))
	require.Len(t, c.InhibitRules, 1)
	r := c.InhibitRules[0]
	t.Log(r)
	require.Equal(t, config.Matchers{{
		Name:  "foo",
		Type:  labels.MatchEqual,
		Value: "bar",
	}}, r.SourceMatchers)
	require.Equal(t, config.Matchers{{
		Name:  "bar",
		Type:  labels.MatchEqual,
		Value: "baz",
	}}, r.TargetMatchers)
	require.Equal(t, []string{"qux", "corge"}, r.Equal)
}

// This test asserts the same as [TestInhibitRule_Unmarshal_YAML],
// however it also checks that UTF-8 characters are allowed in the
// Equal field of an inhibition rule when UTF-8 strict mode is
// enabled.
func TestInhibitRule_UTF8_In_Equals_Unmarshal_YAML(t *testing.T) {
	s := `
 route:
   receiver: test
 inhibit_rules:
 - source_matchers:
     - foo=bar
   target_matchers:
     - bar=baz
   equal:
     - qux
     - corge🙂
 `
	var c Config
	require.EqualError(t, yaml.Unmarshal([]byte(s), &c), "invalid label name \"corge🙂\" in equal list")

	// Change the mode to UTF-8 mode.
	ff, err := featurecontrol.NewFlags(log.NewNopLogger(), featurecontrol.FeatureUTF8StrictMode)
	require.NoError(t, err)
	compat.InitFromFlags(log.NewNopLogger(), ff)

	// Restore the mode to classic at the end of the test.
	ff, err = featurecontrol.NewFlags(log.NewNopLogger(), featurecontrol.FeatureClassicMode)
	require.NoError(t, err)
	defer compat.InitFromFlags(log.NewNopLogger(), ff)

	require.NoError(t, yaml.Unmarshal([]byte(s), &c))
	require.Len(t, c.InhibitRules, 1)
	r := c.InhibitRules[0]
	require.Equal(t, config.Matchers{{
		Name:  "foo",
		Type:  labels.MatchEqual,
		Value: "bar",
	}}, r.SourceMatchers)
	require.Equal(t, config.Matchers{{
		Name:  "bar",
		Type:  labels.MatchEqual,
		Value: "baz",
	}}, r.TargetMatchers)
	require.Equal(t, []string{"qux", "corge🙂"}, r.Equal)
}

func TestInhibitRule_Marshal_YAML(t *testing.T) {
	r := config.InhibitRule{
		SourceMatchers: config.Matchers{{
			Name:  "foo",
			Type:  labels.MatchEqual,
			Value: "bar",
		}},
		TargetMatchers: config.Matchers{{
			Name:  "bar",
			Type:  labels.MatchEqual,
			Value: "baz",
		}},
		Equal: []string{"qux", "corge🙂"},
	}
	b, err := yaml.Marshal(r)
	require.NoError(t, err)
	require.Equal(t, `source_matchers:
    - foo="bar"
target_matchers:
    - bar="baz"
equal:
    - qux
    - "corge\U0001F642"
`, string(b))
}
