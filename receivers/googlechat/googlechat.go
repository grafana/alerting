package googlechat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"

	"github.com/grafana/alerting/images"
	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	template2 "github.com/grafana/alerting/templates"
)

// Notifier is responsible for sending
// alert notifications to Google chat.
type Notifier struct {
	*receivers.Base
	log        logging.Logger
	ns         receivers.WebhookSender
	images     images.ImageStore
	tmpl       *template.Template
	settings   Config
	appVersion string
}

var (
	// Provides current time. Can be overwritten in tests.
	timeNow = time.Now
)

func New(fc receivers.FactoryConfig) (*Notifier, error) {
	settings, err := NewConfig(fc.Config.Settings)
	if err != nil {
		return nil, err
	}
	return &Notifier{
		Base:       receivers.NewBase(fc.Config),
		log:        fc.Logger,
		ns:         fc.NotificationService,
		images:     fc.ImageStore,
		tmpl:       fc.Template,
		settings:   settings,
		appVersion: fc.GrafanaBuildVersion,
	}, nil
}

// Notify send an alert notification to Google Chat.
func (gcn *Notifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	gcn.log.Debug("executing Google Chat notification")

	var tmplErr error
	tmpl, _ := template2.TmplText(ctx, gcn.tmpl, as, gcn.log, &tmplErr)

	var widgets []widget

	if msg := tmpl(gcn.settings.Message); msg != "" {
		// Add a text paragraph widget for the message if there is a message.
		// Google Chat API doesn't accept an empty text property.
		widgets = append(widgets, textParagraphWidget{Text: text{Text: msg}})
	}

	if tmplErr != nil {
		gcn.log.Warn("failed to template Google Chat message", "error", tmplErr.Error())
		tmplErr = nil
	}

	ruleURL := receivers.JoinURLPath(gcn.tmpl.ExternalURL.String(), "/alerting/list", gcn.log)
	if gcn.isURLAbsolute(ruleURL) {
		// Add a button widget (link to Grafana).
		widgets = append(widgets, buttonWidget{
			Buttons: []button{
				{
					TextButton: textButton{
						Text: "OPEN IN GRAFANA",
						OnClick: onClick{
							OpenLink: openLink{
								URL: ruleURL,
							},
						},
					},
				},
			},
		})
	} else {
		gcn.log.Warn("Grafana external URL setting is missing or invalid. Skipping 'open in grafana' button to prevent Google from displaying empty alerts.", "ruleURL", ruleURL)
	}

	// Add text paragraph widget for the build version and timestamp.
	widgets = append(widgets, textParagraphWidget{
		Text: text{
			Text: "Grafana v" + gcn.appVersion + " | " + (timeNow()).Format(time.RFC822),
		},
	})

	title := tmpl(gcn.settings.Title)
	// Nest the required structs.
	res := &outerStruct{
		PreviewText:  title,
		FallbackText: title,
		Cards: []card{
			{
				Header: header{Title: title},
				Sections: []section{
					{Widgets: widgets},
				},
			},
		},
	}
	if screenshots := gcn.buildScreenshotCard(ctx, as); screenshots != nil {
		res.Cards = append(res.Cards, *screenshots)
	}

	if tmplErr != nil {
		gcn.log.Warn("failed to template GoogleChat message", "error", tmplErr.Error())
		tmplErr = nil
	}

	u := tmpl(gcn.settings.URL)
	if tmplErr != nil {
		gcn.log.Warn("failed to template GoogleChat URL", "error", tmplErr.Error(), "fallback", gcn.settings.URL)
		u = gcn.settings.URL
	}

	body, err := json.Marshal(res)
	if err != nil {
		return false, fmt.Errorf("marshal json: %w", err)
	}

	cmd := &receivers.SendWebhookSettings{
		URL:        u,
		HTTPMethod: "POST",
		HTTPHeader: map[string]string{
			"Content-Type": "application/json; charset=UTF-8",
		},
		Body: string(body),
	}

	if err := gcn.ns.SendWebhook(ctx, cmd); err != nil {
		gcn.log.Error("Failed to send Google Hangouts Chat alert", "error", err, "webhook", gcn.Name)
		return false, err
	}

	return true, nil
}

func (gcn *Notifier) SendResolved() bool {
	return !gcn.GetDisableResolveMessage()
}

func (gcn *Notifier) isURLAbsolute(urlToCheck string) bool {
	parsed, err := url.Parse(urlToCheck)
	if err != nil {
		gcn.log.Warn("could not parse URL", "urlToCheck", urlToCheck)
		return false
	}

	return parsed.IsAbs()
}

func (gcn *Notifier) buildScreenshotCard(ctx context.Context, alerts []*types.Alert) *card {
	card := card{
		Header:   header{Title: "Screenshots"},
		Sections: []section{},
	}

	_ = images.WithStoredImages(ctx, gcn.log, gcn.images,
		func(index int, image images.Image) error {
			if len(image.URL) == 0 {
				return nil
			}

			section := section{
				Widgets: []widget{
					textParagraphWidget{
						Text: text{
							Text: fmt.Sprintf("%s: %s", alerts[index].Status(), alerts[index].Name()),
						},
					},
					imageWidget{Image: imageData{ImageURL: image.URL}},
				},
			}
			card.Sections = append(card.Sections, section)

			return nil
		}, alerts...)

	if len(card.Sections) == 0 {
		return nil
	}
	return &card
}

// Structs used to build a custom Google Hangouts Chat message card.
// See: https://developers.google.com/hangouts/chat/reference/message-formats/cards
type outerStruct struct {
	PreviewText  string `json:"previewText"`
	FallbackText string `json:"fallbackText"`
	Cards        []card `json:"cards"`
}

type card struct {
	Header   header    `json:"header"`
	Sections []section `json:"sections"`
}

type header struct {
	Title string `json:"title"`
}

type section struct {
	Widgets []widget `json:"widgets"`
}

// "generic" widget used to add different types of widgets (buttonWidget, textParagraphWidget, imageWidget)
type widget interface{}

type buttonWidget struct {
	Buttons []button `json:"buttons"`
}

type textParagraphWidget struct {
	Text text `json:"textParagraph"`
}

type imageWidget struct {
	Image imageData `json:"image"`
}

type imageData struct {
	ImageURL string `json:"imageUrl"`
}

type text struct {
	Text string `json:"text"`
}

type button struct {
	TextButton textButton `json:"textButton"`
}

type textButton struct {
	Text    string  `json:"text"`
	OnClick onClick `json:"onClick"`
}

type onClick struct {
	OpenLink openLink `json:"openLink"`
}

type openLink struct {
	URL string `json:"url"`
}
