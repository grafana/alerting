package notify

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-openapi/strfmt"
	amv2 "github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/pkg/labels"
	"github.com/prometheus/alertmanager/provider/mem"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/notify/nfstatus"
	"github.com/grafana/alerting/receivers"
)

func setupAMTest(t *testing.T) (*GrafanaAlertmanager, *prometheus.Registry) {
	t.Helper()

	reg := prometheus.NewPedanticRegistry()

	opts := GrafanaAlertmanagerOpts{
		Silences: newFakeMaintanenceOptions(t),
		Nflog:    newFakeMaintanenceOptions(t),

		TenantKey:     "org",
		TenantID:      1,
		Peer:          &NilPeer{},
		Logger:        log.NewNopLogger(),
		Metrics:       NewGrafanaAlertmanagerMetrics(reg, log.NewNopLogger()),
		EmailSender:   receivers.MockNotificationService(),
		ImageProvider: images.NewFakeProvider(1),
		Decrypter:     NoopDecrypt,
	}

	require.NoError(t, opts.Validate())

	am, err := NewGrafanaAlertmanager(opts)
	require.NoError(t, err)
	return am, reg
}

func TestPutAlert(t *testing.T) {
	am, _ := setupAMTest(t)

	startTime := time.Now()
	endTime := startTime.Add(2 * time.Hour)

	cases := []struct {
		title          string
		postableAlerts amv2.PostableAlerts
		expAlerts      func(now time.Time) []*types.Alert
		expError       *AlertValidationError
	}{
		{
			title: "Valid alerts with different start/end set",
			postableAlerts: amv2.PostableAlerts{
				{ // Start and end set.
					Annotations: amv2.LabelSet{"msg": "Alert1 annotation"},
					Alert: amv2.Alert{
						Labels:       amv2.LabelSet{"alertname": "Alert1"},
						GeneratorURL: "http://localhost/url1",
					},
					StartsAt: strfmt.DateTime(startTime),
					EndsAt:   strfmt.DateTime(endTime),
				}, { // Only end is set.
					Annotations: amv2.LabelSet{"msg": "Alert2 annotation"},
					Alert: amv2.Alert{
						Labels:       amv2.LabelSet{"alertname": "Alert2"},
						GeneratorURL: "http://localhost/url2",
					},
					StartsAt: strfmt.DateTime{},
					EndsAt:   strfmt.DateTime(endTime),
				}, { // Only start is set.
					Annotations: amv2.LabelSet{"msg": "Alert3 annotation"},
					Alert: amv2.Alert{
						Labels:       amv2.LabelSet{"alertname": "Alert3"},
						GeneratorURL: "http://localhost/url3",
					},
					StartsAt: strfmt.DateTime(startTime),
					EndsAt:   strfmt.DateTime{},
				}, { // Both start and end are not set.
					Annotations: amv2.LabelSet{"msg": "Alert4 annotation"},
					Alert: amv2.Alert{
						Labels:       amv2.LabelSet{"alertname": "Alert4"},
						GeneratorURL: "http://localhost/url4",
					},
					StartsAt: strfmt.DateTime{},
					EndsAt:   strfmt.DateTime{},
				},
			},
			expAlerts: func(now time.Time) []*types.Alert {
				return []*types.Alert{
					{
						Alert: model.Alert{
							Annotations:  model.LabelSet{"msg": "Alert1 annotation"},
							Labels:       model.LabelSet{"alertname": "Alert1"},
							StartsAt:     startTime,
							EndsAt:       endTime,
							GeneratorURL: "http://localhost/url1",
						},
						UpdatedAt: now,
					}, {
						Alert: model.Alert{
							Annotations:  model.LabelSet{"msg": "Alert2 annotation"},
							Labels:       model.LabelSet{"alertname": "Alert2"},
							StartsAt:     endTime,
							EndsAt:       endTime,
							GeneratorURL: "http://localhost/url2",
						},
						UpdatedAt: now,
					}, {
						Alert: model.Alert{
							Annotations:  model.LabelSet{"msg": "Alert3 annotation"},
							Labels:       model.LabelSet{"alertname": "Alert3"},
							StartsAt:     startTime,
							EndsAt:       now.Add(defaultResolveTimeout),
							GeneratorURL: "http://localhost/url3",
						},
						UpdatedAt: now,
						Timeout:   true,
					}, {
						Alert: model.Alert{
							Annotations:  model.LabelSet{"msg": "Alert4 annotation"},
							Labels:       model.LabelSet{"alertname": "Alert4"},
							StartsAt:     now,
							EndsAt:       now.Add(defaultResolveTimeout),
							GeneratorURL: "http://localhost/url4",
						},
						UpdatedAt: now,
						Timeout:   true,
					},
				}
			},
		}, {
			title: "Removing empty labels and annotations",
			postableAlerts: amv2.PostableAlerts{
				{
					Annotations: amv2.LabelSet{"msg": "Alert4 annotation", "empty": ""},
					Alert: amv2.Alert{
						Labels:       amv2.LabelSet{"alertname": "Alert4", "emptylabel": ""},
						GeneratorURL: "http://localhost/url1",
					},
					StartsAt: strfmt.DateTime{},
					EndsAt:   strfmt.DateTime{},
				},
			},
			expAlerts: func(now time.Time) []*types.Alert {
				return []*types.Alert{
					{
						Alert: model.Alert{
							Annotations:  model.LabelSet{"msg": "Alert4 annotation"},
							Labels:       model.LabelSet{"alertname": "Alert4"},
							StartsAt:     now,
							EndsAt:       now.Add(defaultResolveTimeout),
							GeneratorURL: "http://localhost/url1",
						},
						UpdatedAt: now,
						Timeout:   true,
					},
				}
			},
		}, {
			title: "Allow spaces in label and annotation name",
			postableAlerts: amv2.PostableAlerts{
				{
					Annotations: amv2.LabelSet{"Dashboard URL": "http://localhost:3000"},
					Alert: amv2.Alert{
						Labels:       amv2.LabelSet{"alertname": "Alert4", "Spaced Label": "works"},
						GeneratorURL: "http://localhost/url1",
					},
					StartsAt: strfmt.DateTime{},
					EndsAt:   strfmt.DateTime{},
				},
			},
			expAlerts: func(now time.Time) []*types.Alert {
				return []*types.Alert{
					{
						Alert: model.Alert{
							Annotations:  model.LabelSet{"Dashboard URL": "http://localhost:3000"},
							Labels:       model.LabelSet{"alertname": "Alert4", "Spaced Label": "works"},
							StartsAt:     now,
							EndsAt:       now.Add(defaultResolveTimeout),
							GeneratorURL: "http://localhost/url1",
						},
						UpdatedAt: now,
						Timeout:   true,
					},
				}
			},
		}, {
			title: "Special characters in labels",
			postableAlerts: amv2.PostableAlerts{
				{
					Alert: amv2.Alert{
						Labels: amv2.LabelSet{"alertname$": "Alert1", "az3-- __...++!!!£@@312312": "1"},
					},
				},
			},
			expAlerts: func(now time.Time) []*types.Alert {
				return []*types.Alert{
					{
						Alert: model.Alert{
							Labels:       model.LabelSet{"alertname$": "Alert1", "az3-- __...++!!!£@@312312": "1"},
							Annotations:  model.LabelSet{},
							StartsAt:     now,
							EndsAt:       now.Add(defaultResolveTimeout),
							GeneratorURL: "",
						},
						UpdatedAt: now,
						Timeout:   true,
					},
				}
			},
		}, {
			title: "Special characters in annotations",
			postableAlerts: amv2.PostableAlerts{
				{
					Annotations: amv2.LabelSet{"az3-- __...++!!!£@@312312": "Alert4 annotation"},
					Alert: amv2.Alert{
						Labels: amv2.LabelSet{"alertname": "Alert4"},
					},
				},
			},
			expAlerts: func(now time.Time) []*types.Alert {
				return []*types.Alert{
					{
						Alert: model.Alert{
							Labels:       model.LabelSet{"alertname": "Alert4"},
							Annotations:  model.LabelSet{"az3-- __...++!!!£@@312312": "Alert4 annotation"},
							StartsAt:     now,
							EndsAt:       now.Add(defaultResolveTimeout),
							GeneratorURL: "",
						},
						UpdatedAt: now,
						Timeout:   true,
					},
				}
			},
		}, {
			title: "No labels after removing empty",
			postableAlerts: amv2.PostableAlerts{
				{
					Alert: amv2.Alert{
						Labels: amv2.LabelSet{"alertname": ""},
					},
				},
			},
			expError: &AlertValidationError{
				Alerts: amv2.PostableAlerts{
					{
						Alert: amv2.Alert{
							Labels: amv2.LabelSet{"alertname": ""},
						},
					},
				},
				Errors: []error{errors.New("at least one label pair required")},
			},
		}, {
			title: "Start should be before end",
			postableAlerts: amv2.PostableAlerts{
				{
					Alert: amv2.Alert{
						Labels: amv2.LabelSet{"alertname": ""},
					},
					StartsAt: strfmt.DateTime(endTime),
					EndsAt:   strfmt.DateTime(startTime),
				},
			},
			expError: &AlertValidationError{
				Alerts: amv2.PostableAlerts{
					{
						Alert: amv2.Alert{
							Labels: amv2.LabelSet{"alertname": ""},
						},
						StartsAt: strfmt.DateTime(endTime),
						EndsAt:   strfmt.DateTime(startTime),
					},
				},
				Errors: []error{errors.New("start time must be before end time")},
			},
		},
	}

	for _, c := range cases {
		var err error
		t.Run(c.title, func(t *testing.T) {
			r := prometheus.NewRegistry()
			am.marker = types.NewMarker(r)
			am.alerts, err = mem.NewAlerts(context.Background(), am.marker, 15*time.Minute, nil, am.logger, r)
			require.NoError(t, err)

			alerts := []*types.Alert{}
			err := am.PutAlerts(c.postableAlerts)
			if c.expError != nil {
				require.Error(t, err)
				require.Equal(t, c.expError, err)
				require.Equal(t, 0, len(alerts))
				return
			}
			require.NoError(t, err)

			iter := am.alerts.GetPending()
			defer iter.Close()
			for a := range iter.Next() {
				alerts = append(alerts, a)
			}

			// We take the "now" time from one of the UpdatedAt.
			now := alerts[0].UpdatedAt
			expAlerts := c.expAlerts(now)

			sort.Sort(types.AlertSlice(expAlerts))
			sort.Sort(types.AlertSlice(alerts))

			require.Equal(t, expAlerts, alerts)
		})
	}
}

func TestCreateSilence(t *testing.T) {
	am, _ := setupAMTest(t)

	cases := []struct {
		name    string
		silence PostableSilence
		expErr  string
	}{{
		name: "can create silence for foo=bar",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("foo"),
					Value:   ptr("bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
	}, {
		name: "can create silence for _foo1=bar",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("_foo1"),
					Value:   ptr("bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
	}, {
		name: "can create silence for 0foo=bar",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("0foo"),
					Value:   ptr("bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
	}, {
		name: "can create silence for foo=🙂bar",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("foo"),
					Value:   ptr("🙂bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
	}, {
		name: "can create silence for foo🙂=bar",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("foo🙂"),
					Value:   ptr("bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
	}, {
		name: "can't create silence for missing label name",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr(""),
					Value:   ptr("bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
		expErr: "unable to save silence: invalid silence: invalid label matcher 0: invalid label name \"\": unable to create silence",
	}, {
		name: "can't create silence for missing label value",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("foo"),
					Value:   ptr(""),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
		expErr: "unable to save silence: invalid silence: at least one matcher must not match the empty string: unable to create silence",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			silenceID, err := am.CreateSilence(&c.silence)
			if c.expErr != "" {
				require.EqualError(t, err, c.expErr)
				require.Empty(t, silenceID)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, silenceID)
			}
		})
	}
}

func TestUpsertSilence(t *testing.T) {
	am, _ := setupAMTest(t)

	cases := []struct {
		name    string
		silence PostableSilence
		expErr  string
	}{{
		name: "can create silence with pre-set ID",
		silence: PostableSilence{
			ID: "test_id",
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("foo"),
					Value:   ptr("bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
	}, {
		name: "can create silence without pre-set ID",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("foo"),
					Value:   ptr("bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
	}, {
		name: "can't create silence for missing label name",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr(""),
					Value:   ptr("bar"),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
		expErr: "unable to upsert silence: invalid silence: invalid label matcher 0: invalid label name \"\": unable to create silence",
	}, {
		name: "can't create silence for missing label value",
		silence: PostableSilence{
			Silence: amv2.Silence{
				Comment:   ptr("This is a comment"),
				CreatedBy: ptr("test"),
				EndsAt:    ptr(strfmt.DateTime(time.Now().Add(time.Minute))),
				Matchers: amv2.Matchers{{
					IsEqual: ptr(true),
					IsRegex: ptr(false),
					Name:    ptr("foo"),
					Value:   ptr(""),
				}},
				StartsAt: ptr(strfmt.DateTime(time.Now())),
			},
		},
		expErr: "unable to upsert silence: invalid silence: at least one matcher must not match the empty string: unable to create silence",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sID := c.silence.ID
			silenceID, err := am.UpsertSilence(&c.silence)
			if c.expErr != "" {
				require.EqualError(t, err, c.expErr)
				require.Empty(t, silenceID)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, silenceID)
				if sID != "" {
					require.Equal(t, sID, silenceID)
				}
			}
		})
	}
}

func TestGrafanaAlertmanager_setInhibitionRulesMetrics(t *testing.T) {
	am, reg := setupAMTest(t)

	m1, err := labels.NewMatcher(labels.MatchEqual, "foo", "bar")
	require.NoError(t, err)
	m2, err := labels.NewMatcher(labels.MatchEqual, "bar", "baz")
	require.NoError(t, err)
	m3, err := labels.NewMatcher(labels.MatchEqual, "baz", "qux")
	require.NoError(t, err)
	m4, err := labels.NewMatcher(labels.MatchEqual, "qux", "corge")
	require.NoError(t, err)

	r := []InhibitRule{{
		SourceMatchers: config.Matchers{m1},
		TargetMatchers: config.Matchers{m2},
	}, {
		SourceMatchers: config.Matchers{m3},
		TargetMatchers: config.Matchers{m4},
	}}
	am.setInhibitionRulesMetrics(r)

	require.NoError(t, testutil.GatherAndCompare(reg, bytes.NewBufferString(`
							# HELP grafana_alerting_alertmanager_inhibition_rules Number of configured inhibition rules.
        	            	# TYPE grafana_alerting_alertmanager_inhibition_rules gauge
        	            	grafana_alerting_alertmanager_inhibition_rules{org="1"} 2
`), "grafana_alerting_alertmanager_inhibition_rules"))
}

func TestGrafanaAlertmanager_setReceiverMetrics(t *testing.T) {
	fn := &fakeNotifier{}
	integrations := []*nfstatus.Integration{
		nfstatus.NewIntegration(fn, fn, "grafana-oncall", 0, "test-grafana-oncall"),
		nfstatus.NewIntegration(fn, fn, "sns", 1, "test-sns"),
	}

	am, reg := setupAMTest(t)

	receivers := []*nfstatus.Receiver{
		nfstatus.NewReceiver("ActiveNoIntegrations", true, nil),
		nfstatus.NewReceiver("InactiveNoIntegrations", false, nil),
		nfstatus.NewReceiver("ActiveMultipleIntegrations", true, integrations),
		nfstatus.NewReceiver("InactiveMultipleIntegrations", false, integrations),
	}

	am.setReceiverMetrics(receivers, 2)

	require.NoError(t, testutil.GatherAndCompare(reg, bytes.NewBufferString(`
        	            	# HELP grafana_alerting_alertmanager_integrations Number of configured integrations.
        	            	# TYPE grafana_alerting_alertmanager_integrations gauge
        	            	grafana_alerting_alertmanager_integrations{org="1",type="grafana-oncall"} 2
        	            	grafana_alerting_alertmanager_integrations{org="1",type="sns"} 2
        	            	# HELP grafana_alerting_alertmanager_receivers Number of configured receivers by state. It is considered active if used within a route.
        	            	# TYPE grafana_alerting_alertmanager_receivers gauge
        	            	grafana_alerting_alertmanager_receivers{org="1",state="active"} 2
        	            	grafana_alerting_alertmanager_receivers{org="1",state="inactive"} 2
`), "grafana_alerting_alertmanager_receivers", "grafana_alerting_alertmanager_integrations"))
}

// Tests cleanup of expired Silences. We rely on prometheus/alertmanager for
// our alert silencing functionality, so we rely on its tests. However, we
// implement a custom maintenance function for silences, because we snapshot
// our data differently, so we test that functionality.
func TestSilenceCleanup(t *testing.T) {
	am, _ := setupAMTest(t)
	now := time.Now()
	dt := func(t time.Time) strfmt.DateTime { return strfmt.DateTime(t) }

	makeSilence := func(comment string, createdBy string,
		startsAt, endsAt strfmt.DateTime, matchers amv2.Matchers) *PostableSilence {
		return &PostableSilence{
			ID: "",
			Silence: amv2.Silence{
				Comment:   &comment,
				CreatedBy: &createdBy,
				StartsAt:  &startsAt,
				EndsAt:    &endsAt,
				Matchers:  matchers,
			},
		}
	}

	tru := true
	testString := "testName"
	matchers := amv2.Matchers{&amv2.Matcher{Name: &testString, IsEqual: &tru, IsRegex: &tru, Value: &testString}}
	// Create silences - one in the future, one currently active, one expired but
	// retained, one expired and not retained.
	silences := []*PostableSilence{
		// Active in future
		makeSilence("", "tests", dt(now.Add(5*time.Hour)), dt(now.Add(6*time.Hour)), matchers),
		// Active now
		makeSilence("", "tests", dt(now.Add(-5*time.Hour)), dt(now.Add(6*time.Hour)), matchers),
		// Expiring soon.
		makeSilence("", "tests", dt(now.Add(-5*time.Hour)), dt(now.Add(5*time.Second)), matchers),
		// Expiring *very* soon
		makeSilence("", "tests", dt(now.Add(-5*time.Hour)), dt(now.Add(2*time.Second)), matchers),
	}

	for _, s := range silences {
		_, err := am.CreateSilence(s)
		require.NoError(t, err)
	}

	// Let enough time pass for the maintenance window to run.
	require.Eventually(t, func() bool {
		// So, what silences do we have now?
		found, err := am.ListSilences(nil)
		require.NoError(t, err)
		return len(found) == 3
	}, 3*time.Second, 150*time.Millisecond)

	// Wait again for another silence to expire.
	require.Eventually(t, func() bool {
		found, err := am.ListSilences(nil)
		require.NoError(t, err)
		return len(found) == 2
	}, 6*time.Second, 150*time.Millisecond)
}

func TestStatusForTestReceivers(t *testing.T) {
	t.Run("assert HTTP 400 Status Bad Request for no receivers", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{}, []*APIReceiver{}, time.Now())
		require.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("assert HTTP 400 Bad Request when all invalid receivers", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{
			{
				ReceiverName: "receiver 1",
				Config:       &GrafanaIntegrationConfig{Name: "integration 1"},
				Error: IntegrationValidationError{
					Integration: &GrafanaIntegrationConfig{Name: "integration 1"},
					Err:         errors.New("error 1"),
				},
			},
			{
				ReceiverName: "receiver 2",
				Config:       &GrafanaIntegrationConfig{Name: "integration 2"},
				Error: IntegrationValidationError{
					Integration: &GrafanaIntegrationConfig{Name: "integration 2"},
					Err:         errors.New("error 2"),
				},
			},
		}, []*APIReceiver{
			{
				ConfigReceiver: ConfigReceiver{
					Name: "receiver 1",
				},
				GrafanaIntegrations: GrafanaIntegrations{
					Integrations: []*GrafanaIntegrationConfig{
						{Name: "integration 1"},
					},
				},
			},
			{
				ConfigReceiver: ConfigReceiver{
					Name: "receiver 2",
				},
				GrafanaIntegrations: GrafanaIntegrations{
					Integrations: []*GrafanaIntegrationConfig{
						{Name: "integration 2"},
					},
				},
			},
		}, time.Now())
		require.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("assert HTTP 408 Request Timeout when all receivers timed out", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{
			{
				ReceiverName: "receiver 1",
				Config:       &GrafanaIntegrationConfig{Name: "integration 1"},
				Error: IntegrationTimeoutError{
					Integration: &GrafanaIntegrationConfig{Name: "integration 1"},
					Err:         errors.New("error 1"),
				},
			},
			{
				ReceiverName: "receiver 2",
				Config:       &GrafanaIntegrationConfig{Name: "integration 2"},
				Error: IntegrationTimeoutError{
					Integration: &GrafanaIntegrationConfig{Name: "integration 2"},
					Err:         errors.New("error 2"),
				},
			},
		}, []*APIReceiver{
			{
				ConfigReceiver: ConfigReceiver{
					Name: "receiver 1",
				},
				GrafanaIntegrations: GrafanaIntegrations{
					Integrations: []*GrafanaIntegrationConfig{
						{Name: "integration 1"},
					},
				},
			},
			{
				ConfigReceiver: ConfigReceiver{
					Name: "receiver 2",
				},
				GrafanaIntegrations: GrafanaIntegrations{
					Integrations: []*GrafanaIntegrationConfig{
						{Name: "integration 2"},
					},
				},
			},
		}, time.Now())
		require.Equal(t, http.StatusRequestTimeout, status)
	})

	t.Run("assert 207 Multi Status for different errors", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{
			{
				ReceiverName: "receiver 1",
				Config:       &GrafanaIntegrationConfig{Name: "integration 1"},
				Error: IntegrationValidationError{
					Integration: &GrafanaIntegrationConfig{Name: "integration 1"},
					Err:         errors.New("error 1"),
				},
			},
			{
				ReceiverName: "receiver 2",
				Config:       &GrafanaIntegrationConfig{Name: "integration 2"},
				Error: IntegrationTimeoutError{
					Integration: &GrafanaIntegrationConfig{Name: "integration 2"},
					Err:         errors.New("error 2"),
				},
			},
		}, []*APIReceiver{
			{
				ConfigReceiver: ConfigReceiver{
					Name: "receiver 1",
				},
				GrafanaIntegrations: GrafanaIntegrations{
					Integrations: []*GrafanaIntegrationConfig{
						{Name: "integration 1"},
					},
				},
			},
			{
				ConfigReceiver: ConfigReceiver{
					Name: "receiver 2",
				},
				GrafanaIntegrations: GrafanaIntegrations{
					Integrations: []*GrafanaIntegrationConfig{
						{Name: "integration 2"},
					},
				},
			},
		}, time.Now())
		require.Equal(t, http.StatusMultiStatus, status)
	})

	t.Run("assert 200 for no errors", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{
			{
				ReceiverName: "receiver 1",
				Config:       &GrafanaIntegrationConfig{Name: "integration 1"},
			},
			{
				ReceiverName: "receiver 2",
				Config:       &GrafanaIntegrationConfig{Name: "integration 2"},
			},
		}, []*APIReceiver{
			{
				ConfigReceiver: ConfigReceiver{
					Name: "receiver 1",
				},
				GrafanaIntegrations: GrafanaIntegrations{
					Integrations: []*GrafanaIntegrationConfig{
						{Name: "integration 1"},
					},
				},
			},
			{
				ConfigReceiver: ConfigReceiver{
					Name: "receiver 2",
				},
				GrafanaIntegrations: GrafanaIntegrations{
					Integrations: []*GrafanaIntegrationConfig{
						{Name: "integration 2"},
					},
				},
			},
		}, time.Now())
		require.Equal(t, http.StatusOK, status)
	})
}
