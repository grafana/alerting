package googlechat

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	constNow := time.Now()
	defer mockTimeNow(constNow)()
	appVersion := fmt.Sprintf("%d.0.0", rand.Uint32())

	cases := []struct {
		name        string
		settings    Config
		alerts      []*types.Alert
		expMsg      *outerStruct
		expMsgError error
		externalURL string
	}{
		{
			name: "One alert",
			settings: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: templates.DefaultMessageEmbed,
				URL:     "http://localhost",
			},
			externalURL: "http://localhost",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &outerStruct{
				PreviewText:  "[FIRING:1]  (val1)",
				FallbackText: "[FIRING:1]  (val1)",
				Cards: []card{
					{
						Header: header{
							Title: "[FIRING:1]  (val1)",
						},
						Sections: []section{
							{
								Widgets: []widget{
									textParagraphWidget{
										Text: text{
											Text: "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
										},
									},
									buttonWidget{
										Buttons: []button{
											{
												TextButton: textButton{
													Text: "OPEN IN GRAFANA",
													OnClick: onClick{
														OpenLink: openLink{
															URL: "http://localhost/alerting/list",
														},
													},
												},
											},
										},
									},
									textParagraphWidget{
										Text: text{
											// RFC822 only has the minute, hence it works in most cases.
											Text: "Grafana v" + appVersion + " | " + constNow.Format(time.RFC822),
										},
									},
								},
							},
						},
					},
				},
			},
			expMsgError: nil,
		}, {
			name: "Multiple alerts",
			settings: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: templates.DefaultMessageEmbed,
				URL:     "http://localhost",
			},
			externalURL: "http://localhost",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2"},
					},
				},
			},
			expMsg: &outerStruct{
				PreviewText:  "[FIRING:2]  ",
				FallbackText: "[FIRING:2]  ",
				Cards: []card{
					{
						Header: header{
							Title: "[FIRING:2]  ",
						},
						Sections: []section{
							{
								Widgets: []widget{
									textParagraphWidget{
										Text: text{
											Text: "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
										},
									},
									buttonWidget{
										Buttons: []button{
											{
												TextButton: textButton{
													Text: "OPEN IN GRAFANA",
													OnClick: onClick{
														OpenLink: openLink{
															URL: "http://localhost/alerting/list",
														},
													},
												},
											},
										},
									},
									textParagraphWidget{
										Text: text{
											Text: "Grafana v" + appVersion + " | " + constNow.Format(time.RFC822),
										},
									},
								},
							},
						},
					},
				},
			},
			expMsgError: nil,
		},
		{
			name: "Customized message",
			settings: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: "I'm a custom template and you have {{ len .Alerts.Firing }} firing alert.",
				URL:     "http://localhost",
			},
			externalURL: "http://localhost",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &outerStruct{
				PreviewText:  "[FIRING:1]  (val1)",
				FallbackText: "[FIRING:1]  (val1)",
				Cards: []card{
					{
						Header: header{
							Title: "[FIRING:1]  (val1)",
						},
						Sections: []section{
							{
								Widgets: []widget{
									textParagraphWidget{
										Text: text{
											Text: "I'm a custom template and you have 1 firing alert.",
										},
									},
									buttonWidget{
										Buttons: []button{
											{
												TextButton: textButton{
													Text: "OPEN IN GRAFANA",
													OnClick: onClick{
														OpenLink: openLink{
															URL: "http://localhost/alerting/list",
														},
													},
												},
											},
										},
									},
									textParagraphWidget{
										Text: text{
											// RFC822 only has the minute, hence it works in most cases.
											Text: "Grafana v" + appVersion + " | " + constNow.Format(time.RFC822),
										},
									},
								},
							},
						},
					},
				},
			},
			expMsgError: nil,
		}, {
			name: "Customized title",
			settings: Config{
				Title:   "Alerts firing: {{ len .Alerts.Firing }}",
				Message: templates.DefaultMessageEmbed,
				URL:     "http://localhost",
			},
			externalURL: "http://localhost",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &outerStruct{
				PreviewText:  "Alerts firing: 1",
				FallbackText: "Alerts firing: 1",
				Cards: []card{
					{
						Header: header{
							Title: "Alerts firing: 1",
						},
						Sections: []section{
							{
								Widgets: []widget{
									textParagraphWidget{
										Text: text{
											Text: "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
										},
									},
									buttonWidget{
										Buttons: []button{
											{
												TextButton: textButton{
													Text: "OPEN IN GRAFANA",
													OnClick: onClick{
														OpenLink: openLink{
															URL: "http://localhost/alerting/list",
														},
													},
												},
											},
										},
									},
									textParagraphWidget{
										Text: text{
											// RFC822 only has the minute, hence it works in most cases.
											Text: "Grafana v" + appVersion + " | " + constNow.Format(time.RFC822),
										},
									},
								},
							},
						},
					},
				},
			},
			expMsgError: nil,
		}, {
			name: "Missing field in message template",
			settings: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: "I'm a custom template {{ .NotAField }} bad template",
				URL:     "http://localhost",
			},
			externalURL: "http://localhost",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &outerStruct{
				PreviewText:  "[FIRING:1]  (val1)",
				FallbackText: "[FIRING:1]  (val1)",
				Cards: []card{
					{
						Header: header{
							Title: "[FIRING:1]  (val1)",
						},
						Sections: []section{
							{
								Widgets: []widget{
									textParagraphWidget{
										Text: text{
											Text: "I'm a custom template ",
										},
									},
									buttonWidget{
										Buttons: []button{
											{
												TextButton: textButton{
													Text: "OPEN IN GRAFANA",
													OnClick: onClick{
														OpenLink: openLink{
															URL: "http://localhost/alerting/list",
														},
													},
												},
											},
										},
									},
									textParagraphWidget{
										Text: text{
											// RFC822 only has the minute, hence it works in most cases.
											Text: "Grafana v" + appVersion + " | " + constNow.Format(time.RFC822),
										},
									},
								},
							},
						},
					},
				},
			},
			expMsgError: nil,
		}, {
			name: "Invalid template",
			settings: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: "I'm a custom template {{ {.NotAField }} bad template",
				URL:     "http://localhost",
			},
			externalURL: "http://localhost",
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &outerStruct{
				PreviewText:  "[FIRING:1]  (val1)",
				FallbackText: "[FIRING:1]  (val1)",
				Cards: []card{
					{
						Header: header{
							Title: "[FIRING:1]  (val1)",
						},
						Sections: []section{
							{
								Widgets: []widget{
									buttonWidget{
										Buttons: []button{
											{
												TextButton: textButton{
													Text: "OPEN IN GRAFANA",
													OnClick: onClick{
														OpenLink: openLink{
															URL: "http://localhost/alerting/list",
														},
													},
												},
											},
										},
									},
									textParagraphWidget{
										Text: text{
											// RFC822 only has the minute, hence it works in most cases.
											Text: "Grafana v" + appVersion + " | " + constNow.Format(time.RFC822),
										},
									},
								},
							},
						},
					},
				},
			},
			expMsgError: nil,
		},
		{
			name: "Empty external URL",
			settings: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: templates.DefaultMessageEmbed,
				URL:     "http://localhost",
			}, // URL in settings = googlechat url
			externalURL: "", // external URL = URL of grafana from configuration
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &outerStruct{
				PreviewText:  "[FIRING:1]  (val1)",
				FallbackText: "[FIRING:1]  (val1)",
				Cards: []card{
					{
						Header: header{
							Title: "[FIRING:1]  (val1)",
						},
						Sections: []section{
							{
								Widgets: []widget{
									textParagraphWidget{
										Text: text{
											Text: "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\n",
										},
									},

									// No button widget here since the external URL is not absolute

									textParagraphWidget{
										Text: text{
											// RFC822 only has the minute, hence it works in most cases.
											Text: "Grafana v" + appVersion + " | " + constNow.Format(time.RFC822),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Relative external URL",
			settings: Config{
				Title:   templates.DefaultMessageTitleEmbed,
				Message: templates.DefaultMessageEmbed,
				URL:     "http://localhost",
			}, // URL in settings = googlechat url
			externalURL: "/grafana", // external URL = URL of grafana from configuration
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
					},
				},
			},
			expMsg: &outerStruct{
				PreviewText:  "[FIRING:1]  (val1)",
				FallbackText: "[FIRING:1]  (val1)",
				Cards: []card{
					{
						Header: header{
							Title: "[FIRING:1]  (val1)",
						},
						Sections: []section{
							{
								Widgets: []widget{
									textParagraphWidget{
										Text: text{
											Text: "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: /grafana/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: /grafana/d/abcd\nPanel: /grafana/d/abcd?viewPanel=efgh\n",
										},
									},

									// No button widget here since the external URL is not absolute

									textParagraphWidget{
										Text: text{
											// RFC822 only has the minute, hence it works in most cases.
											Text: "Grafana v" + appVersion + " | " + constNow.Format(time.RFC822),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl := templates.ForTests(t)

			externalURL, err := url.Parse(c.externalURL)
			require.NoError(t, err)
			tmpl.ExternalURL = externalURL

			webhookSender := receivers.MockNotificationService()
			imageProvider := &images.UnavailableProvider{}

			pn := &Notifier{
				Base: &receivers.Base{
					Name:                  "",
					Type:                  "",
					UID:                   "",
					DisableResolveMessage: false,
				},
				log:        &logging.FakeLogger{},
				ns:         webhookSender,
				tmpl:       tmpl,
				settings:   c.settings,
				images:     imageProvider,
				appVersion: appVersion,
			}

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
			ok, err := pn.Notify(ctx, c.alerts...)
			if c.expMsgError != nil {
				require.False(t, ok)
				require.Error(t, err)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			require.True(t, ok)

			require.NotEmpty(t, webhookSender.Webhook.URL)

			expBody, err := json.Marshal(c.expMsg)
			require.NoError(t, err)

			require.JSONEq(t, string(expBody), webhookSender.Webhook.Body)
		})
	}
}

// resetTimeNow resets the global variable timeNow to the default value, which is time.Now
func resetTimeNow() {
	timeNow = time.Now
}

func mockTimeNow(constTime time.Time) func() {
	timeNow = func() time.Time {
		return constTime
	}
	return resetTimeNow
}
