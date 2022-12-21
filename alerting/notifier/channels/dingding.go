package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
)

const defaultDingdingMsgType = "link"

type dingDingSettings struct {
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
	MessageType string `json:"msgType,omitempty" yaml:"msgType,omitempty"`
	Title       string `json:"title,omitempty" yaml:"title,omitempty"`
	Message     string `json:"message,omitempty" yaml:"message,omitempty"`
}

func buildDingDingSettings(fc FactoryConfig) (*dingDingSettings, error) {
	var settings dingDingSettings
	err := json.Unmarshal(fc.Config.Settings, &settings)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	if settings.URL == "" {
		return nil, errors.New("could not find url property in settings")
	}
	if settings.MessageType == "" {
		settings.MessageType = defaultDingdingMsgType
	}
	if settings.Title == "" {
		settings.Title = DefaultMessageTitleEmbed
	}
	if settings.Message == "" {
		settings.Message = DefaultMessageEmbed
	}
	return &settings, nil
}

func DingDingFactory(fc FactoryConfig) (NotificationChannel, error) {
	n, err := newDingDingNotifier(fc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return n, nil
}

// newDingDingNotifier is the constructor for the Dingding notifier
func newDingDingNotifier(fc FactoryConfig) (*DingDingNotifier, error) {
	settings, err := buildDingDingSettings(fc)
	if err != nil {
		return nil, err
	}
	return &DingDingNotifier{
		Base:     NewBase(fc.Config),
		log:      fc.Logger,
		ns:       fc.NotificationService,
		tmpl:     fc.Template,
		settings: *settings,
	}, nil
}

// DingDingNotifier is responsible for sending alert notifications to ding ding.
type DingDingNotifier struct {
	*Base
	log      Logger
	ns       WebhookSender
	tmpl     *template.Template
	settings dingDingSettings
}

// Notify sends the alert notification to dingding.
func (dd *DingDingNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	dd.log.Info("sending dingding")

	dingDingURL := buildDingDingURL(dd)

	var tmplErr error
	tmpl, _ := TmplText(ctx, dd.tmpl, as, dd.log, &tmplErr)

	message := tmpl(dd.settings.Message)
	title := tmpl(dd.settings.Title)

	msgType := tmpl(dd.settings.MessageType)
	b, err := buildBody(dingDingURL, msgType, title, message)
	if err != nil {
		return false, err
	}

	if tmplErr != nil {
		dd.log.Warn("failed to template DingDing message", "error", tmplErr.Error())
		tmplErr = nil
	}

	u := tmpl(dd.settings.URL)
	if tmplErr != nil {
		dd.log.Warn("failed to template DingDing URL", "error", tmplErr.Error(), "fallback", dd.settings.URL)
		u = dd.settings.URL
	}

	cmd := &SendWebhookSettings{URL: u, Body: b}

	if err := dd.ns.SendWebhook(ctx, cmd); err != nil {
		return false, fmt.Errorf("send notification to dingding: %w", err)
	}

	return true, nil
}

func (dd *DingDingNotifier) SendResolved() bool {
	return !dd.GetDisableResolveMessage()
}

func buildDingDingURL(dd *DingDingNotifier) string {
	q := url.Values{
		"pc_slide": {"false"},
		"url":      {joinURLPath(dd.tmpl.ExternalURL.String(), "/alerting/list", dd.log)},
	}

	// Use special link to auto open the message url outside Dingding
	// Refer: https://open-doc.dingtalk.com/docs/doc.htm?treeId=385&articleId=104972&docType=1#s9
	return "dingtalk://dingtalkclient/page/link?" + q.Encode()
}

func buildBody(url string, msgType string, title string, msg string) (string, error) {
	var bodyMsg map[string]interface{}
	if msgType == "actionCard" {
		bodyMsg = map[string]interface{}{
			"msgtype": "actionCard",
			"actionCard": map[string]string{
				"text":        msg,
				"title":       title,
				"singleTitle": "More",
				"singleURL":   url,
			},
		}
	} else {
		bodyMsg = map[string]interface{}{
			"msgtype": "link",
			"link": map[string]string{
				"text":       msg,
				"title":      title,
				"messageUrl": url,
			},
		}
	}
	body, err := json.Marshal(bodyMsg)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
