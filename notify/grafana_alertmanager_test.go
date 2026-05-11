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
	"github.com/stretchr/testify/require"

	amv2 "github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/pkg/labels"
	"github.com/prometheus/alertmanager/provider/mem"
	"github.com/prometheus/alertmanager/timeinterval"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/models"
	"github.com/grafana/alerting/notify/nfstatus"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/receivers/schema"
	"github.com/grafana/alerting/templates"
)

type withOptsFn func(opts *GrafanaAlertmanagerOpts)

func testOpts(t *testing.T, reg prometheus.Registerer) GrafanaAlertmanagerOpts {
	t.Helper()

	return GrafanaAlertmanagerOpts{
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
}

func setupAMTest(t *testing.T, withOpts ...withOptsFn) (*GrafanaAlertmanager, *prometheus.Registry) {
	t.Helper()

	reg := prometheus.NewPedanticRegistry()

	opts := testOpts(t, reg)
	for _, fn := range withOpts {
		fn(&opts)
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
		nfstatus.NewIntegration(fn, fn, "grafana-oncall", 0, "test-grafana-oncall", nil, log.NewNopLogger()),
		nfstatus.NewIntegration(fn, fn, "sns", 1, "test-sns", nil, log.NewNopLogger()),
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
		startsAt, endsAt strfmt.DateTime, matchers amv2.Matchers,
	) *PostableSilence {
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
		_, status := newTestReceiversResult(types.Alert{}, []result{}, []models.ReceiverConfig{}, time.Now())
		require.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("assert HTTP 400 Bad Request when all invalid receivers", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{
			{
				ReceiverName: "receiver 1",
				Config:       &models.IntegrationConfig{Name: "integration 1"},
				Error: IntegrationValidationError{
					Integration: &models.IntegrationConfig{Name: "integration 1"},
					Err:         errors.New("error 1"),
				},
			},
			{
				ReceiverName: "receiver 2",
				Config:       &models.IntegrationConfig{Name: "integration 2"},
				Error: IntegrationValidationError{
					Integration: &models.IntegrationConfig{Name: "integration 2"},
					Err:         errors.New("error 2"),
				},
			},
		}, []models.ReceiverConfig{
			{
				Name: "receiver 1",
				Integrations: []*models.IntegrationConfig{
					{Name: "integration 1"},
				},
			},
			{
				Name: "receiver 2",
				Integrations: []*models.IntegrationConfig{
					{Name: "integration 2"},
				},
			},
		}, time.Now())
		require.Equal(t, http.StatusBadRequest, status)
	})

	t.Run("assert HTTP 408 Request Timeout when all receivers timed out", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{
			{
				ReceiverName: "receiver 1",
				Config:       &models.IntegrationConfig{Name: "integration 1"},
				Error: IntegrationTimeoutError{
					Integration: &models.IntegrationConfig{Name: "integration 1"},
					Err:         errors.New("error 1"),
				},
			},
			{
				ReceiverName: "receiver 2",
				Config:       &models.IntegrationConfig{Name: "integration 2"},
				Error: IntegrationTimeoutError{
					Integration: &models.IntegrationConfig{Name: "integration 2"},
					Err:         errors.New("error 2"),
				},
			},
		}, []models.ReceiverConfig{
			{
				Name: "receiver 1",
				Integrations: []*models.IntegrationConfig{
					{Name: "integration 1"},
				},
			},
			{
				Name: "receiver 2",
				Integrations: []*models.IntegrationConfig{
					{Name: "integration 2"},
				},
			},
		}, time.Now())
		require.Equal(t, http.StatusRequestTimeout, status)
	})

	t.Run("assert 207 Multi Status for different errors", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{
			{
				ReceiverName: "receiver 1",
				Config:       &models.IntegrationConfig{Name: "integration 1"},
				Error: IntegrationValidationError{
					Integration: &models.IntegrationConfig{Name: "integration 1"},
					Err:         errors.New("error 1"),
				},
			},
			{
				ReceiverName: "receiver 2",
				Config:       &models.IntegrationConfig{Name: "integration 2"},
				Error: IntegrationTimeoutError{
					Integration: &models.IntegrationConfig{Name: "integration 2"},
					Err:         errors.New("error 2"),
				},
			},
		}, []models.ReceiverConfig{
			{
				Name: "receiver 1",
				Integrations: []*models.IntegrationConfig{
					{Name: "integration 1"},
				},
			},
			{
				Name: "receiver 2",
				Integrations: []*models.IntegrationConfig{
					{Name: "integration 2"},
				},
			},
		}, time.Now())
		require.Equal(t, http.StatusMultiStatus, status)
	})

	t.Run("assert 200 for no errors", func(t *testing.T) {
		_, status := newTestReceiversResult(types.Alert{}, []result{
			{
				ReceiverName: "receiver 1",
				Config:       &models.IntegrationConfig{Name: "integration 1"},
			},
			{
				ReceiverName: "receiver 2",
				Config:       &models.IntegrationConfig{Name: "integration 2"},
			},
		}, []models.ReceiverConfig{
			{
				Name: "receiver 1",
				Integrations: []*models.IntegrationConfig{
					{Name: "integration 1"},
				},
			},
			{
				Name: "receiver 2",
				Integrations: []*models.IntegrationConfig{
					{Name: "integration 2"},
				},
			},
		}, time.Now())
		require.Equal(t, http.StatusOK, status)
	})
}

func Test_GrafanaAlertmanagerOpts_Validate(t *testing.T) {
	t.Run("if SyncDispatchTimer, flushlog should not be nil", func(t *testing.T) {
		testOpts := testOpts(t, prometheus.NewRegistry())
		testOpts.DispatchTimer = DispatchTimerSync
		require.NotNil(t, testOpts.Validate())
	})
}

func TestCalculateConfigFingerprint(t *testing.T) {
	baseA := richNotificationsConfiguration(t, "default")
	baseB := richNotificationsConfiguration(t, "default")
	require.Equal(t, CalculateConfigFingerprint(baseA), CalculateConfigFingerprint(baseB))

	fieldGetters := map[string]func(ConfigFingerprint) uint64{
		"RoutingTree":       func(fp ConfigFingerprint) uint64 { return fp.RoutingTree },
		"InhibitRules":      func(fp ConfigFingerprint) uint64 { return fp.InhibitRules },
		"MuteTimeIntervals": func(fp ConfigFingerprint) uint64 { return fp.MuteTimeIntervals },
		"TimeIntervals":     func(fp ConfigFingerprint) uint64 { return fp.TimeIntervals },
		"Templates":         func(fp ConfigFingerprint) uint64 { return fp.Templates },
		"Receivers":         func(fp ConfigFingerprint) uint64 { return fp.Receivers },
		"Limits":            func(fp ConfigFingerprint) uint64 { return fp.Limits },
	}

	cases := []struct {
		name    string
		field   string
		mutator func(cfg *NotificationsConfiguration)
	}{
		{
			name:  "routing tree",
			field: "RoutingTree",
			mutator: func(cfg *NotificationsConfiguration) {
				cfg.RoutingTree.Routes[0].Match["cluster"] = "prod-us-west-2"
			},
		},
		{
			name:  "inhibit rules",
			field: "InhibitRules",
			mutator: func(cfg *NotificationsConfiguration) {
				cfg.InhibitRules[0].Equal[0] = "service"
			},
		},
		{
			name:  "mute time intervals",
			field: "MuteTimeIntervals",
			mutator: func(cfg *NotificationsConfiguration) {
				cfg.MuteTimeIntervals[0].TimeIntervals[0].Times[0].StartMinute++
			},
		},
		{
			name:  "time intervals",
			field: "TimeIntervals",
			mutator: func(cfg *NotificationsConfiguration) {
				cfg.TimeIntervals[0].TimeIntervals[0].Weekdays[0].Begin = 0
			},
		},
		{
			name:  "templates",
			field: "Templates",
			mutator: func(cfg *NotificationsConfiguration) {
				cfg.Templates[0].Template = `{{ define "custom.title" }}Alert changed: {{ .Status }}{{ end }}`
			},
		},
		{
			name:  "receivers",
			field: "Receivers",
			mutator: func(cfg *NotificationsConfiguration) {
				cfg.Receivers[0].Integrations[0].Name = "primary-webhook-v2"
			},
		},
		{
			name:  "limits",
			field: "Limits",
			mutator: func(cfg *NotificationsConfiguration) {
				cfg.Limits.Templates.MaxTemplateOutputSize++
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := richNotificationsConfiguration(t, "default")
			mutated := richNotificationsConfiguration(t, "default")
			tc.mutator(&mutated)

			originalFP := CalculateConfigFingerprint(original)
			mutatedFP := CalculateConfigFingerprint(mutated)

			require.NotEqual(t, originalFP.Overall, mutatedFP.Overall)
			require.NotEqual(t, fieldGetters[tc.field](originalFP), fieldGetters[tc.field](mutatedFP))

			for field, get := range fieldGetters {
				if field == tc.field {
					continue
				}
				require.Equal(t, get(originalFP), get(mutatedFP), "unexpected hash change for field %s", field)
			}
		})
	}
}

type staticDispatcherLimits struct {
	maxNumberOfAggregationGroups int
}

func (l staticDispatcherLimits) MaxNumberOfAggregationGroups() int {
	return l.maxNumberOfAggregationGroups
}

func mustLabelMatcher(t *testing.T, mt labels.MatchType, name, value string) *labels.Matcher {
	t.Helper()
	m, err := labels.NewMatcher(mt, name, value)
	require.NoError(t, err)
	return m
}

func richNotificationsConfiguration(t *testing.T, rootReceiver string) NotificationsConfiguration {
	t.Helper()

	rootGroupWait := model.Duration(30 * time.Second)
	rootGroupInterval := model.Duration(5 * time.Minute)
	rootRepeatInterval := model.Duration(4 * time.Hour)
	childGroupWait := model.Duration(1 * time.Minute)
	childGroupInterval := model.Duration(10 * time.Minute)
	childRepeatInterval := model.Duration(2 * time.Hour)

	return NotificationsConfiguration{
		RoutingTree: &Route{
			Receiver:   rootReceiver,
			GroupByStr: []string{"alertname", "cluster", "namespace"},
			GroupBy:    []model.LabelName{"alertname", "cluster", "namespace"},
			Match: map[string]string{
				"team": "platform",
				"env":  "prod",
			},
			Matchers: config.Matchers{
				mustLabelMatcher(t, labels.MatchEqual, "service", "api"),
				mustLabelMatcher(t, labels.MatchNotEqual, "severity", "none"),
			},
			MuteTimeIntervals:   []string{"non_business_hours"},
			ActiveTimeIntervals: []string{"business_hours"},
			Continue:            true,
			GroupWait:           &rootGroupWait,
			GroupInterval:       &rootGroupInterval,
			RepeatInterval:      &rootRepeatInterval,
			Routes: []*Route{
				{
					Receiver:   "database-team",
					GroupByStr: []string{"alertname", "database"},
					GroupBy:    []model.LabelName{"alertname", "database"},
					Match: map[string]string{
						"team":    "database",
						"cluster": "prod-us-east-1",
					},
					Matchers: config.Matchers{
						mustLabelMatcher(t, labels.MatchEqual, "component", "postgres"),
					},
					MuteTimeIntervals: []string{"db_maintenance"},
					GroupWait:         &childGroupWait,
					GroupInterval:     &childGroupInterval,
					RepeatInterval:    &childRepeatInterval,
					Routes: []*Route{
						{
							Receiver: "database-pager",
							Match: map[string]string{
								"severity": "critical",
								"region":   "us-east-1",
							},
							Matchers: config.Matchers{
								mustLabelMatcher(t, labels.MatchEqual, "tier", "backend"),
							},
						},
					},
				},
			},
		},
		InhibitRules: []InhibitRule{
			{
				SourceMatch: map[string]string{
					"severity": "critical",
					"team":     "platform",
				},
				SourceMatchers: config.Matchers{
					mustLabelMatcher(t, labels.MatchEqual, "environment", "prod"),
				},
				TargetMatch: map[string]string{
					"severity": "warning",
				},
				TargetMatchers: config.Matchers{
					mustLabelMatcher(t, labels.MatchEqual, "component", "api"),
				},
				Equal: []string{"alertname", "cluster", "namespace"},
			},
		},
		MuteTimeIntervals: []MuteTimeInterval{
			{
				Name: "non_business_hours",
				TimeIntervals: []timeinterval.TimeInterval{
					{
						Times: []timeinterval.TimeRange{
							{StartMinute: 0, EndMinute: 540},
							{StartMinute: 1080, EndMinute: 1440},
						},
						Weekdays: []timeinterval.WeekdayRange{
							{InclusiveRange: timeinterval.InclusiveRange{Begin: 1, End: 5}},
						},
						Months: []timeinterval.MonthRange{
							{InclusiveRange: timeinterval.InclusiveRange{Begin: 1, End: 12}},
						},
						Years: []timeinterval.YearRange{
							{InclusiveRange: timeinterval.InclusiveRange{Begin: 2024, End: 2030}},
						},
					},
				},
			},
			{
				Name: "db_maintenance",
				TimeIntervals: []timeinterval.TimeInterval{
					{
						Times: []timeinterval.TimeRange{
							{StartMinute: 120, EndMinute: 240},
						},
						DaysOfMonth: []timeinterval.DayOfMonthRange{
							{InclusiveRange: timeinterval.InclusiveRange{Begin: 1, End: 3}},
						},
						Weekdays: []timeinterval.WeekdayRange{
							{InclusiveRange: timeinterval.InclusiveRange{Begin: 0, End: 0}},
						},
						Location: &timeinterval.Location{
							Location: time.FixedZone("EST", -5*60*60),
						},
					},
				},
			},
		},
		TimeIntervals: []TimeInterval{
			{
				Name: "business_hours",
				TimeIntervals: []timeinterval.TimeInterval{
					{
						Times: []timeinterval.TimeRange{
							{StartMinute: 540, EndMinute: 1020},
						},
						Weekdays: []timeinterval.WeekdayRange{
							{InclusiveRange: timeinterval.InclusiveRange{Begin: 1, End: 5}},
						},
						Months: []timeinterval.MonthRange{
							{InclusiveRange: timeinterval.InclusiveRange{Begin: 1, End: 12}},
						},
						Years: []timeinterval.YearRange{
							{InclusiveRange: timeinterval.InclusiveRange{Begin: 2024, End: 2030}},
						},
						Location: &timeinterval.Location{
							Location: time.FixedZone("PST", -8*60*60),
						},
					},
				},
			},
		},
		Templates: []templates.TemplateDefinition{
			{
				Name:     "custom.title",
				Template: `{{ define "custom.title" }}Alert: {{ .Status }}{{ end }}`,
				Kind:     templates.GrafanaKind,
			},
			{
				Name:     "custom.mimir.body",
				Template: `{{ define "custom.mimir.body" }}{{ .Receiver }}{{ end }}`,
				Kind:     templates.MimirKind,
			},
		},
		Receivers: []models.ReceiverConfig{
			{
				Name: "grafana-default",
				Integrations: []*models.IntegrationConfig{
					{
						UID:                   "integration-webhook-main",
						Name:                  "primary-webhook",
						Type:                  schema.WebhookType,
						Version:               schema.V1,
						DisableResolveMessage: false,
						Settings:              []byte(`{"url":"https://example.org/hooks/primary","httpMethod":"POST","maxAlerts":10}`),
						SecureSettings: map[string]string{
							"authorizationHeader": "Bearer token-a",
							"password":            "secret-a",
						},
					},
					{
						UID:                   "integration-slack-main",
						Name:                  "primary-slack",
						Type:                  schema.SlackType,
						Version:               schema.V1,
						DisableResolveMessage: true,
						Settings:              []byte(`{"recipient":"#alerts-prod","title":"Critical alert","mentionUsers":"oncall"}`),
						SecureSettings: map[string]string{
							"url": "https://hooks.slack.com/services/T000/B000/XXX",
						},
					},
				},
			},
			{
				Name: "grafana-fallback",
				Integrations: []*models.IntegrationConfig{
					{
						UID:                   "integration-email-fallback",
						Name:                  "fallback-email",
						Type:                  schema.EmailType,
						DisableResolveMessage: false,
						Settings:              []byte(`{"singleEmail":true,"addresses":"oncall@example.org;ops@example.org"}`),
						SecureSettings: map[string]string{
							"password": "smtp-secret",
						},
					},
				},
			},
		},
		Limits: DynamicLimits{
			Dispatcher: staticDispatcherLimits{maxNumberOfAggregationGroups: 250},
			Templates: templates.Limits{
				MaxTemplateOutputSize: 2 * 1024 * 1024,
			},
		},
	}
}
